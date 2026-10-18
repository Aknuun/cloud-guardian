package router

import (
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/aknuun/cg-server/internal/alerts"
	"github.com/aknuun/cg-server/internal/pg"
)

func randHex(n int) string {
	b := make([]byte, n)
	_, _ = rand.Read(b)
	return hex.EncodeToString(b)
}

func (r *Router) publicBase() string {
	if r.FO != nil {
		return strings.TrimRight(r.FO["public_url"], "/")
	}
	return ""
}

func pgTGUsername(token string) string {
	hc := &http.Client{Timeout: 10 * time.Second}
	resp, err := hc.Get("https://api.telegram.org/bot" + token + "/getMe")
	if err != nil {
		return ""
	}
	defer resp.Body.Close()
	b, _ := io.ReadAll(resp.Body)
	var d struct {
		Result struct {
			Username string `json:"username"`
		} `json:"result"`
	}
	if json.Unmarshal(b, &d) != nil {
		return ""
	}
	return d.Result.Username
}

func (r *Router) pghookHome(chat, mid int64) {
	cfg := alerts.GetCfg(r.Store)
	panels := r.panels()
	lines := []string{"🔔 هشدار انقضا و حجم پاسارگارد", ""}
	status := "⏸ غیرفعال (پیام نمی‌آید)"
	if cfg.Enabled {
		status = "▶️ فعال"
	}
	lines = append(lines, "وضعیت: "+status)
	lines = append(lines, "📅 آستانه‌ها در خود پنل تنظیم می‌شوند (پیشنهاد: روز ۱، حجم ۹۰٪).")
	lines = append(lines, "📦 ارسال جمعی پولی: حجم ساعتی یک‌جا، انقضا روزی یک‌بار؛ فقط بحران امروزی (۰ روز / ۱۰۰٪) فوری می‌آید.")
	lines = append(lines, fmt.Sprintf("🖥 پنل‌های متصل: %d از %d", len(cfg.Panels), len(panels)))
	lines = append(lines, "")
	lines = append(lines, "روش: در تنظیمات webhook هر پنل، آدرس اختصاصی‌اش را بگذار تا وقتی کاربری به آستانه رسید، پنل خودش خبر بده.")
	toggle := "▶️ فعال‌سازی"
	if cfg.Enabled {
		toggle = "⏸ غیرفعال‌سازی"
	}
	silent := "🔔 سایلنت من: خاموش"
	if cfg.SilentMe {
		silent = "🔕 سایلنت من: روشن"
	}
	kb := [][]Btn{{btn(toggle, "pgtoggle"), btn(silent, "pgsilent")}}
	for i, p := range panels {
		pid := p.URL
		reg := false
		if _, ok := cfg.Panels[pid]; ok {
			reg = true
		}
		mark := "⬜ "
		if reg {
			mark = "✅ "
		}
		kb = append(kb, []Btn{btn(fmt.Sprintf("%s⚙️ تنظیم %s", mark, truncRune(pname(p), 22)), fmt.Sprintf("pgpanel:%d", i))})
	}
	kb = append(kb, []Btn{btn("🏠 خانه", "menu")})
	r.Show(chat, mid, strings.Join(lines, "\n"), kb)
}

