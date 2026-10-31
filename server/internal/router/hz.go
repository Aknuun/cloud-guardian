package router

import (
	"encoding/json"
	"fmt"
	"math"
	"strconv"
	"strings"
	"time"

	"github.com/aknuun/cg-server/internal/hz"
)

func (r *Router) hzAccs() []map[string]any {
	var accs []map[string]any
	if !r.Store.GetJSON("hz_accounts", &accs) {
		return nil
	}
	return accs
}

func (r *Router) hzSaveAccs(accs []map[string]any) {
	r.Store.PutJSON("hz_accounts", accs, 0)
}

// ---------- cached Hetzner lookups (slow API, cached like Arvan) ----------

func (r *Router) hzCli(i int) (*hz.Client, bool) {
	accs := r.hzAccs()
	if i < 0 || i >= len(accs) {
		return nil, false
	}
	return hz.New(strv(accs[i]["token"])), true
}

// hzDropServers invalidates the cached server list after a mutation.
func (r *Router) hzDropServers(i int) {
	arvMem.Delete(fmt.Sprintf("mhzs:%d", i))
	r.Store.Delete(fmt.Sprintf("hzsrv:%d", i))
}

// hzFlushIndex drops every cached Hetzner entry for an account index (used
// when an account is removed and the following ones shift down).
func (r *Router) hzFlushIndex(i int) {
	heads := map[string]bool{
		fmt.Sprintf("mhzs:%d", i): true, fmt.Sprintf("mhzi:%d", i): true,
		fmt.Sprintf("mhzt:%d", i): true, fmt.Sprintf("mhzl:%d", i): true,
	}
	arvMem.Range(func(k, v any) bool {
		if sk, ok := k.(string); ok && heads[sk] {
			arvMem.Delete(k)
		}
		return true
	})
	for _, db := range []string{"hzsrv", "hzimg", "hztyp", "hzloc"} {
		r.Store.Delete(fmt.Sprintf("%s:%d", db, i))
	}
}

func (r *Router) hzServersCached(i int) []map[string]any {
	cli, ok := r.hzCli(i)
	if !ok {
		return nil
	}
	return r.arvCachedMaps(fmt.Sprintf("mhzs:%d", i), fmt.Sprintf("hzsrv:%d", i), 60*time.Second,
		func() []map[string]any { return cli.All("/servers") })
}

// hzServersQuick returns the cached server list without blocking. When there is
// no cache at all it reports ok=false so the caller can show a loader.
func (r *Router) hzServersQuick(i int) ([]map[string]any, bool) {
	if _, ok := arvMem.Load(fmt.Sprintf("mhzs:%d", i)); ok {
		return r.hzServersCached(i), true
	}
	if _, ok := r.Store.GetOK(fmt.Sprintf("hzsrv:%d", i)); ok {
		return r.hzServersCached(i), true
	}
	return nil, false
}

func (r *Router) hzImagesCached(cli *hz.Client, i int) []map[string]any {
	return r.arvCachedMaps(fmt.Sprintf("mhzi:%d", i), fmt.Sprintf("hzimg:%d", i), 6*time.Hour,
		func() []map[string]any { return r.hzAllImages(cli) })
}

func (r *Router) hzTypesCached(cli *hz.Client, i int) []map[string]any {
	return r.arvCachedMaps(fmt.Sprintf("mhzt:%d", i), fmt.Sprintf("hztyp:%d", i), 6*time.Hour,
		func() []map[string]any { return cli.All("/server_types") })
}

func (r *Router) hzLocsCached(cli *hz.Client, i int) []map[string]any {
	return r.arvCachedMaps(fmt.Sprintf("mhzl:%d", i), fmt.Sprintf("hzloc:%d", i), 24*time.Hour,
		func() []map[string]any { return cli.All("/locations") })
}

// hzEuroCached never blocks: it returns the last known EUR rate and refreshes
// it in the background.
func (r *Router) hzEuroCached() float64 {
	var v float64
	if r.Store.GetJSON("hzeur", &v) && v > 0 {
		return v
	}
	go func() {
		if x := hz.EuroRate(); x > 0 {
			r.Store.PutJSON("hzeur", x, 6*3600)
		}
	}()
	return 0
}

