package router

import "strings"

// orDash renders an empty value as a dash.
func orDash(s string) string {
	if strings.TrimSpace(s) == "" {
		return "—"
	}
	return s
}
