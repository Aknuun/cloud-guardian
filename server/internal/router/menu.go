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
	if u := r.webLink(goID); u != "" {
		return btnWA(label, u)
	}
	return btnS(label, native, "plain")
}

// Menu renders the home menu. Everything except Cloudflare opens in the mini app
// (the bot itself only sends reports); the rest stays reachable from "بیشتر".
func (r *Router) Menu(chat, mid int64) {
	kb := [][]Btn{
		{btn("☁️ کلودفلر", "zones")},
		{btnWA("🇩🇪 هتزنر", r.webLink("hz")), btnWA("🇮🇷 آروان", r.webLink("arv"))},
		{btnWA("🖥 سرورها و مانیتورینگ", r.webLink("srv")), btnWA("⏰ یادآورها", r.webLink("rem"))},
		{btnWA("🎛 پاسارگارد", r.webLink("pg"))},
		{btnS("📦 بخش‌های دیگر", "more", "plain")},
		{chanBtn()},
	}
	if r.webLink("hz") == "" {
		// no web app configured: keep the native screens usable
		kb = [][]Btn{
			{btnS("➕ افزودن رکورد", "addrec", "plain"), btn("☁️ کلودفلر", "zones"), btnS("⚡ تغییر سریع", "qamenu", "plain")},
			{btnS("🇩🇪 هتزنر", "hz", "success"), btnS("🇮🇷 آروان", "arvan", "success")},
			{btnS("🖥 سرورها و مانیتورینگ", "srv", "plain"), btnS("⏰ یادآورها", "rem", "plain")},
			{btnS("📦 بخش‌های دیگر", "more", "plain")},
			{chanBtn()},
		}
	}
	text := uiTitle("⚙️", "نگهبان ابری", "") + fmt.Sprintf("\n\nنسخه v%s", Version) +
		"\n📣 کانال: @" + ChannelHandle
	if p := r.PromoText(); p != "" {
		text += "\n\n" + p
	}
	r.Show(chat, mid, text, kb)
}

// moreMenu keeps the screens that have no mini app page yet.
func (r *Router) moreMenu(chat, mid int64) {
	kb := [][]Btn{
		{btnS("➕ افزودن رکورد", "addrec", "plain"), btnS("⚡ تغییر سریع", "qamenu", "plain")},
		{btnS("🎛 پنل پاسارگارد", "pndef", "primary")},
		{btn("🧭 تعویض خودکار ساب فیلتر", "hf"), btn("📡 مانیتور نود پاسارگارد", "nd")},
		{btn("🔔 هشدار انقضا و حجم", "pghook"), btn("🔥 گزارش بد مصرف پاسارگارد", "um")},
		{btnS("🌐 مانیتور دامنه و ترافیک‌ها", "dommon", "plain")},
		{btnS("✉️ ایمیل دامنه", "fmail", "plain"), btnS("⚖️ لود بالانسر IP", "lb", "plain")},
		{btnS("⚙️ تنظیمات و راهنما و ...", "settings", "success")},
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
