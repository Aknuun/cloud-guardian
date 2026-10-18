package router

import (
	"fmt"
	"net/url"
	"regexp"
	"sort"
	"strconv"
	"strings"

	"github.com/aknuun/cg-server/internal/domexpiry"
	"github.com/aknuun/cg-server/internal/jalali"
)

const domexpPerPage = 30

func (r *Router) domexpList() []domexpiry.Item {
	var lst []domexpiry.Item
	r.Store.GetJSON("dom_expiry", &lst)
	return lst
}

func (r *Router) domexpCfg() (provider, key string) {
	var c map[string]any
	r.Store.GetJSON("dom_expiry_cfg", &c)
	provider = "whoisxmlapi_domaininfo"
	if c != nil {
		if p, ok := c["provider"].(string); ok && p != "" {
			provider = p
		}
		if k, ok := c["key"].(string); ok {
			key = k
		}
	}
	return
}

func dateOnly(ms int64) string {
	s := jalali.Fmt(ms)
	if i := strings.Index(s, " - "); i > 0 {
		return s[:i]
	}
	return s
}

func (r *Router) domexpHome(chat, mid int64, page int) {
	now := domexpiry.NowMS()
	lst := r.domexpList()
	sort.SliceStable(lst, func(i, j int) bool {
		ei, ej := lst[i].Expiry, lst[j].Expiry
		if ei == nil {
			return false
		}
		if ej == nil {
			return true
		}
		return *ei < *ej
	})
	_, key := r.domexpCfg()
	var b strings.Builder
	b.WriteString("🗓 مانیتور انقضای دامنه\n\n")
	b.WriteString("ℹ️ دامنه‌های بین‌المللی از RDAP و دامنه‌های .ir از whois.nic.ir استعلام می‌شوند.\n")
	if key != "" {
		b.WriteString("🔑 کلید API ثبت شده؛ برای دامنه‌های .ir که ایرنیک مخفی کرده، از دیتابیس تاریخی (WhoisXMLAPI) استعلام می‌شود.\n\n")
	} else {
		b.WriteString("⚠️ ایرنیک تاریخ دامنه‌های .ir را حذف/مخفی کرده. برای استعلام خودکار، «🔑 ثبت کلید API» را بزن؛ وگرنه از «✍️ ورود دستی» تاریخ را ثبت کن.\n\n")
	}
	if len(lst) == 0 {
		b.WriteString("📭 دامنه‌ای ثبت نشده. از دکمه‌های زیر اضافه کن:\n")
		b.WriteString("• «🌍 افزودن از اکانت‌ها»: همهٔ دامنه‌های اکانت‌های کلودفلر یک‌جا اضافه می‌شوند.\n")
		b.WriteString("• «✍️ ورود دستی»: یک دامنه را با نام یا با تاریخ دستی ثبت کن.")
	} else {
		b.WriteString(fmt.Sprintf("📌 %d دامنه (مرتب بر اساس نزدیک‌ترین انقضا):\n\n", len(lst)))
		pages := (len(lst) + domexpPerPage - 1) / domexpPerPage
		if pages < 1 {
			pages = 1
		}
		if page < 0 {
			page = 0
		}
		if page > pages-1 {
			page = pages - 1
		}
		lo := page * domexpPerPage
		hi := lo + domexpPerPage
		if hi > len(lst) {
			hi = len(lst)
		}
		for _, it := range lst[lo:hi] {
			b.WriteString(domexpiry.Line(it, now) + "\n")
			if it.Expiry != nil {
				b.WriteString("   🗓 انقضا: " + dateOnly(*it.Expiry) + "\n")
			}
		}
		if pages > 1 {
			b.WriteString(fmt.Sprintf("\nصفحه %d از %d", page+1, pages))
		}
	}
	kb := [][]Btn{{btn("🌍 افزودن از اکانت‌ها", "domexpall"), btn("✍️ ورود دستی", "domexpa")}}
	if len(lst) > 0 {
		kb = append(kb, []Btn{btn("🗓 ثبت تاریخ دستی", "domexpcal"), btn("🗑 حذف دامنه", "domexpd")})
	}
	kb = append(kb, []Btn{btn("🔄 استعلام دوبارهٔ همه", "domexp")})
	if key != "" {
		kb = append(kb, []Btn{btn("🔑 کلید API (ثبت‌شده ✅)", "domexpkey")})
	} else {
		kb = append(kb, []Btn{btn("🔑 ثبت کلید API", "domexpkey")})
	}
	kb = append(kb, []Btn{btn("🔙 بازگشت", "dommon"), btn("🏠 خانه", "menu")})
	text := b.String()
	if len(text) > 4000 {
		text = text[:4000]
	}
	r.Show(chat, mid, text, kb)
}

