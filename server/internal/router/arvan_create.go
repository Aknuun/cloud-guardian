package router

import (
	"fmt"
	"strings"
	"time"

	"github.com/aknuun/cg-server/internal/arvan"
	"github.com/aknuun/cg-server/internal/hostfilter"
)

// ---------- Arvan ECC server creation wizard ----------

func (r *Router) arvMkLoad(chat int64) map[string]any {
	var m map[string]any
	if r.Store.GetJSON(fmt.Sprintf("arvmk:%d", chat), &m) {
		return m
	}
	return nil
}

func (r *Router) arvMkSave(chat int64, m map[string]any) {
	r.Store.PutJSON(fmt.Sprintf("arvmk:%d", chat), m, 3600)
}

func (r *Router) arvMkClear(chat int64) {
	r.Store.Delete(fmt.Sprintf("arvmk:%d", chat))
}

func (r *Router) arvMkExpired(chat, mid int64) {
	r.Show(chat, mid, "⏳ نشست منقضی شد؛ از منو دوباره وارد شو.",
		[][]Btn{{btn("🔙 بازگشت", "arvsrv"), btn("🏠 خانه", "menu")}})
}

func arvMkBack(i int, region string) [][]Btn {
	return [][]Btn{{btn("⬅️ انصراف", fmt.Sprintf("arvreg:%d:%s", i, region))}}
}

// arvMkStart begins the wizard after the user taps "ساخت سرور جدید".
func (r *Router) arvMkStart(chat, mid int64, i int, region string) {
	if i < 0 {
		r.arvMkExpired(chat, mid)
		return
	}
	r.arvMkSave(chat, map[string]any{"acc": i, "region": region, "firewall": "Default"})
	r.setWizard(chat, map[string]any{"kind": "arvan_mk_name", "acc": i, "region": region})
	kb := [][]Btn{{btn("🎲 نام خودکار", "arvmkauto")}}
	kb = append(kb, arvMkBack(i, region)...)
	r.Show(chat, mid,
		fmt.Sprintf("➕ ساخت سرور جدید\n📍 %s\n\n📛 نام سرور را بفرست:\n"+
			"• فقط حروف/اعداد انگلیسی، خط تیره و نقطه\n• ۱ تا ۶۰ کاراکتر\n• مثال: <code>my-server-01</code>",
			Esc(r.arvRegionName(region))),
		kb)
}

// arvSanitizeName keeps only characters accepted by the Arvan API.
func arvSanitizeName(s string) string {
	var b strings.Builder
	for _, r := range strings.TrimSpace(s) {
		switch {
		case r >= 'a' && r <= 'z', r >= 'A' && r <= 'Z', r >= '0' && r <= '9', r == '-':
			b.WriteRune(r)
		case r == ' ' || r == '\t':
			b.WriteRune('-')
		}
	}
	return strings.Trim(b.String(), "-._")
}

func arvAutoName() string {
	return fmt.Sprintf("cg-%d", time.Now().UnixNano()%100000000)
}

func (r *Router) arvMkNameSave(chat, mid int64, text string) {
	name := arvSanitizeName(text)
	sess := r.arvMkLoad(chat)
	if sess == nil {
		r.arvMkExpired(chat, mid)
		return
	}
	i := intOrAny(sess["acc"])
	region := strv(sess["region"])
	if name == "" {
		name = arvAutoName()
	}
	if len([]rune(name)) > 60 {
		name = string([]rune(name)[:60])
	}
	if name == "" {
		r.Show(chat, mid, "❌ نام نامعتبر. دوباره بفرست (مثال: <code>my-server-01</code>):", arvMkBack(i, region))
		return
	}
	sess["name"] = name
	r.arvMkSave(chat, sess)
	r.clearWizard(chat)
	r.arvMkImages(chat, mid)
}

func (r *Router) arvImages(i int, region string) []any {
	var out []any
	seen := map[string]bool{}
	add := func(m map[string]any, prefix string) {
		if len(out) >= 60 {
			return
		}
		id := strv(m["id"])
		if id == "" || seen[id] {
			return
		}
		seen[id] = true
		nm := strings.TrimSpace(prefix + " " + firstOf(m["name"], firstOf(m["distribution"], id)))
		out = append(out, map[string]any{"id": id, "name": nm})
	}
	{
		for _, g := range r.arvImageGroups(i, region) {
			gname := strv(g["name"])
			for _, im := range asMapList(g["images"]) {
				add(im, gname)
			}
		}
	}
	return out
}

