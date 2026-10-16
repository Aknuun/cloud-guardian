package router

import "fmt"

// Menu renders the home menu.
func (r *Router) Menu(chat, mid int64) {
	kb := [][]Btn{
		{btnS("➕ افزودن رکورد", "addrec", "plain"), btn("☁️ کلودفلر", "zones"), btnS("🔍 جست و جو", "search", "plain"), btnS("⚡ تغییر سریع", "qamenu", "plain")},
		{btnS("🟢 لینود", "ln", "success"), btnS("🇩🇪 هتزنر", "hz", "success"), btnS("🇮🇷 آروان", "arvan", "success")},
		{btnS("🎛 پنل پاسارگارد", "pndef", "primary")},
		{btn("🧭 تعویض خودکار ساب فیلتر", "hf"), btn("📡 مانیتور نود پاسارگارد", "nd")},
		{btn("🔔 هشدار انقضا و حجم", "pghook"), btn("🔥 گزارش بد مصرف پاسارگارد", "um")},
		{btnS("🖥 سرورها و مانیتورینگ", "srv", "plain"), btnS("🌐 مانیتور دامنه و ترافیک‌ها", "dommon", "plain")},
		{btnS("⏰ یادآورها", "rem", "plain"), btnS("✉️ ایمیل دامنه", "fmail", "plain"), btnS("⚖️ لود بالانسر IP", "lb", "plain")},
		{btnS("⚙️ تنظیمات و راهنما و ...", "settings", "success")},
	}
	r.Show(chat, mid,
		fmt.Sprintf("به ربات نگهبان ابری خوش اومدی ⚙️ نسخه v%s", Version),
		kb)
}

func (r *Router) settingsHome(chat, mid int64) {
	kb := [][]Btn{
		{btn("📖 راهنمای بخش‌ها", "help")},
		{btn("🔄 نسخه‌ها و آپدیت", "vers")},
	}
	if r.IsMain(r.curFrom) {
		kb = append(kb, []Btn{btn("👥 مدیریت ادمین‌ها", "admins_menu")})
		kb = append(kb,
			[]Btn{btnS("📊 آمار نصب‌ها", "hubstats", "primary"), btnS("📢 پیام همگانی", "hubcast", "success")},
			[]Btn{btn("📋 اعلان‌ها", "hubann"), btn("⚙️ تنظیم هاب", "hubcfg")})
	}
	kb = append(kb, []Btn{btn("📞 ارتباط با سازنده", "hubcontact")})
	kb = append(kb, []Btn{btn("🔙 بازگشت", "menu"), btn("🏠 خانه", "menu")})
	r.Show(chat, mid,
		fmt.Sprintf("⚙️ تنظیمات و راهنما\n\n🤖 نسخه سرور: <code>%s</code>\n"+
			"اجرا کامل روی سرور خودت؛ بدون KV و ورکر.", Version), kb)
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

func (r *Router) helpList(chat, mid int64) {
	kb := [][]Btn{}
	for k, v := range guides {
		kb = append(kb, []Btn{btn(v[0], "help:"+k)})
	}
	kb = append(kb, []Btn{btn("🔙 بازگشت", "settings"), btn("🏠 خانه", "menu")})
	r.Show(chat, mid, "📖 راهنمای بخش‌ها:", kb)
}

func (r *Router) helpShow(chat, mid int64, key string) {
	g, ok := guides[key]
	if !ok {
		r.Show(chat, mid, "❌", HomeKB())
		return
	}
	r.Show(chat, mid, g[0]+"\n\n"+g[1],
		[][]Btn{{btn("🔙 بازگشت", "help"), btn("🏠 خانه", "menu")}})
}
