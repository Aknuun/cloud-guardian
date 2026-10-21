package router

import (
	"fmt"
	"math"
	"math/rand"
	"net"
	"net/url"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/aknuun/cg-server/internal/ln"
)

func (r *Router) lnAccs() []map[string]any {
	var accs []map[string]any
	if !r.Store.GetJSON("ln_accounts", &accs) {
		return nil
	}
	return accs
}

func (r *Router) lnSaveAccs(accs []map[string]any) {
	r.Store.PutJSON("ln_accounts", accs, 0)
}

func (r *Router) lnSel() int {
	accs := r.lnAccs()
	i, err := strconv.Atoi(strings.TrimSpace(r.Store.Get("ln_sel")))
	if err == nil && i >= 0 && i < len(accs) {
		return i
	}
	return 0
}

// lnDropServers invalidates the cached instance list after a mutation.
func (r *Router) lnDropServers(i int) {
	arvMem.Delete(fmt.Sprintf("mlnl:%d", i))
	r.Store.Delete(fmt.Sprintf("lnsrv:%d", i))
}

// lnFlushIndex drops every cached Linode entry for an account index (used when
// an account is removed and the following ones shift down).
func (r *Router) lnFlushIndex(i int) {
	arvMem.Delete(fmt.Sprintf("mlnl:%d", i))
	r.Store.Delete(fmt.Sprintf("lnsrv:%d", i))
}

func (r *Router) lnServersCached(i int) []map[string]any {
	accs := r.lnAccs()
	if i < 0 || i >= len(accs) {
		return nil
	}
	cli := ln.New(strv(accs[i]["token"]))
	return r.arvCachedMaps(fmt.Sprintf("mlnl:%d", i), fmt.Sprintf("lnsrv:%d", i), 60*time.Second,
		func() []map[string]any { return cli.All("/linode/instances") })
}

func lnStatusEmoji(st string) string {
	switch st {
	case "running":
		return "🟢"
	case "offline":
		return "🔴"
	}
	return "🟡"
}

func lnFirstIP(s map[string]any) string {
	if arr, ok := s["ipv4"].([]any); ok && len(arr) > 0 {
		return strv(arr[0])
	}
	return "—"
}

func lnListText(s map[string]any) string {
	return truncRune(fmt.Sprintf("%s %s (%s)", lnStatusEmoji(strv(s["status"])), strv(s["label"]), lnFirstIP(s)), 30)
}

func num2(f float64) string {
	r := math.Round(f*100) / 100
	return strconv.FormatFloat(r, 'f', -1, 64)
}

func gbBytes(v float64) string {
	return num2(v / (1024 * 1024 * 1024))
}

func (r *Router) lnHome(chat, mid int64) {
	accs := r.lnAccs()
	if len(accs) == 0 {
		r.Show(chat, mid, "🟢 لینود\n\nاکانتی نیست؛ اضافه کن:",
			[][]Btn{{btn("➕ افزودن اکانت", "lna")}, {btn("🏠 خانه", "menu")}})
		return
	}
	sel := r.lnSel()
	var kb [][]Btn
	lines := []string{"🟢 لینود", ""}
	if len(accs) > 1 {
		var ab []Btn
		for i, a := range accs {
			mark := "👤 "
			if i == sel {
				mark = "✅ "
			}
			ab = append(ab, btn(mark+strv(a["name"]), fmt.Sprintf("lnm:%d", i)))
		}
		per := 4
		if len(accs) == 2 {
			per = 2
		} else if len(accs) == 3 {
			per = 3
		}
		kb = append(kb, gridBTN(ab, per)...)
	}
	lines = append(lines, fmt.Sprintf("«%s» انتخاب شده:", strv(accs[sel]["name"])), "")
	servers := r.lnServersCached(sel)
	var srvItems []Btn
	for i, s := range servers {
		if i >= 20 {
			break
		}
		srvItems = append(srvItems, btnS(lnListText(s), fmt.Sprintf("lnsi:%d:%s", sel, strv(s["id"])), "plain"))
	}
	kb = append(kb, gridBTN(srvItems, 2)...)
	nm := strv(accs[sel]["name"])
	label := "⚙️ تنظیمات"
	if len(accs) > 1 {
		label += " (" + nm + ")"
	}
	kb = append(kb, []Btn{btnS("➕ ساخت سرور", fmt.Sprintf("lnsc:%d", sel), "success")})
	kb = append(kb, []Btn{btnS(label, fmt.Sprintf("lnacc:%d", sel), "plain")})
	kb = append(kb, []Btn{btn("🔙 بازگشت", "back:menu"), btn("👤 اکانت‌ها", "lnaccs"), btn("🏠 خانه", "menu")})
	r.Show(chat, mid, strings.Join(lines, "\n"), kb)
}

