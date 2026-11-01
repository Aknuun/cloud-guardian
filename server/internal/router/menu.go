package router

import (
	"fmt"
	"strings"
)

// Promo channel shown in the bot menu and the mini app.
const (
	ChannelHandle = "openWEBcode"
	ChannelURL    = "https://t.me/openWEBcode"
)

func chanBtn() Btn { return Btn{Text: "📣 @" + ChannelHandle, URL: ChannelURL} }

// webLink builds a deep link into the mini app for one section.
func (r *Router) webLink(goID string) string {
	u := strings.TrimSpace(r.WebAppURL)
	if u == "" {
		return ""
	}
	if strings.Contains(u, "?") {
		return u + "&go=" + goID
	}
	return u + "?go=" + goID
}

// webBtn opens a section in the mini app; falls back to the native screen when
// no web app URL is configured.
func (r *Router) webBtn(label, native, goID string) Btn {
	return r.webBtnS(label, native, goID, "plain")
}

// webBtnS is webBtn with an explicit button colour.
func (r *Router) webBtnS(label, native, goID, style string) Btn {
	if u := r.webLink(goID); u != "" {
		return btnWAS(label, u, style)
	}
	return btnS(label, native, style)
}

// Menu renders the home menu. Everything except Cloudflare opens in the mini app
// (the bot itself only sends reports); the rest stays reachable from "بیشتر".
func (r *Router) Menu(chat, mid int64) {
	// the channel button is gone: the last row opens the web panel instead
	siteBtn := chanBtn()
	if u := strings.TrimSpace(r.WebAppURL); u != "" {
		// signed one-time ticket: opening the panel from the bot needs no approval
		sep := "?"
		if strings.Contains(u, "?") {
			sep = "&"
		}
		siteBtn = Btn{Text: "🌐 نسخهٔ وب سایت", URL: u + sep + "k=" + r.webMakeTicket(), Style: "success"}
	}
	if r.webLink("hz") == "" {
		// no web app configured: keep every native screen reachable
		kb := [][]Btn{
			{btnS("🏠 صفحهٔ اصلی", "menu", "primary")},
			{btnS("➕ افزودن رکورد", "addrec", "plain"), btn("☁️ کلودفلر", "zones"), btnS("⚡ تغییر سریع", "qamenu", "plain")},
			{btnS("🇩🇪 هتزنر", "hz", "success"), btnS("🇮🇷 آروان", "arvan", "success")},
			{btnS("🖥 سرورها و مانیتورینگ", "srv", "plain"), btnS("⏰ یادآورها", "rem", "plain")},
			{btnS("📦 بخش‌های دیگر", "more", "plain")},
			{siteBtn},
		}
		r.Show(chat, mid, r.menuText(), kb)
		return
	}
	// same row layout as the worker edition's home screen
	kb := [][]Btn{
		{r.webBtnS("➕ افزودن رکورد", "addrec", "rec", "plain"), r.webBtnS("☁️ کلودفلر", "zones", "cf", "primary"), r.webBtnS("⚡ تغییر سریع", "qamenu", "rec", "plain")},
		{r.webBtnS("🇩🇪 هتزنر", "hz", "hz", "success"), r.webBtnS("🇮🇷 آروان", "arvan", "arv", "success")},
		{r.webBtnS("🎛 پنل پاسارگارد", "pndef", "pg", "primary")},
		{r.webBtnS("🧭 تعویض خودکار ساب‌فیلتر", "hf", "hf", "primary"), r.webBtnS("🔁 چرخش روزانه ساب‌ها", "hfd", "hfd", "primary")},
		{r.webBtnS("🔥 گزارش بد مصرف پاسارگارد", "um", "um", "primary"), r.webBtnS("🔔 هشدار انقضا و حجم", "pghook", "hook", "primary")},
		{r.webBtnS("🖥 سرورها و مانیتورینگ", "srv", "srv", "success"), r.webBtnS("🌐 دامنه و SSL", "dommon", "dom", "success")},
		{r.webBtnS("⏰ یادآورها", "rem", "rem", "primary"), r.webBtnS("✉️ ایمیل دامنه", "fmail", "mail", "primary")},
		{r.webBtnS("⚙️ تنظیمات و راهنما", "settings", "set", "success")},
		{siteBtn},
	}
	r.Show(chat, mid, r.menuText(), kb)
}

func (r *Router) menuText() string {
	text := uiTitle("⚙️", "نگهبان ابری", "") + fmt.Sprintf("\n\nنسخه v%s", Version) +
		"\n📣 کانال: @" + ChannelHandle
	if p := r.PromoText(); p != "" {
		text += "\n\n" + p
	}
	return text
}