func (r *Router) hzSel() int {
	accs := r.hzAccs()
	i, err := strconv.Atoi(strings.TrimSpace(r.Store.Get("hz_sel")))
	if err == nil && i >= 0 && i < len(accs) {
		return i
	}
	return 0
}

func hzF(f float64) string { return strconv.FormatFloat(f, 'f', -1, 64) }

func hzIP(s map[string]any) string {
	pn, _ := s["public_net"].(map[string]any)
	ip4, _ := pn["ipv4"].(map[string]any)
	return strv(ip4["ip"])
}

func (r *Router) hzHome(chat, mid int64) {
	accs := r.hzAccs()
	if len(accs) == 0 {
		r.Show(chat, mid, "🇩🇪 هتزنر\n\nاکانتی نیست؛ اضافه کن:",
			[][]Btn{{btn("➕ افزودن اکانت", "hza")}, {btn("🏠 خانه", "menu")}})
		return
	}
	sel := r.hzSel()
	var kb [][]Btn
	multi := len(accs) > 1
	nm := strv(accs[sel]["name"])
	if multi {
		var row []Btn
		for i, a := range accs {
			if i >= 4 {
				break
			}
			mark := "👤 "
			if i == sel {
				mark = "✅ "
			}
			row = append(row, btn(mark+strv(a["name"]), fmt.Sprintf("hzm:%d", i)))
		}
		kb = append(kb, row)
	}
	slist, ready := r.hzServersQuick(sel)
	if !ready {
		r.Show(chat, mid, "⏳ در حال بارگذاری سرورها…", nil)
		slist = r.hzServersCached(sel)
	}
	lines := []string{fmt.Sprintf("🇩🇪 هتزنر — «%s»", Esc(strv(accs[sel]["name"]))), ""}
	if len(slist) == 0 {
		lines = append(lines, "📭 سروری نیست.")
	}
	var srvItems []Btn
	for i, s := range slist {
		if i >= 20 {
			break
		}
		ip := hzIP(s)
		if ip == "" {
			ip = "—"
		}
		label := strv(s["name"])
		if label == "" {
			label = strv(s["id"])
		}
		srvItems = append(srvItems, btnS(fmt.Sprintf("%s %s · %s", hz.StatusEmoji(strv(s["status"])), truncRune(label, 16), ip),
			fmt.Sprintf("hzsi:%d:%s", sel, strv(s["id"])), "plain"))
	}
	kb = append(kb, gridBTN(srvItems, 2)...)
	kb = append(kb, []Btn{btn("🖥 سرورها", fmt.Sprintf("hzs:%d:0", sel)), btn("👤 اکانت‌ها", "hzaccs")})
	createLabel := "➕ ساخت سرور"
	if multi {
		createLabel = "➕ ساخت سرور در " + nm
	}
	kb = append(kb, []Btn{btn(createLabel, fmt.Sprintf("hzsc:%d", sel))})
	kb = append(kb, []Btn{btn("🧼 استخر IP تمیز", fmt.Sprintf("hzip:%d", sel))})
	kb = append(kb, []Btn{btn("🏠 خانه", "menu")})
	r.Show(chat, mid, strings.Join(lines, "\n"), kb)
}

func hzList(d map[string]any, key string) []map[string]any {
	arr, _ := d[key].([]any)
	var out []map[string]any
	for _, x := range arr {
		if m, ok := x.(map[string]any); ok {
			out = append(out, m)
		}
	}
	return out
}

func (r *Router) hzAccsMenu(chat, mid int64) {
	kb := [][]Btn{{btn("➕ افزودن اکانت", "hza")}}
	if len(r.hzAccs()) > 0 {
		kb = append(kb, []Btn{btn("🗑 حذف اکانت", "hzdel")})
	}
	kb = append(kb, []Btn{btn("🔙 بازگشت", "back:hz"), btn("🏠 خانه", "menu")})
	r.Show(chat, mid, "👤 اکانت‌های هتزنر:", kb)
}

