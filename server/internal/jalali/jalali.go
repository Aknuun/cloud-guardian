// Package jalali provides Jalali/Gregorian conversion (no deps).
package jalali

import (
	"fmt"
	"regexp"
	"strconv"
	"strings"
	"time"
)

func div(a, b int) int {
	if a*b >= 0 {
		return a / b
	}
	return -((-a) / b)
}

func mod(a, b int) int { return a - div(a, b)*b }

var breaks = []int{-61, 9, 38, 199, 426, 686, 756, 818, 1111, 1181, 1210,
	1635, 2060, 2097, 2192, 2262, 2324, 2394, 2456, 3178}

func jalCal(jy int) (leap, gy, march int, err error) {
	bl := len(breaks)
	gy = jy + 621
	leapJ := -14
	jp := breaks[0]
	if jy < jp || jy >= breaks[bl-1] {
		return 0, 0, 0, fmt.Errorf("bad jalali year")
	}
	jm := 0
	jump := 0
	for i := 1; i < bl; i++ {
		jm = breaks[i]
		jump = jm - jp
		if jy < jm {
			break
		}
		leapJ = leapJ + div(jump, 33)*8 + div(mod(jump, 33), 4)
		jp = jm
	}
	n := jy - jp
	leapJ = leapJ + div(n, 33)*8 + div(mod(n, 33)+3, 4)
	if mod(jump, 33) == 4 && jump-n == 4 {
		leapJ++
	}
	leapG := div(gy, 4) - div((div(gy, 100)+1)*3, 4) - 150
	march = 20 + leapJ - leapG
	if jump-n < 6 {
		n = n - jump + div(jump+4, 33)*33
	}
	leap = mod(mod(n+1, 33)-1, 4)
	if leap == -1 {
		leap = 4
	}
	return leap, gy, march, nil
}

func g2d(gy, gm, gd int) int {
	return int(time.Date(gy, time.Month(gm), gd, 0, 0, 0, 0, time.UTC).Unix()/86400) + 719162 + 1
}

func d2g(jdn int) (int, int, int) {
	t := time.Unix(int64(jdn-719163)*86400, 0).UTC()
	return t.Year(), int(t.Month()), t.Day()
}

func j2d(jy, jm, jd int) int {
	_, gy, march, _ := jalCal(jy)
	return g2d(gy, 3, march) + (jm-1)*31 - div(jm, 7)*(jm-7) + jd - 1
}

func d2j(jdn int) (int, int, int) {
	gy, _, _ := d2g(jdn)
	jy := gy - 621
	jdn2 := 1 + jdn - j2d(jy, 1, 1)
	if jdn2 < 1 {
		jy--
		jdn2 = 1 + jdn - j2d(jy, 1, 1)
	}
	var jm, jd int
	if jdn2 <= 186 {
		jm = 1 + div(jdn2-1, 31)
		jd = mod(jdn2-1, 31) + 1
	} else {
		jm = 7 + div(jdn2-187, 30)
		jd = mod(jdn2-187, 30) + 1
	}
	return jy, jm, jd
}

// ToGregorian converts a Jalali date to Gregorian (with round-trip guard).
func ToGregorian(jy, jm, jd int) (int, int, int, error) {
	gy, gm, gd := d2g(j2d(jy, jm, jd))
	rj, rm, rd := d2j(j2d(jy, jm, jd))
	if rj != jy || rm != jm || rd != jd {
		return 0, 0, 0, fmt.Errorf("bad jalali day")
	}
	return gy, gm, gd, nil
}

// ToJalali converts a Gregorian date to Jalali.
func ToJalali(gy, gm, gd int) (int, int, int) { return d2j(g2d(gy, gm, gd)) }

// JNow returns today's Jalali date.
func JNow() (int, int, int) {
	t := time.Now()
	return ToJalali(t.Year(), int(t.Month()), t.Day())
}