func (r *Router) arvMkImages(chat, mid int64) {
	sess := r.arvMkLoad(chat)
	if sess == nil {
		r.arvMkExpired(chat, mid)
		return
	}
	i := intOrAny(sess["acc"])
	region := strv(sess["region"])
	if _, ok := r.arvCli(i); !ok {
		r.arvMkExpired(chat, mid)
		return
	}
	r.Show(chat, mid, "⏳ در حال بارگذاری ایمیج‌ها…", nil)
	imgs := r.arvImages(i, region)
	sess["images"] = imgs
	delete(sess, "imageId")
	delete(sess, "imageName")
	r.arvMkSave(chat, sess)
	var kb [][]Btn
	for j, x := range imgs {
		if j >= 40 {
			break
		}
		m := x.(map[string]any)
		kb = append(kb, []Btn{btn("💿 "+truncRune(strv(m["name"]), 48), fmt.Sprintf("arvimg:%d", j))})
	}
	kb = append(kb, []Btn{btn("⌨️ شناسهٔ ایمیج دستی", "arvimgman")})
	kb = append(kb, arvMkBack(i, region)...)
	r.Show(chat, mid,
		fmt.Sprintf("➕ <code>%s</code>\n📍 %s\n\n💿 ایمیج را انتخاب کن:", Esc(strv(sess["name"])), Esc(r.arvRegionName(region))), kb)
}

func (r *Router) arvMkPickImage(chat, mid int64, j int) {
	sess := r.arvMkLoad(chat)
	if sess == nil {
		r.arvMkExpired(chat, mid)
		return
	}
	i := intOrAny(sess["acc"])
	region := strv(sess["region"])
	imgs := asMapList(sess["images"])
	if j < 0 || j >= len(imgs) {
		r.Show(chat, mid, "❌ ایمیج پیدا نشد.", arvMkBack(i, region))
		return
	}
	sess["imageId"] = strv(imgs[j]["id"])
	sess["imageName"] = strv(imgs[j]["name"])
	r.arvMkSave(chat, sess)
	r.arvMkFlavors(chat, mid)
}

func (r *Router) arvMkImageManual(chat, mid int64) {
	sess := r.arvMkLoad(chat)
	if sess == nil {
		r.arvMkExpired(chat, mid)
		return
	}
	i := intOrAny(sess["acc"])
	region := strv(sess["region"])
	r.setWizard(chat, map[string]any{"kind": "arvan_mk_imgid", "acc": i, "region": region})
	r.Show(chat, mid, "⌨️ شناسهٔ ایمیج (UUID) را بفرست:", arvMkBack(i, region))
}

func (r *Router) arvMkImageIDSave(chat, mid int64, text string) {
	id := strings.TrimSpace(text)
	sess := r.arvMkLoad(chat)
	if sess == nil {
		r.arvMkExpired(chat, mid)
		return
	}
	i := intOrAny(sess["acc"])
	region := strv(sess["region"])
	if id == "" {
		r.Show(chat, mid, "❌ شناسهٔ نامعتبر.", arvMkBack(i, region))
		return
	}
	sess["imageId"] = id
	sess["imageName"] = "دستی: " + id
	r.arvMkSave(chat, sess)
	r.clearWizard(chat)
	r.arvMkFlavors(chat, mid)
}

