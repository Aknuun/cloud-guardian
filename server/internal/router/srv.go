package router

import (
	"fmt"
	"math/rand"
	"net"
	"strconv"
	"strings"
	"time"

	"github.com/aknuun/cg-server/internal/cf"
	"github.com/aknuun/cg-server/internal/hz"
	"github.com/aknuun/cg-server/internal/ln"
	"github.com/aknuun/cg-server/internal/srvmon"
	"github.com/aknuun/cg-server/internal/sshop"
)

func maxf(a, b float64) float64 {
	if a > b {
		return a
	}
	return b
}

func srvRec(id, name, host, user string, port int, auth, secret, note string) map[string]any {
	return map[string]any{"id": id, "name": name, "host": host, "user": user,
		"port": port, "auth": auth, "secret": secret, "note": note}
}

func (r *Router) srvList() []map[string]any {
	var lst []map[string]any
	if !r.Store.GetJSON("srv_list", &lst) {
		return nil
	}
	return lst
}

func (r *Router) srvSave(lst []map[string]any) { r.Store.PutJSON("srv_list", lst, 0) }

// SrvList exposes the stored server list for background monitors.
func (r *Router) SrvList() []map[string]any { return r.srvList() }

func (r *Router) srvGet(sid string) (int, map[string]any) {
	for i, s := range r.srvList() {
		if strv(s["id"]) == strv(sid) {
			return i, s
		}
	}
	return -1, nil
}

func (r *Router) srvHome(chat, mid int64) {
	lst := r.srvList()
	lines := []string{"🖥 سرورها و مانیتورینگ", "",
		"مدیریت سرورهای لینوکسی از طریق SSH:\n" +
			"• مانیتور CPU / RAM / دیسک / پهنای باند\n", ""}
	var kb [][]Btn
	if len(lst) > 0 {
		lines = append(lines, fmt.Sprintf("🖧 سرورهای ثبت‌شده (%d): روی هرکدام بزنید:", len(lst)))
		var rows []Btn
		for _, s := range lst {
			rows = append(rows, btn("🖧 "+strv(s["name"]), fmt.Sprintf("srvopen:%s", strv(s["id"]))))
		}
		kb = append(kb, gridBTN(rows, 2)...)
	}
	kb = append(kb, []Btn{btn("📥 افزودن گروهی", "srvaddbulk"), btn("➕ افزودن سرور", "srvadd")})
	kb = append(kb, []Btn{btn("➕ افزودن از ساب‌دامنه", "srvsub"), btn("📥 وارد کردن از دیتاسنترها", "srvimport")})
	if len(lst) > 0 {
		kb = append(kb, []Btn{btn("🗑 حذف سرور", "srvdel")})
	}
	kb = append(kb, []Btn{btn("📊 مانیتور سرورها", "srvmon"), btn("🔑 رمزهای ذخیره‌شده", "srvpw")})
	kb = append(kb, []Btn{btn("🏠 خانه", "menu")})
	r.Show(chat, mid, strings.Join(lines, "\n"), kb)
}

func (r *Router) srvOpen(chat, mid int64, sid string) {
	i, s := r.srvGet(sid)
	if s == nil {
		r.Show(chat, mid, "❌ سرور پیدا نشد.", [][]Btn{{btn("🖥 سرورها و مانیتورینگ", "srv")}})
		return
	}
	auth := "🔒 رمز عبور"
	if strv(s["auth"]) == "key" {
		auth = "🔑 کلید SSH"
	}
	lines := []string{fmt.Sprintf("🖧 %s", Esc(strv(s["name"]))),
		fmt.Sprintf("🌐 <code>%s:%s</code>  (%s)", Esc(strv(s["host"])), Esc(orDash(strv(s["port"]))), Esc(orDefault(strv(s["user"]), "root"))),
		"🔐 " + auth}
	if n := strv(s["note"]); n != "" {
		lines = append(lines, "📝 "+Esc(n))
	}
	lines = append(lines, "", "یک عملیات را انتخاب کنید:")
	kb := [][]Btn{
		{btn("📊 مانیتور سرور", fmt.Sprintf("srvstats:%d", i)), btn("⚙️ آستانه‌ها", "srvmon")},
		{btn("🔄 ریبوت", fmt.Sprintf("srvreboot:%s", strv(s["id"]))), btn("⏱ آپدیت و آپگرید", fmt.Sprintf("srvupd:%s", strv(s["id"])))},
		{btn("ℹ️ مشخصات سیستم", fmt.Sprintf("srvinfo:%s", strv(s["id"]))), btn("💾 فضای دیسک", fmt.Sprintf("srvdisk:%s", strv(s["id"])))},
		{btn("✏️ ویرایش", fmt.Sprintf("srvedit:%s", strv(s["id"]))), btnS("🗑 حذف", fmt.Sprintf("srvdelx:%s", strv(s["id"])), "danger")},
		{btn("🔙 بازگشت", "srv"), btn("🏠 خانه", "menu")}}
	r.Show(chat, mid, strings.Join(lines, "\n"), kb)
}

func orDefault(s, def string) string {
	if s == "" {
		return def
	}
	return s
}

func (r *Router) srvSSH(s map[string]any, cmd string) sshop.Result {
	return sshop.RunCmd(strv(s["host"]), orDefault(strv(s["user"]), "root"),
		strv(s["secret"]), intOfAny(s["port"]), cmd, 20)
}

