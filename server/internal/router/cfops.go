package router

import (
	"fmt"
	"net"
	"regexp"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/aknuun/cg-server/internal/cf"
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

// cfTypeForValue infers the record type an address/hostname belongs to.
func cfTypeForValue(s string) string {
	s = strings.TrimSpace(s)
	if ip := net.ParseIP(s); ip != nil {
		if ip.To4() != nil {
			return "A"
		}
		return "AAAA"
	}
	if cfIsHost(s) {
		return "CNAME"
	}
	return ""
}

// ---------- zone name cache ----------

type cfZnEntry struct {
	name string
	at   time.Time
}

var (
	cfZnMu    sync.Mutex
	cfZnCache = map[string]cfZnEntry{}
)

// cfZoneName resolves a zone id to its name, cached for a few minutes to keep
// the records/settings screens to a single API call.
func (r *Router) cfZoneName(acc int, zid string) string {
	key := fmt.Sprintf("%d:%s", acc, zid)
	cfZnMu.Lock()
	e, ok := cfZnCache[key]
	cfZnMu.Unlock()
	if ok && e.name != "" && time.Since(e.at) < 10*time.Minute {
		return e.name
	}
	cli, ok2 := r.cfClientFor(acc)
	if !ok2 {
		return zid
	}
	name := ""
	if zs, err := cli.Zones(); err == nil {
		for _, z := range zs {
			if z.ID == zid {
				name = z.Name
				break
			}
		}
	}
	if name != "" {
		cfZnMu.Lock()
		cfZnCache[key] = cfZnEntry{name: name, at: time.Now()}
		cfZnMu.Unlock()
		return name
	}
	return zid
}

func (r *Router) cfDropZoneNames(acc int) {
	prefix := fmt.Sprintf("%d:", acc)
	cfZnMu.Lock()
	for k := range cfZnCache {
		if strings.HasPrefix(k, prefix) {
			delete(cfZnCache, k)
		}
	}
	cfZnMu.Unlock()
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
		r.Show(chat, mid, body, navKB("cfr:"+rid))
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
		r.Show(chat, mid, "❌ رکورد پیدا نشد (شاید حذف شده).", navKB("zones"))
		return
	}
	key := strings.ToLower(strings.TrimSuffix(rec.Name, "."))
	n := t.Counts[key]
	if n == 0 && !strings.Contains(rec.Name, ".") {
		zname := strings.ToLower(r.cfZoneName(acc, zid))
		if rec.Name == "@" || rec.Name == "" {
			n = t.Counts[zname]
		} else {
			n = t.Counts[strings.ToLower(rec.Name)+"."+zname]
		}
	}
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
		r.Show(chat, mid, "❌ رکورد پیدا نشد.", navKB("zones"))
		return
	}
	target := strings.ToLower(strings.TrimRight(rec.Name, "."))
	r.Show(chat, mid, "⏳ در حال بررسی دامنه‌ها…", navKB("cfr:"+rid))
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
	r.Show(chat, mid, out, navKB("cfr:"+rid))
}

// ---------- favorites ----------

func (r *Router) cfFavsPage(chat, mid int64) {
	favs := r.favs()
	if len(favs) == 0 {
		r.Show(chat, mid, "⭐ ساب‌های منتخب\n\n📭 خالی است.",
			navKB("zones"))
		return
	}
	var items []Btn
	results := map[string]any{}
	n := min(len(favs), 40)
	for _, f := range favs[:n] {
		items = append(items, btnS(strvVal(f["name"]), "cfr:"+strvVal(f["rid"]), "plain"))
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
		favs = append(favs, map[string]any{"acc": acc, "zid": zid, "rid": rid,
			"name": cfRecLabel(rec.Name, rec.Type, r.cfZoneName(acc, zid))})
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
	zname := r.cfZoneName(acc, zid)
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
		items = append(items, btnS("👤 "+a.Name, fmt.Sprintf("zf:%d:0", i), "plain"))
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
	kb = append(kb, row(btn("🔙 بازگشت", "back:accounts")))
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
