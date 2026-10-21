package router

import (
	"fmt"
	"sort"
	"strconv"
	"strings"

	"github.com/aknuun/cg-server/internal/cf"
	"github.com/aknuun/cg-server/internal/hostfilter"
	"github.com/aknuun/cg-server/internal/jalali"
)

func jalaliFA(s string) string { return jalali.ParseFA(s) }

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
	case bool:
		return strconv.FormatBool(t)
	default:
		return fmt.Sprint(v)
	}
}

func (r *Router) cfAccs() []cf.Account {
	var accs []cf.Account
	if r.Store.GetJSON("cf_accounts", &accs) && len(accs) > 0 {
		return accs
	}
	if r.CFToken != "" {
		return []cf.Account{{Name: "main", Token: r.CFToken}}
	}
	return nil
}

// CFAccounts exposes the stored Cloudflare credentials for background runs.
func (r *Router) CFAccounts() []cf.Account { return r.cfAccs() }

func hfShort(t string, n int) string {
	rr := []rune(t)
	if len(rr) <= n {
		return t
	}
	return string(rr[:n-1]) + "…"
}

func (r *Router) hfHome(chat, mid int64) {
	cfg := hostfilter.GetCfg(r.Store)
	enabled := "⏸ غیرفعال"
	if cfg.IsEnabled() {
		enabled = "▶️ فعال"
	}
	cities := "—"
	if len(cfg.CitiesSel) > 0 {
		cities = strings.Join(cfg.CitiesSel, "، ")
	}
	recheck := "غیرفعال (تعویض فوری)"
	if cfg.RecheckCount > 0 {
		recheck = fmt.Sprintf("%d بار، هر %d دقیقه", cfg.RecheckCount, cfg.RecheckMin)
	}
	lines := []string{"🧭 تعویض خودکار هاست فیلتر", "",
		"💡 برای بررسی سریع‌تر: هاست‌هایی که نیاز به تعویض ندارند را از «📋 لیست هاست‌ها» با «🚫 استثنا» از چرخه خارج کن؛ هرچه هاست کمتر، ران زودتر تمام می‌شود.",
		"وضعیت: " + enabled,
		"🔌 سرویس بررسی: check-host (در صورت قطعی: Globalping)",
		fmt.Sprintf("⏱ فاصلهٔ اجرا: هر %d دقیقه", cfg.IntervalMin),
		"🌆 شهرها: " + cities,
		fmt.Sprintf("🎯 آستانه: %d شهر از %d (پینگ موفق ≤ %d از ۴)", cfg.Cities, len(cfg.CitiesSel), cfg.MaxOk),
		fmt.Sprintf("📦 تعداد هر اجرا: %d · 🔁 حداکثر تعویض: %d", cfg.Batch, cfg.MaxChanges),
		"🔁 بررسی مجدد: " + recheck,
		fmt.Sprintf("روش: دامنه‌های هاست‌ها هر %d دقیقه از داخل ایران بررسی می‌شوند؛ در صورت فیلتر یک دامنهٔ شماره‌دار جدید ساخته و در همان هاست جایگزین می‌شود.", cfg.IntervalMin)}
	if cfg.LastSummary != "" {
		lines = append(lines, "", "آخرین ران: "+cfg.LastSummary)
	}
	toggle := "▶️ فعال‌سازی"
	if cfg.IsEnabled() {
		toggle = "⏸ غیرفعال‌سازی"
	}
	kb := [][]Btn{
		{btn(toggle, "hftg")},
		{btn("⚡ فورس بررسی کامل", "hfforceall")},
		{btn("📋 لیست هاست‌ها", "hflist")},
		{btn("💾 بکاپ هاست‌ها", "hfbk")},
		{btn("🔁 چرخش روزانه ساب‌ها", "hfd")},
		{btn("↩️ بازگردانی تعویض‌ها", "hfrevlist")},
		{btn("⚙️ تنظیمات چک‌هاست", "hfsetch")},
		{btn("🏠 خانه", "menu")},
	}
	r.Show(chat, mid, strings.Join(lines, "\n"), kb)
}

