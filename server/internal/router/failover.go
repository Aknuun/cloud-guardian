package router

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"strings"
	"time"

	"github.com/aknuun/cg-server/internal/cfworkers"
)

const foCfgKey = "fo:cfg"

// default source for the Worker script (standby failover edition).
const defaultWorkerJS = "https://raw.githubusercontent.com/Aknuun/cloud-guardian/main/worker.js"

func (r *Router) foGet() map[string]string {
	var m map[string]string
	if r.Store.GetJSON(foCfgKey, &m) && m != nil {
		return m
	}
	return map[string]string{}
}

func (r *Router) foSet(m map[string]string) {
	r.Store.PutJSON(foCfgKey, m, 0)
	if r.FO == nil {
		r.FO = map[string]string{}
	}
	for k, v := range m {
		r.FO[k] = v
	}
}

func (r *Router) foConfigured() bool {
	c := r.foGet()
	return c["worker_name"] != "" && c["cf_api_token"] != "" && c["cf_account_id"] != ""
}

func (r *Router) fetchWorkerJS() ([]byte, error) {
	url := strings.TrimSpace(os.Getenv("CG_WORKER_JS_URL"))
	if url == "" {
		url = defaultWorkerJS
	}
	if strings.HasPrefix(url, "/") {
		return os.ReadFile(url)
	}
	cli := &http.Client{Timeout: 40 * time.Second}
	resp, err := cli.Get(url)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode != 200 {
		return nil, fmt.Errorf("http %d", resp.StatusCode)
	}
	return io.ReadAll(io.LimitReader(resp.Body, 8<<20))
}

func foPing(url string) (bool, int) {
	if url == "" {
		return false, 0
	}
	cli := &http.Client{Timeout: 8 * time.Second}
	resp, err := cli.Get(url)
	if err != nil {
		return false, 0
	}
	defer resp.Body.Close()
	_, _ = io.Copy(io.Discard, resp.Body)
	return resp.StatusCode >= 200 && resp.StatusCode < 300, resp.StatusCode
}

// ---------- status ----------

func (r *Router) foWebhookURL() string {
	info, err := r.Bot.GetWebhookInfo()
	if err != nil {
		return ""
	}
	s, _ := info["url"].(string)
	return s
}

func (r *Router) foStatusText() string {
	c := r.foGet()
	if c["worker_name"] == "" {
		return "🛟 <b>پشتیبان فیلور (Standby)</b>\n\n" +
			"❌ هنوز تنظیم نشده.\n" +
			"وقتی سرور قطع شود، یک ورکر کلادفلر جای ربات را می‌گیرد و تا برگشت سرور پاسخ می‌دهد.\n\n" +
			"برای راه‌اندازی روی «⚙️ راه‌اندازی» بزن."
	}
	wh := r.foWebhookURL()
	owner := "سرور (polling)"
	if strings.Contains(wh, c["worker_name"]) && c["worker_name"] != "" {
		owner = "ورکر پشتیبان (فعال)"
	} else if wh != "" {
		owner = "نامشخص"
	}
	health := c["public_url"]
	if health != "" {
		health += "/health"
	}
	srvUp, _ := foPing(health)
	srvTxt := "🔴 در دسترس نیست"
	if srvUp {
		srvTxt = "🟢 سالم"
	}
	state := "🟢 فعال (standby)"
	if strings.HasPrefix(owner, "ورکر") {
		state = "🟠 فیلور فعال شد (سرور قطع است)"
	}
	acc := c["account_name"]
	if acc == "" {
		acc = c["cf_account_id"]
	}
	return "🛟 <b>پشتیبان فیلور (Standby)</b>\n\n" +
		"وضعیت: " + state + "\n" +
		"سرور: " + srvTxt + "\n" +
		"مسئول فعلی webhook: " + owner + "\n" +
		"نام ورکر: <code>" + Esc(c["worker_name"]) + "</code>\n" +
		"آدرس: <code>" + Esc(c["worker_url"]) + "</code>\n" +
		"اکانت: <code>" + Esc(acc) + "</code>\n" +
		"KV: <code>" + Esc(c["fo_namespace_id"]) + "</code>\n\n" +
		"کرون ورکر هر ۵ دقیقه سرور را چک می‌کند؛ اگر قطع بود webhook را به خودش می‌برد و اگر سالم شد پس می‌دهد."
}
func (r *Router) foMaybePrompt(chat int64) {
	if r.foConfigured() {
		return
	}
	if r.Store.Get("fo:prompted") == "1" {
		return
	}
	r.Store.Put("fo:prompted", "1", 0)
	r.Bot.Send(chat,
		"🛟 <b>پیشنهاد: پشتیبان فیلور</b>\n\n"+
			"می‌خواهی پشتیبان فیلور راه بیندازی؟ اگر سرورت قطع شود، یک ورکر کلادفلر (رایگان) موقتاً جای ربات را می‌گیرد تا سرور برگردد.\n\n"+
			"الان تنظیم کن یا بعداً از «تنظیمات» هم می‌شود.",
		[][]Btn{{btnS("🛟 راه‌اندازی فیلور", "fo", "success"), btn("بعداً", "fo_skip")}})
}

