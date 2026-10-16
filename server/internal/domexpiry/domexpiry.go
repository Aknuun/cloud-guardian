// Package domexpiry resolves domain expiry via whois(.ir), RDAP and an
// optional WhoisXMLAPI key.
package domexpiry

import (
	"encoding/json"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/url"
	"regexp"
	"strconv"
	"strings"
	"time"
)

const (
	TehranOffsetMS = 12600000 // +03:30
	ua             = "Mozilla/5.0 (compatible; CloudGuardianBot/1.0)"
)

var AlertDays = []int{60, 30, 14, 7, 3, 1, 0}

type Item struct {
	Domain       string `json:"domain"`
	Status       string `json:"status"`
	Expiry       *int64 `json:"expiry,omitempty"`
	Manual       bool   `json:"manual,omitempty"`
	Added        int64  `json:"added,omitempty"`
	LastAlertDay *int   `json:"last_alert_day,omitempty"`
}

type Result struct {
	Status  string `json:"status"`
	Expiry  *int64 `json:"expiry,omitempty"`
	Message string `json:"message,omitempty"`
}

func NowMS() int64 { return time.Now().UnixMilli() }

// IsValidDomain reports whether d looks like a registrable domain.
func IsValidDomain(d string) bool {
	return reValid.MatchString(strings.ToLower(strings.TrimSpace(d)))
}

func whoisServerFor(domain string) string {
	if strings.HasSuffix(strings.ToLower(domain), ".ir") {
		return "whois.nic.ir"
	}
	return ""
}

func whoisQuery(server, domain string, timeout time.Duration) string {
	conn, err := net.DialTimeout("tcp", server+":43", timeout)
	if err != nil {
		return ""
	}
	defer conn.Close()
	_, _ = conn.Write([]byte(domain + "\r\n"))
	_ = conn.SetReadDeadline(time.Now().Add(timeout))
	buf := make([]byte, 0, 8192)
	tmp := make([]byte, 4096)
	for len(buf) < 60000 {
		n, err := conn.Read(tmp)
		if n > 0 {
			buf = append(buf, tmp[:n]...)
		}
		if err != nil {
			break
		}
	}
	return string(buf)
}

var (
	reErr101 = regexp.MustCompile(`%ERROR:101`)
	reNoEnt  = regexp.MustCompile(`(?i)no entries found`)
	reExpir  = regexp.MustCompile(`(?i)expir[ye][d]?[\s_-]*date\s*[:=]\s*(\d{4})-(\d{2})-(\d{2})`)
	reHidden = regexp.MustCompile(`(?i)hidden by the domain holder`)
	reAvail  = regexp.MustCompile(`(?i)available for registration`)
	reDomain = regexp.MustCompile(`(?i)domain:\s*\S+`)
	reValid  = regexp.MustCompile(`^[a-z0-9][a-z0-9.-]*\.[a-z]{2,}$`)
)

func parseIRWhois(text string) Result {
	if text == "" {
		return Result{Status: "error"}
	}
	if reErr101.MatchString(text) || reNoEnt.MatchString(text) {
		return Result{Status: "not_found"}
	}
	if m := reExpir.FindStringSubmatch(text); m != nil {
		y, _ := strconv.Atoi(m[1])
		mo, _ := strconv.Atoi(m[2])
		d, _ := strconv.Atoi(m[3])
		t := time.Date(y, time.Month(mo), d, 12, 0, 0, 0, time.UTC)
		ms := t.UnixMilli()
		return Result{Status: "ok", Expiry: &ms}
	}
	if reHidden.MatchString(text) {
		return Result{Status: "hidden"}
	}
	if reAvail.MatchString(text) {
		return Result{Status: "available"}
	}
	if reDomain.MatchString(text) {
		return Result{Status: "no_date"}
	}
	return Result{Status: "error"}
}

func rdapQuery(domain string, timeout time.Duration) map[string]any {
	u := "https://rdap.org/domain/" + url.PathEscape(domain)
	req, _ := http.NewRequest("GET", u, nil)
	req.Header.Set("accept", "application/rdap+json")
	req.Header.Set("user-agent", ua)
	hc := &http.Client{Timeout: timeout}
	resp, err := hc.Do(req)
	if err != nil {
		return nil
	}
	defer resp.Body.Close()
	b, _ := io.ReadAll(resp.Body)
	var m map[string]any
	if json.Unmarshal(b, &m) != nil {
		return nil
	}
	return m
}

func parseRDAPExpiry(data map[string]any) *int64 {
	events, ok := data["events"].([]any)
	if !ok {
		return nil
	}
	for _, e := range events {
		em, ok := e.(map[string]any)
		if !ok {
			continue
		}
		if act, _ := em["eventAction"].(string); act == "expiration" {
			if ds, _ := em["eventDate"].(string); ds != "" {
				if ms := parseAnyDate(ds); ms != 0 {
					return &ms
				}
			}
		}
	}
	return nil
}

func parseAnyDate(s string) int64 {
	s = strings.TrimSpace(strings.Replace(s, "Z", "+00:00", 1))
	layouts := []string{
		time.RFC3339, "2006-01-02T15:04:05.999999999-07:00",
		"2006-01-02 15:04:05", "2006-01-02", "2006-01-02T15:04:05Z07:00",
	}
	for _, l := range layouts {
		if t, err := time.Parse(l, s); err == nil {
			if t.Location() == time.UTC || t.Location() == nil {
				t = t.UTC()
			}
			return t.UnixMilli()
		}
	}
	return 0
}

