package router

import (
	"fmt"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/aknuun/cg-server/internal/hz"
)

// ---------- clean IPv4 pool (Hetzner primary IPs) ----------

type hzPoolIP struct {
	ID       int
	IP       string
	Loc      string
	Assignee int
	Auto     bool
	Check    hz.CleanResult
	HasCheck bool
}

func (r *Router) hzChecks(i int) map[string]hz.CleanResult {
	var m map[string]hz.CleanResult
	if !r.Store.GetJSON(fmt.Sprintf("hzipchk:%d", i), &m) || m == nil {
		m = map[string]hz.CleanResult{}
	}
	return m
}

func (r *Router) hzSaveChecks(i int, m map[string]hz.CleanResult) {
	r.Store.PutJSON(fmt.Sprintf("hzipchk:%d", i), m, 0)
}

func (r *Router) hzPoolList(cli *hz.Client, i int) []hzPoolIP {
	d, err := cli.Call("/primary_ips", "GET", nil)
	if err != nil {
		return nil
	}
	checks := r.hzChecks(i)
	var out []hzPoolIP
	for _, p := range hzList(d, "primary_ips") {
		if strv(p["type"]) != "ipv4" {
			continue
		}
		loc, _ := p["location"].(map[string]any)
		pid := intOfAny(p["id"])
		pi := hzPoolIP{
			ID:       pid,
			IP:       strv(p["ip"]),
			Loc:      strv(loc["name"]),
			Assignee: intOfAny(p["assignee_id"]),
			Auto:     boolVal(p["auto_delete"]),
		}
		if c, ok := checks[strconv.Itoa(pid)]; ok {
			pi.Check, pi.HasCheck = c, true
		}
		out = append(out, pi)
	}
	return out
}

func (r *Router) hzIPPool(chat, mid int64, i int) {
	accs := r.hzAccs()
	if i < 0 || i >= len(accs) {
		r.Show(chat, mid, "❌ اکانت پیدا نشد.", navKB(""))
		return
	}
	cli := hz.New(strv(accs[i]["token"]))
	ips := r.hzPoolList(cli, i)
	if ips == nil {
		r.Show(chat, mid, "❌ دریافت آی‌پی‌ها ناموفق بود.", navKB(fmt.Sprintf("hzm:%d", i)))
		return
	}
	var clean, unchecked, dirty, used []hzPoolIP
	for _, p := range ips {
		switch {
		case p.Assignee > 0:
			used = append(used, p)
		case p.HasCheck && p.Check.Clean:
			clean = append(clean, p)
		case p.HasCheck && len(p.Check.Listed) > 0:
			dirty = append(dirty, p)
		default:
			unchecked = append(unchecked, p)
		}
	}
	var b strings.Builder
	b.WriteString("🧼 <b>استخر IP تمیز (IPv4)</b>\n\n")
	b.WriteString(fmt.Sprintf("✅ آماده: <b>%d</b>   ⏳ بررسی‌نشده: <b>%d</b>\n❌ آلوده: <b>%d</b>   🔒 متصل: <b>%d</b>\n\n",
		len(clean), len(unchecked), len(dirty), len(used)))
	if len(clean) == 0 {
		b.WriteString("📭 آی‌پی تمیزِ آزاد نداری؛ با «➕ ساخت/افزودن» اضافه کن.\n")
	} else {
		b.WriteString("آی‌پی‌های آماده:\n")
		for _, p := range clean {
			b.WriteString("• <code>" + Esc(p.IP) + "</code> — " + Esc(p.Loc) + "\n")
		}
	}
	kb := [][]Btn{
		{btn("➕ ساخت/افزودن آی‌پی", fmt.Sprintf("hzipadd:%d", i))},
		{btn("🧪 بررسی تمیزی همه", fmt.Sprintf("hzipchk:%d", i)), btn("♻️ تعویض IP یک سرور", fmt.Sprintf("hzipsw:%d", i))},
		{btn("🗑 حذف آی‌پی‌های آلوده", fmt.Sprintf("hzipcl:%d", i)), btn("🗑 حذف آی‌پی دلخواه", fmt.Sprintf("hzipdel:%d", i))},
	}
	kb = append(kb, navRow(fmt.Sprintf("hzm:%d", i)))
	r.Show(chat, mid, b.String(), kb)
}