func (r *Router) arvMkFlavors(chat, mid int64) {
	sess := r.arvMkLoad(chat)
	if sess == nil {
		r.arvMkExpired(chat, mid)
		return
	}
	i := intOrAny(sess["acc"])
	region := strv(sess["region"])
	if _, ok := r.arvCli(i); !ok {
		r.arvMkExpired(chat, mid)
		return
	}
	r.Show(chat, mid, "⏳ در حال بارگذاری پلن‌ها…", nil)
	var flavors []any
	for _, sz := range r.arvSizes(i, region) {
		if sz.ID == "" {
			continue
		}
		flavors = append(flavors, map[string]any{
			"id":    sz.ID,
			"name":  sz.Name,
			"cpu":   sz.CPU,
			"ram":   sz.RAM,
			"disk":  sz.Disk,
			"price": sz.Price,
		})
	}
	sess["flavors"] = flavors
	delete(sess, "flavorId")
	delete(sess, "flavorName")
	delete(sess, "flavorPrice")
	r.arvMkSave(chat, sess)
	var kb [][]Btn
	for j, x := range flavors {
		if j >= 40 {
			break
		}
		m := x.(map[string]any)
		var sp []string
		if c := intOrAny(m["cpu"]); c > 0 {
			sp = append(sp, fmt.Sprintf("%d vCPU", c))
		}
		if m2 := intOrAny(m["ram"]); m2 > 0 {
			sp = append(sp, fmt.Sprintf("%dGB", m2))
		}
		if d := intOrAny(m["disk"]); d > 0 {
			sp = append(sp, fmt.Sprintf("%dGB", d))
		}
		label := strings.Join(sp, " · ")
		if label == "" {
			label = "📦 پلن"
		}
		if pr := arvPriceShort(m["price"]); pr != "" {
			label += " · " + pr
		}
		kb = append(kb, []Btn{btn(truncRune(label, 55), fmt.Sprintf("arvflv:%d", j))})
	}
	if len(kb) == 0 {
		kb = append(kb, []Btn{btn("🔄 تلاش دوباره", "arvflvretry")})
	}
	kb = append(kb, arvMkBack(i, region)...)
	r.Show(chat, mid, "📦 پلن (سایز) را انتخاب کن:", kb)
}

func (r *Router) arvMkPickFlavor(chat, mid int64, j int) {
	sess := r.arvMkLoad(chat)
	if sess == nil {
		r.arvMkExpired(chat, mid)
		return
	}
	i := intOrAny(sess["acc"])
	region := strv(sess["region"])
	fl := asMapList(sess["flavors"])
	if j < 0 || j >= len(fl) {
		r.Show(chat, mid, "❌ پلن پیدا نشد.", arvMkBack(i, region))
		return
	}
	sess["flavorId"] = strv(fl[j]["id"])
	sess["flavorName"] = strv(fl[j]["name"])
	sess["flavorPrice"] = strv(fl[j]["price"])
	r.arvMkSave(chat, sess)
	r.arvMkNetworks(chat, mid)
}

func (r *Router) arvMkNetworks(chat, mid int64) {
	sess := r.arvMkLoad(chat)
	if sess == nil {
		r.arvMkExpired(chat, mid)
		return
	}
	i := intOrAny(sess["acc"])
	region := strv(sess["region"])
	cli, ok := r.arvCli(i)
	if !ok {
		r.arvMkExpired(chat, mid)
		return
	}
	if ids := r.arvNetworkIDs(i, region); len(ids) > 0 {
		sess["networkId"] = ids[0]
		sess["networkName"] = "شبکهٔ پیش‌فرض"
		r.arvMkSave(chat, sess)
		r.arvMkSsh(chat, mid)
		return
	}
	r.Show(chat, mid, "⏳ در حال بارگذاری شبکه‌ها…", nil)
	var nets []any
	for _, m := range cli.EccList(region, "/networks") {
		id := strv(m["id"])
		if id == "" {
			continue
		}
		nm := firstOf(m["name"], id)
		if strings.HasPrefix(strings.ToLower(nm), "public") {
			continue
		}
		nets = append(nets, map[string]any{"id": id, "name": nm})
	}
	sess["networks"] = nets
	delete(sess, "networkId")
	delete(sess, "networkName")
	r.arvMkSave(chat, sess)
	if len(nets) == 0 {
		sess["networkId"] = ""
		sess["networkName"] = "پیش‌فرض"
		r.arvMkSave(chat, sess)
		r.arvMkSsh(chat, mid)
		return
	}
	var kb [][]Btn
	for j, x := range nets {
		m := x.(map[string]any)
		kb = append(kb, []Btn{btn("🌐 "+truncRune(strv(m["name"]), 45), fmt.Sprintf("arvnet:%d", j))})
	}
	kb = append(kb, []Btn{btn("🌐 شبکهٔ پیش‌فرض", "arvnetdef")})
	kb = append(kb, arvMkBack(i, region)...)
	r.Show(chat, mid, "🌐 شبکه را انتخاب کن:", kb)
}

