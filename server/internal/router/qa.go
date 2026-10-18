package router

import (
	"fmt"
	"strings"

	"github.com/aknuun/cg-server/internal/cf"
)

type qaHit struct {
	Acc   int    `json:"acc"`
	ZID   string `json:"zid"`
	ZName string `json:"zname"`
	RID   string `json:"rid"`
	Name  string `json:"name"`
	Type  string `json:"type"`
}

type qaSess struct {
	IP   string  `json:"ip"`
	Hits []qaHit `json:"hits"`
}

const qaPerPage = 20

var qaIPRe = ipRe2

func (r *Router) qaHome(chat, mid int64) {
	sess := map[string]any{}
	r.Store.GetJSON(fmt.Sprintf("qa:%d", chat), &sess)
	ip := strv(sess["ip"])
	lines := []string{"⚡ تغییر سریع", "",
		"یک آی‌پی یا دامنه بفرست تا همهٔ رکوردهای مربوط به آن را پیدا کنم و بتوانی سریع حذف/منتخب/افزودن ساب کنی."}
	if ip != "" {
		lines = append(lines, "", fmt.Sprintf("🌐 آخرین آی‌پی: <code>%s</code>", Esc(ip)))
	}
	kb := [][]Btn{
		{btn("🔍 جست‌وجوی نام", "qasearch")},
		{btn("🔙 بازگشت", "menu"), btn("🏠 خانه", "menu")},
	}
	r.Show(chat, mid, strings.Join(lines, "\n"), kb)
}

func (r *Router) qaSearchStart(chat, mid int64) {
	r.setWizard(chat, map[string]any{"kind": "cr_gsearch"})
	r.Show(chat, mid, "🔍 نام ساب‌دامین یا بخشی از آن را بفرستید:",
		[][]Btn{{btn("❌ لغو", "qacancel")}})
}

func (r *Router) qaCancel(chat, mid int64) {
	r.clearWizard(chat)
	r.Show(chat, mid, "لغو شد.", [][]Btn{{btn("🔙 بازگشت", "qamenu"), btn("🏠 خانه", "menu")}})
}

// qaIPSearch finds every DNS record (across all CF accounts) whose value is ip.
func (r *Router) qaIPSearch(chat, mid int64, ip string) {
	ip = strings.TrimSpace(ip)
	r.Store.PutJSON(fmt.Sprintf("qa:%d", chat), map[string]any{"ip": ip}, 3600)
	var hits []qaHit
	for i, a := range r.cfAccs() {
		cli := cf.New(a.Token)
		zs, err := cli.Zones()
		if err != nil {
			continue
		}
		for _, z := range zs {
			if z.Status != "active" {
				continue
			}
			recs, err := cli.Records(z.ID)
			if err != nil {
				continue
			}
			for k := range recs {
				if strings.TrimSpace(recs[k].Content) == ip {
					hits = append(hits, qaHit{Acc: i, ZID: z.ID, ZName: z.Name,
						RID: recs[k].ID, Name: recs[k].Name, Type: recs[k].Type})
				}
			}
		}
		if len(hits) >= 200 {
			break
		}
	}
	if len(hits) == 0 {
		r.Show(chat, mid, fmt.Sprintf("🤷 آی‌پی <code>%s</code> در هیچ رکوردی پیدا نشد.", Esc(ip)),
			[][]Btn{{btn("➕ افزودن ساب برای این آی‌پی", "qadd2:"+ipShort(ip))},
				{btn("🔙 بازگشت", "qamenu"), btn("🏠 خانه", "menu")}})
		return
	}
	token := newBkID()
	r.Store.PutJSON("ips:"+token, qaSess{IP: ip, Hits: hits}, 3600)
	r.qaResults(chat, mid, token, 0)
}

func ipShort(ip string) string {
	if len(ip) <= 60 {
		return ip
	}
	return ip[:60]
}