func (r *Router) srvInfo(chat, mid int64, sid string) {
	_, s := r.srvGet(sid)
	if s == nil {
		r.Show(chat, mid, "❌ سرور پیدا نشد.", HomeKB())
		return
	}
	res := r.srvSSH(s, sshop.InfoCmd)
	if !res.Ok {
		r.Show(chat, mid, fmt.Sprintf("❌ وصل نشد: <code>%s</code>", Esc(res.Err)),
			[][]Btn{{btn("🔙 بازگشت", "srv")}})
		return
	}
	var out []string
	for _, l := range strings.Split(res.Out, "\n") {
		if idx := strings.Index(l, ":"); idx >= 0 {
			out = append(out, fmt.Sprintf("%s: <code>%s</code>", strings.TrimSpace(l[:idx]), Esc(strings.TrimSpace(l[idx+1:]))))
		}
	}
	r.Show(chat, mid, fmt.Sprintf("🖥 %s\n\n%s", Esc(strv(s["name"])), strings.Join(out, "\n")),
		[][]Btn{{btn("🔙 بازگشت", fmt.Sprintf("srvopen:%s", sid)), btn("🏠 خانه", "menu")}})
}

func (r *Router) srvDisk(chat, mid int64, sid string) {
	_, s := r.srvGet(sid)
	if s == nil {
		r.Show(chat, mid, "❌ سرور پیدا نشد.", HomeKB())
		return
	}
	res := r.srvSSH(s, sshop.DiskCmd)
	if !res.Ok {
		r.Show(chat, mid, fmt.Sprintf("❌ وصل نشد: <code>%s</code>", Esc(res.Err)),
			[][]Btn{{btn("🔙 بازگشت", "srv")}})
		return
	}
	r.Show(chat, mid, fmt.Sprintf("💾 دیسک «%s»:\n<code>%s</code>", Esc(strv(s["name"])), Esc(res.Out)),
		[][]Btn{{btn("🔙 بازگشت", fmt.Sprintf("srvopen:%s", sid)), btn("🏠 خانه", "menu")}})
}

func (r *Router) srvRebootAsk(chat, mid int64, sid string) {
	_, s := r.srvGet(sid)
	if s == nil {
		r.Show(chat, mid, "❌ سرور پیدا نشد.", HomeKB())
		return
	}
	r.Show(chat, mid, "⚠️ سرور ریبوت شود؟",
		[][]Btn{{btn("✅ بله", fmt.Sprintf("srvrebootgo:%s", sid)), btn("❌ انصراف", fmt.Sprintf("srvopen:%s", sid))}})
}

func (r *Router) srvRebootGo(chat, mid int64, sid string) {
	_, s := r.srvGet(sid)
	if s == nil {
		r.Show(chat, mid, "❌ سرور پیدا نشد.", HomeKB())
		return
	}
	res := r.srvSSH(s, "reboot")
	if res.Ok {
		r.Bot.Send(chat, "✅ دستور ریبوت ارسال شد.", nil)
	} else {
		r.Bot.Send(chat, fmt.Sprintf("❌ نشد: <code>%s</code>", Esc(res.Err)), nil)
	}
	r.srvOpen(chat, 0, sid)
}

func (r *Router) srvDelList(chat, mid int64) {
	lst := r.srvList()
	if len(lst) == 0 {
		r.Show(chat, mid, "📭 سروری ثبت نشده.", [][]Btn{{btn("🔙 بازگشت", "srv")}})
		return
	}
	var kb [][]Btn
	for _, s := range lst {
		kb = append(kb, []Btn{btnS("🗑 "+strv(s["name"]), fmt.Sprintf("srvdelx:%s", strv(s["id"])), "danger")})
	}
	kb = append(kb, []Btn{btn("🔙 بازگشت", "srv")})
	r.Show(chat, mid, "🗑 کدام سرور حذف شود؟", kb)
}

func (r *Router) srvAddBulk(chat, mid int64) {
	r.setWizard(chat, map[string]any{"kind": "srv_bulk"})
	r.Show(chat, mid, "📥 هر خط یک سرور:\n<code>host user password [port]</code>",
		[][]Btn{{btn("🔙 انصراف", "srv")}})
}

func (r *Router) srvAdd(chat, mid int64) {
	r.setWizard(chat, map[string]any{"kind": "srv_add_host"})
	r.Show(chat, mid, "🌐 هاست یا آی‌پی سرور را بفرستید:",
		[][]Btn{{btn("🔙 انصراف", "srv")}})
}

func (r *Router) srvDelAsk(chat, mid int64, sid string) {
	r.Show(chat, mid, "⚠️ سرور از لیست حذف شود؟",
		[][]Btn{{btn("✅ بله", fmt.Sprintf("srvdely:%s", sid)), btn("❌ انصراف", "srv")}})
}

func (r *Router) srvDelDo(chat, mid int64, sid string) {
	var out []map[string]any
	for _, s := range r.srvList() {
		if strv(s["id"]) != strv(sid) {
			out = append(out, s)
		}
	}
	r.srvSave(out)
	r.Bot.Send(chat, "✅ حذف شد.", nil)
	r.srvHome(chat, 0)
}

func (r *Router) srvUpdateAsk(chat, mid int64, sid string) {
	_, s := r.srvGet(sid)
	if s == nil {
		r.Show(chat, mid, "❌ سرور پیدا نشد.", HomeKB())
		return
	}
	r.Show(chat, mid, fmt.Sprintf("⚠️ آپدیت و آپگرید روی «%s» اجرا شود؟", Esc(strv(s["name"]))),
		[][]Btn{{btn("✅ بله", fmt.Sprintf("srvupdgo:%s", sid)), btn("❌ انصراف", fmt.Sprintf("srvopen:%s", sid))}})
}