func (r *Router) domexpRefresh(chat, mid int64) {
	r.Show(chat, mid, "⏳ در حال استعلام دامنه‌ها…", HomeKB())
	r.domexpCheckAll()
	r.domexpHome(chat, mid, 0)
}

func (r *Router) domexpCheckAll() []domexpiry.Item {
	lst := r.domexpList()
	_, key := r.domexpCfg()
	for i := range lst {
		if lst[i].Manual && lst[i].Expiry != nil {
			continue
		}
		res := domexpiry.FetchDomainExpiry(lst[i].Domain, key)
		if res.Status == "ok" {
			lst[i].Status = "ok"
			lst[i].Expiry = res.Expiry
		} else {
			lst[i].Status = res.Status
			lst[i].Expiry = nil
		}
	}
	r.Store.PutJSON("dom_expiry", lst, 0)
	return lst
}

// DomexpDaily refreshes all domains and sends due expiry alerts (once per
// threshold). Returns the number of messages sent.
func (r *Router) DomexpDaily() int {
	r.domexpCheckAll()
	lst := r.domexpList()
	now := domexpiry.NowMS()
	sent := 0
	dirty := false
	for i := range lst {
		it := &lst[i]
		if it.Expiry == nil {
			continue
		}
		daysLeft := int((*it.Expiry - now) / 86400000)
		if daysLeft < 0 {
			daysLeft = 0
		}
		if !containsInt(domexpiry.AlertDays, daysLeft) {
			continue
		}
		if it.LastAlertDay != nil && *it.LastAlertDay == daysLeft {
			continue
		}
		d := daysLeft
		it.LastAlertDay = &d
		dirty = true
		em := domexpiry.Emoji(*it, now)
		jstr := "?"
		if parts := strings.Split(jalali.Fmt(*it.Expiry), " - "); len(parts) > 0 {
			jstr = parts[0]
		}
		msg := fmt.Sprintf("🗓 هشدار انقضای دامنه %s\n\n🌐 <code>%s</code>\n🗓 انقضا: %s\n⏳ باقی‌مانده: %s",
			em, it.Domain, jstr, domexpiry.Remaining(it.Expiry, now))
		if daysLeft == 0 {
			msg += "\n\n⛔ امروز منقضی می‌شود!"
		}
		for _, t := range r.Admins() {
			r.Bot.Send(t, msg, nil)
		}
		sent++
	}
	if dirty {
		r.Store.PutJSON("dom_expiry", lst, 0)
	}
	return sent
}

func containsInt(a []int, v int) bool {
	for _, x := range a {
		if x == v {
			return true
		}
	}
	return false
}

func (r *Router) domexpAddAll(chat, mid int64) {
	r.Show(chat, mid,
		"⚠️ «افزودن از اکانت‌ها» به لایهٔ کلودفلر نیاز دارد که در نسخهٔ Go هنوز پورت نشده.\nاز «✍️ ورود دستی» استفاده کن.",
		[][]Btn{{btn("✍️ ورود دستی", "domexpa"), btn("🔙 بازگشت", "domexpp:0")}})
}