func findExpiryInJSON(obj any) *int64 {
	type hit struct {
		ms   int64
		path string
	}
	var hits []hit
	var walk func(o any, path string)
	walk = func(o any, path string) {
		switch t := o.(type) {
		case map[string]any:
			for k, v := range t {
				kl := strings.ToLower(k)
				p := kl
				if path != "" {
					p = path + "." + kl
				}
				if s, ok := v.(string); ok && strings.Contains(kl, "expir") {
					if ms := parseAnyDate(s); ms != 0 {
						hits = append(hits, hit{ms, p})
					}
				} else {
					walk(v, p)
				}
			}
		case []any:
			for _, v := range t {
				walk(v, path)
			}
		}
	}
	walk(obj, "")
	if len(hits) == 0 {
		return nil
	}
	for _, h := range hits {
		if strings.Contains(h.path, "registrydata") {
			ms := h.ms
			return &ms
		}
	}
	ms := hits[0].ms
	return &ms
}

func whoisXMLDomainInfo(domain, key string, timeout time.Duration) Result {
	u := "https://domain-info.whoisxmlapi.com/api/v1?apiKey=" +
		url.QueryEscape(key) + "&domainName=" + url.QueryEscape(domain)
	req, _ := http.NewRequest("GET", u, nil)
	req.Header.Set("accept", "application/json")
	hc := &http.Client{Timeout: timeout}
	resp, err := hc.Do(req)
	if err != nil {
		return Result{Status: "api_error", Message: trunc(err.Error(), 220)}
	}
	defer resp.Body.Close()
	b, _ := io.ReadAll(resp.Body)
	var data map[string]any
	if json.Unmarshal(b, &data) != nil {
		return Result{Status: "api_error", Message: "bad json"}
	}
	if exp := findExpiryInJSON(data); exp != nil {
		return Result{Status: "ok", Expiry: exp}
	}
	return Result{Status: "no_date"}
}

// FetchDomainExpiry resolves a domain's expiry, optionally with an API key.
func FetchDomainExpiry(domain, key string) Result {
	d := strings.ToLower(strings.TrimSpace(domain))
	if !reValid.MatchString(d) {
		return Result{Status: "invalid"}
	}
	if ws := whoisServerFor(d); ws != "" {
		r := parseIRWhois(whoisQuery(ws, d, 12*time.Second))
		if r.Status == "ok" {
			return r
		}
		if r.Status == "hidden" || r.Status == "no_date" || r.Status == "error" {
			if key != "" {
				api := whoisXMLDomainInfo(d, key, 20*time.Second)
				if api.Status == "ok" {
					return api
				}
				if r.Status == "error" && api.Status == "api_error" {
					return api
				}
			}
		}
		return r
	}
	data := rdapQuery(d, 15*time.Second)
	if exp := parseRDAPExpiry(data); exp != nil {
		return Result{Status: "ok", Expiry: exp}
	}
	if key != "" {
		if api := whoisXMLDomainInfo(d, key, 20*time.Second); api.Status == "ok" {
			return api
		}
	}
	if data != nil {
		return Result{Status: "no_date"}
	}
	return Result{Status: "error"}
}

func Remaining(expiry *int64, now int64) string {
	if expiry == nil {
		return "؟"
	}
	if *expiry <= now {
		return "منقضی شده"
	}
	a := time.UnixMilli(now + TehranOffsetMS).UTC()
	b := time.UnixMilli(*expiry + TehranOffsetMS).UTC()
	years := b.Year() - a.Year()
	months := int(b.Month()) - int(a.Month())
	days := b.Day() - a.Day()
	if days < 0 {
		months--
		first := time.Date(b.Year(), b.Month(), 1, 0, 0, 0, 0, time.UTC)
		prevLast := first.AddDate(0, 0, -1)
		days += prevLast.Day()
	}
	if months < 0 {
		years--
		months += 12
	}
	var parts []string
	if years > 0 {
		parts = append(parts, fmt.Sprintf("%d سال", years))
	}
	if months > 0 {
		parts = append(parts, fmt.Sprintf("%d ماه", months))
	}
	if days > 0 || len(parts) == 0 {
		parts = append(parts, fmt.Sprintf("%d روز", days))
	}
	return strings.Join(parts, " و ")
}

func Emoji(it Item, now int64) string {
	if it.Manual {
		return "✍️"
	}
	switch it.Status {
	case "not_found":
		return "❔"
	case "hidden", "no_date", "available":
		return "⚪"
	}
	if it.Status != "ok" || it.Expiry == nil {
		return "❓"
	}
	days := (*it.Expiry - now) / 86400000
	if (*it.Expiry-now)%86400000 != 0 && *it.Expiry > now {
		days++
	}
	if days < 0 {
		return "⛔"
	}
	if days <= 7 {
		return "🔴"
	}
	if days <= 30 {
		return "🟠"
	}
	if days <= 90 {
		return "🟡"
	}
	return "🟢"
}

func Line(it Item, now int64) string {
	em := Emoji(it, now)
	var tail string
	switch {
	case (it.Manual || it.Status == "ok") && it.Expiry != nil:
		tail = Remaining(it.Expiry, now)
	case it.Status == "hidden":
		tail = "تاریخ مخفی (دستی ثبت کنید)"
	case it.Status == "no_date":
		tail = "تاریخ اعلام نشده"
	case it.Status == "not_found":
		tail = "در ایرنیک ثبت نیست"
	case it.Status == "available":
		tail = "آزاد / ثبت‌نشده"
	case it.Status == "api_error":
		tail = "خطای API انقضا (کلید/اعتبار)"
	default:
		tail = "خطا در استعلام"
	}
	return fmt.Sprintf("%s %s — %s", em, it.Domain, tail)
}

func trunc(s string, n int) string {
	if len(s) > n {
		return s[:n]
	}
	return s
}