func (r *Router) srvUpdateGo(chat, mid int64, sid string) {
	_, s := r.srvGet(sid)
	if s == nil {
		r.Show(chat, mid, "❌ سرور پیدا نشد.", HomeKB())
		return
	}
	r.Show(chat, mid, "⏳ شروع آپدیت…", nil)
	sv := s
	go func() {
		cmd := "export DEBIAN_FRONTEND=noninteractive; sudo apt-get update -y && sudo apt-get upgrade -y"
		res := sshop.RunCmd(strv(sv["host"]), orDefault(strv(sv["user"]), "root"),
			strv(sv["secret"]), intOfAny(sv["port"]), cmd, 300)
		if !res.Ok {
			r.Bot.Send(chat, fmt.Sprintf("❌ خطا: <code>%s</code>", Esc(res.Err)),
				[][]Btn{{btn("🔙 بازگشت", fmt.Sprintf("srvopen:%s", sid)), btn("🏠 خانه", "menu")}})
			return
		}
		out := res.Out
		if len(out) > 1200 {
			out = out[len(out)-1200:]
		}
		r.Bot.Send(chat, "✅ آپدیت و آپگرید انجام شد.\n\n📄 انتهای خروجی:\n<pre>"+Esc(out)+"</pre>",
			[][]Btn{{btn("🔙 بازگشت", fmt.Sprintf("srvopen:%s", sid)), btn("🏠 خانه", "menu")}})
	}()
}

func (r *Router) srvEditMenu(chat, mid int64, sid string) {
	_, s := r.srvGet(sid)
	if s == nil {
		r.Show(chat, mid, "❌ سرور پیدا نشد.", HomeKB())
		return
	}
	kb := [][]Btn{
		{btn("✏️ نام", fmt.Sprintf("srvedith:%s:name", sid)), btn("🌐 هاست/پورت", fmt.Sprintf("srvedith:%s:host", sid))},
		{btn("👤 کاربر", fmt.Sprintf("srvedith:%s:user", sid)), btn("🔐 رمز/کلید", fmt.Sprintf("srvedith:%s:auth", sid))},
		{btn("📝 یادداشت", fmt.Sprintf("srvedith:%s:note", sid))},
		{btn("🔙 بازگشت", fmt.Sprintf("srvopen:%s", sid)), btn("🏠 خانه", "menu")}}
	r.Show(chat, mid, fmt.Sprintf("✏️ ویرایش «%s» — کدام مورد؟", Esc(strv(s["name"]))), kb)
}

func (r *Router) srvEditField(chat, mid int64, sid, field string) {
	prompts := map[string]string{
		"name": "✏️ نام جدید سرور را بفرستید:",
		"host": "🌐 هاست جدید را بفرستید (مثل 1.2.3.4 یا domain.com:2222 — پورت با : جدا می‌شود):",
		"user": "👤 نام کاربر جدید را بفرستید (پیش‌فرض root):",
		"auth": "🔐 رمز عبور جدید را بفرستید؛ یا کلید خصوصی را با - شروع کنید (مثل -----BEGIN...)؛ یا - بفرستید تا بدون تغییر بماند:",
		"note": "📝 یادداشت جدید را بفرستید (- برای حذف):",
	}
	r.setWizard(chat, map[string]any{"kind": "srv_edit", "srv": sid, "field": field})
	r.Show(chat, mid, orDefault(prompts[field], "مقدار جدید را بفرستید:"),
		[][]Btn{{btn("⬅️ انصراف", fmt.Sprintf("srvopen:%s", sid))}})
}

func (r *Router) srvPwList() []any {
	var pws []any
	if !r.Store.GetJSON("srv_passwords", &pws) {
		return nil
	}
	return pws
}

func (r *Router) srvPwHome(chat, mid int64) {
	pws := r.srvPwList()
	lines := []string{"🔑 رمزهای ذخیره‌شده", ""}
	if len(pws) == 0 {
		lines = append(lines, "📭 رمزی ذخیره نشده.\nاز دکمهٔ زیر رمزهای پرتکرار SSH را ذخیره کن تا در افزودن سرور سریع انتخاب شوند.")
	} else {
		lines = append(lines, fmt.Sprintf("%d رمز ذخیره شده (ماسک‌شده):", len(pws)))
		for i, p := range pws {
			s := strv(p)
			masked := "…"
			if len(s) > 4 {
				masked = s[:2] + "…" + s[len(s)-2:]
			}
			lines = append(lines, fmt.Sprintf("%d) <code>%s</code>", i+1, Esc(masked)))
		}
	}
	kb := [][]Btn{{btn("➕ افزودن رمز", "srvpwadd")}}
	for i := 0; i < len(pws); i += 3 {
		var row []Btn
		for j := i; j < i+3 && j < len(pws); j++ {
			row = append(row, btn(fmt.Sprintf("🗑 %d", j+1), fmt.Sprintf("srvpwdel:%d", j)))
		}
		kb = append(kb, row)
	}
	kb = append(kb, []Btn{btn("🔙 بازگشت", "srv"), btn("🏠 خانه", "menu")})
	r.Show(chat, mid, strings.Join(lines, "\n"), kb)
}