func (r *Router) hfSettings(chat, mid int64) {
	cfg := hostfilter.GetCfg(r.Store)
	cities := "—"
	if len(cfg.CitiesSel) > 0 {
		cities = strings.Join(cfg.CitiesSel, "، ")
	}
	lines := []string{"⚙️ تنظیمات چک‌هاست", "",
		"⚠️ هرچه فاصلهٔ اجرا کوتاه‌تر یا تعداد هر اجرا بیشتر باشد، درخواست‌های بیشتری به کلادفلر فرستاده می‌شود و ممکن است باعث محدود شدن شود.", "",
		fmt.Sprintf("• ⏱ فاصلهٔ اجرا: %d دقیقه", cfg.IntervalMin),
		"• 🌆 شهرهای بررسی‌شده: " + cities,
		fmt.Sprintf("• 🎯 حداقل شهرهای فیلتر: %d", cfg.Cities),
		fmt.Sprintf("• 📶 حداکثر پینگ موفق مجاز: %d از ۴", cfg.MaxOk),
		fmt.Sprintf("• 🔁 تعداد بررسی مجدد: %d", cfg.RecheckCount),
		fmt.Sprintf("• ⏱ فاصلهٔ بررسی مجدد: هر %d دقیقه", cfg.RecheckMin),
		fmt.Sprintf("• 📦 تعداد بررسی در هر اجرا: %d", cfg.Batch),
		fmt.Sprintf("• 🔁 حداکثر تعویض در هر اجرا: %d", cfg.MaxChanges)}
	kb := [][]Btn{
		{btn("⏱ فاصلهٔ اجرا", "hfnum:intervalMin")},
		{btn("🎯 حداقل شهر", "hfnum:cities"), btn("📶 حداکثر پینگ موفق", "hfnum:maxOk")},
		{btn("📦 تعداد هر اجرا", "hfnum:batch"), btn("🔁 حداکثر تعویض", "hfnum:maxChanges")},
		{btn("♻️ تعداد بررسی مجدد", "hfnum:recheckCount"), btn("⏱ فاصلهٔ بررسی مجدد", "hfnum:recheckMin")},
		{btn("🔙 بازگشت", "back:hf"), btn("🏠 خانه", "menu")},
	}
	r.Show(chat, mid, strings.Join(lines, "\n"), kb)
}

func (r *Router) hfHist(chat, mid int64) {
	var log []map[string]any
	r.Store.GetJSON("host_filter_log", &log)
	lines := []string{"📜 تاریخچه تعویض‌ها", ""}
	start := 0
	if len(log) > 15 {
		start = len(log) - 15
	}
	for _, e := range log[start:] {
		ts := strv(e["ts"])
		if len(ts) > 16 {
			ts = ts[:16]
		}
		ts = strings.Replace(ts, "T", " ", 1)
		if strv(e["kind"]) == "swap" {
			lines = append(lines, fmt.Sprintf("%s · %s → %s", ts, strv(e["from"]), strv(e["to"])))
		} else {
			lines = append(lines, fmt.Sprintf("%s · %s", ts, strv(e["kind"])))
		}
	}
	if len(log) <= 1 {
		lines = append(lines, "📭 چیزی ثبت نشده.")
	}
	r.Show(chat, mid, strings.Join(lines, "\n"),
		navKB("hflist"))
}

// hfAllKeys returns "<pi>:<hid>:<field>" keys for every enabled host field.
// pid >= 0 restricts to one panel.
func (r *Router) hfAllKeys(pid int) []string {
	var keys []string
	for pi, p := range r.panels() {
		if pid >= 0 && pi != pid {
			continue
		}
		c := p.Client()
		tok, err := c.Login()
		if err != nil {
			continue
		}
		hosts, err := c.Hosts(tok)
		if err != nil {
			continue
		}
		for _, hv := range hosts {
			h, ok := hv.(map[string]any)
			if !ok {
				continue
			}
			hid := strv(h["id"])
			for _, f := range []string{"host", "address", "sni"} {
				keys = append(keys, fmt.Sprintf("%d:%s:%s", pi, hid, f))
			}
		}
	}
	return keys
}