func (r *Router) qaResults(chat, mid int64, token string, page int) {
	var s qaSess
	if !r.Store.GetJSON("ips:"+token, &s) {
		r.Show(chat, mid, "⏳ نشست منقضی شد. دوباره آی‌پی را بفرستید.", HomeKB())
		return
	}
	n := len(s.Hits)
	pages := (n + qaPerPage - 1) / qaPerPage
	if pages < 1 {
		pages = 1
	}
	if page < 0 {
		page = 0
	}
	if page > pages-1 {
		page = pages - 1
	}
	start, end := page*qaPerPage, page*qaPerPage+qaPerPage
	if end > n {
		end = n
	}
	results := map[string]any{}
	var items []Btn
	for _, h := range s.Hits[start:end] {
		results[h.RID] = map[string]any{"acc": h.Acc, "zid": h.ZID}
		items = append(items, btn(cfRecLabel(h.Name, h.Type, h.ZName), "cfr:"+h.RID))
	}
	kb := gridBTN(items, 2)
	r.cfSaveSession(chat, map[string]any{"results": results})
	if pages > 1 {
		var nav []Btn
		if page > 0 {
			nav = append(nav, btn("◀️", fmt.Sprintf("qap:%s:%d", token, page-1)))
		}
		nav = append(nav, btn(fmt.Sprintf("📄 %d/%d", page+1, pages), "noop"))
		if page < pages-1 {
			nav = append(nav, btn("▶️", fmt.Sprintf("qap:%s:%d", token, page+1)))
		}
		kb = append(kb, nav)
	}
	kb = append(kb,
		[]Btn{btnS("🗑 حذف همه", "qadelall:"+token, "danger"), btnS("⭐ افزودن همه به منتخب", "qafavall:"+token, "primary")},
		[]Btn{btn("➕ افزودن ساب برای این آی‌پی", "qadd:"+token)},
		navRow("qamenu"),
	)
	r.Show(chat, mid, fmt.Sprintf("⚡ آی‌پی <code>%s</code> در %d رکورد:", Esc(s.IP), n), kb)
}

func (r *Router) qaDelAllAsk(chat, mid int64, token string) {
	var s qaSess
	if !r.Store.GetJSON("ips:"+token, &s) {
		r.Show(chat, mid, "⏳ نشست منقضی شد.", HomeKB())
		return
	}
	r.Show(chat, mid, fmt.Sprintf("⚠️ همهٔ %d رکورد حذف شود؟", len(s.Hits)),
		[][]Btn{{btnS("✅ بله، حذف کن", "qadely:"+token, "danger"),
			btn("❌ انصراف", fmt.Sprintf("qap:%s:0", token))}})
}

func (r *Router) qaDelAllDo(chat, mid int64, token string) {
	var s qaSess
	if !r.Store.GetJSON("ips:"+token, &s) {
		r.Show(chat, mid, "⏳ نشست منقضی شد.", HomeKB())
		return
	}
	okn, fail := 0, 0
	for _, h := range s.Hits {
		cli, ok := r.cfClientFor(h.Acc)
		if !ok {
			fail++
			continue
		}
		if err := cli.DelRecord(h.ZID, h.RID); err != nil {
			fail++
		} else {
			okn++
		}
	}
	msg := fmt.Sprintf("✅ %d حذف شد.", okn)
	if fail > 0 {
		msg += fmt.Sprintf(" ⚠️ %d ناموفق.", fail)
	}
	r.Bot.Send(chat, msg, nil)
	r.Show(chat, mid, msg, [][]Btn{{btn("🔙 بازگشت", "qamenu"), btn("🏠 خانه", "menu")}})
}

func (r *Router) qaFavAll(chat, mid int64, token string) {
	var s qaSess
	if !r.Store.GetJSON("ips:"+token, &s) {
		r.Show(chat, mid, "⏳ نشست منقضی شد.", HomeKB())
		return
	}
	favs := r.favs()
	added := 0
	for _, h := range s.Hits {
		exists := false
		for _, f := range favs {
			if strvVal(f["rid"]) == h.RID {
				exists = true
				break
			}
		}
		if exists {
			continue
		}
		favs = append(favs, map[string]any{"acc": h.Acc, "zid": h.ZID, "rid": h.RID,
			"name": cfRecLabel(h.Name, h.Type, h.ZName)})
		added++
	}
	r.Store.PutJSON("favs", toAnyList(favs), 0)
	msg := fmt.Sprintf("⭐ %d ساب به منتخب‌ها اضافه شد.", added)
	r.Show(chat, mid, msg, [][]Btn{{btn("🔙 بازگشت", fmt.Sprintf("qap:%s:0", token)), btn("🏠 خانه", "menu")}})
}

