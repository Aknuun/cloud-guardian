package router

import (
	"fmt"
	"math/rand"
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
	servers, err := hz.New(strv(accs[sel]["token"])).Call("/servers", "GET", nil)
	if err != nil {
		r.Show(chat, mid, "❌ "+Esc(err.Error()), [][]Btn{{btn("🏠 خانه", "menu")}})
		return
	}
	slist := hzList(servers, "servers")
	lines := []string{fmt.Sprintf("🇩🇪 هتزنر — «%s»", Esc(strv(accs[sel]["name"]))), ""}
	if len(slist) == 0 {
		lines = append(lines, "📭 سروری نیست.")
	}
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
		kb = append(kb, []Btn{
			btn("🌐 "+ip, fmt.Sprintf("hzsi:%d:%s", sel, strv(s["id"]))),
			btn(hz.StatusEmoji(strv(s["status"]))+" "+truncRune(label, 28), fmt.Sprintf("hzsi:%d:%s", sel, strv(s["id"])))})
	}
	kb = append(kb, []Btn{btn("🖥 سرورها", fmt.Sprintf("hzs:%d:0", sel)), btn("👤 اکانت‌ها", "hzaccs")})
	createLabel := "➕ ساخت سرور"
	if multi {
		createLabel = "➕ ساخت سرور در " + nm
	}
	kb = append(kb, []Btn{btn(createLabel, fmt.Sprintf("hzsc:%d", sel))})
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
	kb = append(kb, []Btn{btn("🔙 بازگشت", "hz"), btn("🏠 خانه", "menu")})
	r.Show(chat, mid, "👤 اکانت‌های هتزنر:", kb)
}

