package router

import (
	"fmt"
	"strconv"
	"strings"

	"github.com/aknuun/cg-server/internal/um"
)

func (r *Router) umHome(chat, mid int64) {
	cfg := um.Cfg(r.Store)
	on := "خاموش ❌"
	if cfg.Enabled {
		on = "روشن ✅"
	}
	rep := "خاموش ❌"
	if cfg.Report {
		rep = "روشن ✅"
	}
	txt := strings.Join([]string{
		"🔥 گزارش بد مصرف پاسارگارد", "",
		"کاربرانی که لینکشان پخش شده و بی‌رویه مصرف می‌کنند را پیدا می‌کند.",
		"هر ۶۰ دقیقه از API پنل خوانده و با مصرف ~۱ ساعت و ~۲۴ ساعت قبل مقایسه می‌شود.",
		"الگوی مصرف هر هشدار (پیوسته/جهشی) هم مشخص می‌شود.", "",
		"وضعیت: " + on,
		fmt.Sprintf("⏱ آستانهٔ ساعتی: %g GB", cfg.HourlyGb),
		fmt.Sprintf("📅 آستانهٔ روزانه: %g GB", cfg.DailyGb),
		fmt.Sprintf("🔁 فاصلهٔ تکرار هشدار هر کاربر: %g ساعت", cfg.CooldownH),
		fmt.Sprintf("🧮 حداقل مصرف برای اسنپ‌شات: %g GB", cfg.MinGb),
		"📊 گزارش روزانهٔ اخطارشده‌ها: " + rep, "",
		"هشدار فقط برای ادمین ارسال می‌شود.",
	}, "\n")
	toggle := "▶️ روشن کن"
	if cfg.Enabled {
		toggle = "⏸ خاموش کن"
	}
	repBtn := "📊 گزارش: خاموش"
	if cfg.Report {
		repBtn = "📊 گزارش: روشن"
	}
	kb := [][]Btn{
		{btn(toggle, "um:toggle")},
		{btn(fmt.Sprintf("⏱ ساعتی: %gGB", cfg.HourlyGb), "um:h"),
			btn(fmt.Sprintf("📅 روزانه: %gGB", cfg.DailyGb), "um:d")},
		{btn(fmt.Sprintf("🔁 فاصلهٔ هشدار: %gh", cfg.CooldownH), "um:c")},
		{btn(fmt.Sprintf("🧮 حداقل اسنپ‌شات: %gGB", cfg.MinGb), "um:min"),
			btn(repBtn, "um:report")},
		{btn("🏠 خانه", "menu")},
	}
	r.Show(chat, mid, txt, kb)
}

func (r *Router) umToggle(chat, mid int64) {
	cfg := um.Cfg(r.Store)
	cfg.Enabled = !cfg.Enabled
	um.SaveCfg(r.Store, cfg)
	r.umHome(chat, mid)
}

func (r *Router) umReport(chat, mid int64) {
	cfg := um.Cfg(r.Store)
	cfg.Report = !cfg.Report
	um.SaveCfg(r.Store, cfg)
	r.umHome(chat, mid)
}

func (r *Router) umField(chat, mid int64, field, label string) {
	r.setWizard(chat, map[string]any{"kind": "um_num", "key": field})
	r.Show(chat, mid, "🔢 "+label+" را بفرستید:",
		[][]Btn{{btn("🔙 انصراف", "um")}})
}

func (r *Router) umNumSave(chat, mid int64, pend map[string]any, text string) {
	key, _ := pend["key"].(string)
	v, err := strconv.ParseFloat(strings.TrimSpace(text), 64)
	if err != nil || v < 0 {
		r.Bot.Send(chat, "❌ عدد نامعتبر.", nil)
		return
	}
	r.clearWizard(chat)
	cfg := um.Cfg(r.Store)
	switch key {
	case "hourlyGb":
		cfg.HourlyGb = v
	case "dailyGb":
		cfg.DailyGb = v
	case "cooldownH":
		cfg.CooldownH = v
	case "minGb":
		cfg.MinGb = v
	case "activeMin":
		cfg.ActiveMin = v
	}
	um.SaveCfg(r.Store, cfg)
	r.umHome(chat, 0)
}