func (r *Router) lnAccSettings(chat, mid int64, i int) {
	accs := r.lnAccs()
	if i < 0 || i >= len(accs) {
		r.Show(chat, mid, "❌ اکانت پیدا نشد.", HomeKB())
		return
	}
	acc := accs[i]
	tok := strv(acc["token"])
	masked := "—"
	if len(tok) > 14 {
		masked = tok[:8] + "…" + tok[len(tok)-4:]
	}
	r.Show(chat, mid,
		fmt.Sprintf("⚙️ تنظیمات کلاینت\n\n👤 نام: <code>%s</code>\n🔑 توکن: <code>%s</code>",
			Esc(strv(acc["name"])), Esc(masked)),
		[][]Btn{
			{btn("🗑 حذف اکانت", fmt.Sprintf("lnd:%d", i))},
			{btn("🔙 بازگشت", "back:ln"), btn("🏠 خانه", "menu")}})
}

func (r *Router) lnAccsMenu(chat, mid int64) {
	kb := [][]Btn{{btn("➕ افزودن اکانت", "lna")}}
	if len(r.lnAccs()) > 0 {
		kb = append(kb, []Btn{btn("🗑 حذف اکانت", "lndel")})
	}
	kb = append(kb, []Btn{btn("🔙 بازگشت", "back:ln"), btn("🏠 خانه", "menu")})
	r.Show(chat, mid, "👤 اکانت‌های لینود:", kb)
}

func (r *Router) lnDelList(chat, mid int64) {
	accs := r.lnAccs()
	var items []Btn
	for i, a := range accs {
		items = append(items, btnS("🗑 "+strv(a["name"]), fmt.Sprintf("lnd:%d", i), "plain"))
	}
	kb := gridBTN(items, 2)
	kb = append(kb, row(btn("🔙 بازگشت", "back:ln")))
	r.Show(chat, mid, "🗑 کدام اکانت حذف شود؟", kb)
}

func (r *Router) lnDelAsk(chat, mid int64, idx string) {
	accs := r.lnAccs()
	i := atoiSafe(idx)
	if i < 0 || i >= len(accs) {
		r.Show(chat, mid, "❌ اکانت پیدا نشد.", HomeKB())
		return
	}
	r.Show(chat, mid, fmt.Sprintf("⚠️ اکانت «%s» حذف شود؟", Esc(strv(accs[i]["name"]))),
		[][]Btn{{btn("✅ بله", fmt.Sprintf("lndy:%s", idx)), btn("❌ انصراف", "lndel")}})
}

func (r *Router) lnDelDo(chat, mid int64, idx string) {
	accs := r.lnAccs()
	i := atoiSafe(idx)
	if i < 0 || i >= len(accs) {
		r.Show(chat, mid, "❌ اکانت پیدا نشد.", HomeKB())
		return
	}
	accs = append(accs[:i], accs[i+1:]...)
	r.lnSaveAccs(accs)
	r.lnFlushIndex(i)
	r.Bot.Send(chat, "✅ اکانت حذف شد.", nil)
	r.lnHome(chat, 0)
}

func (r *Router) lnServers(chat, mid int64, i int, pg int) {
	accs := r.lnAccs()
	if i < 0 || i >= len(accs) {
		r.Show(chat, mid, "❌ اکانت پیدا نشد.", HomeKB())
		return
	}
	servers := r.lnServersCached(i)
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
		items = append(items, btnS(lnListText(s), fmt.Sprintf("lnsi:%d:%s", i, strv(s["id"])), "plain"))
	}
	kb := gridBTN(items, 2)
	if pages > 1 {
		var nav []Btn
		if pg > 0 {
			nav = append(nav, btn("◀️", fmt.Sprintf("lns:%d:%d", i, pg-1)))
		} else {
			nav = append(nav, emptyBtn())
		}
		nav = append(nav, btn(fmt.Sprintf("📄 %d/%d", pg+1, pages), "noop"))
		if pg < pages-1 {
			nav = append(nav, btn("▶️", fmt.Sprintf("lns:%d:%d", i, pg+1)))
		} else {
			nav = append(nav, emptyBtn())
		}
		kb = append(kb, nav)
	}
	kb = append(kb, []Btn{btn("🔙 بازگشت", "back:ln"), btn("🏠 خانه", "menu")})
	r.Show(chat, mid, fmt.Sprintf("🖥 سرورهای «%s» (%d):", Esc(strv(accs[i]["name"])), len(servers)), kb)
}

