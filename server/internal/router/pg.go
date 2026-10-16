package router

import (
	"fmt"
	"strconv"
	"strings"

	"github.com/aknuun/cg-server/internal/pg"
)

func (r *Router) panels() []pg.Panel {
	var lst []pg.Panel
	r.Store.GetJSON("panels", &lst)
	return lst
}

func (r *Router) savePanels(lst []pg.Panel) { r.Store.PutJSON("panels", lst, 0) }

// PanelsRaw exposes the stored panels (used by background alert loops).
func (r *Router) PanelsRaw() []pg.Panel { return r.panels() }

func (r *Router) pgHome(chat, mid int64) {
	panels := r.panels()
	if len(panels) == 0 {
		r.Show(chat, mid, "🎛 پنل پاسارگارد\n\nپنلی نیست؛ اضافه کن:",
			[][]Btn{{btn("➕ افزودن پنل", "pga")}, {btn("🏠 خانه", "menu")}})
		return
	}
	var kb [][]Btn
	for i, p := range panels {
		kb = append(kb, []Btn{
			btn("🖥 "+pname(p), fmt.Sprintf("pnlops:%d", i)),
			btn("📊", fmt.Sprintf("pgl:%d:0", i)),
		})
	}
	kb = append(kb, []Btn{btn("🔁 چرخش روزانه ساب‌ها", "hfd")})
	kb = append(kb, []Btn{btn("➕ افزودن پنل", "pga"), btn("🗑 حذف پنل", "pgdel")})
	kb = append(kb, []Btn{btn("✏️ ویرایش پنل‌ها", "pnle"), btn("🏠 خانه", "menu")})
	r.Show(chat, mid, fmt.Sprintf("🎛 پنل پاسارگارد (%d پنل):", len(panels)), kb)
}

// ---------- panel edit ----------

func (r *Router) pnleList(chat, mid int64) {
	panels := r.panels()
	if len(panels) == 0 {
		r.Show(chat, mid, "📭 پنلی ثبت نشده.",
			[][]Btn{{btn("➕ افزودن پنل", "pga")}, {btn("🔙 بازگشت", "pg"), btn("🏠 خانه", "menu")}})
		return
	}
	var kb [][]Btn
	for i, p := range panels {
		kb = append(kb, []Btn{btn("✏️ "+pname(p), fmt.Sprintf("pnle:%d", i))})
	}
	kb = append(kb, []Btn{btn("🔙 بازگشت", "pg"), btn("🏠 خانه", "menu")})
	r.Show(chat, mid, "کدام پنل را ویرایش کنم؟", kb)
}

func (r *Router) pnlePanel(chat, mid int64, i int) {
	panels := r.panels()
	if i < 0 || i >= len(panels) {
		r.Show(chat, mid, "❌ پنل پیدا نشد.", [][]Btn{{btn("🎛 پنل‌ها", "pg")}})
		return
	}
	p := panels[i]
	lines := []string{fmt.Sprintf("✏️ ویرایش پنل «%s»", Esc(pname(p))),
		"🌐 " + strv(p.URL),
		"👤 " + orDash(strv(p.Username)),
		"", "کدام فیلد را تغییر دهم؟"}
	kb := [][]Btn{
		{btn("✏️ نام", fmt.Sprintf("pnleN:%d", i)), btn("🌐 آدرس", fmt.Sprintf("pnleU:%d", i))},
		{btn("👤 نام‌کاربری", fmt.Sprintf("pnleP:%d", i)), btn("🔑 رمز", fmt.Sprintf("pnleW:%d", i))},
		{btn("🔙 بازگشت", "pnle"), btn("🏠 خانه", "menu")},
	}
	r.Show(chat, mid, strings.Join(lines, "\n"), kb)
}

func (r *Router) pnleField(chat, mid int64, field string, i int) {
	panels := r.panels()
	if i < 0 || i >= len(panels) {
		r.Show(chat, mid, "❌ پنل پیدا نشد.", [][]Btn{{btn("🎛 پنل‌ها", "pg")}})
		return
	}
	p := panels[i]
	var kind, prompt string
	switch field {
	case "N":
		kind = "p_ename"
		prompt = fmt.Sprintf("✏️ نام جدید پنل «%s» را بفرستید:", Esc(pname(p)))
	case "U":
		kind = "p_eurl"
		prompt = "🌐 آدرس جدید پنل را بفرستید (فعلی: " + strv(p.URL) + "):"
	case "P":
		kind = "p_euser"
		prompt = "👤 نام‌کاربری جدید را بفرستید (فعلی: " + orDash(strv(p.Username)) + "):"
	case "W":
		kind = "p_epass"
		prompt = "🔑 رمز جدید پنل را بفرستید:"
	default:
		return
	}
	r.setWizard(chat, map[string]any{"kind": kind, "idx": i})
	r.Show(chat, mid, prompt, [][]Btn{{btn("⬅️ انصراف", fmt.Sprintf("pnle:%d", i))}})
}