func (r *Router) failoverHome(chat, mid int64) {
	var kb [][]Btn
	if r.foConfigured() {
		kb = [][]Btn{
			{btn("🔄 بررسی وضعیت", "forefresh")},
			{btn("♻️ تنظیم مجدد / تغییر", "fosetup")},
			{btnS("⏸ غیرفعال کردن", "fodisable", "danger")},
			{btnS("🗑 حذف کامل ورکر", "fodelete", "danger")},
		}
	} else {
		kb = [][]Btn{{btnS("⚙️ راه‌اندازی فیلور", "fosetup", "success")}}
	}
	kb = append(kb, []Btn{btn("🧹 پاک‌سازی ورکرهای قبلی", "foold")})
	kb = append(kb, []Btn{btn("🔙 بازگشت", "settings"), btn("🏠 خانه", "menu")})
	r.Show(chat, mid, r.foStatusText(), kb)
}

func (r *Router) foRefresh(chat, mid int64) {
	r.failoverHome(chat, mid)
}

// ---------- setup wizard ----------

func (r *Router) foSetupStart(chat, mid int64) {
	r.setWizard(chat, map[string]any{"kind": "fo_token"})
	r.Show(chat, mid,
		"🛟 <b>راه‌اندازی پشتیبان فیلور</b>\n\n"+
			"برای اینکه ربات بتواند ورکر پشتیبان را روی اکانت خودت بسازد، یک توکن کلادفلر با این ۸ دسترسی لازم است:\n\n"+
			"<b>اجباری:</b>\n"+
			"۱) Account · Workers Scripts · Edit\n"+
			"۲) Account · Workers KV Storage · Edit\n"+
			"۳) Zone · DNS · Edit\n"+
			"۴) Zone · Zone Settings · Edit\n\n"+
			"<b>اختیاری:</b>\n"+
			"۵) Zone · Email Routing Rules · Edit\n"+
			"۶) Account · Email Routing Addresses · Edit\n"+
			"۷) Zone · Analytics · Read\n"+
			"۸) Account · Account Analytics · Read\n\n"+
			"ساخت توکن: <code>dash.cloudflare.com/profile/api-tokens</code> → Create Custom Token\n"+
			"(Account Resources = All accounts · Zone Resources = All zones)\n\n"+
			"توکن <code>cfut_...</code> را بفرست (یا «لغو»).",
		[][]Btn{{btn("❌ لغو", "fo")}})
}

