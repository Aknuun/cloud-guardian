package router

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"net/url"
	"sort"
	"strings"
	"testing"
	"time"
)

func makeInitData(t *testing.T, token string, uid int64, authDate int64) string {
	t.Helper()
	v := url.Values{}
	v.Set("auth_date", fmt.Sprint(authDate))
	v.Set("query_id", "AAH-test")
	v.Set("user", fmt.Sprintf(`{"id":%d,"first_name":"Test","username":"tester"}`, uid))
	keys := make([]string, 0, len(v))
	for k := range v {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	lines := make([]string, 0, len(keys))
	for _, k := range keys {
		lines = append(lines, k+"="+v.Get(k))
	}
	secret := hmac.New(sha256.New, []byte("WebAppData"))
	secret.Write([]byte(token))
	m := hmac.New(sha256.New, secret.Sum(nil))
	m.Write([]byte(strings.Join(lines, "\n")))
	v.Set("hash", hex.EncodeToString(m.Sum(nil)))
	return v.Encode()
}

func TestValidateInitData(t *testing.T) {
	token := "123456:TEST-token"
	initData := makeInitData(t, token, 42, time.Now().Unix())

	uid, err := validateInitData(initData, token, 24*time.Hour)
	if err != nil || uid != 42 {
		t.Fatalf("valid initData rejected: uid=%d err=%v", uid, err)
	}
	if _, err := validateInitData(initData, "999:other", 24*time.Hour); err == nil {
		t.Fatal("wrong bot token accepted")
	}
	stale := makeInitData(t, token, 42, time.Now().Add(-time.Hour).Unix())
	if _, err := validateInitData(stale, token, time.Minute); err == nil {
		t.Fatal("stale auth_date accepted")
	}
	tampered := strings.Replace(initData, "42", "43", 1)
	if _, err := validateInitData(tampered, token, 24*time.Hour); err == nil {
		t.Fatal("tampered initData accepted")
	}
	if _, err := validateInitData("", token, 24*time.Hour); err == nil {
		t.Fatal("empty initData accepted")
	}
}

func TestWebSessionRoundTrip(t *testing.T) {
	r := &Router{BotToken: "123:abc"}
	exp := time.Now().Add(time.Hour).Unix()
	tok := r.webSignSession(7, exp)
	if uid, ok := r.webCheckSession(tok); !ok || uid != 7 {
		t.Fatalf("session rejected: uid=%d ok=%v", uid, ok)
	}
	if _, ok := r.webCheckSession(tok + "x"); ok {
		t.Fatal("tampered session accepted")
	}
	if _, ok := r.webCheckSession(r.webSignSession(7, time.Now().Add(-time.Minute).Unix())); ok {
		t.Fatal("expired session accepted")
	}
	if _, ok := r.webCheckSession("garbage"); ok {
		t.Fatal("garbage session accepted")
	}
}

func TestWebHzTraffic(t *testing.T) {
	traffic := hzWebTraffic(map[string]any{
		"outgoing_traffic": float64(10 * 1024 * 1024 * 1024),
		"ingoing_traffic":  float64(2 * 1024 * 1024 * 1024),
		"included_traffic": float64(20 * 1024 * 1024 * 1024),
	})
	if got := traffic["total_gb"].(float64); got != 12 {
		t.Fatalf("total_gb = %v, want 12", got)
	}
	if got := traffic["used_pct"].(float64); got != 50 {
		t.Fatalf("used_pct = %v, want 50", got)
	}
	if got := traffic["extra_gb"].(float64); got != 0 {
		t.Fatalf("extra_gb = %v, want 0", got)
	}
}

func TestLnWebServerObj(t *testing.T) {
	obj := lnWebServerObj(map[string]any{
		"id": "123", "label": "web-1", "status": "running", "region": "de-fra-2",
		"type": "g6-standard-1", "image": "linode/ubuntu24.04",
		"ipv4":  []any{"1.2.3.4", "10.0.0.2"},
		"ipv6":  "2001:db8::1/128",
		"specs": map[string]any{"vcpus": float64(2), "memory": float64(4096), "disk": float64(81920)},
	})
	if obj["status"] != "running" || obj["ipv4"] != "1.2.3.4 · 10.0.0.2" {
		t.Fatalf("unexpected server object: %#v", obj)
	}
	specs := obj["specs"].(map[string]any)
	if specs["ram_mb"] != 4096 || specs["disk_gb"] != 80.0 {
		t.Fatalf("unexpected specs: %#v", specs)
	}
}

func TestLnWebTransferGB(t *testing.T) {
	if got := lnWebTransferGB(500, 2000); got != 500 {
		t.Fatalf("GB passthrough = %v", got)
	}
	if got := lnWebTransferGB(1024*1024, 2*1024*1024); got != 1024 {
		t.Fatalf("MB conversion = %v", got)
	}
	if got := lnWebTransferGB(2*1024*1024*1024, 4*1024*1024*1024); got != 2 {
		t.Fatalf("byte conversion = %v", got)
	}
}
