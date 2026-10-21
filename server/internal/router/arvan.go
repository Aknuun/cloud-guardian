package router

import (
	"fmt"
	"strings"

	"github.com/aknuun/cg-server/internal/arvan"
)

func (r *Router) arvanAccList() []map[string]any {
	var accs []map[string]any
	if !r.Store.GetJSON("arvan_accounts", &accs) {
		return nil
	}
	return accs
}

func (r *Router) arvanSaveAccs(accs []map[string]any) {
	r.Store.PutJSON("arvan_accounts", accs, 0)
}

func arBool(v any) bool {
	switch t := v.(type) {
	case bool:
		return t
	case string:
		return t == "true" || t == "1"
	case float64:
		return t != 0
	case map[string]any:
		return len(t) > 0
	}
	return false
}

func arvanName(d map[string]any) string {
	if s := strv(d["name"]); s != "" {
		return s
	}
	if s := strv(d["domain"]); s != "" {
		return s
	}
	return "?"
}

func (r *Router) arvanHome(chat, mid int64) {
	accs := r.arvanAccList()
	names := "—"
	if len(accs) > 0 {
		var parts []string
		for _, a := range accs {
			parts = append(parts, "<code>"+Esc(strv(a["name"]))+"</code>")
		}
		names = strings.Join(parts, "، ")
	}
	r.Show(chat, mid,
		"🇮🇷 آروان کلاد\n\n"+
			"اکانت‌ها: "+names+"\n\n"+
			"کلید ماشین‌یوزر (npanel → Settings → IAM → Machine users) "+
			"با دسترسی‌های CDN و Cloud Server لازم است.",
		[][]Btn{
			{btn("🌐 دامنه‌ها (DNS)", "arvdoms"), btn("🖥 سرورهای ابری", "arvsrv")},
			{btn("👤 اکانت‌ها", "arvanacc"), btn("🔙 بازگشت", "menu")}})
}

func arvKeyGuideKB() [][]Btn {
	return [][]Btn{
		{btnURL("🛡 ساخت Policy", "https://panel.arvancloud.ir/profile/iam/policies"),
			btnURL("🤖 Machine Users", "https://panel.arvancloud.ir/profile/iam/machine-users")},
		{btn("❌ انصراف", "arvanacc")},
	}
}

func (r *Router) arvanAccHelp(chat, mid int64) {
	r.setWizard(chat, map[string]any{"kind": "arvan_key"})
	r.Show(chat, mid, arvKeyGuideText(), arvKeyGuideKB())
}

func arvKeyGuideText() string {
	return "🔑 <b>افزودن اکانت آروان — راهنمای کامل</b>\n\n" +
		"<b>۱) ساخت Policy (دسترسی‌ها)</b>\n" +
		"• دکمهٔ «🛡 ساخت Policy» را بزن.\n" +
		"• <b>Create Policy</b> → نام: <code>cloud-guardian</code>\n" +
		"• این دسترسی‌ها را فعال کن:\n" +
		"  • CDN: <code>domains</code> و <code>dns-records</code> (خواندن/نوشتن)\n" +
		"  • ECC: <code>servers</code> (خواندن/نوشتن)، <code>sizes</code>، <code>images</code>، " +
		"<code>networks</code>، <code>security-groups</code>، <code>snapshots</code>\n" +
		"• ذخیره کن.\n\n" +
		"<b>۲) ساخت Machine User</b>\n" +
		"• دکمهٔ «🤖 Machine Users» → <b>Create Machine User</b>\n" +
		"• نام: <code>cloud-guardian</code>\n" +
		"• Policy ساخته‌شده را وصل کن (اگر چند پروژه داری، همه را انتخاب کن).\n" +
		"• بعد از ساخت، <b>API Key</b> را کپی کن — فقط یک‌بار نمایش داده می‌شود.\n\n" +
		"<b>۳) فرستادن کلید</b>\n" +
		"• کلید را همین‌جا بفرست (با یا بدون پیشوند <code>Apikey</code>).\n" +
		"• ربات خودش اعتبار کلید را تست می‌کند.\n\n" +
		"⚠️ اگر دسترسی‌ها ناقص باشند، ساخت/شکار سرور با خطای 401/403 متوقف می‌شود."
}