func (r *Router) foTokenSave(chat, mid int64, token string) {
	token = strings.TrimSpace(token)
	if !strings.HasPrefix(token, "cfut_") && !strings.HasPrefix(token, "cf") {
		r.Show(chat, mid, "❌ توکن معتبر نیست (باید با <code>cfut_</code> شروع شود). دوباره بفرست:",
			[][]Btn{{btn("❌ لغو", "fo")}})
		return
	}
	r.Show(chat, mid, "⏳ در حال بررسی توکن و پیدا کردن اکانت…", nil)
	go func() {
		c := cfworkers.New(token)
		if err := c.Verify(); err != nil {
			r.Bot.Send(chat, "❌ توکن نامعتبر یا بدون دسترسی: "+Esc(err.Error()), [][]Btn{{btn("❌ لغو", "fo")}})
			r.setWizard(chat, map[string]any{"kind": "fo_token"})
			return
		}
		accs, err := c.Accounts()
		if err != nil || len(accs) == 0 {
			r.Bot.Send(chat, "❌ دسترسی به اکانت ندارم. مطمئن شو Account:Read یا Workers Scripts:Edit دادی.", nil)
			return
		}
		aid, aname := accs[0].ID, accs[0].Name
		// pick a non-clashing default name
		existing, _ := c.ListScripts(aid)
		set := map[string]bool{}
		for _, s := range existing {
			set[s] = true
		}
		name := "cg-failover-" + randHex(2)
		for set[name] {
			name = "cg-failover-" + randHex(2)
		}
		r.setWizard(chat, map[string]any{"kind": "fo_name", "token": token, "account_id": aid, "account_name": aname})
		r.Bot.Send(chat, fmt.Sprintf(
			"✅ توکن سالم است.\n🏢 اکانت: <code>%s</code>\n👀 ورکرهای موجود: <code>%s</code>\n\n"+
				"نام ورکر پشتیبان: <code>%s</code>\n"+
				"برای شروع دپلو این اسم را تأیید کن یا یک اسم تازه بفرست:",
			Esc(aname), Esc(strings.Join(existing, ", ")), Esc(name)),
			[][]Btn{{btnS("✅ دپلو با همین اسم", "fodeploy", "success"), btn("❌ لغو", "fo")}})
	}()
}

func (r *Router) foNameSave(chat, mid int64, pend map[string]any, name string) {
	name = strings.TrimSpace(name)
	if name == "" || len(name) > 50 || strings.ContainsAny(name, " /\\") {
		r.Show(chat, mid, "❌ اسم نامعتبر. یک اسم ساده بفرست (حروف/عدد/خط تیره):", [][]Btn{{btn("❌ لغو", "fo")}})
		return
	}
	pend["name"] = name
	r.setWizard(chat, pend)
	r.foDeploy(chat, pend)
}

// ---------- deploy ----------

func (r *Router) foDeploy(chat int64, pend map[string]any) {
	token, _ := pend["token"].(string)
	aid, _ := pend["account_id"].(string)
	aname, _ := pend["account_name"].(string)
	name, _ := pend["name"].(string)
	if token == "" || aid == "" || name == "" {
		r.Bot.Send(chat, "❌ اطلاعات ناقص؛ دوباره از «تنظیم مجدد» شروع کن.", nil)
		return
	}
	r.clearWizard(chat)
	r.Bot.Send(chat, "🚀 در حال ساخت ورکر پشتیبان… (چند لحظه طول می‌کشد)", nil)
	go func() {
		c := cfworkers.New(token)
		// 1) new KV namespace
		ns, err := c.CreateKV(aid, "cg-failover-kv-"+randHex(3))
		if err != nil {
			c.DeleteScript(aid, name)
			r.Bot.Send(chat, failMsg("ساخت فضای KV", err), [][]Btn{{btn("🔙 فیلور", "fo")}})
			return
		}
		// 2) fetch worker.js
		js, err := r.fetchWorkerJS()
		if err != nil || len(js) < 1000 {
			c.DeleteKV(aid, ns)
			r.Bot.Send(chat, "❌ دریافت فایل ورکر ناموفق بود: "+Esc(fmt.Sprint(err)), [][]Btn{{btn("🔙 فیلور", "fo")}})
			return
		}
		pub := strings.TrimRight(r.FO["public_url"], "/")
		health := ""
		if pub != "" {
			health = pub + "/health"
		}
		selfURL := ""
		if sub, e := c.WorkersSubdomain(aid); e == nil && sub != "" {
			selfURL = fmt.Sprintf("https://%s.%s.workers.dev/tg", name, sub)
		}
		cfJSON, _ := json.Marshal(r.cfAccs())
		md := cfworkers.Metadata{Bindings: []cfworkers.Binding{
			{Type: "kv_namespace", Name: "BOT_KV", NamespaceID: ns},
			{Type: "secret_text", Name: "BOT_TOKEN", Text: r.botToken()},
			{Type: "plain_text", Name: "ADMIN_ID", Text: fmt.Sprint(r.AdminID)},
			{Type: "plain_text", Name: "CF_ACCOUNTS", Text: string(cfJSON)},
			{Type: "plain_text", Name: "WORKER_ACCOUNT_ID", Text: aid},
			{Type: "plain_text", Name: "WORKER_NAME", Text: name},
			{Type: "plain_text", Name: "PUBLIC_URL", Text: pub},
			{Type: "plain_text", Name: "CG_STANDBY", Text: "1"},
			{Type: "plain_text", Name: "CG_HEALTH_URL", Text: health},
			{Type: "plain_text", Name: "CG_SELF_URL", Text: selfURL},
		}}
		// 3) deploy
		if err := c.DeployModule(aid, name, js, md); err != nil {
			c.DeleteKV(aid, ns)
			c.DeleteScript(aid, name)
			r.Bot.Send(chat, failMsg("دپلوی ورکر", err), [][]Btn{{btn("🔙 فیلور", "fo")}})
			return
		}
		// 4) cron every 5 min + workers.dev
		_ = c.SetSchedules(aid, name, []string{"*/5 * * * *"})
		_ = c.SetDev(aid, name, true)
		url, _ := c.ScriptURL(aid, name, "/tg")

		cfg := map[string]string{
			"enabled":         "1",
			"cf_api_token":    token,
			"cf_account_id":   aid,
			"account_name":    aname,
			"worker_name":     name,
			"worker_url":      url,
			"fo_namespace_id": ns,
			"health_url":      health,
			"public_url":      pub,
			"created":         fmt.Sprint(time.Now().Unix()),
		}
		r.foSet(cfg)
		r.SyncPush()
		r.Bot.Send(chat, fmt.Sprintf(
			"✅ <b>ورکر پشتیبان ساخته شد.</b>\n\n"+
				"نام: <code>%s</code>\nآدرس: <code>%s</code>\nKV: <code>%s</code>\n\n"+
				"حالا سرور با polling کار می‌کند و ورکر در حالت standby است؛ هر ۵ دقیقه سلامت سرور را چک می‌کند و فقط وقتی سرور قطع باشد جای ربات را می‌گیرد.",
			Esc(name), Esc(url), Esc(ns)),
			[][]Btn{{btn("🛟 وضعیت فیلور", "fo"), btn("🏠 خانه", "menu")}})
	}()
}

