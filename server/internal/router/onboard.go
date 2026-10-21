package router

import (
	"fmt"
	"strings"
)

// onboardPrompt sends a one-time first-start guide to every admin. It is only
// used by the Go server edition; the Cloudflare Worker edition has no mini app.
func (r *Router) onboardPrompt(chat, frm int64) {
	webKey := fmt.Sprintf("onboard:webapp:%d", frm)
	showWeb := r.Store.Get(webKey) != "1"
	showFo := r.IsMain(frm) && !r.foConfigured() && r.Store.Get("fo:prompted") != "1"
	if !showWeb && !showFo {
		return
	}

	webURL := strings.TrimSpace(r.WebAppURL)
	webOK := strings.HasPrefix(webURL, "https://")

	var b strings.Builder
	if showWeb {
		b.WriteString("🚀 <b>مینی‌اپ نگهبان ابری</b>\n\n")
		if webOK {
			b.WriteString("از دکمهٔ «هتزنر» کنار کادر پیام یا دکمهٔ زیر بازش کن.\n")
			b.WriteString("همهٔ کارهای هتزنر و کلادفلر داخل مینی‌اپ است: سرورها، رکوردهای DNS، تنظیمات دامنه، ایمیل، ترافیک، جست‌وجو و لود بالانسر.\n")
			b.WriteString("برای دسترسی همیشگی، دکمه را Pin کن.\n\n")
		} else {
			b.WriteString("مینی‌اپ روی این نصب فعال نشده است.\n")
			b.WriteString("برای فعال‌سازی SSL و مینی‌اپ، روی سرور این دستور را اجرا کن:\n")
			b.WriteString("<code>bash install-server.sh webapp</code>\n\n")
		}
	}
	if showFo {
		if showWeb {
			b.WriteString("────────────\n\n")
		}
		b.WriteString("🛟 <b>پشتیبان فیلور (Standby)</b>\n\n")
		b.WriteString("اگر سرورت قطع شود، یک ورکر رایگان کلادفلر موقتاً جای ربات را می‌گیرد تا سرور برگردد.\n")
		b.WriteString("همین حالا تنظیمش کن یا بعداً از «تنظیمات».\n")
	}

	var kb [][]Btn
	if showWeb && webOK {
		if chat > 0 {
			kb = append(kb, []Btn{btnWA("🚀 باز کردن مینی‌اپ", webURL)})
		} else {
			kb = append(kb, []Btn{btnURL("🚀 باز کردن مینی‌اپ", webURL)})
		}
	}
	if showFo {
		kb = append(kb, []Btn{btnS("🛟 راه‌اندازی فیلور", "fo", "success")})
	}
	kb = append(kb, []Btn{btn("بستن", "onboard_skip")})

	r.Bot.Send(chat, b.String(), kb)
	if showWeb {
		r.Store.Put(webKey, "1", 0)
	}
	if showFo {
		r.Store.Put("fo:prompted", "1", 0)
	}
}
