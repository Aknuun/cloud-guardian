package router

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"runtime"
	"sort"
	"strings"
	"time"

	"github.com/aknuun/cg-server/internal/cf"
	"github.com/aknuun/cg-server/internal/tg"
)

var appStarted = time.Now()

const defaultAnnounceURL = "https://raw.githubusercontent.com/Aknuun/cloud-guardian/main/announcements.json"

// ---------- config ----------

func (r *Router) hubURL() string {
	if v := r.Store.Get("hub_url"); v != "" {
		return strings.TrimRight(v, "/")
	}
	return ""
}

func (r *Router) hubSecret() string { return r.Store.Get("hub_secret") }

func (r *Router) announceURL() string {
	if v := r.Store.Get("announce_url"); v != "" {
		return v
	}
	return defaultAnnounceURL
}

func httpJSON(method, url string, body any, out any, timeout time.Duration) error {
	var rd io.Reader
	if body != nil {
		b, _ := json.Marshal(body)
		rd = bytes.NewReader(b)
	}
	req, err := http.NewRequest(method, url, rd)
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/json")
	hc := &http.Client{Timeout: timeout}
	resp, err := hc.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	b, _ := io.ReadAll(io.LimitReader(resp.Body, 1<<20))
	if resp.StatusCode >= 300 {
		return fmt.Errorf("HTTP %d", resp.StatusCode)
	}
	if out != nil && len(b) > 0 {
		return json.Unmarshal(b, out)
	}
	return nil
}

// ---------- seen users (for broadcast + support) ----------

type hubUser struct {
	ID        int64  `json:"id"`
	Username  string `json:"username,omitempty"`
	FirstName string `json:"first_name,omitempty"`
	Last      int64  `json:"last"`
}

func (r *Router) trackUser(u *tg.User) {
	if u == nil || u.ID == 0 {
		return
	}
	var list []hubUser
	r.Store.GetJSON("users", &list)
	found := false
	for i := range list {
		if list[i].ID == u.ID {
			list[i].Username = u.Username
			list[i].FirstName = u.FirstName
			list[i].Last = nowSec()
			found = true
			break
		}
	}
	if !found {
		list = append(list, hubUser{ID: u.ID, Username: u.Username, FirstName: u.FirstName, Last: nowSec()})
	}
	if len(list) > 2000 {
		list = list[len(list)-2000:]
	}
	r.Store.PutJSON("users", list, 0)
}

func (r *Router) users() []hubUser {
	var list []hubUser
	r.Store.GetJSON("users", &list)
	return list
}

// ---------- stats ----------

func humanDur(d time.Duration) string {
	sec := int64(d.Seconds())
	day := sec / 86400
	sec %= 86400
	h := sec / 3600
	sec %= 3600
	m := sec / 60
	if day > 0 {
		return fmt.Sprintf("%d روز و %d ساعت", day, h)
	}
	if h > 0 {
		return fmt.Sprintf("%d ساعت و %d دقیقه", h, m)
	}
	return fmt.Sprintf("%d دقیقه", m)
}

func fileMB(path string) float64 {
	if path == "" {
		return 0
	}
	fi, err := os.Stat(path)
	if err != nil {
		return 0
	}
	return float64(fi.Size()) / (1024 * 1024)
}

func (r *Router) hubStatsBuild() map[string]any {
	var mem runtime.MemStats
	runtime.ReadMemStats(&mem)
	zones := 0
	for _, a := range r.cfAccs() {
		if zs, err := cf.New(a.Token).Zones(); err == nil {
			zones += len(zs)
		}
	}
	return map[string]any{
		"bot":       r.Store.Get("bot_username"),
		"version":   Version,
		"go":        runtime.Version(),
		"uptime_s":  int64(time.Since(appStarted).Seconds()),
		"uptime":    humanDur(time.Since(appStarted)),
		"mem_mb":    float64(mem.Alloc) / (1024 * 1024),
		"db_mb":     fileMB(r.DBPath),
		"admins":    len(r.Admins()),
		"users":     len(r.users()),
		"panels":    len(r.PanelsRaw()),
		"zones":     zones,
		"cf_accs":   len(r.cfAccs()),
		"servers":   len(r.SrvList()),
		"ts":        nowSec(),
		"goroutine": runtime.NumGoroutine(),
	}
}