func (r *Router) hfExcToggle(chat, mid int64, pid, hid, field string) {
	cfg := hostfilter.GetCfg(r.Store)
	key := pid + ":" + hid + ":" + field
	set := map[string]bool{}
	for _, x := range cfg.Exceptions {
		set[x] = true
	}
	if set[key] {
		delete(set, key)
	} else {
		set[key] = true
	}
	out := make([]string, 0, len(set))
	for k := range set {
		out = append(out, k)
	}
	sort.Strings(out)
	cfg.Exceptions = out
	hostfilter.SaveCfg(r.Store, cfg)
	r.hfHosts(chat, mid)
}

func (r *Router) hfExcAll(chat, mid int64) {
	cfg := hostfilter.GetCfg(r.Store)
	set := map[string]bool{}
	for _, x := range cfg.Exceptions {
		set[x] = true
	}
	for _, k := range r.hfAllKeys(-1) {
		set[k] = true
	}
	cfg.Exceptions = sortedKeys(set)
	hostfilter.SaveCfg(r.Store, cfg)
	r.hfHosts(chat, mid)
}

func (r *Router) hfUnExcAll(chat, mid int64) {
	cfg := hostfilter.GetCfg(r.Store)
	cfg.Exceptions = nil
	hostfilter.SaveCfg(r.Store, cfg)
	r.hfHosts(chat, mid)
}

func (r *Router) hfExcPanel(chat, mid int64, pid int) {
	cfg := hostfilter.GetCfg(r.Store)
	set := map[string]bool{}
	for _, x := range cfg.Exceptions {
		set[x] = true
	}
	for _, k := range r.hfAllKeys(pid) {
		set[k] = true
	}
	cfg.Exceptions = sortedKeys(set)
	hostfilter.SaveCfg(r.Store, cfg)
	r.hfHosts(chat, mid)
}

func (r *Router) hfUnExcPanel(chat, mid int64, pid int) {
	cfg := hostfilter.GetCfg(r.Store)
	pre := strconv.Itoa(pid) + ":"
	var out []string
	for _, x := range cfg.Exceptions {
		if !strings.HasPrefix(x, pre) {
			out = append(out, x)
		}
	}
	cfg.Exceptions = out
	hostfilter.SaveCfg(r.Store, cfg)
	r.hfHosts(chat, mid)
}

func sortedKeys(set map[string]bool) []string {
	out := make([]string, 0, len(set))
	for k := range set {
		out = append(out, k)
	}
	sort.Strings(out)
	return out
}

func selStrings(v any) []string {
	switch t := v.(type) {
	case []string:
		return append([]string{}, t...)
	case []any:
		var out []string
		for _, x := range t {
			if s, ok := x.(string); ok {
				out = append(out, s)
			}
		}
		return out
	}
	return nil
}

func firstOf(v any, def string) string {
	for _, x := range toList(v) {
		if s := strv(x); s != "" {
			return s
		}
	}
	return def
}

func toList(v any) []any {
	if v == nil {
		return nil
	}
	if arr, ok := v.([]any); ok {
		return arr
	}
	return []any{v}
}

