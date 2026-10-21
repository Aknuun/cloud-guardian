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

// webappHelp is the persistent "mini app" guide in Settings.
func (r *Router) webappHelp(chat, mid int64) {
	webURL := strings.TrimSpace(r.WebAppURL)
	webOK := strings.HasPrefix(webURL, "https://")

	var b strings.Builder
	b.WriteString("📱 <b>راهنمای مینی‌اپ</b>\n\n")
	b.WriteString("مینی‌اپ نسخهٔ سروری، رابط سریع و لمسی ربات است:\n")
	b.WriteString("• مدیریت سرورهای هتزنر: روشن/خاموش، ریبوت، رمز، اسنپ‌شات\n")
	b.WriteString("• کلادفلر: دامنه‌ها، رکوردهای DNS، پروکسی و TTL\n")
	b.WriteString("• تنظیمات دامنه، ایمیل، ترافیک، جست‌وجو و لود بالانسر\n")
	b.WriteString("• عملیات گروهی و منتخب‌ها\n\n")

	if webOK {
		b.WriteString("برای باز کردن، دکمهٔ «هتزنر» کنار کادر پیام را بزن یا از دکمهٔ زیر استفاده کن.\n")
		b.WriteString("برای دسترسی همیشگی، دکمه را Pin کن.\n\n")
		b.WriteString("نسخهٔ ورکر کلادفلر مینی‌اپ ندارد؛ این امکان فقط روی نسخهٔ سروری فعال است.")
	} else {
		b.WriteString("مینی‌اپ روی این نصب فعال نیست.\n")
		b.WriteString("برای فعال‌سازی SSL و مینی‌اپ، روی سرور این دستور را اجرا کن:\n")
		b.WriteString("<code>bash install-server.sh webapp</code>\n\n")
		b.WriteString("پورت‌های ۸۰ و ۹۴۴۳ باید باز باشند. این امکان فقط برای نسخهٔ سروری است و نسخهٔ ورکر مینی‌اپ ندارد.")
	}

	rows := [][]Btn{}
	if webOK {
		if chat > 0 {
			rows = append(rows, []Btn{btnWA("🚀 باز کردن مینی‌اپ", webURL)})
		} else {
			rows = append(rows, []Btn{btnURL("🚀 باز کردن مینی‌اپ", webURL)})
		}
	}
	r.Show(chat, mid, b.String(), stdKB("settings", rows...))
}
