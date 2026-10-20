package router

import "testing"

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
