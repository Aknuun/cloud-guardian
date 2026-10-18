package router

import (
	"fmt"
	"net"
	"regexp"
	"strconv"
	"strings"

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

func inSet(s string, set []string) bool {
	for _, x := range set {
		if x == s {
			return true
		}
	}
	return false
}

func cfSettingGroup(key string) (string, []string) {
	if inSet(key, cfSSLKeys) {
		return "zssl", cfSSLKeys
	}
	if inSet(key, cfSECKeys) {
		return "zsec", cfSECKeys
	}
	return "zperf", cfPERFKeys
}

func cfTtlLabel(ttl any) string {
	t, err := strconv.Atoi(strings.TrimSpace(fmt.Sprint(ttl)))
	if err != nil {
		return "خودکار"
	}
	if t == 1 || t == 120 {
		return "خودکار"
	}
	if t < 3600 {
		return fmt.Sprintf("%d دقیقه", t/60)
	}
	return fmt.Sprintf("%d ساعت", t/3600)
}

func cfRecLabel(name, rtype, zname string) string {
	if zname != "" && strings.HasSuffix(name, "."+zname) {
		name = name[:len(name)-(len(zname)+1)]
		if name == "" {
			name = "@"
		}
	}
	if rtype == "" {
		rtype = "A"
	}
	return name + "-" + strings.ToUpper(rtype)
}

func cfDupLabels(recs []cf.Record, zname string) map[string]string {
	counts := map[string]int{}
	for _, r := range recs {
		counts[r.Name]++
	}
	seen := map[string]int{}
	out := map[string]string{}
	for _, r := range recs {
		label := cfRecLabel(r.Name, r.Type, zname)
		if counts[r.Name] > 1 {
			seen[r.Name]++
			label = fmt.Sprintf("%d. %s", seen[r.Name], label)
		}
		out[r.ID] = label
	}
	return out
}

var cfHostRe = regexp.MustCompile(`^([a-z0-9]([a-z0-9-]{0,61}[a-z0-9])?\.)+[a-z]{2,}$`)

func cfIsHost(s string) bool {
	return len(s) <= 253 && cfHostRe.MatchString(s)
}

func cfValueHint(value, rtype string) string {
	s := strings.TrimSpace(value)
	if s == "" {
		return "مقدار خالی است؛ یک IPv4 یا نام دامنهٔ معتبر بفرستید."
	}
	ip := net.ParseIP(s)
	isV4 := ip != nil && ip.To4() != nil
	isV6 := ip != nil && ip.To4() == nil
	switch strings.ToUpper(rtype) {
	case "CNAME":
		if ip != nil {
			return "برای رکورد CNAME باید نام دامنهٔ معتبر بفرستید (مثل sub.example.com)؛ آی‌پی خام پذیرفته نمی‌شود."
		}
		if cfIsHost(s) {
			return ""
		}
		return "برای رکورد CNAME باید نام دامنهٔ معتبر بفرستید (مثل sub.example.com)."
	case "A":
		if isV4 {
			return ""
		}
		if isV6 {
			return "برای رکورد A باید IPv4 معتبر بفرستید (مثل 1.2.3.4)؛ مقدار ارسالی IPv6 است."
		}
		if cfIsHost(s) {
			return "برای رکورد A باید IPv4 معتبر بفرستید (مثل 1.2.3.4)؛ نام دامنه پذیرفته نمی‌شود."
		}
		return "برای رکورد A باید IPv4 معتبر بفرستید (مثل 1.2.3.4)."
	case "AAAA":
		if isV6 {
			return ""
		}
		return "برای رکورد AAAA باید IPv6 معتبر بفرستید."
	}
	return ""
}

// ---------- sessions ----------

func (r *Router) cfGetSession(chat int64) map[string]any {
	var s map[string]any
	if r.Store.GetJSON(fmt.Sprintf("cfctx:%d", chat), &s) {
		return s
	}
	return map[string]any{}
}

func (r *Router) cfSaveSession(chat int64, s map[string]any) {
	r.Store.PutJSON(fmt.Sprintf("cfctx:%d", chat), s, 86400)
}

func (r *Router) cfFind(acc int, zid, rid string) *cf.Record {
	cli, ok := r.cfClientFor(acc)
	if !ok {
		return nil
	}
	recs, err := cli.Records(zid)
	if err != nil {
		return nil
	}
	for i := range recs {
		if recs[i].ID == rid {
			return &recs[i]
		}
	}
	return nil
}

func cfBoolAny(v any) bool {
	b, ok := v.(bool)
	return ok && b
}

func zonesOf(cli *cf.Client) []cf.Zone {
	zs, _ := cli.Zones()
	return zs
}

// ---------- zones ----------

func (r *Router) cfZones(chat, mid int64, flt string, pg int) {
	accs := r.cfAccs()
	if len(accs) == 0 {
		r.Show(chat, mid, "❌ اکانت کلادفلر تنظیم نشده.\n\nتوکن API کلادفلر رو اضافه کن:",
			[][]Btn{{btn("➕ افزودن اکانت", "accadd"), btn("🏠 خانه", "menu")}})
		return
	}
	allz := r.cfAllZones()
	if len(allz) == 0 {
		r.Show(chat, mid, "☁️ کلودفلر\n\n📭 هیچ دامنه‌ای یافت نشد.", HomeKB())
		return
	}
	var zones []cfZone
	for _, z := range allz {
		if z.Zone.Status == "active" && (flt == "all" || strv(z.Acc) == flt) {
			zones = append(zones, z)
		}
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
		if z.Zone.Status == "active" {
			mark = "🟢"
		}
		rows = append(rows, btn(mark+" "+z.Zone.Name, fmt.Sprintf("cfz:%d:%s", z.Acc, z.Zone.ID)))
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
		nav = append(nav, btn("🔙 بازگشت", "menu"))
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
	zname := zid
	for _, z := range zonesOf(cli) {
		if z.ID == zid {
			zname = z.Name
			break
		}
	}
	sess := r.cfGetSession(chat)
	sess["acc"], sess["zid"] = acc, zid
	r.cfSaveSession(chat, sess)
	sel, hasSel := sess["sel"].([]any)
	if len(recs) == 0 {
		r.Show(chat, mid, fmt.Sprintf("📭 رکوردی برای %s یافت نشد.", Esc(zname)),
			[][]Btn{{btn("➕ افزودن", fmt.Sprintf("cfadd:%d:%s", acc, zid))},
				{btn("⚙️ تنظیمات", "zset")},
				{btn("🔙 بازگشت", "zones"), btn("🏠 خانه", "menu")}})
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
			rows = append(rows, btn(labels[rec.ID], "cfr:"+rec.ID))
		} else {
			if selHas(sel, rec.ID) {
				rows = append(rows, btn("✅ "+labels[rec.ID], "sel:"+rec.ID))
			} else {
				rows = append(rows, btn("⬜ "+labels[rec.ID], "sel:"+rec.ID))
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
		r.Show(chat, mid, fmt.Sprintf("🗂 %s — %d انتخاب شده", Esc(zname), len(sel)), kb)
		return
	}
	kb = append(kb, []Btn{btn("⚙️ تنظیمات", "zset"), btn("🗂 گروهی", "selmode"),
		btn("➕ افزودن", fmt.Sprintf("cfadd:%d:%s", acc, zid)), btn("🔍 جستجو", "zsearch")})
	kb = append(kb, []Btn{btn("✉️ ایمیل", "zmail"), btn("📊 ترافیک ساب‌ها", "ztraf")})
	var nav []Btn
	if pg > 0 {
		nav = append(nav, btn("◀️", fmt.Sprintf("cfp:%d", pg-1)))
	} else {
		nav = append(nav, emptyBtn())
	}
	if pages > 1 {
		nav = append(nav, btn(fmt.Sprintf("📄 %d/%d", pg+1, pages), "noop"))
	} else {
		nav = append(nav, btn("🔙 بازگشت", "zones"))
	}
	if pg < pages-1 {
		nav = append(nav, btn("▶️", fmt.Sprintf("cfp:%d", pg+1)))
	} else {
		nav = append(nav, emptyBtn())
	}
	kb = append(kb, nav)
	if pages > 1 {
		kb = append(kb, []Btn{btn("🔙 بازگشت", "zones"), btn("🏠 خانه", "menu")})
	}
	title := "📋 " + Esc(zname)
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
	zname := zid
	for _, z := range zonesOf(cli) {
		if z.ID == zid {
			zname = z.Name
			break
		}
	}
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
	kb = append(kb, []Btn{btn("🔙 بازگشت", "zones"), btn("🏠 خانه", "menu")})
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
			r.Show(chat, mid, "❌ حذف نشد: "+Esc(err.Error()), HomeKB())
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
		r.Show(chat, mid, "❌ برای نوع A باید مقدار آیپی باشد.", HomeKB())
		return
	}
	if target == "CNAME" && isIP {
		r.Show(chat, mid, "❌ برای نوع CNAME باید مقدار هاست‌نیم باشد.", HomeKB())
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
		rows = append(rows, btn(z.Zone.Name, fmt.Sprintf("addrec:%d:%s", z.Acc, z.Zone.ID)))
	}
	kb := gridBTN(rows, 2)
	kb = append(kb, []Btn{btn("🔙 بازگشت", "zones"), btn("🏠 خانه", "menu")})
	r.Show(chat, mid, "➕ افزودن رکورد — دامنه را انتخاب کن:", kb)
}

func (r *Router) cfAddStart(chat, mid int64, acc int, zid string) {
	types := []string{"A", "AAAA", "CNAME", "TXT", "MX", "NS"}
	var rows []Btn
	for _, t := range types {
		rows = append(rows, btn(t, fmt.Sprintf("cfat:%d:%s:%s", acc, zid, t)))
	}
	kb := gridBTN(rows, 3)
	kb = append(kb, []Btn{btn("🔙 بازگشت", fmt.Sprintf("cfz:%d:%s", acc, zid)), btn("🏠 خانه", "menu")})
	zname := zid
	if cli, ok := r.cfClientFor(acc); ok {
		if zs, err := cli.Zones(); err == nil {
			for _, z := range zs {
				if z.ID == zid {
					zname = z.Name
				}
			}
		}
	}
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

func (r *Router) cfArCreate(chat int64, pend map[string]any, content string, priority *int) {
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
	if _, err := cli.AddRecord(strvVal(pend["zid"]), rtype, strvVal(pend["name"]), content, 1, false, priority); err != nil {
		r.Bot.Send(chat, "❌ ثبت نشد: "+Esc(err.Error()), nil)
		return
	}
	r.clearWizard(chat)
	r.Bot.Send(chat, "✅ رکورد ثبت شد.", nil)
	r.cfRecords(chat, 0, acc, strvVal(pend["zid"]), 0)
}

// ---------- edit record (wizard) ----------

func (r *Router) cfEditStart(chat, mid int64, acc int, zid, rid string) {
	rec := r.cfFind(acc, zid, rid)
	r.setWizard(chat, map[string]any{"kind": "cr_cfedit", "acc": acc, "zid": zid, "rid": rid})
	name, content := "", ""
	if rec != nil {
		name, content = rec.Name, rec.Content
	}
	r.Show(chat, mid, fmt.Sprintf("✏️ مقدار جدید را بفرستید (IP یا هدف CNAME):\n\n📛 سابی که تغییر می‌کند: <code>%s</code>\n\n🔴 مقدار فعلی که عوض می‌شود:\n<code>%s</code>",
		Esc(name), Esc(content)), [][]Btn{{btn("⬅️ انصراف", "cfr:"+rid)}})
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
	if hint := cfValueHint(text, strings.ToUpper(rec.Type)); hint != "" {
		r.Bot.Send(chat, "❌ "+hint+"\n\nپیام اصلاح‌شده را دوباره بفرستید؛ همین‌جا ویرایش می‌شود.",
			[][]Btn{{btn("⬅️ انصراف", "cfr:"+rid)}})
		return
	}
	cli, _ := r.cfClientFor(acc)
	if _, err := cli.UpdateRecord(zid, *rec, map[string]any{"content": strings.TrimSpace(text)}); err != nil {
		r.Bot.Send(chat, "❌ نشد: "+Esc(err.Error()), nil)
		return
	}
	r.clearWizard(chat)
	r.Bot.Send(chat, "✅ مقدار عوض شد.", nil)
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
	zname := zid
	if zs, err := cli.Zones(); err == nil {
		for _, z := range zs {
			if z.ID == zid {
				zname = z.Name
			}
		}
	}
	return acc, zid, zname, cli, true
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
	kb = append(kb, []Btn{btn("🔙 بازگشت", fmt.Sprintf("cfz:%d:%s", acc, zid)), btn("🏠 خانه", "menu")})
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
	kb = append(kb, []Btn{btn("🔙 بازگشت", "zset"), btn("🏠 خانه", "menu")})
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
		r.Show(chat, mid, "❌ خطا:\n"+Esc(err.Error()), [][]Btn{{btn("🔙 بازگشت", "zset")}})
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
	kb = append(kb, []Btn{btn("🔙 بازگشت", "zset"), btn("🏠 خانه", "menu")})
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
		r.Show(chat, mid, "❌ خطا:\n"+Esc(err.Error()), [][]Btn{{btn("🔙 بازگشت", "zset")}})
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
		[][]Btn{{btn("🔙 بازگشت", fmt.Sprintf("cfz:%d:%s", acc, zid)), btn("🏠 خانه", "menu")}})
}

func (r *Router) cfPurge(chat, mid int64) {
	_, zid, _, cli, ok := r.cfZctx(chat)
	if !ok {
		r.Show(chat, mid, "⏳ نشست منقضی شد.", HomeKB())
		return
	}
	if err := cli.PurgeCache(zid); err != nil {
		r.Show(chat, mid, "❌ خطا:\n"+Esc(err.Error()), [][]Btn{{btn("🔙 بازگشت", "zset")}})
		return
	}
	r.Show(chat, mid, "🧹 کل کش دامنه پاک شد.",
		[][]Btn{{btn("🔙 بازگشت", "zset"), btn("🏠 خانه", "menu")}})
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
				r.Show(chat, mid, "🚧 حالت توسعه روشن شد.", [][]Btn{{btn("🔙 بازگشت", "zset"), btn("🏠 خانه", "menu")}})
			} else {
				r.Show(chat, mid, "🚧 حالت توسعه خاموش شد.", [][]Btn{{btn("🔙 بازگشت", "zset"), btn("🏠 خانه", "menu")}})
			}
			return
		}
	}
	r.Show(chat, mid, "❌ خطا:\n"+Esc(err.Error()), [][]Btn{{btn("🔙 بازگشت", "zset")}})
}

