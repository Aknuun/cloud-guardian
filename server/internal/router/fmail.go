package router

import (
	"fmt"
	"sort"
	"strconv"
	"strings"

	"github.com/aknuun/cg-server/internal/cf"
)

type cfZone struct {
	Zone cf.Zone
	Acc  int
}

func (r *Router) cfClientFor(acc int) (*cf.Client, bool) {
	accs := r.cfAccs()
	if acc < 0 || acc >= len(accs) {
		return nil, false
	}
	return cf.New(accs[acc].Token), true
}

func (r *Router) cfAllZones() []cfZone {
	var out []cfZone
	for i, a := range r.cfAccs() {
		zs, err := cf.New(a.Token).Zones()
		if err != nil {
			continue
		}
		for _, z := range zs {
			out = append(out, cfZone{Zone: z, Acc: i})
		}
	}
	return out
}

func mailDestsSorted(lst []map[string]any) []map[string]any {
	out := append([]map[string]any{}, lst...)
	sort.SliceStable(out, func(i, j int) bool {
		ei := strings.ToLower(strv(out[i]["email"]))
		ej := strings.ToLower(strv(out[j]["email"]))
		li := strings.SplitN(ei, "@", 2)[0]
		lj := strings.SplitN(ej, "@", 2)[0]
		ki, kj := 1, 1
		if strings.HasPrefix(li, "info") {
			ki = 0
		}
		if strings.HasPrefix(lj, "info") {
			kj = 0
		}
		if ki != kj {
			return ki < kj
		}
		return ei < ej
	})
	return out
}

// fmailScan returns a cached (300s) map of zones with an active catch-all,
// split into forward destinations and worker entries.
func (r *Router) fmailScan(chat int64) map[string]any {
	key := fmt.Sprintf("fmail_scan:%d", chat)
	var cached map[string]any
	if r.Store.GetJSON(key, &cached) {
		if fany(cached["ts"]) > 0 && nowSec()-int64(fany(cached["ts"])) < 300 {
			return cached
		}
	}
	var fwd, worker []any
	clients := map[int]*cf.Client{}
	for _, z := range r.cfAllZones() {
		cli, ok := clients[z.Acc]
		if !ok {
			cli = cf.New(r.cfAccs()[z.Acc].Token)
			clients[z.Acc] = cli
		}
		ca := cli.EmailCatchAll(z.Zone.ID)
		if ca == nil {
			continue
		}
		if b, _ := ca["enabled"].(bool); !b {
			continue
		}
		act := map[string]any{}
		if acts, ok := ca["actions"].([]any); ok && len(acts) > 0 {
			if m, ok := acts[0].(map[string]any); ok {
				act = m
			}
		}
		entry := map[string]any{"acc": z.Acc, "zid": z.Zone.ID, "zname": z.Zone.Name}
		if strv(act["type"]) == "forward" {
			var dests []string
			if vals, ok := act["value"].([]any); ok {
				for _, v := range vals {
					dests = append(dests, strv(v))
				}
			}
			entry["dest"] = strings.Join(dests, ",")
			fwd = append(fwd, entry)
		} else {
			worker = append(worker, entry)
		}
	}
	out := map[string]any{"ts": float64(nowSec()), "fwd": fwd, "worker": worker}
	r.Store.PutJSON(key, out, 600)
	return out
}

func asMapList(v any) []map[string]any {
	var out []map[string]any
	if arr, ok := v.([]any); ok {
		for _, x := range arr {
			if m, ok := x.(map[string]any); ok {
				out = append(out, m)
			}
		}
	}
	return out
}