func (r *Router) lnInfo(chat, mid int64, i int, sid string) {
	accs := r.lnAccs()
	if i < 0 || i >= len(accs) {
		r.Show(chat, mid, "❌ اکانت پیدا نشد.", HomeKB())
		return
	}
	cli := ln.New(strv(accs[i]["token"]))
	s, err := cli.Call("GET", "/linode/instances/"+sid, nil)
	if err != nil {
		r.Show(chat, mid, "❌ "+Esc(err.Error()), HomeKB())
		return
	}
	if strv(s["id"]) == "" {
		r.Show(chat, mid, "❌ سرور پیدا نشد.", HomeKB())
		return
	}
	specs, _ := s["specs"].(map[string]any)
	mem := "?"
	vcp := "?"
	disk := "?"
	if specs != nil {
		vcp = strv(specs["vcpus"])
		mem = gbBytes(floatVal(specs["memory"]) * 1024 * 1024)
		disk = gbBytes(floatVal(specs["disk"]) * 1024 * 1024)
	}
	used, quota, bill := "—", "—", "—"
	if tr, err := cli.Call("GET", "/linode/instances/"+sid+"/transfer", nil); err == nil {
		if v, ok := nf(tr["used"]); ok {
			used = num2(v / (1024 * 1024 * 1024))
		}
		if v, ok := nf(tr["quota"]); ok {
			quota = num2(v / (1024 * 1024 * 1024))
		}
		if v, ok := nf(tr["billable"]); ok {
			bill = num2(v / (1024 * 1024 * 1024))
		}
	}
	text := fmt.Sprintf("🚀 <code>%s</code> [<code>%s</code>]\n\n"+
		"🔗 IPv4: <code>%s</code>\n🔗 IPv6: <code>%s</code>\n"+
		"🌍 منطقه: <code>%s</code>\n"+
		"⚙️ مشخصات: <code>%s هسته / %sGB رم / %sGB دیسک</code>\n"+
		"🖼️ تصویر: <code>%s</code>\n📊 ترافیک (GB):\n"+
		" • مصرف: <code>%s</code>\n • سهمیه: <code>%s</code>\n • مازاد: <code>%s</code>",
		Esc(strv(s["label"])), Esc(strv(s["status"])),
		Esc(lnFirstIP(s)), Esc(orDash(strv(s["ipv6"]))),
		Esc(strv(s["region"])), Esc(vcp), Esc(mem), Esc(disk),
		Esc(orDash(strv(s["image"]))), Esc(used), Esc(quota), Esc(bill))
	kb := gridBTN([]Btn{
		btn("⚡ روشن", fmt.Sprintf("lnsu:%d:%s:on", i, sid)),
		btn("🔌 خاموش", fmt.Sprintf("lnsu:%d:%s:off", i, sid)),
		btn("🔄 ریبوت", fmt.Sprintf("lnsu:%d:%s:reboot", i, sid)),
		btn("🔄 ریست", fmt.Sprintf("lnsu:%d:%s:reset", i, sid)),
		btn("🔓 ریست رمز", fmt.Sprintf("lnsu:%d:%s:pass", i, sid)),
		btn("✏️ تغییر نام", fmt.Sprintf("lnsu:%d:%s:rename", i, sid)),
		btn("🛠️ ریبیلد", fmt.Sprintf("lnsu:%d:%s:rebuild", i, sid)),
		btn("⬆️ ارتقا", fmt.Sprintf("lnsu:%d:%s:upgrade", i, sid)),
		btn("📷 اسنپ‌شات", fmt.Sprintf("lnsu:%d:%s:snap", i, sid)),
		btn("🗑 حذف اسنپ‌شات", fmt.Sprintf("lnsu:%d:%s:dsnap", i, sid)),
		btn("➕ IPv4", fmt.Sprintf("lnsu:%d:%s:a4", i, sid)),
		btn("➕ IPv6", fmt.Sprintf("lnsu:%d:%s:a6", i, sid)),
		btn("❌ IPv4", fmt.Sprintf("lnsu:%d:%s:u4", i, sid)),
		btn("❌ IPv6", fmt.Sprintf("lnsu:%d:%s:u6", i, sid)),
		btnS("🗑 حذف سرور", fmt.Sprintf("lnsu:%d:%s:del", i, sid), "danger"),
	}, 2)
	kb = append(kb, []Btn{btn("🔙 بازگشت", "back:"+fmt.Sprintf("lns:%d:0", i)), btn("🏠 خانه", "menu")})
	r.Show(chat, mid, text, kb)
}

func orDash(s string) string {
	if s == "" {
		return "—"
	}
	return s
}