func (r *Router) domexpAddStart(chat, mid int64) {
	r.setWizard(chat, map[string]any{"kind": "domexp_domain"})
	r.Show(chat, mid,
		"✍️ دامنه‌ای که می‌خواهید انقضایش نظارت شود را بفرستید (مثلاً example.ir یا example.com):\n\n"+
			"ℹ️ دامنهٔ .ir از whois.nic.ir استعلام می‌شود؛ اگر ایرنیک تاریخ را مخفی کرده باشد و «🔑 کلید API» ثبت شده باشد، خودکار از WhoisXMLAPI گرفته می‌شود؛ وگرنه از «🗓 ثبت تاریخ دستی» وارد کن.",
		[][]Btn{{btn("🔙 انصراف", "domexpp:0")}})
}

func (r *Router) domexpCalList(chat, mid int64) {
	lst := r.domexpList()
	if len(lst) == 0 {
		r.Show(chat, mid, "📭 دامنه‌ای ثبت نشده.",
			[][]Btn{{btn("🔙 بازگشت", "domexpp:0"), btn("🏠 خانه", "menu")}})
		return
	}
	var hidden []domexpiry.Item
	for _, x := range lst {
		if x.Status != "ok" {
			hidden = append(hidden, x)
		}
	}
	show := hidden
	if len(show) == 0 {
		show = lst
	}
	if len(show) > 30 {
		show = show[:30]
	}
	var items []Btn
	for _, x := range show {
		items = append(items, btn("🗓 "+x.Domain, "domexpi:"+url.QueryEscape(x.Domain)))
	}
	kb := gridBTN(items, 2)
	kb = append(kb, navRow("domexpp:0"))
	r.Show(chat, mid, "🗓 برای کدام دامنه تاریخ انقضا را دستی ثبت/اصلاح کنیم؟", kb)
}

func (r *Router) domexpCalDomain(chat, mid int64, dom string) {
	if d, err := url.QueryUnescape(dom); err == nil {
		dom = d
	}
	item := r.findDomexp(dom)
	if item == nil {
		r.Show(chat, mid, "❌ دامنه پیدا نشد.",
			[][]Btn{{btn("🔙 بازگشت", "domexpp:0"), btn("🏠 خانه", "menu")}})
		return
	}
	r.setWizard(chat, map[string]any{"kind": "domexp_date", "domain": dom})
	cur := ""
	if item.Expiry != nil {
		cur = "\n\n🗓 فعلی: " + dateOnly(*item.Expiry)
	}
	r.Show(chat, mid,
		fmt.Sprintf("🗓 تاریخ انقضای <code>%s</code> را به شمسی بفرستید:\nمثال: 1405/06/20\n\nℹ️ بعد از ثبت، تاریخ دستی مرجع است و دیگر استعلام خودکار روی آن انجام نمی‌شود.%s", Esc(dom), cur),
		[][]Btn{{btn("🔙 انصراف", "domexpp:0")}})
}

func (r *Router) domexpDelList(chat, mid int64) {
	lst := r.domexpList()
	if len(lst) == 0 {
		r.Show(chat, mid, "📭 دامنه‌ای ثبت نشده.",
			[][]Btn{{btn("🔙 بازگشت", "domexpp:0"), btn("🏠 خانه", "menu")}})
		return
	}
	n := len(lst)
	if n > 40 {
		n = 40
	}
	var items []Btn
	for i := 0; i < n; i++ {
		items = append(items, btn("🗑 "+lst[i].Domain, "domexpdy:"+strconv.Itoa(i)))
	}
	kb := gridBTN(items, 2)
	kb = append(kb, navRow("domexpp:0"))
	r.Show(chat, mid, "🗑 کدام دامنه از مانیتور حذف شود؟", kb)
}