func (r *Router) fmailHome(chat, mid int64) {
	sc := r.fmailScan(chat)
	fwd := asMapList(sc["fwd"])
	worker := asMapList(sc["worker"])
	lines := []string{"✉️ ایمیل سازمانی", "",
		"ایمیل‌های دامنه‌ها را به هر آدرس فوروارد کن.",
		"ℹ️ «دریافت در ربات» نیاز به Email Worker کلادفلر دارد و در حالت سرور در دسترس نیست."}
	var kb [][]Btn
	if len(fwd) == 0 && len(worker) == 0 {
		lines = append(lines, "", "📭 هنوز دامنه‌ای وصل نیست.")
	}
	if len(fwd) > 0 {
		lines = append(lines, "", fmt.Sprintf("📩 فوروارد به ایمیل (%d):", len(fwd)))
		for i, f := range fwd {
			if i >= 20 {
				break
			}
			lines = append(lines, fmt.Sprintf("• %s ← %s", Esc(strv(f["zname"])), Esc(strv(f["dest"]))))
			kb = append(kb, []Btn{btn(
				fmt.Sprintf("📩 %s ← %s", truncRune(strv(f["zname"]), 16), truncRune(strv(f["dest"]), 16)),
				fmt.Sprintf("fmailoff:%d:%s", int(fany(f["acc"])), strv(f["zid"])))})
		}
	}
	if len(worker) > 0 {
		lines = append(lines, "", fmt.Sprintf("📥 (ورکر) فعال (%d):", len(worker)))
		for i, w := range worker {
			if i >= 20 {
				break
			}
			lines = append(lines, "• "+Esc(strv(w["zname"])))
			kb = append(kb, []Btn{btn("📥 "+truncRune(strv(w["zname"]), 24),
				fmt.Sprintf("fmailoff:%d:%s", int(fany(w["acc"])), strv(w["zid"])))})
		}
	}
	kb = append(kb, []Btn{btnS("➕ افزودن ایمیل (وصل دامنه جدید)", "fmailadd", "success")})
	kb = append(kb, []Btn{btn("🔄 همگام‌سازی", "fmailsync"), btn("🏠 خانه", "menu")})
	txt := strings.Join(lines, "\n")
	if len(txt) > 3500 {
		txt = txt[:3500]
	}
	r.Show(chat, mid, txt, kb)
}

func (r *Router) fmailAdd(chat, mid int64) {
	sc := r.fmailScan(chat)
	on := map[string]bool{}
	for _, f := range append(asMapList(sc["fwd"]), asMapList(sc["worker"])...) {
		on[strv(f["zid"])] = true
	}
	var zones []cfZone
	for _, z := range r.cfAllZones() {
		if !on[z.Zone.ID] {
			zones = append(zones, z)
		}
	}
	lines := []string{"➕ وصل دامنه جدید", "", "دامنه را بزن و حالت دریافت را انتخاب کن:"}
	var kb [][]Btn
	if len(zones) == 0 {
		if len(on) > 0 {
			lines = append(lines, "", "همهٔ دامنه‌ها تنظیم شده‌اند.")
		} else {
			lines = append(lines, "", "📭 دامنه‌ای پیدا نشد.")
		}
	}
	for i, z := range zones {
		if i >= 20 {
			break
		}
		kb = append(kb, []Btn{btnS("➕ "+z.Zone.Name,
			fmt.Sprintf("fmode:%d:%s", z.Acc, z.Zone.ID), "success")})
	}
	if len(zones) > 20 {
		lines = append(lines, "", fmt.Sprintf("… و %d دامنه دیگر", len(zones)-20))
	}
	kb = append(kb, []Btn{btn("🔙 بازگشت", "fmail"), btn("🏠 خانه", "menu")})
	txt := strings.Join(lines, "\n")
	if len(txt) > 3500 {
		txt = txt[:3500]
	}
	r.Show(chat, mid, txt, kb)
}

