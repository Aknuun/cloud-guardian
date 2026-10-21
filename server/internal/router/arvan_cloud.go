package router

import (
	"fmt"
	"strings"
	"time"

	"github.com/aknuun/cg-server/internal/arvan"
)

// ---------- ArvanCloud ECC (cloud servers) ----------

func (r *Router) arvAccounts() []map[string]any {
	var accs []map[string]any
	if !r.Store.GetJSON("arvan_accounts", &accs) {
		return nil
	}
	return accs
}

func (r *Router) arvSess(chat int64) map[string]any {
	var m map[string]any
	if r.Store.GetJSON(fmt.Sprintf("arvsess:%d", chat), &m) {
		return m
	}
	return map[string]any{}
}

func (r *Router) arvSessSave(chat int64, m map[string]any) {
	r.Store.PutJSON(fmt.Sprintf("arvsess:%d", chat), m, 3600)
}

func (r *Router) arvCli(i int) (*arvan.Client, bool) {
	accs := r.arvAccounts()
	if i < 0 || i >= len(accs) {
		return nil, false
	}
	return arvan.New(strv(accs[i]["token"])), true
}

var arvanFallbackRegions = []string{
	"ir-thr-ba1", "ir-thr-fr1", "ir-tbz-sh1", "ir-thr-si1", "ir-southwest1-a", "eu-west1-a",
}

func arvEmoji(status string) string {
	st := strings.ToLower(status)
	switch {
	case strings.Contains(st, "run") || strings.Contains(st, "active"):
		return "🟢"
	case strings.Contains(st, "build") || strings.Contains(st, "creat") || strings.Contains(st, "pending") || strings.Contains(st, "reboot"):
		return "🟡"
	case strings.Contains(st, "off") || strings.Contains(st, "stop") || strings.Contains(st, "shut"):
		return "🔴"
	}
	return "⚪"
}

func (r *Router) arvWarmArvan() {
	for i := range r.arvAccounts() {
		r.arvAllServersSWR(i)
		r.arvRegionsFast(i)
	}
}

func (r *Router) arvCloudHome(chat, mid int64) {
	accs := r.arvAccounts()
	if len(accs) == 0 {
		r.Show(chat, mid, "🖥 سرورهای ابری آروان\n\nاول یک اکانت آروان اضافه کن.",
			[][]Btn{{btn("👤 اکانت‌ها", "arvanacc")}, {btn("🔙 بازگشت", "arvan"), btn("🏠 خانه", "menu")}})
		return
	}
	kb := [][]Btn{}
	lines := []string{"🖥 سرورهای ابری آروان", ""}
	stale := false
	for i, a := range accs {
		regions := r.arvRegionsFast(i)
		list, st := r.arvAllServersSWR(i)
		if st {
			stale = true
		}
		lines = append(lines, fmt.Sprintf("👤 <b>%s</b> — %d سرور · %d ریجن", Esc(strv(a["name"])), len(list), len(regions)))
		for j, s := range list {
			if j >= 8 {
				break
			}
			label := fmt.Sprintf("%s %s", arvEmoji(strv(s["status"])), strv(s["name"]))
			if ip := firstPubIP(s); ip != "" {
				label += " · " + ip
			}
			label += " · " + r.arvRegionName(strv(s["region"]))
			kb = append(kb, []Btn{btnS(truncRune(label, 58), fmt.Sprintf("arvpick:%d:%d", i, j), "plain")})
		}
		if len(list) == 0 {
			lines = append(lines, "📭 سروری در این اکانت نیست.")
		}
		kb = append(kb, arvNav(i)...)
	}
	if stale {
		lines = append(lines, "", "🔄 در حال به‌روزرسانی در پس‌زمینه…")
	}
	kb = append(kb, []Btn{btn("🔄 تازه‌سازی", "arvregrefresh")})
	kb = append(kb, []Btn{btn("🔙 بازگشت", "arvan"), btn("🏠 خانه", "menu")})
	r.Show(chat, mid, strings.Join(lines, "\n"), kb)
	if !stale {
		return
	}
	if _, loaded := arvHomeBusy.LoadOrStore(chat, true); loaded {
		return
	}
	go func() {
		defer arvHomeBusy.Delete(chat)
		for i := range accs {
			r.arvAllServers(i, true)
			regs := r.arvRegionsFast(i)
			if len(regs) > 0 {
				rg := regs[0].Code
				r.arvSizes(i, rg)
				r.arvImageGroups(i, rg)
				r.arvPublicNetworkID(i, rg)
			}
		}
		v, _ := r.lastNav.Load(chat)
		if cur, _ := v.(string); cur == "arvsrv" || cur == "arvregrefresh" {
			r.arvCloudHome(chat, mid)
		}
	}()
}