func failMsg(what string, err error) string {
	return "❌ خطا در " + what + ":\n<code>" + Esc(err.Error()) + "</code>\n\n" +
		"اگر دسترسی ناقص است، توکن را با ۸ دسترسی لازم دوباره بساز و «تنظیم مجدد» بزن."
}

func (r *Router) botToken() string {
	return r.BotToken
}

// ---------- disable / delete ----------

func (r *Router) foDisable(chat, mid int64) {
	c := r.foGet()
	if c["worker_name"] == "" {
		r.failoverHome(chat, mid)
		return
	}
	go func() {
		cl := cfworkers.New(c["cf_api_token"])
		_ = cl.SetSchedules(c["cf_account_id"], c["worker_name"], nil)
		c["enabled"] = "0"
		r.foSet(c)
		r.Bot.Send(chat, "⏸ فیلور غیرفعال شد (کرون ورکر حذف شد). ورکر/داده دست‌نخورده ماند.", [][]Btn{{btn("🛟 فیلور", "fo")}})
	}()
	r.Show(chat, mid, "⏳ در حال غیرفعال‌سازی…", nil)
}

func (r *Router) foDelete(chat, mid int64) {
	c := r.foGet()
	if c["worker_name"] == "" {
		r.failoverHome(chat, mid)
		return
	}
	r.Show(chat, mid,
		"⚠️ حذف کامل ورکر پشتیبان:\n"+
			"• اسکریپت <code>"+Esc(c["worker_name"])+"</code> حذف می‌شود\n"+
			"• فضای KV <code>"+Esc(c["fo_namespace_id"])+"</code> حذف می‌شود\n\n"+
			"مطمئنی؟",
		[][]Btn{{btnS("🗑 بله، حذف کن", "fodelete_yes", "danger"), btn("🔙 بازگشت", "fo")}})
}

