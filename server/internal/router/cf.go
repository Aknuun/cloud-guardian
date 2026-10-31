package router

import (
	"fmt"
	"net"
	"strconv"
	"strings"
	"sync"

	"github.com/aknuun/cg-server/internal/cf"
)

type enumVal struct{ V, L string }

type settingInfo struct {
	Label  string
	Enum   bool
	Values []enumVal
}

var cfSetInfo = map[string]settingInfo{
	"ssl":                      {"حالت SSL", true, []enumVal{{"off", "خاموش"}, {"flexible", "انعطاف‌پذیر"}, {"full", "کامل"}, {"strict", "کامل (سخت)"}}},
	"min_tls_version":          {"حداقل نسخه TLS", true, []enumVal{{"1.0", "1.0"}, {"1.1", "1.1"}, {"1.2", "1.2"}, {"1.3", "1.3"}}},
	"tls_1_3":                  {"TLS 1.3", false, nil},
	"always_use_https":         {"همیشه HTTPS", false, nil},
	"automatic_https_rewrites": {"بازنویسی خودکار HTTPS", false, nil},
	"security_level":           {"سطح امنیت", true, []enumVal{{"off", "خاموش"}, {"essentially_off", "خیلی کم"}, {"low", "کم"}, {"medium", "متوسط"}, {"high", "زیاد"}, {"under_attack", "حالت حمله"}}},
	"http2":                    {"HTTP/2", false, nil},
	"http3":                    {"HTTP/3", false, nil},
	"brotli":                   {"فشرده‌سازی Brotli", false, nil},
	"ipv6":                     {"IPv6", false, nil},
	"always_online":            {"همیشه آنلاین", false, nil},
}

var cfSSLKeys = []string{"ssl", "min_tls_version", "tls_1_3", "always_use_https", "automatic_https_rewrites"}
var cfSECKeys = []string{"security_level"}
var cfPERFKeys = []string{"http2", "http3", "brotli", "ipv6", "always_online"}

const (
	cfPageSize   = 30
	cfZonePage   = 20
	cfSearchText = "🔍 جست‌وجو\n\n" +
		"در هر بخش از ربات با وارد کردن آی‌پی یا دامنه، افزودن سریع یا " +
		"جست‌وجو انجام می‌شود.\n\n" +
		"آی‌پی یا دامنه یا بخشی از آن را بفرستید:"
)

func (r *Router) cfZones(chat, mid int64, flt string, pg int) {
	accs := r.cfAccs()
	if len(accs) == 0 {
		r.Show(chat, mid, "❌ اکانت کلادفلر تنظیم نشده.\n\nتوکن API کلادفلر رو اضافه کن:",
			[][]Btn{{btn("➕ افزودن اکانت", "accadd"), btn("🏠 خانه", "menu")}})
		return
	}
	allz, zerr := r.cfAllZonesE()
	if len(allz) == 0 {
		if zerr != nil {
			r.Show(chat, mid, "❌ خطا در دریافت دامنه‌ها:\n"+Esc(zerr.Error())+
				"\n\nتوکن/دسترسی اکانت را بررسی کن.",
				[][]Btn{{btn("👤 اکانت‌ها", "accounts")}, navRow("menu")})
			return
		}
		r.Show(chat, mid, "☁️ کلودفلر\n\n📭 هیچ دامنه‌ای یافت نشد.", HomeKB())
		return
	}
	var zones []cfZone
	for _, z := range allz {
		st := z.Zone.Status
		if (st == "active" || st == "paused") && (flt == "all" || strv(z.Acc) == flt) {
			zones = append(zones, z)
		}
	}
	if len(zones) == 0 {
		r.Show(chat, mid, "📭 دامنه‌ای برای این فیلتر نیست.", stdKB("menu",
			row(btn("👤 اکانت‌ها", "accounts"))))
		return
	}
	pages := (len(zones) + cfZonePage - 1) / cfZonePage
	if pages < 1 {
		pages = 1
	}
	if pg < 0 {
		pg = 0
	}
	if pg > pages-1 {
		pg = pages - 1
	}
	var kb [][]Btn
	var rows []Btn
	for _, z := range zones[pg*cfZonePage : min(len(zones), (pg+1)*cfZonePage)] {
		mark := "⚪"
		switch z.Zone.Status {
		case "active":
			mark = "🟢"
		case "paused":
			mark = "⏸️"
		case "pending":
			mark = "🟡"
		}
		rows = append(rows, btnS(mark+" "+z.Zone.Name, fmt.Sprintf("cfz:%d:%s", z.Acc, z.Zone.ID), "plain"))
	}
	kb = append(kb, gridBTN(rows, 2)...)
	allLabel := "📁 همه اکانت‌ها"
	if flt == "all" {
		allLabel = "📁 ✅ همه اکانت‌ها"
	}
	kb = append(kb, []Btn{btnS("⭐ ساب‌های منتخب", "favs", "success"), btn(allLabel, "zf:all:0")})
	if len(accs) > 1 {
		var arows []Btn
		for j, a := range accs {
			mark := "👤 "
			if flt == strv(j) {
				mark = "👤 ✅ "
			}
			arows = append(arows, btn(mark+a.Name, fmt.Sprintf("zf:%d:0", j)))
		}
		kb = append(kb, gridBTN(arows, 3)...)
	}
	kb = append(kb, []Btn{btn("➕ افزودن رکورد", "addrec"), btn("👤 اکانت‌ها", "accounts")})
	if pages > 1 {
		var nav []Btn
		if pg > 0 {
			nav = append(nav, btn("⬅️", fmt.Sprintf("zf:%s:%d", flt, pg-1)))
		} else {
			nav = append(nav, emptyBtn())
		}
		nav = append(nav, btn("🔙 بازگشت", "back:menu"))
		if pg < pages-1 {
			nav = append(nav, btn("➡️", fmt.Sprintf("zf:%s:%d", flt, pg+1)))
		} else {
			nav = append(nav, emptyBtn())
		}
		kb = append(kb, nav)
	} else {
		kb = append(kb, []Btn{btn("🏠 خانه", "menu")})
	}
	label := "همه اکانت‌ها"
	if flt != "all" {
		if isDigits(flt) && atoiSafe(flt) < len(accs) {
			label = accs[atoiSafe(flt)].Name
		} else {
			label = "؟"
		}
	}
	title := "☁️ کلودفلر — " + label
	if pages > 1 {
		title += fmt.Sprintf(" — صفحه %d از %d", pg+1, pages)
	}
	r.Show(chat, mid, title, kb)
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}