func (r *Router) srvPwDelAsk(chat, mid int64, idx string) {
	i := atoiSafe(idx)
	if i < 0 || i >= len(r.srvPwList()) {
		r.srvPwHome(chat, mid)
		return
	}
	r.Show(chat, mid, fmt.Sprintf("⚠️ «رمز %d» حذف شود؟", i+1),
		[][]Btn{{btn("✅ بله، حذف کن", fmt.Sprintf("srvpwdely:%d", i)), btn("❌ انصراف", "srvpw")}})
}

func (r *Router) srvPwDelDo(chat, mid int64, idx string) {
	pws := r.srvPwList()
	i := atoiSafe(idx)
	if i >= 0 && i < len(pws) {
		pws = append(pws[:i], pws[i+1:]...)
	}
	r.Store.PutJSON("srv_passwords", pws, 0)
	r.srvPwHome(chat, mid)
}

func (r *Router) srvSubSrc(chat, mid int64) {
	r.Show(chat, mid, "➕ افزودن سرور از ساب‌دامنه\n\nساب‌ها از کجا بیایند؟",
		[][]Btn{{btn("📁 از دامنه‌ها", "srvsubzone")},
			{btn("⭐ از منتخب‌ها", "srvsubfav")},
			{btn("⬅️ انصراف", "srv")}})
}

func (r *Router) srvSubZones(chat, mid int64) {
	zs := r.cfAllZones()
	if len(zs) == 0 {
		r.Show(chat, mid, "📭 دامنه‌ای یافت نشد.",
			[][]Btn{{btn("🔙 بازگشت", "srvsub"), btn("🏠 خانه", "menu")}})
		return
	}
	var items []Btn
	for _, z := range zs {
		items = append(items, btn("📁 "+z.Zone.Name, fmt.Sprintf("srvsubz:%d:%s", z.Acc, z.Zone.ID)))
	}
	rows := gridBTN(items, 2)
	rows = append(rows, navRow("srvsub"))
	r.Show(chat, mid, "📁 دامنه را انتخاب کنید:", rows)
}

func (r *Router) favs() []map[string]any {
	var favs []map[string]any
	if !r.Store.GetJSON("favs", &favs) {
		return nil
	}
	return favs
}

func (r *Router) srvSubFavs(chat, mid int64) {
	favs := r.favs()
	if len(favs) == 0 {
		r.Show(chat, mid, "📭 موردی در منتخب‌ها نیست.",
			[][]Btn{{btn("🔙 بازگشت", "srvsub"), btn("🏠 خانه", "menu")}})
		return
	}
	byZone := map[string][]cf.Record{}
	var items []map[string]any
	for _, f := range favs {
		key := fmt.Sprintf("%v|%v", f["acc"], f["zid"])
		if _, ok := byZone[key]; !ok {
			cli, ok2 := r.cfClientFor(intOfAny(f["acc"]))
			if ok2 {
				recs, _ := cli.Records(strv(f["zid"]))
				byZone[key] = recs
			} else {
				byZone[key] = nil
			}
		}
		var rec *cf.Record
		for i := range byZone[key] {
			if byZone[key][i].ID == strv(f["rid"]) {
				rec = &byZone[key][i]
				break
			}
		}
		if rec == nil {
			continue
		}
		items = append(items, subItem(rec.Name, rec.Name, strings.ToUpper(rec.Type), rec.Content))
	}
	if len(items) == 0 {
		r.Show(chat, mid, "📭 رکوردی در منتخب‌ها نیست.",
			[][]Btn{{btn("🔙 بازگشت", "srvsub"), btn("🏠 خانه", "menu")}})
		return
	}
	r.srvSubStore(chat, "⭐ منتخب‌ها", items)
	r.srvSubRender(chat, mid)
}

func subItem(label, name, typ, content string) map[string]any {
	if label == "" {
		label = "?"
	}
	return map[string]any{"label": label, "name": name, "type": typ, "content": content}
}

func (r *Router) srvSubPick(chat, mid int64, acc, zid string) {
	cli, ok := r.cfClientFor(atoiSafe(acc))
	if !ok {
		r.Show(chat, mid, "❌ خطای کلادفلر", [][]Btn{{btn("🔙 بازگشت", "srvsubzone")}})
		return
	}
	recs, err := cli.Records(zid)
	if err != nil {
		r.Show(chat, mid, "❌ خطای کلادفلر: "+Esc(err.Error()), [][]Btn{{btn("🔙 بازگشت", "srvsubzone")}})
		return
	}
	zname := ""
	for _, z := range r.cfAllZones() {
		if z.Zone.ID == zid {
			zname = z.Zone.Name
			break
		}
	}
	var items []map[string]any
	for _, x := range recs {
		t := strings.ToUpper(x.Type)
		if t != "A" && t != "AAAA" && t != "CNAME" {
			continue
		}
		items = append(items, subItem(x.Name, x.Name, t, x.Content))
	}
	if len(items) == 0 {
		r.Show(chat, mid, "📭 رکورد A/AAAA/CNAME در این دامنه نیست.", [][]Btn{{btn("🔙 بازگشت", "srvsubzone")}})
		return
	}
	r.srvSubStore(chat, "📁 "+zname, items)
	r.srvSubRender(chat, mid)
}

func (r *Router) srvSubStore(chat int64, title string, items []map[string]any) {
	r.Store.PutJSON(fmt.Sprintf("srvsub:%d", chat),
		map[string]any{"title": title, "items": items, "ids": []any{}, "page": 0}, 3600)
}

func (r *Router) srvSubGet(chat int64) map[string]any {
	var st map[string]any
	r.Store.GetJSON(fmt.Sprintf("srvsub:%d", chat), &st)
	return st
}