func (r *Router) fmailMode(chat, mid int64, acc, zid string) {
	name := zid
	if cli, ok := r.cfClientFor(atoiSafe(acc)); ok {
		if z, err := cli.Zone(zid); err == nil {
			if n := strv(z["name"]); n != "" {
				name = n
			}
		}
	}
	r.Show(chat, mid,
		fmt.Sprintf("🔀 %s — ایمیل‌ها چطور دریافت شوند؟\n\n"+
			"• 📩 فوروارد به ایمیل: مستقیم به آدرس مقصد می‌رود (روتینگ کلادفلر)\n"+
			"• 📥 دریافت در ربات: در حالت سرور در دسترس نیست (نیاز به Email Worker)\n\n"+
			"📧 آدرس‌ها به شکل <code>name@%s</code> هستند.", Esc(name), Esc(name)),
		[][]Btn{
			{btnS("📩 فوروارد به ایمیل مقصد", fmt.Sprintf("fmailfwd:%s:%s", acc, zid), "success")},
			{btn("🔙 بازگشت", "fmail"), btn("🏠 خانه", "menu")}})
}

func (r *Router) fmailFwdPick(chat, mid int64, acc, zid string) {
	cli, ok := r.cfClientFor(atoiSafe(acc))
	if !ok {
		r.Show(chat, mid, "❌ اکانت پیدا نشد.", [][]Btn{{btn("🔙 بازگشت", "fmail")}})
		return
	}
	aid := cli.AccountID(zid)
	lst, err := cli.EmailDests(aid)
	if err != nil {
		r.Show(chat, mid, "❌ "+Esc(err.Error()), [][]Btn{{btn("🔙 بازگشت", "fmail")}})
		return
	}
	lst = mailDestsSorted(lst)
	var verified, pending []map[string]any
	for _, d := range lst {
		if b, _ := d["verified"].(bool); b {
			verified = append(verified, d)
		} else {
			pending = append(pending, d)
		}
	}
	lines := []string{"📩 به کدام آدرس فوروارد شود؟", ""}
	var kb [][]Btn
	var verifiedEmails []string
	for i, d := range verified {
		if i >= 10 {
			break
		}
		kb = append(kb, []Btn{btn("✅ "+strv(d["email"]),
			fmt.Sprintf("fmailfwdc:%s:%s:%d", acc, zid, i))})
		verifiedEmails = append(verifiedEmails, strv(d["email"]))
	}
	if len(pending) > 0 {
		var pe []string
		for i, d := range pending {
			if i >= 3 {
				break
			}
			pe = append(pe, strv(d["email"]))
		}
		lines = append(lines, "⏳ در انتظار تأیید: "+strings.Join(pe, "، "))
	}
	if len(verified) == 0 {
		lines = append(lines, "هنوز مقصد تأییدشده‌ای نیست؛ اول یکی اضافه کن.")
	}
	kb = append(kb, []Btn{btn("➕ مقصد جدید", fmt.Sprintf("fmailfwdnew:%s:%s", acc, zid))})
	kb = append(kb, []Btn{btn("🔙 بازگشت", "fmail"), btn("🏠 خانه", "menu")})
	r.Store.PutJSON(fmt.Sprintf("fmailctx:%d", chat),
		map[string]any{"verified": verifiedEmails}, 600)
	r.Show(chat, mid, strings.Join(lines, "\n"), kb)
}