// ---------- records ----------

func (r *Router) cfRecords(chat, mid int64, acc int, zid string, pg int) {
	cli, ok := r.cfClientFor(acc)
	if !ok {
		r.Show(chat, mid, "❌ خطای کلادفلر: اکانت پیدا نشد.", HomeKB())
		return
	}
	recs, err := cli.Records(zid)
	if err != nil {
		r.Show(chat, mid, "❌ خطای کلادفلر: "+Esc(err.Error()), HomeKB())
		return
	}
	zname := r.cfZoneName(acc, zid)
	sess := r.cfGetSession(chat)
	if strvVal(sess["zid"]) != zid {
		delete(sess, "sel")
	}
	sess["acc"], sess["zid"] = acc, zid
	r.cfSaveSession(chat, sess)
	sel, hasSel := sess["sel"].([]any)
	if len(recs) == 0 {
		r.Show(chat, mid, fmt.Sprintf("📭 رکوردی برای %s یافت نشد.", Esc(zname)),
			[][]Btn{{btn("➕ افزودن", fmt.Sprintf("cfadd:%d:%s", acc, zid)),
				btn("🔄 تازه‌سازی", fmt.Sprintf("cfz:%d:%s", acc, zid))},
				{btn("⚙️ تنظیمات", "zset"), btn("🧹 پاک‌سازی کش", "zpurge")},
				{btn("🔙 بازگشت", "back:zones"), btn("🏠 خانه", "menu")}})
		return
	}
	pages := (len(recs) + cfPageSize - 1) / cfPageSize
	if pages < 1 {
		pages = 1
	}
	if pg < 0 {
		pg = 0
	}
	if pg > pages-1 {
		pg = pages - 1
	}
	sess["pg"] = pg
	r.cfSaveSession(chat, sess)
	labels := cfDupLabels(recs, zname)
	var rows []Btn
	for _, rec := range recs[pg*cfPageSize : min(len(recs), (pg+1)*cfPageSize)] {
		if !hasSel {
			rows = append(rows, btnS(labels[rec.ID], "cfr:"+rec.ID, "plain"))
		} else {
			if selHas(sel, rec.ID) {
				rows = append(rows, btnS("✅ "+labels[rec.ID], "sel:"+rec.ID, "plain"))
			} else {
				rows = append(rows, btnS("⬜ "+labels[rec.ID], "sel:"+rec.ID, "plain"))
			}
		}
	}
	kb := gridBTN(rows, 3)
	if hasSel {
		if len(sel) > 0 {
			kb = append(kb, []Btn{btn("🗑 حذف انتخاب‌ها", "bulkdel"), btn("✏️ تغییر مقدار", "bulkedit")})
			kb = append(kb, []Btn{btn("🔄 تغییر نوع", "bulktype")})
			kb = append(kb, []Btn{btn("⭐ افزودن به منتخب‌ها", "favsel")})
		} else {
			kb = append(kb, []Btn{btn("❌ لغو انتخاب", "seldone")})
		}
		var nav []Btn
		if pg > 0 {
			nav = append(nav, btn("◀️", fmt.Sprintf("selp:%d", pg-1)))
		} else {
			nav = append(nav, emptyBtn())
		}
		nav = append(nav, btn(fmt.Sprintf("📄 %d/%d", pg+1, pages), "noop"))
		if pg < pages-1 {
			nav = append(nav, btn("▶️", fmt.Sprintf("selp:%d", pg+1)))
		} else {
			nav = append(nav, emptyBtn())
		}
		kb = append(kb, nav)
		kb = append(kb, []Btn{btn("✅ پایان انتخاب", "seldone"), btn("🏠 خانه", "menu")})
		r.Show(chat, mid, fmt.Sprintf("🗂 %s — %d انتخاب شده", Esc(zname), len(sel)), kb)
		return
	}
	kb = append(kb, []Btn{btn("⚙️ تنظیمات", "zset"), btn("🗂 گروهی", "selmode"),
		btn("➕ افزودن", fmt.Sprintf("cfadd:%d:%s", acc, zid)), btn("🔍 جستجو", "zsearch")})
	kb = append(kb, []Btn{btn("✉️ ایمیل", "zmail"), btn("📊 ترافیک ساب‌ها", "ztraf")})
	kb = append(kb, []Btn{btn("🔄 تازه‌سازی", fmt.Sprintf("cfz:%d:%s", acc, zid)), btn("🧹 پاک‌سازی کش", "zpurge")})
	var nav []Btn
	if pg > 0 {
		nav = append(nav, btn("◀️", fmt.Sprintf("cfp:%d", pg-1)))
	} else {
		nav = append(nav, emptyBtn())
	}
	if pages > 1 {
		nav = append(nav, btn(fmt.Sprintf("📄 %d/%d", pg+1, pages), "noop"))
	} else {
		nav = append(nav, btn("🔙 بازگشت", "back:zones"))
	}
	if pg < pages-1 {
		nav = append(nav, btn("▶️", fmt.Sprintf("cfp:%d", pg+1)))
	} else {
		nav = append(nav, emptyBtn())
	}
	kb = append(kb, nav)
	if pages > 1 {
		kb = append(kb, []Btn{btn("🔙 بازگشت", "back:zones"), btn("🏠 خانه", "menu")})
	}
	title := fmt.Sprintf("📋 %s — %d رکورد", Esc(zname), len(recs))
	if pages > 1 {
		title += fmt.Sprintf(" — صفحه %d از %d", pg+1, pages)
	}
	r.Show(chat, mid, title, kb)
}

func selHas(sel []any, rid string) bool {
	for _, x := range sel {
		if strvVal(x) == rid {
			return true
		}
	}
	return false
}