func (r *Router) cfZonePause(chat, mid int64) {
	_, zid, _, cli, ok := r.cfZctx(chat)
	if !ok {
		r.Show(chat, mid, "⏳ نشست منقضی شد.", HomeKB())
		return
	}
	z, err := cli.Zone(zid)
	if err != nil {
		r.Show(chat, mid, "❌ خطا:\n"+Esc(err.Error()), [][]Btn{{btn("🔙 بازگشت", "zset")}})
		return
	}
	active := strvVal(z["status"]) == "active"
	if err := cli.SetZonePaused(zid, active); err != nil {
		r.Show(chat, mid, "❌ خطا:\n"+Esc(err.Error()), [][]Btn{{btn("🔙 بازگشت", "zset")}})
		return
	}
	msg := "▶️ دامنه فعال شد."
	if active {
		msg = "⏸️ دامنه متوقف شد."
	}
	r.Show(chat, mid, msg, [][]Btn{{btn("🔙 بازگشت", "zset"), btn("🏠 خانه", "menu")}})
}

func (r *Router) cfRecordTraffic(chat, mid int64) {
	sess := r.cfGetSession(chat)
	acc := intOrAny(sess["acc"])
	zid := strvVal(sess["zid"])
	rid := strvVal(sess["rid"])
	if zid == "" || rid == "" {
		r.Show(chat, mid, "⏳ نشست منقضی شد؛ رکورد را دوباره باز کن.", HomeKB())
		return
	}
	cli, ok := r.cfClientFor(acc)
	if !ok {
		r.Show(chat, mid, "⏳ نشست منقضی شد؛ رکورد را دوباره باز کن.", HomeKB())
		return
	}
	recs, err := cli.Records(zid)
	if err != nil {
		r.Show(chat, mid, "❌ خطای کلادفلر: "+Esc(err.Error()), HomeKB())
		return
	}
	accs := r.cfAccs()
	token := ""
	if acc >= 0 && acc < len(accs) {
		token = accs[acc].Token
	}
	t := cf.FetchTrafficCounts(token, zid, 1)
	if t.NeedPerm || t.Error != "" {
		body := "❌ دسترسی Analytics نیست (Zone → Analytics → Read)."
		if !t.NeedPerm {
			body = "❌ خطا در آمار:\n" + Esc(t.Error)
		}
		r.Show(chat, mid, body, [][]Btn{{btn("🔙 بازگشت", "cfr:"+rid), btn("🏠 خانه", "menu")}})
		return
	}
	var rec *cf.Record
	for i := range recs {
		if recs[i].ID == rid {
			rec = &recs[i]
			break
		}
	}
	if rec == nil {
		r.Show(chat, mid, "❌ رکورد پیدا نشد (شاید حذف شده).", [][]Btn{{btn("🔙 بازگشت", "zones")}})
		return
	}
	n := t.Counts[strings.ToLower(rec.Name)]
	traffic := fmt.Sprintf("📊 ترافیک امروز: %s کوئری", faD(commaFmt(n)))
	r.cfRecord(chat, mid, acc, zid, rid, traffic)
}