func floatVal(v any) float64 {
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

func nf(v any) (float64, bool) {
	switch t := v.(type) {
	case float64:
		return t, true
	case int:
		return float64(t), true
	case string:
		f, err := strconv.ParseFloat(t, 64)
		return f, err == nil
	}
	return 0, false
}

func (r *Router) lnOp(chat, mid int64, i int, sid, step string) {
	accs := r.lnAccs()
	if i < 0 || i >= len(accs) {
		r.Show(chat, mid, "❌ اکانت پیدا نشد.", HomeKB())
		return
	}
	cli := ln.New(strv(accs[i]["token"]))
	s, err := cli.Call("GET", "/linode/instances/"+sid, nil)
	if err != nil || strv(s["id"]) == "" {
		r.Show(chat, mid, "❌ سرور پیدا نشد.", HomeKB())
		return
	}
	back := fmt.Sprintf("lnsi:%d:%s", i, sid)
	home := []Btn{btn("🔙 بازگشت", "back:"+back), btn("🏠 خانه", "menu")}
	if step == "rename" {
		r.setWizard(chat, map[string]any{"kind": "ln_srv_rename", "acc": i, "sid": sid})
		r.Show(chat, mid, "✏️ نام جدید سرور را بفرستید:", [][]Btn{{btn("🔙 انصراف", back)}})
		return
	}
	switch step {
	case "rebuild":
		var avail []map[string]any
		for _, im := range cli.All("/images") {
			if strv(im["status"]) == "available" {
				avail = append(avail, im)
			}
		}
		if len(avail) == 0 {
			r.Show(chat, mid, "❌ تصویری پیدا نشد.", [][]Btn{home})
			return
		}
		sort.SliceStable(avail, func(a, b int) bool {
			return strings.ToLower(strv(avail[a]["label"])) < strings.ToLower(strv(avail[b]["label"]))
		})
		rows := []Btn{}
		for j, im := range avail {
			if j >= 60 {
				break
			}
			rows = append(rows, btnS(truncRune(strv(im["label"]), 28), fmt.Sprintf("lnsv:%d:%s:rebuild:%s", i, sid, strv(im["id"])), "plain"))
		}
		kb := gridBTN(rows, 2)
		kb = append(kb, home)
		r.Show(chat, mid, "🛠️ تصویر جدید برای ریبیلد:\n⚠️ همهٔ داده‌های سرور پاک می‌شود.", kb)
		return
	case "upgrade":
		cur, _ := s["specs"].(map[string]any)
		var up []map[string]any
		for _, t := range cli.All("/linode/types") {
			if strv(t["id"]) == strv(s["type"]) {
				continue
			}
			bigger := intOfAny(t["memory"]) > intOfAny(cur["memory"]) ||
				intOfAny(t["vcpus"]) > intOfAny(cur["vcpus"]) ||
				intOfAny(t["disk"]) > intOfAny(cur["disk"])
			notSmaller := intOfAny(t["memory"]) >= intOfAny(cur["memory"]) &&
				intOfAny(t["vcpus"]) >= intOfAny(cur["vcpus"]) &&
				intOfAny(t["disk"]) >= intOfAny(cur["disk"])
			if bigger && notSmaller {
				up = append(up, t)
			}
		}
		if len(up) == 0 {
			r.Show(chat, mid, "❌ پلن ارتقای بالاتری موجود نیست.", [][]Btn{home})
			return
		}
		sort.SliceStable(up, func(a, b int) bool {
			sa := intOfAny(up[a]["memory"]) + intOfAny(up[a]["vcpus"]) + intOfAny(up[a]["disk"])
			sb := intOfAny(up[b]["memory"]) + intOfAny(up[b]["vcpus"]) + intOfAny(up[b]["disk"])
			return sa < sb
		})
		region := strv(s["region"])
		rows := []Btn{}
		for j, t := range up {
			if j >= 60 {
				break
			}
			price := lnRegionPrice(t, region)
			rows = append(rows, btnS(fmt.Sprintf("%s ($%s/m)", strv(t["label"]), orDash(strvVal(price["monthly"]))),
				fmt.Sprintf("lnsv:%d:%s:upgrade:%s", i, sid, strv(t["id"])), "plain"))
		}
		kb := gridBTN(rows, 2)
		kb = append(kb, home)
		r.Show(chat, mid, "⬆️ پلن جدید را انتخاب کنید:", kb)
		return
	case "dsnap":
		var avail []map[string]any
		for _, im := range cli.All("/images?is_public=false") {
			if strv(im["status"]) == "available" {
				avail = append(avail, im)
			}
		}
		if len(avail) == 0 {
			r.Show(chat, mid, "❌ اسنپ‌شاتی برای حذف پیدا نشد.", [][]Btn{home})
			return
		}
		rows := []Btn{}
		for j, im := range avail {
			if j >= 60 {
				break
			}
			rows = append(rows, btnS(truncRune(strv(im["label"]), 28), fmt.Sprintf("lnsv:%d:%s:dsnap:%s", i, sid, strv(im["id"])), "plain"))
		}
		kb := gridBTN(rows, 2)
		kb = append(kb, home)
		r.Show(chat, mid, "🗑 اسنپ‌شات برای حذف:", kb)
		return
	}
	labels := map[string]string{"on": "روشن کردن", "off": "خاموش کردن",
		"reboot": "ریبوت", "reset": "ریست", "pass": "ریست رمز", "del": "حذف سرور",
		"snap": "ساخت اسنپ‌شات", "a4": "افزودن IPv4", "a6": "افزودن IPv6",
		"u4": "حذف IPv4", "u6": "حذف IPv6"}
	label, ok := labels[step]
	if !ok {
		r.Show(chat, mid, "❓ عملیات ناشناخته.", HomeKB())
		return
	}
	r.Show(chat, mid, fmt.Sprintf("⚠️ مطمئنید «%s» روی «%s» انجام شود؟", label, Esc(strv(s["label"]))),
		[][]Btn{{btn("✅ بله", fmt.Sprintf("lnok:%d:%s:%s:", i, sid, step)), btn("❌ انصراف", back)}})
}

func lnRegionPrice(t map[string]any, region string) map[string]any {
	if t == nil {
		return map[string]any{}
	}
	if arr, ok := t["region_prices"].([]any); ok {
		for _, x := range arr {
			if m, ok := x.(map[string]any); ok && strv(m["id"]) == region {
				return m
			}
		}
	}
	if p, ok := t["price"].(map[string]any); ok {
		return p
	}
	return map[string]any{}
}

// lnLabelPart keeps only characters Linode accepts in image labels.
func lnLabelPart(s string) string {
	var b strings.Builder
	for _, c := range s {
		switch {
		case c >= 'a' && c <= 'z', c >= 'A' && c <= 'Z', c >= '0' && c <= '9', c == '-', c == '_':
			b.WriteRune(c)
		default:
			b.WriteByte('-')
		}
	}
	out := strings.Trim(b.String(), "-")
	if len(out) > 32 {
		out = out[:32]
	}
	if out == "" {
		out = "srv"
	}
	return out
}

func lnRandomPass() string {
	b := make([]byte, 18)
	for k := range b {
		b[k] = randChars[rand.Intn(len(randChars))]
	}
	return string(b)
}

func (r *Router) lnPick(chat, mid int64, i int, sid, step, arg string) {
	accs := r.lnAccs()
	if i < 0 || i >= len(accs) {
		r.Show(chat, mid, "❌ اکانت پیدا نشد.", HomeKB())
		return
	}
	s, err := ln.New(strv(accs[i]["token"])).Call("GET", "/linode/instances/"+sid, nil)
	if err != nil || strv(s["id"]) == "" {
		r.Show(chat, mid, "❌ سرور پیدا نشد.", HomeKB())
		return
	}
	labels := map[string]string{"rebuild": "ریبیلد", "upgrade": "ارتقا", "dsnap": "حذف اسنپ‌شات"}
	label := step
	if v, ok := labels[step]; ok {
		label = v
	}
	r.Show(chat, mid, fmt.Sprintf("⚠️ مطمئنید «%s» روی «%s» انجام شود؟", label, Esc(strv(s["label"]))),
		[][]Btn{{btn("✅ بله", fmt.Sprintf("lnok:%d:%s:%s:%s", i, sid, step, arg)),
			btn("❌ انصراف", fmt.Sprintf("lnsi:%d:%s", i, sid))}})
}

func (r *Router) lnDo(chat, mid int64, i int, sid, step, arg string) {
	back := navKB(fmt.Sprintf("lnsi:%d:%s", i, sid))
	backList := [][]Btn{{btn("🖥 سرورها", fmt.Sprintf("lns:%d:0", i))}}
	accs := r.lnAccs()
	if i < 0 || i >= len(accs) {
		r.Show(chat, mid, "❌ اکانت پیدا نشد.", HomeKB())
		return
	}
	cli := ln.New(strv(accs[i]["token"]))
	fail := func(err error) { r.Show(chat, mid, "❌ خطا: "+Esc(err.Error()), back) }
	switch step {
	case "del":
		if _, err := cli.Call("DELETE", "/linode/instances/"+sid, nil); err != nil {
			fail(err)
			return
		}
		r.lnDropServers(i)
		r.Bot.Send(chat, "✅ سرور حذف شد.", nil)
		r.lnServers(chat, 0, i, 0)
	case "on", "off", "reboot", "reset":
		action := "boot"
		if step == "off" {
			action = "shutdown"
		} else if step == "reboot" || step == "reset" {
			action = "reboot"
		}
		if _, err := cli.Call("POST", fmt.Sprintf("/linode/instances/%s/%s", sid, action), map[string]any{}); err != nil {
			fail(err)
			return
		}
		r.Bot.Send(chat, "✅ دستور ارسال شد.", nil)
		r.lnInfo(chat, 0, i, sid)
	case "pass":
		disk, err := cli.Disk(sid)
		if err != nil {
			fail(err)
			return
		}
		if disk == nil {
			r.Show(chat, mid, "❌ دیسکی پیدا نشد.", back)
			return
		}
		pw := lnRandomPass()
		if _, err := cli.Call("POST", fmt.Sprintf("/linode/instances/%s/disks/%s/password", sid, strv(disk["id"])),
			map[string]any{"password": pw}); err != nil {
			r.Show(chat, mid, "❌ خطا: "+Esc(err.Error())+"\n(برای تغییر رمز، سرور باید خاموش باشد)", back)
			return
		}
		r.Bot.Send(chat, "🔓 رمز جدید سرور:\n<code>"+Esc(pw)+"</code>\n\n⚠️ برای اعمال، سرور را خاموش و روشن کن.", nil)
		r.lnInfo(chat, 0, i, sid)
	case "snap":
		s, err := cli.Call("GET", "/linode/instances/"+sid, nil)
		if err != nil {
			fail(err)
			return
		}
		disk, err := cli.Disk(sid)
		if err != nil {
			fail(err)
			return
		}
		if disk == nil {
			r.Show(chat, mid, "❌ دیسکی پیدا نشد.", back)
			return
		}
		label := fmt.Sprintf("snap-%s-%d", lnLabelPart(strv(s["label"])), time.Now().Unix())
		if _, err := cli.Call("POST", "/images", map[string]any{"disk_id": disk["id"], "label": label}); err != nil {
			fail(err)
			return
		}
		r.Bot.Send(chat, "✅ اسنپ‌شات ساخته شد.", nil)
		r.lnInfo(chat, 0, i, sid)
	case "dsnap":
		if _, err := cli.Call("DELETE", "/images/"+arg, nil); err != nil {
			fail(err)
			return
		}
		r.Bot.Send(chat, "✅ اسنپ‌شات حذف شد.", nil)
		r.lnInfo(chat, 0, i, sid)
	case "a4", "a6":
		typ := "ipv4"
		if step == "a6" {
			typ = "ipv6"
		}
		if _, err := cli.Call("POST", fmt.Sprintf("/linode/instances/%s/ips", sid),
			map[string]any{"type": typ, "public": true}); err != nil {
			fail(err)
			return
		}
		r.Bot.Send(chat, "✅ آی‌پی "+strings.ToUpper(typ)+" اضافه شد.", nil)
		r.lnInfo(chat, 0, i, sid)
	case "u4":
		s, err := cli.Call("GET", "/linode/instances/"+sid, nil)
		if err != nil {
			fail(err)
			return
		}
		var extra []string
		for _, a := range selStrings(s["ipv4"]) {
			if ip := net.ParseIP(a); ip != nil && !ip.IsPrivate() {
				extra = append(extra, a)
			}
		}
		if len(extra) < 2 {
			r.Show(chat, mid, "❌ آی‌پی IPv4 اضافه‌ای برای حذف وجود ندارد.", back)
			return
		}
		addr := extra[len(extra)-1]
		if _, err := cli.Call("DELETE", "/networking/ips/"+url.PathEscape(addr), nil); err != nil {
			fail(err)
			return
		}
		r.Bot.Send(chat, "✅ آی‌پی حذف شد.", nil)
		r.lnInfo(chat, 0, i, sid)
	case "u6":
		var addrs []string
		for _, x := range cli.All("/networking/ips") {
			if strv(x["type"]) == "ipv6" && strv(x["linode_id"]) == sid {
				addrs = append(addrs, strv(x["address"]))
			}
		}
		if len(addrs) == 0 {
			r.Show(chat, mid, "❌ آی‌پی IPv6 قابل حذف پیدا نشد.", back)
			return
		}
		addr := addrs[len(addrs)-1]
		if _, err := cli.Call("DELETE", "/networking/ips/"+url.PathEscape(addr), nil); err != nil {
			fail(err)
			return
		}
		r.Bot.Send(chat, "✅ آی‌پی IPv6 حذف شد.", nil)
		r.lnInfo(chat, 0, i, sid)
	case "rebuild":
		pw := lnRandomPass()
		if _, err := cli.Call("POST", "/linode/instances/"+sid+"/rebuild",
			map[string]any{"image": arg, "root_pass": pw, "booted": true}); err != nil {
			fail(err)
			return
		}
		r.Bot.Send(chat, "🛠️ ریبیلد شروع شد؛ رمز root:\n<code>"+Esc(pw)+"</code>", backList)
	case "upgrade":
		if _, err := cli.Call("POST", "/linode/instances/"+sid+"/resize",
			map[string]any{"type": arg, "allow_auto_disk_resize": true}); err != nil {
			fail(err)
			return
		}
		r.Bot.Send(chat, "✅ ارتقا انجام شد.", nil)
		r.lnInfo(chat, 0, i, sid)
	default:
		r.Show(chat, mid, "❓ عملیات ناشناخته.", back)
	}
}

func (r *Router) lnAddName(chat, mid int64, text string) {
	name := strings.ReplaceAll(text, " ", "")
	if name == "" {
		r.Bot.Send(chat, "❌ نام معتبر نیست.", nil)
		return
	}
	r.setWizard(chat, map[string]any{"kind": "ln_add_token", "name": name})
	r.Bot.Send(chat, "🔑 توکن API لینود را بفرستید:", nil)
}

func (r *Router) lnAddToken(chat, mid int64, pend map[string]any, text string) {
	tok := strings.TrimSpace(text)
	acc, err := ln.New(tok).Call("GET", "/account", nil)
	if err != nil || (strv(acc["email"]) == "" && strv(acc["id"]) == "") {
		msg := "invalid"
		if err != nil {
			msg = err.Error()
		}
		r.Bot.Send(chat, "❌ توکن نامعتبر است: "+Esc(msg), nil)
		return
	}
	accs := r.lnAccs()
	name, _ := pend["name"].(string)
	if name == "" {
		name = "ln"
	}
	accs = append(accs, map[string]any{"name": name, "token": tok})
	r.lnSaveAccs(accs)
	r.clearWizard(chat)
	r.Bot.Send(chat, "✅ اکانت لینود اضافه شد.", nil)
	r.lnHome(chat, 0)
}

func (r *Router) lnRename(chat, mid int64, pend map[string]any, text string) {
	i := intOrAny(pend["acc"])
	sid, _ := pend["sid"].(string)
	accs := r.lnAccs()
	if i < 0 || i >= len(accs) {
		r.Bot.Send(chat, "❌ اکانت پیدا نشد.", nil)
		return
	}
	if _, err := ln.New(strv(accs[i]["token"])).Call("PUT",
		"/linode/instances/"+sid, map[string]any{"label": strings.TrimSpace(text)}); err != nil {
		r.Bot.Send(chat, "❌ نشد: "+Esc(err.Error()), nil)
		return
	}
	r.Bot.Send(chat, "✅ نام سرور تغییر کرد.", nil)
	r.clearWizard(chat)
	r.lnInfo(chat, 0, i, sid)
}

// ---------- create server (lnsc) ----------

func (r *Router) lnCreateStart(chat, mid int64, i int) {
	accs := r.lnAccs()
	if i < 0 || i >= len(accs) {
		r.Show(chat, mid, "❌ اکانت پیدا نشد.", HomeKB())
		return
	}
	r.setWizard(chat, map[string]any{"kind": "ln_srv_remark", "acc": i})
	r.Show(chat, mid, "➕ ساخت سرور لینود\n\nنام سرور را بفرستید (بدون فاصله، مثلاً web1):",
		[][]Btn{{btn("🔙 انصراف", "ln")}})
}

func (r *Router) lnCreateRemark(chat, mid int64, pend map[string]any, text string) {
	remark := strings.TrimSpace(text)
	if remark == "" || strings.ContainsAny(remark, " \t") {
		r.Bot.Send(chat, "⚠️ نام سرور نباید خالی یا دارای فاصله باشد. دوباره بفرستید:", nil)
		return
	}
	i := intOrAny(pend["acc"])
	accs := r.lnAccs()
	if i < 0 || i >= len(accs) {
		r.clearWizard(chat)
		r.Bot.Send(chat, "❌ اکانت پیدا نشد.", nil)
		return
	}
	regions := ln.New(strv(accs[i]["token"])).All("/regions")
	var rows []Btn
	for _, rg := range regions {
		ok := false
		if caps, cok := rg["capabilities"].([]any); cok {
			for _, c := range caps {
				if strv(c) == "Linodes" {
					ok = true
				}
			}
		}
		if !ok {
			continue
		}
		rows = append(rows, btnS(strv(rg["label"]), fmt.Sprintf("lnscd:%d:%s", i, strv(rg["id"])), "plain"))
	}
	if len(rows) == 0 {
		r.clearWizard(chat)
		r.Bot.Send(chat, "❌ منطقه‌ای پیدا نشد.", nil)
		return
	}
	r.setWizard(chat, map[string]any{"kind": "ln_srv_remark", "acc": i, "remark": remark})
	kb := gridBTN(rows, 2)
	kb = append(kb, []Btn{btn("🔙 انصراف", "ln"), btn("🏠 خانه", "menu")})
	r.Show(chat, mid, "🌍 منطقه را انتخاب کنید:", kb)
}

func (r *Router) lnCreateRegion(chat, mid int64, i int, region string) {
	pend := r.getWizard(chat)
	if pend == nil || strv(pend["kind"]) != "ln_srv_remark" || intOrAny(pend["acc"]) != i || strv(pend["remark"]) == "" {
		r.Show(chat, mid, "⏳ عملیات منقضی شده.", navKB("ln"))
		return
	}
	accs := r.lnAccs()
	if i < 0 || i >= len(accs) {
		r.Show(chat, mid, "❌ اکانت پیدا نشد.", HomeKB())
		return
	}
	types := ln.New(strv(accs[i]["token"])).All("/linode/types")
	sort.SliceStable(types, func(a, b int) bool {
		return floatVal(lnRegionPrice(types[a], region)["monthly"]) <
			floatVal(lnRegionPrice(types[b], region)["monthly"])
	})
	var rows []Btn
	for _, t := range types {
		pr := lnRegionPrice(t, region)
		m := strv(pr["monthly"])
		if m == "" {
			m = "?"
		}
		rows = append(rows, btnS(fmt.Sprintf("%s ($%s/m)", strv(t["label"]), m),
			fmt.Sprintf("lnscp:%d:%s:%s", i, region, strv(t["id"])), "plain"))
	}
	if len(rows) == 0 {
		r.Show(chat, mid, "❌ پلنی پیدا نشد.", navKB("ln"))
		return
	}
	pend["region"] = region
	r.setWizard(chat, pend)
	kb := gridBTN(rows, 2)
	kb = append(kb, []Btn{btn("🔙 بازگشت", "back:ln"), btn("🏠 خانه", "menu")})
	r.Show(chat, mid, "💰 پلن سرور را انتخاب کنید:", kb)
}

func (r *Router) lnCreatePlan(chat, mid int64, i int, region, typeID string) {
	pend := r.getWizard(chat)
	if pend == nil || strv(pend["kind"]) != "ln_srv_remark" || strv(pend["remark"]) == "" {
		r.Show(chat, mid, "⏳ عملیات منقضی شده.", navKB("ln"))
		return
	}
	accs := r.lnAccs()
	if i < 0 || i >= len(accs) {
		r.Show(chat, mid, "❌ اکانت پیدا نشد.", HomeKB())
		return
	}
	imgs := ln.New(strv(accs[i]["token"])).All("/images")
	var rows []Btn
	for _, im := range imgs {
		if !boolVal(im["is_public"]) || strv(im["status"]) != "available" {
			continue
		}
		if len(rows) >= 60 {
			break
		}
		rows = append(rows, btnS(strv(im["label"]), fmt.Sprintf("lnsci:%d:%s", i, strv(im["id"])), "plain"))
	}
	if len(rows) == 0 {
		r.Show(chat, mid, "❌ تصویری پیدا نشد.", navKB("ln"))
		return
	}
	pend["region"] = region
	pend["type"] = typeID
	r.setWizard(chat, pend)
	kb := gridBTN(rows, 2)
	kb = append(kb, []Btn{btn("🔙 بازگشت", "back:ln"), btn("🏠 خانه", "menu")})
	r.Show(chat, mid, "🖼️ تصویر (سیستم‌عامل) را انتخاب کنید:", kb)
}

func (r *Router) lnCreateImage(chat, mid int64, i int, imageID string) {
	pend := r.getWizard(chat)
	if pend == nil || strv(pend["kind"]) != "ln_srv_remark" || strv(pend["remark"]) == "" {
		r.Show(chat, mid, "⏳ عملیات منقضی شده.", navKB("ln"))
		return
	}
	accs := r.lnAccs()
	if i < 0 || i >= len(accs) {
		r.Show(chat, mid, "❌ اکانت پیدا نشد.", HomeKB())
		return
	}
	pass := lnRandomPass()
	body := map[string]any{
		"label": strv(pend["remark"]), "region": strv(pend["region"]),
		"type": strv(pend["type"]), "image": imageID,
		"root_pass": pass, "booted": true,
	}
	res, err := ln.New(strv(accs[i]["token"])).Call("POST", "/linode/instances", body)
	if err != nil {
		r.Show(chat, mid, "❌ خطا: "+Esc(err.Error()), navKB("ln"))
		return
	}
	if strv(res["id"]) == "" {
		detail := "—"
		if e, ok := res["errors"].([]any); ok && len(e) > 0 {
			detail = truncRune(fmt.Sprintf("%v", e), 200)
		}
		r.Show(chat, mid, "❌ خطا: "+Esc(detail), navKB("ln"))
		return
	}
	r.clearWizard(chat)
	r.lnDropServers(i)
	ip := lnFirstIP(res)
	lst := r.srvList()
	lst = append(lst, srvRec(fmt.Sprintf("s%d%d%d", time.Now().Unix(), rand.Intn(900)+100, len(lst)),
		strv(res["label"]), ip, "root", 22, "pass", pass, "Linode"))
	r.srvSave(lst)
	kb := [][]Btn{}
	if ip != "" {
		kb = append(kb, row(copyBtnL("📋 کپی آی‌پی", ip)))
	}
	kb = append(kb, row(copyBtnL("📋 کپی رمز", pass)))
	kb = append(kb, row(btn("🖥 سرورها", "srv"), btn("🔙 لینود", fmt.Sprintf("lns:%d:0", i))))
	kb = append(kb, row(btn("🏠 خانه", "menu")))
	r.Show(chat, mid, fmt.Sprintf("🎉 سرور لینود ساخته شد.\n\n🏷 %s\n🔗 <code>%s</code>\n👤 root\n🔑 رمز: <code>%s</code>\n\nبه «سرورها» اضافه شد.",
		Esc(strv(res["label"])), Esc(ip), Esc(pass)), kb)
}
