package router

import (
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"sort"
	"strconv"
	"strings"

	"github.com/aknuun/cg-server/internal/jalali"
)

type Reminder struct {
	ID       string `json:"id"`
	Text     string `json:"text"`
	When     int64  `json:"when"`
	Created  int64  `json:"created"`
	By       int64  `json:"by"`
	Server   string `json:"server,omitempty"`
	Notified bool   `json:"notified"`
	Ack      bool   `json:"ack"`
	Note     string `json:"note,omitempty"`
}

func newID() string {
	b := make([]byte, 4)
	_, _ = rand.Read(b)
	return hex.EncodeToString(b)
}

func (r *Router) rems() []Reminder {
	var list []Reminder
	r.Store.GetJSON("reminders", &list)
	return list
}

func (r *Router) saveRems(list []Reminder) {
	r.Store.PutJSON("reminders", list, 0)
}

func (r *Router) remHome(chat, mid int64) {
	list := r.rems()
	sort.Slice(list, func(i, j int) bool { return list[i].When < list[j].When })
	var delItems []Btn
	var b strings.Builder
	b.WriteString("⏰ <b>یادآورها</b>\n\n")
	if len(list) == 0 {
		b.WriteString("— هیچ یادآوری ثبت نشده.\n")
	}
	for i, rm := range list {
		mark := "🔔"
		if rm.Notified {
			if rm.Ack {
				mark = "✅"
			} else {
				mark = "🟡"
			}
		}
		txt := rm.Text
		if len(txt) > 24 {
			txt = txt[:24] + "…"
		}
		b.WriteString(fmt.Sprintf("%d) %s <code>%s</code> — %s\n", i+1, mark, txt, jalali.Fmt(rm.When)))
		delItems = append(delItems, btn(fmt.Sprintf("🗑 %d", i+1), "remdelx:"+rm.ID))
	}
	kb := gridBTN(delItems, 4)
	kb = append(kb, row(btn("➕ یادآور جدید", "remnew"), btn("🗑 حذف", "remdel")))
	kb = append(kb, row(btn("🏠 خانه", "menu")))
	r.Show(chat, mid, b.String(), kb)
}

func (r *Router) remNew(chat, mid int64) {
	r.setWizard(chat, map[string]any{"kind": "rem_text"})
	r.Show(chat, mid, "📝 متن یادآور را بفرست:", [][]Btn{{btn("🔙 لغو", "rem")}})
}

func (r *Router) remWhen(chat, mid int64) {
	pend := r.getWizard(chat)
	if pend == nil {
		r.remHome(chat, mid)
		return
	}
	y, m, d := jalali.JNow()
	kb := [][]Btn{
		{btn("⏱ نسبی (دقیقه/ساعت/…)", "remrel"), btn("📅 تاریخ دقیق", "remabs")},
		{btn("🔔 بعداً آنلاین شدم", "remwhenback")},
		{btn("🔙 لغو", "rem")},
	}
	r.Show(chat, mid, fmt.Sprintf("🕓 چه زمانی؟ (امروز %d/%02d/%02d)", y, m, d), kb)
}

func (r *Router) remRel(chat, mid int64) {
	var kb [][]Btn
	var row []Btn
	for _, u := range []string{"min", "hr", "day", "wk", "mon", "yr"} {
		row = append(row, btn(jalali.RelUnits[u], "remrelu:"+u))
		if len(row) == 3 {
			kb = append(kb, row)
			row = nil
		}
	}
	if len(row) > 0 {
		kb = append(kb, row)
	}
	kb = append(kb, []Btn{btn("🔙 بازگشت", "remnew"), btn("🏠 خانه", "menu")})
	r.Show(chat, mid, "⏱ واحد را انتخاب کن:", kb)
}

func (r *Router) remRelU(chat, mid int64, unit string) {
	pend := r.getWizard(chat)
	if pend == nil {
		r.remHome(chat, mid)
		return
	}
	pend["unit"] = unit
	pend["kind"] = "rem_rel_num"
	r.setWizard(chat, pend)
	r.Show(chat, mid, fmt.Sprintf("🔢 چند %s بعد؟ (عدد بفرست)", jalali.RelUnits[unit]),
		[][]Btn{{btn("🔙 لغو", "rem")}})
}

