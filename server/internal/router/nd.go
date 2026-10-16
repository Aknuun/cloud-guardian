package router

import (
	"fmt"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/aknuun/cg-server/internal/jalali"
	"github.com/aknuun/cg-server/internal/ndmon"
)

func nowJalali() string { return jalali.Fmt(time.Now().UnixMilli()) }

func nowJalaliDigits(s string) string { return jalali.ParseFA(s) }

func nowSec() int64 { return time.Now().Unix() }

func (r *Router) ndGet(idStr string) *ndmon.Monitor {
	for _, m := range ndmon.Monitors(r.Store) {
		if m.ID == idStr {
			return &m
		}
	}
	return nil
}

func (r *Router) ndEnsureToken(m *ndmon.Monitor) string {
	if m.HookToken != "" {
		return m.HookToken
	}
	tok := randHex(8)
	m.HookToken = tok
	lst := ndmon.Monitors(r.Store)
	for i := range lst {
		if lst[i].ID == m.ID {
			lst[i].HookToken = tok
		}
	}
	ndmon.SaveMonitors(r.Store, lst)
	return tok
}

func (r *Router) ndHome(chat, mid int64) {
	mons := ndmon.Monitors(r.Store)
	lines := []string{"📡 مانیتور نود پاسارگارد", "",
		"هر پنل پاسارگاردی را ثبت کن و برایش «مانیتور نود» بساز؛ " +
			"سپس همین‌جا وضعیت همهٔ نودها را مستقیم می‌بینی. " +
			"اگر نودی قطع یا دوباره وصل شود، فوری خبر داده می‌شود.",
		"پایش خودکار با فاصلهٔ قابل تنظیم انجام می‌شود.", ""}
	var kb [][]Btn
	for _, m := range mons {
		st := "🟢"
		if !m.IsEnabled() {
			st = "⏸"
		}
		kb = append(kb, []Btn{btn(fmt.Sprintf("%s %s", st, m.Label()), "ndd:"+m.ID)})
	}
	kb = append(kb, []Btn{btn("➕ ساخت مانیتور", "ndmk")})
	if len(mons) > 0 {
		kb = append(kb, []Btn{btn("🗑 حذف مانیتور", "nddelmenu")})
	}
	kb = append(kb, []Btn{btn("❓ راهنما", "ndhelp"), btn("🏠 خانه", "menu")})
	r.Show(chat, mid, strings.Join(lines, "\n"), kb)
}

func (r *Router) ndDetail(chat, mid int64, idStr string) {
	m := r.ndGet(idStr)
	if m == nil {
		r.Show(chat, mid, "❌ مانیتور پیدا نشد.", [][]Btn{{btn("📡", "nd")}})
		return
	}
	nodes := ndmon.Nodes(r.Store, m.ID)
	up := 0
	for _, n := range nodes {
		if ndmon.NodeDir(n.Status) == "up" {
			up++
		}
	}
	status := "🟢 فعال"
	if !m.IsEnabled() {
		status = "⏸ توقف"
	}
	interval := m.IntervalMin
	if interval == 0 {
		interval = 10
	}
	lines := []string{
		fmt.Sprintf("📡 مانیتور «%s»", m.Label()), "",
		"وضعیت: " + status,
		fmt.Sprintf("⏱ فاصله پایش: %d دقیقه", interval),
		fmt.Sprintf("🧮 نودها: %d (سالم: %d)", len(nodes), up),
	}
	toggleLabel := "⏸ توقف"
	if !m.IsEnabled() {
		toggleLabel = "▶️ فعال"
	}
	kb := [][]Btn{
		{btn("🔄 همگام‌سازی", "ndsync:"+m.ID), btn("⏱ فاصله", "ndint:"+m.ID)},
		{btn("🚫 استثناها", "ndexc:"+m.ID), btn("🔗 وب‌هوک", "ndhook:"+m.ID)},
		{btn("🧪 تست هشدار", "ndtest:"+m.ID), btn(toggleLabel, "ndtg:"+m.ID)},
		{btn("🔙 بازگشت", "nd"), btn("🏠 خانه", "menu")},
	}
	names := make([]string, 0, len(nodes))
	for name := range nodes {
		names = append(names, name)
	}
	sort.Strings(names)
	var nodeKB [][]Btn
	for i, name := range names {
		if i >= 10 {
			break
		}
		icon := "🔴"
		if ndmon.NodeDir(nodes[name].Status) == "up" {
			icon = "🟢"
		}
		nodeKB = append(nodeKB, []Btn{btn(fmt.Sprintf("%s %s", icon, name), "ndi:"+m.ID+":"+name)})
	}
	kb = append(nodeKB, kb...)
	r.Show(chat, mid, strings.Join(lines, "\n"), kb)
}

