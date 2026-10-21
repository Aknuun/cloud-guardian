package router

import (
	"fmt"
	"strings"
)

// ============================================================================
// UI standard (single source of truth for every screen).
//
// Every page MUST follow the same shape so the bot feels consistent:
//
//	<emoji> <b>عنوان</b>            <- line 1: section title
//	<ctx emoji> متن کوتاه          <- optional context (account/zone/server)
//	                               <- blank line
//	• ردیف اطلاعات                 <- body lines (bullets, stats)
//
//	[ لیست دوستونه خاکستری ]        <- listKB(items): 2 cols, always "plain"
//	[ عملیات سبز/آبی ]             <- actions: btnS(..., "success"/"primary")
//	[ 🔙 بازگشت ] [ 🏠 خانه ]       <- footer: navRow(back) / navKB(back)
//
// Rules:
//   - List items are never styled (plain/gray) -> listKB().
//   - Action buttons carry the colour: create/add/refresh = "success",
//     secondary/neutral = "primary", destructive = "danger".
//   - Loading states use r.loading(); errors use r.uiErr(); empty states use
//     uiEmpty(). Confirmations end with a danger/ok button + navRow(back).
// ============================================================================

// uiTitle builds the standard header: "🖥 <b>سرورها</b>" or with context.
func uiTitle(emoji, name, ctx string) string {
	t := strings.TrimSpace(emoji + " <b>" + name + "</b>")
	if strings.TrimSpace(ctx) != "" {
		t += " — " + Esc(ctx)
	}
	return t
}

// uiBody joins header + body lines with the standard blank line separator.
func uiBody(header string, lines ...string) string {
	var b strings.Builder
	b.WriteString(header)
	for _, l := range lines {
		b.WriteString("\n")
		b.WriteString(l)
	}
	return b.String()
}

// listKB lays out list items in two gray columns (the standard list layout).
func listKB(items []Btn) [][]Btn { return gridBTN(items, 2) }

// listKB1 lays out long list items one per row (still gray).
func listKB1(items []Btn) [][]Btn { return gridBTN(items, 1) }

// stdKB appends the standard back/home footer to the given rows.
func stdKB(back string, rows ...[]Btn) [][]Btn {
	var kb [][]Btn
	for _, r := range rows {
		if len(r) > 0 {
			kb = append(kb, r)
		}
	}
	return append(kb, navRow(back))
}

// uiEmpty renders the standard "nothing here" state.
func uiEmpty(header, what string) string {
	return uiBody(header, "", "📭 "+what+" نیست.")
}

// loading shows the standard spinner text (no keyboard) before slow calls.
func (r *Router) loading(chat, mid int64, what string) {
	r.Show(chat, mid, "⏳ "+what+"…", nil)
}

// uiErr renders the standard error page: short message + back/home footer.
func (r *Router) uiErr(chat, mid int64, back, msg string) {
	r.Show(chat, mid, "❌ "+Esc(msg), stdKB(back))
}

// uiOK renders the standard success page (short message) with a footer.
func (r *Router) uiOK(chat, mid int64, back, msg string) {
	r.Show(chat, mid, "✅ "+Esc(msg), stdKB(back))
}

// statRow renders a compact "label: value" pair line.
func statLine(label, value string) string {
	return fmt.Sprintf("%s: <b>%s</b>", Esc(label), value)
}