func (r *Router) pgPanel(chat, mid int64, i int) {
	panels := r.panels()
	if i < 0 || i >= len(panels) {
		r.Show(chat, mid, "❌ پنل پیدا نشد.",
			[][]Btn{{btn("🔙 هشدارها", "pghook"), btn("🏠 خانه", "menu")}})
		return
	}
	p := panels[i]
	pid := p.URL
	cfg := alerts.GetCfg(r.Store)
	pc := cfg.Panels[pid]
	if pc == nil || pc.Secret == "" {
		if pc == nil {
			pc = &alerts.PanelCfg{}
			cfg.Panels[pid] = pc
		}
		pc.Secret = randHex(12)
		alerts.SaveCfg(r.Store, cfg)
	}
	base := r.publicBase()
	if base == "" {
		base = "<server-url>"
	}
	url := base + "/pghook/" + pc.Secret
	dash := pc.Dashpath
	if dash == "" {
		dash = "/dashboard"
	}
	lines := []string{
		fmt.Sprintf("🔔 وبهوک پنل: %s", pname(p)), "",
		"📨 آدرس وبهوک:", "<code>" + Esc(url) + "</code>", "",
		"🔑 سکرت:", "<code>" + Esc(pc.Secret) + "</code>", "",
		"آدرس بالا را در قسمت url و سکرت زیر را در قسمت secret تنظیمات webhook پنل کپی کن (days_left و usage_percent هم همان‌جاست).",
		"", "🛣 مسیر داشبورد: <code>" + Esc(dash) + "</code>",
	}
	rb := pc.Renewbot
	if rb != "" {
		auto := ""
		if pc.RenewbotAuto {
			auto = " (خودکار)"
		}
		lines = append(lines, "🤖 ربات داخلی: @"+rb+auto)
	} else {
		lines = append(lines, "🤖 ربات داخلی: تعریف نشده")
	}
	recurLabel := "⏱ recurrent"
	kb := [][]Btn{
		{btn("🛣 مسیر داشبورد", fmt.Sprintf("pgdash:%d", i)),
			btn("🤖 ربات داخلی", fmt.Sprintf("pgrenewbot:%d", i))},
		{btn("🔄 توکن جدید", fmt.Sprintf("pgregen:%d", i)),
			btn(recurLabel, fmt.Sprintf("pgrecur:%d", i))},
		{btn("🧪 پیام تستی", fmt.Sprintf("pgtest:%d", i))},
		navRow("pghook"),
	}
	r.Show(chat, mid, strings.Join(lines, "\n"), kb)
}

func (r *Router) pgToggle(chat, mid int64) {
	cfg := alerts.GetCfg(r.Store)
	cfg.Enabled = !cfg.Enabled
	alerts.SaveCfg(r.Store, cfg)
	r.pghookHome(chat, mid)
}

func (r *Router) pgSilent(chat, mid int64) {
	cfg := alerts.GetCfg(r.Store)
	cfg.SilentMe = !cfg.SilentMe
	alerts.SaveCfg(r.Store, cfg)
	r.pghookHome(chat, mid)
}

func (r *Router) pgRegen(chat, mid int64, i int) {
	panels := r.panels()
	if i < 0 || i >= len(panels) {
		r.Show(chat, mid, "❌ پنل پیدا نشد.", HomeKB())
		return
	}
	cfg := alerts.GetCfg(r.Store)
	pid := panels[i].URL
	pc := cfg.Panels[pid]
	if pc == nil {
		pc = &alerts.PanelCfg{}
		cfg.Panels[pid] = pc
	}
	pc.Secret = randHex(12)
	alerts.SaveCfg(r.Store, cfg)
	r.pgPanel(chat, mid, i)
}

func (r *Router) pghookTest(chat, mid int64, i int) {
	panels := r.panels()
	if i < 0 || i >= len(panels) {
		r.Show(chat, mid, "❌ پنل پیدا نشد.", [][]Btn{{btn("🔙 هشدارها", "pghook")}})
		return
	}
	p := panels[i]
	cfg := alerts.GetCfg(r.Store)
	rb := ""
	if pc := cfg.Panels[p.URL]; pc != nil {
		rb = pc.Renewbot
	}
	items := []map[string]any{{"username": "test-user", "kind": "days",
		"value": 1, "expireTs": nil, "status": "", "owner": "-"}}
	alerts.SendAlert(r.Bot, chat, pg.Panel{Name: pname(p), URL: p.URL,
		Username: rb}, items, true)
	r.pgPanel(chat, 0, i)
}

func (r *Router) pgDashAsk(chat, mid int64, i int) {
	r.setWizard(chat, map[string]any{"kind": "pg_dashpath", "panel": i})
	r.Show(chat, mid,
		"🛣 مسیر داشبورد این پنل را بفرست (مثلاً `/dashboard` یا `/fxeybo`).\nبرای برگشت به پیش‌فرض (`/dashboard`) یک خط «-» بفرست.",
		[][]Btn{{btn("🔙 انصراف", fmt.Sprintf("pgpanel:%d", i))}})
}