func (r *Router) hzDelList(chat, mid int64) {
	accs := r.hzAccs()
	if len(accs) == 0 {
		r.Show(chat, mid, "📭 اکانتی نیست.", [][]Btn{{btn("🔙 بازگشت", "hz")}})
		return
	}
	var items []Btn
	for i, a := range accs {
		items = append(items, btn("🗑 "+strv(a["name"]), fmt.Sprintf("hzd:%d", i)))
	}
	kb := gridBTN(items, 2)
	kb = append(kb, row(btn("🔙 بازگشت", "hz")))
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
	d, err := hz.New(strv(accs[i]["token"])).Call("/servers", "GET", nil)
	if err != nil {
		r.Show(chat, mid, "❌ "+Esc(err.Error()), HomeKB())
		return
	}
	servers := hzList(d, "servers")
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
	var kb [][]Btn
	for _, s := range servers[lo:hi] {
		kb = append(kb, []Btn{btn(hz.StatusEmoji(strv(s["status"]))+" "+strv(s["name"]),
			fmt.Sprintf("hzsi:%d:%s", i, strv(s["id"])))})
	}
	if pages > 1 {
		kb = append(kb, []Btn{btn(fmt.Sprintf("%d/%d", pg+1, pages),
			fmt.Sprintf("hzs:%d:%d", i, (pg+1)%pages))})
	}
	kb = append(kb, []Btn{btn("🔙 بازگشت", "hz"), btn("🏠 خانه", "menu")})
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
	euro := hz.EuroRate()
	st, _ := s["server_type"].(map[string]any)
	var p map[string]any
	if prices, ok := st["prices"].([]any); ok && len(prices) > 0 {
		p, _ = prices[0].(map[string]any)
	}
	hourly := hz.PriceFmt(priceGross(p, "price_hourly"), euro)
	monthly := hz.PriceFmt(priceGross(p, "price_monthly"), euro)
	snaps := "—"
	if list := r.hzAllImages(cli); len(list) > 0 {
		snaps = fmt.Sprint(hz.SnapCount(list, sid))
	}
	loc, _ := s["datacenter"].(map[string]any)
	var location map[string]any
	if loc != nil {
		location, _ = loc["location"].(map[string]any)
	}
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
	kb = append(kb, []Btn{btn("🔄 بروزرسانی", fmt.Sprintf("hzsi:%d:%s", i, sid)),
		btn("🗑 حذف سرور", fmt.Sprintf("hzsu:%d:%s:del", i, sid))})
	kb = append(kb, []Btn{btn("🔙 بازگشت", fmt.Sprintf("hzs:%d:0", i)), btn("🏠 خانه", "menu")})
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

func (r *Router) hzOp(chat, mid int64, i int, sid, step string) {
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
	back := fmt.Sprintf("hzsi:%d:%s", i, sid)
	home := []Btn{btn("🔙 بازگشت", back), btn("🏠 خانه", "menu")}
	if step == "rename" {
		r.setWizard(chat, map[string]any{"kind": "hz_srv_rename", "acc": i, "sid": sid})
		r.Show(chat, mid, "✏️ نام جدید سرور را بفرستید:", [][]Btn{{btn("🔙 انصراف", back)}})
		return
	}
	if step == "rebuild" {
		imgs := r.hzAllImages(cli)
		arch := ""
		if im, ok := s["image"].(map[string]any); ok {
			arch = strv(im["architecture"])
		}
		var avail []map[string]any
		for _, im := range imgs {
			t := strv(im["type"])
			if (t == "system" || t == "snapshot") && (arch == "" || strv(im["architecture"]) == arch) {
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
			rows = append(rows, btn(label, fmt.Sprintf("hzsv:%d:%s:rebuild:%s", i, sid, strv(im["id"]))))
		}
		kb := gridBTN(rows, 2)
		kb = append(kb, home)
		r.Show(chat, mid, "🛠️ ریبیلد\n⚠️ همهٔ داده‌های سرور پاک می‌شود.\n\nتصویر جدید را انتخاب کنید:", kb)
		return
	}
	if step == "upgrade" {
		types := r.hzAllTypes(cli)
		cur, _ := s["server_type"].(map[string]any)
		var up []map[string]any
		for _, t := range types {
			if strv(t["architecture"]) != strv(cur["architecture"]) || strv(t["id"]) == strv(cur["id"]) {
				continue
			}
			if intOfAny(t["memory"]) >= intOfAny(cur["memory"]) ||
				intOfAny(t["cores"]) >= intOfAny(cur["cores"]) ||
				intOfAny(t["disk"]) >= intOfAny(cur["disk"]) {
				up = append(up, t)
			}
		}
		if len(up) == 0 {
			r.Show(chat, mid, "❌ پلن ارتقای بالاتری موجود نیست.", HomeKB())
			return
		}
		sortByTypeSize(up)
		var rows []Btn
		for j, t := range up {
			if j >= 40 {
				break
			}
			rows = append(rows, btn(fmt.Sprintf("%dC/%dG", intOfAny(t["cores"]), intOfAny(t["memory"])), fmt.Sprintf("hzsv:%d:%s:upgrade:%s", i, sid, strv(t["id"]))))
		}
		kb := gridBTN(rows, 2)
		kb = append(kb, home)
		warn := ""
		if strv(s["status"]) != "off" {
			warn = "⚠️ برای ارتقا اول سرور را خاموش کن.\n"
		}
		r.Show(chat, mid, "⬆️ ارتقا\n"+warn+"پلن جدید را انتخاب کنید:", kb)
		return
	}
	if step == "a4" || step == "a6" {
		typ := "ipv4"
		if step == "a6" {
			typ = "ipv6"
		}
		pn, _ := s["public_net"].(map[string]any)
		if pn != nil && pn[typ] != nil {
			r.Show(chat, mid, fmt.Sprintf("⚠️ این سرور از قبل %s اختصاصی دارد؛ اول آن را حذف کن.", typ), HomeKB())
			return
		}
		ips, _ := cli.Call("/primary_ips", "GET", nil)
		list := hzList(ips, "primary_ips")
		var avail []map[string]any
		for _, ip := range list {
			if ip["assignee_id"] == nil && strv(ip["type"]) == typ {
				avail = append(avail, ip)
			}
		}
		if len(avail) == 0 {
			r.Show(chat, mid, fmt.Sprintf("❌ آی‌پی %s خالی پیدا نشد.", typ), HomeKB())
			return
		}
		var rows []Btn
		for j, ip := range avail {
			if j >= 40 {
				break
			}
			label := strings.TrimSpace(strv(ip["name"]) + " " + strv(ip["ip"]))
			rows = append(rows, btn(label, fmt.Sprintf("hzsv:%d:%s:%s:%s", i, sid, step, strv(ip["id"]))))
		}
		kb := gridBTN(rows, 2)
		kb = append(kb, home)
		r.Show(chat, mid, fmt.Sprintf("🔗 آی‌پی %s برای اختصاص (سرور خاموش می‌شود):", strings.ToUpper(typ)), kb)
		return
	}
	if step == "dsnap" {
		imgs := r.hzAllImages(cli)
		sidInt, _ := strconv.Atoi(sid)
		var snaps []map[string]any
		for _, im := range imgs {
			if strv(im["type"]) != "snapshot" {
				continue
			}
			if cf, ok := im["created_from"].(map[string]any); ok && intOfAny(cf["id"]) == sidInt {
				snaps = append(snaps, im)
			}
		}
		if len(snaps) == 0 {
			r.Show(chat, mid, "❌ اسنپ‌شاتی برای این سرور پیدا نشد.", HomeKB())
			return
		}
		var rows []Btn
		for j, im := range snaps {
			if j >= 40 {
				break
			}
			label := strv(im["name"])
			if label == "" {
				label = strv(im["description"])
			}
			if label == "" {
				label = strv(im["id"])
			}
			rows = append(rows, btn(label, fmt.Sprintf("hzsv:%d:%s:dsnap:%s", i, sid, strv(im["id"]))))
		}
		kb := gridBTN(rows, 2)
		kb = append(kb, home)
		r.Show(chat, mid, "🗑 اسنپ‌شات برای حذف:", kb)
		return
	}
	labels := map[string]string{"on": "روشن کردن", "off": "خاموش کردن", "reset": "ریست شدید",
		"reboot": "ریبوت", "pass": "ریست رمز", "del": "حذف سرور",
		"snap": "ساخت اسنپ‌شات", "u4": "حذف اختصاص IPv4", "u6": "حذف اختصاص IPv6",
		"r4": "جایگزینی IPv4 با یکی تازه (قدیمی حذف و سرور دوباره روشن می‌شود)"}
	label, ok := labels[step]
	if !ok {
		r.Show(chat, mid, "❓ عملیات ناشناخته.", HomeKB())
		return
	}
	r.Show(chat, mid, fmt.Sprintf("⚠️ مطمئنید «%s» روی «%s» انجام شود؟", label, strv(s["name"])),
		[][]Btn{{btn("✅ بله", fmt.Sprintf("hzok:%d:%s:%s", i, sid, step)), btn("❌ انصراف", back)}})
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
	types := r.hzAllTypes(hz.New(strv(accs[i]["token"])))
	var avail []map[string]any
	for _, t := range types {
		if boolVal(t["deprecated"]) {
			continue
		}
		for _, p := range priceList(t) {
			if strv(p["location"]) == loc {
				avail = append(avail, t)
				break
			}
		}
	}
	sortPlanByPrice(avail, loc)
	pend["location"] = loc
	r.setWizard(chat, pend)
	famLabels := map[string]string{"cpx": "🧩 CPX · پردازنده AMD", "cx": "🧩 CX · پردازنده Intel",
		"cax": "🧩 CAX · پردازنده Ampere ARM", "ccx": "🧩 CCX · اختصاصی (Dedicated)",
		"other": "🧩 سایر پلن‌ها"}
	groups := map[string][]map[string]any{}
	var order []string
	for _, t := range avail {
		if len(order) >= 60 {
			break
		}
		nm := strings.ToLower(strv(t["name"]))
		fam := "other"
		for _, f := range []string{"cpx", "cx", "cax", "ccx"} {
			if strings.HasPrefix(nm, f) {
				fam = f
				break
			}
		}
		groups[fam] = append(groups[fam], t)
	}
	var kb [][]Btn
	for _, fam := range []string{"cpx", "cx", "cax", "ccx", "other"} {
		arr := groups[fam]
		if len(arr) == 0 {
			continue
		}
		kb = append(kb, []Btn{{Text: famLabels[fam], Data: "noop", Style: ""}})
		var rows []Btn
		for _, t := range arr {
			eur := "?"
			for _, p := range priceList(t) {
				if strv(p["location"]) == loc {
					if pm, ok := p["price_monthly"].(map[string]any); ok && pm["gross"] != nil {
						eur = fmt.Sprint(pm["gross"])
					}
					break
				}
			}
			rows = append(rows, btn(fmt.Sprintf("%dC/%dG · %s€", intOfAny(t["cores"]), intOfAny(t["memory"]), eur), fmt.Sprintf("hzscp:%d:%s", i, strv(t["id"]))))
		}
		kb = append(kb, gridBTN(rows, 2)...)
	}
	kb = append(kb, []Btn{btn("🔙 بازگشت", "hz"), btn("🏠 خانه", "menu")})
	r.Show(chat, mid, fmt.Sprintf("💰 پلن سرور را انتخاب کن (لوکیشن: %s):", loc), kb)
}

func priceList(t map[string]any) []map[string]any {
	arr, _ := t["prices"].([]any)
	var out []map[string]any
	for _, x := range arr {
		if m, ok := x.(map[string]any); ok {
			out = append(out, m)
		}
	}
	return out
}

func sortPlanByPrice(avail []map[string]any, loc string) {
	price := func(t map[string]any) float64 {
		for _, p := range priceList(t) {
			if strv(p["location"]) == loc {
				if pm, ok := p["price_monthly"].(map[string]any); ok {
					if n, ok := pm["net"].(float64); ok && n > 0 {
						return n
					}
				}
			}
		}
		return 1e9
	}
	for i := 1; i < len(avail); i++ {
		for j := i; j > 0 && price(avail[j-1]) > price(avail[j]); j-- {
			avail[j-1], avail[j] = avail[j], avail[j-1]
		}
	}
}

func (r *Router) hzPickImage(chat, mid int64, i int, plan string) {
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
	cli := hz.New(strv(accs[i]["token"]))
	td, err := cli.Call("/server_types/"+plan, "GET", nil)
	if err != nil {
		r.Show(chat, mid, "❌ "+Esc(err.Error()), HomeKB())
		return
	}
	stype, _ := td["server_type"].(map[string]any)
	imgs := r.hzAllImages(cli)
	arch := ""
	if stype != nil {
		arch = strv(stype["architecture"])
	}
	var avail []map[string]any
	for _, im := range imgs {
		t := strv(im["type"])
		if (t == "system" || t == "snapshot") && (arch == "" || strv(im["architecture"]) == arch) {
			avail = append(avail, im)
		}
	}
	if len(avail) == 0 {
		r.Show(chat, mid, "❌ تصویری پیدا نشد.", HomeKB())
		return
	}
	pend["planId"] = plan
	r.setWizard(chat, pend)
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
		rows = append(rows, btn(label, fmt.Sprintf("hzsci:%d:%s", i, strv(im["id"]))))
	}
	kb := gridBTN(rows, 2)
	kb = append(kb, []Btn{btn("🔙 بازگشت", "hz"), btn("🏠 خانه", "menu")})
	r.Show(chat, mid, "🖼️ تصویر (سیستم‌عامل) را انتخاب کنید:", kb)
}

const randChars = "abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ0123456789"

func (r *Router) hzCreate(chat, mid int64, i int, img string) {
	accs := r.hzAccs()
	if i < 0 || i >= len(accs) {
		r.Show(chat, mid, "❌ اکانت پیدا نشد.", HomeKB())
		return
	}
	pend := r.getWizard(chat)
	if pend == nil || strv(pend["kind"]) != "hz_srv_remark" || pend["location"] == nil || pend["planId"] == nil {
		r.Show(chat, mid, "⏳ عملیات منقضی شده.", HomeKB())
		return
	}
	b := make([]byte, 16)
	for k := range b {
		b[k] = randChars[rand.Intn(len(randChars))]
	}
	pw := string(b)
	cloudInit := "#cloud-config\nssh_pwauth: true\nchpasswd:\n  list: |\n    root:" + pw + "\n  expire: False\n"
	resp, err := hz.New(strv(accs[i]["token"])).Call("/servers", "POST", map[string]any{
		"name":               strv(pend["remark"]),
		"server_type":        strv(pend["planId"]),
		"image":              img,
		"location":           strv(pend["location"]),
		"start_after_create": true,
		"user_data":          cloudInit,
	})
	if err != nil {
		loc := strv(pend["location"])
		kb := [][]Btn{
			{btn("🔁 یک پلن دیگر امتحان کن", fmt.Sprintf("hzsl:%d:%s", i, loc))},
			{btn("🏠 خانه", "menu")},
		}
		r.Show(chat, mid, "❌ ساخت سرور انجام نشد.\n\n🔎 علت: "+Esc(err.Error())+
			"\n\n👇 یک پلن دیگر انتخاب کن (پیام قبلی جایگزین می‌شود):", kb)
		return
	}
	r.clearWizard(chat)
	hs, _ := resp["server"].(map[string]any)
	ip := ""
	if hs != nil {
		pn, _ := hs["public_net"].(map[string]any)
		ip4, _ := pn["ipv4"].(map[string]any)
		ip = strv(ip4["ip"])
	}
	if ip != "" {
		lst := r.srvList()
		name := strv(pend["remark"])
		if hs != nil {
			if n := strv(hs["name"]); n != "" {
				name = n
			}
		}
		lst = append(lst, srvRec(fmt.Sprintf("s%d%d", time.Now().Unix(), rand.Intn(90)+10),
			name, ip, "root", 22, "pass", pw, "Hetzner: "+name))
		r.srvSave(lst)
	}
	msg := "🎉 سرور هتزنر ساخته شد.\n🔑 رمز root:\n<code>" + Esc(pw) + "</code>\n\n" +
		"⚠️ آن را امن نگه دار؛ دوباره نمایش داده نمی‌شود."
	if ip != "" {
		msg += "\n🌐 آی‌پی: <code>" + Esc(ip) + "</code>"
	}
	kb := [][]Btn{{copyBtnL("📋 کپی رمز", pw)}}
	if ip != "" {
		kb = append(kb, row(copyBtnL("📋 کپی آی‌پی", ip)))
	}
	kb = append(kb, row(btn("🖥 سرورها", fmt.Sprintf("hzs:%d:0", i))))
	r.Bot.Send(chat, msg, kb)
}

func (r *Router) hzPick(chat, mid int64, i int, sid, step, arg string) {
	accs := r.hzAccs()
	if i < 0 || i >= len(accs) {
		r.Show(chat, mid, "❌ اکانت پیدا نشد.", HomeKB())
		return
	}
	if _, err := hz.New(strv(accs[i]["token"])).Call("/servers/"+sid, "GET", nil); err != nil {
		r.Show(chat, mid, "❌ "+Esc(err.Error()), HomeKB())
		return
	}
	labels := map[string]string{"rebuild": "ریبیلد", "upgrade": "ارتقا",
		"a4": "اختصاص IPv4", "a6": "اختصاص IPv6", "dsnap": "حذف اسنپ‌شات"}
	label := step
	if v, ok := labels[step]; ok {
		label = v
	}
	r.Show(chat, mid, fmt.Sprintf("⚠️ مطمئنید «%s» انجام شود؟", label),
		[][]Btn{{btn("✅ بله", fmt.Sprintf("hzok:%d:%s:%s:%s", i, sid, step, arg)),
			btn("❌ انصراف", fmt.Sprintf("hzsi:%d:%s", i, sid))}})
}

func (r *Router) hzDo(chat, mid int64, i int, sid, step, arg string) {
	accs := r.hzAccs()
	if i < 0 || i >= len(accs) {
		r.Show(chat, mid, "❌ اکانت پیدا نشد.", HomeKB())
		return
	}
	cli := hz.New(strv(accs[i]["token"]))
	back := [][]Btn{{btn("🔙 بازگشت", fmt.Sprintf("hzsi:%d:%s", i, sid)), btn("🏠 خانه", "menu")}}
	fail := func(err error) {
		r.Show(chat, mid, "❌ خطا: "+Esc(err.Error()), back)
	}
	switch step {
	case "del":
		if _, err := cli.Call("/servers/"+sid, "DELETE", nil); err != nil {
			fail(err)
			return
		}
		r.Bot.Send(chat, "✅ سرور حذف شد.", nil)
		r.hzServers(chat, 0, i, 0)
	case "on", "off", "reboot", "reset":
		path := "/servers/" + sid + "/actions/" + step
		if step == "on" || step == "off" {
			path = "/servers/" + sid + "/actions/power" + step
		}
		if _, err := cli.Call(path, "POST", map[string]any{}); err != nil {
			fail(err)
			return
		}
		r.Bot.Send(chat, "✅ دستور ارسال شد.", nil)
		r.hzInfo(chat, 0, i, sid)
	case "pass":
		res, err := cli.Call("/servers/"+sid+"/actions/reset_password", "POST", map[string]any{})
		if err != nil {
			fail(err)
			return
		}
		pw := ""
		if act, ok := res["action"].(map[string]any); ok {
			pw = strv(act["root_password"])
		}
		if pw == "" {
			pw = strv(res["root_password"])
		}
		r.Bot.Send(chat, "🔓 رمز root جدید:\n<code>"+Esc(pw)+"</code>\n\n⚠️ آن را امن نگه دار.", nil)
		r.hzInfo(chat, 0, i, sid)
	case "snap":
		name := fmt.Sprintf("snap-%s-%s", sid, time.Now().Format("20060102-1504"))
		if _, err := cli.Call("/servers/"+sid+"/actions/create_image", "POST",
			map[string]any{"description": name, "type": "snapshot"}); err != nil {
			fail(err)
			return
		}
		r.Bot.Send(chat, fmt.Sprintf("✅ ساخت اسنپ‌شات شروع شد (%s).", Esc(name)), nil)
		r.hzInfo(chat, 0, i, sid)
	case "rebuild":
		if _, err := cli.Call("/servers/"+sid+"/actions/rebuild", "POST", map[string]any{"image": arg}); err != nil {
			fail(err)
			return
		}
		r.Bot.Send(chat, "✅ ریبیلد شروع شد.", nil)
		r.hzInfo(chat, 0, i, sid)
	case "upgrade":
		d, err := cli.Call("/servers/"+sid, "GET", nil)
		if err != nil {
			fail(err)
			return
		}
		s, _ := d["server"].(map[string]any)
		if strv(s["status"]) != "off" {
			r.Show(chat, mid, "⚠️ برای ارتقا اول سرور را خاموش کن.", back)
			return
		}
		if _, err := cli.Call("/servers/"+sid+"/actions/change_type", "POST",
			map[string]any{"server_type": arg, "upgrade_disk": true}); err != nil {
			fail(err)
			return
		}
		r.Bot.Send(chat, "✅ ارتقا شروع شد.", nil)
		r.hzInfo(chat, 0, i, sid)
	case "a4", "a6":
		if !hz.EnsureOff(cli, sid, 30000) {
			r.Show(chat, mid, "⚠️ سرور در ۳۰ ثانیه خاموش نشد؛ دوباره تلاش کن.", back)
			return
		}
		if _, err := cli.Call("/primary_ips/"+arg+"/actions/assign", "POST",
			map[string]any{"assignee_id": atoiSafe(sid), "assignee_type": "server"}); err != nil {
			fail(err)
			return
		}
		r.Bot.Send(chat, "✅ آی‌پی اختصاص یافت.", nil)
		r.hzInfo(chat, 0, i, sid)
	case "u4", "u6":
		d, err := cli.Call("/servers/"+sid, "GET", nil)
		if err != nil {
			fail(err)
			return
		}
		s, _ := d["server"].(map[string]any)
		pn, _ := s["public_net"].(map[string]any)
		key := "ipv4"
		if step == "u6" {
			key = "ipv6"
		}
		ipid := hz.PrimaryIPID(pn[key])
		if ipid == nil {
			r.Show(chat, mid, "ℹ️ آی‌پی اختصاصی ندارد.", back)
			return
		}
		if !hz.EnsureOff(cli, sid, 30000) {
			r.Show(chat, mid, "⚠️ سرور در ۳۰ ثانیه خاموش نشد؛ دوباره تلاش کن.", back)
			return
		}
		if _, err := cli.Call(fmt.Sprintf("/primary_ips/%v/actions/unassign", ipid), "POST", map[string]any{}); err != nil {
			fail(err)
			return
		}
		r.Bot.Send(chat, "✅ اختصاص حذف شد.", nil)
		r.hzInfo(chat, 0, i, sid)
	case "r4":
		d, err := cli.Call("/servers/"+sid, "GET", nil)
		if err != nil {
			fail(err)
			return
		}
		s, _ := d["server"].(map[string]any)
		dc, _ := s["datacenter"].(map[string]any)
		dcName := strv(dc["name"])
		if dcName == "" {
			r.Show(chat, mid, "❌ دیتاسنتر سرور مشخص نیست.", back)
			return
		}
		pn, _ := s["public_net"].(map[string]any)
		old := hz.PrimaryIPID(pn["ipv4"])
		if !hz.EnsureOff(cli, sid, 30000) {
			r.Show(chat, mid, "⚠️ سرور در ۳۰ ثانیه خاموش نشد؛ دوباره تلاش کن.", back)
			return
		}
		c, err := cli.Call("/primary_ips", "POST", map[string]any{
			"type": "ipv4", "name": "replace-" + sid, "datacenter": dcName, "auto_delete": true})
		if err != nil {
			fail(err)
			return
		}
		nip, _ := c["primary_ip"].(map[string]any)
		if nip == nil || nip["id"] == nil {
			r.Show(chat, mid, "❌ ساخت IPv4 تازه ناموفق شد.", back)
			return
		}
		if old != nil {
			_, _ = cli.Call(fmt.Sprintf("/primary_ips/%v/actions/unassign", old), "POST", map[string]any{})
		}
		_, _ = cli.Call(fmt.Sprintf("/primary_ips/%v/actions/assign", nip["id"]), "POST",
			map[string]any{"assignee_id": atoiSafe(sid), "assignee_type": "server"})
		if old != nil {
			_, _ = cli.Call(fmt.Sprintf("/primary_ips/%v", old), "DELETE", nil)
		}
		_, _ = cli.Call("/servers/"+sid+"/actions/poweron", "POST", map[string]any{})
		r.Bot.Send(chat, "✅ جایگزینی انجام شد.\nIPv4 تازه: <code>"+Esc(strv(nip["ip"]))+"</code>", nil)
		r.hzInfo(chat, 0, i, sid)
	case "dsnap":
		if _, err := cli.Call("/images/"+arg, "DELETE", nil); err != nil {
			fail(err)
			return
		}
		r.Bot.Send(chat, "✅ اسنپ‌شات حذف شد.", nil)
		r.hzInfo(chat, 0, i, sid)
	default:
		r.Show(chat, mid, "❓ عملیات ناشناخته.", back)
	}
}

func (r *Router) hzAddName(chat, mid int64, text string) {
	name := strings.ReplaceAll(text, " ", "")
	if name == "" {
		r.Bot.Send(chat, "❌ نام معتبر نیست.", nil)
		return
	}
	r.setWizard(chat, map[string]any{"kind": "hz_add_token", "name": name})
	r.Bot.Send(chat, "🔑 توکن API هتزنر را بفرستید:", nil)
}

func (r *Router) hzAddToken(chat, mid int64, pend map[string]any, text string) {
	tok := strings.TrimSpace(text)
	if _, err := hz.New(tok).Call("/servers?per_page=50", "GET", nil); err != nil {
		r.Bot.Send(chat, "❌ توکن نامعتبر است: "+Esc(err.Error()), nil)
		return
	}
	accs := r.hzAccs()
	name, _ := pend["name"].(string)
	if name == "" {
		name = "hz"
	}
	accs = append(accs, map[string]any{"name": name, "token": tok})
	r.hzSaveAccs(accs)
	r.clearWizard(chat)
	r.Bot.Send(chat, "✅ اکانت هتزنر اضافه شد.", nil)
	r.hzHome(chat, 0)
}

func (r *Router) hzRename(chat, mid int64, pend map[string]any, text string) {
	i := intOrAny(pend["acc"])
	sid, _ := pend["sid"].(string)
	accs := r.hzAccs()
	if i < 0 || i >= len(accs) {
		r.Bot.Send(chat, "❌ اکانت پیدا نشد.", nil)
		return
	}
	if _, err := hz.New(strv(accs[i]["token"])).Call("/servers/"+sid, "PUT",
		map[string]any{"name": strings.TrimSpace(text)}); err != nil {
		r.Bot.Send(chat, "❌ نشد: "+Esc(err.Error()), nil)
		return
	}
	r.Bot.Send(chat, "✅ نام سرور تغییر کرد.", nil)
	r.clearWizard(chat)
	r.hzInfo(chat, 0, i, sid)
}

func (r *Router) hzRemark(chat, mid int64, pend map[string]any, text string) {
	remark := strings.TrimSpace(text)
	if remark == "" || strings.ContainsAny(remark, " \t\n") {
		r.Bot.Send(chat, "⚠️ نام سرور نباید فاصله داشته باشد. دوباره بفرستید:", nil)
		return
	}
	i := intOrAny(pend["acc"])
	accs := r.hzAccs()
	if i < 0 || i >= len(accs) {
		r.clearWizard(chat)
		r.Bot.Send(chat, "❌ اکانت پیدا نشد.", nil)
		return
	}
	locs := hz.New(strv(accs[i]["token"])).All("/locations")
	if len(locs) == 0 {
		r.clearWizard(chat)
		r.Bot.Send(chat, "❌ لوکیشنی پیدا نشد.", nil)
		return
	}
	pend["remark"] = remark
	r.setWizard(chat, pend)
	var rows []Btn
	for j, l := range locs {
		if j >= 40 {
			break
		}
		city := strv(l["city"])
		if city == "" {
			city = strv(l["name"])
		}
		rows = append(rows, btn(fmt.Sprintf("%s (%s)", city, strv(l["name"])), fmt.Sprintf("hzsl:%d:%s", i, strv(l["name"]))))
	}
	kb := gridBTN(rows, 2)
	kb = append(kb, []Btn{btn("🔙 انصراف", fmt.Sprintf("hzs:%d:0", i))})
	r.Bot.Send(chat, "🌍 لوکیشن (دیتاسنتر) را انتخاب کنید:", kb)
}

func boolVal(v any) bool {
	switch t := v.(type) {
	case bool:
		return t
	case float64:
		return t != 0
	case string:
		return t == "true" || t == "1"
	}
	return false
}
