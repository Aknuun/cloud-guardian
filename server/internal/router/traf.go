package router

import (
	"fmt"
	"sort"
	"strconv"
	"strings"

	"github.com/aknuun/cg-server/internal/cf"
)

const trafPage = 20

var faDigits = strings.NewReplacer(
	"0", "۰", "1", "۱", "2", "۲", "3", "۳", "4", "۴",
	"5", "۵", "6", "۶", "7", "۷", "8", "۸", "9", "۹")

func faD(s string) string { return faDigits.Replace(s) }

func commaFmt(n int) string {
	s := strconv.Itoa(n)
	neg := strings.HasPrefix(s, "-")
	if neg {
		s = s[1:]
	}
	var out []byte
	for i := 0; i < len(s); i++ {
		if i > 0 && (len(s)-i)%3 == 0 {
			out = append(out, ',')
		}
		out = append(out, s[i])
	}
	if neg {
		return "-" + string(out)
	}
	return string(out)
}

func gridBTN(btns []Btn, perRow int) [][]Btn {
	var out [][]Btn
	for i := 0; i < len(btns); i += perRow {
		end := i + perRow
		if end > len(btns) {
			end = len(btns)
		}
		out = append(out, btns[i:end])
	}
	return out
}

func emptyBtn() Btn { return Btn{Text: " ", Data: "noop"} }

