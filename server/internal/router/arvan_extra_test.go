package router

import (
	"strings"
	"testing"
)

func TestArvServerIPsPublic201(t *testing.T) {
	s := map[string]any{"addresses": map[string]any{
		"public201": []any{map[string]any{"addr": "194.5.207.136", "is_public": true, "type": "fixed"}},
		"private":   []any{map[string]any{"addr": "10.0.0.5", "is_public": false}},
	}}
	pub, priv := arvServerIPs(s)
	if len(pub) != 1 || pub[0] != "194.5.207.136" {
		t.Fatalf("pub=%v", pub)
	}
	if len(priv) != 1 || priv[0] != "10.0.0.5" {
		t.Fatalf("priv=%v", priv)
	}
	n := arvNormalize(map[string]any{
		"id": "x", "name": "srv", "status": "ACTIVE", "created": "2026-10-15T13:18:41Z",
		"addresses": map[string]any{"public201": []any{map[string]any{"addr": "194.5.207.136", "is_public": true}}},
		"flavor":    map[string]any{"id": "eco-2-2-0", "ram": 2048, "vcpus": 2, "disk": 30},
	})
	if n == nil || firstPubIP(n) != "194.5.207.136" {
		t.Fatalf("normalize=%v", n)
	}
	if intOrAny(n["cpu"]) != 0 {
		t.Logf("cpu parsed from vcpus: %v", n["cpu"])
	}
}

func TestArvCreditErr(t *testing.T) {
	raw := `HTTP 402: {"data":{"balance":1638693,"required":4390000},"message":"You are required to have enough credit"}`
	msg := arvCreditErr(raw)
	if msg == "" || !strings.Contains(msg, "4,390,000") || !strings.Contains(msg, "1,638,693") {
		t.Fatalf("bad credit message: %q", msg)
	}
	if arvCreditErr("HTTP 500: boom") != "" {
		t.Fatal("non-402 must not produce a credit message")
	}
}

func TestArvSanitizeName(t *testing.T) {
	cases := map[string]string{
		"my server_01": "my-server01",
		"cg-۱۲۳":       "cg",
		"  a.b  ":      "ab",
	}
	for in, want := range cases {
		if got := arvSanitizeName(in); got != want {
			t.Fatalf("%q -> %q want %q", in, got, want)
		}
	}
}