func (r *Router) cfRecord(chat, mid int64, acc int, zid, rid, traffic string) {
	cli, ok := r.cfClientFor(acc)
	if !ok {
		r.Show(chat, mid, "❌ خطای کلادفلر: اکانت پیدا نشد.", HomeKB())
		return
	}
	recs, err := cli.Records(zid)
	if err != nil {
		r.Show(chat, mid, "❌ خطای کلادفلر: "+Esc(err.Error()), HomeKB())
		return
	}
	zname := r.cfZoneName(acc, zid)
	var rec *cf.Record
	for i := range recs {
		if recs[i].ID == rid {
			rec = &recs[i]
			break
		}
	}
	if rec == nil {
		r.Show(chat, mid, "❌ رکورد پیدا نشد (شاید حذف شده). از لیست دوباره بازش کن.", HomeKB())
		return
	}
	sess := r.cfGetSession(chat)
	sess["acc"], sess["zid"], sess["rid"] = acc, zid, rid
	r.cfSaveSession(chat, sess)
	faved := false
	for _, f := range r.favs() {
		if strvVal(f["rid"]) == rid {
			faved = true
		}
	}
	text := fmt.Sprintf("✏️ ویرایش رکورد\n\n📛 نام: <code>%s</code>\n💡 مقدار: <code>%s</code>\n📁 دامنه: <code>%s</code>",
		Esc(rec.Name), Esc(rec.Content), Esc(zname))
	if traffic != "" {
		text += "\n" + traffic
	}
	var tbtn Btn
	up := strings.ToUpper(rec.Type)
	if up == "A" || up == "CNAME" {
		tbtn = btn("🏷 نوع: "+rec.Type, "ct:"+rid)
	} else {
		tbtn = btn("🔄 تغییر نوع", "ct:"+rid)
	}
	proxyState := "خاموش"
	if cfBoolAny(rec.Proxied) {
		proxyState = "روشن"
	}
	kb := [][]Btn{
		{tbtn, btn("✏️ تغییر مقدار", "cfedit:"+rid)},
		{btn("⏱ TTL: "+cfTtlLabel(rec.TTL), "cfttl:"+rid), btn("🌐 Proxy: "+proxyState, "cfpx:"+rid)},
	}
	fqdn := rec.Name
	if rec.Name != zname && rec.Name != "@" && !strings.HasSuffix(rec.Name, "."+zname) {
		fqdn = rec.Name + "." + zname
	} else if rec.Name == "@" {
		fqdn = zname
	}
	cp := []Btn{}
	if fqdn != "" {
		cp = append(cp, copyBtnL("📋 کپی نام", fqdn))
	}
	if rec.Content != "" {
		cp = append(cp, copyBtnL("📋 کپی مقدار", rec.Content))
	}
	if len(cp) > 0 {
		kb = append(kb, cp)
	}
	if up == "A" || up == "AAAA" || up == "CNAME" {
		kb = append(kb, []Btn{btnS("🛟 آیپی زاپاس", "lbss:"+rid, "success"), btnS("⚙️ تنظیمات لود بالانسر", "lbs:"+rid, "success")})
	}
	favLabel := "⭐ افزودن به منتخب"
	if faved {
		favLabel = "⭐ حذف از منتخب"
	}
	kb = append(kb, []Btn{btnS("🔗 CNAME به این", "cref", "primary"), btnS(favLabel, "favtg:"+rid, "primary")})
	kb = append(kb, []Btn{btnS("🗑 حذف", "cfdel:"+rid, "danger"), btn("📊 مشاهده ترافیک", "rtraf")})
	kb = append(kb, []Btn{btn("🔙 بازگشت", "back:zones"), btn("🏠 خانه", "menu")})
	r.Show(chat, mid, text, kb)
}

func (r *Router) cfDelAsk(chat, mid int64, acc int, zid, rid string) {
	r.Show(chat, mid, "⚠️ مطمئنی حذف شود؟",
		[][]Btn{{btnS("✅ بله، حذف کن", "cfdelok:"+rid, "danger"), btn("❌ انصراف", "cfr:"+rid)}})
}

func (r *Router) cfDelDo(chat, mid int64, acc int, zid, rid string) {
	cli, ok := r.cfClientFor(acc)
	if ok {
		if err := cli.DelRecord(zid, rid); err != nil {
			r.Show(chat, mid, "❌ حذف نشد: "+Esc(err.Error()), navKB("cfr:"+rid))
			return
		}
	}
	r.cfRecords(chat, mid, acc, zid, 0)
}

func (r *Router) cfTogglePx(chat, mid int64, acc int, zid, rid string) {
	rec := r.cfFind(acc, zid, rid)
	if rec == nil {
		r.Show(chat, mid, "❌ رکورد پیدا نشد.", HomeKB())
		return
	}
	if t := strings.ToUpper(rec.Type); t != "A" && t != "AAAA" && t != "CNAME" {
		r.Show(chat, mid, "ℹ️ پروکسی فقط برای رکوردهای A / AAAA / CNAME معنی دارد.", navKB("cfr:"+rid))
		return
	}
	cli, _ := r.cfClientFor(acc)
	if _, err := cli.UpdateRecord(zid, *rec, map[string]any{"proxied": !cfBoolAny(rec.Proxied)}); err != nil {
		r.Show(chat, mid, "❌ نشد: "+Esc(err.Error()), HomeKB())
		return
	}
	r.cfRecord(chat, mid, acc, zid, rid, "")
}

var cfTTLSteps = []int{1, 120, 300, 1800, 3600, 21600, 86400}

func (r *Router) cfCycleTTL(chat, mid int64, acc int, zid, rid string) {
	rec := r.cfFind(acc, zid, rid)
	if rec == nil {
		r.Show(chat, mid, "❌ رکورد پیدا نشد.", HomeKB())
		return
	}
	cur, err := strconv.Atoi(strings.TrimSpace(fmt.Sprint(rec.TTL)))
	if err != nil {
		cur = 1
	}
	nxt := cfTTLSteps[0]
	for i, s := range cfTTLSteps {
		if s == cur {
			nxt = cfTTLSteps[(i+1)%len(cfTTLSteps)]
			break
		}
	}
	cli, _ := r.cfClientFor(acc)
	if _, err := cli.UpdateRecord(zid, *rec, map[string]any{"ttl": nxt}); err != nil {
		r.Show(chat, mid, "❌ نشد: "+Esc(err.Error()), HomeKB())
		return
	}
	r.cfRecord(chat, mid, acc, zid, rid, "")
}

func (r *Router) cfTypeToggle(chat, mid int64, acc int, zid, rid string) {
	rec := r.cfFind(acc, zid, rid)
	if rec == nil {
		r.Show(chat, mid, "❌ رکورد پیدا نشد.", HomeKB())
		return
	}
	t := strings.ToUpper(rec.Type)
	target := ""
	if t == "A" {
		target = "CNAME"
	} else if t == "CNAME" {
		target = "A"
	}
	if target == "" {
		r.Show(chat, mid, "❌ فقط A↔CNAME.", HomeKB())
		return
	}
	v := strings.TrimSpace(rec.Content)
	isIP := net.ParseIP(v) != nil
	if target == "A" && !isIP {
		r.cfEditStartTyped(chat, mid, acc, zid, rid, "A")
		return
	}
	if target == "CNAME" && isIP {
		r.cfEditStartTyped(chat, mid, acc, zid, rid, "CNAME")
		return
	}
	cli, _ := r.cfClientFor(acc)
	if _, err := cli.UpdateRecord(zid, *rec, map[string]any{"type": target}); err != nil {
		r.Show(chat, mid, "❌ نشد: "+Esc(err.Error()), HomeKB())
		return
	}
	r.cfRecord(chat, mid, acc, zid, rid, "")
}