func (r *Router) hzDelList(chat, mid int64) {
	accs := r.hzAccs()
	if len(accs) == 0 {
		r.Show(chat, mid, "📭 اکانتی نیست.", navKB("hz"))
		return
	}
	var items []Btn
	for i, a := range accs {
		items = append(items, btnS("🗑 "+strv(a["name"]), fmt.Sprintf("hzd:%d", i), "plain"))
	}
	kb := gridBTN(items, 2)
	kb = append(kb, row(btn("🔙 بازگشت", "back:hz")))
	r.Show(chat, mid, "🗑 کدام اکانت حذف شود؟", kb)
}

func (r *Router) hzDelAsk(chat, mid int64, idx string) {
	accs := r.hzAccs()
	i := atoiSafe(idx)
	if i < 0 || i >= len(accs) {
		r.Show(chat, mid, "❌ اکانت پیدا نشد.", HomeKB())
		return
	}
	r.Show(chat, mid, fmt.Sprintf("⚠️ اکانت «%s» حذف شود؟", Esc(strv(accs[i]["name"]))),
		[][]Btn{{btn("✅ بله", fmt.Sprintf("hzdy:%s", idx)), btn("❌ انصراف", "hzdel")}})
}

func (r *Router) hzDelDo(chat, mid int64, idx string) {
	accs := r.hzAccs()
	i := atoiSafe(idx)
	if i < 0 || i >= len(accs) {
		r.Show(chat, mid, "❌ اکانت پیدا نشد.", HomeKB())
		return
	}
	r.hzFlushIndex(i)
	accs = append(accs[:i], accs[i+1:]...)
	r.hzSaveAccs(accs)
	r.Bot.Send(chat, "✅ اکانت حذف شد.", nil)
	r.hzHome(chat, 0)
}

func (r *Router) hzServers(chat, mid int64, i int, pg int) {
	accs := r.hzAccs()
	if i < 0 || i >= len(accs) {
		r.Show(chat, mid, "❌ اکانت پیدا نشد.", HomeKB())
		return
	}
	servers := r.hzServersCached(i)
	per := 10
	pages := (len(servers) + per - 1) / per
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
	if hi > len(servers) {
		hi = len(servers)
	}
	var items []Btn
	for _, s := range servers[lo:hi] {
		name := strv(s["name"])
		if ip := hzIP(s); ip != "" {
			name += " · " + ip
		}
		items = append(items, btnS(hz.StatusEmoji(strv(s["status"]))+" "+truncRune(name, 24),
			fmt.Sprintf("hzsi:%d:%s", i, strv(s["id"])), "plain"))
	}
	kb := gridBTN(items, 2)
	if pages > 1 {
		prow := []Btn{}
		if pg > 0 {
			prow = append(prow, btn("◀️", fmt.Sprintf("hzs:%d:%d", i, pg-1)))
		} else {
			prow = append(prow, emptyBtn())
		}
		prow = append(prow, btn(fmt.Sprintf("📄 %d/%d", pg+1, pages), "noop"))
		if pg < pages-1 {
			prow = append(prow, btn("▶️", fmt.Sprintf("hzs:%d:%d", i, pg+1)))
		} else {
			prow = append(prow, emptyBtn())
		}
		kb = append(kb, prow)
	}
	kb = append(kb, navRow("hz"))
	r.Show(chat, mid, fmt.Sprintf("🖥 سرورهای «%s» (%d):", Esc(strv(accs[i]["name"])), len(servers)), kb)
}