func (r *Router) ndSync(chat, mid int64, idStr string) {
	m := r.ndGet(idStr)
	if m == nil {
		r.Show(chat, mid, "❌ مانیتور پیدا نشد.", [][]Btn{{btn("📡", "nd")}})
		return
	}
	r.Show(chat, mid, "⏳ در حال همگام‌سازی نودها…", [][]Btn{{btn("📡", "nd")}})
	ndmon.RunNodePoll(r.Bot, r.Store, r.panels(), r.AdminID, true)
	r.ndDetail(chat, 0, idStr)
}

func (r *Router) ndTest(chat, mid int64, idStr string) {
	m := r.ndGet(idStr)
	if m == nil {
		r.Show(chat, mid, "❌ مانیتور پیدا نشد.", [][]Btn{{btn("📡", "nd")}})
		return
	}
	r.Bot.Send(chat, "🧪 پیام آزمایشی هشدار نود:\n\n"+
		"🚨 نود قطع شد!\n"+
		"🖥 پنل: <code>"+Esc(m.Label())+"</code>\n"+
		"🖧 نود: <code>node-test</code>\n"+
		"🌐 آی‌پی: <code>1.2.3.4</code>\n"+
		"⏱ زمان: "+nowJalali()+" به وقت ایران\n"+
		"🔎 علت: <code>تست</code>", nil)
	r.ndDetail(chat, 0, idStr)
}

func (r *Router) ndHook(chat, mid int64, idStr string) {
	m := r.ndGet(idStr)
	if m == nil {
		r.Show(chat, mid, "❌ مانیتور پیدا نشد.", [][]Btn{{btn("📡", "nd")}})
		return
	}
	tok := r.ndEnsureToken(m)
	base := r.publicBase()
	url := "/ndhook/" + tok
	if base != "" {
		url = base + "/ndhook/" + tok
	}
	body := `{"event":"down","node":"نام نود","reason":"علت"}`
	r.Show(chat, mid,
		"🔗 وب‌هوک مانیتور نود\n\n"+
			"این آدرس را روی سرور پنل تنظیم کن؛ هر تغییر وضعیت نود را به آن POST کن:\n\n"+
			"آدرس: <code>"+Esc(url)+"</code>\n\n"+
			"بدنه: <code>"+Esc(body)+"</code>",
		[][]Btn{{btn("🔙 بازگشت", "ndd:"+m.ID), btn("🏠 خانه", "menu")}})
}

func (r *Router) ndExc(chat, mid int64, idStr string) {
	m := r.ndGet(idStr)
	if m == nil {
		r.Show(chat, mid, "❌ مانیتور پیدا نشد.", [][]Btn{{btn("📡", "nd")}})
		return
	}
	nodes := ndmon.Nodes(r.Store, m.ID)
	if len(nodes) == 0 {
		r.Show(chat, mid, "📭 نودی ثبت نشده (ابتدا همگام‌سازی کن).",
			[][]Btn{{btn("🔄 همگام‌سازی", "ndsync:"+m.ID), btn("🔙 بازگشت", "ndd:"+m.ID)}})
		return
	}
	exc := map[string]bool{}
	for _, e := range m.Excluded {
		exc[e] = true
	}
	names := make([]string, 0, len(nodes))
	for name := range nodes {
		names = append(names, name)
	}
	sort.Strings(names)
	var kb [][]Btn
	for _, name := range names {
		mark := "🔔"
		if exc[name] {
			mark = "🚫"
		}
		kb = append(kb, []Btn{btn(mark+" "+name, "ndx:"+m.ID+":"+name)})
	}
	kb = append(kb, []Btn{btn("🔙 بازگشت", "ndd:"+m.ID), btn("🏠 خانه", "menu")})
	r.Show(chat, mid, "🚫 برای خاموش‌کردن هشدار هر نود بزن:", kb)
}

func (r *Router) ndToggleExc(chat, mid int64, idStr, name string) {
	lst := ndmon.Monitors(r.Store)
	for i := range lst {
		if lst[i].ID == idStr {
			s := map[string]bool{}
			for _, e := range lst[i].Excluded {
				s[e] = true
			}
			if s[name] {
				delete(s, name)
			} else {
				s[name] = true
			}
			out := make([]string, 0, len(s))
			for k := range s {
				out = append(out, k)
			}
			sort.Strings(out)
			lst[i].Excluded = out
		}
	}
	ndmon.SaveMonitors(r.Store, lst)
	r.ndExc(chat, mid, idStr)
}

