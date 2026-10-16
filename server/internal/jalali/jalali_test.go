package jalali

import "testing"

func TestRoundTrip(t *testing.T) {
	gy, gm, gd, err := ToGregorian(1404, 6, 20)
	if err != nil {
		t.Fatal(err)
	}
	jy, jm, jd := ToJalali(gy, gm, gd)
	if jy != 1404 || jm != 6 || jd != 20 {
		t.Fatalf("roundtrip got %d/%d/%d", jy, jm, jd)
	}
}

func TestParseDatetime(t *testing.T) {
	ms, err := ParseDatetime("1404/06/20 14:30")
	if err != nil {
		t.Fatal(err)
	}
	if got := Fmt(ms); got != "1404/06/20 - 14:30" {
		t.Fatalf("fmt got %q", got)
	}
}

func TestParseFA(t *testing.T) {
	if ParseFA("۱۴۰۴/۰۶/۲۰") != "1404/06/20" {
		t.Fatal("fa digits")
	}
}

func TestRelUnits(t *testing.T) {
	if AddRelUnit("day", 1) <= 0 {
		t.Fatal("rel day")
	}
	if AddRelUnit("nope", 1) != -1 {
		t.Fatal("unknown unit")
	}
}