// arvCloudRefresh forces a fresh fetch of every account's regions.
func (r *Router) arvCloudRefresh(chat, mid int64) {
	accs := r.arvAccounts()
	if len(accs) == 0 {
		r.arvCloudHome(chat, mid)
		return
	}
	r.Show(chat, mid, "⏳ در حال تازه‌سازی…", nil)
	for i := range accs {
		r.arvRegions(i, true)
		r.arvAllServers(i, true)
	}
	r.arvCloudHome(chat, mid)
}

func arvNormalize(s map[string]any) map[string]any {
	if s == nil {
		return nil
	}
	flavor, _ := s["flavor"].(map[string]any)
	image, _ := s["image"].(map[string]any)
	name := firstOf(s["name"], firstOf(s["hostname"], strv(s["id"])))
	fl, flID := "", ""
	cpu, ram, disk := 0, 0, 0
	if flavor != nil {
		fl = firstOf(flavor["name"], strv(flavor["id"]))
		flID = strv(flavor["id"])
		cpu = intOrAny(flavor["cpu"])
		if cpu == 0 {
			cpu = intOrAny(flavor["cpu_count"])
		}
		if cpu == 0 {
			cpu = intOrAny(flavor["vcpus"])
		}
		ram = intOrAny(flavor["ram"])
		if ram == 0 {
			ram = intOrAny(flavor["memory"])
		}
		if ram >= 512 {
			ram /= 1024
		}
		disk = intOrAny(flavor["disk"])
	}
	if cpu == 0 {
		cpu = intOrAny(s["cpu"])
	}
	if ram == 0 {
		ram = intOrAny(s["ram"])
	}
	if ram >= 512 {
		ram /= 1024
	}
	if disk == 0 {
		disk = intOrAny(s["disk"])
	}
	im := ""
	osName := ""
	if image != nil {
		im = firstOf(image["name"], strv(image["distribution"]))
		if v := strv(image["os_version"]); v != "" {
			im += " " + v
		}
		osName = firstOf(image["os"], strv(image["distribution"]))
	}
	pub, priv := arvServerIPs(s)
	return map[string]any{
		"id":       strv(s["id"]),
		"name":     name,
		"status":   firstOf(s["status"], firstOf(s["state"], strv(s["power_state"]))),
		"task":     strv(s["task_status"]),
		"flavor":   fl,
		"flavorID": flID,
		"image":    im,
		"cpu":      cpu,
		"ram":      ram,
		"disk":     disk,
		"region":   firstOf(s["region"], strv(s["region_code"])),
		"pub":      pub,
		"priv":     priv,
		"created":  strv(s["created"]),
		"os":       osName,
		"ha":       boolVal(s["ha_enabled"]),
	}
}

func containsStr(a []string, x string) bool {
	for _, v := range a {
		if v == x {
			return true
		}
	}
	return false
}

