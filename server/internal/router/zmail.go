package router

import (
	"fmt"
	"strconv"
	"strings"
)

// ---------- per-zone Email Routing editor (zmail) ----------

func (r *Router) zmailHome(chat, mid int64) {
	acc, zid, zname, cli, ok := r.cfZctx(chat)
	if !ok {
		r.Show(chat, mid, "⏳ نشست منقضی شد؛ دامنه را دوباره باز کن.", HomeKB())
		return
	}
	st := cli.EmailRouting(zid)
	if st == nil {
		r.Show(chat, mid,
			fmt.Sprintf("✉️ ایمیل %s\n\n❌ دسترسی به Email Routing نیست.\n\n"+
				"به توکن دسترسی <b>Zone → Email Routing Rules → Edit</b> را اضافه کن.", Esc(zname)),
			navKB("zset"))
		return
	}
	enabled, _ := st["enabled"].(bool)
	lines := []string{fmt.Sprintf("✉️ ایمیل %s", Esc(zname)), "",
		"وضعیت: " + map[bool]string{true: "✅ فعال", false: "⏸ غیرفعال"}[enabled]}
	if s := strv(st["status"]); s != "" {
		lines[len(lines)-1] += " (" + Esc(s) + ")"
	}
	var kb [][]Btn
	if !enabled {
		lines = append(lines, "", "با فعال‌سازی، رکوردهای MX/SPF لازم خودکار ساخته می‌شوند.")
		kb = append(kb, []Btn{btnS("✉️ فعال‌سازی ایمیل", "zmailon", "success")})
	} else {
		rules, err := cli.EmailRules(zid)
		if err != nil {
			rules = nil
		}
		ca := cli.EmailCatchAll(zid)
		var customs []map[string]any
		for _, rule := range rules {
			isAll := false
			for _, m := range asMapList(rule["matchers"]) {
				if strv(m["type"]) == "all" {
					isAll = true
				}
			}
			if !isAll {
				customs = append(customs, rule)
			}
		}
		lines = append(lines, "", fmt.Sprintf("📧 %d قانون فوروارد:", len(customs)))
		for i, rule := range customs {
			if i >= 12 {
				break
			}
			label := strv(rule["name"])
			for _, m := range asMapList(rule["matchers"]) {
				if strv(m["type"]) == "literal" {
					label = strv(m["value"])
				}
			}
			dest := r.ruleDest(rule)
			lines = append(lines, fmt.Sprintf("• %s → %s", Esc(label), Esc(dest)))
			kb = append(kb, []Btn{btnS("🗑 "+truncRune(label, 32), "zmaildel:"+strv(rule["id"]), "danger")})
		}
		if len(customs) > 12 {
			lines = append(lines, fmt.Sprintf("… و %d مورد دیگر", len(customs)-12))
		}
		lines = append(lines, "", "📥 catch-all: "+Esc(r.catchAllDesc(ca)))
		kb = append(kb, []Btn{btnS("➕ آدرس جدید", "zmailadd", "success")})
		kb = append(kb, []Btn{btn("📥 catch-all", "zmailcatch"), btn("📧 مقصدها", "zmaildests")})
	}
	kb = append(kb, []Btn{btn("🔙 بازگشت", "zset"), btn("🏠 خانه", "menu")})
	r.Show(chat, mid, strings.Join(lines, "\n"), kb)
	_ = acc
}

func (r *Router) ruleDest(rule map[string]any) string {
	acts := asMapList(rule["actions"])
	if len(acts) == 0 {
		return "؟"
	}
	var vals []string
	for _, v := range toList(acts[0]["value"]) {
		vals = append(vals, strv(v))
	}
	if len(vals) > 0 {
		return strings.Join(vals, ",")
	}
	return strv(acts[0]["type"])
}

func (r *Router) catchAllDesc(ca map[string]any) string {
	if ca == nil {
		return "—"
	}
	if b, _ := ca["enabled"].(bool); !b {
		return "⏸ غیرفعال"
	}
	acts := asMapList(ca["actions"])
	if len(acts) == 0 {
		return "—"
	}
	var vals []string
	for _, v := range toList(acts[0]["value"]) {
		vals = append(vals, strv(v))
	}
	if len(vals) > 0 {
		return "✅ " + strings.Join(vals, ",")
	}
	return "✅ " + strv(acts[0]["type"])
}