func (r *Router) remAbs(chat, mid int64) {
	pend := r.getWizard(chat)
	if pend == nil {
		r.remHome(chat, mid)
		return
	}
	y, _, _ := jalali.JNow()
	var items []Btn
	for yy := y; yy <= y+1; yy++ {
		items = append(items, btnS(fmt.Sprintf("%d", yy), "remabsy:"+strconv.Itoa(yy), "plain"))
	}
	kb := gridBTN(items, 4)
	kb = append(kb, navRow("remnew"))
	r.Show(chat, mid, "📅 سال را انتخاب کن:", kb)
}

func (r *Router) remAbsY(chat, mid int64, s string) {
	pend := r.getWizard(chat)
	if pend == nil {
		r.remHome(chat, mid)
		return
	}
	pend["year"] = s
	r.setWizard(chat, pend)
	var kb [][]Btn
	var row []Btn
	for i, mn := range jalali.JMonths {
		row = append(row, btn(mn, "remabsm:"+strconv.Itoa(i+1)))
		if len(row) == 3 {
			kb = append(kb, row)
			row = nil
		}
	}
	if len(row) > 0 {
		kb = append(kb, row)
	}
	kb = append(kb, []Btn{btn("🔙 بازگشت", "remabs"), btn("🏠 خانه", "menu")})
	r.Show(chat, mid, "📅 ماه را انتخاب کن:", kb)
}

func (r *Router) remAbsM(chat, mid int64, s string) {
	pend := r.getWizard(chat)
	if pend == nil {
		r.remHome(chat, mid)
		return
	}
	pend["month"] = s
	pend["kind"] = "rem_abs_day"
	r.setWizard(chat, pend)
	y := fmt.Sprint(pend["year"])
	r.Show(chat, mid, fmt.Sprintf("📅 روز ماه را بفرست (مثلاً 20) برای %s/%s:", y, s),
		[][]Btn{{btn("✍️ دستی (تاریخ کامل)", "remabsman"), btn("🔙 لغو", "rem")}})
}

func (r *Router) remAbsMan(chat, mid int64) {
	pend := r.getWizard(chat)
	if pend == nil {
		r.remHome(chat, mid)
		return
	}
	delete(pend, "year")
	delete(pend, "month")
	pend["kind"] = "rem_abs_day"
	r.setWizard(chat, pend)
	r.Show(chat, mid, "🗓 تاریخ و ساعت شمسی را بفرست، مثل <code>1404/06/20 14:30</code>:",
		[][]Btn{{btn("🔙 لغو", "rem")}})
}

func (r *Router) remWhenBack(chat, mid int64) {
	pend := r.getWizard(chat)
	if pend == nil {
		r.remHome(chat, mid)
		return
	}
	delete(pend, "server")
	pend["backOnline"] = true
	r.finalizeRem(chat, mid, pend)
}