func (r *Router) arvMkPickNetwork(chat, mid int64, j int) {
	sess := r.arvMkLoad(chat)
	if sess == nil {
		r.arvMkExpired(chat, mid)
		return
	}
	nets := asMapList(sess["networks"])
	if j < 0 || j >= len(nets) {
		r.Show(chat, mid, "❌ شبکه پیدا نشد.",
			[][]Btn{{btn("🌐 شبکهٔ پیش‌فرض", "arvnetdef")}})
		return
	}
	sess["networkId"] = strv(nets[j]["id"])
	sess["networkName"] = strv(nets[j]["name"])
	r.arvMkSave(chat, sess)
	r.arvMkSsh(chat, mid)
}

func (r *Router) arvMkSkipNetwork(chat, mid int64) {
	sess := r.arvMkLoad(chat)
	if sess == nil {
		r.arvMkExpired(chat, mid)
		return
	}
	sess["networkId"] = ""
	sess["networkName"] = "پیش‌فرض"
	r.arvMkSave(chat, sess)
	r.arvMkSsh(chat, mid)
}

func (r *Router) arvMkSsh(chat, mid int64) {
	sess := r.arvMkLoad(chat)
	if sess == nil {
		r.arvMkExpired(chat, mid)
		return
	}
	i := intOrAny(sess["acc"])
	region := strv(sess["region"])
	cli, ok := r.arvCli(i)
	if !ok {
		r.arvMkExpired(chat, mid)
		return
	}
	r.Show(chat, mid, "⏳ در حال بارگذاری کلیدهای SSH…", nil)
	var keys []any
	for _, m := range cli.EccList(region, "/ssh-keys") {
		nm := firstOf(m["name"], strv(m["id"]))
		if nm == "" {
			continue
		}
		keys = append(keys, map[string]any{"id": strv(m["id"]), "name": nm})
	}
	sess["sshKeys"] = keys
	delete(sess, "sshKey")
	r.arvMkSave(chat, sess)
	if len(keys) == 0 {
		r.arvMkConfirm(chat, mid)
		return
	}
	var kb [][]Btn
	for j, x := range keys {
		m := x.(map[string]any)
		kb = append(kb, []Btn{btn("🔑 "+truncRune(strv(m["name"]), 45), fmt.Sprintf("arvsshk:%d", j))})
	}
	kb = append(kb, []Btn{btn("⌨️ نام کلید دستی", "arvsshman")})
	kb = append(kb, []Btn{btn("⏭ بدون کلید", "arvsshskip")})
	kb = append(kb, arvMkBack(i, region)...)
	r.Show(chat, mid, "🔑 کلید SSH (اختیاری):", kb)
}

func (r *Router) arvMkPickSsh(chat, mid int64, j int) {
	sess := r.arvMkLoad(chat)
	if sess == nil {
		r.arvMkExpired(chat, mid)
		return
	}
	keys := asMapList(sess["sshKeys"])
	if j < 0 || j >= len(keys) {
		r.Show(chat, mid, "❌ کلید پیدا نشد.", [][]Btn{{btn("⏭ بدون کلید", "arvsshskip")}})
		return
	}
	sess["sshKey"] = strv(keys[j]["name"])
	r.arvMkSave(chat, sess)
	r.arvMkConfirm(chat, mid)
}

func (r *Router) arvMkSshManual(chat, mid int64) {
	sess := r.arvMkLoad(chat)
	if sess == nil {
		r.arvMkExpired(chat, mid)
		return
	}
	i := intOrAny(sess["acc"])
	region := strv(sess["region"])
	r.setWizard(chat, map[string]any{"kind": "arvan_mk_sshn", "acc": i, "region": region})
	r.Show(chat, mid, "⌨️ نام کلید SSH را بفرست (یا «-» برای رد شدن):",
		[][]Btn{{btn("⏭ رد شدن", "arvsshskip")}, {btn("⬅️ انصراف", fmt.Sprintf("arvreg:%d:%s", i, region))}})
}