func (r *Router) domexpDel(chat, mid int64, idx string) {
	lst := r.domexpList()
	i, err := strconv.Atoi(idx)
	if err != nil || i < 0 || i >= len(lst) {
		r.Show(chat, mid, "❌ دامنه پیدا نشد.", HomeKB())
		return
	}
	rm := lst[i]
	lst = append(lst[:i:i], lst[i+1:]...)
	r.Store.PutJSON("dom_expiry", lst, 0)
	r.Show(chat, mid, fmt.Sprintf("✅ «%s» از مانیتور حذف شد.", Esc(rm.Domain)),
		[][]Btn{{btn("🗓 مانیتور انقضای دامنه", "domexpp:0")}})
}

func (r *Router) domexpKeyPage(chat, mid int64) {
	_, key := r.domexpCfg()
	has := key != ""
	masked := "—"
	if has {
		if len(key) > 8 {
			masked = key[:4] + "…" + key[len(key)-4:]
		} else {
			masked = key
		}
	}
	title := "🔑 ثبت کلید API"
	if has {
		title = "♻️ تغییر کلید API"
	}
	kb := [][]Btn{{btn(title, "domexpk")}}
	if has {
		kb = append(kb, []Btn{btn("🧪 تست روی pcapps.ir", "domexptest")})
		kb = append(kb, []Btn{btn("🗑 حذف کلید", "domexpkdel")})
	}
	kb = append(kb, []Btn{btn("🔙 بازگشت", "domexpp:0"), btn("🏠 خانه", "menu")})
	status := "ثبت‌نشده ❌"
	if has {
		status = "ثبت‌شده ✅"
	}
	r.Show(chat, mid,
		"🔑 کلید API انقضای دامنه\n\n"+
			"وضعیت: "+status+"\n"+
			fmt.Sprintf("کلید: <code>%s</code>\n", Esc(masked))+
			"سرویس: WhoisXMLAPI — Domain Info API\n\n"+
			"چرا لازم است؟ ایرنیک تاریخ انقضای دامنه‌های .ir را از whois حذف کرده؛ این API فیلدهای حذف‌شده را از دیتابیس تاریخی پر می‌کند.\n\n"+
			"کلید فقط در همین ربات ذخیره می‌شود.", kb)
}

func (r *Router) domexpKeyStart(chat, mid int64) {
	r.setWizard(chat, map[string]any{"kind": "domexp_key"})
	r.Show(chat, mid, "🔑 کلید API را بفرستید:\n(برای حذف کلید، «-» بفرستید)",
		[][]Btn{{btn("🔙 انصراف", "domexpkey")}})
}

func (r *Router) domexpKeyDel(chat, mid int64) {
	r.Store.PutJSON("dom_expiry_cfg", map[string]any{
		"provider": "whoisxmlapi_domaininfo", "key": ""}, 0)
	r.Show(chat, mid, "🗑 کلید API حذف شد.",
		[][]Btn{{btn("🔙 بازگشت", "domexpkey"), btn("🏠 خانه", "menu")}})
}

func (r *Router) domexpTest(chat, mid int64) {
	_, key := r.domexpCfg()
	if key == "" {
		r.Show(chat, mid, "❌ کلیدی ثبت نشده.",
			[][]Btn{{btn("🔙 بازگشت", "domexpkey"), btn("🏠 خانه", "menu")}})
		return
	}
	r.Show(chat, mid, "⏳ در حال تست کلید روی pcapps.ir…", HomeKB())
	res := domexpiry.FetchDomainExpiry("pcapps.ir", key)
	if res.Status == "ok" && res.Expiry != nil {
		r.Show(chat, mid,
			fmt.Sprintf("✅ کلید معتبر است.\n🗓 انقضای pcapps.ir: %s\n⏳ باقی‌مانده: %s",
				dateOnly(*res.Expiry), domexpiry.Remaining(res.Expiry, domexpiry.NowMS())),
			[][]Btn{{btn("🔙 بازگشت", "domexpkey"), btn("🏠 خانه", "menu")}})
		return
	}
	r.Show(chat, mid, "⚠️ تست ناموفق بود: "+Esc(res.Status),
		[][]Btn{{btn("🔙 بازگشت", "domexpkey"), btn("🏠 خانه", "menu")}})
}