func (r *Router) trafZones(chat, mid int64, flt string, pg int) {
	accs := r.cfAccs()
	allz := r.cfAllZones()
	if len(allz) == 0 {
		r.Show(chat, mid, "📊 ترافیک ساب‌ها\n\n📭 دامنه‌ای پیدا نشد.",
			navKB("dommon"))
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
	pages := (len(zones) + trafPage - 1) / trafPage
	if pages < 1 {
		pages = 1
	}
	if pg < 0 {
		pg = 0
	}
	if pg > pages-1 {
		pg = pages - 1
	}
	lo := pg * trafPage
	hi := lo + trafPage
	if hi > len(zones) {
		hi = len(zones)
	}
	var rows []Btn
	for _, z := range zones[lo:hi] {
		rows = append(rows, btnS("📊 "+z.Zone.Name, fmt.Sprintf("traz:%d:%s:0", z.Acc, z.Zone.ID), "plain"))
	}
	kb := gridBTN(rows, 2)
	allLabel := "📁 همه اکانت‌ها"
	if flt == "all" {
		allLabel = "📁 ✅ همه اکانت‌ها"
	}
	kb = append(kb, []Btn{btnS(allLabel, "trazf:all:0", "plain")})
	if len(accs) > 1 {
		var accBtns []Btn
		for j, a := range accs {
			label := "👤 " + a.Name
			if flt == strconv.Itoa(j) {
				label = "👤 ✅ " + a.Name
			}
			accBtns = append(accBtns, btn(label, fmt.Sprintf("trazf:%d:0", j)))
		}
		kb = append(kb, gridBTN(accBtns, 3)...)
	}
	if pages > 1 {
		var nav []Btn
		if pg > 0 {
			nav = append(nav, btn("⬅️", fmt.Sprintf("trazf:%s:%d", flt, pg-1)))
		} else {
			nav = append(nav, emptyBtn())
		}
		nav = append(nav, btn("🔙 بازگشت", "dommon"))
		if pg < pages-1 {
			nav = append(nav, btn("➡️", fmt.Sprintf("trazf:%s:%d", flt, pg+1)))
		} else {
			nav = append(nav, emptyBtn())
		}
		kb = append(kb, nav)
	} else {
		kb = append(kb, []Btn{btn("🔙 بازگشت", "dommon"), btn("🏠 خانه", "menu")})
	}
	label := "همه"
	if flt != "all" {
		if a := atoiSafe(flt); a >= 0 && a < len(accs) && isDigits(flt) {
			label = accs[a].Name
		} else {
			label = "؟"
		}
	}
	title := "📊 ترافیک ساب‌ها — " + label
	if pages > 1 {
		title += fmt.Sprintf(" — صفحه %d از %d", pg+1, pages)
	}
	title += "\n\nترافیک ۷ روزهٔ DNS را ببینید — روی دامنه کلیک کنید."
	r.Show(chat, mid, title, kb)
}

func isDigits(s string) bool {
	if s == "" {
		return false
	}
	for _, c := range s {
		if c < '0' || c > '9' {
			return false
		}
	}
	return true
}

func (r *Router) trafZone(chat, mid int64, acc int, zid string, pg int) {
	accs := r.cfAccs()
	if acc < 0 || acc >= len(accs) {
		r.Show(chat, mid, "❌ اکانت پیدا نشد.",
			navKB("traf"))
		return
	}
	token := accs[acc].Token
	cli := cf.New(token)
	zones, err := cli.Zones()
	if err != nil {
		r.Show(chat, mid, "❌ خطای کلادفلر: "+Esc(err.Error()), navKB("traf"))
		return
	}
	var zone *cf.Zone
	for i := range zones {
		if zones[i].ID == zid {
			zone = &zones[i]
			break
		}
	}
	if zone == nil {
		r.Show(chat, mid, "❌ دامنه پیدا نشد.",
			navKB("traf"))
		return
	}
	t := cf.FetchTrafficCounts(token, zid, 7)
	if t.NeedPerm || t.Error != "" {
		body := "❌ خطا در آمار:\n" + Esc(t.Error)
		if t.NeedPerm {
			body = "❌ دسترسی Analytics نیست.\n\n۱) به توکن این دسترسی را اضافه کن:\n" +
				"<code>Zone → Analytics → Read</code>\n\n" +
				"۲) چون توکن قابل ویرایش نیست، توکن جدید بساز و توی ربات جایگزین کن:\n" +
				"کلودفلر ← 👤 اکانت‌ها ← حذف قدیمی + افزودن جدید"
		}
		r.Show(chat, mid, fmt.Sprintf("📊 ترافیک %s\n\n%s", Esc(zone.Name), body),
			navKB("traf"))
		return
	}
	counts := t.Counts
	recs, err := cli.Records(zid)
	if err != nil {
		r.Show(chat, mid, "❌ خطای کلادفلر: "+Esc(err.Error()), navKB("traf"))
		return
	}
	type row struct {
		r cf.Record
		n int
	}
	var rows []row
	for _, rec := range recs {
		typ := strings.ToUpper(rec.Type)
		if typ != "A" && typ != "AAAA" && typ != "CNAME" {
			continue
		}
		rows = append(rows, row{r: rec, n: counts[strings.ToLower(rec.Name)]})
	}
	sort.SliceStable(rows, func(i, j int) bool {
		if rows[i].n != rows[j].n {
			return rows[i].n > rows[j].n
		}
		return rows[i].r.Name < rows[j].r.Name
	})
	pages := (len(rows) + trafPage - 1) / trafPage
	if pages < 1 {
		pages = 1
	}
	if pg < 0 {
		pg = 0
	}
	if pg > pages-1 {
		pg = pages - 1
	}
	active := 0
	for _, x := range rows {
		if x.n > 0 {
			active++
		}
	}
	zeros := len(rows) - active
	zname := zone.Name
	lines := []string{
		fmt.Sprintf("📊 ترافیک ۷ روز گذشته — %s", Esc(zname)),
		fmt.Sprintf("🧮 %d ساب | 🟢 %d فعال | ⚪ %d صفر", len(rows), active, zeros), ""}
	lo := pg * trafPage
	hi := lo + trafPage
	if hi > len(rows) {
		hi = len(rows)
	}
	for _, x := range rows[lo:hi] {
		n := x.r.Name
		short := n
		if n == zname {
			short = "@"
		} else if strings.HasSuffix(n, "."+zname) {
			short = n[:len(n)-(len(zname)+1)]
		}
		icon := "⚪"
		if x.n > 0 {
			icon = "🟢"
		}
		lines = append(lines, fmt.Sprintf("%s %s (%s) — %s کوئری", icon, Esc(short), Esc(x.r.Type), faD(commaFmt(x.n))))
	}
	if pages > 1 {
		lines = append(lines, "", fmt.Sprintf("صفحه %d از %d", pg+1, pages))
	}
	lines = append(lines, "", "⚠️ ساب کم‌استفاده هم صفر نشون میده؛ قبل از حذف مطمئن شو.")
	var kb [][]Btn
	if pg > 0 && pg < pages-1 {
		kb = append(kb, []Btn{
			btn("⬅️ قبلی", fmt.Sprintf("traz:%d:%s:%d", acc, zid, pg-1)),
			btn("➡️ بعدی", fmt.Sprintf("traz:%d:%s:%d", acc, zid, pg+1))})
	} else if pg > 0 {
		kb = append(kb, []Btn{btn("⬅️ قبلی", fmt.Sprintf("traz:%d:%s:%d", acc, zid, pg-1))})
	} else if pg < pages-1 {
		kb = append(kb, []Btn{btn("➡️ بعدی", fmt.Sprintf("traz:%d:%s:%d", acc, zid, pg+1))})
	}
	if zeros > 0 {
		kb = append(kb, []Btn{btnS(fmt.Sprintf("🗑 حذف %d ساب صفر", zeros), fmt.Sprintf("trazdel:%d:%s", acc, zid), "danger")})
	}
	kb = append(kb, []Btn{btn("🔙 بازگشت", "traf"), btn("🏠 خانه", "menu")})
	txt := strings.Join(lines, "\n")
	if len(txt) > 3500 {
		txt = txt[:3500]
	}
	r.Show(chat, mid, txt, kb)
}

func (r *Router) trafZeroConfirm(chat, mid int64, acc int, zid string) {
	accs := r.cfAccs()
	zname := zid
	if acc >= 0 && acc < len(accs) {
		if zones, err := cf.New(accs[acc].Token).Zones(); err == nil {
			for _, z := range zones {
				if z.ID == zid {
					zname = z.Name
					break
				}
			}
		}
	}
	r.Show(chat, mid,
		fmt.Sprintf("🗑 همهٔ ساب‌های %s با ترافیک صفر حذف شوند؟\n\n⚠️ ساب کم‌استفاده هم صفر حساب می‌شود؛ مطمئنی؟", Esc(zname)),
		[][]Btn{
			{btnS("✅ بله، حذف کن", fmt.Sprintf("trazdely:%d:%s", acc, zid), "danger"),
				btn("❌ انصراف", fmt.Sprintf("traz:%d:%s:0", acc, zid))}})
}

func (r *Router) trafZeroDo(chat, mid int64, acc int, zid string) {
	accs := r.cfAccs()
	if acc < 0 || acc >= len(accs) {
		r.Show(chat, mid, "❌ اکانت پیدا نشد.", navKB("traf"))
		return
	}
	token := accs[acc].Token
	t := cf.FetchTrafficCounts(token, zid, 7)
	if t.NeedPerm || t.Error != "" {
		r.Show(chat, mid, "❌ خطا در آمار.", navKB("traf"))
		return
	}
	client := cf.New(token)
	recs, err := client.Records(zid)
	if err != nil {
		r.Show(chat, mid, "❌ خطای کلادفلر: "+Esc(err.Error()), navKB("traf"))
		return
	}
	var zeros []cf.Record
	for _, rec := range recs {
		typ := strings.ToUpper(rec.Type)
		if (typ == "A" || typ == "AAAA" || typ == "CNAME") &&
			t.Counts[strings.ToLower(rec.Name)] == 0 {
			zeros = append(zeros, rec)
		}
	}
	ok := 0
	for _, rec := range zeros {
		if err := client.DelRecord(zid, rec.ID); err == nil {
			ok++
		}
	}
	r.Show(chat, mid,
		fmt.Sprintf("✅ %d از %d رکورد بدون ترافیک حذف شد.", ok, len(zeros)),
		[][]Btn{{btn("📊 ترافیک", "traf")}})
}
