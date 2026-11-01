package router

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"path/filepath"
	"strings"
	"testing"

	"github.com/aknuun/cg-server/internal/store"
)

func TestCfgParseLinks(t *testing.T) {
	vm := base64.RawURLEncoding.EncodeToString([]byte(`{"v":"2","ps":"🇩🇪 DE-1","add":"de.example.com","port":"443","id":"11111111-2222-3333-4444-555555555555","aid":"0","net":"ws","host":"cdn.example.com","path":"/ws","tls":"tls"}`))
	cases := []struct {
		name     string
		link     string
		proto    string
		host     string
		port     int
		net      string
		security string
	}{
		{"vless reality", "vless://11111111-2222-3333-4444-555555555555@de.example.com:443?encryption=none&security=reality&sni=www.example.com&fp=chrome&pbk=abc&sid=12&type=tcp&flow=xtls-rprx-vision#DE-01", "vless", "de.example.com", 443, "tcp", "reality"},
		{"vless ws", "vless://11111111-2222-3333-4444-555555555555@de.example.com:2087?type=ws&security=tls&sni=cdn.example.com&path=%2Fws&host=cdn.example.com#x", "vless", "de.example.com", 2087, "ws", "tls"},
		{"vmess", "vmess://" + vm, "vmess", "de.example.com", 443, "ws", "tls"},
		{"trojan", "trojan://pass123@ir.example.com:443?security=tls&sni=ir.example.com&type=grpc&serviceName=grpcsvc#IR", "trojan", "ir.example.com", 443, "grpc", "tls"},
	}
	for _, c := range cases {
		got, err := cfgParseLink(c.link)
		if err != nil {
			t.Fatalf("%s: %v", c.name, err)
		}
		if got.Protocol != c.proto || got.Host != c.host || got.Port != c.port || got.Net != c.net || got.Security != c.security {
			t.Fatalf("%s: got %+v", c.name, got)
		}
	}

	bad := []string{"", "http://example.com", "vless://@de.example.com:443", "vless://id@de.example.com", "vmess://not-base64", "trojan://pass@host:0"}
	for _, l := range bad {
		if _, err := cfgParseLink(l); err == nil {
			t.Fatalf("want error for %q", l)
		}
	}
}

func TestCfgGuardBlocksPrivate(t *testing.T) {
	ctx := context.Background()
	for _, h := range []string{"127.0.0.1", "10.0.0.5", "192.168.1.1", "169.254.169.254", "::1", "localhost"} {
		if err := cfgGuardHost(ctx, h); err == nil {
			t.Fatalf("%s should be blocked", h)
		}
	}
	t.Setenv("CG_CFGTEST_ALLOW_PRIVATE", "1")
	if err := cfgGuardHost(ctx, "10.0.0.5"); err != nil {
		t.Fatalf("escape hatch should allow private: %v", err)
	}
}

func TestCfgStats(t *testing.T) {
	min, med, p95, jit, loss := cfgSummarize([]float64{128, 131, 0, 129, 0})
	if min != 128 || med != 129 || p95 != 131 || loss != 0.4 {
		t.Fatalf("got min=%v med=%v p95=%v loss=%v", min, med, p95, loss)
	}
	if jit != 2.5 {
		t.Fatalf("jitter got %v", jit)
	}
	if _, med2, _, _, loss2 := cfgSummarize([]float64{0, 0}); med2 != 0 || loss2 != 1 {
		t.Fatalf("all-fail stats wrong")
	}
	if cfgVerdict(4, 5) != "ok" || cfgVerdict(2, 5) != "degraded" || cfgVerdict(0, 5) != "fail" {
		t.Fatal("verdict thresholds wrong")
	}
}

func TestCfgRedact(t *testing.T) {
	link := "vless://11111111-2222-3333-4444-555555555555@de.example.com:443?security=tls#x"
	out := cfgRedact(link)
	if strings.Contains(out, "1111") || !strings.Contains(out, "de.example.com:443") {
		t.Fatalf("redact leaked: %s", out)
	}
	if cfgRedact("garbage") != "invalid-link" {
		t.Fatal("bad redact")
	}
}

func TestCfgHistoryCap(t *testing.T) {
	st, err := store.Open(filepath.Join(t.TempDir(), "t.db"))
	if err != nil {
		t.Fatal(err)
	}
	r := &Router{Store: st}
	for i := 0; i < 60; i++ {
		r.cfgHistAdd(cfgResult{FP: "f", Host: "h", Port: 1, Verdict: "ok"})
	}
	if got := len(r.cfgHistory()); got != cfgHistCap {
		t.Fatalf("history len %d", got)
	}
}

// Some providers ship VLESS with its own post-quantum encryption; xray now
// refuses plain VLESS unless TLS or that encryption is set, so the link value
// must survive into the generated config.
func TestCfgVlessEncryption(t *testing.T) {
	raw := "vless://42a1c069-9dc9-4181-86b8-13bf28632851@dl2.example.com:2041?encryption=mlkem768x25519plus.random.0rtt.ABCDEF&security=none&type=tcp&headerType=http&path=%2F&host=www.upwork.com#FI"
	c, err := cfgParseLink(raw)
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	if c.Encryption != "mlkem768x25519plus.random.0rtt.ABCDEF" || c.HeaderType != "http" {
		t.Fatalf("link fields lost: %+v", c)
	}
	cfg := cfgBuildConfig(c, 10808)
	b, err := json.Marshal(cfg)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(b), "mlkem768x25519plus") {
		t.Fatalf("encryption dropped from config: %s", b)
	}
	if !strings.Contains(string(b), `"path":["/"]`) {
		t.Fatalf("http header path missing: %s", b)
	}
}