func (r *Router) findDomexp(dom string) *domexpiry.Item {
	lst := r.domexpList()
	for i := range lst {
		if lst[i].Domain == dom {
			return &lst[i]
		}
	}
	return nil
}

var reStripScheme = regexp.MustCompile(`(?i)^https?://`)
var reDomDate = regexp.MustCompile(`^(\d{4})[/\-.](\d{1,2})[/\-.](\d{1,2})$`)

func (r *Router) domexpWizardKey(chat int64, text string) {
	key := strings.TrimSpace(text)
	r.clearWizard(chat)
	cfg := map[string]any{"provider": "whoisxmlapi_domaininfo", "key": key}
	if key == "" || key == "-" {
		cfg["key"] = ""
		r.Store.PutJSON("dom_expiry_cfg", cfg, 0)
		r.Bot.Send(chat, "🗑 کلید API حذف شد.",
			[][]Btn{{btn("🗓 مانیتور انقضای دامنه", "domexpp:0")}})
		return
	}
	r.Store.PutJSON("dom_expiry_cfg", cfg, 0)
	r.Bot.Send(chat, "⏳ کلید ذخیره شد. در حال تست روی pcapps.ir…", nil)
	res := domexpiry.FetchDomainExpiry("pcapps.ir", key)
	switch {
	case res.Status == "ok" && res.Expiry != nil:
		r.Bot.Send(chat,
			fmt.Sprintf("✅ کلید API معتبر است.\n🗓 انقضای pcapps.ir: %s\n⏳ باقی‌مانده: %s",
				dateOnly(*res.Expiry), domexpiry.Remaining(res.Expiry, domexpiry.NowMS())),
			[][]Btn{{btn("🗓 مانیتور انقضای دامنه", "domexpp:0"), btn("🔙 بازگشت", "menu")}})
	case res.Status == "api_error":
		r.Bot.Send(chat,
			fmt.Sprintf("⚠️ کلید ذخیره شد اما تست ناموفق بود:\n<code>%s</code>", Esc(res.Message)),
			[][]Btn{{btn("🗓 مانیتور انقضای دامنه", "domexpp:0")}})
	default:
		r.Bot.Send(chat,
			"⚠️ کلید ذخیره شد، اما برای pcapps.ir تاریخی برنگشت (ممکن است در دیتابیس تاریخی هم نباشد).",
			[][]Btn{{btn("🗓 مانیتور انقضای دامنه", "domexpp:0")}})
	}
}

func (r *Router) domexpWizardDate(chat, mid int64, pend map[string]any, text string) {
	s := jalali.ParseFA(strings.TrimSpace(text))
	m := reDomDate.FindStringSubmatch(s)
	if m == nil {
		r.Bot.Send(chat, "❌ قالب درست نیست. مثال: 1405/06/20", nil)
		return
	}
	mo, _ := strconv.Atoi(m[2])
	da, _ := strconv.Atoi(m[3])
	ms, err := jalali.ParseDatetime(fmt.Sprintf("%s/%d/%d 12:00", m[1], mo, da))
	if err != nil {
		r.Bot.Send(chat, "❌ تاریخ نامعتبر است. دوباره بفرستید (مثلاً 1405/06/20):", nil)
		return
	}
	if ms <= domexpiry.NowMS() {
		r.Bot.Send(chat, "⚠️ این تاریخ گذشته است. تاریخ انقضای آینده را بفرستید:", nil)
		return
	}
	r.clearWizard(chat)
	dom, _ := pend["domain"].(string)
	lst := r.domexpList()
	found := false
	for i := range lst {
		if lst[i].Domain == dom {
			lst[i].Manual = true
			lst[i].Status = "ok"
			e := ms
			lst[i].Expiry = &e
			found = true
		}
	}
	if !found {
		r.Bot.Send(chat, "❌ دامنه در مانیتور پیدا نشد.",
			[][]Btn{{btn("🗓 مانیتور انقضای دامنه", "domexpp:0")}})
		return
	}
	r.Store.PutJSON("dom_expiry", lst, 0)
	r.Bot.Send(chat, fmt.Sprintf("✅ تاریخ انقضای <code>%s</code> ثبت شد: %s", Esc(dom), dateOnly(ms)),
		[][]Btn{{btn("🗓 مانیتور انقضای دامنه", "domexpp:0"), btn("🏠 خانه", "menu")}})
}

