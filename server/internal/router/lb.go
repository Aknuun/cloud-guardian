package router

import (
	"fmt"
	"net"
	"sort"
	"strconv"
	"strings"

	"github.com/aknuun/cg-server/internal/cf"
	"github.com/aknuun/cg-server/internal/lb"
)

const (
	lbPage       = 20
	lbGroupsPage = 30
)

type lbSess struct {
	Acc  int    `json:"acc"`
	Zid  string `json:"zid"`
	Name string `json:"name"`
	Rid  string `json:"rid"`
}

func (r *Router) lbGetSess(gid string) lbSess {
	var s lbSess
	r.Store.GetJSON("lbg:"+gid, &s)
	return s
}

func (r *Router) lbSetSess(gid string, s lbSess) {
	r.Store.PutJSON("lbg:"+gid, s, 86400)
}

func splitTail(s string) (head, tail string) {
	i := strings.LastIndex(s, ":")
	if i < 0 {
		return s, ""
	}
	return s[:i], s[i+1:]
}

func ttlInt(v any) int {
	switch t := v.(type) {
	case float64:
		return int(t)
	case int:
		return t
	case int64:
		return int(t)
	}
	return 1
}

func (r *Router) lbGroupOf(acc int, zid, name string) (*cf.Client, []cf.Record, lb.Config, string, error) {
	cli, ok := r.cfClientFor(acc)
	if !ok {
		return nil, nil, lb.Config{}, "", fmt.Errorf("❌ دامنه پیدا نشد.")
	}
	recs, err := cli.Records(zid)
	if err != nil {
		return nil, nil, lb.Config{}, "", fmt.Errorf("❌ خطای کلادفلر: %s", Esc(err.Error()))
	}
	var group []cf.Record
	for _, x := range recs {
		if x.Name == name && lb.IsLbRec(x.Type) {
			group = append(group, x)
		}
	}
	key := lb.KeyFor(zid, name)
	return cli, group, lb.GetCfg(r.Store, key), key, nil
}

func (r *Router) lbEntries(s lbSess) []lb.Entry {
	cli, group, cfg, _, err := r.lbGroupOf(s.Acc, s.Zid, s.Name)
	if err != nil || cli == nil {
		return nil
	}
	return lb.EntriesOf(group, cfg)
}

func (r *Router) cfCtx(chat int64, rid string) (int, string) {
	var sess map[string]any
	if !r.Store.GetJSON(fmt.Sprintf("cfctx:%d", chat), &sess) {
		return 0, ""
	}
	if zid, _ := sess["zid"].(string); zid != "" {
		return intOrAny(sess["acc"]), zid
	}
	if results, ok := sess["results"].(map[string]any); ok {
		if res, ok := results[rid].(map[string]any); ok {
			if zid, _ := res["zid"].(string); zid != "" {
				return intOrAny(res["acc"]), zid
			}
		}
	}
	return 0, ""
}

func intOrAny(v any) int {
	switch t := v.(type) {
	case float64:
		return int(t)
	case int:
		return t
	case string:
		return atoiSafe(t)
	}
	return 0
}