func (r *Router) hubHome(chat, mid int64) {
	kb := [][]Btn{
		{btnS("📊 آمار نصب‌ها", "hubstats", "primary"), btn("📢 اعلان‌ها", "hubann")},
		{btnS("✉️ پیام همگانی", "hubcast", "success")},
		{btn("⚙️ تنظیم هاب", "hubcfg")},
		{btn("🔙 بازگشت", "settings"), btn("🏠 خانه", "menu")},
	}
	r.Show(chat, mid,
		"📊 مرکز هاب\n\n"+r.hubURLLine()+"\n"+
			"همهٔ ادمین‌ها و کاربران ربات اینجا جمع می‌شوند.", kb)
}

func (r *Router) hubURLLine() string {
	if u := r.hubURL(); u != "" {
		return "🔗 هاب: <code>" + Esc(u) + "</code>"
	}
	return "🔗 هاب تنظیم نشده (آمار فقط محلی است)."
}

func (r *Router) hubStats(chat, mid int64) {
	s := r.hubStatsBuild()
	lines := []string{
		"📊 آمار این نصب", "",
		"🤖 ربات: " + Esc(strv(s["bot"])),
		"🏷 نسخه: <code>" + Esc(strv(s["version"])) + "</code>",
		"🐹 Go: <code>" + Esc(strv(s["go"])) + "</code>",
		"⏱ آپ‌تایم: " + Esc(strv(s["uptime"])),
		"🧠 رم: " + fmt.Sprintf("%.1f MB", fany(s["mem_mb"])) + " · threads: " + strv(s["goroutine"]),
		"💾 دیتابیس: " + fmt.Sprintf("%.2f MB", fany(s["db_mb"])),
		"",
		fmt.Sprintf("👤 ادمین‌ها: %d · 🙋 کاربران: %d", intOrAny(s["admins"]), intOrAny(s["users"])),
		fmt.Sprintf("🎛 پنل‌ها: %d · ☁️ دامنه‌ها: %d (اکانت: %d)", intOrAny(s["panels"]), intOrAny(s["zones"]), intOrAny(s["cf_accs"])),
		fmt.Sprintf("🖥 سرورها: %d", intOrAny(s["servers"])),
	}
	// aggregate from hub if configured
	if u := r.hubURL(); u != "" {
		sec := r.hubSecret()
		var agg map[string]any
		if err := httpJSON("GET", u+"/installs/"+sec, nil, &agg, 8*time.Second); err == nil {
			if arr, ok := agg["installs"].([]any); ok {
				lines = append(lines, "", fmt.Sprintf("🌐 مجموع نصب‌ها در هاب: %d", len(arr)))
				for i, x := range arr {
					if i >= 15 {
						lines = append(lines, fmt.Sprintf("… و %d مورد دیگر", len(arr)-15))
						break
					}
					if m, ok := x.(map[string]any); ok {
						lines = append(lines, fmt.Sprintf("• %s — v%s · %d ادمین · %d پنل",
							Esc(strv(m["bot"])), strv(m["version"]), intOrAny(m["admins"]), intOrAny(m["panels"])))
					}
				}
			}
		} else {
			lines = append(lines, "", "⚠️ هاب در دسترس نبود: "+Esc(err.Error()))
		}
	}
	r.Show(chat, mid, strings.Join(lines, "\n"),
		[][]Btn{{btn("🔄 بروزرسانی", "hubstats"), btn("🔙 بازگشت", "hub")}, {btn("🏠 خانه", "menu")}})
}

// ---------- broadcast ----------

func (r *Router) hubCastStart(chat, mid int64) {
	r.setWizard(chat, map[string]any{"kind": "hub_broadcast"})
	recips := len(r.Admins()) + len(r.users())
	r.Show(chat, mid,
		fmt.Sprintf("📢 پیام همگانی\n\nمتن پیام را بفرست؛ برای %d گیرنده ارسال می‌شود.\n"+
			"(ادمین‌ها + کاربرانی که با ربات حرف زده‌اند)", recips),
		[][]Btn{{btn("❌ انصراف", "hub")}, {btn("🏠 خانه", "menu")}})
}