// Fill: pick a location first (a primary IP is bound to a location).
func (r *Router) hzIPPoolFill(chat, mid int64, i int) {
	accs := r.hzAccs()
	if i < 0 || i >= len(accs) {
		r.Show(chat, mid, "❌ اکانت پیدا نشد.", navKB(""))
		return
	}
	cli := hz.New(strv(accs[i]["token"]))
	d, err := cli.Call("/servers", "GET", nil)
	if err != nil {
		r.Show(chat, mid, "❌ "+Esc(err.Error()), navKB(fmt.Sprintf("hzip:%d", i)))
		return
	}
	seen := map[string]bool{}
	var locs []string
	for _, s := range hzList(d, "servers") {
		loc, _ := s["location"].(map[string]any)
		n := strv(loc["name"])
		if n != "" && !seen[n] {
			seen[n] = true
			locs = append(locs, n)
		}
	}
	if len(locs) == 0 {
		r.Show(chat, mid, "❌ لوکیشنی پیدا نشد.", navKB(fmt.Sprintf("hzip:%d", i)))
		return
	}
	rows := make([]Btn, 0, len(locs))
	for _, l := range locs {
		rows = append(rows, btnS("🌍 "+l, fmt.Sprintf("hzipl:%d:%s", i, l), "plain"))
	}
	kb := gridBTN(rows, 2)
	kb = append(kb, navRow(fmt.Sprintf("hzip:%d", i)))
	r.Show(chat, mid, "🌍 آی‌پی در کدام لوکیشن ساخته شود؟\n(فقط به سرورهای همان لوکیشن قابل اتصال است)", kb)
}

func (r *Router) hzIPPoolFillLoc(chat, mid int64, i int, loc string) {
	kb := gridBTN([]Btn{
		btn("۱ عدد", fmt.Sprintf("hzipan:%d:%s:1", i, loc)),
		btn("۲ عدد", fmt.Sprintf("hzipan:%d:%s:2", i, loc)),
		btn("۳ عدد", fmt.Sprintf("hzipan:%d:%s:3", i, loc)),
		btn("۵ عدد", fmt.Sprintf("hzipan:%d:%s:5", i, loc)),
	}, 4)
	kb = append(kb, navRow(fmt.Sprintf("hzipadd:%d", i)))
	r.Show(chat, mid, fmt.Sprintf("چند IPv4 در «%s» ساخته و بررسی شود؟\n⚠️ هر IPv4 ماهانه هزینهٔ جداگانه دارد.", Esc(loc)), kb)
}

func (r *Router) hzIPPoolFillDo(chat, mid int64, i int, loc string, n int) {
	accs := r.hzAccs()
	if i < 0 || i >= len(accs) {
		r.Show(chat, mid, "❌ اکانت پیدا نشد.", navKB(""))
		return
	}
	if n < 1 {
		n = 1
	}
	if n > 5 {
		n = 5
	}
	cli := hz.New(strv(accs[i]["token"]))
	var created []hzPoolIP
	var failMsg string
	for k := 0; k < n; k++ {
		name := fmt.Sprintf("clean-%d-%d", time.Now().Unix(), k)
		resp, err := cli.Call("/primary_ips", "POST", map[string]any{
			"type": "ipv4", "name": name, "location": loc, "auto_delete": false,
		})
		if err != nil {
			failMsg = err.Error()
			break
		}
		p, _ := resp["primary_ip"].(map[string]any)
		if p == nil {
			continue
		}
		created = append(created, hzPoolIP{ID: intOfAny(p["id"]), IP: strv(p["ip"]), Loc: loc})
	}

	if len(created) > 0 {
		r.Bot.Send(chat, fmt.Sprintf("⏳ %d آی‌پی ساخته شد؛ در حال بررسی تمیزی…", len(created)), nil)
	}
	checks := r.hzChecks(i)
	runChecks(created, checks)
	r.hzSaveChecks(i, checks)

	deleted := 0
	for _, p := range created {
		c := checks[strconv.Itoa(p.ID)]
		if len(c.Listed) > 0 { // definitely dirty → remove to stop billing
			if _, err := cli.Call(fmt.Sprintf("/primary_ips/%d", p.ID), "DELETE", nil); err == nil {
				deleted++
				delete(checks, strconv.Itoa(p.ID))
			}
		}
	}
	r.hzSaveChecks(i, checks)

	var cleanN, pendN int
	for _, p := range created {
		if _, gone := checks[strconv.Itoa(p.ID)]; !gone {
			continue
		}
		if checks[strconv.Itoa(p.ID)].Clean {
			cleanN++
		} else {
			pendN++
		}
	}
	var b strings.Builder
	b.WriteString("🧼 نتیجهٔ ساخت IP\n\n")
	b.WriteString(fmt.Sprintf("ساخته‌شده: %d\n✅ تمیز: %d\n⏳ نامشخص/آلوده: %d\n🗑 حذف‌شده (آلوده): %d\n", len(created), cleanN, pendN, deleted))
	if failMsg != "" {
		b.WriteString("\n⚠️ متوقف شد: " + Esc(failMsg))
	}
	kb := [][]Btn{
		{btn("🔙 استخر IP", fmt.Sprintf("hzip:%d", i)), btn("➕ ساخت بیشتر", fmt.Sprintf("hzipl:%d:%s", i, loc))},
		{btn("🏠 خانه", "menu")},
	}
	r.Show(chat, mid, b.String(), kb)
}