func (r *Router) lbZones(chat, mid int64, flt string, pg int) {
	accs := r.cfAccs()
	allz := r.cfAllZones()
	if len(allz) == 0 {
		r.Show(chat, mid, "⚖️ لود بالانسر IP\n\n📭 دامنه‌ای پیدا نشد.", navKB(""))
		return
	}
	var zones []cfZone
	if flt == "all" {
		zones = allz
	} else {
		a := atoiSafe(flt)
		for _, z := range allz {
			if z.Acc == a {
				zones = append(zones, z)
			}
		}
	}
	pages := (len(zones) + lbPage - 1) / lbPage
	if pages < 1 {
		pages = 1
	}
	if pg < 0 {
		pg = 0
	}
	if pg > pages-1 {
		pg = pages - 1
	}
	lo, hi := pg*lbPage, pg*lbPage+lbPage
	if hi > len(zones) {
		hi = len(zones)
	}
	var rows []Btn
	for _, z := range zones[lo:hi] {
		rows = append(rows, btnS("⚖️ "+z.Zone.Name, fmt.Sprintf("lbz:%d:%s", z.Acc, z.Zone.ID), "plain"))
	}
	kb := gridBTN(rows, 2)
	allLabel := "📁 همه اکانت‌ها"
	if flt == "all" {
		allLabel = "📁 ✅ همه اکانت‌ها"
	}
	kb = append(kb, []Btn{btnS(allLabel, "lbzf:all:0", "plain")})
	if len(accs) > 1 {
		var ab []Btn
		for j, a := range accs {
			label := "👤 " + a.Name
			if flt == strconv.Itoa(j) {
				label = "👤 ✅ " + a.Name
			}
			ab = append(ab, btn(label, fmt.Sprintf("lbzf:%d:0", j)))
		}
		kb = append(kb, gridBTN(ab, 3)...)
	}
	if pages > 1 {
		var nav []Btn
		if pg > 0 {
			nav = append(nav, btn("⬅️", fmt.Sprintf("lbzf:%s:%d", flt, pg-1)))
		} else {
			nav = append(nav, emptyBtn())
		}
		nav = append(nav, btn("🏠 خانه", "menu"))
		if pg < pages-1 {
			nav = append(nav, btn("➡️", fmt.Sprintf("lbzf:%s:%d", flt, pg+1)))
		} else {
			nav = append(nav, emptyBtn())
		}
		kb = append(kb, nav)
	} else {
		kb = append(kb, []Btn{btn("🏠 خانه", "menu")})
	}
	label := "همه"
	if flt != "all" {
		if a := atoiSafe(flt); isDigits(flt) && a < len(accs) {
			label = accs[a].Name
		} else {
			label = "؟"
		}
	}
	title := "⚖️ لود بالانسر IP — " + label
	if pages > 1 {
		title += fmt.Sprintf(" — صفحه %d از %d", pg+1, pages)
	}
	title += "\n\nروی دامنه کلیک کن تا ساب‌هاش رو ببینی.\n\n" +
		"• «بالانسر۱/۲/...» وزن هر آی‌پی است (سهمش از ترافیک)\n" +
		"• «🛟 زاپاس» آی‌پی/CNAME رزرو است؛ اگر اصلی فیلتر شد با یک دکمه جایگزین می‌شود"
	r.Show(chat, mid, title, kb)
}

func lbShort(name, zname string) string {
	if name == zname {
		return "@"
	}
	if strings.HasSuffix(name, "."+zname) {
		return name[:len(name)-(len(zname)+1)]
	}
	return name
}

func (r *Router) lbZoneGroups(chat, mid int64, acc int, zid string, pg int) {
	accs := r.cfAccs()
	if acc < 0 || acc >= len(accs) {
		r.Show(chat, mid, "❌ دامنه پیدا نشد.", navKB("lb"))
		return
	}
	cli := cf.New(accs[acc].Token)
	recs, err := cli.Records(zid)
	if err != nil {
		r.Show(chat, mid, "❌ خطای کلادفلر: "+Esc(err.Error()), navKB("lb"))
		return
	}
	zones, _ := cli.Zones()
	zname := ""
	for _, z := range zones {
		if z.ID == zid {
			zname = z.Name
			break
		}
	}
	if zname == "" {
		r.Show(chat, mid, "❌ دامنه پیدا نشد.", navKB("lb"))
		return
	}
	groups := map[string][]cf.Record{}
	var names []string
	total := 0
	for _, rec := range recs {
		if !lb.IsLbRec(rec.Type) {
			continue
		}
		if _, ok := groups[rec.Name]; !ok {
			names = append(names, rec.Name)
		}
		groups[rec.Name] = append(groups[rec.Name], rec)
		total++
	}
	sort.Strings(names)
	pages := (len(names) + lbGroupsPage - 1) / lbGroupsPage
	if pages < 1 {
		pages = 1
	}
	if pg < 0 {
		pg = 0
	}
	if pg > pages-1 {
		pg = pages - 1
	}
	lo, hi := pg*lbGroupsPage, pg*lbGroupsPage+lbGroupsPage
	if hi > len(names) {
		hi = len(names)
	}
	slice := names[lo:hi]
	lines := []string{fmt.Sprintf("⚖️ لود بالانسر IP — %s", Esc(zname)),
		fmt.Sprintf("📦 %d ساب‌دامین، %d رکورد فعال", len(names), total), ""}
	var kb [][]Btn
	if len(names) == 0 {
		lines = append(lines, "📭 هنوز لود بالانسری تعریف نشده.")
	} else {
		for _, name := range slice {
			g := groups[name]
			short := lbShort(name, zname)
			cfg := lb.GetCfg(r.Store, lb.KeyFor(zid, name))
			line := fmt.Sprintf("%s — %d رکورد", short, len(g))
			if n := len(cfg.Disabled); n > 0 {
				line += fmt.Sprintf(" (+%d غیرفعال)", n)
			}
			lines = append(lines, line)
		}
		for i := 0; i < len(slice); i += 3 {
			var row []Btn
			for j := i; j < i+3 && j < len(slice); j++ {
				name := slice[j]
				short := lbShort(name, zname)
				label := short
				if len(short) > 16 {
					label = short[:15] + "…"
				}
				idx := pg*lbGroupsPage + j
				row = append(row, btn(label, fmt.Sprintf("lbgopen:%d:%s:%d", acc, zid, idx)))
			}
			kb = append(kb, row)
		}
		if pages > 1 {
			lines = append(lines, "", fmt.Sprintf("صفحه %d از %d", pg+1, pages))
		}
	}
	if pages > 1 {
		var nav []Btn
		if pg > 0 {
			nav = append(nav, btn("⬅️", fmt.Sprintf("lbzp:%d:%s:%d", acc, zid, pg-1)))
		} else {
			nav = append(nav, emptyBtn())
		}
		if pg < pages-1 {
			nav = append(nav, btn("➡️", fmt.Sprintf("lbzp:%d:%s:%d", acc, zid, pg+1)))
		} else {
			nav = append(nav, emptyBtn())
		}
		kb = append(kb, nav)
	}
	kb = append(kb, []Btn{btn("🔙 بازگشت", "lb"), btn("🏠 خانه", "menu")})
	txt := strings.Join(lines, "\n")
	if len(txt) > 3500 {
		txt = txt[:3500]
	}
	r.Show(chat, mid, txt, kb)
}