func (r *Router) arvServerText(s map[string]any, region string, extra ...string) string {
	pub := selStrings(s["pub"])
	priv := selStrings(s["priv"])
	var v4, v6 []string
	for _, ip := range pub {
		if strings.Contains(ip, ":") {
			v6 = append(v6, ip)
		} else {
			v4 = append(v4, ip)
		}
	}
	plan := orDash(strv(s["flavor"]))
	var sp []string
	cpu, ram, disk := intOrAny(s["cpu"]), intOrAny(s["ram"]), intOrAny(s["disk"])
	if cpu > 0 {
		sp = append(sp, fmt.Sprintf("%d vCPU", cpu))
	}
	if ram > 0 {
		sp = append(sp, fmt.Sprintf("%d GB RAM", ram))
	}
	if disk > 0 {
		sp = append(sp, fmt.Sprintf("%d GB دیسک", disk))
	}
	specs := ""
	if len(sp) > 0 {
		specs = " (" + strings.Join(sp, " · ") + ")"
	}
	st := orDash(strv(s["status"]))
	if fa := arvStatusFa(strv(s["status"])); fa != "" && fa != strv(s["status"]) {
		st = fa + " (" + strv(s["status"]) + ")"
	}
	if t := strv(s["task"]); t != "" && t != "done" {
		st += " · ⏳ " + t
	}
	lines := []string{
		"🖥 <b>" + Esc(strv(s["name"])) + "</b>",
		"",
		"📊 وضعیت: <code>" + Esc(st) + "</code>",
		"🌐 IPv4: <code>" + Esc(orDash(strings.Join(v4, "، "))) + "</code>",
	}
	if len(v6) > 0 {
		lines = append(lines, "🌐 IPv6: <code>"+Esc(strings.Join(v6, "، "))+"</code>")
	}
	lines = append(lines,
		"🔒 خصوصی: <code>"+Esc(orDash(strings.Join(priv, "، ")))+"</code>",
		"📦 پلن: <code>"+Esc(plan)+"</code>"+Esc(specs),
		"💿 ایمیج: <code>"+Esc(orDash(strv(s["image"])))+"</code>",
		"📍 لوکیشن: <code>"+Esc(orDash(r.arvRegionName(region)))+"</code>",
	)
	if ha, ok := s["ha"].(bool); ok && ha {
		lines = append(lines, "🛡 HA: <code>فعال</code>")
	}
	if c := strv(s["created"]); c != "" {
		day := c
		age := ""
		if t, err := time.Parse(time.RFC3339, c); err == nil {
			if len(day) >= 10 {
				day = day[:10]
			}
			age = fmt.Sprintf(" · %d روز پیش", int(time.Since(t).Hours()/24))
		}
		lines = append(lines, "📅 ساخته‌شده: <code>"+Esc(day)+"</code>"+Esc(age))
	}
	for _, x := range extra {
		if strings.TrimSpace(x) != "" {
			lines = append(lines, x)
		}
	}
	lines = append(lines, "🆔 <code>"+Esc(truncRune(strv(s["id"]), 18))+"</code>")
	return strings.Join(lines, "\n")
}

func (r *Router) arvServerList(chat, mid int64, i int, region string) {
	if _, ok := r.arvCli(i); !ok {
		r.Show(chat, mid, "❌ اکانت پیدا نشد.", HomeKB())
		return
	}
	var servers []any
	var ch arvAllCache
	if r.Store.GetJSON(fmt.Sprintf("arvall:%d", i), &ch) && ch.List != nil {
		for _, s := range ch.List {
			if strv(s["region"]) == region {
				servers = append(servers, s)
			}
		}
		go r.arvAllServersSWR(i)
	} else {
		r.Show(chat, mid, "⏳ لطفاً صبر کنید… در حال بارگذاری سرورها", nil)
		for _, s := range r.arvAllServers(i, true) {
			if strv(s["region"]) == region {
				servers = append(servers, s)
			}
		}
	}
	sess := r.arvSess(chat)
	sess["acc"] = i
	sess["region"] = region
	sess["servers"] = servers
	r.arvSessSave(chat, sess)
	var srvItems []Btn
	for j, x := range servers {
		s := x.(map[string]any)
		if j >= 30 {
			break
		}
		srvItems = append(srvItems, btnS(fmt.Sprintf("%s %s", arvEmoji(strv(s["status"])), truncRune(strv(s["name"]), 30)), fmt.Sprintf("arvs:%d:%d", i, j), "plain"))
	}
	kb := gridBTN(srvItems, 2)
	kb = append(kb, row(btn("➕ ساخت سرور جدید", fmt.Sprintf("arvnew:%d:%s", i, region))))
	kb = append(kb, row(btn("🔄 تازه‌سازی", fmt.Sprintf("arvreg:%d:%s", i, region))))
	kb = append(kb, arvNav(i)...)
	title := fmt.Sprintf("🖥 سرورهای ابری (%d)\n📍 %s", len(servers), Esc(r.arvRegionName(region)))
	if len(servers) == 0 {
		title += "\n\nℹ️ سروری در این ریجن نیست."
	}
	r.Show(chat, mid, title, kb)
}