func (r *Router) foDeleteYes(chat, mid int64) {
	c := r.foGet()
	r.Show(chat, mid, "⏳ در حال حذف…", nil)
	go func() {
		cl := cfworkers.New(c["cf_api_token"])
		if c["worker_name"] != "" && c["cf_account_id"] != "" {
			_ = cl.DeleteScript(c["cf_account_id"], c["worker_name"])
		}
		if c["fo_namespace_id"] != "" && c["cf_account_id"] != "" {
			_ = cl.DeleteKV(c["cf_account_id"], c["fo_namespace_id"])
		}
		r.Store.Delete(foCfgKey)
		for _, k := range []string{"cf_api_token", "cf_account_id", "account_name", "worker_name", "worker_url", "fo_namespace_id", "health_url", "public_url", "health_url"} {
			delete(r.FO, k)
		}
		r.Bot.Send(chat, "🗑 ورکر پشتیبان و فضای KV حذف شدند.", [][]Btn{{btn("🛟 فیلور", "fo"), btn("🏠 خانه", "menu")}})
	}()
}

// ---------- old worker cleanup (migration) ----------

func (r *Router) foOldWorkers(chat, mid int64) {
	c := r.foGet()
	tok := c["cf_api_token"]
	aid := c["cf_account_id"]
	if tok == "" {
		r.Show(chat, mid,
			"🧹 <b>پاک‌سازی ورکرهای قبلی</b>\n\nبرای فهرست‌کردن ورکرها اول باید توکن کلادفلر ست شود.\n"+
				"اگر از نسخهٔ ورکری مهاجرت می‌کنی، ورکر قدیمی چون همان توکن بات را دارد ممکن است webhook را از سرور بدزدد؛ باید کرون/اسکریپتش را غیرفعال یا حذف کنی.",
			[][]Btn{{btn("⚙️ راه‌اندازی فیلور", "fosetup"), btn("🔙 بازگشت", "fo")}})
		return
	}
	r.Show(chat, mid, "⏳ در حال خواندن ورکرها…", nil)
	go func() {
		cl := cfworkers.New(tok)
		scripts, err := cl.ListScripts(aid)
		if err != nil {
			r.Bot.Send(chat, "❌ خطا: "+Esc(err.Error()), [][]Btn{{btn("🔙 فیلور", "fo")}})
			return
		}
		var kb [][]Btn
		for _, s := range scripts {
			if s == c["worker_name"] {
				continue
			}
			kb = append(kb, []Btn{
				btnS("🧹 کرون "+s, "fooldcron:"+s, "danger"),
				btnS("🗑 حذف "+s, "foolddel:"+s, "danger"),
			})
		}
		if len(kb) == 0 {
			kb = append(kb, []Btn{btn("🔙 بازگشت", "fo")})
		}
		r.Bot.Send(chat,
			"🧹 <b>ورکرهای اکانت</b>\n\n"+
				"«کرون» = تریگرهای زمان‌بندی ورکر را پاک می‌کند (دیگر webhook را نمی‌دزدد).\n"+
				"«حذف» = کل اسکریپت را پاک می‌کند.\n\n"+
				"اگر ورکرِ نسخهٔ قدیمی را نمی‌خواهی، اول کرونش را پاک کن.",
			kb)
	}()
}

func (r *Router) foOldCron(chat, mid int64, name string) {
	c := r.foGet()
	go func() {
		cl := cfworkers.New(c["cf_api_token"])
		if err := cl.SetSchedules(c["cf_account_id"], name, nil); err != nil {
			r.Bot.Send(chat, "❌ خطا: "+Esc(err.Error()), nil)
			return
		}
		r.Bot.Send(chat, "🧹 کرون <code>"+Esc(name)+"</code> پاک شد.", [][]Btn{{btn("🔙 فیلور", "fo")}})
	}()
	r.Show(chat, mid, "⏳ …", nil)
}

func (r *Router) foOldDelete(chat, mid int64, name string) {
	c := r.foGet()
	go func() {
		cl := cfworkers.New(c["cf_api_token"])
		if err := cl.DeleteScript(c["cf_account_id"], name); err != nil {
			r.Bot.Send(chat, "❌ خطا: "+Esc(err.Error()), nil)
			return
		}
		r.Bot.Send(chat, "🗑 اسکریپت <code>"+Esc(name)+"</code> حذف شد.", [][]Btn{{btn("🔙 فیلور", "fo")}})
	}()
	r.Show(chat, mid, "⏳ …", nil)
}