func (r *Router) lbOpenGroup(chat, mid int64, acc int, zid, idx string) {
	accs := r.cfAccs()
	if acc < 0 || acc >= len(accs) {
		r.Show(chat, mid, "❌ دامنه پیدا نشد.", navKB("lb"))
		return
	}
	recs, err := cf.New(accs[acc].Token).Records(zid)
	if err != nil {
		r.Show(chat, mid, "❌ خطای کلادفلر: "+Esc(err.Error()), navKB("lb"))
		return
	}
	sset := map[string]bool{}
	for _, rec := range recs {
		if lb.IsLbRec(rec.Type) {
			sset[rec.Name] = true
		}
	}
	var names []string
	for n := range sset {
		names = append(names, n)
	}
	sort.Strings(names)
	i := atoiSafe(idx)
	if i < 0 || i >= len(names) {
		r.Show(chat, mid, "❌ پیدا نشد.", navKB("lb"))
		return
	}
	name := names[i]
	gid := "lb:cf:" + zid + ":" + name
	r.lbSetSess(gid, lbSess{Acc: acc, Zid: zid, Name: name})
	r.lbRender(chat, mid, gid)
}

func (r *Router) lbRender(chat, mid int64, gid string) {
	s := r.lbGetSess(gid)
	if s.Zid == "" || s.Name == "" {
		r.Show(chat, mid, "⏳ نشست منقضی شد.", HomeKB())
		return
	}
	_, group, cfg, _, err := r.lbGroupOf(s.Acc, s.Zid, s.Name)
	if err != nil {
		r.Show(chat, mid, Esc(err.Error()), HomeKB())
		return
	}
	entries := lb.EntriesOf(group, cfg)
	onN := 0
	for _, e := range entries {
		if e.On {
			onN++
		}
	}
	lines := []string{"⚖️ تنظیمات لود بالانسر IP", "",
		fmt.Sprintf("📛 ساب‌دامین: <code>%s</code>", Esc(s.Name)),
		fmt.Sprintf("🌍 آی‌پی‌ها: %d فعال از %d", onN, len(entries)), ""}
	var kb [][]Btn
	if len(entries) == 0 {
		lines = append(lines, "📭 هنوز آی‌پی‌ای اضافه نشده.")
	} else {
		for i, e := range entries {
			icon := "🔴"
			if e.On {
				icon = "🟢"
			}
			line := fmt.Sprintf("%d) %s <code>%s</code> — وزن %s", i+1, icon, Esc(e.Content), faW(e.Weight))
			if !e.On {
				line += " (غیرفعال)"
			}
			lines = append(lines, line)
		}
		lines = append(lines, "",
			"وزن و فعال/غیرفعال هر آی‌پی را از دکمه‌های زیر تغییر بده؛ زاپاس را از دکمهٔ 🛟 زیر.")
		for i, e := range entries {
			tgl := "⏸ غیرفعال"
			if !e.On {
				tgl = "▶️ فعال"
			}
			kb = append(kb, []Btn{
				btn("بالانسر"+faD(strconv.Itoa(i+1)), fmt.Sprintf("lbw:%s:%d", gid, i)),
				btn(tgl, fmt.Sprintf("lbt:%s:%d", gid, i))})
		}
	}
	kb = append(kb, []Btn{btn("➕ افزودن آی‌پی", fmt.Sprintf("lbadd:%s", gid))})
	if onN > 0 {
		kb = append(kb, []Btn{
			btn("🟠 Proxy همه روشن", fmt.Sprintf("lbp:%s:on", gid)),
			btn("⚪️ Proxy همه خاموش", fmt.Sprintf("lbp:%s:off", gid))})
	}
	kb = append(kb, []Btn{btnS("🛟 آیپی زاپاس", fmt.Sprintf("lbsh:%s", gid), "success")})
	back := "zones"
	if s.Rid != "" {
		back = "cfr:" + s.Rid
	}
	kb = append(kb, []Btn{btn("🔙 بازگشت", back)})
	r.Show(chat, mid, strings.Join(lines, "\n"), kb)
}

