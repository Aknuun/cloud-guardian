package um

import (
	"strconv"
	"strings"
	"time"

	"github.com/aknuun/cg-server/internal/store"
)

func i64(s string) int64 {
	n, _ := strconv.ParseInt(s, 10, 64)
	return n
}

func i64of(v any) int64 {
	switch t := v.(type) {
	case float64:
		return int64(t)
	case int64:
		return t
	case int:
		return int64(t)
	case string:
		return i64(t)
	}
	return 0
}

func strv(v any) string {
	switch t := v.(type) {
	case string:
		return t
	case float64:
		if t == float64(int64(t)) {
			return strconv.FormatInt(int64(t), 10)
		}
		return strconv.FormatFloat(t, 'f', -1, 64)
	case nil:
		return ""
	default:
		return strings.TrimSpace(strings.Join([]string{toStringAny(t)}, ""))
	}
}

func toStringAny(v any) string {
	if s, ok := v.(string); ok {
		return s
	}
	return strings.TrimSpace(fmtAny(v))
}

func fmtAny(v any) string {
	switch t := v.(type) {
	case float64:
		return strconv.FormatFloat(t, 'f', -1, 64)
	case int64:
		return strconv.FormatInt(t, 10)
	case bool:
		return strconv.FormatBool(t)
	}
	return ""
}

func fval(v any) float64 {
	switch t := v.(type) {
	case float64:
		return t
	case int64:
		return float64(t)
	case int:
		return float64(t)
	case string:
		f, _ := strconv.ParseFloat(t, 64)
		return f
	}
	return 0
}

func join(lines []string) string { return strings.Join(lines, "\n") }

func fmtGB(g float64) string { return strconv.FormatFloat(g, 'f', 1, 64) }

func toUMap(v any) map[string]float64 {
	out := map[string]float64{}
	if m, ok := v.(map[string]any); ok {
		for k, val := range m {
			out[k] = fval(val)
		}
	}
	return out
}

func logAppend(st *store.Store, entry map[string]any) {
	var log []map[string]any
	st.GetJSON("um_alerts_log", &log)
	now := time.Now().UnixMilli()
	log = append(log, entry)
	cutoff := now - 3*24*3600000
	var keep []map[string]any
	for _, x := range log {
		if i64of(x["t"]) >= cutoff {
			keep = append(keep, x)
		}
	}
	if len(keep) > 2000 {
		keep = keep[len(keep)-2000:]
	}
	st.PutJSON("um_alerts_log", keep, 0)
}
