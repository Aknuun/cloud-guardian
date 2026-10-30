package router

import "strings"

// orDash renders an empty value as a dash.
func orDash(s string) string {
	if strings.TrimSpace(s) == "" {
		return "—"
	}
	return s
}

// isTopScreen reports screens whose back button should land on the main menu.
func isTopScreen(d string) bool {
	switch d {
	case "", "menu", "more", "hz", "arvan", "srv", "rem", "pg", "pndef",
		"settings", "dommon", "um", "hf", "hfd", "fmail", "lb", "hub",
		"fo", "qamenu", "quota", "zones", "addrec", "pghook", "cf", "nd":
		return true
	}
	return false
}
