package ndmon

import (
	"html"
	"strconv"
	"strings"
	"time"

	"github.com/aknuun/cg-server/internal/pg"
)

func isoNow() string { return time.Now().UTC().Format(time.RFC3339Nano) }

func parseISO(s string) int64 {
	s = strings.TrimSpace(s)
	if s == "" {
		return 0
	}
	if t, err := time.Parse(time.RFC3339Nano, s); err == nil {
		return t.UnixMilli()
	}
	if t, err := time.Parse(time.RFC3339, strings.Replace(s, "Z", "+00:00", 1)); err == nil {
		return t.UnixMilli()
	}
	return 0
}

func parseI64(s string) int64 {
	n, _ := strconv.ParseInt(strings.TrimSpace(s), 10, 64)
	return n
}

func itoa64(n int64) string { return strconv.FormatInt(n, 10) }

func esc(s any) string { return html.EscapeString(fmtAny(s)) }

func strv(v any) string {
	switch t := v.(type) {
	case nil:
		return ""
	case string:
		return t
	case float64:
		if t == float64(int64(t)) {
			return strconv.FormatInt(int64(t), 10)
		}
		return strconv.FormatFloat(t, 'f', -1, 64)
	default:
		return fmtAny(v)
	}
}

func fmtAny(v any) string {
	switch t := v.(type) {
	case nil:
		return ""
	case string:
		return t
	case float64:
		if t == float64(int64(t)) {
			return strconv.FormatInt(int64(t), 10)
		}
		return strconv.FormatFloat(t, 'f', -1, 64)
	case int64:
		return strconv.FormatInt(t, 10)
	case int:
		return strconv.Itoa(t)
	case bool:
		return strconv.FormatBool(t)
	}
	return ""
}

func panelName(p pg.Panel) string {
	if p.Name != "" {
		return p.Name
	}
	return p.URL
}

func AddrOf(n NodeSt) string    { return strv(n.Address) }
func VersionOf(n NodeSt) string { return strv(n.Version) }
func ReasonOf(n NodeSt) string  { return strv(n.Reason) }