func (r *Router) hzIPPoolRecheck(chat, mid int64, i int) {
	accs := r.hzAccs()
	if i < 0 || i >= len(accs) {
		r.Show(chat, mid, "❌ اکانت پیدا نشد.", navKB(""))
		return
	}
	cli := hz.New(strv(accs[i]["token"]))
	ips := r.hzPoolList(cli, i)
	var targets []hzPoolIP
	for _, p := range ips {
		if p.Assignee == 0 {
			targets = append(targets, p)
		}
	}
	if len(targets) == 0 {
		r.Show(chat, mid, "📭 آی‌پی آزادی برای بررسی نیست.", navKB(fmt.Sprintf("hzip:%d", i)))
		return
	}
	r.loading(chat, mid, fmt.Sprintf("در حال بررسی %d آی‌پی", len(targets)))
	checks := r.hzChecks(i)
	runChecks(targets, checks)
	r.hzSaveChecks(i, checks)
	r.hzIPPool(chat, mid, i)
}

func (r *Router) hzIPPoolSwap(chat, mid int64, i int) {
	accs := r.hzAccs()
	if i < 0 || i >= len(accs) {
		r.Show(chat, mid, "❌ اکانت پیدا نشد.", navKB(""))
		return
	}
	cli := hz.New(strv(accs[i]["token"]))
	d, err := cli.Call("/servers", "GET", nil)
	if err != nil {
		r.Show(chat, mid, "❌ "+Esc(err.Error()), navKB(fmt.Sprintf("hzip:%d", i)))
		return
	}
	var rows []Btn
	for _, s := range hzList(d, "servers") {
		if len(rows) >= 40 {
			break
		}
		label := strv(s["name"])
		if label == "" {
			label = strv(s["id"])
		}
		rows = append(rows, btnS(label+" · "+hzIP(s), fmt.Sprintf("hzipsws:%d:%s", i, strv(s["id"])), "plain"))
	}
	if len(rows) == 0 {
		r.Show(chat, mid, "📭 سروری نیست.", navKB(fmt.Sprintf("hzip:%d", i)))
		return
	}
	kb := gridBTN(rows, 1)
	kb = append(kb, navRow(fmt.Sprintf("hzip:%d", i)))
	r.Show(chat, mid, "♻️ IP کدام سرور عوض شود؟", kb)
}