func (r *Router) srvSubRender(chat, mid int64) {
	st := r.srvSubGet(chat)
	items, _ := st["items"].([]any)
	if st == nil || len(items) == 0 {
		r.Store.Delete(fmt.Sprintf("srvsub:%d", chat))
		r.Show(chat, mid, "⏳ نشست منقضی شده.", [][]Btn{{btn("🖥 سرورها و مانیتورینگ", "srv")}})
		return
	}
	chosen := map[int]bool{}
	if ids, ok := st["ids"].([]any); ok {
		for _, x := range ids {
			chosen[intOfAny(x)] = true
		}
	}
	per := 8
	pages := (len(items) + per - 1) / per
	if pages < 1 {
		pages = 1
	}
	page := intOfAny(st["page"])
	if page < 0 {
		page = 0
	}
	if page > pages-1 {
		page = pages - 1
	}
	st["page"] = page
	r.Store.PutJSON(fmt.Sprintf("srvsub:%d", chat), st, 3600)
	lo, hi := page*per, page*per+per
	if hi > len(items) {
		hi = len(items)
	}
	sl := items[lo:hi]
	var kb [][]Btn
	for i := 0; i < len(sl); i += 2 {
		var row []Btn
		for j := i; j < i+2; j++ {
			if j < len(sl) {
				idx := lo + j
				mark := "⬜"
				if chosen[idx] {
					mark = "✅"
				}
				m, _ := sl[j].(map[string]any)
				row = append(row, btn(fmt.Sprintf("%s %s", mark, strv(m["label"])), fmt.Sprintf("srvsubsel:%d", idx)))
			} else {
				row = append(row, emptyBtn())
			}
		}
		kb = append(kb, row)
	}
	var nav []Btn
	if page > 0 {
		nav = append(nav, btn("◀️", fmt.Sprintf("srvsubpg:%d", page-1)))
	} else {
		nav = append(nav, emptyBtn())
	}
	nav = append(nav, btn(fmt.Sprintf("📄 %d/%d", page+1, pages), "noop"))
	if page < pages-1 {
		nav = append(nav, btn("▶️", fmt.Sprintf("srvsubpg:%d", page+1)))
	} else {
		nav = append(nav, emptyBtn())
	}
	kb = append(kb, nav)
	kb = append(kb, []Btn{btn(fmt.Sprintf("✅ افزودن (%d)", len(chosen)), "srvsubsave"), btn("❌ انصراف", "srvsubcancel")})
	r.Show(chat, mid, fmt.Sprintf("➕ افزودن سرور از ساب‌دامنه — %s\n\nساب‌ها را انتخاب کنید (⬜ → ✅):", Esc(strv(st["title"]))), kb)
}

func (r *Router) srvSubToggle(chat, mid int64, idx string) {
	st := r.srvSubGet(chat)
	items, _ := st["items"].([]any)
	if st == nil || len(items) == 0 {
		r.srvSubRender(chat, mid)
		return
	}
	i := atoiSafe(idx)
	if i < 0 || i >= len(items) {
		r.srvSubRender(chat, mid)
		return
	}
	set := map[int]bool{}
	if ids, ok := st["ids"].([]any); ok {
		for _, x := range ids {
			set[intOfAny(x)] = true
		}
	}
	if set[i] {
		delete(set, i)
	} else {
		set[i] = true
	}
	var sorted []any
	for k := 0; k < len(items); k++ {
		if set[k] {
			sorted = append(sorted, k)
		}
	}
	st["ids"] = sorted
	r.Store.PutJSON(fmt.Sprintf("srvsub:%d", chat), st, 3600)
	r.srvSubRender(chat, mid)
}

func (r *Router) srvSubPage(chat, mid int64, page string) {
	st := r.srvSubGet(chat)
	if st != nil {
		st["page"] = atoiSafe(page)
		r.Store.PutJSON(fmt.Sprintf("srvsub:%d", chat), st, 3600)
	}
	r.srvSubRender(chat, mid)
}

func (r *Router) srvSubSave(chat, mid int64) {
	st := r.srvSubGet(chat)
	if st == nil || st["ids"] == nil || lenAny(st["ids"]) == 0 {
		r.Show(chat, mid, "⚠️ هیچ سابی انتخاب نشده.", [][]Btn{{btn("🔙 بازگشت", "srvsub")}})
		return
	}
	cands, skipped := r.srvSubCandidates(st)
	if len(cands) == 0 {
		r.Show(chat, mid, "❌ هیچ آی‌پی قابل‌استفاده‌ای پیدا نشد.",
			[][]Btn{{btn("🔙 بازگشت", "srvsubcancel")}})
		return
	}
	r.Store.PutJSON(fmt.Sprintf("srvsubsel:%d", chat),
		map[string]any{"cands": cands, "skipped": skipped}, 3600)
	pws := r.srvPwList()
	var kb [][]Btn
	for i := 0; i < len(pws); i += 3 {
		var row []Btn
		for j := i; j < i+3 && j < len(pws); j++ {
			row = append(row, btn(fmt.Sprintf("🔒 رمز %d", j+1), fmt.Sprintf("srvsubpw:%d", j)))
		}
		kb = append(kb, row)
	}
	kb = append(kb, []Btn{btn("🔓 بدون رمز", "srvsubpw:none"), btn("❌ انصراف", "srvsubcancel")})
	r.Show(chat, mid, fmt.Sprintf("🔐 رمز SSH برای %d سرور جدید؟\n\nیک رمز ذخیره‌شده انتخاب کن یا رمز را بفرست («-» یعنی بدون رمز):", len(cands)), kb)
	r.setWizard(chat, map[string]any{"kind": "srv_sub_pw"})
}