func (r *Router) cfCnameRefs(chat, mid int64) {
	sess := r.cfGetSession(chat)
	acc := intOrAny(sess["acc"])
	zid := strvVal(sess["zid"])
	rid := strvVal(sess["rid"])
	if zid == "" || rid == "" {
		r.Show(chat, mid, "⏳ نشست منقضی شد؛ رکورد را دوباره باز کن.", HomeKB())
		return
	}
	cli, ok := r.cfClientFor(acc)
	if !ok {
		r.Show(chat, mid, "⏳ نشست منقضی شد؛ رکورد را دوباره باز کن.", HomeKB())
		return
	}
	recs, err := cli.Records(zid)
	if err != nil {
		r.Show(chat, mid, "❌ خطای کلادفلر: "+Esc(err.Error()), HomeKB())
		return
	}
	var rec *cf.Record
	for i := range recs {
		if recs[i].ID == rid {
			rec = &recs[i]
			break
		}
	}
	if rec == nil {
		r.Show(chat, mid, "❌ رکورد پیدا نشد.", [][]Btn{{btn("🔙 بازگشت", "zones")}})
		return
	}
	target := strings.ToLower(strings.TrimRight(rec.Name, "."))
	r.Show(chat, mid, "⏳ در حال بررسی دامنه‌ها…", [][]Btn{{btn("🔙 بازگشت", "cfr:"+rid)}})
	type ref struct{ name, zone string }
	var refs []ref
	for _, z := range r.cfAllZones() {
		zc, ok := r.cfClientFor(z.Acc)
		if !ok {
			continue
		}
		zrecs, err := zc.Records(z.Zone.ID)
		if err != nil {
			continue
		}
		for _, x := range zrecs {
			if strings.ToUpper(x.Type) != "CNAME" {
				continue
			}
			if strings.ToLower(strings.TrimRight(x.Content, ".")) == target {
				refs = append(refs, ref{name: x.Name, zone: z.Zone.Name})
			}
		}
	}
	lines := []string{"🔗 CNAMEهای وارده", "", "🎯 رکورد: <code>" + Esc(target) + "</code>", ""}
	if len(refs) == 0 {
		lines = append(lines, "📭 هیچ دامنه/ساب‌دامین‌ای به این رکورد CNAME نکرده است.")
	} else {
		lines = append(lines, fmt.Sprintf("📊 تعداد: %d", len(refs)), "")
		for i, f := range refs {
			lines = append(lines, fmt.Sprintf("%d) ☁️ <code>%s</code> — <code>%s</code>", i+1, Esc(f.name), Esc(f.zone)))
		}
	}
	out := strings.Join(lines, "\n")
	if len(out) > 3500 {
		out = out[:3500]
	}
	r.Show(chat, mid, out, [][]Btn{{btn("🔙 بازگشت", "cfr:"+rid), btn("🏠 خانه", "menu")}})
}