func (r *Router) arvServerDetail(chat, mid int64, i int, idx int) {
	cli, ok := r.arvCli(i)
	if !ok {
		r.Show(chat, mid, "❌ اکانت پیدا نشد.", HomeKB())
		return
	}
	sess := r.arvSess(chat)
	region := strv(sess["region"])
	servers := asMapList(sess["servers"])
	if idx < 0 || idx >= len(servers) {
		r.Show(chat, mid, "⏳ نشست منقضی شد؛ از منو دوباره وارد شو.",
			navKB("arvsrv"))
		return
	}
	s := servers[idx]
	if d, err := cli.Ecc(region, "GET", "/servers/"+strv(s["id"]), nil); err == nil {
		if one := arvan.EccOne(d); one != nil {
			if n := arvNormalize(one); n != nil {
				n["region"] = region
				s = n
				servers[idx] = s
				sess["servers"] = servers
				r.arvSessSave(chat, sess)
			}
		}
	}
	st := strings.ToLower(strv(s["status"]))
	isOn := (strings.Contains(st, "run") || strings.Contains(st, "active")) && !strings.Contains(st, "off")
	var kb [][]Btn
	if isOn {
		kb = append(kb, []Btn{
			btnS("🔌 خاموش", fmt.Sprintf("arvoff:%d:%d", i, idx), "danger"),
			btn("🔄 ریبوت", fmt.Sprintf("arvreboot:%d:%d", i, idx))})
	} else {
		kb = append(kb, []Btn{btnS("⚡ روشن", fmt.Sprintf("arvon:%d:%d", i, idx), "success")})
	}
	kb = append(kb,
		[]Btn{btn("🧪 بررسی IP (ایران/آلمان)", fmt.Sprintf("arvipc:%d:%d", i, idx)), btn("⬆️ ارتقا", fmt.Sprintf("arvup:%d:%d", i, idx))},
		[]Btn{btn("📷 اسنپ‌شات", fmt.Sprintf("arvsnap:%d:%d", i, idx)), btn("📷 اسنپ‌شات‌ها", fmt.Sprintf("arvsnapls:%d:%d", i, idx))},
		[]Btn{btn("✏️ تغییرنام", fmt.Sprintf("arvren:%d:%d", i, idx)), btn("🔑 ریست رمز root", fmt.Sprintf("arvyroot:%d:%d", i, idx))},
	)
	if ip := firstPubIP(s); ip != "" {
		kb = append(kb, []Btn{copyBtnL("📋 کپی IPv4", ip), btn("🔄 تازه‌سازی", fmt.Sprintf("arvs:%d:%d", i, idx))})
	} else {
		kb = append(kb, []Btn{btn("🔄 تازه‌سازی", fmt.Sprintf("arvs:%d:%d", i, idx))})
	}
	kb = append(kb, arvNav(i)...)
	kb = append(kb,
		[]Btn{btnS("🗑 حذف سرور", fmt.Sprintf("arvterm:%d:%d", i, idx), "danger")},
		[]Btn{btn("🔙 بازگشت", fmt.Sprintf("arvreg:%d:%s", i, region)), btn("🏠 خانه", "menu")})
	traffic, ready := r.arvTrafficLineAsync(i, region, strv(s["id"]))
	r.Show(chat, mid, r.arvServerText(s, region, traffic), kb)
	if !ready {
		key := fmt.Sprintf("dtrf:%d:%d:%d", chat, i, idx)
		if _, loaded := arvDetailBusy.LoadOrStore(key, true); !loaded {
			go func() {
				defer arvDetailBusy.Delete(key)
				r.arvTraffic(i, region, strv(s["id"]))
				v, _ := r.lastNav.Load(chat)
				if cur, _ := v.(string); cur == "arvs" || cur == "arvpick" {
					r.arvServerDetail(chat, mid, i, idx)
				}
			}()
		}
	}
}