func (r *Router) finalizeRem(chat, mid int64, pend map[string]any) {
	text, _ := pend["text"].(string)
	text = strings.TrimSpace(text)
	if text == "" {
		r.clearWizard(chat)
		r.Show(chat, mid, "❌ متن خالی بود.", HomeKB())
		return
	}
	var when int64
	if _, ok := pend["backOnline"]; ok {
		when = 0
	} else if u, ok := pend["unit"].(string); ok {
		n, _ := pend["num"].(int)
		when = jalali.AddRelUnit(u, n)
	} else {
		y, _ := strconv.Atoi(fmt.Sprint(pend["year"]))
		m, _ := strconv.Atoi(fmt.Sprint(pend["month"]))
		d, _ := strconv.Atoi(fmt.Sprint(pend["absDay"]))
		hhmm, _ := pend["absTime"].(string)
		if y == 0 || m == 0 || d == 0 {
			ms, err := jalali.ParseDatetime(strings.TrimSpace(fmt.Sprint(pend["absDay"])))
			if err != nil {
				r.clearWizard(chat)
				r.Show(chat, mid, "❌ تاریخ نامعتبر.", HomeKB())
				return
			}
			when = ms
		} else {
			var hh, mm int
			_, _ = fmt.Sscanf(hhmm, "%d:%d", &hh, &mm)
			ms, err := jalali.DatetimeMS(y, m, d, hh, mm)
			if err != nil {
				r.clearWizard(chat)
				r.Show(chat, mid, "❌ تاریخ نامعتبر.", HomeKB())
				return
			}
			when = ms
		}
	}
	rm := Reminder{ID: newID(), Text: text, When: when, Created: nowMS(), By: r.curFrom}
	if s, ok := pend["server"].(string); ok {
		rm.Server = s
	}
	list := r.rems()
	list = append(list, rm)
	r.saveRems(list)
	r.clearWizard(chat)
	whenStr := "هر وقت آنلاین شدی"
	if rm.When > 0 {
		whenStr = jalali.Fmt(rm.When)
	}
	r.Show(chat, mid, fmt.Sprintf("✅ یادآور ثبت شد.\n📝 %s\n🕓 %s", Esc(rm.Text), whenStr),
		[][]Btn{{btn("⏰ یادآورها", "rem"), btn("🏠 خانه", "menu")}})
}

func (r *Router) remDelList(chat, mid int64) {
	r.remHome(chat, mid)
}

func (r *Router) remDelAsk(chat, mid int64, id string) {
	r.Show(chat, mid, "🗑 این یادآور حذف شود؟", [][]Btn{
		{btn("✅ بله", "remdely:"+id), btn("↩️ نه", "rem")}})
}

func (r *Router) remDelDo(chat, mid int64, id string) {
	list := r.rems()
	var next []Reminder
	for _, x := range list {
		if x.ID != id {
			next = append(next, x)
		}
	}
	r.saveRems(next)
	r.Show(chat, mid, "✅ حذف شد.", [][]Btn{{btn("⏰ یادآورها", "rem"), btn("🏠 خانه", "menu")}})
}

func (r *Router) remAck(chat, mid int64, id string) {
	list := r.rems()
	for i := range list {
		if list[i].ID == id {
			list[i].Ack = true
		}
	}
	r.saveRems(list)
	r.Bot.Answer("", "✅")
	r.Show(chat, mid, "✅ ثبت شد.", [][]Btn{{btn("⏰ یادآورها", "rem"), btn("🏠 خانه", "menu")}})
}

func (r *Router) remAgain(chat, mid int64, id string) {
	_ = r.findRem(id)
	r.Bot.Answer("", "🔁")
}

func (r *Router) remAgainX(chat, mid int64, id string) {
	rm := r.findRem(id)
	if rm == nil {
		r.remHome(chat, mid)
		return
	}
	nr := *rm
	nr.ID = newID()
	nr.When = jalali.AddRelUnit("day", 1)
	nr.Notified = false
	nr.Ack = false
	list := r.rems()
	list = append(list, nr)
	r.saveRems(list)
	r.Show(chat, mid, "🔁 یک روز بعد تکرار شد.", [][]Btn{{btn("⏰ یادآورها", "rem"), btn("🏠 خانه", "menu")}})
}

func (r *Router) findRem(id string) *Reminder {
	list := r.rems()
	for i := range list {
		if list[i].ID == id {
			return &list[i]
		}
	}
	return nil
}

// placeholders for server-scoped reminders
func (r *Router) remSrv(chat, mid int64, foreign bool) { r.remHome(chat, mid) }
func (r *Router) remPage(chat, mid int64, tok string)  { r.remHome(chat, mid) }
func (r *Router) remPick(chat, mid int64, tok string)  { r.remHome(chat, mid) }
func (r *Router) remText(chat, mid int64)              { r.remNew(chat, mid) }
func (r *Router) remSet(chat, mid int64)               { r.remRel(chat, mid) }
func (r *Router) remSetH(chat, mid int64, h string)    { r.remHome(chat, mid) }
func (r *Router) remExp(chat, mid int64)               { r.remHome(chat, mid) }
func (r *Router) remNote(chat, mid int64)              { r.remHome(chat, mid) }