// ---------- favorites ----------

func (r *Router) cfFavsPage(chat, mid int64) {
	favs := r.favs()
	if len(favs) == 0 {
		r.Show(chat, mid, "⭐ ساب‌های منتخب\n\n📭 خالی است.",
			[][]Btn{{btn("🔙 بازگشت", "zones"), btn("🏠 خانه", "menu")}})
		return
	}
	var items []Btn
	results := map[string]any{}
	n := min(len(favs), 40)
	for _, f := range favs[:n] {
		items = append(items, btn(strvVal(f["name"]), "cfr:"+strvVal(f["rid"])))
		results[strvVal(f["rid"])] = map[string]any{"acc": intOrAny(f["acc"]), "zid": strvVal(f["zid"])}
	}
	kb := gridBTN(items, 2)
	kb = append(kb, navRow("zones"))
	r.cfSaveSession(chat, map[string]any{"results": results})
	r.Show(chat, mid, fmt.Sprintf("⭐ ساب‌های منتخب (%d):", len(favs)), kb)
}

func (r *Router) cfFavRm(chat, mid int64, rid string) {
	var out []map[string]any
	for _, f := range r.favs() {
		if strvVal(f["rid"]) != rid {
			out = append(out, f)
		}
	}
	r.Store.PutJSON("favs", toAnyList(out), 0)
	r.cfFavsPage(chat, mid)
}