func (r *Router) pnleSave(chat, mid int64, field string, i int, text string) {
	r.clearWizard(chat)
	panels := r.panels()
	if i < 0 || i >= len(panels) {
		r.Show(chat, mid, "❌ پنل پیدا نشد.", [][]Btn{{btn("🎛 پنل‌ها", "pg")}})
		return
	}
	t := strings.TrimSpace(text)
	if t == "" {
		r.Show(chat, mid, "⚠️ مقدار خالی است.", [][]Btn{{btn("🔙 بازگشت", fmt.Sprintf("pnle:%d", i))}})
		return
	}
	switch field {
	case "N":
		panels[i].Name = t
	case "U":
		panels[i].URL = strings.TrimRight(t, "/")
	case "P":
		panels[i].Username = t
	case "W":
		panels[i].Password = t
	}
	r.savePanels(panels)
	r.Bot.Send(chat, "✅ ذخیره شد.", nil)
	r.pnlePanel(chat, 0, i)
}

// pnlops is the per-panel operations menu.
func (r *Router) pnlops(chat, mid int64, i int) {
	panels := r.panels()
	if i < 0 || i >= len(panels) {
		r.Show(chat, mid, "❌ پنل پیدا نشد.", [][]Btn{{btn("🎛 پنل‌ها", "pg")}})
		return
	}
	p := panels[i]
	lines := []string{fmt.Sprintf("🔧 عملیات پنل «%s»", Esc(pname(p))), "🌐 " + strv(p.URL), ""}
	kb := [][]Btn{
		{btn("🔁 چرخش روزانه ساب‌ها", fmt.Sprintf("hfdpanel:%d", i))},
		{btn("✏️ ویرایش پنل", fmt.Sprintf("pnle:%d", i))},
		{btn("🔔 هشدارها/وبهوک", "pghook")},
		{btn("🔙 بازگشت", "pg"), btn("🏠 خانه", "menu")},
	}
	r.Show(chat, mid, strings.Join(lines, "\n"), kb)
}

func pname(p pg.Panel) string {
	if p.Name != "" {
		return p.Name
	}
	return p.URL
}

func (r *Router) pgAdd(chat, mid int64) {
	r.setWizard(chat, map[string]any{"kind": "pg_add_url"})
	r.Show(chat, mid, "🔗 آدرس پنل را بفرستید (مثل https://panel.example.com):",
		[][]Btn{{btn("🔙 بازگشت", "pg"), btn("🏠 خانه", "menu")}})
}

func (r *Router) pgDelList(chat, mid int64) {
	panels := r.panels()
	var kb [][]Btn
	for i, p := range panels {
		kb = append(kb, []Btn{btn("🗑 "+pname(p), "pgd:"+strconv.Itoa(i))})
	}
	kb = append(kb, []Btn{btn("🔙 بازگشت", "pg")})
	r.Show(chat, mid, "🗑 کدام پنل حذف شود؟", kb)
}

func (r *Router) pgDelAsk(chat, mid int64, idx string) {
	i, err := strconv.Atoi(idx)
	panels := r.panels()
	if err != nil || i < 0 || i >= len(panels) {
		r.Show(chat, mid, "❌ پنل پیدا نشد.", HomeKB())
		return
	}
	r.Show(chat, mid, fmt.Sprintf("⚠️ پنل «%s» حذف شود؟", Esc(pname(panels[i]))),
		[][]Btn{{btn("✅ بله", "pgdy:"+idx), btn("❌ انصراف", "pgdel")}})
}

func (r *Router) pgDelDo(chat, mid int64, idx string) {
	i, err := strconv.Atoi(idx)
	panels := r.panels()
	if err != nil || i < 0 || i >= len(panels) {
		r.Show(chat, mid, "❌ پنل پیدا نشد.", HomeKB())
		return
	}
	panels = append(panels[:i:i], panels[i+1:]...)
	r.savePanels(panels)
	r.Bot.Send(chat, "✅ پنل حذف شد.", nil)
	r.pgHome(chat, 0)
}

func (r *Router) pgLogin(i int) (*pg.Pg, pg.Panel, bool) {
	panels := r.panels()
	if i < 0 || i >= len(panels) {
		return nil, pg.Panel{}, false
	}
	p := panels[i]
	return p.Client(), p, true
}

const pgPerPage = 30