func (r *Router) arvMkSshNameSave(chat, mid int64, text string) {
	name := strings.TrimSpace(text)
	sess := r.arvMkLoad(chat)
	if sess == nil {
		r.arvMkExpired(chat, mid)
		return
	}
	r.clearWizard(chat)
	if name == "-" {
		name = ""
	}
	sess["sshKey"] = name
	r.arvMkSave(chat, sess)
	r.arvMkConfirm(chat, mid)
}

func (r *Router) arvMkConfirm(chat, mid int64) {
	sess := r.arvMkLoad(chat)
	if sess == nil {
		r.arvMkExpired(chat, mid)
		return
	}
	i := intOrAny(sess["acc"])
	region := strv(sess["region"])
	price := ""
	if p := arvToman(sess["flavorPrice"]); p != "" {
		price = "\n💰 " + Esc(p)
	}
	net := firstOf(strv(sess["networkName"]), "پیش‌فرض")
	ssh := firstOf(strv(sess["sshKey"]), "—")
	fw := firstOf(strv(sess["firewall"]), "Default")
	txt := fmt.Sprintf("➕ تأیید ساخت سرور\n\n"+
		"📍 لوکیشن: <code>%s</code>\n📛 نام: <code>%s</code>\n💿 ایمیج: <code>%s</code>\n"+
		"📦 پلن: <code>%s</code>%s\n🌐 شبکه: <code>%s</code>\n🔑 کلید: <code>%s</code>\n🛡 فایروال: <code>%s</code>\n\n"+
		"⚠️ ساخت سرور هزینه دارد. مطمئنی؟",
		Esc(r.arvRegionName(region)), Esc(strv(sess["name"])), Esc(strv(sess["imageName"])),
		Esc(strv(sess["flavorName"])), price, Esc(net), Esc(ssh), Esc(fw))
	r.Show(chat, mid, txt, [][]Btn{
		{btn("🛡 تغییر فایروال", "arvfwman")},
		{btnS("✅ بله، بساز", "arvcreate", "success"), btn("❌ انصراف", fmt.Sprintf("arvreg:%d:%s", i, region))},
	})
}

func firstPubIP(s map[string]any) string {
	if arr := selStrings(s["pub"]); len(arr) > 0 {
		return arr[0]
	}
	return ""
}

func (r *Router) arvMkFwStart(chat, mid int64) {
	sess := r.arvMkLoad(chat)
	if sess == nil {
		r.arvMkExpired(chat, mid)
		return
	}
	i := intOrAny(sess["acc"])
	region := strv(sess["region"])
	r.setWizard(chat, map[string]any{"kind": "arvan_mk_fw", "acc": i, "region": region})
	r.Show(chat, mid, "🛡 نام گروه فایروال را بفرست.\n\n"+
		"در پنل آروان: سرور ابری ← فایروال ← ساخت گروه. نام همان گروه را اینجا بفرست.\n"+
		"پیش‌فرض: <code>Default</code>",
		[][]Btn{{btn("⬅️ انصراف", fmt.Sprintf("arvreg:%d:%s", i, region))}})
}

func (r *Router) arvMkFwSave(chat, mid int64, text string) {
	name := strings.TrimSpace(text)
	sess := r.arvMkLoad(chat)
	if sess == nil {
		r.arvMkExpired(chat, mid)
		return
	}
	r.clearWizard(chat)
	if name == "" {
		name = "Default"
	}
	sess["firewall"] = name
	r.arvMkSave(chat, sess)
	r.arvMkConfirm(chat, mid)
}