// ---------- add record ----------

func (r *Router) cfAddrecZones(chat, mid int64) {
	allz := r.cfAllZones()
	var zones []cfZone
	for _, z := range allz {
		if z.Zone.Status == "active" {
			zones = append(zones, z)
		}
	}
	if len(zones) == 0 {
		r.Show(chat, mid, "📭 دامنه‌ای نیست.", HomeKB())
		return
	}
	var rows []Btn
	for _, z := range zones[:min(len(zones), 40)] {
		rows = append(rows, btnS(z.Zone.Name, fmt.Sprintf("addrec:%d:%s", z.Acc, z.Zone.ID), "plain"))
	}
	kb := gridBTN(rows, 2)
	kb = append(kb, []Btn{btn("🔙 بازگشت", "back:zones"), btn("🏠 خانه", "menu")})
	note := ""
	if len(zones) > 40 {
		note = fmt.Sprintf("\n\nℹ️ %d دامنهٔ دیگر هم هست؛ نام دامنه را در «🔍 جست‌وجو» بفرست تا سریع پیدا شود.", len(zones)-40)
	}
	r.Show(chat, mid, "➕ افزودن رکورد — دامنه را انتخاب کن:"+note, kb)
}

func (r *Router) cfAddStart(chat, mid int64, acc int, zid string) {
	types := []string{"A", "AAAA", "CNAME", "TXT", "MX", "NS"}
	var rows []Btn
	for _, t := range types {
		rows = append(rows, btnS(t, fmt.Sprintf("cfat:%d:%s:%s", acc, zid, t), "plain"))
	}
	kb := gridBTN(rows, 3)
	kb = append(kb, []Btn{btn("🔙 بازگشت", "back:"+fmt.Sprintf("cfz:%d:%s", acc, zid)), btn("🏠 خانه", "menu")})
	zname := r.cfZoneName(acc, zid)
	r.Show(chat, mid, fmt.Sprintf("➕ ساخت رکورد در %s\n\nنوع رکورد را انتخاب کنید:", Esc(zname)), kb)
}

func (r *Router) cfAddType(chat, mid int64, acc int, zid, rtype string) {
	if !inSet(rtype, []string{"A", "AAAA", "CNAME", "TXT", "MX", "NS"}) {
		r.Show(chat, mid, "❌ نوع رکورد نامعتبر است.", HomeKB())
		return
	}
	r.setWizard(chat, map[string]any{"kind": "cr_ar_name", "acc": acc, "zid": zid, "rtype": rtype})
	r.Show(chat, mid, fmt.Sprintf("➕ ساخت رکورد %s\n\nنام رکورد را بفرستید (مثلاً www یا @):", rtype),
		[][]Btn{{btn("⬅️ انصراف", fmt.Sprintf("cfz:%d:%s", acc, zid))}})
}

func (r *Router) cfArCreate(chat, mid int64, pend map[string]any, content string, priority *int, proxied, force bool) {
	rtype := strings.ToUpper(strvVal(pend["rtype"]))
	if rtype == "" {
		rtype = "A"
	}
	if hint := cfValueHint(content, rtype); hint != "" {
		r.Bot.Send(chat, "❌ "+hint+"\n\nپیام اصلاح‌شده را دوباره بفرستید.",
			[][]Btn{{btn("⬅️ انصراف", fmt.Sprintf("cfz:%d:%s", intOfAny(pend["acc"]), strvVal(pend["zid"])))}})
		return
	}
	acc := intOfAny(pend["acc"])
	cli, ok := r.cfClientFor(acc)
	if !ok {
		r.Bot.Send(chat, "❌ اکانت پیدا نشد.", nil)
		return
	}
	if !force {
		if dup := cfDuplicate(cli, strvVal(pend["zid"]), strvVal(pend["name"]), rtype, content, r.cfZoneName(acc, strvVal(pend["zid"]))); dup {
			pend["kind"] = "cr_ar_force"
			pend["force_px"] = proxied
			if priority != nil {
				pend["force_prio"] = *priority
			}
			r.setWizard(chat, pend)
			r.Show(chat, mid, fmt.Sprintf("⚠️ رکورد مشابهی از قبل وجود دارد:\n<code>%s %s → %s</code>\n\nباز هم ساخته شود؟",
				Esc(rtype), Esc(strvVal(pend["name"])), Esc(content)),
				[][]Btn{
					{btnS("✅ باز هم بساز", "cfarforce", "danger")},
					{btn("⬅️ انصراف", fmt.Sprintf("cfz:%d:%s", intOfAny(pend["acc"]), strvVal(pend["zid"])))},
				})
			return
		}
	}
	if _, err := cli.AddRecord(strvVal(pend["zid"]), rtype, strvVal(pend["name"]), content, 1, proxied, priority); err != nil {
		r.Bot.Send(chat, "❌ ثبت نشد: "+Esc(err.Error()), nil)
		return
	}
	r.clearWizard(chat)
	r.Bot.Send(chat, "✅ رکورد ثبت شد.", nil)
	r.cfRecords(chat, 0, acc, strvVal(pend["zid"]), 0)
}

// cfDuplicate reports whether the zone already has an identical record.
func cfDuplicate(cli *cf.Client, zid, name, rtype, content, zname string) bool {
	recs, err := cli.Records(zid)
	if err != nil {
		return false
	}
	n := strings.ToLower(strings.TrimSpace(name))
	target := content
	if ip := net.ParseIP(target); ip != nil {
		target = ip.String()
	} else {
		target = strings.ToLower(strings.TrimSuffix(strings.TrimSpace(target), "."))
	}
	for _, x := range recs {
		if strings.ToUpper(x.Type) != strings.ToUpper(rtype) {
			continue
		}
		var xc string
		if ip := net.ParseIP(x.Content); ip != nil {
			xc = ip.String()
		} else {
			xc = strings.ToLower(strings.TrimSuffix(strings.TrimSpace(x.Content), "."))
		}
		if xc != target {
			continue
		}
		xn := strings.ToLower(strings.TrimSuffix(x.Name, "."))
		if n == "@" {
			if xn == strings.ToLower(zname) || xn == "@" {
				return true
			}
			continue
		}
		if xn == n || xn == n+"."+strings.ToLower(zname) {
			return true
		}
	}
	return false
}