func faW(w float64) string {
	if w == float64(int64(w)) {
		return faD(strconv.FormatInt(int64(w), 10))
	}
	return faD(strconv.FormatFloat(w, 'f', -1, 64))
}

func (r *Router) lbSettings(chat, mid int64, rid string) {
	acc, zid := r.cfCtx(chat, rid)
	if zid == "" {
		r.Show(chat, mid, "⏳ نشست منقضی شد.", HomeKB())
		return
	}
	cli, ok := r.cfClientFor(acc)
	if !ok {
		r.Show(chat, mid, "❌ دامنه پیدا نشد.", HomeKB())
		return
	}
	recs, err := cli.Records(zid)
	if err != nil {
		r.Show(chat, mid, "❌ "+Esc(err.Error()), HomeKB())
		return
	}
	name := ""
	for _, x := range recs {
		if x.ID == rid {
			name = x.Name
			break
		}
	}
	if name == "" {
		r.Show(chat, mid, "❌ رکورد پیدا نشد.", HomeKB())
		return
	}
	gid := lb.NewGid(r.Store, acc, zid, name)
	r.lbSetSess(gid, lbSess{Acc: acc, Zid: zid, Name: name, Rid: rid})
	r.lbRender(chat, mid, gid)
}

func (r *Router) lbWeightAsk(chat, mid int64, gid string, idx int) {
	r.setWizard(chat, map[string]any{"kind": "lb_weight", "gid": gid, "idx": idx})
	r.Show(chat, mid, "⚖️ وزن جدید را بفرستید (عدد ۰ یا بیشتر):",
		[][]Btn{{btn("⬅️ انصراف", "lbsr:"+gid)}})
}

func (r *Router) lbToggleAsk(chat, mid int64, gid string, idx int) {
	s := r.lbGetSess(gid)
	entries := r.lbEntries(s)
	if idx < 0 || idx >= len(entries) {
		r.Show(chat, mid, "❌ مورد پیدا نشد.", HomeKB())
		return
	}
	e := entries[idx]
	var q string
	if e.On {
		q = "⏸ این آی‌پی از لود بالانسر حذف (غیرفعال) شود؟"
	} else {
		q = "▶️ این آی‌پی به لود بالانسر اضافه (فعال) شود؟"
	}
	q += fmt.Sprintf("\n\n<code>%s</code>", Esc(e.Content))
	r.Show(chat, mid, q, [][]Btn{
		{btn("✅ بله", fmt.Sprintf("lbtc:%s:%d", gid, idx)),
			btn("❌ انصراف", "lbsr:"+gid)}})
}

