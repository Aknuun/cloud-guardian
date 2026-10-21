package hz

import (
	"bytes"
	"encoding/json"
	"io"
	"net/http"
	"strconv"
	"strings"
	"time"
)

const api = "https://api.hetzner.cloud/v1"

type Error struct{ Msg string }

func (e *Error) Error() string { return e.Msg }

type Client struct {
	token string
	hc    *http.Client
}

func New(token string) *Client {
	return &Client{token: token, hc: &http.Client{Timeout: 20 * time.Second}}
}

var faCodes = []struct{ key, fa string }{
	{"unsupported_location", "این پلن در لوکیشن انتخاب‌شده موجود نیست."},
	{"server limit", "محدودیت تعداد سرور این اکانت پر شده است."},
	{"shared core limit", "ظرفیت هسته‌های shared اکانت پر شده است."},
	{"dedicated core limit", "ظرفیت هسته‌های dedicated اکانت پر شده است."},
	{"resource_limit_exceeded", "محدودیت منابع اکانت پر شده است."},
	{"rate_limit_exceeded", "تعداد درخواست‌ها زیاد شده؛ چند لحظه بعد دوباره تلاش کن."},
	{"unauthorized", "توکن هتزنر نامعتبر است."},
	{"resource_locked", "ریسورس قفل است؛ چند لحظه بعد دوباره تلاش کن."},
	{"resource_unavailable", "این پلن/ریسورس در لوکیشن انتخاب‌شده فعلاً موجود نیست."},
	{"conflict", "ریسورس در حال تغییر است؛ چند لحظه بعد دوباره تلاش کن."},
	{"maintenance", "به‌خاطر تعمیرات موقتاً ممکن نیست؛ بعداً تلاش کن."},
	{"deprecated_api_endpoint", "این قابلیت از API هتزنر حذف شده است."},
	{"uniqueness_error", "نام انتخاب‌شده تکراری است."},
	{"invalid_input", "ورودی نامعتبر است."},
	{"not_found", "موردی پیدا نشد."},
	{"insufficient", "موجودی/پرداخت اکانت کافی نیست."},
	{"network_error", "ارتباط با هتزنر برقرار نشد."},
}

func ErrFa(payload map[string]any) string {
	err, _ := payload["error"].(map[string]any)
	code := strAny(err["code"])
	msg := strAny(err["message"])
	s := strings.ToLower(code + " " + msg)
	for _, f := range faCodes {
		if strings.Contains(s, f.key) {
			if msg != "" {
				return f.fa + "\n(" + msg + ")"
			}
			return f.fa
		}
	}
	if msg != "" {
		return msg
	}
	return "خطای نامشخص هتزنر"
}

func (c *Client) raw(method, path string, body any) map[string]any {
	var rdr io.Reader
	if body != nil {
		b, _ := json.Marshal(body)
		rdr = bytes.NewReader(b)
	}
	req, err := http.NewRequest(method, api+path, rdr)
	if err != nil {
		return map[string]any{}
	}
	req.Header.Set("Authorization", "Bearer "+c.token)
	req.Header.Set("Content-Type", "application/json")
	resp, err := c.hc.Do(req)
	if err != nil {
		return map[string]any{"error": map[string]any{
			"code": "network_error", "message": err.Error()}}
	}
	defer resp.Body.Close()
	b, _ := io.ReadAll(resp.Body)
	out := map[string]any{}
	if len(b) > 0 {
		_ = json.Unmarshal(b, &out)
	}
	return out
}

func (c *Client) Call(path, method string, body any) (map[string]any, error) {
	var last map[string]any
	for k := 0; k < 4; k++ {
		out := c.raw(method, path, body)
		err, _ := out["error"].(map[string]any)
		code := ""
		if err != nil {
			code = strAny(err["code"])
		}
		if (code == "resource_locked" || code == "rate_limit_exceeded") && k < 3 {
			time.Sleep(time.Duration(1200*(k+1)) * time.Millisecond)
			last = out
			continue
		}
		if err != nil {
			return nil, &Error{ErrFa(out)}
		}
		return out, nil
	}
	return nil, &Error{ErrFa(last)}
}

func (c *Client) All(path string) []map[string]any {
	var out []map[string]any
	page := 1
	sep := "?"
	if strings.Contains(path, "?") {
		sep = "&"
	}
	for {
		d, err := c.Call(path+sep+"per_page=50&page="+strconv.Itoa(page), "GET", nil)
		if err != nil {
			return out
		}
		var arr []map[string]any
		for _, k := range sortedKeys(d) {
			if l, ok := d[k].([]any); ok {
				arr = mapList(l)
				break
			}
		}
		out = append(out, arr...)
		meta, _ := d["meta"].(map[string]any)
		pag, _ := meta["pagination"].(map[string]any)
		if np := intOr(pag["next_page"]); np > 0 {
			page = np
			continue
		}
		if pag == nil && len(arr) >= 50 {
			page++
			continue
		}
		return out
	}
}

func sortedKeys(d map[string]any) []string {
	// deterministic-ish: prefer known keys first
	pref := []string{"servers", "server_types", "images", "locations",
		"primary_ips", "data", "actions"}
	seen := map[string]bool{}
	var out []string
	for _, k := range pref {
		if _, ok := d[k]; ok {
			out = append(out, k)
			seen[k] = true
		}
	}
	for k := range d {
		if !seen[k] {
			out = append(out, k)
		}
	}
	return out
}