func (r *Router) cfArForce(chat, mid int64) {
	pend := r.getWizard(chat)
	if pend == nil || strvVal(pend["kind"]) != "cr_ar_force" {
		r.Show(chat, mid, "⏳ نشست منقضی شد؛ از دامنه دوباره شروع کن.", HomeKB())
		return
	}
	var prio *int
	if v, ok := pend["force_prio"]; ok {
		p := intOrAny(v)
		prio = &p
	}
	r.cfArCreate(chat, mid, pend, strvVal(pend["content"]), prio, boolVal(pend["force_px"]), true)
}

// cfArProxy finishes a record creation after the user picked the proxy mode.
func (r *Router) cfArProxy(chat, mid int64, on bool) {
	pend := r.getWizard(chat)
	if pend == nil || strvVal(pend["kind"]) != "cr_ar_px" {
		r.Show(chat, mid, "⏳ نشست منقضی شد؛ از دامنه دوباره شروع کن.", HomeKB())
		return
	}
	r.cfArCreate(chat, mid, pend, strvVal(pend["content"]), nil, on, false)
}

// ---------- edit record (wizard) ----------

func (r *Router) cfEditStart(chat, mid int64, acc int, zid, rid string) {
	rec := r.cfFind(acc, zid, rid)
	r.setWizard(chat, map[string]any{"kind": "cr_cfedit", "acc": acc, "zid": zid, "rid": rid})
	name, content := "", ""
	if rec != nil {
		name, content = rec.Name, rec.Content
	}
	r.Show(chat, mid, fmt.Sprintf("✏️ مقدار جدید را بفرستید (IP یا هدف CNAME):\n\n📛 سابی که تغییر می‌کند: <code>%s</code>\n\n🔴 مقدار فعلی که عوض می‌شود:\n<code>%s</code>\n\n💡 اگر IPv4 بفرستی نوع A، اگر IPv6 بفرستی AAAA و اگر دامنه بفرستی CNAME می‌شود.",
		Esc(name), Esc(content)), [][]Btn{{btn("⬅️ انصراف", "cfr:"+rid)}})
}

// cfEditStartTyped starts the edit wizard for a specific target type (used by
// the A<->CNAME toggle when the current value does not fit the target type).
func (r *Router) cfEditStartTyped(chat, mid int64, acc int, zid, rid, newType string) {
	rec := r.cfFind(acc, zid, rid)
	r.setWizard(chat, map[string]any{"kind": "cr_cfedit", "acc": acc, "zid": zid, "rid": rid, "newtype": newType})
	name, content := "", ""
	if rec != nil {
		name, content = rec.Name, rec.Content
	}
	example := "1.2.3.4"
	if strings.ToUpper(newType) == "CNAME" {
		example = "sub.example.com"
	}
	r.Show(chat, mid, fmt.Sprintf("✏️ برای تبدیل به رکورد %s، مقدار جدید را بفرستید (مثل <code>%s</code>):\n\n📛 ساب: <code>%s</code>\n🔴 مقدار فعلی: <code>%s</code>",
		Esc(newType), Esc(example), Esc(name), Esc(content)),
		[][]Btn{{btn("⬅️ انصراف", "cfr:"+rid)}})
}

func (r *Router) cfEditSave(chat, mid int64, pend map[string]any, text string) {
	acc := intOfAny(pend["acc"])
	zid := strvVal(pend["zid"])
	rid := strvVal(pend["rid"])
	rec := r.cfFind(acc, zid, rid)
	if rec == nil {
		r.clearWizard(chat)
		r.Bot.Send(chat, "❌ رکورد پیدا نشد.", nil)
		return
	}
	recType := strings.ToUpper(rec.Type)
	eff := recType
	if forced := strings.ToUpper(strvVal(pend["newtype"])); forced != "" {
		eff = forced
	} else if recType == "A" || recType == "AAAA" || recType == "CNAME" {
		// Sending an IP switches to A/AAAA; a hostname switches to CNAME.
		if t := cfTypeForValue(text); t != "" {
			eff = t
		}
	}
	if hint := cfValueHint(text, eff); hint != "" {
		r.Bot.Send(chat, "❌ "+hint+"\n\nپیام اصلاح‌شده را دوباره بفرستید؛ همین‌جا ویرایش می‌شود.",
			[][]Btn{{btn("⬅️ انصراف", "cfr:"+rid)}})
		return
	}
	cli, _ := r.cfClientFor(acc)
	patch := map[string]any{"content": strings.TrimSpace(text)}
	if eff != recType {
		patch["type"] = eff
	}
	if _, err := cli.UpdateRecord(zid, *rec, patch); err != nil {
		r.Bot.Send(chat, "❌ نشد: "+Esc(err.Error()), nil)
		return
	}
	r.clearWizard(chat)
	if eff != recType {
		r.Bot.Send(chat, fmt.Sprintf("✅ رکورد به %s تغییر کرد و مقدار عوض شد.", Esc(eff)), nil)
	} else {
		r.Bot.Send(chat, "✅ مقدار عوض شد.", nil)
	}
	r.cfRecord(chat, 0, acc, zid, rid, "")
}

// ---------- zone settings ----------

func (r *Router) cfZctx(chat int64) (int, string, string, *cf.Client, bool) {
	sess := r.cfGetSession(chat)
	acc := intOrAny(sess["acc"])
	zid := strvVal(sess["zid"])
	if zid == "" {
		return 0, "", "", nil, false
	}
	cli, ok := r.cfClientFor(acc)
	if !ok {
		return 0, "", "", nil, false
	}
	return acc, zid, r.cfZoneName(acc, zid), cli, true
}