func (r *Router) hzInfo(chat, mid int64, i int, sid string) {
	accs := r.hzAccs()
	if i < 0 || i >= len(accs) {
		r.Show(chat, mid, "❌ اکانت پیدا نشد.", HomeKB())
		return
	}
	cli := hz.New(strv(accs[i]["token"]))
	d, err := cli.Call("/servers/"+sid, "GET", nil)
	if err != nil {
		r.Show(chat, mid, "❌ "+Esc(err.Error()), HomeKB())
		return
	}
	s, _ := d["server"].(map[string]any)
	if s == nil {
		r.Show(chat, mid, "❌ سرور پیدا نشد.", HomeKB())
		return
	}
	ib := hz.GB(floatAny(s["ingoing_traffic"]))
	ob := hz.GB(floatAny(s["outgoing_traffic"]))
	total := float64(int64((ib+ob)*1000+0.5)) / 1000
	incl := hz.GB(floatAny(s["included_traffic"]))
	used := "—"
	bill := 0.0
	if incl != 0 {
		used = hzF(float64(int64(ob/incl*1000+0.5))/10) + "%"
		bill = float64(int64(maxf(total-incl, 0)*1000+0.5)) / 1000
	}
	euro := r.hzEuroCached()
	st, _ := s["server_type"].(map[string]any)
	location, _ := s["location"].(map[string]any)
	locName := strv(location["name"])
	var p map[string]any
	// The server object embeds server_type without prices; fetch them.
	if stid := strv(st["id"]); stid != "" {
		for _, t2 := range r.hzTypesCached(cli, i) {
			if strv(t2["id"]) != stid {
				continue
			}
			for _, x := range priceList(t2) {
				if strv(x["location"]) == locName {
					p = x
					break
				}
			}
			if p == nil {
				if ps := priceList(t2); len(ps) > 0 {
					p = ps[0]
				}
			}
			break
		}
	}
	hourly := hz.PriceFmt(priceGross(p, "price_hourly"), euro)
	monthly := hz.PriceFmt(priceGross(p, "price_monthly"), euro)
	var snapList []map[string]any
	for _, im := range r.hzImagesCached(cli, i) {
		if strv(im["type"]) == "snapshot" {
			snapList = append(snapList, im)
		}
	}
	snaps := fmt.Sprint(hz.SnapCount(snapList, sid))
	locs := "—"
	if location != nil {
		locs = fmt.Sprintf("%s, %s", orDash(strv(location["country"])), orDash(strv(location["city"])))
	}
	pn, _ := s["public_net"].(map[string]any)
	ip4m, _ := pn["ipv4"].(map[string]any)
	ip6m, _ := pn["ipv6"].(map[string]any)
	ip4, ip6 := "—", "—"
	if ip4m != nil {
		ip4 = orDash(strv(ip4m["ip"]))
	}
	if ip6m != nil {
		ip6 = orDash(strv(ip6m["ip"]))
	}
	img, _ := s["image"].(map[string]any)
	imgName := "—"
	if img != nil {
		if v := strv(img["name"]); v != "" {
			imgName = v
		} else if v := strv(img["description"]); v != "" {
			imgName = v
		}
	}
	created := "—"
	if c := strv(s["created"]); len(c) >= 10 {
		created = c[:10]
	}
	text := fmt.Sprintf("🚀 نام: <code>%s</code> [<code>%s</code>]\n"+
		"🔗 IPv4: <code>%s</code>\n🔗 IPv6: <code>%s</code>\n"+
		"🌍 کشور/شهر: <code>%s</code>\n"+
		"⚙️ CPU: <code>%s Core</code>\n💾 RAM: <code>%s GB</code>\n💿 Disk: <code>%s GB</code>\n"+
		"📸 اسنپ‌شات: <code>%s</code>\n🖼️ تصویر: <code>%s</code>\n"+
		"📊 ترافیک:\n • ورودی: <code>%s GB</code>\n • خروجی: <code>%s GB</code>\n"+
		" • کل: <code>%s GB</code>\n • سهمیه: <code>%s GB</code>\n"+
		" • مصرف: <code>%s [Out/Included]</code>\n • مازاد: <code>%s GB</code>\n"+
		"💰 قیمت:\n • ساعتی: <code>%s</code>\n • ماهانه: <code>%s</code>\n"+
		"📅 ساخته‌شده: <code>%s [%s روز پیش]</code>",
		Esc(strv(s["name"])), Esc(strv(s["status"])), Esc(ip4), Esc(ip6), Esc(locs),
		Esc(orDash(strvVal(st["cores"]))), Esc(orDash(strvVal(st["memory"]))), Esc(orDash(strvVal(st["disk"]))),
		Esc(snaps), Esc(imgName), hzF(ib), hzF(ob), hzF(total), hzF(incl), Esc(used), hzF(bill),
		Esc(hourly), Esc(monthly), Esc(created), Esc(hz.AgeDays(strv(s["created"]))))
	kb := gridBTN([]Btn{
		btn("⚡ روشن", fmt.Sprintf("hzsu:%d:%s:on", i, sid)),
		btn("🔌 خاموش", fmt.Sprintf("hzsu:%d:%s:off", i, sid)),
		btn("🔄 ریبوت", fmt.Sprintf("hzsu:%d:%s:reboot", i, sid)),
		btn("🔄 ریست", fmt.Sprintf("hzsu:%d:%s:reset", i, sid)),
		btn("🔓 ریست رمز", fmt.Sprintf("hzsu:%d:%s:pass", i, sid)),
		btn("✏️ تغییر نام", fmt.Sprintf("hzsu:%d:%s:rename", i, sid)),
		btn("🛠️ ریبیلد", fmt.Sprintf("hzsu:%d:%s:rebuild", i, sid)),
		btn("⬆️ ارتقا", fmt.Sprintf("hzsu:%d:%s:upgrade", i, sid)),
		btn("📷 ساخت اسنپ‌شات", fmt.Sprintf("hzsu:%d:%s:snap", i, sid)),
		btn("🗑 حذف اسنپ‌شات", fmt.Sprintf("hzsu:%d:%s:dsnap", i, sid)),
		btn("🔗 اختصاص IPv4", fmt.Sprintf("hzsu:%d:%s:a4", i, sid)),
		btn("🔗 اختصاص IPv6", fmt.Sprintf("hzsu:%d:%s:a6", i, sid)),
		btn("❌ حذف IPv4", fmt.Sprintf("hzsu:%d:%s:u4", i, sid)),
		btn("❌ حذف IPv6", fmt.Sprintf("hzsu:%d:%s:u6", i, sid)),
		btn("🔄 جایگزینی IPv4", fmt.Sprintf("hzsu:%d:%s:r4", i, sid)),
	}, 2)
	kb = append(kb, []Btn{btn("🔄 بروزرسانی", fmt.Sprintf("hzsi:%d:%s", i, sid))})
	kb = append(kb, []Btn{btnS("🗑 حذف سرور", fmt.Sprintf("hzsu:%d:%s:del", i, sid), "danger")})
	kb = append(kb, []Btn{btn("🔙 بازگشت", "back:"+fmt.Sprintf("hzs:%d:0", i)), btn("🏠 خانه", "menu")})
	r.Show(chat, mid, text, kb)
}