func (r *Router) zmailEnableAsk(chat, mid int64) {
	_, _, zname, _, ok := r.cfZctx(chat)
	if !ok {
		r.Show(chat, mid, "⏳ نشست منقضی شد.", HomeKB())
		return
	}
	r.Show(chat, mid,
		fmt.Sprintf("✉️ ایمیل روی %s فعال شود؟\n\nرکوردهای MX/SPF لازم خودکار ساخته می‌شوند.\n"+
			"⚠️ اگر الان سرویس ایمیل دیگری (مثل گوگل) روی این دامنه داری، خراب می‌شود.", Esc(zname)),
		[][]Btn{{btnS("✅ بله، فعال کن", "zmailony", "success"), btn("❌ انصراف", "zmail")}})
}

func (r *Router) zmailEnableDo(chat, mid int64) {
	_, zid, _, cli, ok := r.cfZctx(chat)
	if !ok {
		r.Show(chat, mid, "⏳ نشست منقضی شد.", HomeKB())
		return
	}
	if err := cli.EmailEnable(zid); err != nil {
		r.Show(chat, mid, "❌ خطا: "+Esc(err.Error()), [][]Btn{{btn("✉️ ایمیل", "zmail")}})
		return
	}
	r.Show(chat, mid, "✅ ایمیل فعال شد.", [][]Btn{{btn("✉️ ایمیل", "zmail")}})
}

func (r *Router) zmailAddStart(chat, mid int64) {
	_, _, zname, _, ok := r.cfZctx(chat)
	if !ok {
		r.Show(chat, mid, "⏳ نشست منقضی شد.", HomeKB())
		return
	}
	r.setWizard(chat, map[string]any{"kind": "zmail_addr"})
	r.Show(chat, mid,
		fmt.Sprintf("➕ آدرس جدید (مثلاً <code>info@%s</code> یا فقط <code>info</code>):", Esc(zname)),
		[][]Btn{{btn("⬅️ انصراف", "zmail")}})
}

func (r *Router) zmailAddrSave(chat, mid int64, text string) {
	r.clearWizard(chat)
	_, _, zname, _, ok := r.cfZctx(chat)
	if !ok {
		r.Show(chat, mid, "⏳ نشست منقضی شد.", HomeKB())
		return
	}
	local := strings.TrimSpace(text)
	if strings.Contains(local, "@") {
		parts := strings.SplitN(local, "@", 2)
		local = parts[0]
	}
	local = strings.TrimSpace(local)
	if local == "" {
		r.Show(chat, mid, "❌ آدرس نامعتبر.", navKB("zmail"))
		return
	}
	addr := local + "@" + zname
	sess := r.cfGetSession(chat)
	sess["mail_addr"] = addr
	sess["mail_local"] = local
	r.cfSaveSession(chat, sess)
	r.zmailDestListPick(chat, mid, false)
}