func (r *Router) cfZset(chat, mid int64) {
	acc, zid, zname, cli, ok := r.cfZctx(chat)
	if !ok {
		r.Show(chat, mid, "⏳ نشست منقضی شد؛ دامنه را دوباره باز کن.", HomeKB())
		return
	}
	z, err := cli.Zone(zid)
	if err != nil {
		r.Show(chat, mid, "❌ خطای کلادفلر: "+Esc(err.Error()), HomeKB())
		return
	}
	status := strvVal(z["status"])
	pauseLabel := "▶️ فعال‌سازی دامنه"
	if status == "active" {
		pauseLabel = "⏸️ توقف دامنه"
	}
	kb := gridBTN([]Btn{
		btn("🔒 SSL / TLS", "zssl"), btn("🛡 امنیت", "zsec"), btn("⚡ کارایی", "zperf"),
		btn("ℹ️ جزئیات دامنه", "zd"), btn("🧹 پاکسازی کامل کش", "zpurge"),
		btn("🚧 حالت توسعه", "zdev"), btn("✉️ ایمیل (فوروارد)", "zmail"),
		btn(pauseLabel, "zpause"),
	}, 2)
	kb = append(kb, []Btn{btn("🔙 بازگشت", "back:"+fmt.Sprintf("cfz:%d:%s", acc, zid)), btn("🏠 خانه", "menu")})
	r.Show(chat, mid, fmt.Sprintf("⚙️ تنظیمات %s:\n\nوضعیت: %s", Esc(zname), Esc(status)), kb)
}

func (r *Router) cfSettingsGroup(chat, mid int64, which string) {
	_, _, zname, cli, ok := r.cfZctx(chat)
	if !ok {
		r.Show(chat, mid, "⏳ نشست منقضی شد؛ دامنه را دوباره باز کن.", HomeKB())
		return
	}
	sess := r.cfGetSession(chat)
	zid := strvVal(sess["zid"])
	keys := cfSSLKeys
	title := "🔒 SSL / TLS"
	if which == "zsec" {
		keys, title = cfSECKeys, "🛡 امنیت"
	} else if which == "zperf" {
		keys, title = cfPERFKeys, "⚡ کارایی"
	}
	smap, err := cli.Settings(zid)
	if err != nil {
		r.Show(chat, mid, "❌ خطای کلادفلر: "+Esc(err.Error()), HomeKB())
		return
	}
	var btns []Btn
	for _, k := range keys {
		info := cfSetInfo[k]
		cur := strvVal(smap[k])
		if !info.Enum {
			mark := "❌"
			if cur == "on" {
				mark = "✅"
			}
			btns = append(btns, btn(mark+" "+info.Label, "ztog:"+k))
		} else {
			lab := cur
			for _, v := range info.Values {
				if v.V == cur {
					lab = v.L
				}
			}
			btns = append(btns, btn("🔧 "+info.Label+": "+lab, "zval:"+k))
		}
	}
	kb := gridBTN(btns, 2)
	kb = append(kb, []Btn{btn("🔙 بازگشت", "back:zset"), btn("🏠 خانه", "menu")})
	r.Show(chat, mid, fmt.Sprintf("%s — %s:\n\nبرای تغییر روی هر گزینه کلیک کنید:", title, Esc(zname)), kb)
}

func (r *Router) cfSettingToggle(chat, mid int64, key string) {
	sess := r.cfGetSession(chat)
	zid := strvVal(sess["zid"])
	_, _, _, cli, ok := r.cfZctx(chat)
	if !ok {
		r.Show(chat, mid, "⏳ نشست منقضی شد.", HomeKB())
		return
	}
	smap, err := cli.Settings(zid)
	if err == nil {
		cur := strvVal(smap[key])
		next := "on"
		if cur == "on" {
			next = "off"
		}
		err = cli.SetSetting(zid, key, next)
	}
	if err != nil {
		r.Show(chat, mid, "❌ خطا:\n"+Esc(err.Error()), navKB("zset"))
		return
	}
	g, _ := cfSettingGroup(key)
	r.cfSettingsGroup(chat, mid, g)
}

func (r *Router) cfSettingEnum(chat, mid int64, key string) {
	info, ok := cfSetInfo[key]
	if !ok {
		r.Show(chat, mid, "❌ تنظیم ناشناخته.", HomeKB())
		return
	}
	sess := r.cfGetSession(chat)
	zid := strvVal(sess["zid"])
	_, _, _, cli, ok2 := r.cfZctx(chat)
	if !ok2 {
		r.Show(chat, mid, "⏳ نشست منقضی شد.", HomeKB())
		return
	}
	smap, err := cli.Settings(zid)
	if err != nil {
		r.Show(chat, mid, "❌ خطا:\n"+Esc(err.Error()), HomeKB())
		return
	}
	cur := strvVal(smap[key])
	var btns []Btn
	for _, v := range info.Values {
		mark := ""
		if v.V == cur {
			mark = "✅ "
		}
		btns = append(btns, btn(mark+v.L, "zvset:"+key+":"+v.V))
	}
	kb := gridBTN(btns, 2)
	kb = append(kb, []Btn{btn("🔙 بازگشت", "back:zset"), btn("🏠 خانه", "menu")})
	curLabel := cur
	for _, v := range info.Values {
		if v.V == cur {
			curLabel = v.L
		}
	}
	r.Show(chat, mid, fmt.Sprintf("🔧 %s:\n\nمقدار فعلی: %s", info.Label, Esc(curLabel)), kb)
}

func (r *Router) cfSettingSet(chat, mid int64, key, value string) {
	sess := r.cfGetSession(chat)
	zid := strvVal(sess["zid"])
	_, _, _, cli, ok := r.cfZctx(chat)
	if !ok {
		r.Show(chat, mid, "⏳ نشست منقضی شد.", HomeKB())
		return
	}
	if err := cli.SetSetting(zid, key, value); err != nil {
		r.Show(chat, mid, "❌ خطا:\n"+Esc(err.Error()), navKB("zset"))
		return
	}
	g, _ := cfSettingGroup(key)
	r.cfSettingsGroup(chat, mid, g)
}

