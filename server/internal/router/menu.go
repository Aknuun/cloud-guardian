package router

import "fmt"

// Menu renders the home menu.
func (r *Router) Menu(chat, mid int64) {
	kb := stdKB("",
		row(btnS("➕ افزودن رکورد", "addrec", "success"), btnS("☁️ کلودفلر", "zones", "plain"), btnS("⚡ تغییر سریع", "qamenu", "primary")),
		row(btnS("🟢 لینود", "ln", "plain"), btnS("🇩🇪 هتزنر", "hz", "plain"), btnS("🇮🇷 آروان", "arvan", "plain")),
		row(btnS("🎛 پنل پاسارگارد", "pndef", "plain")),
		row(btnS("🧭 تعویض خودکار ساب فیلتر", "hf", "plain"), btnS("📡 مانیتور نود پاسارگارد", "nd", "plain")),
		row(btnS("🔔 هشدار انقضا و حجم", "pghook", "plain"), btnS("🔥 گزارش بد مصرف پاسارگارد", "um", "plain")),
		row(btnS("🖥 سرورها و مانیتورینگ", "srv", "plain"), btnS("🌐 مانیتور دامنه و ترافیک‌ها", "dommon", "plain")),
		row(btnS("⏰ یادآورها", "rem", "plain"), btnS("✉️ ایمیل دامنه", "fmail", "plain"), btnS("⚖️ لود بالانسر IP", "lb", "plain")),
		row(btnS("⚙️ تنظیمات و راهنما و ...", "settings", "plain")),
	)
	text := uiTitle("⚙️", "نگهبان ابری", "") + fmt.Sprintf("\n\nنسخه v%s", Version)
	if p := r.PromoText(); p != "" {
		text += "\n\n" + p
	}
	r.Show(chat, mid, text, kb)
}

func (r *Router) settingsHome(chat, mid int64) {
	rows := [][]Btn{
		row(btnS("📖 راهنمای بخش‌ها", "help", "primary")),
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
	"ln":    {"🟢 لینود", "مثل هتزنر: لیست، پاور، ریبوت، تغییرنام، حذف."},
	"hz":    {"🇩🇪 هتزنر", "سرورها را ببین، روشن/خاموش/ریبوت کن، رمز ریست کن، اسنپ‌شات بگیر، IPv4 جایگزین کن، سرور بساز."},
	"srv":   {"🖥 سرورها و مانیتورینگ", "مدیریت سرورهای لینوکسی با SSH، آمار CPU/RAM/دیسک و هشدار خودکار."},
}

var guideOrder = []string{"cf", "rem", "vers", "arvan", "ln", "hz", "srv"}

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