func (r *Router) hubCastSave(chat, mid int64, text string) {
	r.clearWizard(chat)
	msg := strings.TrimSpace(text)
	if msg == "" {
		r.Show(chat, mid, "❌ متن خالی بود.", navKB("hub"))
		return
	}
	targets := map[int64]bool{}
	var order []int64
	for _, a := range r.Admins() {
		if !targets[a] {
			targets[a] = true
			order = append(order, a)
		}
	}
	for _, u := range r.users() {
		if !targets[u.ID] {
			targets[u.ID] = true
			order = append(order, u.ID)
		}
	}
	ok, fail := 0, 0
	for _, id := range order {
		if r.Bot.Send(id, "📢 "+msg, nil) != 0 {
			ok++
		} else {
			fail++
		}
		time.Sleep(40 * time.Millisecond)
	}
	r.Show(chat, mid,
		fmt.Sprintf("✅ پیام همگانی ارسال شد.\n\n📬 موفق: %d · ⚠️ ناموفق: %d", ok, fail),
		[][]Btn{{btn("📊 هاب", "hub"), btn("🏠 خانه", "menu")}})
}

// ---------- announcements ----------

type announcement struct {
	ID    string `json:"id"`
	Text  string `json:"text"`
	Until string `json:"until"`
	Reply bool   `json:"reply"`
}

func (r *Router) hubAnnFetch() ([]announcement, error) {
	var list []announcement
	if err := httpJSON("GET", r.announceURL(), nil, &list, 10*time.Second); err != nil {
		return nil, err
	}
	return list, nil
}

func (r *Router) hubAnn(chat, mid int64) {
	list, err := r.hubAnnFetch()
	if err != nil {
		r.Show(chat, mid, "📢 اعلان‌ها\n\n❌ دریافت ناموفق:\n"+Esc(err.Error()),
			[][]Btn{{btnS("🔗 منبع اعلان‌ها", "annurl", "primary")}, {btn("🔙 بازگشت", "hub")}})
		return
	}
	lines := []string{"📢 اعلان‌ها", ""}
	kb := [][]Btn{}
	if len(list) == 0 {
		lines = append(lines, "📭 اعلان فعالی نیست.")
	}
	for i, a := range list {
		if i >= 20 {
			break
		}
		tag := ""
		if a.Until != "" {
			tag = " ⏳" + string(a.Until)
		}
		lines = append(lines, "• "+Esc(truncRune(a.Text, 80))+tag)
		kb = append(kb, []Btn{btnS("📄 "+truncRune(a.Text, 30), "hubannv:"+a.ID, "plain")})
	}
	kb = append(kb, []Btn{btnS("🔗 منبع اعلان‌ها", "annurl", "primary")})
	kb = append(kb, []Btn{btn("🔄 بروزرسانی", "hubann"), btn("🔙 بازگشت", "hub")})
	r.Show(chat, mid, strings.Join(lines, "\n"), kb)
}

func (r *Router) hubAnnView(chat, mid int64, id string) {
	list, err := r.hubAnnFetch()
	if err != nil {
		r.Show(chat, mid, "❌ "+Esc(err.Error()), navKB("hubann"))
		return
	}
	for _, a := range list {
		if a.ID != id {
			continue
		}
		kb := [][]Btn{}
		if a.Reply {
			kb = append(kb, []Btn{btnS("💬 پاسخ / ثبت‌نام", "hubannreply:"+a.ID, "primary")})
		}
		kb = append(kb, []Btn{btn("🔙 بازگشت", "hubann"), btn("🏠 خانه", "menu")})
		r.Show(chat, mid, "📢 اعلان\n\n"+Esc(a.Text), kb)
		return
	}
	r.Show(chat, mid, "❌ اعلان پیدا نشد.", navKB("hubann"))
}

func (r *Router) hubAnnReplyStart(chat, mid int64, id string) {
	r.setWizard(chat, map[string]any{"kind": "hub_annreply", "annId": id})
	r.Show(chat, mid, "💬 پاسخت را بنویس و بفرست؛ برای سازنده ارسال می‌شود:",
		[][]Btn{{btnS("❌ انصراف", "hubannv:"+id, "danger")}})
}