func (r *Router) hzIPPoolSwapAsk(chat, mid int64, i int, sid string) {
	accs := r.hzAccs()
	if i < 0 || i >= len(accs) {
		r.Show(chat, mid, "❌ اکانت پیدا نشد.", navKB(""))
		return
	}
	cli := hz.New(strv(accs[i]["token"]))
	d, err := cli.Call("/servers/"+sid, "GET", nil)
	if err != nil {
		r.Show(chat, mid, "❌ "+Esc(err.Error()), navKB(fmt.Sprintf("hzipsw:%d", i)))
		return
	}
	s, _ := d["server"].(map[string]any)
	loc, _ := s["location"].(map[string]any)
	locName := strv(loc["name"])
	clean := 0
	for _, p := range r.hzPoolList(cli, i) {
		if p.Assignee == 0 && p.Loc == locName && p.HasCheck && p.Check.Clean {
			clean++
		}
	}
	msg := fmt.Sprintf("♻️ تعویض IPv4 سرور «%s»\n\n📍 لوکیشن: %s\n🌐 IP فعلی: <code>%s</code>\n✅ IP تمیز آماده در این لوکیشن: <b>%d</b>\n\n⚠️ سرور برای تعویض خاموش و دوباره روشن می‌شود (چند ثانیه قطعی).",
		Esc(strv(s["name"])), Esc(locName), Esc(hzIP(s)), clean)
	kb := [][]Btn{}
	if clean > 0 {
		kb = append(kb, []Btn{btn("✅ تعویض کن", fmt.Sprintf("hzipswy:%d:%s", i, sid))})
	} else {
		kb = append(kb, []Btn{btn("➕ ساخت IP تمیز", fmt.Sprintf("hzipadd:%d", i))})
	}
	kb = append(kb, navRow(fmt.Sprintf("hzipsw:%d", i)))
	r.Show(chat, mid, msg, kb)
}

func (r *Router) hzIPPoolSwapDo(chat, mid int64, i int, sid string) {
	accs := r.hzAccs()
	if i < 0 || i >= len(accs) {
		r.Show(chat, mid, "❌ اکانت پیدا نشد.", navKB(""))
		return
	}
	cli := hz.New(strv(accs[i]["token"]))
	d, err := cli.Call("/servers/"+sid, "GET", nil)
	if err != nil {
		r.Show(chat, mid, "❌ "+Esc(err.Error()), navKB(fmt.Sprintf("hzipsw:%d", i)))
		return
	}
	s, _ := d["server"].(map[string]any)
	loc, _ := s["location"].(map[string]any)
	locName := strv(loc["name"])
	pn, _ := s["public_net"].(map[string]any)
	old4, _ := pn["ipv4"].(map[string]any)
	oldID := intOfAny(old4["id"])
	oldIP := strv(old4["ip"])

	var pick *hzPoolIP
	for _, p := range r.hzPoolList(cli, i) {
		if p.Assignee == 0 && p.Loc == locName && p.HasCheck && p.Check.Clean {
			pp := p
			pick = &pp
			break
		}
	}
	if pick == nil {
		r.Show(chat, mid, "❌ IP تمیز آماده در این لوکیشن نیست. اول استخر را پر کن.", [][]Btn{
			{btn("➕ ساخت IP تمیز", fmt.Sprintf("hzipadd:%d", i))},
			navRow(fmt.Sprintf("hzipsw:%d", i)),
		})
		return
	}
	if !hz.EnsureOff(cli, sid, 45000) {
		r.Show(chat, mid, "⚠️ سرور در ۴۵ ثانیه خاموش نشد؛ دوباره تلاش کن.", navKB(fmt.Sprintf("hzipsw:%d", i)))
		return
	}
	if oldID > 0 {
		_, _ = cli.Call(fmt.Sprintf("/primary_ips/%d/actions/unassign", oldID), "POST", map[string]any{})
		if !hzWaitPrimaryState(cli, oldID, 0, 45000) {
			_, _ = cli.Call(fmt.Sprintf("/primary_ips/%d/actions/assign", oldID), "POST", map[string]any{
				"assignee_id": atoiSafe(sid), "assignee_type": "server",
			})
			_, _ = cli.Call("/servers/"+sid+"/actions/poweron", "POST", map[string]any{})
			r.Show(chat, mid, "⚠️ IP قبلی در زمان مقرر جدا نشد؛ دوباره تلاش کن.", navKB(fmt.Sprintf("hzipsw:%d", i)))
			return
		}
	}
	if _, err := cli.Call(fmt.Sprintf("/primary_ips/%d/actions/assign", pick.ID), "POST", map[string]any{
		"assignee_id": atoiSafe(sid), "assignee_type": "server",
	}); err != nil {
		// roll back: put the original IP back so the server keeps connectivity
		hzRestorePrimary(cli, sid, oldID, pick.ID)
		r.Show(chat, mid, "❌ اتصال IP تازه ناموفق بود (IP قبلی برگردانده شد):\n"+Esc(err.Error()), navKB(fmt.Sprintf("hzipsw:%d", i)))
		return
	}
	if !hzWaitPrimaryState(cli, pick.ID, atoiSafe(sid), 45000) || !hzWaitServerPrimary(cli, sid, pick.ID, 45000) {
		hzRestorePrimary(cli, sid, oldID, pick.ID)
		r.Show(chat, mid, "⚠️ IP تازه در زمان مقرر روی سرور فعال نشد؛ IP قبلی برگردانده شد.", navKB(fmt.Sprintf("hzipsw:%d", i)))
		return
	}
	_, _ = cli.Call("/servers/"+sid+"/actions/poweron", "POST", map[string]any{})
	r.hzSyncIP(strv(s["name"]), pick.IP)
	r.hzDropServers(i)
	if oldID > 0 && oldIP != "" {
		go r.hzRecheckOne(i, oldID, oldIP)
	}
	r.Show(chat, mid, fmt.Sprintf("✅ IPv4 سرور «%s» عوض شد.\n\n🌐 IP جدید: <code>%s</code>\n🔙 IP قبلی: <code>%s</code> (بعد از بررسی، اگر پاک بود به استخر برمی‌گردد)",
		Esc(strv(s["name"])), Esc(pick.IP), Esc(oldIP)),
		[][]Btn{
			{copyBtnL("📋 کپی IP جدید", pick.IP)},
			{btn("🧼 استخر IP", fmt.Sprintf("hzip:%d", i)), btn("🖥 سرور", fmt.Sprintf("hzsi:%d:%s", i, sid))},
			{btn("🏠 خانه", "menu")},
		})
}