func (r *Router) arvanAccsPage(chat, mid int64) {
	accs := r.arvanAccList()
	var kb [][]Btn
	for i, a := range accs {
		kb = append(kb, []Btn{
			btn("👤 "+strv(a["name"]), "arvanacc"),
			btnS("🗑", fmt.Sprintf("arvanaccdel:%d", i), "danger")})
	}
	kb = append(kb, []Btn{btn("➕ افزودن اکانت", "arvanaccadd"), btn("📘 راهنما", "arvanacchelp")})
	kb = append(kb, []Btn{btn("🔙 بازگشت", "arvan"), btn("🏠 خانه", "menu")})
	r.Show(chat, mid,
		"🇮🇷 آروان — اکانت‌ها\n\nکلید را از پنل آروان "+
			"(Settings → Machine User) بسازید.", kb)
}

func (r *Router) arvanAccDelAsk(chat, mid int64, idx string) {
	accs := r.arvanAccList()
	i := atoiSafe(idx)
	if i < 0 || i >= len(accs) {
		r.Show(chat, mid, "❌ اکانت پیدا نشد.", HomeKB())
		return
	}
	name := strv(accs[i]["name"])
	r.Show(chat, mid, fmt.Sprintf("⚠️ اکانت «%s» حذف شود؟", Esc(name)),
		[][]Btn{{btn("✅ بله، حذف کن", fmt.Sprintf("arvanaccdelok:%s", idx)),
			btn("❌ انصراف", "arvanacc")}})
}

func (r *Router) arvanAccDelDo(chat, mid int64, idx string) {
	accs := r.arvanAccList()
	i := atoiSafe(idx)
	if i < 0 || i >= len(accs) {
		r.Show(chat, mid, "❌ اکانت پیدا نشد.", HomeKB())
		return
	}
	accs = append(accs[:i], accs[i+1:]...)
	r.arvanSaveAccs(accs)
	r.Bot.Send(chat, "✅ اکانت حذف شد.", nil)
	r.arvanAccsPage(chat, 0)
}

func (r *Router) arvanDoms(chat, mid int64) {
	accs := r.arvanAccList()
	if len(accs) == 0 {
		r.Show(chat, mid, "📭 اکانتی نیست.",
			[][]Btn{{btn("➕ افزودن اکانت", "arvanaccadd"), btn("📘 راهنما", "arvanacchelp")},
				{btn("🔙 بازگشت", "arvan")}})
		return
	}
	var items []Btn
	for i, a := range accs {
		doms := arvan.New(strv(a["token"])).Domains()
		if len(doms) == 0 {
			continue
		}
		for _, d := range doms {
			if len(items) >= 20 {
				break
			}
			dn := arvanName(d)
			items = append(items, btn("🇮🇷 "+dn, fmt.Sprintf("arvdom:%d:%s", i, dn)))
		}
	}
	kb := gridBTN(items, 2)
	kb = append(kb, navRow("arvan"))
	r.Show(chat, mid, "🌐 دامنه‌های آروان:", kb)
}