// domexpWizardDomain handles the "domexp_domain" wizard input.
func (r *Router) domexpWizardDomain(chat, mid int64, text string) {
	domain := strings.ToLower(strings.TrimSpace(reStripScheme.ReplaceAllString(text, "")))
	domain = strings.TrimSpace(strings.SplitN(domain, "/", 2)[0])
	if !domexpiry.IsValidDomain(domain) {
		r.Bot.Send(chat, "❌ نام دامنه معتبر نیست. دوباره بفرستید (مثلاً example.ir):", nil)
		return
	}
	lst := r.domexpList()
	if r.findDomexp(domain) != nil {
		r.Bot.Send(chat, "❌ این دامنه از قبل در مانیتور هست.",
			[][]Btn{{btn("🗓 مانیتور انقضای دامنه", "domexpp:0")}})
		r.clearWizard(chat)
		return
	}
	r.clearWizard(chat)
	r.Bot.Send(chat, "⏳ در حال استعلام انقضای دامنه…", nil)
	_, key := r.domexpCfg()
	res := domexpiry.FetchDomainExpiry(domain, key)
	added := domexpiry.NowMS()
	if res.Status == "ok" && res.Expiry != nil {
		lst = append(lst, domexpiry.Item{Domain: domain, Status: "ok", Expiry: res.Expiry, Added: added})
		r.Store.PutJSON("dom_expiry", lst, 0)
		r.Bot.Send(chat,
			fmt.Sprintf("✅ <code>%s</code> به مانیتور اضافه شد.\n🗓 انقضا: %s\n⏳ باقی‌مانده: %s",
				Esc(domain), dateOnly(*res.Expiry), domexpiry.Remaining(res.Expiry, domexpiry.NowMS())),
			[][]Btn{{btn("🗓 مانیتور انقضای دامنه", "domexpp:0"), btn("🔙 بازگشت", "menu")}})
		return
	}
	lst = append(lst, domexpiry.Item{Domain: domain, Status: res.Status, Added: added})
	r.Store.PutJSON("dom_expiry", lst, 0)
	reason := "استعلام ناموفق بود؛ بعداً با «🔄 استعلام دوبارهٔ همه» امتحان کن."
	switch res.Status {
	case "hidden":
		reason = "ایرنیک تاریخ این دامنه را مخفی کرده. برای استعلام خودکار «🔑 ثبت کلید API» را بزن."
	case "not_found":
		reason = "دامنه در ایرنیک ثبت نیست."
	case "available":
		reason = "دامنه آزاد/ثبت‌نشده است."
	case "api_error":
		reason = "خطای API: " + res.Message
	}
	r.Bot.Send(chat,
		fmt.Sprintf("➕ <code>%s</code> اضافه شد، اما تاریخ انقضا پیدا نشد.\nℹ️ %s\n\nاز «🗓 ثبت تاریخ دستی» می‌توانی تاریخ را خودت وارد کنی.", Esc(domain), reason),
		[][]Btn{{btn("🗓 ثبت تاریخ دستی", "domexpcal"), btn("🗓 مانیتور انقضای دامنه", "domexpp:0")}})
}