func priceGross(p map[string]any, key string) any {
	if p == nil {
		return nil
	}
	m, _ := p[key].(map[string]any)
	if m == nil {
		return nil
	}
	return m["gross"]
}

func floatAny(v any) float64 {
	switch t := v.(type) {
	case float64:
		return t
	case int:
		return float64(t)
	case string:
		f, _ := strconv.ParseFloat(t, 64)
		return f
	}
	return 0
}

// eur0 formats a Hetzner price as a whole euro amount (no decimals).
func eur0(v any) string {
	f := floatAny(v)
	if f <= 0 {
		return "?"
	}
	return strconv.Itoa(int(math.Round(f))) + "€"
}

func strvVal(v any) string {
	switch t := v.(type) {
	case float64:
		return strconv.FormatFloat(t, 'f', -1, 64)
	case int:
		return strconv.Itoa(t)
	case string:
		return t
	}
	return ""
}

func (r *Router) hzAllImages(cli *hz.Client) []map[string]any {
	var out []map[string]any
	seen := map[string]bool{}
	add := func(path string) {
		for _, im := range cli.All(path) {
			id := strv(im["id"])
			if id == "" || seen[id] {
				continue
			}
			seen[id] = true
			out = append(out, im)
		}
	}
	// Hetzner /images returns only system+app by default; snapshots/backups
	// must be requested explicitly, otherwise they never show up.
	add("/images")
	add("/images?type=snapshot")
	add("/images?type=backup")
	return out
}