func (r *Router) zmailDestListPick(chat, mid int64, forCatch bool) {
	_, zid, _, cli, ok := r.cfZctx(chat)
	if !ok {
		r.Show(chat, mid, "⏳ نشست منقضی شد.", HomeKB())
		return
	}
	aid := cli.AccountID(zid)
	lst, err := cli.EmailDests(aid)
	if err != nil {
		r.Show(chat, mid, "❌ "+Esc(err.Error()), navKB("zmail"))
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
	head := "📥 مقصد catch-all:"
	if !forCatch {
		sess := r.cfGetSession(chat)
		head = fmt.Sprintf("📧 فوروارد %s به کدام مقصد؟", Esc(strv(sess["mail_addr"])))
	}
	lines := []string{head, ""}
	var kb [][]Btn
	var verifiedEmails []string
	cb := "zmailcset"
	if !forCatch {
		cb = "zmaildest"
	}
	for i, d := range verified {
		if i >= 10 {
			break
		}
		kb = append(kb, []Btn{btnS("✅ "+strv(d["email"]), fmt.Sprintf("%s:%d", cb, i), "plain")})
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
		lines = append(lines, "⏳ در انتظار تأیید ("+strconv.Itoa(len(pending))+"): "+strings.Join(pe, "، "),
			"روی لینک داخل ایمیل تأیید کلیک کن، بعد برگرد.")
	}
	if len(verified) == 0 {
		lines = append(lines, "هنوز مقصد تأییدشده‌ای نیست؛ اول یکی اضافه کن.")
	}
	kb = append(kb, []Btn{btn("➕ مقصد جدید", "zmaildestnew")})
	kb = append(kb, []Btn{btn("🔙 بازگشت", "zmail"), btn("🏠 خانه", "menu")})
	r.Store.PutJSON(fmt.Sprintf("zmailctx:%d", chat), map[string]any{"verified": verifiedEmails}, 600)
	r.Show(chat, mid, strings.Join(lines, "\n"), kb)
}

func (r *Router) zmailDestPick(chat, mid int64, idx string, forCatch bool) {
	_, zid, _, cli, ok := r.cfZctx(chat)
	if !ok {
		r.Show(chat, mid, "⏳ نشست منقضی شد.", HomeKB())
		return
	}
	var zctx map[string]any
	r.Store.GetJSON(fmt.Sprintf("zmailctx:%d", chat), &zctx)
	dests := selStrings(zctx["verified"])
	i := atoiSafe(idx)
	if i < 0 || i >= len(dests) {
		r.Show(chat, mid, "❌ مقصد پیدا نشد.", navKB("zmail"))
		return
	}
	dest := dests[i]
	var msg string
	if forCatch {
		if err := cli.EmailCatchAllSet(zid, map[string]any{
			"enabled": true,
			"actions": []any{map[string]any{"type": "forward", "value": []string{dest}}}}); err != nil {
			msg = "❌ خطا: " + Esc(err.Error())
		} else {
			msg = fmt.Sprintf("✅ catch-all به %s فوروارد می‌شود.", Esc(dest))
		}
	} else {
		sess := r.cfGetSession(chat)
		addr := strv(sess["mail_addr"])
		local := strings.SplitN(addr, "@", 2)[0]
		if err := cli.EmailRuleAdd(zid, local, addr, dest); err != nil {
			msg = "❌ خطا: " + Esc(err.Error())
		} else {
			delete(sess, "mail_addr")
			delete(sess, "mail_local")
			r.cfSaveSession(chat, sess)
			msg = fmt.Sprintf("✅ ساخته شد:\n%s → %s", Esc(addr), Esc(dest))
		}
	}
	r.Show(chat, mid, msg, [][]Btn{{btn("✉️ ایمیل", "zmail")}})
}

func (r *Router) zmailDests(chat, mid int64) {
	_, zid, _, cli, ok := r.cfZctx(chat)
	if !ok {
		r.Show(chat, mid, "⏳ نشست منقضی شد.", HomeKB())
		return
	}
	aid := cli.AccountID(zid)
	lst, err := cli.EmailDests(aid)
	if err != nil {
		r.Show(chat, mid, "❌ "+Esc(err.Error()), navKB("zmail"))
		return
	}
	lines := []string{fmt.Sprintf("📧 مقصدهای اکانت (%d):", len(lst)), ""}
	for i, d := range lst {
		if i >= 15 {
			break
		}
		st := "⏳ در انتظار تأیید"
		if b, _ := d["verified"].(bool); b {
			st = "✅ تأییدشده"
		}
		lines = append(lines, fmt.Sprintf("• %s — %s", Esc(strv(d["email"])), st))
	}
	if len(lst) > 15 {
		lines = append(lines, fmt.Sprintf("… و %d مورد دیگر", len(lst)-15))
	}
	r.Show(chat, mid, strings.Join(lines, "\n"),
		[][]Btn{{btn("➕ مقصد جدید", "zmaildestnew")}, {btn("🔙 بازگشت", "zmail"), btn("🏠 خانه", "menu")}})
}

func (r *Router) zmailDelAsk(chat, mid int64, rid string) {
	r.Show(chat, mid, "⚠️ این قانون حذف شود؟",
		[][]Btn{{btnS("✅ بله، حذف کن", "zmaildely:"+rid, "danger"), btn("❌ انصراف", "zmail")}})
}

func (r *Router) zmailDelDo(chat, mid int64, rid string) {
	_, zid, _, cli, ok := r.cfZctx(chat)
	if !ok {
		r.Show(chat, mid, "⏳ نشست منقضی شد.", HomeKB())
		return
	}
	if err := cli.EmailRuleDel(zid, rid); err != nil {
		r.Show(chat, mid, "❌ خطا: "+Esc(err.Error()), [][]Btn{{btn("✉️ ایمیل", "zmail")}})
		return
	}
	r.Show(chat, mid, "✅ قانون حذف شد.", [][]Btn{{btn("✉️ ایمیل", "zmail")}})
}

func (r *Router) zmailCatch(chat, mid int64) {
	_, zid, zname, cli, ok := r.cfZctx(chat)
	if !ok {
		r.Show(chat, mid, "⏳ نشست منقضی شد.", HomeKB())
		return
	}
	ca := cli.EmailCatchAll(zid)
	r.Show(chat, mid,
		fmt.Sprintf("📥 catch-all در %s (ایمیل‌هایی که قانون ندارند):\n\nوضعیت: %s",
			Esc(zname), Esc(r.catchAllDesc(ca))),
		[][]Btn{{btn("📩 فوروارد به مقصد", "zmailcpick")},
			{btnS("⏸ غیرفعال", "zmailcoff", "danger"), btn("🔙 بازگشت", "zmail")}})
}

func (r *Router) zmailCatchOff(chat, mid int64) {
	_, zid, _, cli, ok := r.cfZctx(chat)
	if !ok {
		r.Show(chat, mid, "⏳ نشست منقضی شد.", HomeKB())
		return
	}
	if err := cli.EmailCatchAllSet(zid, map[string]any{
		"enabled": false,
		"actions": []any{map[string]any{"type": "drop", "value": []string{}}}}); err != nil {
		r.Show(chat, mid, "❌ خطا: "+Esc(err.Error()), [][]Btn{{btn("✉️ ایمیل", "zmail")}})
		return
	}
	r.Show(chat, mid, "📩 catch-all غیرفعال شد.", [][]Btn{{btn("✉️ ایمیل", "zmail")}})
}

func (r *Router) zmailDestNewAsk(chat, mid int64) {
	_, zid, _, cli, ok := r.cfZctx(chat)
	if !ok {
		r.Show(chat, mid, "⏳ نشست منقضی شد.", HomeKB())
		return
	}
	if cli.AccountID(zid) == "" {
		r.Show(chat, mid, "❌ شناسهٔ اکانت پیدا نشد.", navKB("zmail"))
		return
	}
	r.setWizard(chat, map[string]any{"kind": "zmail_dest_new"})
	r.Show(chat, mid, "📧 ایمیل مقصد (مثلاً جیمیل خودت) را بفرست؛ لینک تأیید به همان ایمیل می‌رود:",
		[][]Btn{{btn("⬅️ انصراف", "zmail")}})
}

func (r *Router) zmailDestNewSave(chat, mid int64, text string) {
	r.clearWizard(chat)
	_, zid, _, cli, ok := r.cfZctx(chat)
	if !ok {
		r.Bot.Send(chat, "❌ نشست منقضی شد.", nil)
		return
	}
	email := strings.TrimSpace(text)
	aid := cli.AccountID(zid)
	if aid == "" {
		r.Bot.Send(chat, "❌ شناسهٔ اکانت پیدا نشد.", nil)
		return
	}
	if err := cli.EmailDestAdd(aid, email); err != nil {
		r.Bot.Send(chat, "❌ خطا: "+Esc(err.Error()), nil)
	} else {
		r.Bot.Send(chat, "✅ مقصد اضافه شد: "+Esc(email)+
			"\nلینک تأیید به همین ایمیل رفت؛ بعد از تأیید دوباره این بخش را باز کن.", nil)
	}
	r.zmailDestListPick(chat, 0, false)
}
