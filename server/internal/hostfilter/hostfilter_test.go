package hostfilter

import "testing"

func TestBlockedCitiesIgnoresEmptySamples(t *testing.T) {
	ping := map[string]pingRes{
		"n1": {City: "tehran", Ok: 0, Total: 0},
		"n2": {City: "tehran", Ok: 0, Total: 0},
	}
	cities := []string{"tehran"}
	if got := blockedCities(ping, cities, 0); got != 0 {
		t.Fatalf("nodes without samples must not count as blocked, got %d", got)
	}
}

func TestBlockedCitiesCountsRealFailures(t *testing.T) {
	ping := map[string]pingRes{
		"n1": {City: "tehran", Ok: 0, Total: 4},
		"n2": {City: "tehran", Ok: 0, Total: 4},
		"n3": {City: "shiraz", Ok: 4, Total: 4},
	}
	cities := []string{"tehran", "shiraz"}
	if got := blockedCities(ping, cities, 0); got != 1 {
		t.Fatalf("blocked cities = %d, want 1", got)
	}
}
