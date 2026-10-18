package router

import (
	"fmt"
	"strings"

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

func (r *Router) arvCloudHome(chat, mid int64) {
	accs := r.arvAccounts()
	if len(accs) == 0 {
		r.Show(chat, mid, "🖥 سرورهای ابری آروان\n\nاول یک اکانت آروان اضافه کن.",
			[][]Btn{{btn("👤 اکانت‌ها", "arvanacc")}, {btn("🔙 بازگشت", "arvan"), btn("🏠 خانه", "menu")}})
		return
	}
	r.Show(chat, mid, "⏳ لطفاً صبر کنید… در حال بارگذاری ریجن‌ها", nil)
	kb := [][]Btn{}
	lines := []string{"🖥 سرورهای ابری آروان", "", "یک اکانت و ریجن را انتخاب کن:", ""}
	for i, a := range accs {
		regions := r.arvRegions(i, false)
		lines = append(lines, fmt.Sprintf("👤 %s — %d ریجن", Esc(strv(a["name"])), len(regions)))
		for _, rg := range regions {
			if len(kb) >= 40 {
				break
			}
			label := rg.Fa
			if rg.Def {
				label += " ✅"
			}
			labelshort := truncRune(label, 45)
			btnlabel := truncRune("📍 "+strv(a["name"])+" · "+labelshort, 60)
			kb = append(kb, []Btn{btn(btnlabel, fmt.Sprintf("arvreg:%d:%s", i, rg.Code))})
		}
		kb = append(kb, []Btn{btn("⌨️ ریجن دستی («"+strv(a["name"])+"»)", fmt.Sprintf("arvregcustom:%d", i))})
	}
	kb = append(kb, []Btn{btn("🔄 تازه‌سازی ریجن‌ها", "arvregrefresh")})
	kb = append(kb, []Btn{btn("🔙 بازگشت", "arvan"), btn("🏠 خانه", "menu")})
	r.Show(chat, mid, strings.Join(lines, "\n"), kb)
}

// arvCloudRefresh forces a fresh fetch of every account's regions.
func (r *Router) arvCloudRefresh(chat, mid int64) {
	accs := r.arvAccounts()
	if len(accs) == 0 {
		r.arvCloudHome(chat, mid)
		return
	}
	r.Show(chat, mid, "⏳ در حال تازه‌سازی ریجن‌ها…", nil)
	for i := range accs {
		r.arvRegions(i, true)
	}
	r.arvCloudHome(chat, mid)
}

func arvServerIPs(s map[string]any) (pub, priv []string) {
	addrs := s["addresses"]
	if arr, ok := addrs.([]any); ok {
		for _, x := range arr {
			m, _ := x.(map[string]any)
			ip := firstOf(m["addr"], firstOf(m["ip"], strv(m["address"])))
			if ip == "" {
				continue
			}
			privFlag := false
			if b, ok := m["is_public"].(bool); ok && !b {
				privFlag = true
			}
			if strv(m["type"]) == "private" || boolVal(m["is_vpc"]) {
				privFlag = true
			}
			if privFlag {
				priv = append(priv, ip)
			} else {
				pub = append(pub, ip)
			}
		}
	} else if m, ok := addrs.(map[string]any); ok {
		for _, k := range []string{"public", "private", "vpc"} {
			for _, x := range toList(m[k]) {
				ip := strv(x)
				if mm, ok := x.(map[string]any); ok {
					ip = firstOf(mm["addr"], firstOf(mm["ip"], strv(mm["address"])))
				}
				if ip == "" {
					continue
				}
				if k == "public" {
					pub = append(pub, ip)
				} else {
					priv = append(priv, ip)
				}
			}
		}
	}
	for _, x := range toList(s["networks"]) {
		m, _ := x.(map[string]any)
		ip := firstOf(m["ip"], firstOf(m["addr"], strv(m["address"])))
		if ip == "" || containsStr(pub, ip) || containsStr(priv, ip) {
			continue
		}
		if b, ok := m["public"].(bool); ok && !b {
			priv = append(priv, ip)
		} else {
			pub = append(pub, ip)
		}
	}
	return pub, priv
}

func arvNormalize(s map[string]any) map[string]any {
	if s == nil {
		return nil
	}
	flavor, _ := s["flavor"].(map[string]any)
	image, _ := s["image"].(map[string]any)
	name := firstOf(s["name"], firstOf(s["hostname"], strv(s["id"])))
	fl := ""
	cpu, ram, disk := 0, 0, 0
	if flavor != nil {
		fl = firstOf(flavor["name"], strv(flavor["id"]))
		cpu = intOrAny(flavor["cpu"])
		if cpu == 0 {
			cpu = intOrAny(flavor["cpu_count"])
		}
		ram = intOrAny(flavor["ram"])
		if ram == 0 {
			ram = intOrAny(flavor["memory"])
		}
		disk = intOrAny(flavor["disk"])
	}
	if cpu == 0 {
		cpu = intOrAny(s["cpu"])
	}
	if ram == 0 {
		ram = intOrAny(s["ram"])
	}
	if disk == 0 {
		disk = intOrAny(s["disk"])
	}
	im := ""
	if image != nil {
		im = firstOf(image["name"], strv(image["distribution"]))
	}
	pub, priv := arvServerIPs(s)
	return map[string]any{
		"id":     strv(s["id"]),
		"name":   name,
		"status": firstOf(s["status"], firstOf(s["state"], strv(s["power_state"]))),
		"task":   strv(s["task_status"]),
		"flavor": fl,
		"image":  im,
		"cpu":    cpu,
		"ram":    ram,
		"disk":   disk,
		"region": firstOf(s["region"], strv(s["region_code"])),
		"pub":    pub,
		"priv":   priv,
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

func (r *Router) arvServerText(s map[string]any, region string) string {
	pub := "—"
	if arr := selStrings(s["pub"]); len(arr) > 0 {
		pub = strings.Join(arr, "، ")
	}
	priv := "—"
	if arr := selStrings(s["priv"]); len(arr) > 0 {
		priv = strings.Join(arr, "، ")
	}
	plan := orDash(strv(s["flavor"]))
	specs := ""
	cpu, ram, disk := intOrAny(s["cpu"]), intOrAny(s["ram"]), intOrAny(s["disk"])
	var sp []string
	if cpu > 0 {
		sp = append(sp, fmt.Sprintf("%d vCPU", cpu))
	}
	if ram > 0 {
		sp = append(sp, fmt.Sprintf("%d GB RAM", ram))
	}
	if disk > 0 {
		sp = append(sp, fmt.Sprintf("%d GB دیسک", disk))
	}
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
	loc := r.arvRegionName(region)
	return fmt.Sprintf("🖥 <code>%s</code>\n\n"+
		"📊 وضعیت: <code>%s</code>\n🌐 عمومی: <code>%s</code>\n🔒 خصوصی: <code>%s</code>\n"+
		"📦 پلن: <code>%s</code>%s\n💿 ایمیج: <code>%s</code>\n📍 لوکیشن: <code>%s</code>\n"+
		"🆔 <code>%s</code>",
		Esc(strv(s["name"])), Esc(st), Esc(pub), Esc(priv),
		Esc(plan), Esc(specs), Esc(orDash(strv(s["image"]))), Esc(loc),
		Esc(truncRune(strv(s["id"]), 18)))
}

func (r *Router) arvServerList(chat, mid int64, i int, region string) {
	cli, ok := r.arvCli(i)
	if !ok {
		r.Show(chat, mid, "❌ اکانت پیدا نشد.", HomeKB())
		return
	}
	r.Show(chat, mid, "⏳ لطفاً صبر کنید… در حال بارگذاری سرورها", nil)
	raw := cli.EccList(region, "/servers")
	var servers []any
	for _, s := range raw {
		n := arvNormalize(s)
		if n != nil {
			servers = append(servers, n)
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
		st := strings.ToLower(strv(s["status"]))
		emoji := "⚪"
		if strings.Contains(st, "run") || strings.Contains(st, "active") {
			emoji = "🟢"
		} else if strings.Contains(st, "build") || strings.Contains(st, "creat") || strings.Contains(st, "pending") {
			emoji = "🟡"
		} else if strings.Contains(st, "off") || strings.Contains(st, "stop") || strings.Contains(st, "shut") {
			emoji = "🔴"
		}
		srvItems = append(srvItems, btn(fmt.Sprintf("%s %s", emoji, truncRune(strv(s["name"]), 30)), fmt.Sprintf("arvs:%d:%d", i, j)))
	}
	kb := gridBTN(srvItems, 2)
	kb = append(kb, row(btn("➕ ساخت سرور جدید", fmt.Sprintf("arvnew:%d:%s", i, region))))
	kb = append(kb, row(btn("🔄 تازه‌سازی", fmt.Sprintf("arvreg:%d:%s", i, region))))
	kb = append(kb, navRow("arvsrv"))
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
			[][]Btn{{btn("🔙 بازگشت", "arvsrv"), btn("🏠 خانه", "menu")}})
		return
	}
	s := servers[idx]
	if d, err := cli.Ecc(region, "GET", "/servers/"+strv(s["id"]), nil); err == nil {
		if one := arvan.EccOne(d); one != nil {
			if n := arvNormalize(one); n != nil {
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
		[]Btn{btn("📷 اسنپ‌شات", fmt.Sprintf("arvsnap:%d:%d", i, idx)), btn("📷 اسنپ‌شات‌ها", fmt.Sprintf("arvsnapls:%d:%d", i, idx))},
		[]Btn{btn("✏️ تغییرنام", fmt.Sprintf("arvren:%d:%d", i, idx)), btn("🔑 ریست رمز root", fmt.Sprintf("arvyroot:%d:%d", i, idx))},
		[]Btn{btn("🔄 تازه‌سازی", fmt.Sprintf("arvs:%d:%d", i, idx))},
		[]Btn{btnS("🗑 حذف سرور", fmt.Sprintf("arvterm:%d:%d", i, idx), "danger")},
		[]Btn{btn("🔙 بازگشت", fmt.Sprintf("arvreg:%d:%s", i, region)), btn("🏠 خانه", "menu")})
	r.Show(chat, mid, r.arvServerText(s, region), kb)
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
	back := [][]Btn{{btn("🔙 بازگشت", fmt.Sprintf("arvs:%d:%d", i, idx))}}
	if idx < 0 || idx >= len(servers) {
		r.Show(chat, mid, "⏳ نشست منقضی شد.", [][]Btn{{btn("🔙 بازگشت", "arvsrv")}})
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
		r.Show(chat, mid, "⏳ نشست منقضی شد.", [][]Btn{{btn("🔙 بازگشت", "arvsrv")}})
		return
	}
	sid := strv(servers[idx]["id"])
	d, err := cli.Ecc(region, "POST", "/servers/"+sid+"/"+action, body)
	if err != nil {
		r.Show(chat, mid, "❌ خطا: "+Esc(err.Error()), [][]Btn{{btn("🔙 بازگشت", fmt.Sprintf("arvs:%d:%d", i, idx))}})
		return
	}
	if msg := strv(d["message"]); msg == "Unauthenticated." {
		r.Show(chat, mid, "❌ کلید دسترسی این عمل را ندارد.", [][]Btn{{btn("🔙 بازگشت", fmt.Sprintf("arvs:%d:%d", i, idx))}})
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
			[][]Btn{{btn("🔙 بازگشت", fmt.Sprintf("arvs:%d:%d", i, idx))}})
		return
	}
	var kb [][]Btn
	for j, sn := range arr {
		if j >= 10 {
			break
		}
		nm := firstOf(sn["name"], strv(sn["id"]))
		kb = append(kb, []Btn{
			btn("📷 "+truncRune(nm, 24), "noop"),
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
		r.Show(chat, mid, "❌ خطا: "+Esc(err.Error()), [][]Btn{{btn("🔙 بازگشت", fmt.Sprintf("arvs:%d:%d", i, idx))}})
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
		r.Show(chat, mid, "⏳ نشست منقضی شد.", [][]Btn{{btn("🔙 بازگشت", "arvsrv")}})
		return
	}
	sid := strv(servers[idx]["id"])
	if _, err := cli.Ecc(region, "DELETE", "/servers/"+sid, nil); err != nil {
		r.Show(chat, mid, "❌ خطا: "+Esc(err.Error()), [][]Btn{{btn("🔙 بازگشت", fmt.Sprintf("arvs:%d:%d", i, idx))}})
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
		r.Show(chat, mid, "❌ کد ریجن نامعتبر.", [][]Btn{{btn("🔙 بازگشت", "arvsrv")}})
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