func (r *Router) lbToggleDo(chat, mid int64, gid string, idx int) {
	s := r.lbGetSess(gid)
	entries := r.lbEntries(s)
	if idx < 0 || idx >= len(entries) {
		r.Show(chat, mid, "❌ مورد پیدا نشد.", HomeKB())
		return
	}
	e := entries[idx]
	key := lb.KeyFor(s.Zid, s.Name)
	cfg := lb.GetCfg(r.Store, key)
	cli, ok := r.cfClientFor(s.Acc)
	if !ok {
		r.Show(chat, mid, "❌ دامنه پیدا نشد.", HomeKB())
		return
	}
	if e.On {
		if e.ID != "" {
			_ = cli.DelRecord(s.Zid, e.ID)
		}
		cfg.Disabled = append(cfg.Disabled, lb.Disabled{
			Type: e.Type, Content: e.Content, Proxied: e.Proxied, TTL: e.TTL})
	} else {
		_, err := cli.AddRecord(s.Zid, e.Type, s.Name, e.Content, ttlInt(e.TTL), e.Proxied, nil)
		if err != nil {
			r.Show(chat, mid, "❌ خطا: "+Esc(err.Error()), HomeKB())
			return
		}
		var nd []lb.Disabled
		for _, d := range cfg.Disabled {
			if d.Content != e.Content {
				nd = append(nd, d)
			}
		}
		cfg.Disabled = nd
	}
	lb.SaveCfg(r.Store, key, cfg)
	r.lbRender(chat, mid, gid)
}

func (r *Router) lbAddStart(chat, mid int64, gid string) {
	r.setWizard(chat, map[string]any{"kind": "lb_add", "gid": gid})
	r.Show(chat, mid, "➕ نوع و مقدار آی‌پی جدید را بفرستید (مثلاً <code>A 1.2.3.4</code>):",
		[][]Btn{{btn("⬅️ انصراف", "lbsr:"+gid)}})
}

func (r *Router) lbProxyAll(chat, mid int64, gid string, on bool) {
	s := r.lbGetSess(gid)
	entries := r.lbEntries(s)
	cli, ok := r.cfClientFor(s.Acc)
	if !ok {
		r.Show(chat, mid, "❌ دامنه پیدا نشد.", HomeKB())
		return
	}
	for _, e := range entries {
		if !e.On || e.ID == "" {
			continue
		}
		rec := cf.Record{ID: e.ID, Type: e.Type, Name: s.Name, Content: e.Content, TTL: e.TTL, Proxied: e.Proxied}
		_, _ = cli.UpdateRecord(s.Zid, rec, map[string]any{"proxied": on})
	}
	r.lbRender(chat, mid, gid)
}

func (r *Router) lbSpareHome(chat, mid int64, rid string) {
	acc, zid := r.cfCtx(chat, rid)
	if zid == "" {
		r.Show(chat, mid, "⏳ نشست منقضی شد.", HomeKB())
		return
	}
	cli, ok := r.cfClientFor(acc)
	if !ok {
		r.Show(chat, mid, "❌ دامنه پیدا نشد.", HomeKB())
		return
	}
	recs, err := cli.Records(zid)
	if err != nil {
		r.Show(chat, mid, "❌ "+Esc(err.Error()), HomeKB())
		return
	}
	name := ""
	for _, x := range recs {
		if x.ID == rid {
			name = x.Name
			break
		}
	}
	if name == "" {
		r.Show(chat, mid, "❌ رکورد پیدا نشد.", HomeKB())
		return
	}
	gid := lb.NewGid(r.Store, acc, zid, name)
	r.lbSetSess(gid, lbSess{Acc: acc, Zid: zid, Name: name, Rid: rid})
	r.lbSpareHomeGid(chat, mid, gid)
}

func (r *Router) lbSpareHomeGid(chat, mid int64, gid string) {
	s := r.lbGetSess(gid)
	entries := r.lbEntries(s)
	cfg := lb.GetCfg(r.Store, lb.KeyFor(s.Zid, s.Name))
	lines := []string{"🛟 زاپاس گروه", ""}
	var kb [][]Btn
	for i, e := range entries {
		if !e.On {
			continue
		}
		sp := lb.SpareOf(cfg, e.Content)
		if sp != nil {
			lines = append(lines, fmt.Sprintf("%d) <code>%s</code> → <code>%s</code>", i+1, Esc(e.Content), Esc(sp.Content)))
		} else {
			lines = append(lines, fmt.Sprintf("%d) <code>%s</code> → بدون زاپاس", i+1, Esc(e.Content)))
		}
		label := e.Content
		if len(label) > 20 {
			label = label[:20]
		}
		kb = append(kb, []Btn{btnS("🛟 "+label, fmt.Sprintf("lbsm:%s:%d", gid, i), "plain")})
	}
	back := "zones"
	if s.Rid != "" {
		back = "lbs:" + s.Rid
	}
	kb = append(kb, []Btn{btn("🔙 بازگشت", back), btn("🏠 خانه", "menu")})
	r.Show(chat, mid, strings.Join(lines, "\n"), kb)
}