func toAnyList(m []map[string]any) []any {
	out := make([]any, 0, len(m))
	for _, x := range m {
		out = append(out, x)
	}
	return out
}

func (r *Router) cfFavToggle(chat, mid int64, acc int, zid, rid string) {
	favs := r.favs()
	exists := false
	for _, f := range favs {
		if strvVal(f["rid"]) == rid {
			exists = true
		}
	}
	msg := ""
	if exists {
		var out []map[string]any
		for _, f := range favs {
			if strvVal(f["rid"]) != rid {
				out = append(out, f)
			}
		}
		favs = out
		msg = "⭐ از منتخب حذف شد."
	} else {
		rec := r.cfFind(acc, zid, rid)
		if rec == nil {
			r.Show(chat, mid, "❌ رکورد پیدا نشد.", HomeKB())
			return
		}
		zname := zid
		if cli, ok := r.cfClientFor(acc); ok {
			if zs, err := cli.Zones(); err == nil {
				for _, z := range zs {
					if z.ID == zid {
						zname = z.Name
					}
				}
			}
		}
		favs = append(favs, map[string]any{"acc": acc, "zid": zid, "rid": rid,
			"name": cfRecLabel(rec.Name, rec.Type, zname)})
		msg = "⭐ به منتخب اضافه شد."
	}
	r.Store.PutJSON("favs", toAnyList(favs), 0)
	r.Bot.Send(chat, msg, nil)
	r.cfRecord(chat, 0, acc, zid, rid, "")
}