func lenAny(v any) int {
	if arr, ok := v.([]any); ok {
		return len(arr)
	}
	return 0
}

func (r *Router) srvSubCandidates(st map[string]any) ([]map[string]any, int) {
	items, _ := st["items"].([]any)
	ids, _ := st["ids"].([]any)
	var out []map[string]any
	skipped := 0
	for _, iv := range ids {
		i := intOfAny(iv)
		if i < 0 || i >= len(items) {
			skipped++
			continue
		}
		it, _ := items[i].(map[string]any)
		name := strings.TrimSpace(strv(it["name"]))
		t := strings.ToUpper(strv(it["type"]))
		content := strings.TrimSpace(strv(it["content"]))
		ip := ""
		if (t == "A" || t == "AAAA") && content != "" {
			ip = content
		} else if name != "" {
			ips, err := net.LookupIP(name)
			if err == nil {
				for _, x := range ips {
					if v4 := x.To4(); v4 != nil {
						ip = v4.String()
						break
					}
				}
			}
		}
		if ip == "" {
			skipped++
			continue
		}
		out = append(out, map[string]any{"name": name, "host": ip, "port": 22})
	}
	return out, skipped
}

func (r *Router) srvSubFinish(chat, mid int64, sel string, stored bool, pwOverride *string) {
	var st map[string]any
	if !r.Store.GetJSON(fmt.Sprintf("srvsubsel:%d", chat), &st) {
		r.Show(chat, mid, "⏳ نشست منقضی شده.", [][]Btn{{btn("🔙 بازگشت", "srv")}})
		return
	}
	pw := ""
	if pwOverride != nil {
		pw = *pwOverride
	} else if stored && sel != "none" {
		pws := r.srvPwList()
		i := atoiSafe(sel)
		if i >= 0 && i < len(pws) {
			pw = strv(pws[i])
		}
	}
	cands, _ := st["cands"].([]any)
	skipped := intOfAny(st["skipped"])
	lst := r.srvList()
	have := map[string]bool{}
	for _, s := range lst {
		have[strings.ToLower(strv(s["name"]))] = true
	}
	added, dup := 0, 0
	for _, cv := range cands {
		c, _ := cv.(map[string]any)
		key := strings.ToLower(strv(c["name"]))
		if have[key] {
			dup++
			continue
		}
		have[key] = true
		auth := "none"
		if pw != "" {
			auth = "pass"
		}
		lst = append(lst, srvRec(fmt.Sprintf("s%d%d%d", time.Now().Unix(), rand.Intn(900)+100, added),
			truncRune(strv(c["name"]), 60), strv(c["host"]), "root", 22, auth, pw, "افزوده از ساب‌دامنه "+strv(c["name"])))
		added++
	}
	r.srvSave(lst)
	r.Store.Delete(fmt.Sprintf("srvsubsel:%d", chat))
	r.Store.Delete(fmt.Sprintf("srvsub:%d", chat))
	r.clearWizard(chat)
	r.Bot.Send(chat, fmt.Sprintf("✅ %d سرور افزوده شد.\n🔁 تکراری: %d — ⏭ رد‌شده: %d", added, dup, skipped), nil)
	r.srvHome(chat, 0)
}

func (r *Router) srvmonHome(chat, mid int64) {
	cfg := srvmon.Cfg(r.Store)
	state := "⏸ متوقف"
	if boolVal(cfg["enabled"]) {
		state = "▶️ فعال"
	}
	txt := strings.Join([]string{
		"📊 مانیتور سرورها", "",
		fmt.Sprintf("وضعیت: %s", state), "",
		"هشدار خودکار وقتی:",
		fmt.Sprintf("• CPU بیش از %s%% برود", hzF(floatAny(cfg["cpuPct"]))),
		fmt.Sprintf("• RAM بیش از %s%% برود", hzF(floatAny(cfg["memPct"]))),
		fmt.Sprintf("• دیسک بیش از %s%% پر شود", hzF(floatAny(cfg["diskPct"]))),
		fmt.Sprintf("⏱ حداقل فاصلهٔ تکرار هشدار: %s دقیقه", hzF(floatAny(cfg["cooldownMin"]))),
		"", "برای تغییر هر آستانه، روی آن بزنید."}, "\n")
	toggle := "▶️ فعال‌سازی"
	if boolVal(cfg["enabled"]) {
		toggle = "⏸ توقف هشدار"
	}
	kb := [][]Btn{
		{btn(toggle, "srvmontg")},
		{btn(fmt.Sprintf("⚙️ CPU: %s%%", hzF(floatAny(cfg["cpuPct"]))), "srvmonh:cpu"),
			btn(fmt.Sprintf("🧠 RAM: %s%%", hzF(floatAny(cfg["memPct"]))), "srvmonh:mem")},
		{btn(fmt.Sprintf("🗄 دیسک: %s%%", hzF(floatAny(cfg["diskPct"]))), "srvmonh:disk"),
			btn(fmt.Sprintf("⏱ تکرار: %sd", hzF(floatAny(cfg["cooldownMin"]))), "srvmonh:cool")},
		{btn("📊 بررسی الان", "srvmonrun")},
		{btn("🏠 خانه", "menu")}}
	r.Show(chat, mid, txt, kb)
}

func (r *Router) srvmonToggle(chat, mid int64) {
	cfg := srvmon.Cfg(r.Store)
	cfg["enabled"] = !boolVal(cfg["enabled"])
	srvmon.Save(r.Store, cfg)
	r.srvmonHome(chat, mid)
}