func (r *Router) hzAllTypes(cli *hz.Client) []map[string]any {
	return cli.All("/server_types")
}

func intOfAny(v any) int {
	switch t := v.(type) {
	case float64:
		return int(t)
	case int:
		return t
	}
	return 0
}

func sortByTypeSize(up []map[string]any) {
	for i := 1; i < len(up); i++ {
		for j := i; j > 0; j-- {
			a, b := up[j-1], up[j]
			if intOfAny(a["memory"]) > intOfAny(b["memory"]) ||
				(intOfAny(a["memory"]) == intOfAny(b["memory"]) && intOfAny(a["cores"]) > intOfAny(b["cores"])) ||
				(intOfAny(a["memory"]) == intOfAny(b["memory"]) && intOfAny(a["cores"]) == intOfAny(b["cores"]) && intOfAny(a["disk"]) > intOfAny(b["disk"])) {
				up[j-1], up[j] = up[j], up[j-1]
			} else {
				break
			}
		}
	}
}

func (r *Router) hzPickPlan(chat, mid int64, i int, loc string) {
	accs := r.hzAccs()
	if i < 0 || i >= len(accs) {
		r.Show(chat, mid, "❌ اکانت پیدا نشد.", HomeKB())
		return
	}
	pend := r.getWizard(chat)
	if pend == nil || strv(pend["kind"]) != "hz_srv_remark" {
		r.Show(chat, mid, "⏳ عملیات منقضی شده.", HomeKB())
		return
	}
	pend["location"] = loc
	delete(pend, "planId")
	delete(pend, "image")
	delete(pend, "imageName")
	delete(pend, "arch")
	r.setWizard(chat, pend)
	r.hzImageList(chat, mid, i)
}

// hzImageList is the first step after the location: pick the image, then only
// plans compatible with its architecture (x86/ARM) are shown.
func (r *Router) hzImageList(chat, mid int64, i int) {
	accs := r.hzAccs()
	if i < 0 || i >= len(accs) {
		r.Show(chat, mid, "❌ اکانت پیدا نشد.", HomeKB())
		return
	}
	r.loading(chat, mid, "در حال بارگذاری تصاویر")
	imgs := r.hzImagesCached(hz.New(strv(accs[i]["token"])), i)
	var avail []map[string]any
	for _, im := range imgs {
		t := strv(im["type"])
		if t == "system" || t == "snapshot" {
			avail = append(avail, im)
		}
	}
	if len(avail) == 0 {
		r.Show(chat, mid, "❌ تصویری پیدا نشد.", HomeKB())
		return
	}
	var rows []Btn
	for j, im := range avail {
		if j >= 50 {
			break
		}
		label := strv(im["name"])
		if label == "" {
			label = strv(im["description"])
		}
		if label == "" {
			label = strv(im["id"])
		}
		if strv(im["type"]) == "snapshot" {
			label = "💾 " + label
		}
		switch strv(im["architecture"]) {
		case "arm":
			label += " · ARM"
		case "x86":
			label += " · x86"
		}
		rows = append(rows, btnS(label, fmt.Sprintf("hzsci:%d:%s", i, strv(im["id"])), "plain"))
	}
	kb := gridBTN(rows, 2)
	kb = append(kb, row(btn("🔙 بازگشت", "back:hz"), btn("🏠 خانه", "menu")))
	r.Show(chat, mid, "🖼️ تصویر (سیستم‌عامل) را انتخاب کنید:\n\n💡 بعد از انتخاب تصویر، فقط پلن‌های سازگار با معماری آن (x86/ARM) نمایش داده می‌شوند.", kb)
}