// ---------- bulk operations ----------

func (r *Router) cfBulkDelDo(chat, mid int64) {
	sess := r.cfGetSession(chat)
	sel, _ := sess["sel"].([]any)
	acc := intOrAny(sess["acc"])
	zid := strvVal(sess["zid"])
	if len(sel) == 0 || zid == "" {
		r.Show(chat, mid, "⚠️ چیزی انتخاب نشده.", HomeKB())
		return
	}
	cli, ok := r.cfClientFor(acc)
	if !ok {
		r.Show(chat, mid, "❌ اکانت پیدا نشد.", HomeKB())
		return
	}
	okn, fail := 0, 0
	for _, x := range sel {
		if err := cli.DelRecord(zid, strvVal(x)); err != nil {
			fail++
		} else {
			okn++
		}
	}
	delete(sess, "sel")
	r.cfSaveSession(chat, sess)
	msg := fmt.Sprintf("✅ %d حذف شد.", okn)
	if fail > 0 {
		msg += fmt.Sprintf(" ⚠️ %d ناموفق.", fail)
	}
	r.Bot.Send(chat, msg, nil)
	r.cfRecords(chat, 0, acc, zid, intOrAny(sess["pg"]))
}

func (r *Router) cfBulkTypeDo(chat, mid int64, rtype string) {
	sess := r.cfGetSession(chat)
	sel, _ := sess["sel"].([]any)
	acc := intOrAny(sess["acc"])
	zid := strvVal(sess["zid"])
	if len(sel) == 0 || zid == "" {
		r.Show(chat, mid, "⚠️ چیزی انتخاب نشده.", HomeKB())
		return
	}
	cli, ok := r.cfClientFor(acc)
	if !ok {
		r.Show(chat, mid, "❌ اکانت پیدا نشد.", HomeKB())
		return
	}
	recs, err := cli.Records(zid)
	if err != nil {
		r.Show(chat, mid, "❌ "+Esc(err.Error()), HomeKB())
		return
	}
	okn, skip := 0, 0
	for _, x := range sel {
		rid := strvVal(x)
		var rec *cf.Record
		for i := range recs {
			if recs[i].ID == rid {
				rec = &recs[i]
				break
			}
		}
		if rec == nil {
			skip++
			continue
		}
		if cfValueHint(rec.Content, rtype) != "" {
			skip++
			continue
		}
		if _, err := cli.UpdateRecord(zid, *rec, map[string]any{"type": rtype}); err != nil {
			skip++
		} else {
			okn++
		}
	}
	delete(sess, "sel")
	r.cfSaveSession(chat, sess)
	msg := fmt.Sprintf("✅ نوع %d رکورد عوض شد.", okn)
	if skip > 0 {
		msg += fmt.Sprintf(" ⚠️ %d رد شد.", skip)
	}
	r.Bot.Send(chat, msg, nil)
	r.cfRecords(chat, 0, acc, zid, intOrAny(sess["pg"]))
}

