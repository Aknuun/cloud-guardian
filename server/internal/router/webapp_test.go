package router

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"net/url"
	"path/filepath"
	"sort"
	"strings"
	"testing"
	"time"

	"github.com/aknuun/cg-server/internal/store"
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
	st, err := store.Open(filepath.Join(t.TempDir(), "t.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer st.Close()
	r := &Router{BotToken: "123:abc", Store: st}
	exp := time.Now().Add(time.Hour).Unix()
	sid := r.webSessCreate(7, "127.0.0.1", "Mozilla/5.0 (iPhone) Safari", exp)
	tok := r.webSignSession(7, exp, sid)
	uid, got, ok := r.webCheckSession(tok)
	if !ok || uid != 7 || got != sid {
		t.Fatalf("session rejected: uid=%d sid=%q ok=%v", uid, got, ok)
	}
	if _, _, ok := r.webCheckSession(tok + "x"); ok {
		t.Fatal("tampered session accepted")
	}
	expired := r.webSignSession(7, time.Now().Add(-time.Minute).Unix(), "x")
	if _, _, ok := r.webCheckSession(expired); ok {
		t.Fatal("expired session accepted")
	}
	if _, _, ok := r.webCheckSession("garbage"); ok {
		t.Fatal("garbage session accepted")
	}
	if lst := r.webSessList(7); len(lst) != 1 || lst[0].Device != "iPhone · Safari" {
		t.Fatalf("session list wrong: %+v", lst)
	}
	// revoking the row kills the still-valid token
	r.webSessRevoke(7, sid)
	if _, _, ok := r.webCheckSession(tok); ok {
		t.Fatal("revoked session still accepted")
	}
	// revoke-all clears every device
	a := r.webSessCreate(7, "1.1.1.1", "Windows Chrome", exp)
	b := r.webSessCreate(7, "2.2.2.2", "Android Chrome", exp)
	if len(r.webSessList(7)) != 2 {
		t.Fatal("expected two sessions")
	}
	r.webSessRevokeAll(7)
	if len(r.webSessList(7)) != 0 {
		t.Fatal("revoke-all left sessions behind")
	}
	if _, _, ok := r.webCheckSession(r.webSignSession(7, exp, a)); ok {
		t.Fatal("revoked-all token accepted")
	}
	_ = b
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
