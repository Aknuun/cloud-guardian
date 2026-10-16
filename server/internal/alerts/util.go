package alerts

import (
	"fmt"
	"math"
	"strconv"
	"strings"
)

func toString(v any) string {
	if v == nil {
		return ""
	}
	switch t := v.(type) {
	case string:
		return t
	case float64:
		if t == math.Trunc(t) {
			return strconv.FormatInt(int64(t), 10)
		}
		return strconv.FormatFloat(t, 'f', -1, 64)
	case bool:
		if t {
			return "true"
		}
		return "false"
	default:
		return fmt.Sprint(t)
	}
}

func first(vals ...any) any {
	for _, v := range vals {
		if v != nil {
			if s, ok := v.(string); ok && s == "" {
				continue
			}
			return v
		}
	}
	return nil
}

func toI64(v any) (int64, bool) {
	switch t := v.(type) {
	case float64:
		return int64(t), true
	case int:
		return int64(t), true
	case int64:
		return t, true
	case string:
		n, err := strconv.ParseInt(t, 10, 64)
		if err != nil {
			f, err2 := strconv.ParseFloat(t, 64)
			if err2 != nil {
				return 0, false
			}
			return int64(f), true
		}
		return n, true
	}
	return 0, false
}

func lastIndexByte(s string, b byte) int { return strings.LastIndexByte(s, b) }

func parseFloat(s string) (float64, bool) {
	f, err := strconv.ParseFloat(s, 64)
	return f, err == nil
}

func numStr(f float64) string {
	if f == math.Trunc(f) && math.Abs(f) < 1e15 {
		return strconv.FormatInt(int64(f), 10)
	}
	return strconv.FormatFloat(f, 'f', -1, 64)
}

func trimSpace(s string) string      { return strings.TrimSpace(s) }
func lower(s string) string          { return strings.ToLower(s) }
func trimRight(s, cut string) string { return strings.TrimRight(s, cut) }

func joinLines(lines []string) string { return strings.Join(lines, "\n") }

func trunc(s string, n int) string {
	if len(s) > n {
		return s[:n]
	}
	return s
}

func round1(f float64) float64 { return math.Round(f*10) / 10 }

func maxInt(a, b int) int {
	if a > b {
		return a
	}
	return b
}

func itoa(n int) string { return strconv.Itoa(n) }