func (r *Router) cfBulkEditSave(chat, mid int64, text string) {
	sess := r.cfGetSession(chat)
	sel, _ := sess["sel"].([]any)
	acc := intOrAny(sess["acc"])
	zid := strvVal(sess["zid"])
	if len(sel) == 0 || zid == "" {
		r.clearWizard(chat)
		r.Bot.Send(chat, "⚠️ چیزی انتخاب نشده.", nil)
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
	okn, skip := 0, 0
	for _, x := range sel {
		rid := strvVal(x)
		var rec *cf.Record
		for i := range recs {
			if recs[i].ID == rid {
				rec = &recs[i]
				break
			}
		}
		if rec == nil {
			skip++
			continue
		}
		if cfValueHint(text, strings.ToUpper(rec.Type)) != "" {
			skip++
			continue
		}
		if _, err := cli.UpdateRecord(zid, *rec, map[string]any{"content": strings.TrimSpace(text)}); err != nil {
			skip++
		} else {
			okn++
		}
	}
	r.clearWizard(chat)
	delete(sess, "sel")
	r.cfSaveSession(chat, sess)
	msg := fmt.Sprintf("✅ %d رکورد عوض شد.", okn)
	if skip > 0 {
		msg += fmt.Sprintf(" ⚠️ %d رد شد.", skip)
	}
	r.Bot.Send(chat, msg, nil)
	r.cfRecords(chat, 0, acc, zid, intOrAny(sess["pg"]))
}

func (r *Router) cfFavAddMany(chat, mid int64, sess map[string]any) {
	sel, _ := sess["sel"].([]any)
	acc := intOrAny(sess["acc"])
	zid := strvVal(sess["zid"])
	if len(sel) == 0 || zid == "" {
		r.Show(chat, mid, "⚠️ چیزی انتخاب نشده.", HomeKB())
		return
	}
	cli, ok := r.cfClientFor(acc)
	if !ok {
		r.Show(chat, mid, "❌ اکانت پیدا نشد.", HomeKB())
		return
	}
	zname := zid
	if zs, err := cli.Zones(); err == nil {
		for _, z := range zs {
			if z.ID == zid {
				zname = z.Name
			}
		}
	}
	recs, err := cli.Records(zid)
	if err != nil {
		r.Show(chat, mid, "❌ "+Esc(err.Error()), HomeKB())
		return
	}
	favs := r.favs()
	have := map[string]bool{}
	for _, f := range favs {
		have[strvVal(f["rid"])] = true
	}
	n := 0
	for _, x := range sel {
		rid := strvVal(x)
		if have[rid] {
			continue
		}
		var rec *cf.Record
		for i := range recs {
			if recs[i].ID == rid {
				rec = &recs[i]
				break
			}
		}
		if rec == nil {
			continue
		}
		favs = append(favs, map[string]any{"acc": acc, "zid": zid, "rid": rid,
			"name": cfRecLabel(rec.Name, rec.Type, zname)})
		n++
	}
	r.Store.PutJSON("favs", toAnyList(favs), 0)
	delete(sess, "sel")
	r.cfSaveSession(chat, sess)
	r.Bot.Send(chat, fmt.Sprintf("✅ %d مورد به منتخب اضافه شد.", n), nil)
	r.cfRecords(chat, 0, acc, zid, intOrAny(sess["pg"]))
}

// ---------- accounts ----------

func (r *Router) cfAccountsHome(chat, mid int64) {
	accs := r.cfAccs()
	var items []Btn
	for i, a := range accs {
		items = append(items, btn("👤 "+a.Name, fmt.Sprintf("zf:%d:0", i)))
	}
	kb := gridBTN(items, 2)
	kb = append(kb, row(btn("➕ افزودن اکانت", "accadd"), btn("🗑 حذف اکانت", "accdel")))
	kb = append(kb, navRow("zones"))
	r.Show(chat, mid, "👤 اکانت‌های کلادفلر:", kb)
}

func (r *Router) cfAccDelList(chat, mid int64) {
	var items []Btn
	for i, a := range r.cfAccs() {
		items = append(items, btnS("🗑 "+a.Name, fmt.Sprintf("accd:%d", i), "danger"))
	}
	kb := gridBTN(items, 2)
	kb = append(kb, row(btn("🔙 بازگشت", "accounts")))
	r.Show(chat, mid, "🗑 کدام اکانت حذف شود؟", kb)
}

func (r *Router) cfAccDelAsk(chat, mid int64, idx string) {
	accs := r.cfAccs()
	i := atoiSafe(idx)
	if i < 0 || i >= len(accs) {
		r.Show(chat, mid, "❌ اکانت پیدا نشد.", HomeKB())
		return
	}
	r.Show(chat, mid, fmt.Sprintf("⚠️ اکانت «%s» حذف شود؟", Esc(accs[i].Name)),
		[][]Btn{{btnS("✅ بله", "accdy:"+idx, "danger"), btn("❌ انصراف", "accdel")}})
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
	for i, a := range r.cfAccs() {
		cli := cf.New(a.Token)
		zs, err := cli.Zones()
		if err != nil {
			continue
		}
		for _, z := range zs {
			if z.Status != "active" {
				continue
			}
			if strings.Contains(strings.ToLower(z.Name), q) {
				hits = append(hits, hit{acc: i, zone: z})
			}
			recs, err := cli.Records(z.ID)
			if err != nil {
				continue
			}
			for k := range recs {
				rc := recs[k]
				if strings.Contains(strings.ToLower(rc.Name), q) || strings.Contains(strings.ToLower(rc.Content), q) {
					hits = append(hits, hit{acc: i, zone: z, rec: &rc})
				}
			}
		}
		if len(hits) >= 30 {
			break
		}
	}
	if len(hits) == 0 {
		r.Show(chat, mid, "🔍 چیزی پیدا نشد.", HomeKB())
		return
	}
	results := map[string]any{}
	var items []Btn
	n := min(len(hits), 30)
	for _, h := range hits[:n] {
		if h.rec == nil {
			items = append(items, btn("📁 "+h.zone.Name, fmt.Sprintf("cfz:%d:%s", h.acc, h.zone.ID)))
		} else {
			results[h.rec.ID] = map[string]any{"acc": h.acc, "zid": h.zone.ID}
			items = append(items, btn(cfRecLabel(h.rec.Name, h.rec.Type, h.zone.Name), "cfr:"+h.rec.ID))
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
	labels := cfDupLabels(hits, zid)
	var rows []Btn
	for _, rc := range hits {
		rows = append(rows, btn(labels[rc.ID], "cfr:"+rc.ID))
	}
	kb := gridBTN(rows, 3)
	kb = append(kb, []Btn{btn("🔙 بازگشت", "zones"), btn("🏠 خانه", "menu")})
	r.Bot.Send(chat, fmt.Sprintf("🔍 نتایج (%d):", len(hits)), kb)
}

// ---------- wizards ----------

func (r *Router) cfAccName(chat, mid int64, text string) {
	name := strings.TrimSpace(text)
	if name == "" {
		r.Show(chat, mid, "❌ اسم نامعتبر. دوباره بفرست:", [][]Btn{{btn("🔙 بازگشت", "accounts")}})
		return
	}
	r.setWizard(chat, map[string]any{"kind": "cr_cf_acc_token", "name": name})
	r.Show(chat, mid, "🔑 توکن API کلادفلر را بفرستید:", [][]Btn{{btn("🔙 بازگشت", "accounts")}})
}

func (r *Router) cfAccToken(chat, mid int64, pend map[string]any, text string) {
	tok := strings.TrimSpace(text)
	if len(tok) < 20 {
		r.Show(chat, mid, "❌ توکن نامعتبر. دوباره بفرست:", [][]Btn{{btn("🔙 بازگشت", "accounts")}})
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
	r.cfArCreate(chat, pend, content, nil)
}

func (r *Router) cfArPriority(chat, mid int64, pend map[string]any, text string) {
	p, err := strconv.Atoi(strings.TrimSpace(text))
	if err != nil || p < 0 {
		r.Show(chat, mid, "❌ اولویت نامعتبر. یک عدد بفرست:", nil)
		return
	}
	r.cfArCreate(chat, pend, strvVal(pend["content"]), &p)
}