// ---------- add sub for an IP ----------

func (r *Router) qaAddZones(chat, mid int64, token string) {
	var ip string
	if token != "" {
		var s qaSess
		if r.Store.GetJSON("ips:"+token, &s) {
			ip = s.IP
		}
	} else {
		var sess map[string]any
		r.Store.GetJSON(fmt.Sprintf("qa:%d", chat), &sess)
		ip = strv(sess["ip"])
	}
	if ip == "" {
		r.Show(chat, mid, "⏳ نشست منقضی شد. دوباره آی‌پی را بفرستید.", HomeKB())
		return
	}
	var items []Btn
	for i, a := range r.cfAccs() {
		cli := cf.New(a.Token)
		zs, err := cli.Zones()
		if err != nil {
			continue
		}
		for _, z := range zs {
			if z.Status != "active" {
				continue
			}
			items = append(items, btn("📁 "+z.Name, fmt.Sprintf("qanz:%d:%s", i, z.ID)))
		}
	}
	if len(items) == 0 {
		r.Show(chat, mid, "📭 دامنه‌ای نیست.", HomeKB())
		return
	}
	kb := gridBTN(items, 2)
	kb = append(kb, navRow("qamenu"))
	r.Show(chat, mid, fmt.Sprintf("➕ افزودن ساب برای <code>%s</code>\n\nدامنه را انتخاب کن:", Esc(ip)), kb)
}

func (r *Router) qaAddZonePick(chat, mid int64, acc int, zid string) {
	var sess map[string]any
	r.Store.GetJSON(fmt.Sprintf("qa:%d", chat), &sess)
	ip := strv(sess["ip"])
	if ip == "" {
		r.Show(chat, mid, "⏳ نشست منقضی شد.", HomeKB())
		return
	}
	cli, ok := r.cfClientFor(acc)
	if !ok {
		r.Show(chat, mid, "❌ اکانت پیدا نشد.", HomeKB())
		return
	}
	zname := zid
	if zs, err := cli.Zones(); err == nil {
		for _, z := range zs {
			if z.ID == zid {
				zname = z.Name
			}
		}
	}
	r.setWizard(chat, map[string]any{"kind": "qa_addsub", "ip": ip, "acc": acc, "zid": zid, "zname": zname})
	r.Show(chat, mid, fmt.Sprintf("➕ افزودن ساب در <code>%s</code>\n\n🌐 آی‌پی: <code>%s</code>\n\nنام ساب را بفرست (مثلاً test یا @):",
		Esc(zname), Esc(ip)), [][]Btn{{btn("❌ لغو", "qacancel")}})
}

func (r *Router) qaAddSave(chat, mid int64, pend map[string]any, text string) {
	r.clearWizard(chat)
	ip := strv(pend["ip"])
	acc := intOrAny(pend["acc"])
	zid := strv(pend["zid"])
	zname := strv(pend["zname"])
	name := strings.TrimSpace(text)
	if ip == "" || zid == "" || name == "" {
		r.Show(chat, mid, "⚠️ ورودی نامعتبر.", HomeKB())
		return
	}
	if name == "@" {
		name = zname
	}
	cli, ok := r.cfClientFor(acc)
	if !ok {
		r.Show(chat, mid, "❌ اکانت پیدا نشد.", HomeKB())
		return
	}
	if _, err := cli.AddRecord(zid, "A", name, ip, 1, false, nil); err != nil {
		r.Show(chat, mid, "❌ ساخته نشد: "+Esc(err.Error()),
			[][]Btn{{btn("🔙 بازگشت", "qamenu"), btn("🏠 خانه", "menu")}})
		return
	}
	r.Show(chat, mid, fmt.Sprintf("✅ ساب <code>%s</code> با آی‌پی <code>%s</code> ساخته شد.", Esc(name), Esc(ip)),
		[][]Btn{{btn("🔙 بازگشت", "qamenu"), btn("🏠 خانه", "menu")}})
}
