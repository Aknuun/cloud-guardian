package router

import (
	"fmt"
	"strconv"
	"strings"

	"github.com/aknuun/cg-server/internal/sslmon"
)

func (r *Router) dommonHome(chat, mid int64) {
	r.Show(chat, mid, "🌐 مانیتور دامنه و ترافیک‌ها\n\nیک بخش را انتخاب کنید:", [][]Btn{
		{btn("🔐 مانیتور SSL", "sslm"), btn("🗓 مانیتور انقضای دامنه", "domexp")},
		{btn("📊 ترافیک ساب‌ها", "traf"), btn("☁️ سهمیهٔ کلادفلر", "quota")},
		{btn("🏠 خانه", "menu")},
	})
}

func (r *Router) sslmHome(chat, mid int64) {
	mons := sslmon.GetMonitors(r.Store)
	lines := []string{"🔐 مانیتور گواهی SSL", ""}
	if len(mons) == 0 {
		lines = append(lines, "📭 دامنه‌ای برای نظارت ثبت نشده.\nبرای هشدار هنگام نزدیک شدن به انقضا (پیش‌فرض ۵ روز)، یک دامنه اضافه کنید.")
	} else {
		for _, it := range sslmon.CheckAll(r.Store) {
			lines = append(lines, sslmon.ResultLine(it, 5))
		}
	}
	kb := [][]Btn{{btn("➕ افزودن دامنه", "ssdma")}}
	if len(mons) > 0 {
		kb = append(kb, []Btn{btn("🔄 بررسی دوباره", "sslm"), btn("🗑 حذف دامنه", "sslmd")})
	}
	kb = append(kb, []Btn{btn("🔙 بازگشت", "dommon"), btn("🏠 خانه", "menu")})
	r.Show(chat, mid, strings.Join(lines, "\n"), kb)
}

func (r *Router) sslAdd(chat, mid int64) {
	r.setWizard(chat, map[string]any{"kind": "ssl_host"})
	r.Show(chat, mid,
		"🔐 دامنه یا ساب‌دامنه‌ای که می‌خواهید گواهی‌اش نظارت شود را بفرستید (مثلاً example.com):",
		[][]Btn{{btn("🔙 انصراف", "sslm")}})
}

func (r *Router) sslDelList(chat, mid int64) {
	mons := sslmon.GetMonitors(r.Store)
	if len(mons) == 0 {
		r.Show(chat, mid, "📭 دامنه‌ای ثبت نشده.",
			navKB("sslm"))
		return
	}
	var items []Btn
	for i, m := range mons {
		items = append(items, btnS("🗑 "+m.Host, "sslmdy:"+strconv.Itoa(i), "plain"))
	}
	kb := gridBTN(items, 2)
	kb = append(kb, navRow("sslm"))
	r.Show(chat, mid, "🗑 کدام دامنه از نظارت حذف شود؟", kb)
}

func (r *Router) sslDelDo(chat, mid int64, idx string) {
	mons := sslmon.GetMonitors(r.Store)
	i, err := strconv.Atoi(idx)
	if err != nil || i < 0 || i >= len(mons) {
		r.Show(chat, mid, "❌ دامنه پیدا نشد.", HomeKB())
		return
	}
	rm := mons[i]
	next := append(mons[:i:i], mons[i+1:]...)
	r.Store.PutJSON("ssl_monitor", next, 0)
	r.Bot.Send(chat, fmt.Sprintf("✅ «%s» از نظارت حذف شد.", rm.Host), nil)
	r.sslmHome(chat, 0)
}
