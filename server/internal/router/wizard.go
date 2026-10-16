package router

import (
	"fmt"
	"strconv"
	"strings"

	"github.com/aknuun/cg-server/internal/sslmon"
)

func (r *Router) setWizard(chat int64, v map[string]any) {
	r.Store.PutJSON(fmt.Sprintf("pend:%d", chat), v, 3600)
}

func (r *Router) getWizard(chat int64) map[string]any {
	var v map[string]any
	if r.Store.GetJSON(fmt.Sprintf("pend:%d", chat), &v) {
		return v
	}
	return nil
}

func (r *Router) clearWizard(chat int64) {
	r.Store.Delete(fmt.Sprintf("pend:%d", chat))
}

func (r *Router) wizardStep(chat, mid int64, pend map[string]any, text string) {
	kind, _ := pend["kind"].(string)
	switch kind {
	case "admin_add":
		id, err := strconv.ParseInt(strings.TrimSpace(text), 10, 64)
		if err != nil || id <= 0 {
			r.Show(chat, mid, "❌ شناسهٔ نامعتبر. یک عدد بفرست:", [][]Btn{{btn("🔙 لغو", "admins_menu")}})
			return
		}
		cur := r.Admins()
		if !containsI64(cur, id) {
			cur = append(cur, id)
		}
		r.SetAdmins(cur)
		r.clearWizard(chat)
		r.Show(chat, mid, fmt.Sprintf("✅ ادمین <code>%d</code> اضافه شد.", id),
			[][]Btn{{btn("👥 مدیریت ادمین‌ها", "admins_menu"), btn("🏠 خانه", "menu")}})
	case "pg_add_url":
		r.pgAddURL(chat, mid, text)
	case "pg_add_user":
		r.pgAddUser(chat, mid, pend, text)
	case "pg_add_pass":
		r.pgAddPass(chat, mid, pend, text)
	case "pg_search":
		r.pgSearchDo(chat, mid, pend, text)
	case "pg_dashpath":
		r.pgDashSave(chat, mid, pend, text)
	case "pg_renewbot":
		r.pgRenewbotSave(chat, mid, pend, text)
	case "pg_recur":
		r.pgRecurSave(chat, mid, pend, text)
	case "p_ename":
		r.pnleSave(chat, mid, "N", intOrAny(pend["idx"]), text)
	case "p_eurl":
		r.pnleSave(chat, mid, "U", intOrAny(pend["idx"]), text)
	case "p_euser":
		r.pnleSave(chat, mid, "P", intOrAny(pend["idx"]), text)
	case "p_epass":
		r.pnleSave(chat, mid, "W", intOrAny(pend["idx"]), text)
	case "um_num":
		r.umNumSave(chat, mid, pend, text)
	case "quota_limit":
		r.quotaLimitSave(chat, mid, text)
	case "hub_broadcast":
		r.hubCastSave(chat, mid, text)
	case "hub_contact":
		r.hubContactSave(chat, mid, text)
	case "hub_reply":
		r.hubReplySave(chat, mid, pend, text)
	case "hub_annreply":
		r.hubAnnReplySave(chat, mid, pend, text)
	case "fo_token":
		r.foTokenSave(chat, mid, text)
	case "fo_name":
		r.foNameSave(chat, mid, pend, text)
	case "hub_set_hub":
		r.hubCfgField(chat, mid, "hub_set_hub", text)
	case "hub_set_secret":
		r.hubCfgField(chat, mid, "hub_set_secret", text)
	case "hub_set_ann":
		r.hubCfgField(chat, mid, "hub_set_ann", text)
	case "nd_int":
		id, _ := pend["id"].(string)
		r.ndIntSave(chat, mid, id, text)
	case "hf_num":
		key, _ := pend["key"].(string)
		r.hfNumSave(chat, mid, key, text)
	case "fmail_dest_new":
		r.fmailDestNew(chat, mid, pend, text)
	case "zmail_addr":
		r.zmailAddrSave(chat, mid, text)
	case "zmail_dest_new":
		r.zmailDestNewSave(chat, mid, text)
	case "lb_weight":
		r.lbWeightSave(chat, mid, pend, text)
	case "lb_spare":
		r.lbSpareSave(chat, mid, pend, text)
	case "lb_add":
		r.lbAddSave(chat, mid, pend, text)
	case "arvan_key":
		r.arvanKeySave(chat, mid, text)
	case "arvan_value":
		r.arvanValueSave(chat, mid, pend, text)
	case "arvan_add":
		r.arvanAddSave(chat, mid, pend, text)
	case "arvan_region":
		r.arvRegionSave(chat, mid, pend, text)
	case "arvan_srv_snap":
		r.arvSnapSave(chat, mid, pend, text)
	case "arvan_srv_rename":
		r.arvRenameSave(chat, mid, pend, text)
	case "hfd_int":
		r.hfdIntSave(chat, mid, pend, text)
	case "hfd_at":
		r.hfdAtSave(chat, mid, pend, text)
	case "ln_add_name":
		r.lnAddName(chat, mid, text)
	case "ln_add_token":
		r.lnAddToken(chat, mid, pend, text)
	case "ln_srv_remark":
		r.lnCreateRemark(chat, mid, pend, text)
	case "ln_srv_rename":
		r.lnRename(chat, mid, pend, text)
	case "hz_add_name":
		r.hzAddName(chat, mid, text)
	case "hz_add_token":
		r.hzAddToken(chat, mid, pend, text)
	case "hz_srv_remark":
		r.hzRemark(chat, mid, pend, text)
	case "hz_srv_rename":
		r.hzRename(chat, mid, pend, text)
	case "srv_add_host":
		r.srvWizAddHost(chat, mid, pend, text)
	case "srv_add_user":
		r.srvWizAddUserSave(chat, mid, pend, text)
	case "srv_add_secret":
		r.srvWizAddSecret(chat, mid, pend, text)
	case "srv_bulk":
		r.srvWizBulk(chat, mid, text)
	case "srv_edit":
		r.srvWizEdit(chat, mid, pend, text)
	case "srv_pw_add":
		r.srvPwAddSave(chat, mid, text)
	case "srv_sub_pw":
		pw := strings.TrimSpace(text)
		if pw == "-" {
			pw = ""
		}
		r.srvSubFinish(chat, mid, "", false, &pw)
	case "srvmon_num":
		r.srvmonNumSave(chat, mid, pend, text)
	case "cr_gsearch":
		r.clearWizard(chat)
		r.cfGsearch(chat, mid, text)
	case "cr_zsearch":
		r.clearWizard(chat)
		r.cfZsearch(chat, mid, text)
	case "qa_addsub":
		r.qaAddSave(chat, mid, pend, text)
	case "cr_cf_acc_name":
		r.cfAccName(chat, mid, text)
	case "cr_cf_acc_token":
		r.cfAccToken(chat, mid, pend, text)
	case "cr_ar_name":
		r.cfArName(chat, mid, pend, text)
	case "cr_ar_content":
		r.cfArContent(chat, mid, pend, text)
	case "cr_ar_priority":
		r.cfArPriority(chat, mid, pend, text)
	case "cr_cfedit":
		r.cfEditSave(chat, mid, pend, text)
	case "cr_bulkedit":
		r.cfBulkEditSave(chat, mid, text)
	case "domexp_domain":
		r.domexpWizardDomain(chat, mid, text)
	case "domexp_key":
		r.domexpWizardKey(chat, text)
	case "domexp_date":
		r.domexpWizardDate(chat, mid, pend, text)
	case "ssl_host":
		host := strings.ToLower(strings.TrimSpace(text))
		if host == "" || !strings.Contains(host, ".") {
			r.Show(chat, mid, "❌ دامنهٔ نامعتبر. دوباره بفرست:", [][]Btn{{btn("🔙 لغو", "sslm")}})
			return
		}
		mons := sslmon.GetMonitors(r.Store)
		for _, m := range mons {
			if m.Host == host {
				r.clearWizard(chat)
				r.Show(chat, mid, "ℹ️ این دامنه از قبل ثبت شده.",
					[][]Btn{{btn("🔐 مانیتور SSL", "sslm"), btn("🏠 خانه", "menu")}})
				return
			}
		}
		mons = append(mons, sslmon.Monitor{Host: host})
		r.Store.PutJSON("ssl_monitor", mons, 0)
		r.clearWizard(chat)
		r.Show(chat, mid, fmt.Sprintf("✅ <code>%s</code> اضافه شد.", Esc(host)),
			[][]Btn{{btn("🔐 مانیتور SSL", "sslm"), btn("🏠 خانه", "menu")}})
	case "rem_text":
		pend["text"] = strings.TrimSpace(text)
		pend["kind"] = "rem_when"
		r.setWizard(chat, pend)
		r.remWhen(chat, mid)
	case "rem_abs_day":
		pend["absDay"] = strings.TrimSpace(text)
		pend["kind"] = "rem_abs_time"
		r.setWizard(chat, pend)
		r.Show(chat, mid, "🕓 ساعت را به شکل <code>14:30</code> بفرست:",
			[][]Btn{{btn("🔙 لغو", "rem")}})
	case "rem_abs_time":
		pend["absTime"] = strings.TrimSpace(text)
		r.finalizeRem(chat, mid, pend)
	case "rem_rel_num":
		n, err := strconv.Atoi(strings.TrimSpace(text))
		if err != nil || n <= 0 {
			r.Show(chat, mid, "❌ عدد نامعتبر:", [][]Btn{{btn("🔙 لغو", "rem")}})
			return
		}
		pend["num"] = n
		r.finalizeRem(chat, mid, pend)
	default:
		r.clearWizard(chat)
		r.Menu(chat, 0)
	}
}