func (r *Router) pgUsers(chat, mid int64, i, page int, q string) {
	pgc, p, ok := r.pgLogin(i)
	if !ok {
		r.Show(chat, mid, "❌ پنل پیدا نشد.", HomeKB())
		return
	}
	tok, err := pgc.Login()
	if err != nil {
		r.Show(chat, mid, "❌ "+Esc(err.Error()), HomeKB())
		return
	}
	users, err := pgc.Users(tok, 1000)
	if err != nil {
		r.Show(chat, mid, "❌ "+Esc(err.Error()), HomeKB())
		return
	}
	if q != "" {
		var f []pg.UserInfo
		lq := strings.ToLower(q)
		for _, u := range users {
			if strings.Contains(strings.ToLower(u.Username), lq) {
				f = append(f, u)
			}
		}
		users = f
	}
	pages := (len(users) + pgPerPage - 1) / pgPerPage
	if pages < 1 {
		pages = 1
	}
	if page < 0 {
		page = 0
	}
	if page > pages-1 {
		page = pages - 1
	}
	lo := page * pgPerPage
	hi := lo + pgPerPage
	if hi > len(users) {
		hi = len(users)
	}
	pageUsers := users[lo:hi]
	names := make([]string, len(pageUsers))
	for k, u := range pageUsers {
		names[k] = u.Username
	}
	r.Store.PutJSON(fmt.Sprintf("s:%d", chat),
		map[string]any{"panel": i, "users": names}, 3600)
	var kb [][]Btn
	for idx, u := range pageUsers {
		flag := ""
		if p95 := pg.Pct(u.Used, u.Limit); p95 != nil {
			if *p95 >= 95 {
				flag = "🔴 "
			} else if *p95 >= 80 {
				flag = "🟡 "
			}
		}
		kb = append(kb, []Btn{btn(fmt.Sprintf("%s%s", flag, u.Username), "pgu:"+strconv.Itoa(idx))})
	}
	if pages > 1 {
		kb = append(kb, []Btn{btn(fmt.Sprintf("%d/%d", page+1, pages),
			fmt.Sprintf("pgl:%d:%d", i, (page+1)%pages))})
	}
	kb = append(kb, []Btn{btn("🔍 جستجو", "pgq"), btn("🔙 بازگشت", "pg")})
	kb = append(kb, []Btn{btn("🏠 خانه", "menu")})
	r.Show(chat, mid, fmt.Sprintf("👥 کاربران «%s» (%d نفر):", Esc(pname(p)), len(users)), kb)
}

func (r *Router) pgUser(chat, mid int64, idx int) {
	var sess struct {
		Panel int      `json:"panel"`
		Users []string `json:"users"`
	}
	r.Store.GetJSON(fmt.Sprintf("s:%d", chat), &sess)
	if idx < 0 || idx >= len(sess.Users) {
		r.Show(chat, mid, "❌ کاربر پیدا نشد.", HomeKB())
		return
	}
	uname := sess.Users[idx]
	pgc, _, ok := r.pgLogin(sess.Panel)
	if !ok {
		r.Show(chat, mid, "❌ پنل پیدا نشد.", HomeKB())
		return
	}
	tok, err := pgc.Login()
	if err != nil {
		r.Show(chat, mid, "❌ "+Esc(err.Error()), HomeKB())
		return
	}
	d := pgc.User(tok, uname)
	u := uname
	if d != nil {
		if s, ok := d["username"].(string); ok && s != "" {
			u = s
		}
	}
	used := fany(d["used_traffic"])
	lim := fany(d["data_limit"])
	status := "—"
	if d != nil {
		if s, ok := d["status"].(string); ok && s != "" {
			status = s
		}
	}
	exp := any(nil)
	if d != nil {
		exp = d["expire"]
		if exp == nil {
			exp = d["expire_date"]
		}
	}
	p95 := pg.Pct(used, lim)
	dl := pg.DaysLeft(exp)
	limStr := "∞"
	if lim > 0 {
		limStr = fmt.Sprintf("%g", pg.GBOf(lim))
	}
	pctStr := ""
	if p95 != nil {
		pctStr = fmt.Sprintf(" (%g٪)", *p95)
	}
	dlStr := "—"
	if dl != nil {
		dlStr = fmt.Sprintf("%g روز", *dl)
	}
	txt := fmt.Sprintf("👤 <code>%s</code>\nوضعیت: %s\nمصرف: %g / %s گیگ%s\nانقضا: %s",
		Esc(u), Esc(status), pg.GBOf(used), limStr, pctStr, dlStr)
	r.Show(chat, mid, txt, [][]Btn{
		{btn("🔙 بازگشت", fmt.Sprintf("pgl:%d:0", sess.Panel))},
		{btn("🏠 خانه", "menu")}})
}

func (r *Router) pgSearchStart(chat, mid int64) {
	var sess map[string]any
	if !r.Store.GetJSON(fmt.Sprintf("s:%d", chat), &sess) || sess["panel"] == nil {
		r.Show(chat, mid, "اول یک پنل را باز کن.", HomeKB())
		return
	}
	r.setWizard(chat, map[string]any{"kind": "pg_search", "panel": sess["panel"]})
	i := int(fany(sess["panel"]))
	r.Show(chat, mid, "🔍 بخشی از یوزرنیم را بفرستید:",
		[][]Btn{{btn("❌ انصراف", fmt.Sprintf("pgl:%d:0", i))}})
}

func fany(v any) float64 {
	switch t := v.(type) {
	case float64:
		return t
	case int:
		return float64(t)
	case int64:
		return float64(t)
	case string:
		var f float64
		fmt.Sscanf(t, "%g", &f)
		return f
	}
	return 0
}