func (r *Router) srvmonRun(chat int64) {
	lst := r.srvList()
	n := srvmon.Run(r.Store, lst, r.AdminID, func(s map[string]any) map[string]any {
		return sshop.Stats(strv(s["host"]), orDefault(strv(s["user"]), "root"), strv(s["secret"]), intOfAny(s["port"]))
	}, r.Bot, true)
	if n == 0 {
		r.Bot.Send(chat, "✅ بررسی شد؛ هشدار تازه‌ای نیست.", nil)
	}
}

func (r *Router) srvStats(chat, mid int64, idx string) {
	lst := r.srvList()
	i := atoiSafe(idx)
	if i < 0 || i >= len(lst) {
		r.Show(chat, mid, "❌ سرور پیدا نشد.", [][]Btn{{btn("🖥 سرورها و مانیتورینگ", "srv")}})
		return
	}
	s := lst[i]
	r.Show(chat, mid, "⏳ در حال جمع‌آوری آمار…", HomeKB())
	st := sshop.Stats(strv(s["host"]), orDefault(strv(s["user"]), "root"), strv(s["secret"]), intOfAny(s["port"]))
	if e := strvVal(st["error"]); e != "" {
		r.Show(chat, mid, fmt.Sprintf("❌ وصل نشد: <code>%s</code>", Esc(truncRune(e, 200))),
			[][]Btn{{btn("🔄 بروزرسانی", fmt.Sprintf("srvstats:%d", i)), btn("⚙️ آستانه‌ها", "srvmon")},
				{btn("🔙 بازگشت", fmt.Sprintf("srvopen:%s", strv(s["id"]))), btn("🏠 خانه", "menu")}})
		return
	}
	uptime := intOfAny(st["uptimeS"])
	upD := uptime / 86400
	upH := (uptime % 86400) / 3600
	mem, _ := st["mem"].(map[string]any)
	disk, _ := st["disk"].(map[string]any)
	cpu, _ := st["cpu"].(map[string]any)
	shortIP := truncRune(strv(s["host"]), 15)
	txt := fmt.Sprintf("📊 %s\n⏱ %d روز و %d ساعت", strv(s["name"]), upD, upH)
	kb := [][]Btn{
		{btn(fmt.Sprintf("🌐 %s", shortIP), "noop"), btn(fmt.Sprintf("⚙️ %d%%", intOfAny(cpu["pct"])), "noop"),
			btn(fmt.Sprintf("🧠 %d%%", intOfAny(mem["pct"])), "noop"), btn(fmt.Sprintf("🗄 %s", strv(disk["pct"])), "noop")},
		{btn(fmt.Sprintf("⚙️ %v هسته", orDash(strvVal(cpu["cores"]))), "noop"),
			btn(fmt.Sprintf("🧠 %s/%s", srvmon.FmtGB(mem["usedMb"]), srvmon.FmtGB(mem["totalMb"])), "noop"),
			btn(fmt.Sprintf("🗄 %s/%s", strv(disk["used"]), strv(disk["total"])), "noop"),
			btn(fmt.Sprintf("⏱ %dr %dh", upD, upH), "noop")},
		{btn("🔄 بروزرسانی", fmt.Sprintf("srvstats:%d", i)), btn("⚙️ آستانه‌ها", "srvmon")},
		{btn("🔙 بازگشت", fmt.Sprintf("srvopen:%s", strv(s["id"]))), btn("🏠 خانه", "menu")}}
	r.Show(chat, mid, txt, kb)
}

func (r *Router) srvImport(chat, mid int64) {
	r.Show(chat, mid, "⏳ در حال خواندن سرورها از دیتاسنترها…", HomeKB())
	var found []map[string]any
	for _, a := range r.hzAccs() {
		servers, err := hz.New(strv(a["token"])).Call("/servers", "GET", nil)
		if err != nil {
			continue
		}
		for _, s := range hzList(servers, "servers") {
			ip := hzIP(s)
			if ip != "" {
				name := strv(s["name"])
				if name == "" {
					name = ip
				}
				found = append(found, map[string]any{"host": ip, "name": name, "note": "Hetzner: " + strv(a["name"])})
			}
		}
	}
	for _, a := range r.lnAccs() {
		for _, s := range ln.New(strv(a["token"])).All("/linode/instances") {
			ip := lnFirstIP(s)
			if ip != "" && ip != "—" {
				name := strv(s["label"])
				if name == "" {
					name = ip
				}
				found = append(found, map[string]any{"host": ip, "name": name, "note": "Linode: " + strv(a["name"])})
			}
		}
	}
	lst := r.srvList()
	have := map[string]bool{}
	for _, s := range lst {
		have[strings.ToLower(strv(s["host"]))] = true
	}
	n := 0
	for _, f := range found {
		if have[strings.ToLower(strv(f["host"]))] {
			continue
		}
		lst = append(lst, srvRec(fmt.Sprintf("s%d%d%d", time.Now().Unix(), rand.Intn(90)+10, n),
			strv(f["name"]), strv(f["host"]), "root", 22, "pass", "", strv(f["note"])))
		n++
	}
	r.srvSave(lst)
	extra := ""
	if n > 0 {
		extra = " (رمز ندارند؛ ویرایش کنید.)"
	}
	r.Bot.Send(chat, fmt.Sprintf("✅ %d سرور تازه اضافه شد.%s", n, extra),
		[][]Btn{{btn("🖥 سرورها و مانیتورینگ", "srv")}, {btn("🏠 خانه", "menu")}})
}