func (r *Router) pgRenewbotAsk(chat, mid int64, i int) {
	r.setWizard(chat, map[string]any{"kind": "pg_renewbot", "panel": i})
	r.Show(chat, mid,
		"🤖 آیدی ربات داخلی همین پنل را بفرست (بدون @، مثلاً `allmarzbot`).\n\nبرای حذف (فقط دکمه پنل بماند) یک خط «-» بفرست.",
		[][]Btn{{btn("🔙 انصراف", fmt.Sprintf("pgpanel:%d", i))}})
}

func (r *Router) pgRecurAsk(chat, mid int64, i int) {
	cur := ""
	panels := r.panels()
	if i >= 0 && i < len(panels) {
		c := panels[i].Client()
		if tok, err := c.Login(); err == nil {
			if st := c.Settings(tok); st != nil {
				if wh, ok := st["webhook"].(map[string]any); ok {
					if v, ok := wh["recurrent"]; ok {
						cur = fmt.Sprintf("\n\nفعلی: %v ثانیه", v)
					}
				}
			}
		}
	}
	r.setWizard(chat, map[string]any{"kind": "pg_recur", "panel": i})
	r.Show(chat, mid,
		"⏱ هر چند ثانیه پنل کاربرها را برای هشدار ارزیابی کند؟"+cur+
			"\nپیشنهاد: 3600 (ساعتی).\n\nعدد بین ۶۰ تا ۸۶۴۰۰ بفرست.",
		[][]Btn{{btn("🔙 انصراف", fmt.Sprintf("pgpanel:%d", i))}})
}

func truncRune(s string, n int) string {
	rr := []rune(s)
	if len(rr) > n {
		return string(rr[:n])
	}
	return s
}

// ---------- pending wizard handlers ----------

func (r *Router) pgAddURL(chat, mid int64, text string) {
	url := strings.TrimRight(strings.TrimSpace(text), "/")
	if !strings.HasPrefix(url, "http://") && !strings.HasPrefix(url, "https://") {
		r.Bot.Send(chat, "❌ آدرس باید با http شروع شود.", nil)
		return
	}
	r.setWizard(chat, map[string]any{"kind": "pg_add_user", "url": url})
	r.Bot.Send(chat, "👤 یوزر ادمین پنل را بفرستید:", nil)
}

func (r *Router) pgAddUser(chat, mid int64, pend map[string]any, text string) {
	r.setWizard(chat, map[string]any{"kind": "pg_add_pass",
		"url": pend["url"], "username": strings.TrimSpace(text)})
	r.Bot.Send(chat, "🔑 پسورد ادمین پنل را بفرستید:", nil)
}

func (r *Router) pgAddPass(chat, mid int64, pend map[string]any, text string) {
	url, _ := pend["url"].(string)
	user, _ := pend["username"].(string)
	pass := strings.TrimSpace(text)
	if _, err := pg.New(url, user, pass).Login(); err != nil {
		r.Bot.Send(chat, "❌ ورود ناموفق: "+Esc(err.Error()), nil)
		return
	}
	panels := r.panels()
	name := url
	if i := strings.Index(name, "//"); i >= 0 {
		name = name[i+2:]
	}
	if i := strings.Index(name, "/"); i >= 0 {
		name = name[:i]
	}
	panels = append(panels, pg.Panel{URL: url, Username: user, Password: pass, Name: name})
	r.savePanels(panels)
	r.clearWizard(chat)
	r.Bot.Send(chat, "✅ پنل اضافه شد.",
		[][]Btn{{btn("🎛 پنل پاسارگارد", "pg"), btn("🏠 خانه", "menu")}})
}

func (r *Router) pgSearchDo(chat, mid int64, pend map[string]any, text string) {
	i := int(fany(pend["panel"]))
	r.clearWizard(chat)
	r.pgUsers(chat, 0, i, 0, strings.TrimSpace(text))
}