func (r *Router) fmailFwdDo(chat, mid int64, acc, zid, idx string) {
	var ctx map[string]any
	r.Store.GetJSON(fmt.Sprintf("fmailctx:%d", chat), &ctx)
	verified := selStrings(ctx["verified"])
	i := atoiSafe(idx)
	if i < 0 || i >= len(verified) {
		r.Show(chat, mid, "❌ مقصد پیدا نشد.", [][]Btn{{btn("🔙 بازگشت", "fmail")}})
		return
	}
	dest := verified[i]
	cli, ok := r.cfClientFor(atoiSafe(acc))
	if !ok {
		r.Show(chat, mid, "❌ اکانت پیدا نشد.", [][]Btn{{btn("🔙 بازگشت", "fmail")}})
		return
	}
	name := zid
	if z, err := cli.Zone(zid); err == nil {
		if n := strv(z["name"]); n != "" {
			name = n
		}
	}
	if err := cli.EmailCatchAllSet(zid, map[string]any{
		"enabled": true,
		"actions": []any{map[string]any{"type": "forward", "value": []string{dest}}}}); err != nil {
		r.Show(chat, mid, "❌ خطا: "+Esc(err.Error()), [][]Btn{{btn("🔙 بازگشت", "fmail")}})
		return
	}
	r.Store.Delete(fmt.Sprintf("fmail_scan:%d", chat))
	r.Show(chat, mid,
		fmt.Sprintf("📩 %s به %s فوروارد می‌شود.\n\n"+
			"⚠️ ایمیل‌های آینده مستقیم به مقصد می‌روند و وارد ربات نمی‌شوند.",
			Esc(name), Esc(dest)),
		[][]Btn{{btn("🔙 بازگشت", "fmail"), btn("🏠 خانه", "menu")}})
}

func (r *Router) fmailOff(chat, mid int64, acc, zid string) {
	cli, ok := r.cfClientFor(atoiSafe(acc))
	if !ok {
		r.Show(chat, mid, "❌ اکانت پیدا نشد.", [][]Btn{{btn("🔙 بازگشت", "fmail")}})
		return
	}
	name := zid
	if z, err := cli.Zone(zid); err == nil {
		if n := strv(z["name"]); n != "" {
			name = n
		}
	}
	if err := cli.EmailCatchAllSet(zid, map[string]any{
		"enabled": false,
		"actions": []any{map[string]any{"type": "drop", "value": []string{}}}}); err != nil {
		r.Show(chat, mid, "❌ خطا: "+Esc(err.Error()), [][]Btn{{btn("🔙 بازگشت", "fmail")}})
		return
	}
	r.Store.Delete(fmt.Sprintf("fmail_scan:%d", chat))
	r.Show(chat, mid, fmt.Sprintf("📩 دریافت/فوروارد %s قطع شد.", Esc(name)),
		[][]Btn{{btn("🔙 بازگشت", "fmail"), btn("🏠 خانه", "menu")}})
}

func (r *Router) fmailFwdNewAsk(chat, mid int64, acc, zid string) {
	r.setWizard(chat, map[string]any{"kind": "fmail_dest_new", "acc": acc, "zid": zid})
	r.Show(chat, mid, "📧 ایمیل مقصد (مثلاً جیمیل خودت) را بفرست؛ لینک تأیید به همان ایمیل می‌رود:",
		[][]Btn{{btn("⬅️ انصراف", fmt.Sprintf("fmailfwd:%s:%s", acc, zid))}})
}

func (r *Router) fmailDestNew(chat, mid int64, pend map[string]any, text string) {
	acc, _ := pend["acc"].(string)
	zid, _ := pend["zid"].(string)
	r.clearWizard(chat)
	email := strings.TrimSpace(text)
	cli, ok := r.cfClientFor(atoiSafe(acc))
	if !ok {
		r.Bot.Send(chat, "❌ اکانت پیدا نشد.", nil)
		return
	}
	aid := cli.AccountID(zid)
	if err := cli.EmailDestAdd(aid, email); err != nil {
		r.Bot.Send(chat, "❌ خطا: "+Esc(err.Error()), nil)
	} else {
		r.Bot.Send(chat, "✅ مقصد اضافه شد: "+Esc(email)+
			"\nلینک تأیید به همین ایمیل رفت؛ بعد از تأیید دوباره این بخش را باز کن.", nil)
	}
	r.Store.Delete(fmt.Sprintf("fmail_scan:%d", chat))
	r.fmailFwdPick(chat, 0, acc, zid)
}

func atoiSafe(s string) int {
	n, _ := strconv.Atoi(strings.TrimSpace(s))
	return n
}