func (r *Router) hubAnnReplySave(chat, mid int64, pend map[string]any, text string) {
	r.clearWizard(chat)
	msg := strings.TrimSpace(text)
	if msg == "" {
		r.Show(chat, mid, "❌ متن خالی.", navKB("hubann"))
		return
	}
	id := strv(pend["annId"])
	r.forwardSupport(chat, fmt.Sprintf("[پاسخ به اعلان %s]\n\n%s", id, msg))
	r.Show(chat, mid, "✅ پاسخت به سازنده ارسال شد.", navKB(""))
}

// ---------- contact creator ----------

func (r *Router) hubContact(chat, mid int64) {
	main := r.AdminID
	who := fmt.Sprintf("<code>%d</code>", main)
	r.Show(chat, mid,
		"📞 ارتباط با سازنده\n\nهر پیامی بفرستی به ادمین اصلی می‌رسد و می‌تونه جواب بده.\n"+
			"🆔 ادمین اصلی: "+who,
		[][]Btn{{btnS("✉️ فرستادن پیام", "hubctmsg", "primary")}, {btn("🔙 بازگشت", "settings"), btn("🏠 خانه", "menu")}})
}

func (r *Router) hubContactStart(chat, mid int64) {
	r.setWizard(chat, map[string]any{"kind": "hub_contact"})
	r.Show(chat, mid, "✉️ پیامت به سازنده را بفرست:", [][]Btn{{btn("❌ انصراف", "hubcontact")}})
}

func (r *Router) hubContactSave(chat, mid int64, text string) {
	r.clearWizard(chat)
	msg := strings.TrimSpace(text)
	if msg == "" {
		r.Show(chat, mid, "❌ متن خالی.", navKB("hubcontact"))
		return
	}
	r.forwardSupport(chat, msg)
	r.Show(chat, mid, "✅ پیامت به سازنده ارسال شد.", navKB(""))
}

// forwardSupport sends a user message to the main admin with a reply button.
func (r *Router) forwardSupport(fromChat int64, text string) {
	name := ""
	var list []hubUser
	r.Store.GetJSON("users", &list)
	for _, u := range list {
		if u.ID == fromChat {
			name = u.FirstName
			if u.Username != "" {
				name += " (@" + u.Username + ")"
			}
			break
		}
	}
	body := fmt.Sprintf("📩 پیام پشتیبانی\n\n👤 از: %s\n🆔 <code>%d</code>\n\n%s",
		Esc(name), fromChat, Esc(text))
	r.Bot.Send(r.AdminID, body,
		[][]Btn{{btnS("💬 پاسخ", fmt.Sprintf("hubreply:%d", fromChat), "primary")}})
}

func (r *Router) hubReplyStart(chat, mid int64, uid string) {
	id := int64(atoiSafe(uid))
	r.setWizard(chat, map[string]any{"kind": "hub_reply", "uid": id})
	r.Show(chat, mid, "💬 پاسخ به کاربر <code>"+uid+"</code> را بفرست:",
		[][]Btn{{btn("❌ انصراف", "hub")}})
}

func (r *Router) hubReplySave(chat, mid int64, pend map[string]any, text string) {
	r.clearWizard(chat)
	uid := int64(intOrAny(pend["uid"]))
	if uid == 0 {
		r.Show(chat, mid, "❌ کاربر نامعتبر.", HomeKB())
		return
	}
	ok := r.Bot.Send(uid, "📞 پاسخ سازنده:\n\n"+Esc(text), nil) != 0
	if ok {
		r.Show(chat, mid, "✅ پاسخ ارسال شد.", navKB("hub"))
	} else {
		r.Show(chat, mid, "❌ ارسال ناموفق (شاید کاربر ربات را بلاک کرده).", navKB("hub"))
	}
}

// ---------- hub config ----------

func (r *Router) hubCfg(chat, mid int64) {
	lines := []string{
		"⚙️ تنظیم هاب", "",
		r.hubURLLine(),
		"",
		"برای اجرای این نصب به‌عنوان هاب، همان <code>hub_url</code> را روی بقیه نصب‌ها ست کن " +
			"و <code>hub_secret</code> را یکسان بگذار.",
	}
	kb := gridBTN([]Btn{
		btn("🔗 ست‌کردن آدرس هاب", "hubsethub"),
		btn("🔑 ست‌کردن secret هاب", "hubsetsc"),
		btn("📢 منبع اعلان‌ها", "annurl"),
	}, 2)
	kb = append(kb, navRow("hub"))
	r.Show(chat, mid, strings.Join(lines, "\n"), kb)
}