func (r *Router) arvRootReset(chat, mid int64, i, idx int) {
	cli, ok := r.arvCli(i)
	if !ok {
		r.Show(chat, mid, "❌ اکانت پیدا نشد.", HomeKB())
		return
	}
	sess := r.arvSess(chat)
	region := strv(sess["region"])
	servers := asMapList(sess["servers"])
	back := navKB(fmt.Sprintf("arvs:%d:%d", i, idx))
	if idx < 0 || idx >= len(servers) {
		r.Show(chat, mid, "⏳ نشست منقضی شد.", navKB("arvsrv"))
		return
	}
	d, err := cli.Ecc(region, "POST", "/servers/"+strv(servers[idx]["id"])+"/reset-root-password", map[string]any{})
	if err != nil {
		r.Show(chat, mid, "❌ خطا: "+Esc(err.Error()), back)
		return
	}
	if msg := strv(d["message"]); msg == "Unauthenticated." {
		r.Show(chat, mid, "❌ کلید دسترسی این عمل را ندارد.", back)
		return
	}
	pw := ""
	if one := arvan.EccOne(d); one != nil {
		pw = firstOf(one["password"], firstOf(one["root_password"], ""))
	}
	msg := "✅ درخواست ریست رمز root ارسال شد.\n(اگر سرور خاموش باشد اول روشنش کن.)"
	if pw != "" {
		msg = "🔑 رمز root جدید:\n<code>" + Esc(pw) + "</code>\n\n⚠️ یادداشت کن؛ دوباره نمایش داده نمی‌شود."
	}
	r.Show(chat, mid, msg, back)
}

func (r *Router) arvAction(chat, mid int64, i, idx int, action string, body any, okMsg string) {
	cli, ok := r.arvCli(i)
	if !ok {
		r.Show(chat, mid, "❌ اکانت پیدا نشد.", HomeKB())
		return
	}
	sess := r.arvSess(chat)
	region := strv(sess["region"])
	servers := asMapList(sess["servers"])
	if idx < 0 || idx >= len(servers) {
		r.Show(chat, mid, "⏳ نشست منقضی شد.", navKB("arvsrv"))
		return
	}
	sid := strv(servers[idx]["id"])
	d, err := cli.Ecc(region, "POST", "/servers/"+sid+"/"+action, body)
	if err != nil {
		r.Show(chat, mid, "❌ خطا: "+Esc(err.Error()), navKB(fmt.Sprintf("arvs:%d:%d", i, idx)))
		return
	}
	if msg := strv(d["message"]); msg == "Unauthenticated." {
		r.Show(chat, mid, "❌ کلید دسترسی این عمل را ندارد.", navKB(fmt.Sprintf("arvs:%d:%d", i, idx)))
		return
	}
	r.Show(chat, mid, okMsg, [][]Btn{{btn("🔄 تازه‌سازی", fmt.Sprintf("arvs:%d:%d", i, idx)), btn("🏠 خانه", "menu")}})
}

func (r *Router) arvSnapList(chat, mid int64, i, idx int) {
	cli, ok := r.arvCli(i)
	if !ok {
		r.Show(chat, mid, "❌ اکانت پیدا نشد.", HomeKB())
		return
	}
	region := strv(r.arvSess(chat)["region"])
	arr := cli.EccList(region, "/snapshots")
	if len(arr) == 0 {
		r.Show(chat, mid, "📭 اسنپ‌شاتی نیست.",
			navKB(fmt.Sprintf("arvs:%d:%d", i, idx)))
		return
	}
	var kb [][]Btn
	for j, sn := range arr {
		if j >= 10 {
			break
		}
		nm := firstOf(sn["name"], strv(sn["id"]))
		kb = append(kb, []Btn{
			btnS("📷 "+truncRune(nm, 24), "noop", "plain"),
			btnS("🗑", fmt.Sprintf("arvsnapdel:%d:%d:%s", i, idx, strv(sn["id"])), "danger")})
	}
	kb = append(kb, []Btn{btn("🔙 بازگشت", fmt.Sprintf("arvs:%d:%d", i, idx))})
	r.Show(chat, mid, fmt.Sprintf("📷 اسنپ‌شات‌ها (%d)", len(arr)), kb)
}

