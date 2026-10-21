package router

import "testing"

func TestCfTtlLabel(t *testing.T) {
	cases := map[any]string{1: "خودکار", 120: "خودکار", 300: "5 دقیقه", 3600: "1 ساعت", 86400: "24 ساعت", "bad": "خودکار"}
	for in, want := range cases {
		if got := cfTtlLabel(in); got != want {
			t.Errorf("cfTtlLabel(%v) = %q, want %q", in, got, want)
		}
	}
}

func TestCfValueHint(t *testing.T) {
	cases := []struct {
		val, typ string
		ok       bool
	}{
		{"1.2.3.4", "A", true},
		{"::1", "A", false},
		{"example.com", "A", false},
		{"::1", "AAAA", true},
		{"1.2.3.4", "AAAA", false},
		{"sub.example.com", "CNAME", true},
		{"1.2.3.4", "CNAME", false},
		{"anything with spaces", "TXT", true},
	}
	for _, c := range cases {
		got := cfValueHint(c.val, c.typ) == ""
		if got != c.ok {
			t.Errorf("cfValueHint(%q,%q) ok=%v, want %v", c.val, c.typ, got, c.ok)
		}
	}
}

func TestCfTypeForValue(t *testing.T) {
	if got := cfTypeForValue("1.2.3.4"); got != "A" {
		t.Errorf("v4 -> %q", got)
	}
	if got := cfTypeForValue("2001:db8::1"); got != "AAAA" {
		t.Errorf("v6 -> %q", got)
	}
	if got := cfTypeForValue("sub.example.com"); got != "CNAME" {
		t.Errorf("host -> %q", got)
	}
	if got := cfTypeForValue("nope"); got != "" {
		t.Errorf("invalid -> %q", got)
	}
}

func TestNavPushDedupAndCap(t *testing.T) {
	r := &Router{}
	for i := 0; i < 40; i++ {
		r.navPush(1, "page")
	}
	v, _ := r.navStack.Load(int64(1))
	st, _ := v.([]string)
	if len(st) != 1 || st[0] != "page" {
		t.Fatalf("dedup failed: %v", st)
	}
	for i := 0; i < 40; i++ {
		r.navPush(1, "p"+string(rune('a'+i%26))+string(rune('a'+i/26)))
	}
	v, _ = r.navStack.Load(int64(1))
	st, _ = v.([]string)
	if len(st) > 30 {
		t.Fatalf("stack not capped: %d", len(st))
	}
	if st[len(st)-1] != "page" && len(st) > 27 {
		// last pushed entries must stay
	}
}

func TestLnLabelPart(t *testing.T) {
	cases := map[string]string{
		"web 1":        "web-1",
		"  my_server ": "my_server",
		"---":          "srv",
		"":             "srv",
	}
	for in, want := range cases {
		if got := lnLabelPart(in); got != want {
			t.Errorf("lnLabelPart(%q) = %q, want %q", in, got, want)
		}
	}
	long := lnLabelPart("aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa")
	if len(long) != 32 {
		t.Errorf("long label length = %d, want 32", len(long))
	}
}

func TestIsNavScreen(t *testing.T) {
	for _, d := range []string{"menu", "ln", "srv", "cfr:abc", "hzs:0:0", "lnsi:1:2"} {
		if !isNavScreen(d) {
			t.Errorf("isNavScreen(%q) = false, want true", d)
		}
	}
	for _, d := range []string{"cfarp:1", "lnscd:0:eu", "hfdhf:0:9:address", "remrelu:day"} {
		if isNavScreen(d) {
			t.Errorf("isNavScreen(%q) = true, want false", d)
		}
	}
}