func (r *Router) hzIPPoolCleanupAsk(chat, mid int64, i int) {
	accs := r.hzAccs()
	if i < 0 || i >= len(accs) {
		r.Show(chat, mid, "❌ اکانت پیدا نشد.", navKB(""))
		return
	}
	cli := hz.New(strv(accs[i]["token"]))
	var junk []hzPoolIP
	for _, p := range r.hzPoolList(cli, i) {
		if p.Assignee == 0 && p.HasCheck && len(p.Check.Listed) > 0 {
			junk = append(junk, p)
		}
	}
	if len(junk) == 0 {
		r.Show(chat, mid, "✅ آی‌پی آلودهٔ آزادی برای حذف نیست.", navKB(fmt.Sprintf("hzip:%d", i)))
		return
	}
	var b strings.Builder
	b.WriteString("🗑 این آی‌پی‌های آلوده و آزاد حذف شوند؟\n\n")
	for _, p := range junk {
		b.WriteString("• <code>" + Esc(p.IP) + "</code> (" + Esc(p.Loc) + ")\n")
	}
	kb := [][]Btn{
		{btn(fmt.Sprintf("✅ حذف %d آی‌پی", len(junk)), fmt.Sprintf("hzipcly:%d", i))},
		navRow(fmt.Sprintf("hzip:%d", i)),
	}
	r.Show(chat, mid, b.String(), kb)
}

func (r *Router) hzIPPoolCleanupDo(chat, mid int64, i int) {
	accs := r.hzAccs()
	if i < 0 || i >= len(accs) {
		r.Show(chat, mid, "❌ اکانت پیدا نشد.", navKB(""))
		return
	}
	cli := hz.New(strv(accs[i]["token"]))
	checks := r.hzChecks(i)
	deleted := 0
	for _, p := range r.hzPoolList(cli, i) {
		if p.Assignee == 0 && p.HasCheck && len(p.Check.Listed) > 0 {
			if _, err := cli.Call(fmt.Sprintf("/primary_ips/%d", p.ID), "DELETE", nil); err == nil {
				delete(checks, strconv.Itoa(p.ID))
				deleted++
			}
		}
	}
	r.hzSaveChecks(i, checks)
	r.hzIPPool(chat, mid, i)
	_ = deleted
}