func (r *Router) ndNode(chat, mid int64, idStr, name string) {
	m := r.ndGet(idStr)
	if m == nil {
		r.Show(chat, mid, "❌ مانیتور پیدا نشد.", [][]Btn{{btn("📡", "nd")}})
		return
	}
	nodes := ndmon.Nodes(r.Store, m.ID)
	nd, ok := nodes[name]
	if !ok {
		r.Show(chat, mid, "❌ نود پیدا نشد.", [][]Btn{{btn("🔙 بازگشت", "ndd:"+m.ID)}})
		return
	}
	healthy := ndmon.NodeDir(nd.Status) == "up"
	lines := []string{fmt.Sprintf("🖧 نود «%s»", Esc(name))}
	if healthy {
		lines = append(lines, "🟢 سالم")
	} else {
		lines = append(lines, "🔴 قطع")
	}
	if a := ndmon.AddrOf(nd); a != "" {
		lines = append(lines, "🌐 <code>"+Esc(a)+"</code>")
	}
	if v := ndmon.VersionOf(nd); v != "" {
		lines = append(lines, "🧩 نسخه: <code>"+Esc(v)+"</code>")
	}
	if reason := ndmon.ReasonOf(nd); reason != "" {
		lines = append(lines, "🔎 علت: "+Esc(reason))
	}
	exc := false
	for _, e := range m.Excluded {
		if e == name {
			exc = true
		}
	}
	if exc {
		lines = append(lines, "🚫 این نود استثنا شده.")
	}
	excLabel := "🚫 استثنا"
	if exc {
		excLabel = "✅ استثنا"
	}
	r.Show(chat, mid, strings.Join(lines, "\n"),
		[][]Btn{
			{btn("📡 آی‌پی", "ndip:"+m.ID+":"+name), btn(excLabel, "ndx:"+m.ID+":"+name)},
			{btn("🔙 بازگشت", "ndd:"+m.ID), btn("🏠 خانه", "menu")}})
}

func (r *Router) ndNodeIP(chat, mid int64, idStr, name string) {
	var addr string
	if m := r.ndGet(idStr); m != nil {
		if nd, ok := ndmon.Nodes(r.Store, m.ID)[name]; ok {
			addr = ndmon.AddrOf(nd)
		}
	}
	if addr == "" {
		addr = "—"
	}
	r.Show(chat, mid, fmt.Sprintf("🌐 آی‌پی نود «%s»:\n<code>%s</code>", Esc(name), Esc(addr)),
		[][]Btn{{btn("🔙 بازگشت", "ndd:"+idStr), btn("🏠 خانه", "menu")}})
}

func (r *Router) ndHelp(chat, mid int64) {
	r.Show(chat, mid,
		"ℹ️ راهنمای مانیتور نود\n\n"+
			"وضعیت نودهای پنل‌ها را نشان می‌دهد و با قطع/وصل شدن هر نود به ادمین‌ها خبر می‌دهد.\n\n"+
			"• پایش خودکار با فاصله قابل تنظیم (⏱ فاصله).\n"+
			"• وب‌هوک برای هشدار لحظه‌ای (🔗 وب‌هوک).\n"+
			"• استثنا برای خاموش‌کردن هشدار نودهای خاص (🚫 استثنا).",
		[][]Btn{{btn("📡 مانیتور نود", "nd")}})
}

func (r *Router) ndMkInterval(chat, mid int64, i int) {
	panels := r.panels()
	if i < 0 || i >= len(panels) {
		r.Show(chat, mid, "❌ پنل پیدا نشد.", HomeKB())
		return
	}
	opts := []int{5, 10, 15, 30, 60}
	var kb [][]Btn
	var row []Btn
	for _, m := range opts {
		row = append(row, btn(fmt.Sprintf("%d دقیقه", m), fmt.Sprintf("ndset:%d:%d", i, m)))
		if len(row) == 3 {
			kb = append(kb, row)
			row = nil
		}
	}
	if len(row) > 0 {
		kb = append(kb, row)
	}
	kb = append(kb, []Btn{btn("🔙 بازگشت", "nd"), btn("🏠 خانه", "menu")})
	r.Show(chat, mid, fmt.Sprintf("⏱ فاصله پایش نودهای «%s»؟", Esc(pname(panels[i]))), kb)
}