func (r *Router) arvSnapDel(chat, mid int64, i, idx int, snapID string) {
	cli, ok := r.arvCli(i)
	if !ok {
		r.Show(chat, mid, "❌ اکانت پیدا نشد.", HomeKB())
		return
	}
	region := strv(r.arvSess(chat)["region"])
	if _, err := cli.Ecc(region, "DELETE", "/snapshots/"+snapID, nil); err != nil {
		r.Show(chat, mid, "❌ خطا: "+Esc(err.Error()), navKB(fmt.Sprintf("arvs:%d:%d", i, idx)))
		return
	}
	r.Show(chat, mid, "✅ درخواست حذف اسنپ‌شات ارسال شد.",
		[][]Btn{{btn("📷 اسنپ‌شات‌ها", fmt.Sprintf("arvsnapls:%d:%d", i, idx)), btn("🏠 خانه", "menu")}})
}

func (r *Router) arvDelServer(chat, mid int64, i, idx int) {
	cli, ok := r.arvCli(i)
	if !ok {
		r.Show(chat, mid, "❌ اکانت پیدا نشد.", HomeKB())
		return
	}
	sess := r.arvSess(chat)
	region := strv(sess["region"])
	servers := asMapList(sess["servers"])
	if idx < 0 || idx >= len(servers) {
		r.Show(chat, mid, "⏳ نشست منقضی شد.", navKB("arvsrv"))
		return
	}
	sid := strv(servers[idx]["id"])
	if _, err := cli.Ecc(region, "DELETE", "/servers/"+sid, nil); err != nil {
		r.Show(chat, mid, "❌ خطا: "+Esc(err.Error()), navKB(fmt.Sprintf("arvs:%d:%d", i, idx)))
		return
	}
	r.Bot.Send(chat, "✅ سرور حذف شد.", nil)
	r.arvServerList(chat, 0, i, region)
}

// ---------- wizards ----------

func (r *Router) arvRegionStart(chat, mid int64, acc string) {
	r.setWizard(chat, map[string]any{"kind": "arvan_region", "acc": acc})
	r.Show(chat, mid, "⌨️ کد ریجن را بفرست (مثل <code>ir-thr-c2</code>):",
		[][]Btn{{btn("⬅️ انصراف", "arvsrv")}})
}

func (r *Router) arvRegionSave(chat, mid int64, pend map[string]any, text string) {
	r.clearWizard(chat)
	region := strings.TrimSpace(text)
	if region == "" {
		r.Show(chat, mid, "❌ کد ریجن نامعتبر.", navKB("arvsrv"))
		return
	}
	r.arvServerList(chat, mid, intOrAny(pend["acc"]), region)
}

func (r *Router) arvSnapStart(chat, mid int64, acc, idx string) {
	r.setWizard(chat, map[string]any{"kind": "arvan_srv_snap", "acc": acc, "idx": idx})
	r.Show(chat, mid, "📷 نام اسنپ‌شات را بفرست:",
		[][]Btn{{btn("⬅️ انصراف", fmt.Sprintf("arvs:%s:%s", acc, idx))}})
}

func (r *Router) arvSnapSave(chat, mid int64, pend map[string]any, text string) {
	r.clearWizard(chat)
	name := strings.TrimSpace(text)
	if name == "" {
		return
	}
	i := intOrAny(pend["acc"])
	idx := intOrAny(pend["idx"])
	r.arvAction(chat, mid, i, idx, "snapshot", map[string]any{"name": name},
		fmt.Sprintf("✅ اسنپ‌شات «%s» ساخته شد (ممکن است چند دقیقه طول بکشد).", Esc(name)))
}

func (r *Router) arvRenameStart(chat, mid int64, acc, idx string) {
	r.setWizard(chat, map[string]any{"kind": "arvan_srv_rename", "acc": acc, "idx": idx})
	r.Show(chat, mid, "✏️ نام جدید سرور را بفرست:",
		[][]Btn{{btn("⬅️ انصراف", fmt.Sprintf("arvs:%s:%s", acc, idx))}})
}

func (r *Router) arvRenameSave(chat, mid int64, pend map[string]any, text string) {
	r.clearWizard(chat)
	name := strings.TrimSpace(text)
	if name == "" {
		return
	}
	i := intOrAny(pend["acc"])
	idx := intOrAny(pend["idx"])
	r.arvAction(chat, mid, i, idx, "rename", map[string]any{"name": name},
		fmt.Sprintf("✅ نام به «%s» تغییر کرد.", Esc(name)))
}