func (r *Router) srvWizAddHost(chat, mid int64, pend map[string]any, text string) {
	host := strings.Fields(strings.TrimSpace(text))
	if len(host) == 0 {
		r.Bot.Send(chat, "❌ هاست معتبر نیست.", nil)
		return
	}
	r.setWizard(chat, map[string]any{"kind": "srv_add_user", "host": host[0]})
	r.Bot.Send(chat, "👤 یوزر SSH؟ (پیش‌فرض root — بنویسید <code>root</code>):", nil)
}

func (r *Router) srvWizAddSecret(chat, mid int64, pend map[string]any, text string) {
	lst := r.srvList()
	host := strvVal(pend["host"])
	user := orDefault(strings.TrimSpace(strvVal(pend["user"])), "root")
	auth := "pass"
	if strings.HasPrefix(strings.TrimSpace(text), "-----BEGIN") {
		auth = "key"
	}
	lst = append(lst, srvRec(fmt.Sprintf("s%d%d%d", time.Now().Unix(), rand.Intn(90)+10, len(lst)),
		host, host, user, 22, auth, text, ""))
	r.srvSave(lst)
	r.clearWizard(chat)
	r.Bot.Send(chat, "✅ سرور اضافه شد.", nil)
	r.srvHome(chat, 0)
}

func (r *Router) srvWizAddUserSave(chat, mid int64, pend map[string]any, text string) {
	host := strvVal(pend["host"])
	user := strings.TrimSpace(text)
	if user == "" {
		user = "root"
	}
	r.setWizard(chat, map[string]any{"kind": "srv_add_secret", "host": host, "user": user})
	r.Bot.Send(chat, "🔒 رمز عبور (یا کلید خصوصی SSH) را بفرستید:", nil)
}

func (r *Router) srvWizBulk(chat, mid int64, text string) {
	lst := r.srvList()
	n := 0
	for _, ln := range strings.Split(text, "\n") {
		p := strings.Fields(ln)
		if len(p) == 0 {
			continue
		}
		user := "root"
		if len(p) > 1 {
			user = p[1]
		}
		sec := ""
		if len(p) > 2 {
			sec = p[2]
		}
		port := 22
		if len(p) > 3 && isDigits(p[3]) {
			port = atoiSafe(p[3])
		}
		lst = append(lst, srvRec(fmt.Sprintf("s%d%d%d", time.Now().Unix(), rand.Intn(90)+10, n),
			p[0], p[0], user, port, "pass", sec, ""))
		n++
	}
	r.srvSave(lst)
	r.clearWizard(chat)
	r.Bot.Send(chat, fmt.Sprintf("✅ %d سرور اضافه شد.", n), nil)
	r.srvHome(chat, 0)
}

func (r *Router) srvWizEdit(chat, mid int64, pend map[string]any, text string) {
	sid := strvVal(pend["srv"])
	field := strvVal(pend["field"])
	lst := r.srvList()
	i, s := r.srvGet(sid)
	if s == nil {
		r.clearWizard(chat)
		r.Bot.Send(chat, "❌ سرور پیدا نشد.", nil)
		return
	}
	val := strings.TrimSpace(text)
	switch field {
	case "name":
		if val != "" {
			s["name"] = val
		}
	case "host":
		host, port := val, intOfAny(s["port"])
		if port == 0 {
			port = 22
		}
		if idx := strings.LastIndex(val, ":"); idx >= 0 && isDigits(val[idx+1:]) {
			host = val[:idx]
			port = atoiSafe(val[idx+1:])
		}
		s["host"] = host
		s["port"] = port
	case "user":
		s["user"] = orDefault(val, "root")
	case "auth":
		if val == "-" {
			break
		}
		if strings.HasPrefix(val, "-") {
			s["auth"] = "key"
			s["secret"] = text
		} else {
			s["auth"] = "pass"
			s["secret"] = val
		}
	case "note":
		if val == "-" {
			s["note"] = ""
		} else {
			s["note"] = val
		}
	}
	lst[i] = s
	r.srvSave(lst)
	r.clearWizard(chat)
	r.Bot.Send(chat, "✅ ذخیره شد.", nil)
	r.srvOpen(chat, 0, sid)
}

func (r *Router) srvPwAddSave(chat, mid int64, text string) {
	pws := r.srvPwList()
	pws = append(pws, strings.TrimSpace(text))
	r.Store.PutJSON("srv_passwords", pws, 0)
	r.clearWizard(chat)
	r.Bot.Send(chat, "✅ رمز ذخیره شد.", nil)
	r.srvPwHome(chat, 0)
}

func (r *Router) srvmonNumSave(chat, mid int64, pend map[string]any, text string) {
	v := atofSafe(text)
	if v <= 0 {
		r.Bot.Send(chat, "❌ عدد نامعتبر است.", nil)
		return
	}
	keyMap := map[string]string{"cpu": "cpuPct", "mem": "memPct", "disk": "diskPct", "cool": "cooldownMin"}
	key := keyMap[strvVal(pend["key"])]
	if key == "" {
		key = "cpuPct"
	}
	cfg := srvmon.Cfg(r.Store)
	cfg[key] = v
	srvmon.Save(r.Store, cfg)
	r.clearWizard(chat)
	r.srvmonHome(chat, mid)
}

func atofSafe(s string) float64 {
	f, err := strconv.ParseFloat(strings.TrimSpace(s), 64)
	if err != nil {
		return 0
	}
	return f
}