// moreMenu keeps the screens that have no mini app page yet.
func (r *Router) moreMenu(chat, mid int64) {
	kb := [][]Btn{
		{r.webBtn("➕ افزودن رکورد", "addrec", "rec"), r.webBtn("⚡ تغییر سریع", "qamenu", "rec")},
		{r.webBtn("🎛 پنل پاسارگارد", "pndef", "pg")},
		{r.webBtn("🧭 تعویض خودکار ساب فیلتر", "hf", "hf"), r.webBtn("📡 مانیتور نود پاسارگارد", "nd", "pg")},
		{r.webBtn("🔔 هشدار انقضا و حجم", "pghook", "hook"), r.webBtn("🔥 گزارش بد مصرف پاسارگارد", "um", "um")},
		{r.webBtn("🌐 مانیتور دامنه و ترافیک‌ها", "dommon", "dom")},
		{r.webBtn("✉️ ایمیل دامنه", "fmail", "mail"), r.webBtn("⚖️ لود بالانسر IP", "lb", "cf")},
		{r.webBtn("⚙️ تنظیمات و راهنما و ...", "settings", "set")},
		{btn("🏠 خانه", "menu")},
	}
	r.Show(chat, mid, uiTitle("📦", "بخش‌های دیگر", ""), kb)
}

func (r *Router) settingsHome(chat, mid int64) {
	rows := [][]Btn{
		row(btnS("📖 راهنمای بخش‌ها", "help", "primary")),
		row(btnS("📱 راهنمای مینی‌اپ", "wahelp", "primary")),
		row(btnS("🔄 نسخه‌ها و آپدیت", "vers", "primary")),
		row(btnS("🛟 پشتیبان فیلور (Standby)", "fo", "primary")),
	}
	if r.IsMain(r.curFrom) {
		rows = append(rows, row(btnS("👥 مدیریت ادمین‌ها", "admins_menu", "primary")))
	}
	if r.IsMain(r.curFrom) && r.hubRoot() {
		rows = append(rows,
			row(btnS("📊 آمار نصب‌ها", "hubstats", "primary"), btnS("📢 پیام همگانی", "hubcast", "success")),
			row(btnS("📋 اعلان‌ها", "hubann", "plain"), btnS("📢 تبلیغ منوی اصلی", "promo", "primary")),
			row(btnS("⚙️ تنظیم هاب", "hubcfg", "plain")))
	}
	rows = append(rows, row(btnS("📞 ارتباط با سازنده", "hubcontact", "plain")))
	kb := stdKB("menu", rows...)
	r.Show(chat, mid,
		uiBody(uiTitle("⚙️", "تنظیمات و راهنما", ""),
			"",
			"🤖 نسخه سرور: <code>"+Version+"</code>",
			"اجرا کامل روی سرور خودت؛ بدون KV و ورکر."), kb)
}

var guides = map[string][2]string{
	"cf":    {"☁️ کلودفلر", "دامنه‌ها را ببین، رکورد بساز/ویرایش/حذف کن، پروکسی و TTL را درجا عوض کن. جست‌وجو در همه رکوردها هم هست."},
	"rem":   {"⏰ یادآورها", "متن + تاریخ شمسی (مثل 1404/06/20 14:30) بده تا سر ساعت خبرت کنم."},
	"vers":  {"🔄 نسخه‌ها", "نسخهٔ فعلی و آخرین نسخه را نشان می‌دهد و آپدیت از داخل تلگرام."},
	"arvan": {"🇮🇷 آروان", "دامنه‌ها و رکوردهای DNS آروان + ابری (CDN)."},
	"hz":    {"🇩🇪 هتزنر", "سرورها را ببین، روشن/خاموش/ریبوت کن، رمز ریست کن، اسنپ‌شات بگیر، IPv4 جایگزین کن، سرور بساز."},
	"srv":   {"🖥 سرورها و مانیتورینگ", "مدیریت سرورهای لینوکسی با SSH، آمار CPU/RAM/دیسک و هشدار خودکار."},
}

var guideOrder = []string{"cf", "rem", "vers", "arvan", "hz", "srv"}

func (r *Router) helpList(chat, mid int64) {
	var items []Btn
	for _, k := range guideOrder {
		if v, ok := guides[k]; ok {
			items = append(items, btnS(v[0], "help:"+k, "plain"))
		}
	}
	kb := gridBTN(items, 2)
	kb = append(kb, navRow("settings"))
	r.Show(chat, mid, "📖 راهنمای بخش‌ها:", kb)
}

func (r *Router) helpShow(chat, mid int64, key string) {
	g, ok := guides[key]
	if !ok {
		r.Show(chat, mid, "❌", HomeKB())
		return
	}
	r.Show(chat, mid, g[0]+"\n\n"+g[1],
		navKB("help"))
}
