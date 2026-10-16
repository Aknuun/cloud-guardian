package router

import "strings"

// BotCommand mirrors one entry of Telegram's setMyCommands list.
type BotCommand struct {
	Command     string `json:"command"`
	Description string `json:"description"`
}

// slashTargets maps a slash-command to the callback data that opens its menu.
// Commands already handled explicitly in onMessage (start/search/myid/admins/
// hub/contact) are intentionally absent here.
var slashTargets = map[string]string{
	"menu":        "menu",
	"cloudflare":  "zones",
	"addrecord":   "addrec",
	"favorites":   "favs",
	"hetzner":     "hz",
	"linode":      "ln",
	"arvan":       "arvan",
	"servers":     "srv",
	"panels":      "pndef",
	"domains":     "dommon",
	"hostfilter":  "hf",
	"node":        "nd",
	"reminders":   "rem",
	"quota":       "quota",
	"ssl":         "sslm",
	"traffic":     "traf",
	"loadbalance": "lb",
	"email":       "fmail",
	"alerts":      "pghook",
	"usage":       "um",
	"quick":       "qamenu",
	"settings":    "settings",
	"help":        "help",
}

// BotCommands is the list registered via setMyCommands on startup.
func BotCommands() []BotCommand {
	return []BotCommand{
		{"start", "شروع و منوی اصلی"},
		{"menu", "منوی اصلی"},
		{"cloudflare", "دامنه‌ها و DNS کلودفلر"},
		{"addrecord", "افزودن رکورد DNS"},
		{"search", "جست‌وجو در رکوردها"},
		{"favorites", "ساب‌های منتخب"},
		{"hetzner", "هتزنر: مدیریت سرور"},
		{"linode", "لینود: مدیریت سرور"},
		{"arvan", "آروان: دامنه و سرور ابری"},
		{"servers", "سرورها و مانیتورینگ"},
		{"panels", "پنل پاسارگارد"},
		{"domains", "مانیتور دامنه · SSL · ترافیک"},
		{"hostfilter", "تعویض خودکار ساب فیلتر"},
		{"node", "مانیتور نود پاسارگارد"},
		{"reminders", "یادآورها"},
		{"quota", "سهمیه کلادفلر"},
		{"traffic", "ترافیک ساب‌ها"},
		{"loadbalance", "لود بالانسر IP"},
		{"email", "ایمیل دامنه"},
		{"alerts", "هشدار انقضا و حجم"},
		{"usage", "گزارش بدمصرف پاسارگارد"},
		{"quick", "تغییر سریع"},
		{"settings", "تنظیمات و راهنما"},
		{"help", "راهنما"},
		{"myid", "شناسهٔ تلگرام من"},
		{"contact", "ارتباط با سازنده"},
	}
}

// slashCmd extracts "/hetzner@bot /x" → "hetzner".
func slashCmd(text string) string {
	s := strings.TrimPrefix(strings.TrimSpace(text), "/")
	if i := strings.IndexAny(s, " \t@"); i >= 0 {
		s = s[:i]
	}
	return strings.ToLower(s)
}