// hzPickPlanForImage stores the chosen image and shows the plan list.
func (r *Router) hzPickPlanForImage(chat, mid int64, i int, img string) {
	accs := r.hzAccs()
	if i < 0 || i >= len(accs) {
		r.Show(chat, mid, "❌ اکانت پیدا نشد.", HomeKB())
		return
	}
	pend := r.getWizard(chat)
	if pend == nil || strv(pend["kind"]) != "hz_srv_remark" || strv(pend["location"]) == "" {
		r.Show(chat, mid, "⏳ عملیات منقضی شده.", HomeKB())
		return
	}
	cli := hz.New(strv(accs[i]["token"]))
	arch, name := "", ""
	for _, im := range r.hzImagesCached(cli, i) {
		if strv(im["id"]) == img {
			arch = strv(im["architecture"])
			name = strv(im["name"])
			if name == "" {
				name = strv(im["description"])
			}
			break
		}
	}
	pend["image"] = img
	pend["imageName"] = name
	pend["arch"] = arch
	r.setWizard(chat, pend)
	r.hzPlanList(chat, mid, i, "", "")
}

// hzUsage returns the account's current server usage by core type.
func hzUsageOf(servers []map[string]any) (shared, dedicated, count int) {
	for _, s := range servers {
		count++
		st, _ := s["server_type"].(map[string]any)
		if st == nil {
			continue
		}
		c := intOfAny(st["cores"])
		if strings.ToLower(strv(st["cpu_type"])) == "dedicated" {
			dedicated += c
		} else {
			shared += c
		}
	}
	return
}

func (r *Router) hzUsage(cli *hz.Client) (shared, dedicated, count int) {
	for _, s := range cli.All("/servers") {
		count++
		st, _ := s["server_type"].(map[string]any)
		if st == nil {
			continue
		}
		c := intOfAny(st["cores"])
		if strings.ToLower(strv(st["cpu_type"])) == "dedicated" {
			dedicated += c
		} else {
			shared += c
		}
	}
	return
}

// hzCapGet returns the learned capacity failures for an account.
func (r *Router) hzCapGet(accName string) map[string]any {
	var m map[string]any
	if !r.Store.GetJSON("hzcap:"+accName, &m) || m == nil {
		m = map[string]any{}
	}
	return m
}

func (r *Router) hzCapSet(accName, kind string, need, used int) {
	m := r.hzCapGet(accName)
	m[kind] = map[string]any{"need": need, "used": used, "ts": time.Now().Unix()}
	if b, err := json.Marshal(m); err == nil {
		r.Store.Put("hzcap:"+accName, string(b), 0)
	}
}

// hzBlocked reports whether a plan is known to exceed the account limits.
func hzBlocked(t map[string]any, caps map[string]any, shared, dedicated, count int) (bool, string) {
	kind := "shared"
	usedNow := shared
	if strings.ToLower(strv(t["cpu_type"])) == "dedicated" {
		kind, usedNow = "dedicated", dedicated
	}
	cores := intOfAny(t["cores"])
	// Learned from a previous resource_limit_exceeded on this account.
	if m, ok := caps[kind].(map[string]any); ok {
		need, u0 := intOfAny(m["need"]), intOfAny(m["used"])
		ts := int64(0)
		switch v := m["ts"].(type) {
		case float64:
			ts = int64(v)
		case int64:
			ts = v
		}
		if need > 0 && (ts == 0 || time.Now().Unix()-ts < 12*3600) {
			// At failure: u0+need > limit. A plan fits only if
			// usedNow+cores <= limit < u0+need.
			if threshold := need + u0 - usedNow; threshold > 0 && cores >= threshold {
				label := "ظرفیت هسته‌های shared اکانت پر است"
				if kind == "dedicated" {
					label = "ظرفیت هسته‌های dedicated اکانت پر است"
				}
				return true, label
			}
		}
	}
	if m, ok := caps["server"].(map[string]any); ok {
		ts := int64(0)
		switch v := m["ts"].(type) {
		case float64:
			ts = int64(v)
		case int64:
			ts = v
		}
		if (ts == 0 || time.Now().Unix()-ts < 12*3600) && count >= intOfAny(m["used"]) {
			return true, "سقف تعداد سرورهای اکانت پر است"
		}
	}
	return false, ""
}

// hzPlanList renders the plan list for the pending location+image, marking
// plans that are known not to fit the account limits. It edits the current
// message (no navigation) and can show a notice plus a one-tap retry.