func (r *Router) cfZoneDetails(chat, mid int64) {
	acc, zid, _, cli, ok := r.cfZctx(chat)
	if !ok {
		r.Show(chat, mid, "⏳ نشست منقضی شد.", HomeKB())
		return
	}
	z, err := cli.Zone(zid)
	if err != nil {
		r.Show(chat, mid, "❌ خطا:\n"+Esc(err.Error()), HomeKB())
		return
	}
	ns := []string{}
	if arr, ok := z["name_servers"].([]any); ok {
		for _, n := range arr {
			ns = append(ns, strvVal(n))
		}
	}
	created := strvVal(z["created_on"])
	if len(created) > 10 {
		created = created[:10]
	}
	accName := ""
	if acct, ok := z["account"].(map[string]any); ok {
		accName = strvVal(acct["name"])
	}
	lines := []string{"ℹ️ جزئیات دامنه", "",
		fmt.Sprintf("📛 نام: <code>%s</code>", Esc(strvVal(z["name"]))),
		fmt.Sprintf("🆔 شناسه: <code>%s</code>", Esc(strvVal(z["id"]))),
		fmt.Sprintf("📦 وضعیت: %s", Esc(strvVal(z["status"]))),
		fmt.Sprintf("🌐 پلن: %s", Esc(orDash(strvVal(z["plan"])))),
		fmt.Sprintf("📅 ساخت: %s", Esc(created)),
		fmt.Sprintf("👤 اکانت: %s", Esc(orDash(accName))),
		"", "🖧 نیم‌سرورها:"}
	if len(ns) > 0 {
		for _, n := range ns {
			lines = append(lines, "• <code>"+Esc(n)+"</code>")
		}
	} else {
		lines = append(lines, "• —")
	}
	r.Show(chat, mid, strings.Join(lines, "\n"),
		navKB(fmt.Sprintf("cfz:%d:%s", acc, zid)))
}

func (r *Router) cfPurge(chat, mid int64) {
	_, zid, _, cli, ok := r.cfZctx(chat)
	if !ok {
		r.Show(chat, mid, "⏳ نشست منقضی شد.", HomeKB())
		return
	}
	if err := cli.PurgeCache(zid); err != nil {
		r.Show(chat, mid, "❌ خطا:\n"+Esc(err.Error()), navKB("zset"))
		return
	}
	r.Show(chat, mid, "🧹 کل کش دامنه پاک شد.",
		navKB("zset"))
}

func (r *Router) cfDevToggle(chat, mid int64) {
	_, zid, _, cli, ok := r.cfZctx(chat)
	if !ok {
		r.Show(chat, mid, "⏳ نشست منقضی شد.", HomeKB())
		return
	}
	smap, err := cli.Settings(zid)
	if err == nil {
		cur := strvVal(smap["development_mode"])
		next := "on"
		if cur == "on" {
			next = "off"
		}
		err = cli.SetSetting(zid, "development_mode", next)
		if err == nil {
			if next == "on" {
				r.Show(chat, mid, "🚧 حالت توسعه روشن شد.", navKB("zset"))
			} else {
				r.Show(chat, mid, "🚧 حالت توسعه خاموش شد.", navKB("zset"))
			}
			return
		}
	}
	r.Show(chat, mid, "❌ خطا:\n"+Esc(err.Error()), navKB("zset"))
}

func (r *Router) cfZonePause(chat, mid int64) {
	_, zid, _, cli, ok := r.cfZctx(chat)
	if !ok {
		r.Show(chat, mid, "⏳ نشست منقضی شد.", HomeKB())
		return
	}
	z, err := cli.Zone(zid)
	if err != nil {
		r.Show(chat, mid, "❌ خطا:\n"+Esc(err.Error()), navKB("zset"))
		return
	}
	active := strvVal(z["status"]) == "active"
	if err := cli.SetZonePaused(zid, active); err != nil {
		r.Show(chat, mid, "❌ خطا:\n"+Esc(err.Error()), navKB("zset"))
		return
	}
	msg := "▶️ دامنه فعال شد."
	if active {
		msg = "⏸️ دامنه متوقف شد."
	}
	r.Show(chat, mid, msg, navKB("zset"))
}

func (r *Router) cfAccDelDo(chat, mid int64, idx string) {
	accs := r.cfAccs()
	i := atoiSafe(idx)
	if i < 0 || i >= len(accs) {
		r.Show(chat, mid, "❌ اکانت پیدا نشد.", HomeKB())
		return
	}
	accs = append(accs[:i], accs[i+1:]...)
	r.Store.PutJSON("cf_accounts", accs, 0)
	r.cfDropZoneNames(i)
	var keep []map[string]any
	for _, f := range r.favs() {
		fa := intOrAny(f["acc"])
		if fa == i {
			continue
		}
		if fa > i {
			f["acc"] = fa - 1
		}
		keep = append(keep, f)
	}
	r.Store.PutJSON("favs", toAnyList(keep), 0)
	sess := r.cfGetSession(chat)
	if intOrAny(sess["acc"]) == i {
		delete(sess, "acc")
		delete(sess, "zid")
		delete(sess, "rid")
		delete(sess, "sel")
	} else if a := intOrAny(sess["acc"]); a > i {
		sess["acc"] = a - 1
	}
	r.cfSaveSession(chat, sess)
	r.Bot.Send(chat, "✅ اکانت حذف شد.", nil)
	r.cfAccountsHome(chat, 0)
}

// ---------- search ----------

func (r *Router) cfGsearch(chat, mid int64, q string) {
	q = strings.ToLower(strings.TrimSpace(q))
	type hit struct {
		acc  int
		zone cf.Zone
		rec  *cf.Record
	}
	var hits []hit
	var mu sync.Mutex
	var wg sync.WaitGroup
	accs := r.cfAccs()
	for i, a := range accs {
		wg.Add(1)
		go func(i int, tok string) {
			defer wg.Done()
			cli := cf.New(tok)
			zs, err := cli.Zones()
			if err != nil {
				return
			}
			for _, z := range zs {
				if z.Status != "active" && z.Status != "paused" {
					continue
				}
				mu.Lock()
				full := len(hits) >= 40
				mu.Unlock()
				if full {
					return
				}
				if strings.Contains(strings.ToLower(z.Name), q) {
					mu.Lock()
					hits = append(hits, hit{acc: i, zone: z})
					mu.Unlock()
				}
				recs, err := cli.Records(z.ID)
				if err != nil {
					continue
				}
				for k := range recs {
					rc := recs[k]
					if strings.Contains(strings.ToLower(rc.Name), q) || strings.Contains(strings.ToLower(rc.Content), q) {
						mu.Lock()
						hits = append(hits, hit{acc: i, zone: z, rec: &rc})
						n := len(hits)
						mu.Unlock()
						if n >= 40 {
							return
						}
					}
				}
			}
		}(i, a.Token)
	}
	wg.Wait()
	if len(hits) == 0 {
		r.Show(chat, mid, "🔍 چیزی پیدا نشد.", HomeKB())
		return
	}
	results := map[string]any{}
	var items []Btn
	n := min(len(hits), 30)
	for _, h := range hits[:n] {
		if h.rec == nil {
			items = append(items, btnS("📁 "+h.zone.Name, fmt.Sprintf("cfz:%d:%s", h.acc, h.zone.ID), "plain"))
		} else {
			results[h.rec.ID] = map[string]any{"acc": h.acc, "zid": h.zone.ID}
			items = append(items, btnS(cfRecLabel(h.rec.Name, h.rec.Type, h.zone.Name), "cfr:"+h.rec.ID, "plain"))
		}
	}
	kb := gridBTN(items, 2)
	kb = append(kb, navRow("zones"))
	r.cfSaveSession(chat, map[string]any{"results": results})
	r.Show(chat, mid, fmt.Sprintf("🔍 نتایج (%d):", len(hits)), kb)
}