func mapList(arr []any) []map[string]any {
	var out []map[string]any
	for _, x := range arr {
		if m, ok := x.(map[string]any); ok {
			out = append(out, m)
		}
	}
	return out
}

func strAny(v any) string {
	if s, ok := v.(string); ok {
		return s
	}
	if v == nil {
		return ""
	}
	b, _ := json.Marshal(v)
	return strings.Trim(string(b), `"`)
}

func intOr(v any) int {
	switch t := v.(type) {
	case float64:
		return int(t)
	case int:
		return t
	case string:
		n, _ := strconv.Atoi(t)
		return n
	}
	return 0
}

// StatusEmoji maps a server status to an emoji.
func StatusEmoji(st string) string {
	switch st {
	case "running":
		return "🟢"
	case "off":
		return "🔴"
	case "starting":
		return "🟡"
	}
	return "⚪"
}

// GB converts bytes to GB rounded to 3 decimals.
func GB(num float64) float64 {
	return round3(num / (1024 * 1024 * 1024))
}

func round3(v float64) float64 {
	return float64(int64(v*1000+0.5)) / 1000
}

// AgeDays returns whole days between an ISO time and now as a string, or "—".
func AgeDays(iso string) string {
	if iso == "" {
		return "—"
	}
	t, err := time.Parse(time.RFC3339, iso)
	if err != nil {
		t, err = time.Parse("2006-01-02T15:04:05Z", iso)
	}
	if err != nil {
		return "—"
	}
	d := int(time.Since(t).Hours() / 24)
	if d < 0 {
		d = 0
	}
	return strconv.Itoa(d)
}

var euroCache = struct {
	at int64
	v  float64
}{}

// EuroRate fetches the EUR->IRT rate (cached 6h). Returns 0 if unknown.
func EuroRate() float64 {
	now := time.Now().Unix()
	if now-euroCache.at < 6*3600 && euroCache.v > 0 {
		return euroCache.v
	}
	hc := &http.Client{Timeout: 15 * time.Second}
	req, err := http.NewRequest("GET", "https://sarfe.erfjab.com/api/prices", nil)
	if err != nil {
		return euroCache.v
	}
	resp, err := hc.Do(req)
	if err != nil {
		return euroCache.v
	}
	defer resp.Body.Close()
	var d map[string]any
	if json.NewDecoder(resp.Body).Decode(&d) == nil {
		if rate := floatVal(d["eur1"]); rate > 0 {
			euroCache.at = now
			euroCache.v = rate
			return rate
		}
	}
	return euroCache.v
}

// PriceFmt formats a EUR price, optionally with the Toman equivalent.
func PriceFmt(eur any, euro float64) string {
	f, ok := nf(eur)
	if !ok {
		return "—"
	}
	var s string
	if f < 1 {
		s = strconv.FormatFloat(f, 'f', 4, 64) + "€"
	} else {
		s = strconv.FormatFloat(f, 'f', 2, 64) + "€"
	}
	if euro > 0 {
		s += " [" + comma(int64(f*euro+0.5)) + "T]"
	}
	return s
}

func comma(n int64) string {
	s := strconv.FormatInt(n, 10)
	neg := strings.HasPrefix(s, "-")
	if neg {
		s = s[1:]
	}
	var out []byte
	for i := 0; i < len(s); i++ {
		if i > 0 && (len(s)-i)%3 == 0 {
			out = append(out, ',')
		}
		out = append(out, s[i])
	}
	if neg {
		return "-" + string(out)
	}
	return string(out)
}

// SnapCount counts snapshots created from a server id.
func SnapCount(images []map[string]any, serverID string) any {
	sid, err := strconv.Atoi(serverID)
	if err != nil {
		return 0
	}
	n := 0
	for _, im := range images {
		if strAny(im["type"]) != "snapshot" {
			continue
		}
		if cf, ok := im["created_from"].(map[string]any); ok {
			if intOr(cf["id"]) == sid {
				n++
			}
		}
	}
	return n
}

// PrimaryIPID extracts the id from a public_net ipv4/ipv6 slot.
func PrimaryIPID(v any) any {
	if v == nil {
		return nil
	}
	if m, ok := v.(map[string]any); ok {
		return m["id"]
	}
	return v
}

// EnsureOff powers off the server and waits until unlocked+off (max 30s).
func EnsureOff(c *Client, serverID string, maxMs int) bool {
	s0, _ := c.Call("/servers/"+serverID, "GET", nil)
	if sv, _ := s0["server"].(map[string]any); strAny(sv["status"]) != "off" {
		_, _ = c.Call("/servers/"+serverID+"/actions/poweroff", "POST", map[string]any{})
	}
	start := time.Now()
	for time.Since(start).Milliseconds() < int64(maxMs) {
		s, _ := c.Call("/servers/"+serverID, "GET", nil)
		sv, _ := s["server"].(map[string]any)
		if !boolVal(sv["locked"]) && strAny(sv["status"]) == "off" {
			return true
		}
		time.Sleep(1500 * time.Millisecond)
	}
	return false
}

func boolVal(v any) bool {
	switch t := v.(type) {
	case bool:
		return t
	case float64:
		return t != 0
	}
	return false
}

func floatVal(v any) float64 {
	f, _ := nf(v)
	return f
}

func nf(v any) (float64, bool) {
	switch t := v.(type) {
	case float64:
		return t, true
	case int:
		return float64(t), true
	case string:
		f, err := strconv.ParseFloat(t, 64)
		return f, err == nil
	}
	return 0, false
}