func (r *Router) hfHosts(chat, mid int64, note ...string) {
	cfg := hostfilter.GetCfg(r.Store)
	exc := map[string]bool{}
	for _, x := range cfg.Exceptions {
		exc[x] = true
	}
	var sel map[string]any
	r.Store.GetJSON(fmt.Sprintf("hfsel:%d", chat), &sel)
	selMode := sel != nil && sel["on"] == true
	selSet := map[string]bool{}
	for _, x := range selStrings(sel["sel"]) {
		selSet[x] = true
	}
	lines := []string{"📋 لیست هاست‌ها (فقط فعال‌ها)", "",
		"سه ستون: Host | Address | Network — کلیک هر کدام = استثنای همون فیلد (آبی در چرخه · خاکستری مستثنا)",
		"⚡ برای سرعت بیشتر، هاست‌های اضافه را گروهی استثنا کن 👇", ""}
	if len(note) > 0 && note[0] != "" {
		lines = append(lines, note[0], "")
	}
	kb := [][]Btn{{btn("🚫 استثنای همه", "hfexcall"), btn("✅ لغو استثنای همه", "hfunexcall")}}
	n := 0
	for pi, p := range r.panels() {
		c := p.Client()
		tok, err := c.Login()
		if err != nil {
			continue
		}
		hosts, err := c.Hosts(tok)
		if err != nil {
			continue
		}
		lines = append(lines, "", fmt.Sprintf("— 🖥 %s —", pname(p)))
		kb = append(kb, []Btn{
			btn("🚫 همهٔ این پنل", fmt.Sprintf("hfexcpanel:%d", pi)),
			btn("✅ فعال‌سازی همه", fmt.Sprintf("hfunexcpanel:%d", pi))})
		for _, hv := range hosts {
			h, ok := hv.(map[string]any)
			if !ok {
				continue
			}
			if b, ok := h["is_disabled"].(bool); ok && b {
				continue
			}
			n++
			hid := strv(h["id"])
			key := fmt.Sprintf("%d:%s", pi, hid)
			hostL := hfShort(firstOf(h["remark"], firstOf(h["sni"], "host")), 22)
			addrL := hfShort(firstOf(h["address"], "—"), 26)
			netL := hfShort(firstOf(append(toList(h["sni"]), append(toList(h["host"]), "—")...), "—"), 26)
			lines = append(lines, fmt.Sprintf("%d) %s | %s | %s", n, hostL, addrL, netL))
			if selMode {
				picked := selSet[key]
				st := ""
				if picked {
					st = "primary"
				}
				mark := "⬜ "
				if picked {
					mark = "✅ "
				}
				kb = append(kb, []Btn{
					btnS(mark+hfShort(hostL, 20), fmt.Sprintf("hfgs:%d:%s", pi, hid), st),
					btnS(mark+hfShort(addrL, 20), fmt.Sprintf("hfgs:%d:%s", pi, hid), st),
					btnS(mark+hfShort(netL, 20), fmt.Sprintf("hfgs:%d:%s", pi, hid), st)})
			} else {
				eH := exc[key+":host"]
				eA := exc[key+":address"]
				eN := exc[key+":sni"]
				kb = append(kb, []Btn{
					btnS(excMark(eH)+hfShort(hostL, 20), fmt.Sprintf("hfexc:%d:%s:host", pi, hid), excStyle(eH)),
					btnS(excMark(eA)+hfShort(addrL, 20), fmt.Sprintf("hfexc:%d:%s:address", pi, hid), excStyle(eA)),
					btnS(excMark(eN)+hfShort(netL, 20), fmt.Sprintf("hfexc:%d:%s:sni", pi, hid), excStyle(eN))})
			}
		}
	}
	if n == 0 {
		lines = append(lines, "📭 هاستی پیدا نشد.")
	}
	if selMode {
		lines = append(lines, fmt.Sprintf("🗂 %d انتخاب شده:", len(selSet)))
		kb = append(kb, []Btn{btn("🚫 استثنا برای انتخاب‌ها", "hfgexc"), btn("✅ لغو استثنا", "hfgunexc")})
		kb = append(kb, []Btn{btn("✅ انتخاب همه", "hfgall"), btn("🧹 پاک‌سازی", "hfgclr")})
		kb = append(kb, []Btn{btn("❌ خروج از انتخاب", "hfgdone")})
	} else {
		kb = append(kb, []Btn{btn("🗂 انتخاب گروهی", "hfgmode")})
	}
	kb = append(kb, []Btn{btn("📜 تاریخچه", "hfhist")})
	kb = append(kb, []Btn{btn("🔙 بازگشت", "back:hf"), btn("🏠 خانه", "menu")})
	txt := strings.Join(lines, "\n")
	if len(txt) > 3500 {
		txt = txt[:3500]
	}
	r.Show(chat, mid, txt, kb)
}