func (r *Router) cfZsearch(chat, mid int64, q string) {
	sess := r.cfGetSession(chat)
	acc := intOrAny(sess["acc"])
	zid := strvVal(sess["zid"])
	if zid == "" {
		r.Bot.Send(chat, "⏳ نشست منقضی شد.", nil)
		return
	}
	cli, ok := r.cfClientFor(acc)
	if !ok {
		r.Bot.Send(chat, "❌ اکانت پیدا نشد.", nil)
		return
	}
	recs, err := cli.Records(zid)
	if err != nil {
		r.Bot.Send(chat, "❌ "+Esc(err.Error()), nil)
		return
	}
	q = strings.ToLower(strings.TrimSpace(q))
	var hits []cf.Record
	for _, rc := range recs {
		if strings.Contains(strings.ToLower(rc.Name), q) || strings.Contains(strings.ToLower(rc.Content), q) {
			hits = append(hits, rc)
			if len(hits) >= 30 {
				break
			}
		}
	}
	if len(hits) == 0 {
		r.Bot.Send(chat, "🔍 چیزی پیدا نشد.", nil)
		return
	}
	labels := cfDupLabels(hits, r.cfZoneName(acc, zid))
	var rows []Btn
	for _, rc := range hits {
		rows = append(rows, btnS(labels[rc.ID], "cfr:"+rc.ID, "plain"))
	}
	kb := gridBTN(rows, 3)
	kb = append(kb, []Btn{btn("🔙 بازگشت", "back:zones"), btn("🏠 خانه", "menu")})
	r.Bot.Send(chat, fmt.Sprintf("🔍 نتایج (%d):", len(hits)), kb)
}

// ---------- wizards ----------

func (r *Router) cfAccName(chat, mid int64, text string) {
	name := strings.TrimSpace(text)
	if name == "" {
		r.Show(chat, mid, "❌ اسم نامعتبر. دوباره بفرست:", navKB("accounts"))
		return
	}
	r.setWizard(chat, map[string]any{"kind": "cr_cf_acc_token", "name": name})
	r.Show(chat, mid, "🔑 توکن API کلادفلر را بفرستید:", navKB("accounts"))
}

func (r *Router) cfAccToken(chat, mid int64, pend map[string]any, text string) {
	tok := strings.TrimSpace(text)
	if len(tok) < 20 {
		r.Show(chat, mid, "❌ توکن نامعتبر. دوباره بفرست:", navKB("accounts"))
		return
	}
	name := strvVal(pend["name"])
	accs := r.cfAccs()
	accs = append(accs, cf.Account{Name: name, Token: tok})
	r.Store.PutJSON("cf_accounts", accs, 0)
	r.clearWizard(chat)
	r.Bot.Send(chat, fmt.Sprintf("✅ اکانت «%s» اضافه شد.", Esc(name)), nil)
	r.cfAccountsHome(chat, 0)
}

func (r *Router) cfArName(chat, mid int64, pend map[string]any, text string) {
	name := strings.TrimSpace(text)
	if name == "" {
		name = "@"
	}
	if name != "@" {
		name = strings.TrimSuffix(strings.ToLower(name), ".")
	}
	pend["name"] = name
	pend["kind"] = "cr_ar_content"
	r.setWizard(chat, pend)
	r.Show(chat, mid, "📄 مقدار رکورد را بفرستید:",
		[][]Btn{{btn("⬅️ انصراف", fmt.Sprintf("cfz:%d:%s", intOfAny(pend["acc"]), strvVal(pend["zid"])))}})
}

func (r *Router) cfArContent(chat, mid int64, pend map[string]any, text string) {
	content := strings.TrimSpace(text)
	if content == "" || strings.Contains(content, " ") {
		r.Show(chat, mid, "❌ مقدار نامعتبر (نباید فاصله داشته باشد). دوباره بفرست:",
			[][]Btn{{btn("⬅️ انصراف", fmt.Sprintf("cfz:%d:%s", intOfAny(pend["acc"]), strvVal(pend["zid"])))}})
		return
	}
	pend["content"] = content
	rtype := strings.ToUpper(strvVal(pend["rtype"]))
	if rtype == "MX" {
		pend["kind"] = "cr_ar_priority"
		r.setWizard(chat, pend)
		r.Show(chat, mid, "🔢 اولویت (priority) را بفرستید (عدد):", nil)
		return
	}
	if rtype == "A" || rtype == "AAAA" || rtype == "CNAME" {
		pend["kind"] = "cr_ar_px"
		r.setWizard(chat, pend)
		r.Show(chat, mid, fmt.Sprintf("🌐 نحوهٔ ثبت <code>%s</code>:\n\n• <b>پروکسی روشن</b>: ترافیک از کلادفلر می‌گذرد و IP سرور مخفی می‌ماند.\n• <b>پروکسی خاموش</b>: فقط DNS (برای SSH و سرویس‌های غیر HTTP بهتر است).", Esc(content)),
			[][]Btn{
				{btnS("🟠 ثبت با پروکسی", "cfarp:1", "success")},
				{btnS("⚪ ثبت بدون پروکسی", "cfarp:0", "primary")},
				{btn("⬅️ انصراف", fmt.Sprintf("cfz:%d:%s", intOfAny(pend["acc"]), strvVal(pend["zid"])))},
			})
		return
	}
	r.cfArCreate(chat, mid, pend, content, nil, false, false)
}

func (r *Router) cfArPriority(chat, mid int64, pend map[string]any, text string) {
	p, err := strconv.Atoi(strings.TrimSpace(text))
	if err != nil || p < 0 {
		r.Show(chat, mid, "❌ اولویت نامعتبر. یک عدد بفرست:", nil)
		return
	}
	r.cfArCreate(chat, mid, pend, strvVal(pend["content"]), &p, false, false)
}