func (r *Router) hubCfgSetURL(chat, mid int64) {
	r.wizStart(chat, mid, map[string]any{"kind": "hub_set_hub"})
	r.Show(chat, mid, "🔗 آدرس هاب را بفرست (مثلاً <code>http://1.2.3.4:8787/hub</code>)\nبرای پاک‌کردن: <code>-</code>",
		[][]Btn{{btn("❌ انصراف", "hubcfg")}})
}

func (r *Router) hubCfgSetSecret(chat, mid int64) {
	r.wizStart(chat, mid, map[string]any{"kind": "hub_set_secret"})
	r.Show(chat, mid, "🔑 یک secret دلخواه برای هاب بفرست:\nبرای پاک‌کردن: <code>-</code>",
		[][]Btn{{btn("❌ انصراف", "hubcfg")}})
}

func (r *Router) hubCfgSetAnn(chat, mid int64) {
	r.wizStart(chat, mid, map[string]any{"kind": "hub_set_ann"})
	r.Show(chat, mid, "🔗 آدرس JSON اعلان‌ها را بفرست:\nبرای پیش‌فرض: <code>-</code>",
		[][]Btn{{btn("❌ انصراف", "hubcfg")}})
}

func (r *Router) hubCfgField(chat, mid int64, kind, text string) {
	r.clearWizard(chat)
	v := strings.TrimSpace(text)
	switch kind {
	case "hub_set_hub":
		if v == "-" {
			r.Store.Delete("hub_url")
		} else {
			r.Store.Put("hub_url", strings.TrimRight(v, "/"), 0)
		}
		r.hubCfg(chat, mid)
	case "hub_set_secret":
		if v == "-" {
			r.Store.Delete("hub_secret")
		} else {
			r.Store.Put("hub_secret", v, 0)
		}
		r.hubCfg(chat, mid)
	case "hub_set_ann":
		if v == "-" {
			r.Store.Delete("announce_url")
		} else {
			r.Store.Put("announce_url", v, 0)
		}
		r.hubCfg(chat, mid)
	}
}

// ---------- telemetry ----------

// HubTelemetry pushes install stats to the configured hub.
func (r *Router) HubTelemetry() {
	u := r.hubURL()
	if u == "" {
		return
	}
	_ = httpJSON("POST", u+"/telemetry/"+r.hubSecret(), r.hubStatsBuild(), nil, 8*time.Second)
}

// HubRecordInstall stores an incoming telemetry record (server side).
func (r *Router) hubRecordInstall(rec map[string]any) {
	bot := strv(rec["bot"])
	if bot == "" {
		bot = fmt.Sprintf("install-%d", intOrAny(rec["ts"]))
	}
	var list []map[string]any
	r.Store.GetJSON("hub_installs", &list)
	found := false
	for i := range list {
		if strv(list[i]["bot"]) == bot {
			list[i] = rec
			found = true
			break
		}
	}
	if !found {
		list = append(list, rec)
	}
	if len(list) > 500 {
		list = list[len(list)-500:]
	}
	r.Store.PutJSON("hub_installs", list, 0)
}

func (r *Router) hubInstalls() []map[string]any {
	var list []map[string]any
	r.Store.GetJSON("hub_installs", &list)
	sort.SliceStable(list, func(i, j int) bool {
		return intOrAny(list[i]["ts"]) > intOrAny(list[j]["ts"])
	})
	return list
}

// HubIngest accepts a telemetry record when the secret matches.
func (r *Router) HubIngest(secret string, rec map[string]any) bool {
	if r.hubSecret() == "" || secret != r.hubSecret() {
		return false
	}
	r.hubRecordInstall(rec)
	return true
}

// HubInstallsFor returns recorded installs when the secret matches.
func (r *Router) HubInstallsFor(secret string) ([]map[string]any, bool) {
	if r.hubSecret() == "" || secret != r.hubSecret() {
		return nil, false
	}
	return r.hubInstalls(), true
}