func excMark(e bool) string {
	if e {
		return "⬜ "
	}
	return "✅ "
}

func excStyle(e bool) string {
	if e {
		return ""
	}
	return "primary"
}

func (r *Router) hfGMode(chat, mid int64) {
	r.Store.PutJSON(fmt.Sprintf("hfsel:%d", chat), map[string]any{"on": true, "sel": []string{}}, 1800)
	r.hfHosts(chat, mid)
}

func (r *Router) hfGDone(chat, mid int64) {
	r.Store.Delete(fmt.Sprintf("hfsel:%d", chat))
	r.hfHosts(chat, mid)
}

func (r *Router) hfGAll(chat, mid int64) {
	r.Store.PutJSON(fmt.Sprintf("hfsel:%d", chat),
		map[string]any{"on": true, "sel": r.hfAllKeys(-1)}, 1800)
	r.hfHosts(chat, mid)
}

func (r *Router) hfGClr(chat, mid int64) {
	r.Store.PutJSON(fmt.Sprintf("hfsel:%d", chat), map[string]any{"on": true, "sel": []string{}}, 1800)
	r.hfHosts(chat, mid)
}

func (r *Router) hfGSelect(chat, mid int64, pid, hid string) {
	var sel map[string]any
	r.Store.GetJSON(fmt.Sprintf("hfsel:%d", chat), &sel)
	cur := map[string]bool{}
	for _, x := range selStrings(sel["sel"]) {
		cur[x] = true
	}
	key := pid + ":" + hid
	if cur[key] {
		delete(cur, key)
	} else {
		cur[key] = true
	}
	sel = map[string]any{"on": true, "sel": sortedKeys(cur)}
	r.Store.PutJSON(fmt.Sprintf("hfsel:%d", chat), sel, 1800)
	r.hfHosts(chat, mid)
}

func (r *Router) hfGExc(chat, mid int64) {
	cfg := hostfilter.GetCfg(r.Store)
	set := map[string]bool{}
	for _, x := range cfg.Exceptions {
		set[x] = true
	}
	var sel map[string]any
	r.Store.GetJSON(fmt.Sprintf("hfsel:%d", chat), &sel)
	for _, k := range selStrings(sel["sel"]) {
		set[k] = true
	}
	cfg.Exceptions = sortedKeys(set)
	hostfilter.SaveCfg(r.Store, cfg)
	r.Store.Delete(fmt.Sprintf("hfsel:%d", chat))
	r.hfHosts(chat, mid)
}

func (r *Router) hfGUnExc(chat, mid int64) {
	cfg := hostfilter.GetCfg(r.Store)
	var sel map[string]any
	r.Store.GetJSON(fmt.Sprintf("hfsel:%d", chat), &sel)
	selSet := map[string]bool{}
	for _, k := range selStrings(sel["sel"]) {
		selSet[k] = true
	}
	var out []string
	for _, x := range cfg.Exceptions {
		drop := selSet[x]
		for k := range selSet {
			if x == k || strings.HasPrefix(x, k+":") {
				drop = true
				break
			}
		}
		if !drop {
			out = append(out, x)
		}
	}
	cfg.Exceptions = out
	hostfilter.SaveCfg(r.Store, cfg)
	r.Store.Delete(fmt.Sprintf("hfsel:%d", chat))
	r.hfHosts(chat, mid)
}