func (r *Router) lbSpareMenu(chat, mid int64, gid string, idx int) {
	s := r.lbGetSess(gid)
	full := r.lbEntries(s)
	var onIdx []int
	for k, x := range full {
		if x.On {
			onIdx = append(onIdx, k)
		}
	}
	if idx < 0 || idx >= len(onIdx) {
		r.Show(chat, mid, "❌ مورد پیدا نشد.", HomeKB())
		return
	}
	fi := onIdx[idx]
	e := full[fi]
	cfg := lb.GetCfg(r.Store, lb.KeyFor(s.Zid, s.Name))
	sp := lb.SpareOf(cfg, e.Content)
	lines := []string{fmt.Sprintf("🛟 زاپاس — <code>%s</code> (%s)", Esc(e.Content), e.Type), ""}
	if sp != nil {
		lines = append(lines, fmt.Sprintf("زاپاس فعلی: <code>%s</code> (%s)", Esc(sp.Content), sp.Type))
	} else {
		lines = append(lines, "هنوز زاپاسی تعیین نشده.")
	}
	lines = append(lines, "",
		"اگر این آی‌پی فیلتر شد، با «🔄 جایگزینی» رکورد DNS به زاپاس عوض می‌شود؛",
		"بعداً با همان دکمه می‌توانی به اصلی برگردی.")
	label := "➕ تعیین زاپاس"
	if sp != nil {
		label = "✏️ تغییر زاپاس"
	}
	kb := [][]Btn{{btn(label, fmt.Sprintf("lbsset:%s:%d", gid, fi))}}
	if sp != nil {
		sc := sp.Content
		if len(sc) > 24 {
			sc = sc[:24]
		}
		kb = append(kb, []Btn{btn("🔄 جایگزینی با "+sc, fmt.Sprintf("lbsgo:%s:%d", gid, fi))})
		kb = append(kb, []Btn{btn("❌ حذف زاپاس", fmt.Sprintf("lbsdel:%s:%d", gid, fi))})
	}
	kb = append(kb, []Btn{btn("🔙 بازگشت", "lbsh:"+gid), btn("🏠 خانه", "menu")})
	r.Show(chat, mid, strings.Join(lines, "\n"), kb)
}

func (r *Router) lbReplace(chat, mid int64, gid string, idx int, confirm bool) {
	s := r.lbGetSess(gid)
	entries := r.lbEntries(s)
	if idx < 0 || idx >= len(entries) {
		r.Show(chat, mid, "❌ مورد پیدا نشد.", HomeKB())
		return
	}
	e := entries[idx]
	cfg := lb.GetCfg(r.Store, lb.KeyFor(s.Zid, s.Name))
	sp := lb.SpareOf(cfg, e.Content)
	if sp == nil {
		r.Show(chat, mid, "❌ زاپاسی تعیین نشده.", HomeKB())
		return
	}
	if !confirm {
		r.Show(chat, mid,
			fmt.Sprintf("🔄 رکورد <code>%s</code> به <code>%s</code> عوض شود؟", Esc(e.Content), Esc(sp.Content)),
			[][]Btn{{btn("✅ بله", fmt.Sprintf("lbsgoc:%s:%d", gid, idx)),
				btn("❌ انصراف", fmt.Sprintf("lbsm:%s:%d", gid, idx))}})
		return
	}
	cli, ok := r.cfClientFor(s.Acc)
	if !ok {
		r.Show(chat, mid, "❌ دامنه پیدا نشد.", HomeKB())
		return
	}
	if e.ID != "" {
		_ = cli.DelRecord(s.Zid, e.ID)
	}
	spType := sp.Type
	if spType == "" {
		spType = e.Type
	}
	if _, err := cli.AddRecord(s.Zid, spType, s.Name, sp.Content, 1, false, nil); err != nil {
		r.Show(chat, mid, "❌ نشد: "+Esc(err.Error()), HomeKB())
		return
	}
	r.Bot.Send(chat, "✅ جایگزینی انجام شد.", nil)
	r.lbRender(chat, 0, gid)
}

