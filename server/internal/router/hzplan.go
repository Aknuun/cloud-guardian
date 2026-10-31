package router

import (
	"fmt"
	"math/rand"
	"strings"
	"time"

	"github.com/aknuun/cg-server/internal/hz"
)

func (r *Router) hzPlanList(chat, mid int64, i int, notice, retryPlan string) {
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
	loc := strv(pend["location"])
	arch := strv(pend["arch"])
	cli := hz.New(strv(accs[i]["token"]))
	types := r.hzTypesCached(cli, i)
	shared, dedicated, count := hzUsageOf(r.hzServersCached(i))
	caps := r.hzCapGet(strv(accs[i]["name"]))
	var avail []map[string]any
	for _, t := range types {
		if boolVal(t["deprecated"]) {
			continue
		}
		if arch != "" && strv(t["architecture"]) != arch {
			continue
		}
		if hzPlanInLoc(t, loc) {
			avail = append(avail, t)
		}
	}
	sortPlanByPrice(avail, loc)
	famLabels := map[string]string{"cpx": "🧩 CPX · پردازنده AMD", "cx": "🧩 CX · پردازنده Intel",
		"cax": "🧩 CAX · پردازنده Ampere ARM", "ccx": "🧩 CCX · اختصاصی (Dedicated)",
		"other": "🧩 سایر پلن‌ها"}
	groups := map[string][]map[string]any{}
	for _, t := range avail {
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
	anyBlocked := false
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
						eur = eur0(pm["gross"])
					}
					break
				}
			}
			mark, cb := "✅", fmt.Sprintf("hzscp:%d:%s", i, strv(t["id"]))
			if bad, _ := hzBlocked(t, caps, shared, dedicated, count); bad {
				mark, cb = "⛔", fmt.Sprintf("hzcap:%d:%s", i, strv(t["id"]))
				anyBlocked = true
			}
			rows = append(rows, btnS(fmt.Sprintf("%s %dC/%dG · %s", mark, intOfAny(t["cores"]), intOfAny(t["memory"]), eur), cb, "plain"))
		}
		kb = append(kb, gridBTN(rows, 2)...)
	}
	if retryPlan != "" {
		kb = append(kb, row(btn("🔁 به‌هرحال امتحان کن", fmt.Sprintf("hzscp:%d:%s", i, retryPlan))))
	}
	kb = append(kb, row(btn("🔙 بازگشت", "back:hz"), btn("🏠 خانه", "menu")))
	hd := fmt.Sprintf("💰 پلن سرور را انتخاب کن\n📍 لوکیشن: %s", Esc(loc))
	if img := strv(pend["imageName"]); img != "" {
		hd += " • 🖼️ " + Esc(img)
	}
	hd += fmt.Sprintf("\n📊 مصرف فعلی اکانت: %d هستهٔ shared", shared)
	if dedicated > 0 {
		hd += fmt.Sprintf(" • %d هستهٔ dedicated", dedicated)
	}
	if anyBlocked {
		hd += "\n⛔ = با ظرفیت فعلی اکانت جا نمی‌شود • ✅ = قابل ساخت"
	}
	if notice != "" {
		hd = "⚠️ " + notice + "\n\n" + hd
	}
	r.Show(chat, mid, hd, kb)
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