func (r *Router) ndIntervalDo(chat, mid int64, arg string) {
	parts := strings.SplitN(arg, ":", 2)
	if len(parts) != 2 {
		r.Show(chat, mid, "❌", HomeKB())
		return
	}
	i, err := strconv.Atoi(parts[0])
	mins, err2 := strconv.Atoi(parts[1])
	panels := r.panels()
	if err != nil || err2 != nil || i < 0 || i >= len(panels) {
		r.Show(chat, mid, "❌", HomeKB())
		return
	}
	if mins < 1 {
		mins = 1
	}
	p := panels[i]
	lst := ndmon.Monitors(r.Store)
	id := "m" + strconv.FormatInt(time.Now().Unix(), 10)
	lst = append(lst, ndmon.Monitor{
		ID: id, PanelURL: p.URL, Name: pname(p), IntervalMin: mins,
		Excluded: []string{}, HookToken: randHex(8),
	})
	ndmon.SaveMonitors(r.Store, lst)
	r.Bot.Send(chat, fmt.Sprintf("✅ مانیتور نود «%s» ساخته شد (هر %d دقیقه).", pname(p), mins), nil)
	r.ndHome(chat, 0)
}

func (r *Router) ndToggle(chat, mid int64, idStr string) {
	lst := ndmon.Monitors(r.Store)
	for i := range lst {
		if lst[i].ID == idStr {
			v := !lst[i].IsEnabled()
			lst[i].Enabled = &v
		}
	}
	ndmon.SaveMonitors(r.Store, lst)
	r.ndHome(chat, mid)
}

func (r *Router) ndDelAsk(chat, mid int64, idStr string) {
	r.Show(chat, mid, "⚠️ مانیتور حذف شود؟",
		[][]Btn{{btn("✅ بله", "nddy:"+idStr), btn("❌ انصراف", "nd")}})
}

func (r *Router) ndDelDo(chat, mid int64, idStr string) {
	var keep []ndmon.Monitor
	for _, m := range ndmon.Monitors(r.Store) {
		if m.ID != idStr {
			keep = append(keep, m)
		}
	}
	ndmon.SaveMonitors(r.Store, keep)
	r.Store.Delete("ndst:" + idStr)
	r.Bot.Send(chat, "✅ حذف شد.", nil)
	r.ndHome(chat, 0)
}

func (r *Router) ndDelMenu(chat, mid int64) {
	mons := ndmon.Monitors(r.Store)
	var kb [][]Btn
	for _, m := range mons {
		kb = append(kb, []Btn{btn("🗑 "+m.Label(), "nddel:"+m.ID)})
	}
	kb = append(kb, []Btn{btn("🔙 بازگشت", "nd")})
	r.Show(chat, mid, "🗑 کدام مانیتور حذف شود؟", kb)
}

func (r *Router) ndMkPanel(chat, mid int64) {
	panels := r.panels()
	var kb [][]Btn
	for i, p := range panels {
		kb = append(kb, []Btn{btn("🖥 "+pname(p), fmt.Sprintf("ndmk:%d", i))})
	}
	kb = append(kb, []Btn{btn("🔙 بازگشت", "nd"), btn("🏠 خانه", "menu")})
	r.Show(chat, mid, "🖥 برای کدام پنل مانیتور بسازم؟", kb)
}

func (r *Router) ndIntAsk(chat, mid int64, idStr string) {
	r.setWizard(chat, map[string]any{"kind": "nd_int", "id": idStr})
	r.Show(chat, mid, "⏱ فاصلهٔ پایش را به دقیقه بفرستید (مثل ۱۰):",
		[][]Btn{{btn("🔙 بازگشت", "ndd:"+idStr), btn("🏠 خانه", "menu")}})
}

func (r *Router) ndIntSave(chat, mid int64, idStr, text string) {
	n, err := strconv.Atoi(nowJalaliDigits(text))
	if err != nil || n < 1 || n > 1440 {
		r.Bot.Send(chat, "❌ عدد نامعتبر (۱ تا ۱۴۴۰ دقیقه):", nil)
		return
	}
	lst := ndmon.Monitors(r.Store)
	for i := range lst {
		if lst[i].ID == idStr {
			lst[i].IntervalMin = n
		}
	}
	ndmon.SaveMonitors(r.Store, lst)
	r.clearWizard(chat)
	r.Bot.Send(chat, fmt.Sprintf("✅ فاصله پایش روی %d دقیقه تنظیم شد.", n), nil)
	r.ndDetail(chat, 0, idStr)
}