func (r *Router) lbWeightSave(chat, mid int64, pend map[string]any, text string) {
	gid, _ := pend["gid"].(string)
	idx := intOrAny(pend["idx"])
	v, err := strconv.ParseFloat(strings.TrimSpace(text), 64)
	if err != nil || v < 0 {
		r.Bot.Send(chat, "❌ عدد نامعتبر است.", nil)
		return
	}
	s := r.lbGetSess(gid)
	key := lb.KeyFor(s.Zid, s.Name)
	cfg := lb.GetCfg(r.Store, key)
	entries := r.lbEntries(s)
	if idx >= 0 && idx < len(entries) {
		if cfg.Weights == nil {
			cfg.Weights = map[string]float64{}
		}
		cfg.Weights[entries[idx].Content] = v
		lb.SaveCfg(r.Store, key, cfg)
	}
	r.clearWizard(chat)
	r.Bot.Send(chat, "✅ ذخیره شد.", nil)
	r.lbRender(chat, 0, gid)
}

func (r *Router) lbSpareSave(chat, mid int64, pend map[string]any, text string) {
	gid, _ := pend["gid"].(string)
	idx := intOrAny(pend["idx"])
	s := r.lbGetSess(gid)
	entries := r.lbEntries(s)
	var on []lb.Entry
	for _, e := range entries {
		if e.On {
			on = append(on, e)
		}
	}
	if idx < 0 || idx >= len(on) {
		r.clearWizard(chat)
		r.Bot.Send(chat, "❌ مورد پیدا نشد.", nil)
		return
	}
	v := strings.TrimSpace(text)
	typ := "CNAME"
	if ip := net.ParseIP(v); ip != nil {
		if strings.Contains(v, ".") {
			typ = "A"
		} else {
			typ = "AAAA"
		}
	}
	key := lb.KeyFor(s.Zid, s.Name)
	cfg := lb.GetCfg(r.Store, key)
	if cfg.Spares == nil {
		cfg.Spares = map[string]lb.Spare{}
	}
	cfg.Spares[on[idx].Content] = lb.Spare{Type: typ, Content: v}
	lb.SaveCfg(r.Store, key, cfg)
	r.clearWizard(chat)
	r.Bot.Send(chat, "✅ زاپاس ذخیره شد.", nil)
	r.lbSpareHomeGid(chat, 0, gid)
}

func (r *Router) lbAddSave(chat, mid int64, pend map[string]any, text string) {
	gid, _ := pend["gid"].(string)
	s := r.lbGetSess(gid)
	parts := strings.Fields(text)
	if len(parts) < 2 {
		r.Bot.Send(chat, "❌ قالب درست نیست (مثلاً <code>A 1.2.3.4</code>).", nil)
		return
	}
	cli, ok := r.cfClientFor(s.Acc)
	if !ok {
		r.clearWizard(chat)
		r.Bot.Send(chat, "❌ دامنه پیدا نشد.", nil)
		return
	}
	if _, err := cli.AddRecord(s.Zid, strings.ToUpper(parts[0]), s.Name, parts[1], 1, false, nil); err != nil {
		r.Bot.Send(chat, "❌ نشد: "+Esc(err.Error()), nil)
		return
	}
	r.Bot.Send(chat, "✅ آی‌پی اضافه شد.", nil)
	r.clearWizard(chat)
	r.lbRender(chat, 0, gid)
}

func (r *Router) lbSpareDel(chat, mid int64, gid string, idx int) {
	s := r.lbGetSess(gid)
	full := r.lbEntries(s)
	var onIdx []int
	for k, x := range full {
		if x.On {
			onIdx = append(onIdx, k)
		}
	}
	if idx < 0 || idx >= len(onIdx) {
		r.Show(chat, mid, "❌ مورد پیدا نشد.", HomeKB())
		return
	}
	e := full[onIdx[idx]]
	key := lb.KeyFor(s.Zid, s.Name)
	cfg := lb.GetCfg(r.Store, key)
	delete(cfg.Spares, e.Content)
	lb.SaveCfg(r.Store, key, cfg)
	r.Bot.Send(chat, "✅ زاپاس حذف شد.", nil)
	r.lbSpareHomeGid(chat, 0, gid)
}