func (r *Router) hfRunBg(chat int64, force bool, only string) {
	bot, st, panels := r.Bot, r.Store, r.panels()
	accs := r.cfAccs()
	admin := r.AdminID
	go func() {
		res := hostfilter.Run(bot, st, panels, admin, accs, force, only)
		msg := fmt.Sprintf("✅ ران تمام شد: بررسی %d دامنه — %d تعویض.", res.Checked, res.Changed)
		if res.Msg != "" {
			msg = "⚠️ " + res.Msg
		}
		bot.Send(chat, msg, nil)
	}()
}

func (r *Router) hfRevertList(chat, mid int64) {
	hostfilter.EnsureStateFromLog(r.Store, r.panels())
	entries := hostfilter.StateEntries(r.Store)
	if len(entries) == 0 {
		r.Show(chat, mid, "↩️ بازگردانی تعویض‌ها\n\n📭 تعویضی برای بازگردانی نیست.",
			navKB("hf"))
		return
	}
	var kb [][]Btn
	for _, e := range entries {
		from, to, sid := strv(e["from"]), strv(e["to"]), strv(e["sid"])
		lb := "↩️ " + truncRune(from, 24)
		if to != "" {
			lb += " ← " + truncRune(to, 24)
		}
		kb = append(kb, []Btn{btnS(lb, "hfrev:"+sid, "danger")})
	}
	kb = append(kb, []Btn{btn("🔙 بازگشت", "back:hf"), btn("🏠 خانه", "menu")})
	r.Show(chat, mid, "↩️ بازگردانی تعویض‌ها\n\nکدام هاست به دامنهٔ قبلی برگردد؟", kb)
}

func (r *Router) hfRevert(chat, mid int64, sid string) {
	_, err := hostfilter.Revert(r.Store, r.panels(), sid)
	if err != nil {
		r.Show(chat, mid, "❌ بازگردانی ناموفق: "+Esc(hfErrFA(err)),
			navKB("hfrevlist"))
		return
	}
	r.Show(chat, mid, "✅ به دامنهٔ قبلی برگشت.",
		[][]Btn{{btn("🧭 تعویض خودکار", "hf"), btn("🏠 خانه", "menu")}})
}

func hfErrFA(err error) string {
	switch err.Error() {
	case "state_not_found":
		return "این تعویض دیگر در دسترس نیست."
	case "panel_not_found":
		return "پنل پیدا نشد."
	case "login_failed":
		return "ورود به پنل ناموفق بود."
	case "fetch_failed":
		return "دریافت هاست‌ها ناموفق بود."
	case "host_not_found":
		return "هاست موردنظر پیدا نشد."
	default:
		return err.Error()
	}
}

func (r *Router) hfNumAsk(chat, mid int64, key string) {
	r.wizStart(chat, mid, map[string]any{"kind": "hf_num", "key": key})
	r.Show(chat, mid, "🔢 عدد جدید را بفرستید:", [][]Btn{{btn("🔙 انصراف", "hfsetch")}})
}

func (r *Router) hfNumSave(chat, mid int64, key, text string) {
	n, err := strconv.Atoi(strings.TrimSpace(jalaliFA(text)))
	if err != nil || n < 0 {
		r.Bot.Send(chat, "❌ عدد نامعتبر است.", nil)
		return
	}
	cfg := hostfilter.GetCfg(r.Store)
	switch key {
	case "intervalMin":
		cfg.IntervalMin = n
	case "cities":
		cfg.Cities = n
	case "maxOk":
		cfg.MaxOk = n
	case "batch":
		cfg.Batch = n
	case "maxChanges":
		cfg.MaxChanges = n
	case "recheckCount":
		cfg.RecheckCount = n
	case "recheckMin":
		cfg.RecheckMin = n
	default:
		cfg.IntervalMin = n
	}
	hostfilter.SaveCfg(r.Store, cfg)
	r.clearWizard(chat)
	r.hfSettings(chat, mid)
}