func (r *Router) hzIPPoolDeleteList(chat, mid int64, i int) {
	accs := r.hzAccs()
	if i < 0 || i >= len(accs) {
		r.Show(chat, mid, "❌ اکانت پیدا نشد.", navKB(""))
		return
	}
	cli := hz.New(strv(accs[i]["token"]))
	var free []hzPoolIP
	for _, p := range r.hzPoolList(cli, i) {
		if p.Assignee == 0 {
			free = append(free, p)
		}
	}
	if len(free) == 0 {
		r.Show(chat, mid, "📭 آی‌پی آزادی برای حذف نیست.", navKB(fmt.Sprintf("hzip:%d", i)))
		return
	}
	var kb [][]Btn
	for _, p := range free {
		mark := "⏳"
		if p.HasCheck && p.Check.Clean {
			mark = "✅"
		} else if p.HasCheck && len(p.Check.Listed) > 0 {
			mark = "❌"
		}
		kb = append(kb, []Btn{btnS(fmt.Sprintf("%s %s · %s", mark, p.IP, truncRune(p.Loc, 20)),
			fmt.Sprintf("hzipdely:%d:%d", i, p.ID), "plain")})
	}
	kb = append(kb, navRow(fmt.Sprintf("hzip:%d", i)))
	r.Show(chat, mid, "🗑 کدام آی‌پی حذف شود؟\n(فقط آی‌پی‌های آزاد قابل حذف‌اند)", kb)
}

func (r *Router) hzIPPoolDeleteAsk(chat, mid int64, i, ipID int) {
	accs := r.hzAccs()
	if i < 0 || i >= len(accs) {
		r.Show(chat, mid, "❌ اکانت پیدا نشد.", navKB(""))
		return
	}
	cli := hz.New(strv(accs[i]["token"]))
	var target *hzPoolIP
	for _, p := range r.hzPoolList(cli, i) {
		if p.ID == ipID {
			pp := p
			target = &pp
			break
		}
	}
	if target == nil {
		r.Show(chat, mid, "❌ آی‌پی پیدا نشد.", navKB(fmt.Sprintf("hzip:%d", i)))
		return
	}
	r.Show(chat, mid, fmt.Sprintf("⚠️ آی‌پی <code>%s</code> (%s) حذف شود؟\nاین عمل بازگشت‌پذیر نیست.",
		Esc(target.IP), Esc(target.Loc)),
		[][]Btn{
			{btnS("🗑 بله، حذف کن", fmt.Sprintf("hzipdeldo:%d:%d", i, ipID), "danger")},
			navRow(fmt.Sprintf("hzipdel:%d", i)),
		})
}

func (r *Router) hzIPPoolDeleteDo(chat, mid int64, i, ipID int) {
	accs := r.hzAccs()
	if i < 0 || i >= len(accs) {
		r.Show(chat, mid, "❌ اکانت پیدا نشد.", navKB(""))
		return
	}
	cli := hz.New(strv(accs[i]["token"]))
	if _, err := cli.Call(fmt.Sprintf("/primary_ips/%d", ipID), "DELETE", nil); err != nil {
		r.Show(chat, mid, "❌ خطا: "+Esc(err.Error()), navKB(fmt.Sprintf("hzip:%d", i)))
		return
	}
	checks := r.hzChecks(i)
	delete(checks, strconv.Itoa(ipID))
	r.hzSaveChecks(i, checks)
	r.Bot.Send(chat, "✅ آی‌پی حذف شد.", nil)
	r.hzIPPool(chat, mid, i)
}

// runChecks fills the map with fresh results for the given IPs (concurrently).
func runChecks(ips []hzPoolIP, out map[string]hz.CleanResult) {
	var wg sync.WaitGroup
	var mu sync.Mutex
	for _, p := range ips {
		wg.Add(1)
		go func(pp hzPoolIP) {
			defer wg.Done()
			c := hz.CheckIPv4(pp.IP)
			mu.Lock()
			out[strconv.Itoa(pp.ID)] = c
			mu.Unlock()
		}(p)
	}
	wg.Wait()
}

func (r *Router) hzRecheckOne(i, ipID int, ip string) {
	c := hz.CheckIPv4(ip)
	checks := r.hzChecks(i)
	checks[strconv.Itoa(ipID)] = c
	r.hzSaveChecks(i, checks)
}

// hzSyncIP updates the saved connection record of a Hetzner server after its
// public IPv4 changed, so SSH shortcuts keep working.
func (r *Router) hzSyncIP(name, ip string) {
	lst := r.srvList()
	changed := false
	for _, rec := range lst {
		if strv(rec["note"]) == "Hetzner: "+name && strv(rec["host"]) != ip {
			rec["host"] = ip
			changed = true
		}
	}
	if changed {
		r.srvSave(lst)
	}
}

func navKB(back string) [][]Btn { return [][]Btn{navRow(back)} }
