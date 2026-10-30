package router

import (
	"fmt"
	"strings"
)

// linkPrompt asks an admin to connect their panel account so their numeric
// Telegram id is registered in PasarGuard (alerts then reach them directly).
func (r *Router) linkPrompt(chat, frm int64) {
	if r.Store.Get(fmt.Sprintf("linked:%d", frm)) == "1" {
		return
	}
	panels := r.panels()
	if len(panels) == 0 {
		return
	}
	r.setWizard(chat, map[string]any{"kind": "link_user", "panel": 0})
	r.Show(chat, 0,
		"🔗 <b>اتصال حساب پنل</b>\n\n"+
			"برای اینکه هشدار حجم و انقضا مستقیم به تلگرام خودت بیاید، "+
			"نام کاربری حسابت در پنل پاسارگارد را بفرست.\n\n"+
			"اگر نمی‌خواهی، «رد کردن» را بزن.",
		[][]Btn{{btn("❌ رد کردن", "link_skip")}})
}

func (r *Router) linkUserSave(chat, mid int64, text string) {
	u := strings.TrimSpace(text)
	if u == "" || strings.ContainsAny(u, " \t") {
		r.Show(chat, mid, "❌ نام کاربری نامعتبر است. دوباره بفرست:", [][]Btn{{btn("❌ لغو", "link_skip")}})
		return
	}
	r.setWizard(chat, map[string]any{"kind": "link_pass", "user": u})
	r.Show(chat, mid, "🔑 رمز عبور همان حساب را بفرست:", [][]Btn{{btn("❌ لغو", "link_skip")}})
}

func (r *Router) linkPassSave(chat, mid int64, pend map[string]any, text string) {
	user, _ := pend["user"].(string)
	pass := strings.TrimSpace(text)
	panels := r.panels()
	if user == "" || pass == "" || len(panels) == 0 {
		r.clearWizard(chat)
		r.Show(chat, mid, "❌ اتصال انجام نشد.", HomeKB())
		return
	}
	p := panels[0]
	cli := p.Client()
	tok, err := cli.Login()
	if err != nil {
		r.clearWizard(chat)
		r.Show(chat, mid, "❌ ورود به پنل ناموفق بود؛ رمز یا آدرس پنل را بررسی کن.", HomeKB())
		return
	}
	if err := cli.AdminSetTelegram(tok, user, chat); err != nil {
		r.clearWizard(chat)
		r.Show(chat, mid, "⚠️ ورود موفق بود ولی ثبت شناسه ناموفق: "+Esc(err.Error()), HomeKB())
		return
	}
	r.clearWizard(chat)
	r.Store.Put(fmt.Sprintf("linked:%d", chat), "1", 0)
	r.Bot.Send(chat, fmt.Sprintf("✅ حساب «%s» به شناسهٔ <code>%d</code> وصل شد.\nاز این به بعد هشدارهای حجم و انقضا برای خودت می‌آید.", Esc(user), chat), nil)
	r.Menu(chat, 0)
}