func (r *Router) arvanRecs(chat, mid int64, acc int, dom string, pg int) {
	accs := r.arvanAccList()
	if acc < 0 || acc >= len(accs) {
		r.Show(chat, mid, "❌ اکانت پیدا نشد.", HomeKB())
		return
	}
	recs := arvan.New(strv(accs[acc]["token"])).Records(dom)
	results := map[string]any{}
	for i, rec := range recs {
		if i >= 100 {
			break
		}
		results[strv(rec["id"])] = map[string]any{"acc": acc, "dom": dom}
	}
	r.Store.PutJSON(fmt.Sprintf("cfctx:%d", chat),
		map[string]any{"acc": fmt.Sprintf("arv%d", acc), "dom": dom, "results": results}, 86400)
	per := 30
	pages := (len(recs) + per - 1) / per
	if pages < 1 {
		pages = 1
	}
	if pg < 0 {
		pg = 0
	}
	if pg > pages-1 {
		pg = pages - 1
	}
	lo, hi := pg*per, pg*per+per
	if hi > len(recs) {
		hi = len(recs)
	}
	var rows []Btn
	for _, rec := range recs[lo:hi] {
		nm := strv(rec["name"])
		if nm == "" {
			nm = strv(rec["host"])
		}
		if nm == "" {
			nm = "?"
		}
		rows = append(rows, btn(fmt.Sprintf("%s-%s", nm, strv(rec["type"])),
			fmt.Sprintf("arvrec:%d:%s:%s", acc, dom, strv(rec["id"]))))
	}
	kb := gridBTN(rows, 3)
	kb = append(kb, []Btn{btn("➕ افزودن", fmt.Sprintf("arvaadd:%d:%s", acc, dom))})
	if pages > 1 {
		var nav []Btn
		if pg > 0 {
			nav = append(nav, btn("◀️", fmt.Sprintf("arvdom:%d:%s", acc, dom)))
		} else {
			nav = append(nav, emptyBtn())
		}
		nav = append(nav, btn(fmt.Sprintf("📄 %d/%d", pg+1, pages), "noop"))
		nav = append(nav, emptyBtn())
		kb = append(kb, nav)
	}
	kb = append(kb, []Btn{btn("🔙 بازگشت", "arvdoms"), btn("🏠 خانه", "menu")})
	r.Show(chat, mid, fmt.Sprintf("📋 %s (%d رکورد):", dom, len(recs)), kb)
}

func (r *Router) arvanRec(chat, mid int64, acc int, dom, rid string) {
	accs := r.arvanAccList()
	if acc < 0 || acc >= len(accs) {
		r.Show(chat, mid, "❌ اکانت پیدا نشد.", HomeKB())
		return
	}
	recs := arvan.New(strv(accs[acc]["token"])).Records(dom)
	var found map[string]any
	for _, rec := range recs {
		if strv(rec["id"]) == strv(rid) {
			found = rec
			break
		}
	}
	if found == nil {
		r.Show(chat, mid, "❌ رکورد پیدا نشد.", HomeKB())
		return
	}
	cloud := arBool(found["cloud"]) || arBool(found["cdn"])
	cloudStr := "خاموش"
	if cloud {
		cloudStr = "روشن"
	}
	val := strv(found["value"])
	if val == "" {
		val = strv(found["content"])
	}
	txt := fmt.Sprintf("📝 <b>%s</b>\nنوع: %s\nمقدار: <code>%s</code>\nابری: %s",
		Esc(arvanName(found)), Esc(strv(found["type"])), Esc(val), cloudStr)
	kb := [][]Btn{
		{btn("✏️ تغییر مقدار", fmt.Sprintf("arvev:%d:%s:%s", acc, dom, rid)),
			btn("☁️ ابری", fmt.Sprintf("arvcloud:%d:%s:%s", acc, dom, rid))},
		{btnS("🗑 حذف", fmt.Sprintf("arvdel:%d:%s:%s", acc, dom, rid), "danger")},
		{btn("🔙 بازگشت", fmt.Sprintf("arvdom:%d:%s", acc, dom)), btn("🏠 خانه", "menu")}}
	r.Show(chat, mid, txt, kb)
}

func (r *Router) arvanDelDo(chat, mid int64, acc int, dom, rid string) {
	accs := r.arvanAccList()
	if acc < 0 || acc >= len(accs) {
		r.Show(chat, mid, "❌ اکانت پیدا نشد.", HomeKB())
		return
	}
	if _, err := arvan.New(strv(accs[acc]["token"])).Call("DELETE",
		fmt.Sprintf("/domains/%s/dns-records/%s", dom, rid), nil); err != nil {
		r.Show(chat, mid, "❌ نشد: "+Esc(err.Error()), HomeKB())
		return
	}
	r.Bot.Send(chat, "✅ حذف شد.", nil)
	r.arvanRecs(chat, 0, acc, dom, 0)
}

