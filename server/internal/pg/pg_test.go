package pg

import "testing"

func TestGBOf(t *testing.T) {
	if got := GBOf(float64(2 * GB)); got != 2 {
		t.Fatalf("GBOf=%v", got)
	}
}

func TestPct(t *testing.T) {
	if p := Pct(50, 100); p == nil || *p != 50 {
		t.Fatal("pct 50")
	}
	if p := Pct(50, 0); p != nil {
		t.Fatal("pct with zero limit should be nil")
	}
}

func TestDaysLeft(t *testing.T) {
	// expire ~10 days from now (seconds)
	exp := float64(0)
	_ = exp
	if d := DaysLeft(float64(0)); d != nil {
		t.Fatal("zero should be nil")
	}
	if d := DaysLeft(""); d != nil {
		t.Fatal("empty should be nil")
	}
	if d := DaysLeft("2020-01-01T00:00:00Z"); d == nil || *d >= 0 {
		t.Fatal("past date should be negative")
	}
}