var dtRe = regexp.MustCompile(`^(\d{4})[/\-.](\d{1,2})[/\-.](\d{1,2})(?:[\s\-T]+(\d{1,2}):(\d{2}))$`)

// ParseDatetime parses "1404/06/20 14:30" -> epoch ms.
func ParseDatetime(s string) (int64, error) {
	m := dtRe.FindStringSubmatch(strings.TrimSpace(s))
	if m == nil {
		return 0, fmt.Errorf("bad format")
	}
	atoi := func(x string) int { n, _ := strconv.Atoi(x); return n }
	jy, jm, jd, hh, mm := atoi(m[1]), atoi(m[2]), atoi(m[3]), atoi(m[4]), atoi(m[5])
	if !(jm >= 1 && jm <= 12 && jd >= 1 && jd <= 31 && hh <= 23 && mm <= 59) {
		return 0, fmt.Errorf("bad range")
	}
	gy, gm, gd, err := ToGregorian(jy, jm, jd)
	if err != nil {
		return 0, err
	}
	return time.Date(gy, time.Month(gm), gd, hh, mm, 0, 0, time.Local).UnixMilli(), nil
}

// DatetimeMS builds epoch ms from a Jalali date+time.
func DatetimeMS(jy, jm, jd, hh, mm int) (int64, error) {
	gy, gm, gd, err := ToGregorian(jy, jm, jd)
	if err != nil {
		return 0, err
	}
	return time.Date(gy, time.Month(gm), gd, hh, mm, 0, 0, time.Local).UnixMilli(), nil
}

// Fmt formats epoch ms as "1404/06/20 - 14:30".
func Fmt(ms int64) string {
	t := time.UnixMilli(ms)
	jy, jm, jd := ToJalali(t.Year(), int(t.Month()), t.Day())
	return fmt.Sprintf("%d/%02d/%02d - %02d:%02d", jy, jm, jd, t.Hour(), t.Minute())
}

var faDigits = strings.NewReplacer(
	"۰", "0", "۱", "1", "۲", "2", "۳", "3", "۴", "4",
	"۵", "5", "۶", "6", "۷", "7", "۸", "8", "۹", "9",
	"٠", "0", "١", "1", "٢", "2", "٣", "3", "٤", "4",
	"٥", "5", "٦", "6", "٧", "7", "٨", "8", "٩", "9")

// ParseFA normalizes Persian/Arabic digits to ASCII.
func ParseFA(s string) string { return faDigits.Replace(s) }

// AddRelative adds years/months/days to now and returns epoch ms.
func AddRelative(years, months, days int) int64 {
	base := time.Now()
	y := base.Year() + years
	m := int(base.Month()) + months
	y += (m - 1) / 12
	m = (m-1)%12 + 1
	if m <= 0 {
		m += 12
	}
	first := time.Date(y, time.Month(m), 1, base.Hour(), base.Minute(), 0, 0, time.Local)
	lastDay := first.AddDate(0, 1, -1).Day()
	d := base.Day()
	if d > lastDay {
		d = lastDay
	}
	return first.AddDate(0, 0, d-1+days).UnixMilli()
}

var RelUnits = map[string]string{"min": "دقیقه", "hr": "ساعت",
	"day": "روز", "wk": "هفته", "mon": "ماه", "yr": "سال"}

// AddRelUnit returns now+n unit as epoch ms (-1 for unknown unit).
func AddRelUnit(unit string, n int) int64 {
	now := time.Now().UnixMilli()
	switch unit {
	case "min":
		return now + int64(n)*60000
	case "hr":
		return now + int64(n)*3600000
	case "day":
		return now + int64(n)*86400000
	case "wk":
		return now + int64(n)*7*86400000
	case "mon":
		return AddRelative(0, n, 0)
	case "yr":
		return AddRelative(n, 0, 0)
	}
	return -1
}

var JMonths = []string{"فروردین", "اردیبهشت", "خرداد", "تیر", "مرداد",
	"شهریور", "مهر", "آبان", "آذر", "دی", "بهمن", "اسفند"}
