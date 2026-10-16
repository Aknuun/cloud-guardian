package router

import (
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/aknuun/cg-server/internal/cf"
)

const defaultQuotaLimit = 100000

type quotaConfig struct {
	Limit           int   `json:"limit"`
	AutoStop        bool  `json:"autoStop"`
	Alerts          bool  `json:"alerts"`
	CronPausedUntil int64 `json:"cronPausedUntil"`
}

func (r *Router) quotaCfg() quotaConfig {
	c := quotaConfig{Limit: defaultQuotaLimit, AutoStop: true, Alerts: true}
	var raw map[string]any
	if r.Store.GetJSON("quota_cfg", &raw) && raw != nil {
		if n := intOfAny(raw["limit"]); n > 0 {
			c.Limit = n
		}
		if v, ok := raw["autoStop"].(bool); ok {
			c.AutoStop = v
		}
		if v, ok := raw["alerts"].(bool); ok {
			c.Alerts = v
		}
		c.CronPausedUntil = int64(intOfAny(raw["cronPausedUntil"]))
	}
	return c
}

func (r *Router) quotaSave(c quotaConfig) { r.Store.PutJSON("quota_cfg", c, 0) }

// QuotaPaused reports whether background cron jobs are currently paused.
func (r *Router) QuotaPaused() bool {
	return r.quotaCfg().CronPausedUntil > time.Now().UnixMilli()
}

func onOff(b bool) string {
	if b {
		return "روشن"
	}
	return "خاموش"
}

func kvNum(kv map[string]int, k string) any {
	if v, ok := kv[k]; ok {
		return v
	}
	return "?"
}

func (r *Router) quotaUsageLines(limit int) []string {
	accs := r.cfAccs()
	if len(accs) == 0 {
		return []string{"❌ اکانت کلادفلر تنظیم نشده."}
	}
	lines := []string{"☁️ سهمیهٔ کلادفلر (امروز)", ""}
	for _, a := range accs {
		u := cf.AccountUsage(a.Token)
		kv, _ := u["kv"].(map[string]int)
		lines = append(lines, "• "+a.Name+":")
		lines = append(lines, fmt.Sprintf("  🚦 درخواست‌ها: %v / %d", u["requests"], limit))
		lines = append(lines, fmt.Sprintf("  🗄 KV خواندن: %v / ۱۰۰٬۰۰۰", kvNum(kv, "read")))
		lines = append(lines, fmt.Sprintf("  ✍️ KV نوشتن: %v / ۱٬۰۰۰", kvNum(kv, "write")))
		lines = append(lines, fmt.Sprintf("  🗑 KV حذف: %v / ۱٬۰۰۰", kvNum(kv, "delete")))
	}
	return lines
}

func (r *Router) quotaHome(chat, mid int64) {
	cfg := r.quotaCfg()
	now := time.Now().UnixMilli()
	paused := cfg.CronPausedUntil > now
	lines := r.quotaUsageLines(cfg.Limit)
	lines = append(lines, "", "🔒 استاپ خودکار: "+onOff(cfg.AutoStop), "🔔 هشدارها: "+onOff(cfg.Alerts))
	if paused {
		lines = append(lines, fmt.Sprintf("⏸ کرون‌جاب‌ها متوقف — %d دقیقهٔ دیگر", (cfg.CronPausedUntil-now)/60000))
	}
	autoLabel := "🔒 روشن‌کردن استاپ خودکار"
	if cfg.AutoStop {
		autoLabel = "🔓 خاموش‌کردن استاپ خودکار"
	}
	kb := [][]Btn{
		{btn("🔢 تغییر سقف روزانه", "qsetlimit")},
		{btn(autoLabel, "qtgauto")},
		{btn("🔔 هشدارها", "qalerts")},
		{btn("📊 کرون‌جاب‌ها", "qcrons")},
	}
	if len(r.panels()) > 0 {
		kb = append(kb, []Btn{btn("📡 نودها", "nd")})
	}
	kb = append(kb, []Btn{btn("🔙 بازگشت", "dommon"), btn("🏠 خانه", "menu")})
	r.Show(chat, mid, strings.Join(lines, "\n"), kb)
}

func (r *Router) quotaCrons(chat, mid int64) {
	cfg := r.quotaCfg()
	now := time.Now().UnixMilli()
	paused := cfg.CronPausedUntil > now
	txt := "📊 کرون‌جاب‌ها فعال است."
	if paused {
		txt = fmt.Sprintf("📊 کرون‌جاب‌ها متوقف است — %d دقیقهٔ دیگر.", (cfg.CronPausedUntil-now)/60000)
	}
	var kb [][]Btn
	if paused {
		kb = append(kb, []Btn{btn("▶️ روشن‌کردن کرون‌جاب‌ها", "qcron:0")})
	} else {
		kb = append(kb, []Btn{btn("⏸ توقف ۶ ساعت", "qcron:6"), btn("⏸ توقف ۲۴ ساعت", "qcron:24")})
	}
	kb = append(kb, []Btn{btn("🔙 بازگشت", "quota"), btn("🏠 خانه", "menu")})
	r.Show(chat, mid, txt, kb)
}

func (r *Router) quotaSetLimit(chat, mid int64) {
	r.setWizard(chat, map[string]any{"kind": "quota_limit"})
	r.Show(chat, mid, "🔢 سقف درخواست روزانه را بفرستید (عدد):",
		[][]Btn{{btn("🔙 بازگشت", "quota"), btn("🏠 خانه", "menu")}})
}

func (r *Router) quotaLimitSave(chat, mid int64, text string) {
	sd := strings.TrimSpace(jalaliFA(text))
	sd = strings.ReplaceAll(strings.ReplaceAll(sd, ",", ""), "٬", "")
	if !isDigits(sd) {
		r.Bot.Send(chat, "❌ عدد نامعتبر. یک عدد مثبت بفرستید:", nil)
		return
	}
	n, err := strconv.Atoi(sd)
	if err != nil || n <= 0 {
		r.Bot.Send(chat, "❌ عدد نامعتبر. یک عدد مثبت بفرستید:", nil)
		return
	}
	cfg := r.quotaCfg()
	cfg.Limit = n
	r.quotaSave(cfg)
	r.clearWizard(chat)
	r.Bot.Send(chat, fmt.Sprintf("✅ سقف روزانه روی %d تنظیم شد.", n), nil)
	r.quotaHome(chat, 0)
}

// QuotaAlert returns an over-limit warning text (once/day) or "".
func (r *Router) QuotaAlert() string {
	cfg := r.quotaCfg()
	if !cfg.Alerts {
		return ""
	}
	warn := int(float64(cfg.Limit) * 0.9)
	var hits []string
	for _, a := range r.cfAccs() {
		u := cf.AccountUsage(a.Token)
		rq, ok := u["requests"].(int)
		if !ok || rq < warn {
			continue
		}
		hits = append(hits, fmt.Sprintf("• %s: %d / %d", a.Name, rq, cfg.Limit))
	}
	if len(hits) == 0 {
		return ""
	}
	day := time.Now().UTC().Format("2006-01-02")
	if v, ok := r.Store.GetOK("quota_warned_day"); ok && v == day {
		return ""
	}
	r.Store.Put("quota_warned_day", day, 0)
	msg := "⚠️ هشدار سهمیهٔ کلادفلر\n" + strings.Join(hits, "\n")
	if cfg.AutoStop {
		msg += "\n\n🔒 استاپ خودکار روشن است."
	}
	return msg
}