func (r *Router) arvMkCreate(chat, mid int64) {
	sess := r.arvMkLoad(chat)
	if sess == nil {
		r.arvMkExpired(chat, mid)
		return
	}
	i := intOrAny(sess["acc"])
	region := strv(sess["region"])
	name := strv(sess["name"])
	if _, ok := r.arvCli(i); !ok {
		r.arvMkExpired(chat, mid)
		return
	}
	fw := firstOf(strv(sess["firewall"]), "Default")
	flavorID := strv(sess["flavorId"])
	imageID := strv(sess["imageId"])
	disk := 0
	for _, sz := range r.arvSizes(i, region) {
		if sz.ID == flavorID {
			disk = sz.Disk
			break
		}
	}
	r.Show(chat, mid, fmt.Sprintf("⏳ در حال ساخت سرور «%s»…", Esc(name)), nil)
	d, err := r.arvCreateServer(i, region, name, flavorID, imageID, fw, disk)
	if err != nil {
		if msg := arvCreditErr(err.Error()); msg != "" {
			r.Show(chat, mid, msg, arvMkBack(i, region))
			return
		}
		low := strings.ToLower(err.Error())
		if strings.Contains(low, "firewall") {
			r.Show(chat, mid, "❌ آروان بدون «گروه فایروال» سرور نمی‌سازد.\n\n"+
				"در پنل آروان: سرور ابری ← فایروال ← ساخت گروه، یک گروه بساز و نامش را با دکمهٔ «🛡 تغییر فایروال» اینجا وارد کن.",
				arvMkBack(i, region))
			return
		}
		r.Show(chat, mid, "❌ خطا: "+Esc(err.Error()), arvMkBack(i, region))
		return
	}
	if msg := strv(d["message"]); msg == "Unauthenticated." {
		r.Show(chat, mid, "❌ کلید دسترسی این عمل را ندارد.", arvMkBack(i, region))
		return
	}
	one := arvan.EccOne(d)
	pwd := ""
	if one != nil {
		pwd = firstOf(one["password"], firstOf(one["root_password"], ""))
	}
	msg := fmt.Sprintf("⏳ درخواست ساخت «%s» ارسال شد.\nممکن است چند دقیقه طول بکشد؛ نتیجه را همین‌جا می‌فرستم.", Esc(name))
	if pwd != "" {
		msg += fmt.Sprintf("\n\n🔑 رمز root:\n<code>%s</code>\n⚠️ یادداشت کن؛ دوباره نمایش داده نمی‌شود.", Esc(pwd))
	}
	r.arvMkClear(chat)
	r.Show(chat, mid, msg, [][]Btn{{btn("🖥 سرورها", fmt.Sprintf("arvreg:%d:%s", i, region))}})
	go r.arvMkWatch(chat, i, region, name, pwd)
}

// arvMkWatch polls for the newly created server and, once it is up, sends a
// message with its specs, root password and an IP-cleanliness result.
func (r *Router) arvMkWatch(chat int64, i int, region, name, pwd string) {
	defer func() { recover() }()
	time.Sleep(25 * time.Second)
	cli, ok := r.arvCli(i)
	if !ok {
		return
	}
	var srv map[string]any
	for attempt := 0; attempt < 14; attempt++ {
		for _, s := range cli.EccList(region, "/servers") {
			if strv(s["name"]) == name {
				srv = s
				break
			}
		}
		if srv != nil {
			st := strings.ToLower(firstOf(srv["status"], strv(srv["state"])))
			if strings.Contains(st, "active") || strings.Contains(st, "run") {
				break
			}
		}
		time.Sleep(15 * time.Second)
	}
	if srv == nil {
		r.Bot.Send(chat, fmt.Sprintf("⏳ سرور «%s» هنوز در لیست نیست؛ بعداً صفحهٔ سرورها را چک کن.", Esc(name)),
			[][]Btn{{btn("🖥 سرورها", fmt.Sprintf("arvreg:%d:%s", i, region))}})
		return
	}
	n := arvNormalize(srv)
	b := strings.Builder{}
	b.WriteString(fmt.Sprintf("✅ سرور «%s» آماده شد\n\n", Esc(name)))
	b.WriteString(r.arvServerText(n, region))
	if pwd != "" {
		b.WriteString(fmt.Sprintf("\n🔑 رمز root: <code>%s</code>", Esc(pwd)))
	}
	kb := [][]Btn{}
	if ip := firstPubIP(n); ip != "" {
		clean, verdict := hostfilter.CheckIPCleanliness(ip)
		_ = clean
		b.WriteString(fmt.Sprintf("\n\n🧪 چک تمیزی آیپی <code>%s</code>:\n%s", Esc(ip), Esc(verdict)))
		kb = append(kb, row(copyBtnL("📋 کپی آی‌پی", ip)))
	}
	if pwd != "" {
		kb = append(kb, row(copyBtnL("📋 کپی رمز", pwd)))
	}
	kb = append(kb, row(btn("🖥 سرورها", fmt.Sprintf("arvreg:%d:%s", i, region))))
	r.Bot.Send(chat, b.String(), kb)
}