func (r *Router) pgDashSave(chat, mid int64, pend map[string]any, text string) {
	i := int(fany(pend["panel"]))
	panels := r.panels()
	if i < 0 || i >= len(panels) {
		r.Bot.Send(chat, "❌ پنل پیدا نشد.", nil)
		return
	}
	r.clearWizard(chat)
	cfg := alerts.GetCfg(r.Store)
	pid := panels[i].URL
	pc := cfg.Panels[pid]
	if pc == nil {
		pc = &alerts.PanelCfg{}
		cfg.Panels[pid] = pc
	}
	v := strings.TrimSpace(text)
	if v == "-" {
		pc.Dashpath = ""
	} else {
		if !strings.HasPrefix(v, "/") {
			v = "/" + v
		}
		v = strings.TrimRight(v, "/")
		if v == "" {
			v = "/dashboard"
		}
		if len(v) > 60 || strings.ContainsAny(v, " ?#") {
			r.Bot.Send(chat, "❌ مسیر معتبر نیست (مثل `/dashboard`).", nil)
			return
		}
		pc.Dashpath = v
	}
	alerts.SaveCfg(r.Store, cfg)
	r.Bot.Send(chat, "✅ ذخیره شد.",
		[][]Btn{{btn("🔙 پنل", fmt.Sprintf("pgpanel:%d", i)), btn("🏠 خانه", "menu")}})
}

func (r *Router) pgRenewbotSave(chat, mid int64, pend map[string]any, text string) {
	i := int(fany(pend["panel"]))
	panels := r.panels()
	if i < 0 || i >= len(panels) {
		r.Bot.Send(chat, "❌ پنل پیدا نشد.", nil)
		return
	}
	r.clearWizard(chat)
	cfg := alerts.GetCfg(r.Store)
	pid := panels[i].URL
	pc := cfg.Panels[pid]
	if pc == nil {
		pc = &alerts.PanelCfg{}
		cfg.Panels[pid] = pc
	}
	v := strings.TrimSpace(strings.TrimPrefix(strings.TrimSpace(text), "@"))
	if v == "-" {
		pc.Renewbot = ""
	} else {
		if len(v) < 3 || len(v) > 64 || !strings.HasSuffix(strings.ToLower(v), "bot") || !isAlnumUS(v) {
			r.Bot.Send(chat, "❌ آیدی ربات معتبر نیست (مثلاً `allmarzbot`).", nil)
			return
		}
		pc.Renewbot = v
		pc.RenewbotAuto = false
	}
	alerts.SaveCfg(r.Store, cfg)
	r.Bot.Send(chat, "✅ ذخیره شد.",
		[][]Btn{{btn("🔙 پنل", fmt.Sprintf("pgpanel:%d", i)), btn("🏠 خانه", "menu")}})
}

func (r *Router) pgRecurSave(chat, mid int64, pend map[string]any, text string) {
	i := int(fany(pend["panel"]))
	r.clearWizard(chat)
	n, err := strconv.Atoi(strings.TrimSpace(text))
	if err != nil || n < 60 || n > 86400 {
		r.Bot.Send(chat, "❌ عدد بین ۶۰ تا ۸۶۴۰۰ بفرست.", nil)
		return
	}
	panels := r.panels()
	ok := false
	if i >= 0 && i < len(panels) {
		ok = r.setPanelRecurrent(panels[i], n)
	}
	msg := "❌ ذخیره در پنل نشد."
	if ok {
		msg = fmt.Sprintf("✅ recurrent شد %d ثانیه.", n)
	}
	r.Bot.Send(chat, msg,
		[][]Btn{{btn("🔙 پنل", fmt.Sprintf("pgpanel:%d", i)), btn("🏠 خانه", "menu")}})
}

func isAlnumUS(s string) bool {
	for _, c := range s {
		if !(c >= 'a' && c <= 'z' || c >= 'A' && c <= 'Z' ||
			c >= '0' && c <= '9' || c == '_') {
			return false
		}
	}
	return true
}

func (r *Router) setPanelRecurrent(p pg.Panel, n int) bool {
	c := p.Client()
	tok, err := c.Login()
	if err != nil {
		return false
	}
	return c.SetRecurrent(tok, n)
}