func (r *Router) arvanCloud(chat, mid int64, acc int, dom, rid string) {
	accs := r.arvanAccList()
	if acc < 0 || acc >= len(accs) {
		r.Show(chat, mid, "❌ اکانت پیدا نشد.", HomeKB())
		return
	}
	cli := arvan.New(strv(accs[acc]["token"]))
	recs := cli.Records(dom)
	var found map[string]any
	for _, rec := range recs {
		if strv(rec["id"]) == strv(rid) {
			found = rec
			break
		}
	}
	if found == nil {
		r.Show(chat, mid, "❌ رکورد پیدا نشد.", HomeKB())
		return
	}
	cur := arBool(found["cloud"]) || arBool(found["cdn"])
	if _, err := cli.Call("PUT", fmt.Sprintf("/domains/%s/dns-records/%s", dom, rid),
		map[string]any{"cloud": !cur}); err != nil {
		r.Show(chat, mid, "❌ نشد: "+Esc(err.Error()), HomeKB())
		return
	}
	state := "روشن"
	if cur {
		state = "خاموش"
	}
	r.Bot.Send(chat, "✅ ابری "+state+" شد.", nil)
	r.arvanRec(chat, 0, acc, dom, rid)
}

func (r *Router) arvanKeySave(chat, mid int64, text string) {
	key := arvan.NormKey(text)
	if key == "" {
		r.Bot.Send(chat, "❌ کلید معتبر نیست.", nil)
		return
	}
	if _, err := arvan.New(key).Call("GET", "/domains?per_page=1", nil); err != nil {
		r.Bot.Send(chat, "❌ کلید قبول نشد:\n"+Esc(err.Error()), nil)
		return
	}
	accs := r.arvanAccList()
	accs = append(accs, map[string]any{"name": fmt.Sprintf("آروان %d", len(accs)+1), "token": key})
	r.arvanSaveAccs(accs)
	r.clearWizard(chat)
	r.Bot.Send(chat, "✅ اکانت اضافه شد.",
		[][]Btn{{btn("🌐 دامنه‌ها", "arvdoms"), btn("👤 اکانت‌ها", "arvanacc")}})
}

func (r *Router) arvanValueSave(chat, mid int64, pend map[string]any, text string) {
	acc := intOrAny(pend["acc"])
	dom, _ := pend["dom"].(string)
	rid, _ := pend["rid"].(string)
	accs := r.arvanAccList()
	if acc < 0 || acc >= len(accs) {
		r.Bot.Send(chat, "❌ اکانت پیدا نشد.", nil)
		return
	}
	if _, err := arvan.New(strv(accs[acc]["token"])).Call("PUT",
		fmt.Sprintf("/domains/%s/dns-records/%s", dom, rid),
		map[string]any{"value": strings.TrimSpace(text)}); err != nil {
		r.Bot.Send(chat, "❌ نشد: "+Esc(err.Error()), nil)
		return
	}
	r.Bot.Send(chat, "✅ مقدار عوض شد.", nil)
	r.clearWizard(chat)
	r.arvanRec(chat, 0, acc, dom, rid)
}

func (r *Router) arvanAddSave(chat, mid int64, pend map[string]any, text string) {
	acc := intOrAny(pend["acc"])
	dom, _ := pend["dom"].(string)
	parts := strings.Fields(text)
	if len(parts) < 3 {
		r.Bot.Send(chat, "❌ قالب درست نیست.", nil)
		return
	}
	accs := r.arvanAccList()
	if acc < 0 || acc >= len(accs) {
		r.Bot.Send(chat, "❌ اکانت پیدا نشد.", nil)
		return
	}
	if _, err := arvan.New(strv(accs[acc]["token"])).Call("POST",
		fmt.Sprintf("/domains/%s/dns-records", dom),
		map[string]any{"type": strings.ToUpper(parts[0]), "name": parts[1], "value": parts[2]}); err != nil {
		r.Bot.Send(chat, "❌ ثبت نشد: "+Esc(err.Error()), nil)
		return
	}
	r.Bot.Send(chat, "✅ رکورد ثبت شد.", nil)
	r.clearWizard(chat)
	r.arvanRecs(chat, 0, acc, dom, 0)
}
