package hostfilter

import "testing"

func TestNodeCountry(t *testing.T) {
	cases := map[string]string{
		"Tehran": "IR", "Isfahan": "IR", "Shiraz": "IR",
		"Frankfurt": "DE", "Nuremberg": "DE",
		"Amsterdam": "NL", "Meppel": "NL", "Unknown": "",
	}
	for city, want := range cases {
		got := nodeCountry(node{ID: "x", City: city})
		if got != want {
			t.Errorf("nodeCountry(%q) = %q, want %q", city, got, want)
		}
	}
	if got := nodeCountry(node{ID: "ir9.node", City: "?"}); got != "IR" {
		t.Errorf("prefix fallback = %q", got)
	}
}

func TestGpMergeCityAndCountry(t *testing.T) {
	nodes := []node{
		{"ir1.node.check-host.net", "Tehran"},
		{"ir3.node.check-host.net", "Shiraz"},
		{"de2.node.check-host.net", "Frankfurt"},
	}
	probes := []gpProbe{
		{Country: "IR", City: "Tehran", Ok: 2, Total: 2},
		{Country: "IR", City: "Tehran", Ok: 0, Total: 2},
		{Country: "DE", City: "Berlin", Ok: 3, Total: 3},
	}
	out := gpMerge(nodes, probes)
	if r := out["ir1.node.check-host.net"]; r.Ok != 2 || r.Total != 4 {
		t.Errorf("tehran aggregate = %+v", r)
	}
	// No Shiraz probe: falls back to the IR country aggregate (2/4).
	if r := out["ir3.node.check-host.net"]; r.Ok != 2 || r.Total != 4 {
		t.Errorf("shiraz country fallback = %+v", r)
	}
	// No Frankfurt probe: falls back to the DE country aggregate (3/3).
	if r := out["de2.node.check-host.net"]; r.Ok != 3 || r.Total != 3 {
		t.Errorf("frankfurt country fallback = %+v", r)
	}
}

func TestGpMergeSkipsEmpty(t *testing.T) {
	nodes := []node{{"ir1.node.check-host.net", "Tehran"}}
	out := gpMerge(nodes, []gpProbe{{Country: "IR", City: "Tehran", Ok: 0, Total: 0}})
	if len(out) != 0 {
		t.Fatalf("empty samples must be ignored, got %v", out)
	}
}
