package domexpiry

import "testing"

func TestEmojiBuckets(t *testing.T) {
	now := int64(1_700_000_000_000)
	day := int64(86_400_000)
	plus := func(d int64) *int64 { v := now + d*day; return &v }
	cases := []struct {
		it   Item
		want string
	}{
		{Item{Status: "ok", Expiry: plus(3)}, "🔴"},
		{Item{Status: "ok", Expiry: plus(20)}, "🟠"},
		{Item{Status: "ok", Expiry: plus(60)}, "🟡"},
		{Item{Status: "ok", Expiry: plus(200)}, "🟢"},
		{Item{Status: "ok", Expiry: plus(-2)}, "⛔"},
		{Item{Status: "hidden"}, "⚪"},
		{Item{Status: "not_found"}, "❔"},
		{Item{Status: "error"}, "❓"},
		{Item{Status: "ok", Expiry: plus(200), Manual: true}, "✍️"},
	}
	for _, c := range cases {
		if got := Emoji(c.it, now); got != c.want {
			t.Errorf("Emoji(%+v)=%s want %s", c.it, got, c.want)
		}
	}
}

func TestRemaining(t *testing.T) {
	now := NowMS()
	future := now + 30*86400000
	if Remaining(&future, now) == "؟" || Remaining(&future, now) == "منقضی شده" {
		t.Fatal("bad remaining")
	}
	if Remaining(nil, now) != "؟" {
		t.Fatal("nil expiry")
	}
	if !IsValidDomain("example.ir") || IsValidDomain("not a domain") {
		t.Fatal("domain validation")
	}
}

func TestLine(t *testing.T) {
	it := Item{Domain: "example.com", Status: "available"}
	if got := Line(it, NowMS()); got == "" || got[0] == ' ' {
		t.Fatalf("line=%q", got)
	}
}