// hzPlanInLoc reports whether a server type is offered in a location.
func hzPlanInLoc(t map[string]any, loc string) bool {
	if loc == "" {
		return true
	}
	for _, p := range priceList(t) {
		if strv(p["location"]) == loc {
			return true
		}
	}
	return false
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

const randChars = "abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ0123456789"

func (r *Router) hzCreate(chat, mid int64, i int) {
	accs := r.hzAccs()
	if i < 0 || i >= len(accs) {
		r.Show(chat, mid, "❌ اکانت پیدا نشد.", HomeKB())
		return
	}
	pend := r.getWizard(chat)
	if pend == nil || strv(pend["kind"]) != "hz_srv_remark" || strv(pend["location"]) == "" ||
		strv(pend["planId"]) == "" || strv(pend["image"]) == "" {
		r.Show(chat, mid, "⏳ عملیات منقضی شده.", HomeKB())
		return
	}
	cli := hz.New(strv(accs[i]["token"]))
	plan := strv(pend["planId"])
	cores := 0
	if td, err := cli.Call("/server_types/"+plan, "GET", nil); err == nil {
		if st, _ := td["server_type"].(map[string]any); st != nil {
			cores = intOfAny(st["cores"])
		}
	}
	sharedBefore, dedBefore, countBefore := r.hzUsage(cli)
	b := make([]byte, 16)
	for k := range b {
		b[k] = randChars[rand.Intn(len(randChars))]
	}
	pw := string(b)
	cloudInit := "#cloud-config\nssh_pwauth: true\nchpasswd:\n  list: |\n    root:" + pw + "\n  expire: False\n"
	resp, err := cli.Call("/servers", "POST", map[string]any{
		"name":               strv(pend["remark"]),
		"server_type":        plan,
		"image":              strv(pend["image"]),
		"location":           strv(pend["location"]),
		"start_after_create": true,
		"user_data":          cloudInit,
	})
	if err != nil {
		low := strings.ToLower(err.Error())
		accName := strv(accs[i]["name"])
		switch {
		case strings.Contains(low, "core limit") && strings.Contains(low, "dedicated"):
			r.hzCapSet(accName, "dedicated", cores, dedBefore)
		case strings.Contains(low, "core limit"):
			r.hzCapSet(accName, "shared", cores, sharedBefore)
		case strings.Contains(low, "server limit"):
			r.hzCapSet(accName, "server", 0, countBefore)
		}
		// Stay on the plan list (same message) with the reason and a		// one-tap retry; no extra page.
		r.hzPlanList(chat, mid, i, "ساخت سرور انجام نشد: "+Esc(err.Error()), plan)
		return
	}
	r.clearWizard(chat)
	r.hzDropServers(i)
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
	r.hzDropServers(i)
	back := navKB(fmt.Sprintf("hzsi:%d:%s", i, sid))
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
		if pw == "" {
			r.Bot.Send(chat, "🔓 درخواست ریست رمز ارسال شد، ولی هتزنر رمزی برنگرداند. از کنسول سرور رمز را عوض کن.", nil)
		} else {
			r.Bot.Send(chat, "🔓 رمز root جدید:\n<code>"+Esc(pw)+"</code>\n\n⚠️ آن را امن نگه دار.", nil)
		}
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
		_, _ = cli.Call("/servers/"+sid+"/actions/poweron", "POST", map[string]any{})
		r.Bot.Send(chat, "✅ آی‌پی اختصاص یافت و سرور روشن شد.", nil)
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
		ipid := 0
		if m, _ := pn[key].(map[string]any); m != nil {
			ipid = intOfAny(m["id"])
		}
		if ipid <= 0 {
			r.Show(chat, mid, "ℹ️ آی‌پی اختصاصی (Primary IP) برای حذف وجود ندارد.", back)
			return
		}
		if !hz.EnsureOff(cli, sid, 30000) {
			r.Show(chat, mid, "⚠️ سرور در ۳۰ ثانیه خاموش نشد؛ دوباره تلاش کن.", back)
			return
		}
		if _, err := cli.Call(fmt.Sprintf("/primary_ips/%d/actions/unassign", ipid), "POST", map[string]any{}); err != nil {
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
		loc, _ := s["location"].(map[string]any)
		locName := strv(loc["name"])
		if locName == "" {
			r.Show(chat, mid, "❌ لوکیشن سرور مشخص نیست.", back)
			return
		}
		pn, _ := s["public_net"].(map[string]any)
		oldID := 0
		if m, _ := pn["ipv4"].(map[string]any); m != nil {
			oldID = intOfAny(m["id"])
		}
		if !hz.EnsureOff(cli, sid, 30000) {
			r.Show(chat, mid, "⚠️ سرور در ۳۰ ثانیه خاموش نشد؛ دوباره تلاش کن.", back)
			return
		}
		c, err := cli.Call("/primary_ips", "POST", map[string]any{
			"type": "ipv4", "name": "replace-" + sid, "location": locName, "auto_delete": true})
		if err != nil {
			fail(err)
			return
		}
		nip, _ := c["primary_ip"].(map[string]any)
		newID := 0
		if nip != nil {
			newID = intOfAny(nip["id"])
		}
		if newID <= 0 {
			r.Show(chat, mid, "❌ ساخت IPv4 تازه ناموفق شد.", back)
			return
		}
		if oldID > 0 {
			_, _ = cli.Call(fmt.Sprintf("/primary_ips/%d/actions/unassign", oldID), "POST", map[string]any{})
		}
		_, _ = cli.Call(fmt.Sprintf("/primary_ips/%d/actions/assign", newID), "POST",
			map[string]any{"assignee_id": atoiSafe(sid), "assignee_type": "server"})
		if oldID > 0 {
			_, _ = cli.Call(fmt.Sprintf("/primary_ips/%d", oldID), "DELETE", nil)
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
	locs := r.hzLocsCached(hz.New(strv(accs[i]["token"])), i)
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
		rows = append(rows, btnS(fmt.Sprintf("%s (%s)", city, strv(l["name"])), fmt.Sprintf("hzsl:%d:%s", i, strv(l["name"])), "plain"))
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
