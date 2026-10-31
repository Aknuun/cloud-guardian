package router

import "testing"

func TestFoPublicBasePrefersHTTPS(t *testing.T) {
	r := &Router{WebAppURL: "https://cg.example.com/app/", FO: map[string]string{"public_url": "http://1.2.3.4:8789"}}
	if got := r.foPublicBase(); got != "https://cg.example.com" {
		t.Fatalf("got %q, want https origin", got)
	}
	r2 := &Router{WebAppURL: "", FO: map[string]string{"public_url": "https://hermes.example.com/"}}
	if got := r2.foPublicBase(); got != "https://hermes.example.com" {
		t.Fatalf("got %q, want https public_url", got)
	}
	r3 := &Router{WebAppURL: "", FO: map[string]string{"public_url": "http://1.2.3.4:8789"}}
	if got := r3.foPublicBase(); got != "http://1.2.3.4:8789" {
		t.Fatalf("got %q, want raw fallback", got)
	}
}
