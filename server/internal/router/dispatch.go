package router

import (
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/aknuun/cg-server/internal/hostfilter"
)

// navPush records a rendered screen in the per-chat history (deduplicating
// consecutive entries) so the back button can really go back.
func (r *Router) navPush(chat int64, data string) {
	if data == "" || data == "noop" || strings.HasPrefix(data, "back") {
		return
	}
	v, _ := r.navStack.Load(chat)
	st, _ := v.([]string)
	if n := len(st); n > 0 && st[n-1] == data {
		return
	}
	st = append(st, data)
	if len(st) > 30 {
		st = st[len(st)-30:]
	}
	r.navStack.Store(chat, st)
}

// navBack pops the history and re-renders the previous screen. When there is
// no history it falls back to the target encoded after "back:".
func (r *Router) navBack(chat, mid int64, data string) {
	r.clearWizard(chat)
	fallback := strings.TrimPrefix(strings.TrimPrefix(data, "back"), ":")
	v, _ := r.navStack.Load(chat)
	st, _ := v.([]string)
	if len(st) > 0 {
		prev := st[len(st)-1]
		r.navStack.Store(chat, st[:len(st)-1])
		r.navSkip.Store(chat, true)
		r.dispatch(chat, mid, prev)
		return
	}
	if fallback != "" {
		r.navSkip.Store(chat, true)
		r.dispatch(chat, mid, fallback)
		return
	}
	r.Menu(chat, mid)
}

func (r *Router) dispatch(chat, mid int64, data string) {
	if strings.HasPrefix(data, "wblogin:") {
		r.webLoginCallback(chat, mid, data)
		return
	}
	if data == "back" || strings.HasPrefix(data, "back:") {
		r.navBack(chat, mid, data)
		return
	}
	if isNavScreen(data) {
		r.clearWizard(chat)
	}
	r.lastNav.Store(chat, data)
	if _, skip := r.navSkip.LoadAndDelete(chat); !skip {
		r.navPush(chat, data)
	}
	switch {
	case data == "more":
		r.moreMenu(chat, mid)
	case data == "menu":
		r.Menu(chat, mid)
	case data == "settings":
		r.settingsHome(chat, mid)
	case data == "fo":
		r.failoverHome(chat, mid)
	case data == "fosetup":
		r.foSetupStart(chat, mid)
	case data == "fodeploy":
		if w := r.getWizard(chat); w != nil {
			r.foDeploy(chat, w)
		} else {
			r.failoverHome(chat, mid)
		}
	case data == "forefresh":
		r.foRefresh(chat, mid)
	case data == "fo_skip":
		r.Store.Put("fo:prompted", "1", 0)
		r.Menu(chat, mid)
	case data == "onboard_skip":
		r.Menu(chat, mid)
	case data == "fodisable":
		r.foDisable(chat, mid)
	case data == "fodelete":
		r.foDelete(chat, mid)
	case data == "fodelete_yes":
		r.foDeleteYes(chat, mid)
	case data == "foold":
		r.foOldWorkers(chat, mid)
	case strings.HasPrefix(data, "fooldcron:"):
		r.foOldCron(chat, mid, data[10:])
	case strings.HasPrefix(data, "foolddel:"):
		r.foOldDelete(chat, mid, data[9:])
	case data == "help":
		r.helpList(chat, mid)
	case data == "wahelp":
		r.webappHelp(chat, mid)
	case strings.HasPrefix(data, "help:"):
		r.helpShow(chat, mid, data[5:])

	// admins
	case data == "admins_menu":
		r.adminsMenu(chat, mid)
	case data == "adminadd":
		r.adminAdd(chat, mid)
	case data == "admindel":
		r.adminDelList(chat, mid)
	case strings.HasPrefix(data, "admindely:"):
		r.adminDelDo(chat, mid, data[10:])

	// hub
	case data == "hub":
		r.hubHome(chat, mid)
	case data == "hubstats":
		r.hubStats(chat, mid)
	case data == "hubann":
		r.hubAnn(chat, mid)
	case strings.HasPrefix(data, "hubannv:"):
		r.hubAnnView(chat, mid, data[8:])
	case strings.HasPrefix(data, "hubannreply:"):
		r.hubAnnReplyStart(chat, mid, data[13:])
	case data == "hubcast":
		r.hubCastStart(chat, mid)
	case data == "hubcfg":
		r.hubCfg(chat, mid)
	case data == "hubsethub":
		r.hubCfgSetURL(chat, mid)
	case data == "hubsetsc":
		r.hubCfgSetSecret(chat, mid)
	case data == "annurl":
		r.hubCfgSetAnn(chat, mid)
	case data == "hubcontact":
		r.hubContact(chat, mid)
	case data == "promo":
		r.promoHome(chat, mid)
	case data == "promoset":
		r.promoSetAsk(chat, mid)
	case data == "promoreset":
		r.promoReset(chat, mid)
	case data == "hubctmsg":
		r.hubContactStart(chat, mid)
	case strings.HasPrefix(data, "hubreply:"):
		r.hubReplyStart(chat, mid, data[9:])

	// reminders
	case data == "rem":
		r.remHome(chat, mid)
	case data == "remnew":
		r.remNew(chat, mid)
	case data == "remset":
		r.remSet(chat, mid)
	case strings.HasPrefix(data, "remseth:"):
		r.remSetH(chat, mid, data[8:])
	case data == "remsrv", data == "remsrvf":
		r.remSrv(chat, mid, data == "remsrvf")
	case strings.HasPrefix(data, "rempage:"):
		r.remPage(chat, mid, data[8:])
	case strings.HasPrefix(data, "rempick:"):
		r.remPick(chat, mid, data[8:])
	case data == "remtxt":
		r.remText(chat, mid)
	case data == "remexp":
		r.remExp(chat, mid)
	case data == "remnote":
		r.remNote(chat, mid)
	case data == "remwhenback":
		r.remWhenBack(chat, mid)
	case data == "remrel":
		r.remRel(chat, mid)
	case strings.HasPrefix(data, "remrelu:"):
		r.remRelU(chat, mid, data[8:])
	case data == "remabs":
		r.remAbs(chat, mid)
	case strings.HasPrefix(data, "remabsy:"):
		r.remAbsY(chat, mid, data[8:])
	case strings.HasPrefix(data, "remabsm:"):
		r.remAbsM(chat, mid, data[8:])
	case data == "remabsman":
		r.remAbsMan(chat, mid)
	case data == "remdel":
		r.remDelList(chat, mid)
	case strings.HasPrefix(data, "remdelx:"):
		r.remDelAsk(chat, mid, data[8:])
	case strings.HasPrefix(data, "remdely:"):
		r.remDelDo(chat, mid, data[8:])
	case strings.HasPrefix(data, "remack:"):
		r.remAck(chat, mid, data[7:])
	case strings.HasPrefix(data, "remagainx:"):
		r.remAgainX(chat, mid, data[10:])
	case strings.HasPrefix(data, "remagain:"):
		r.remAgain(chat, mid, data[9:])

	// domain & ssl monitors
	case data == "dommon":
		r.dommonHome(chat, mid)
	case data == "quota":
		r.quotaHome(chat, mid)
	case data == "qsetlimit":
		r.quotaSetLimit(chat, mid)
	case data == "qtgauto":
		cfg := r.quotaCfg()
		cfg.AutoStop = !cfg.AutoStop
		r.quotaSave(cfg)
		r.quotaHome(chat, mid)
	case data == "qalerts":
		cfg := r.quotaCfg()
		cfg.Alerts = !cfg.Alerts
		r.quotaSave(cfg)
		r.quotaHome(chat, mid)
	case data == "qcrons":
		r.quotaCrons(chat, mid)
	case strings.HasPrefix(data, "qcron:"):
		h, _ := strconv.Atoi(data[6:])
		cfg := r.quotaCfg()
		if h > 0 {
			cfg.CronPausedUntil = time.Now().UnixMilli() + int64(h)*3600*1000
		} else {
			cfg.CronPausedUntil = 0
		}
		r.quotaSave(cfg)
		r.quotaCrons(chat, mid)
	case data == "sslm":
		r.sslmHome(chat, mid)
	case data == "ssdma":
		r.sslAdd(chat, mid)
	case data == "sslmd":
		r.sslDelList(chat, mid)
	case strings.HasPrefix(data, "sslmdy:"):
		r.sslDelDo(chat, mid, data[7:])
	case data == "domexp":
		r.domexpRefresh(chat, mid)
	case data == "domexpp":
		r.domexpHome(chat, mid, 0)
	case strings.HasPrefix(data, "domexpp:"):
		pg, _ := strconv.Atoi(data[8:])
		r.domexpHome(chat, mid, pg)
	case data == "domexpall":
		r.domexpAddAll(chat, mid)
	case data == "domexpa":
		r.domexpAddStart(chat, mid)
	case data == "domexpcal":
		r.domexpCalList(chat, mid)
	case strings.HasPrefix(data, "domexpi:"):
		r.domexpCalDomain(chat, mid, data[8:])
	case data == "domexpd":
		r.domexpDelList(chat, mid)
	case strings.HasPrefix(data, "domexpdy:"):
		r.domexpDel(chat, mid, data[9:])
	case data == "domexpkey":
		r.domexpKeyPage(chat, mid)
	case data == "domexpk":
		r.domexpKeyStart(chat, mid)
	case data == "domexpkdel":
		r.domexpKeyDel(chat, mid)
	case data == "domexptest":
		r.domexpTest(chat, mid)

	// PasarGuard panels
	case data == "pg", data == "pndef":
		r.pgHome(chat, mid)
	case data == "pga":
		r.pgAdd(chat, mid)
	case data == "pgdel":
		r.pgDelList(chat, mid)
	case strings.HasPrefix(data, "pgd:"):
		r.pgDelAsk(chat, mid, data[4:])
	case strings.HasPrefix(data, "pgdy:"):
		r.pgDelDo(chat, mid, data[5:])
	case strings.HasPrefix(data, "pgl:"):
		p := strings.SplitN(data[4:], ":", 2)
		i, _ := strconv.Atoi(p[0])
		pnum := 0
		if len(p) > 1 {
			pnum, _ = strconv.Atoi(p[1])
		}
		r.pgUsers(chat, mid, i, pnum, "")
	case strings.HasPrefix(data, "pgu:"):
		idx, _ := strconv.Atoi(data[4:])
		r.pgUser(chat, mid, idx)
	case data == "pgq":
		r.pgSearchStart(chat, mid)
	case data == "pnle":
		r.pnleList(chat, mid)
	case strings.HasPrefix(data, "pnleN:"):
		r.pnleField(chat, mid, "N", intOrAny(data[6:]))
	case strings.HasPrefix(data, "pnleU:"):
		r.pnleField(chat, mid, "U", intOrAny(data[6:]))
	case strings.HasPrefix(data, "pnleP:"):
		r.pnleField(chat, mid, "P", intOrAny(data[6:]))
	case strings.HasPrefix(data, "pnleW:"):
		r.pnleField(chat, mid, "W", intOrAny(data[6:]))
	case strings.HasPrefix(data, "pnle:"):
		r.pnlePanel(chat, mid, intOrAny(data[5:]))
	case strings.HasPrefix(data, "pnlops:"):
		r.pnlops(chat, mid, intOrAny(data[7:]))

	// PasarGuard alerts (pghook)
	case data == "pghook":
		r.pghookHome(chat, mid)
	case data == "pgtoggle":
		r.pgToggle(chat, mid)
	case data == "pgsilent":
		r.pgSilent(chat, mid)
	case strings.HasPrefix(data, "pgpanel:"):
		i, _ := strconv.Atoi(data[8:])
		r.pgPanel(chat, mid, i)
	case strings.HasPrefix(data, "pgtest:"):
		i, _ := strconv.Atoi(data[7:])
		r.pghookTest(chat, mid, i)
	case strings.HasPrefix(data, "pgregen:"):
		i, _ := strconv.Atoi(data[8:])
		r.pgRegen(chat, mid, i)
	case strings.HasPrefix(data, "pgdash:"):
		i, _ := strconv.Atoi(data[7:])
		r.pgDashAsk(chat, mid, i)
	case strings.HasPrefix(data, "pgrenewbot:"):
		i, _ := strconv.Atoi(data[11:])
		r.pgRenewbotAsk(chat, mid, i)
	case strings.HasPrefix(data, "pgrecur:"):
		i, _ := strconv.Atoi(data[8:])
		r.pgRecurAsk(chat, mid, i)

	// usage monitor
	case data == "um":
		r.umHome(chat, mid)
	case data == "um:toggle":
		r.umToggle(chat, mid)
	case data == "um:report":
		r.umReport(chat, mid)
	case data == "um:h":
		r.umField(chat, mid, "hourlyGb", "آستانه ساعتی (GB)")
	case data == "um:d":
		r.umField(chat, mid, "dailyGb", "آستانه روزانه (GB)")
	case data == "um:c":
		r.umField(chat, mid, "cooldownH", "فاصله تکرار هشدار (ساعت)")
	case data == "um:min":
		r.umField(chat, mid, "minGb", "حداقل مصرف اسنپ‌شات (GB)")

	// node monitor
	case data == "nd":
		r.ndHome(chat, mid)
	case data == "ndhelp":
		r.ndHelp(chat, mid)
	case data == "ndmk":
		r.ndMkPanel(chat, mid)
	case data == "nddelmenu":
		r.ndDelMenu(chat, mid)
	case strings.HasPrefix(data, "ndd:"):
		r.ndDetail(chat, mid, data[4:])
	case strings.HasPrefix(data, "ndsync:"):
		r.ndSync(chat, mid, data[7:])
	case strings.HasPrefix(data, "ndtest:"):
		r.ndTest(chat, mid, data[7:])
	case strings.HasPrefix(data, "ndhook:"):
		r.ndHook(chat, mid, data[7:])
	case strings.HasPrefix(data, "ndexc:"):
		r.ndExc(chat, mid, data[6:])
	case strings.HasPrefix(data, "ndx:"):
		if p := strings.SplitN(data[4:], ":", 2); len(p) == 2 {
			r.ndToggleExc(chat, mid, p[0], p[1])
		}
	case strings.HasPrefix(data, "ndip:"):
		if p := strings.SplitN(data[5:], ":", 2); len(p) == 2 {
			r.ndNodeIP(chat, mid, p[0], p[1])
		}
	case strings.HasPrefix(data, "ndi:"):
		if p := strings.SplitN(data[4:], ":", 2); len(p) == 2 {
			r.ndNode(chat, mid, p[0], p[1])
		}
	case strings.HasPrefix(data, "ndint:"):
		r.ndIntAsk(chat, mid, data[6:])
	case strings.HasPrefix(data, "ndmk:"):
		i, _ := strconv.Atoi(data[5:])
		r.ndMkInterval(chat, mid, i)
	case strings.HasPrefix(data, "ndset:"):
		r.ndIntervalDo(chat, mid, data[6:])
	case strings.HasPrefix(data, "nddel:"):
		r.ndDelAsk(chat, mid, data[6:])
	case strings.HasPrefix(data, "nddy:"):
		r.ndDelDo(chat, mid, data[5:])
	case strings.HasPrefix(data, "ndtg:"):
		r.ndToggle(chat, mid, data[5:])

	// host filter
	case data == "hf":
		r.hfHome(chat, mid)
	case data == "hftg":
		cfg := hostfilter.GetCfg(r.Store)
		v := !cfg.IsEnabled()
		cfg.Enabled = &v
		hostfilter.SaveCfg(r.Store, cfg)
		r.hfHome(chat, mid)
	case data == "hfforceall":
		r.Show(chat, mid, "🔄 بررسی کامل شروع شد؛ نتیجه همین‌جا اعلام می‌شود.", HomeKB())
		r.hfRunBg(chat, true, "")
	case data == "hflist":
		r.hfHosts(chat, mid)
	case strings.HasPrefix(data, "hfexc:"):
		if p := strings.Split(data, ":"); len(p) == 4 {
			r.hfExcToggle(chat, mid, p[1], p[2], p[3])
		}
	case data == "hfexcall":
		r.hfExcAll(chat, mid)
	case data == "hfunexcall":
		r.hfUnExcAll(chat, mid)
	case strings.HasPrefix(data, "hfexcpanel:"):
		i, _ := strconv.Atoi(data[11:])
		r.hfExcPanel(chat, mid, i)
	case strings.HasPrefix(data, "hfunexcpanel:"):
		i, _ := strconv.Atoi(data[13:])
		r.hfUnExcPanel(chat, mid, i)
	case data == "hfgmode":
		r.hfGMode(chat, mid)
	case data == "hfgdone":
		r.hfGDone(chat, mid)
	case data == "hfgall":
		r.hfGAll(chat, mid)
	case data == "hfgclr":
		r.hfGClr(chat, mid)
	case strings.HasPrefix(data, "hfgs:"):
		if p := strings.Split(data, ":"); len(p) == 3 {
			r.hfGSelect(chat, mid, p[1], p[2])
		}
	case data == "hfgexc":
		r.hfGExc(chat, mid)
	case data == "hfgunexc":
		r.hfGUnExc(chat, mid)
	case data == "hfhist":
		r.hfHist(chat, mid)
	case data == "hfsetch":
		r.hfSettings(chat, mid)
	case strings.HasPrefix(data, "hfnum:"):
		r.hfNumAsk(chat, mid, data[6:])
	case data == "hfrevlist":
		r.hfRevertList(chat, mid)
	case strings.HasPrefix(data, "hfrev:"):
		r.hfRevert(chat, mid, data[6:])
	// daily sub rotation (hfd)
	case data == "hfd":
		r.hfdHome(chat, mid)
	case data == "hfdtg":
		r.hfdToggle(chat, mid)
	case data == "hfdint":
		r.hfdIntervalMenu(chat, mid)
	case strings.HasPrefix(data, "hfdintv:"):
		r.hfdIntervalSet(chat, mid, intOrAny(data[8:]))
	case data == "hfdintc":
		r.hfdIntAsk(chat, mid)
	case data == "hfdat":
		r.hfdAtAsk(chat, mid)
	case data == "hfdhosts":
		r.hfdHostsHome(chat, mid)
	case strings.HasPrefix(data, "hfdpanel:"):
		arg := data[9:]
		if arg == "all" {
			r.hfdHome(chat, mid)
		} else {
			r.hfdPanel(chat, mid, intOrAny(arg))
		}
	case strings.HasPrefix(data, "hfdhf:"):
		if p := strings.Split(data, ":"); len(p) == 4 {
			r.hfdHostField(chat, mid, intOrAny(p[1]), p[2], p[3])
		}
	case data == "hfdrun":
		r.hfdRunNow(chat, mid)
	case data == "hfdlog":
		r.hfdLogView(chat, mid)
	case data == "hfdrvlist":
		r.hfdRevertList(chat, mid)
	case strings.HasPrefix(data, "hfdrv:"):
		if p := strings.Split(data, ":"); len(p) == 3 {
			r.hfdRevert(chat, mid, intOrAny(p[1]), p[2])
		}

	// domain email (fmail)
	case data == "fmail":
		r.fmailHome(chat, mid)
	case data == "fmailadd":
		r.fmailAdd(chat, mid)
	case strings.HasPrefix(data, "fmode:"):
		if p := strings.SplitN(data[6:], ":", 2); len(p) == 2 {
			r.fmailMode(chat, mid, p[0], p[1])
		}
	case strings.HasPrefix(data, "fmailfwdnew:"):
		if p := strings.SplitN(data[12:], ":", 2); len(p) == 2 {
			r.fmailFwdNewAsk(chat, mid, p[0], p[1])
		}
	case strings.HasPrefix(data, "fmailfwd:"):
		if p := strings.SplitN(data[9:], ":", 2); len(p) == 2 {
			r.fmailFwdPick(chat, mid, p[0], p[1])
		}
	case strings.HasPrefix(data, "fmailfwdc:"):
		if p := strings.SplitN(data[10:], ":", 3); len(p) == 3 {
			r.fmailFwdDo(chat, mid, p[0], p[1], p[2])
		}
	case strings.HasPrefix(data, "fmailoff:"):
		if p := strings.SplitN(data[9:], ":", 2); len(p) == 2 {
			r.fmailOff(chat, mid, p[0], p[1])
		}
	case data == "fmailsync":
		r.Store.Delete(fmt.Sprintf("fmail_scan:%d", chat))
		r.fmailHome(chat, mid)

	// per-zone email routing (zmail)
	case data == "zmail":
		r.zmailHome(chat, mid)
	case data == "zmailon":
		r.zmailEnableAsk(chat, mid)
	case data == "zmailony":
		r.zmailEnableDo(chat, mid)
	case data == "zmailadd":
		r.zmailAddStart(chat, mid)
	case strings.HasPrefix(data, "zmaildest:"):
		r.zmailDestPick(chat, mid, data[9:], false)
	case strings.HasPrefix(data, "zmailcset:"):
		r.zmailDestPick(chat, mid, data[9:], true)
	case data == "zmailcpick":
		r.zmailDestListPick(chat, mid, true)
	case data == "zmaildestnew":
		r.zmailDestNewAsk(chat, mid)
	case data == "zmaildests":
		r.zmailDests(chat, mid)
	case strings.HasPrefix(data, "zmaildel:"):
		r.zmailDelAsk(chat, mid, data[8:])
	case strings.HasPrefix(data, "zmaildely:"):
		r.zmailDelDo(chat, mid, data[10:])
	case data == "zmailcatch":
		r.zmailCatch(chat, mid)
	case data == "zmailcoff":
		r.zmailCatchOff(chat, mid)

	// traffic
	case data == "traf":
		r.trafZones(chat, mid, "all", 0)
	case strings.HasPrefix(data, "trazf:"):
		if p := strings.Split(data, ":"); len(p) >= 2 {
			pg := 0
			if len(p) > 2 && isDigits(strings.TrimLeft(p[2], "-")) {
				pg = atoiSafe(p[2])
			}
			r.trafZones(chat, mid, p[1], pg)
		}
	case strings.HasPrefix(data, "traz:"):
		if p := strings.SplitN(data[5:], ":", 3); len(p) >= 2 {
			pg := 0
			if len(p) > 2 {
				pg = atoiSafe(p[2])
			}
			r.trafZone(chat, mid, atoiSafe(p[0]), p[1], pg)
		}
	case strings.HasPrefix(data, "trazdel:"):
		if p := strings.SplitN(data[8:], ":", 2); len(p) == 2 {
			r.trafZeroConfirm(chat, mid, atoiSafe(p[0]), p[1])
		}
	case strings.HasPrefix(data, "trazdely:"):
		if p := strings.SplitN(data[9:], ":", 2); len(p) == 2 {
			r.trafZeroDo(chat, mid, atoiSafe(p[0]), p[1])
		}

	// load balancer
	case data == "lb":
		r.lbZones(chat, mid, "all", 0)
	case strings.HasPrefix(data, "lbzf:"):
		if p := strings.Split(data, ":"); len(p) >= 2 {
			pg := 0
			if len(p) > 2 && isDigits(strings.TrimLeft(p[2], "-")) {
				pg = atoiSafe(p[2])
			}
			r.lbZones(chat, mid, p[1], pg)
		}
	case strings.HasPrefix(data, "lbacc:"):
		r.lbZones(chat, mid, strings.TrimPrefix(data, "lbacc:"), 0)
	case strings.HasPrefix(data, "lbzp:"):
		if p := strings.SplitN(strings.TrimPrefix(data, "lbzp:"), ":", 3); len(p) >= 2 {
			pg := 0
			if len(p) > 2 {
				pg = atoiSafe(p[2])
			}
			r.lbZoneGroups(chat, mid, atoiSafe(p[0]), p[1], pg)
		}
	case strings.HasPrefix(data, "lbz:"):
		if p := strings.SplitN(strings.TrimPrefix(data, "lbz:"), ":", 2); len(p) >= 2 {
			r.lbZoneGroups(chat, mid, atoiSafe(p[0]), p[1], 0)
		}
	case strings.HasPrefix(data, "lbgopen:"):
		if p := strings.SplitN(strings.TrimPrefix(data, "lbgopen:"), ":", 3); len(p) == 3 {
			r.lbOpenGroup(chat, mid, atoiSafe(p[0]), p[1], p[2])
		}
	case strings.HasPrefix(data, "lbs:"):
		r.lbSettings(chat, mid, strings.TrimPrefix(data, "lbs:"))
	case strings.HasPrefix(data, "lbsr:"):
		r.lbRender(chat, mid, strings.TrimPrefix(data, "lbsr:"))
	case strings.HasPrefix(data, "lbss:"):
		r.lbSpareHome(chat, mid, strings.TrimPrefix(data, "lbss:"))
	case strings.HasPrefix(data, "lbsh:"):
		r.lbSpareHomeGid(chat, mid, strings.TrimPrefix(data, "lbsh:"))
	case strings.HasPrefix(data, "lbsm:"):
		gid, idx := splitTail(strings.TrimPrefix(data, "lbsm:"))
		r.lbSpareMenu(chat, mid, gid, atoiSafe(idx))
	case strings.HasPrefix(data, "lbw:"):
		gid, idx := splitTail(strings.TrimPrefix(data, "lbw:"))
		r.lbWeightAsk(chat, mid, gid, atoiSafe(idx))
	case strings.HasPrefix(data, "lbt:"):
		gid, idx := splitTail(strings.TrimPrefix(data, "lbt:"))
		r.lbToggleAsk(chat, mid, gid, atoiSafe(idx))
	case strings.HasPrefix(data, "lbtc:"):
		gid, idx := splitTail(strings.TrimPrefix(data, "lbtc:"))
		r.lbToggleDo(chat, mid, gid, atoiSafe(idx))
	case strings.HasPrefix(data, "lbadd:"):
		r.lbAddStart(chat, mid, strings.TrimPrefix(data, "lbadd:"))
	case strings.HasPrefix(data, "lbp:"):
		gid, onoff := splitTail(strings.TrimPrefix(data, "lbp:"))
		r.lbProxyAll(chat, mid, gid, onoff == "on")
	case strings.HasPrefix(data, "lbsset:"):
		gid, idx := splitTail(strings.TrimPrefix(data, "lbsset:"))
		r.setWizard(chat, map[string]any{"kind": "lb_spare", "gid": gid, "idx": atoiSafe(idx)})
		r.Show(chat, mid, "🛟 مقدار زاپاس را بفرستید (آی‌پی یا هاست‌نیم):",
			[][]Btn{{btn("❌ انصراف", "lbsr:"+gid)}})
	case strings.HasPrefix(data, "lbsgo:"):
		gid, idx := splitTail(strings.TrimPrefix(data, "lbsgo:"))
		r.lbReplace(chat, mid, gid, atoiSafe(idx), false)
	case strings.HasPrefix(data, "lbsgoc:"):
		gid, idx := splitTail(strings.TrimPrefix(data, "lbsgoc:"))
		r.lbReplace(chat, mid, gid, atoiSafe(idx), true)
	case strings.HasPrefix(data, "lbsdel:"):
		gid, idx := splitTail(strings.TrimPrefix(data, "lbsdel:"))
		r.lbSpareDel(chat, mid, gid, atoiSafe(idx))

	// arvan
	case data == "arvan":
		r.arvanHome(chat, mid)
	case data == "arvanacc":
		r.arvanAccsPage(chat, mid)
	case data == "arvanaccadd", data == "arvanacchelp":
		r.arvanAccHelp(chat, mid)
	case strings.HasPrefix(data, "arvanaccdel:"):
		r.arvanAccDelAsk(chat, mid, strings.TrimPrefix(data, "arvanaccdel:"))
	case strings.HasPrefix(data, "arvanaccdelok:"):
		r.arvanAccDelDo(chat, mid, strings.TrimPrefix(data, "arvanaccdelok:"))
	case data == "arvdoms":
		r.arvanDoms(chat, mid)
	case strings.HasPrefix(data, "arvdom:"):
		if p := strings.SplitN(strings.TrimPrefix(data, "arvdom:"), ":", 2); len(p) == 2 {
			r.arvanRecs(chat, mid, atoiSafe(p[0]), p[1], 0)
		}
	case strings.HasPrefix(data, "arvrec:"):
		if p := strings.SplitN(strings.TrimPrefix(data, "arvrec:"), ":", 3); len(p) == 3 {
			r.arvanRec(chat, mid, atoiSafe(p[0]), p[1], p[2])
		}
	case strings.HasPrefix(data, "arvev:"):
		if p := strings.SplitN(strings.TrimPrefix(data, "arvev:"), ":", 3); len(p) == 3 {
			r.setWizard(chat, map[string]any{"kind": "arvan_value", "acc": atoiSafe(p[0]), "dom": p[1], "rid": p[2]})
			r.Show(chat, mid, "✏️ مقدار جدید را بفرستید:", [][]Btn{{btn("❌ انصراف", "arvan")}})
		}
	case strings.HasPrefix(data, "arvdel:"):
		if p := strings.SplitN(strings.TrimPrefix(data, "arvdel:"), ":", 3); len(p) == 3 {
			r.Show(chat, mid, "⚠️ رکورد حذف شود?",
				[][]Btn{{btn("✅ بله", fmt.Sprintf("arvdelok:%s:%s:%s", p[0], p[1], p[2])),
					btn("❌ انصراف", "arvan")}})
		}
	case strings.HasPrefix(data, "arvdelok:"):
		if p := strings.SplitN(strings.TrimPrefix(data, "arvdelok:"), ":", 3); len(p) == 3 {
			r.arvanDelDo(chat, mid, atoiSafe(p[0]), p[1], p[2])
		}
	case strings.HasPrefix(data, "arvaadd:"):
		if p := strings.SplitN(strings.TrimPrefix(data, "arvaadd:"), ":", 2); len(p) == 2 {
			r.setWizard(chat, map[string]any{"kind": "arvan_add", "acc": atoiSafe(p[0]), "dom": p[1]})
			r.Show(chat, mid,
				"➕ رکورد جدید — نوع و نام و مقدار را با فاصله بفرست:\n<code>A sub 1.2.3.4</code>",
				[][]Btn{{btn("❌ انصراف", "arvan")}})
		}
	case strings.HasPrefix(data, "arvcloud:"):
		if p := strings.SplitN(strings.TrimPrefix(data, "arvcloud:"), ":", 3); len(p) == 3 {
			r.arvanCloud(chat, mid, atoiSafe(p[0]), p[1], p[2])
		}

	// arvan cloud servers (ECC)
	case data == "arvsrv":
		r.arvCloudHome(chat, mid)
	case strings.HasPrefix(data, "arvregcustom:"):
		r.arvRegionStart(chat, mid, strings.TrimPrefix(data, "arvregcustom:"))
	case strings.HasPrefix(data, "arvreg:"):
		if p := strings.SplitN(strings.TrimPrefix(data, "arvreg:"), ":", 2); len(p) == 2 {
			r.arvServerList(chat, mid, atoiSafe(p[0]), p[1])
		}
	case strings.HasPrefix(data, "arvs:"):
		if p := strings.SplitN(strings.TrimPrefix(data, "arvs:"), ":", 2); len(p) == 2 {
			r.arvServerDetail(chat, mid, atoiSafe(p[0]), atoiSafe(p[1]))
		}
	case strings.HasPrefix(data, "arvon:"):
		if p := strings.SplitN(strings.TrimPrefix(data, "arvon:"), ":", 2); len(p) == 2 {
			r.arvAction(chat, mid, atoiSafe(p[0]), atoiSafe(p[1]), "power-on", nil, "✅ دستور روشن‌شدن ارسال شد.")
		}
	case strings.HasPrefix(data, "arvoff:"):
		if p := strings.SplitN(strings.TrimPrefix(data, "arvoff:"), ":", 2); len(p) == 2 {
			r.arvAction(chat, mid, atoiSafe(p[0]), atoiSafe(p[1]), "power-off", nil, "✅ دستور خاموش‌شدن ارسال شد.")
		}
	case strings.HasPrefix(data, "arvreboot:"):
		if p := strings.SplitN(strings.TrimPrefix(data, "arvreboot:"), ":", 2); len(p) == 2 {
			r.arvAction(chat, mid, atoiSafe(p[0]), atoiSafe(p[1]), "reboot", nil, "✅ دستور ریبوت ارسال شد.")
		}
	case strings.HasPrefix(data, "arvsnap:"):
		if p := strings.SplitN(strings.TrimPrefix(data, "arvsnap:"), ":", 2); len(p) == 2 {
			r.arvSnapStart(chat, mid, p[0], p[1])
		}
	case strings.HasPrefix(data, "arvsnapls:"):
		if p := strings.SplitN(strings.TrimPrefix(data, "arvsnapls:"), ":", 2); len(p) == 2 {
			r.arvSnapList(chat, mid, atoiSafe(p[0]), atoiSafe(p[1]))
		}
	case strings.HasPrefix(data, "arvsnapdel:"):
		if p := strings.SplitN(strings.TrimPrefix(data, "arvsnapdel:"), ":", 3); len(p) == 3 {
			r.arvSnapDel(chat, mid, atoiSafe(p[0]), atoiSafe(p[1]), p[2])
		}
	case strings.HasPrefix(data, "arvren:"):
		if p := strings.SplitN(strings.TrimPrefix(data, "arvren:"), ":", 2); len(p) == 2 {
			r.arvRenameStart(chat, mid, p[0], p[1])
		}
	case strings.HasPrefix(data, "arvyroot:"):
		if p := strings.SplitN(strings.TrimPrefix(data, "arvyroot:"), ":", 2); len(p) == 2 {
			r.arvRootReset(chat, mid, atoiSafe(p[0]), atoiSafe(p[1]))
		}
	case data == "hfbk":
		r.hfBackupHome(chat, mid)
	case data == "hfbknow":
		r.hfBackupNow(chat, mid)
	case strings.HasPrefix(data, "hfbkr:"):
		r.hfBackupRestoreAsk(chat, mid, strings.TrimPrefix(data, "hfbkr:"))
	case strings.HasPrefix(data, "hfbkrd:"):
		r.hfBackupRestoreDo(chat, mid, strings.TrimPrefix(data, "hfbkrd:"))
	case strings.HasPrefix(data, "hfbkdel:"):
		r.hfBackupDel(chat, mid, strings.TrimPrefix(data, "hfbkdel:"))
	case strings.HasPrefix(data, "arvterm:"):
		if p := strings.SplitN(strings.TrimPrefix(data, "arvterm:"), ":", 2); len(p) == 2 {
			r.Show(chat, mid, "⚠️ مطمئنی سرور حذف شود؟ این عمل بازگشت‌پذیر نیست.",
				[][]Btn{{btnS("🗑 بله، حذف کن", fmt.Sprintf("arvtermok:%s:%s", p[0], p[1]), "danger"),
					btn("❌ انصراف", fmt.Sprintf("arvs:%s:%s", p[0], p[1]))}})
		}
	case strings.HasPrefix(data, "arvtermok:"):
		if p := strings.SplitN(strings.TrimPrefix(data, "arvtermok:"), ":", 2); len(p) == 2 {
			r.arvDelServer(chat, mid, atoiSafe(p[0]), atoiSafe(p[1]))
		}
	case data == "arvregrefresh":
		r.arvCloudRefresh(chat, mid)
	case strings.HasPrefix(data, "arvnew:"):
		if p := strings.SplitN(strings.TrimPrefix(data, "arvnew:"), ":", 2); len(p) == 2 {
			r.arvMkStart(chat, mid, atoiSafe(p[0]), p[1])
		}
	case strings.HasPrefix(data, "arvimg:"):
		r.arvMkPickImage(chat, mid, atoiSafe(strings.TrimPrefix(data, "arvimg:")))
	case data == "arvimgman":
		r.arvMkImageManual(chat, mid)
	case strings.HasPrefix(data, "arvflv:"):
		r.arvMkPickFlavor(chat, mid, atoiSafe(strings.TrimPrefix(data, "arvflv:")))
	case strings.HasPrefix(data, "arvnet:"):
		r.arvMkPickNetwork(chat, mid, atoiSafe(strings.TrimPrefix(data, "arvnet:")))
	case data == "arvnetdef":
		r.arvMkSkipNetwork(chat, mid)
	case strings.HasPrefix(data, "arvsshk:"):
		r.arvMkPickSsh(chat, mid, atoiSafe(strings.TrimPrefix(data, "arvsshk:")))
	case data == "arvsshskip":
		r.arvMkConfirm(chat, mid)
	case data == "arvsshman":
		r.arvMkSshManual(chat, mid)
	case strings.HasPrefix(data, "arvhuntgo:"):
		if p := strings.Split(strings.TrimPrefix(data, "arvhuntgo:"), ":"); len(p) >= 2 {
			idx, rounds, batch := -1, 10, 3
			if len(p) >= 3 {
				idx = atoiSafe(p[2])
			}
			if len(p) >= 4 {
				rounds = atoiSafe(p[3])
			}
			if len(p) >= 5 {
				batch = atoiSafe(p[4])
			}
			r.arvHuntStart(chat, mid, atoiSafe(p[0]), p[1], idx, rounds, batch)
		}
	case strings.HasPrefix(data, "arvhuntstop"):
		r.arvHuntStop(chat)
	case strings.HasPrefix(data, "arvhuntp:"):
		if p := strings.Split(strings.TrimPrefix(data, "arvhuntp:"), ":"); len(p) == 3 {
			r.arvHuntPlanConfirm(chat, mid, atoiSafe(p[0]), p[1], atoiSafe(p[2]))
		}
	case strings.HasPrefix(data, "arvhuntc:"):
		if p := strings.Split(strings.TrimPrefix(data, "arvhuntc:"), ":"); len(p) == 2 {
			r.arvHuntPlans(chat, mid, atoiSafe(p[0]), p[1])
		}
	case strings.HasPrefix(data, "arvhunt:"):
		if p := strings.Split(strings.TrimPrefix(data, "arvhunt:"), ":"); len(p) == 1 {
			r.arvHuntRegions(chat, mid, atoiSafe(p[0]))
		} else if len(p) >= 2 {
			r.arvHuntPlans(chat, mid, atoiSafe(p[0]), p[1])
		}
	case data == "arvmkauto":
		r.arvMkNameSave(chat, mid, arvAutoName())
	case data == "arvflvretry":
		r.arvMkFlavors(chat, mid)
	case data == "arvfwman":
		r.arvMkFwStart(chat, mid)
	case data == "arvcreate":
		r.arvMkCreate(chat, mid)
	case strings.HasPrefix(data, "arvpick:"):
		if p := strings.SplitN(strings.TrimPrefix(data, "arvpick:"), ":", 2); len(p) == 2 {
			r.arvPick(chat, mid, atoiSafe(p[0]), atoiSafe(p[1]))
		}
	case strings.HasPrefix(data, "arvbuy:"):
		rest := strings.TrimPrefix(data, "arvbuy:")
		if p := strings.SplitN(rest, ":", 2); len(p) == 1 {
			r.arvBuyRegions(chat, mid, atoiSafe(p[0]))
		} else if len(p) == 2 {
			r.arvMkStart(chat, mid, atoiSafe(p[0]), p[1])
		}
	case strings.HasPrefix(data, "arvregs:"):
		r.arvBrowseRegions(chat, mid, atoiSafe(strings.TrimPrefix(data, "arvregs:")))
	case strings.HasPrefix(data, "arvipc:"):
		if p := strings.SplitN(strings.TrimPrefix(data, "arvipc:"), ":", 2); len(p) == 2 {
			r.arvIPCheck(chat, atoiSafe(p[0]), atoiSafe(p[1]))
		}
	case strings.HasPrefix(data, "arvup:"):
		if p := strings.SplitN(strings.TrimPrefix(data, "arvup:"), ":", 2); len(p) == 2 {
			r.arvUpgrade(chat, mid, atoiSafe(p[0]), atoiSafe(p[1]))
		}
	case strings.HasPrefix(data, "arvupd:"):
		if p := strings.SplitN(strings.TrimPrefix(data, "arvupd:"), ":", 3); len(p) == 3 {
			r.arvUpgradeAsk(chat, mid, atoiSafe(p[0]), atoiSafe(p[1]), atoiSafe(p[2]))
		}
	case strings.HasPrefix(data, "arvupdo:"):
		if p := strings.SplitN(strings.TrimPrefix(data, "arvupdo:"), ":", 3); len(p) == 3 {
			r.arvUpgradeDo(chat, mid, atoiSafe(p[0]), atoiSafe(p[1]), atoiSafe(p[2]))
		}
	case strings.HasPrefix(data, "arvupok:"):
		if p := strings.SplitN(strings.TrimPrefix(data, "arvupok:"), ":", 2); len(p) == 2 {
			r.arvUpgradeConfirm(chat, mid, atoiSafe(p[0]), atoiSafe(p[1]))
		}

	// hetzner
	case data == "hz":
		r.hzHome(chat, mid)
	case data == "hza":
		r.setWizard(chat, map[string]any{"kind": "hz_add_name"})
		r.Show(chat, mid, "👤 نام دلخواه اکانت هتزنر را بفرستید:",
			navKB("hz"))
	case data == "hzaccs":
		r.hzAccsMenu(chat, mid)
	case data == "hzdel":
		r.hzDelList(chat, mid)
	case strings.HasPrefix(data, "hzm:"):
		i := atoiSafe(strings.TrimPrefix(data, "hzm:"))
		if i >= 0 && i < len(r.hzAccs()) {
			r.Store.Put("hz_sel", strconv.Itoa(i), 0)
		}
		r.hzHome(chat, mid)
	case strings.HasPrefix(data, "hzd:"):
		r.hzDelAsk(chat, mid, strings.TrimPrefix(data, "hzd:"))
	case strings.HasPrefix(data, "hzdy:"):
		r.hzDelDo(chat, mid, strings.TrimPrefix(data, "hzdy:"))
	case strings.HasPrefix(data, "hzs:"):
		if p := strings.SplitN(strings.TrimPrefix(data, "hzs:"), ":", 2); len(p) == 2 {
			r.hzServers(chat, mid, atoiSafe(p[0]), atoiSafe(p[1]))
		}
	case strings.HasPrefix(data, "hzsi:"):
		if p := strings.SplitN(strings.TrimPrefix(data, "hzsi:"), ":", 2); len(p) == 2 {
			r.hzInfo(chat, mid, atoiSafe(p[0]), p[1])
		}
	case strings.HasPrefix(data, "hzip:"):
		r.hzIPPool(chat, mid, atoiSafe(strings.TrimPrefix(data, "hzip:")))
	case strings.HasPrefix(data, "hzipadd:"):
		r.hzIPPoolFill(chat, mid, atoiSafe(strings.TrimPrefix(data, "hzipadd:")))
	case strings.HasPrefix(data, "hzipl:"):
		if p := strings.SplitN(strings.TrimPrefix(data, "hzipl:"), ":", 2); len(p) == 2 {
			r.hzIPPoolFillLoc(chat, mid, atoiSafe(p[0]), p[1])
		}
	case strings.HasPrefix(data, "hzipan:"):
		if p := strings.SplitN(strings.TrimPrefix(data, "hzipan:"), ":", 3); len(p) == 3 {
			r.hzIPPoolFillDo(chat, mid, atoiSafe(p[0]), p[1], atoiSafe(p[2]))
		}
	case strings.HasPrefix(data, "hzipchk:"):
		r.hzIPPoolRecheck(chat, mid, atoiSafe(strings.TrimPrefix(data, "hzipchk:")))
	case strings.HasPrefix(data, "hzipsws:"):
		if p := strings.SplitN(strings.TrimPrefix(data, "hzipsws:"), ":", 2); len(p) == 2 {
			r.hzIPPoolSwapAsk(chat, mid, atoiSafe(p[0]), p[1])
		}
	case strings.HasPrefix(data, "hzipswy:"):
		if p := strings.SplitN(strings.TrimPrefix(data, "hzipswy:"), ":", 2); len(p) == 2 {
			r.hzIPPoolSwapDo(chat, mid, atoiSafe(p[0]), p[1])
		}
	case strings.HasPrefix(data, "hzipsw:"):
		r.hzIPPoolSwap(chat, mid, atoiSafe(strings.TrimPrefix(data, "hzipsw:")))
	case strings.HasPrefix(data, "hzipdel:"):
		r.hzIPPoolDeleteList(chat, mid, atoiSafe(strings.TrimPrefix(data, "hzipdel:")))
	case strings.HasPrefix(data, "hzipdely:"):
		if p := strings.SplitN(strings.TrimPrefix(data, "hzipdely:"), ":", 2); len(p) == 2 {
			r.hzIPPoolDeleteAsk(chat, mid, atoiSafe(p[0]), atoiSafe(p[1]))
		}
	case strings.HasPrefix(data, "hzipdeldo:"):
		if p := strings.SplitN(strings.TrimPrefix(data, "hzipdeldo:"), ":", 2); len(p) == 2 {
			r.hzIPPoolDeleteDo(chat, mid, atoiSafe(p[0]), atoiSafe(p[1]))
		}
	case strings.HasPrefix(data, "hzipcl:"):
		r.hzIPPoolCleanupAsk(chat, mid, atoiSafe(strings.TrimPrefix(data, "hzipcl:")))
	case strings.HasPrefix(data, "hzipcly:"):
		r.hzIPPoolCleanupDo(chat, mid, atoiSafe(strings.TrimPrefix(data, "hzipcly:")))
	case strings.HasPrefix(data, "hzsl:"):
		if p := strings.SplitN(strings.TrimPrefix(data, "hzsl:"), ":", 2); len(p) == 2 {
			r.hzPickPlan(chat, mid, atoiSafe(p[0]), p[1])
		}
	case strings.HasPrefix(data, "hzsc:"):
		r.setWizard(chat, map[string]any{"kind": "hz_srv_remark", "acc": atoiSafe(strings.TrimPrefix(data, "hzsc:"))})
		r.Show(chat, mid, "🏷️ نام سرور (بدون فاصله) را بفرستید:", [][]Btn{{btn("🔙 انصراف", "hz")}})
	case strings.HasPrefix(data, "hzscp:"):
		if p := strings.SplitN(strings.TrimPrefix(data, "hzscp:"), ":", 2); len(p) == 2 {
			i := atoiSafe(p[0])
			if w := r.getWizard(chat); w != nil {
				w["planId"] = p[1]
				r.setWizard(chat, w)
			}
			r.hzCreate(chat, mid, i)
		}
	case strings.HasPrefix(data, "hzcap:"):
		if p := strings.SplitN(strings.TrimPrefix(data, "hzcap:"), ":", 2); len(p) == 2 {
			r.hzPlanList(chat, mid, atoiSafe(p[0]),
				"این پلن با ظرفیت فعلی اکانت جا نمی‌شود؛ یک پلن کوچک‌تر انتخاب کن یا با دکمهٔ زیر به‌هرحال امتحان کن.", p[1])
		}
	case strings.HasPrefix(data, "hzsci:"):
		if p := strings.SplitN(strings.TrimPrefix(data, "hzsci:"), ":", 2); len(p) == 2 {
			r.hzPickPlanForImage(chat, mid, atoiSafe(p[0]), p[1])
		}
	case strings.HasPrefix(data, "hzsu:"):
		if p := strings.SplitN(strings.TrimPrefix(data, "hzsu:"), ":", 3); len(p) == 3 {
			r.hzOp(chat, mid, atoiSafe(p[0]), p[1], p[2])
		}
	case strings.HasPrefix(data, "hzsv:"):
		if p := strings.SplitN(strings.TrimPrefix(data, "hzsv:"), ":", 4); len(p) == 4 {
			r.hzPick(chat, mid, atoiSafe(p[0]), p[1], p[2], p[3])
		}
	case strings.HasPrefix(data, "hzok:"):
		if p := strings.SplitN(strings.TrimPrefix(data, "hzok:"), ":", 4); len(p) >= 3 {
			arg := ""
			if len(p) == 4 {
				arg = p[3]
			}
			r.hzDo(chat, mid, atoiSafe(p[0]), p[1], p[2], arg)
		}

	// servers
	case data == "srv":
		r.srvHome(chat, mid)
	case data == "srvaddbulk":
		r.srvAddBulk(chat, mid)
	case data == "srvadd":
		r.srvAdd(chat, mid)
	case data == "srvimport":
		r.srvImport(chat, mid)
	case data == "srvdel":
		r.srvDelList(chat, mid)
	case strings.HasPrefix(data, "srvopen:"):
		r.srvOpen(chat, mid, strings.TrimPrefix(data, "srvopen:"))
	case strings.HasPrefix(data, "srvdelx:"):
		r.srvDelAsk(chat, mid, strings.TrimPrefix(data, "srvdelx:"))
	case strings.HasPrefix(data, "srvdely:"):
		r.srvDelDo(chat, mid, strings.TrimPrefix(data, "srvdely:"))
	case strings.HasPrefix(data, "srvinfo:"):
		r.srvInfo(chat, mid, strings.TrimPrefix(data, "srvinfo:"))
	case strings.HasPrefix(data, "srvdisk:"):
		r.srvDisk(chat, mid, strings.TrimPrefix(data, "srvdisk:"))
	case strings.HasPrefix(data, "srvreboot:"):
		r.srvRebootAsk(chat, mid, strings.TrimPrefix(data, "srvreboot:"))
	case strings.HasPrefix(data, "srvrebootgo:"):
		r.srvRebootGo(chat, mid, strings.TrimPrefix(data, "srvrebootgo:"))
	case strings.HasPrefix(data, "srvupd:"):
		r.srvUpdateAsk(chat, mid, strings.TrimPrefix(data, "srvupd:"))
	case strings.HasPrefix(data, "srvupdgo:"):
		r.srvUpdateGo(chat, mid, strings.TrimPrefix(data, "srvupdgo:"))
	case strings.HasPrefix(data, "srvedit:"):
		r.srvEditMenu(chat, mid, strings.TrimPrefix(data, "srvedit:"))
	case strings.HasPrefix(data, "srvedith:"):
		if p := strings.SplitN(strings.TrimPrefix(data, "srvedith:"), ":", 2); len(p) == 2 {
			r.srvEditField(chat, mid, p[0], p[1])
		}
	case data == "srvpw":
		r.srvPwHome(chat, mid)
	case data == "srvpwadd":
		r.setWizard(chat, map[string]any{"kind": "srv_pw_add"})
		r.Show(chat, mid, "🔑 رمز عبور SSH موردنظر را بفرستید تا ذخیره شود:", [][]Btn{{btn("⬅️ انصراف", "srvpw")}})
	case strings.HasPrefix(data, "srvpwdel:"):
		r.srvPwDelAsk(chat, mid, strings.TrimPrefix(data, "srvpwdel:"))
	case strings.HasPrefix(data, "srvpwdely:"):
		r.srvPwDelDo(chat, mid, strings.TrimPrefix(data, "srvpwdely:"))
	case data == "srvmon":
		r.srvmonHome(chat, mid)
	case data == "srvmontg":
		r.srvmonToggle(chat, mid)
	case data == "srvmonrun":
		r.Show(chat, mid, "⏳ در حال بررسی همه سرورها…", HomeKB())
		r.srvmonRun(chat)
	case strings.HasPrefix(data, "srvmonh:"):
		r.wizStart(chat, mid, map[string]any{"kind": "srvmon_num", "key": strings.TrimPrefix(data, "srvmonh:")})
		r.Show(chat, mid, "🔢 عدد جدید را بفرستید:", [][]Btn{{btn("🔙 انصراف", "srvmon")}})
	case strings.HasPrefix(data, "srvstats:"):
		r.srvStats(chat, mid, strings.TrimPrefix(data, "srvstats:"))
	case data == "srvsub":
		r.srvSubSrc(chat, mid)
	case data == "srvsubzone":
		r.srvSubZones(chat, mid)
	case data == "srvsubfav":
		r.srvSubFavs(chat, mid)
	case strings.HasPrefix(data, "srvsubz:"):
		if p := strings.SplitN(strings.TrimPrefix(data, "srvsubz:"), ":", 2); len(p) == 2 {
			r.srvSubPick(chat, mid, p[0], p[1])
		}
	case strings.HasPrefix(data, "srvsubsel:"):
		r.srvSubToggle(chat, mid, strings.TrimPrefix(data, "srvsubsel:"))
	case strings.HasPrefix(data, "srvsubpg:"):
		r.srvSubPage(chat, mid, strings.TrimPrefix(data, "srvsubpg:"))
	case data == "srvsubsave":
		r.srvSubSave(chat, mid)
	case strings.HasPrefix(data, "srvsubpw:"):
		r.srvSubFinish(chat, mid, strings.TrimPrefix(data, "srvsubpw:"), true, nil)
	case data == "srvsubcancel":
		r.Store.Delete(fmt.Sprintf("srvsub:%d", chat))
		r.srvHome(chat, mid)

	// cloudflare
	case data == "cf", data == "zones":
		r.cfZones(chat, mid, "all", 0)
	case strings.HasPrefix(data, "zf:"):
		if p := strings.Split(data, ":"); len(p) >= 2 {
			pg := 0
			if len(p) > 2 && isDigits(strings.TrimLeft(p[2], "-")) {
				pg = atoiSafe(p[2])
			}
			r.cfZones(chat, mid, p[1], pg)
		}
	case data == "addrec":
		r.cfAddrecZones(chat, mid)
	case strings.HasPrefix(data, "addrec:"):
		if p := strings.SplitN(data[7:], ":", 2); len(p) == 2 {
			r.cfAddStart(chat, mid, atoiSafe(p[0]), p[1])
		}
	case data == "search":
		r.setWizard(chat, map[string]any{"kind": "cr_gsearch"})
		r.Show(chat, mid, cfSearchText, [][]Btn{{btn("❌ انصراف", "menu")}})
	case data == "qamenu":
		r.qaHome(chat, mid)
	case data == "qasearch":
		r.qaSearchStart(chat, mid)
	case data == "qacancel":
		r.qaCancel(chat, mid)
	case strings.HasPrefix(data, "qap:"):
		if p := strings.Split(data, ":"); len(p) == 3 {
			r.qaResults(chat, mid, p[1], atoiSafe(p[2]))
		}
	case strings.HasPrefix(data, "qadelall:"):
		r.qaDelAllAsk(chat, mid, data[9:])
	case strings.HasPrefix(data, "qadely:"):
		r.qaDelAllDo(chat, mid, data[7:])
	case strings.HasPrefix(data, "qafavall:"):
		r.qaFavAll(chat, mid, data[9:])
	case strings.HasPrefix(data, "qadd2:"):
		r.qaAddZones(chat, mid, "")
	case strings.HasPrefix(data, "qadd:"):
		r.qaAddZones(chat, mid, data[5:])
	case strings.HasPrefix(data, "qanz:"):
		if p := strings.Split(data, ":"); len(p) == 3 {
			r.qaAddZonePick(chat, mid, atoiSafe(p[1]), p[2])
		}
	case data == "favs":
		r.cfFavsPage(chat, mid)
	case strings.HasPrefix(data, "favrm:"):
		r.cfFavRm(chat, mid, strings.TrimPrefix(data, "favrm:"))
	case data == "accounts":
		r.cfAccountsHome(chat, mid)
	case data == "accadd":
		r.setWizard(chat, map[string]any{"kind": "cr_cf_acc_name"})
		r.Show(chat, mid, "👤 چه اسمی برای اکانت کلادفلر بزارم؟",
			navKB("accounts"))
	case data == "accdel":
		r.cfAccDelList(chat, mid)
	case strings.HasPrefix(data, "accd:"):
		r.cfAccDelAsk(chat, mid, strings.TrimPrefix(data, "accd:"))
	case strings.HasPrefix(data, "accdy:"):
		r.cfAccDelDo(chat, mid, strings.TrimPrefix(data, "accdy:"))
	case data == "zsearch":
		sess := r.cfGetSession(chat)
		if strvVal(sess["zid"]) == "" {
			r.Show(chat, mid, "⏳ نشست منقضی شد؛ دامنه را دوباره باز کن.", HomeKB())
		} else {
			r.setWizard(chat, map[string]any{"kind": "cr_zsearch"})
			r.Show(chat, mid, "🔍 بخشی از نام یا مقدار رکورد را بفرستید:", [][]Btn{{btn("❌ انصراف", "zones")}})
		}
	case data == "zset":
		r.cfZset(chat, mid)
	case data == "zssl", data == "zsec", data == "zperf":
		r.cfSettingsGroup(chat, mid, data)
	case strings.HasPrefix(data, "ztog:"):
		r.cfSettingToggle(chat, mid, strings.TrimPrefix(data, "ztog:"))
	case strings.HasPrefix(data, "zval:"):
		r.cfSettingEnum(chat, mid, strings.TrimPrefix(data, "zval:"))
	case strings.HasPrefix(data, "zvset:"):
		if p := strings.SplitN(strings.TrimPrefix(data, "zvset:"), ":", 2); len(p) == 2 {
			r.cfSettingSet(chat, mid, p[0], p[1])
		}
	case data == "zd":
		r.cfZoneDetails(chat, mid)
	case data == "zpurge":
		r.cfPurge(chat, mid)
	case data == "zdev":
		r.cfDevToggle(chat, mid)
	case data == "zpause":
		r.cfZonePause(chat, mid)
	case data == "rtraf":
		r.cfRecordTraffic(chat, mid)
	case data == "cref":
		r.cfCnameRefs(chat, mid)
	case data == "ztraf":
		sess := r.cfGetSession(chat)
		if zid := strvVal(sess["zid"]); zid != "" {
			r.trafZone(chat, mid, intOrAny(sess["acc"]), zid, 0)
		} else {
			r.Show(chat, mid, "📊 ترافیک ساب‌ها", navKB("zones"))
		}
	case strings.HasPrefix(data, "cfz:"):
		if p := strings.SplitN(data[4:], ":", 2); len(p) == 2 {
			r.cfRecords(chat, mid, atoiSafe(p[0]), p[1], 0)
		}
	case strings.HasPrefix(data, "cfp:"):
		sess := r.cfGetSession(chat)
		if zid := strvVal(sess["zid"]); zid != "" {
			r.cfRecords(chat, mid, intOrAny(sess["acc"]), zid, atoiSafe(strings.TrimPrefix(data, "cfp:")))
		} else {
			r.Show(chat, mid, "⏳ نشست منقضی شد؛ دامنه را دوباره باز کن.", HomeKB())
		}
	case strings.HasPrefix(data, "cfarp:"):
		r.cfArProxy(chat, mid, strings.TrimPrefix(data, "cfarp:") == "1")
	case data == "cfarforce":
		r.cfArForce(chat, mid)
	case strings.HasPrefix(data, "cfr:"):
		rid := strings.TrimPrefix(data, "cfr:")
		if acc, zid := r.cfCtx(chat, rid); zid == "" {
			r.Show(chat, mid, "⏳ نشست منقضی شد؛ دامنه را دوباره باز کن.", HomeKB())
		} else {
			r.cfRecord(chat, mid, acc, zid, rid, "")
		}
	case strings.HasPrefix(data, "cfdel:"):
		rid := strings.TrimPrefix(data, "cfdel:")
		if acc, zid := r.cfCtx(chat, rid); zid == "" {
			r.Show(chat, mid, "⏳ نشست منقضی شد؛ دامنه را دوباره باز کن.", HomeKB())
		} else {
			r.cfDelAsk(chat, mid, acc, zid, rid)
		}
	case strings.HasPrefix(data, "cfdelok:"):
		rid := strings.TrimPrefix(data, "cfdelok:")
		if acc, zid := r.cfCtx(chat, rid); zid == "" {
			r.Show(chat, mid, "⏳ نشست منقضی شد؛ دامنه را دوباره باز کن.", HomeKB())
		} else {
			r.cfDelDo(chat, mid, acc, zid, rid)
		}
	case strings.HasPrefix(data, "cfpx:"):
		rid := strings.TrimPrefix(data, "cfpx:")
		if acc, zid := r.cfCtx(chat, rid); zid != "" {
			r.cfTogglePx(chat, mid, acc, zid, rid)
		}
	case strings.HasPrefix(data, "cfttl:"):
		rid := strings.TrimPrefix(data, "cfttl:")
		if acc, zid := r.cfCtx(chat, rid); zid != "" {
			r.cfCycleTTL(chat, mid, acc, zid, rid)
		}
	case strings.HasPrefix(data, "cfedit:"):
		rid := strings.TrimPrefix(data, "cfedit:")
		if acc, zid := r.cfCtx(chat, rid); zid == "" {
			r.Show(chat, mid, "⏳ نشست منقضی شد؛ دامنه را دوباره باز کن.", HomeKB())
		} else {
			r.cfEditStart(chat, mid, acc, zid, rid)
		}
	case strings.HasPrefix(data, "cfadd:"):
		if p := strings.SplitN(data[6:], ":", 2); len(p) == 2 {
			r.cfAddStart(chat, mid, atoiSafe(p[0]), p[1])
		}
	case strings.HasPrefix(data, "cfat:"):
		if p := strings.SplitN(data[5:], ":", 3); len(p) == 3 {
			r.cfAddType(chat, mid, atoiSafe(p[0]), p[1], p[2])
		}
	case data == "selmode":
		sess := r.cfGetSession(chat)
		if zid := strvVal(sess["zid"]); zid == "" {
			r.Show(chat, mid, "⏳ نشست منقضی شد.", HomeKB())
		} else {
			sess["sel"] = []any{}
			r.cfSaveSession(chat, sess)
			r.cfRecords(chat, mid, intOrAny(sess["acc"]), zid, 0)
		}
	case strings.HasPrefix(data, "sel:"):
		rid := strings.TrimPrefix(data, "sel:")
		sess := r.cfGetSession(chat)
		if zid := strvVal(sess["zid"]); zid == "" {
			r.Show(chat, mid, "⏳ نشست منقضی شد.", HomeKB())
		} else {
			sel, _ := sess["sel"].([]any)
			found := false
			var out []any
			for _, x := range sel {
				if strvVal(x) == rid {
					found = true
				} else {
					out = append(out, x)
				}
			}
			if !found {
				out = append(out, rid)
			}
			sess["sel"] = out
			r.cfSaveSession(chat, sess)
			r.cfRecords(chat, mid, intOrAny(sess["acc"]), zid, intOrAny(sess["pg"]))
		}
	case strings.HasPrefix(data, "selp:"):
		sess := r.cfGetSession(chat)
		if zid := strvVal(sess["zid"]); zid == "" {
			r.Show(chat, mid, "⏳ نشست منقضی شد.", HomeKB())
		} else {
			pg := atoiSafe(strings.TrimPrefix(data, "selp:"))
			sess["pg"] = pg
			r.cfSaveSession(chat, sess)
			r.cfRecords(chat, mid, intOrAny(sess["acc"]), zid, pg)
		}
	case data == "seldone":
		sess := r.cfGetSession(chat)
		delete(sess, "sel")
		r.cfSaveSession(chat, sess)
		if zid := strvVal(sess["zid"]); zid != "" {
			r.cfRecords(chat, mid, intOrAny(sess["acc"]), zid, intOrAny(sess["pg"]))
		} else {
			r.Show(chat, mid, "⏳ نشست منقضی شد.", HomeKB())
		}
	case data == "bulkdel":
		sess := r.cfGetSession(chat)
		if n := lenAny(sess["sel"]); n == 0 {
			r.Show(chat, mid, "⚠️ چیزی انتخاب نشده.", HomeKB())
		} else {
			r.Show(chat, mid, fmt.Sprintf("⚠️ %d رکورد حذف شود؟", n),
				[][]Btn{{btnS("✅ بله، حذف کن", "bulkdelok", "danger"), btn("❌ انصراف", "seldone")}})
		}
	case data == "bulkdelok":
		r.cfBulkDelDo(chat, mid)
	case data == "bulkedit":
		sess := r.cfGetSession(chat)
		if lenAny(sess["sel"]) == 0 {
			r.Show(chat, mid, "⚠️ چیزی انتخاب نشده.", HomeKB())
		} else {
			r.setWizard(chat, map[string]any{"kind": "cr_bulkedit"})
			r.Show(chat, mid, "✏️ مقدار جدید همه را بفرستید:", [][]Btn{{btn("⬅️ انصراف", "seldone")}})
		}
	case data == "bulktype":
		kb := gridBTN([]Btn{btn("A", "bulktypev:A"), btn("AAAA", "bulktypev:AAAA"), btn("CNAME", "bulktypev:CNAME")}, 3)
		kb = append(kb, []Btn{btn("❌ انصراف", "seldone")})
		r.Show(chat, mid, "🔄 نوع جدید همه را انتخاب کنید:", kb)
	case strings.HasPrefix(data, "bulktypev:"):
		r.cfBulkTypeDo(chat, mid, strings.TrimPrefix(data, "bulktypev:"))
	case data == "favsel":
		r.cfFavAddMany(chat, mid, r.cfGetSession(chat))
	case strings.HasPrefix(data, "ct:"):
		rid := strings.TrimPrefix(data, "ct:")
		if acc, zid := r.cfCtx(chat, rid); zid != "" {
			r.cfTypeToggle(chat, mid, acc, zid, rid)
		}
	case strings.HasPrefix(data, "favtg:"):
		rid := strings.TrimPrefix(data, "favtg:")
		if acc, zid := r.cfCtx(chat, rid); zid != "" {
			r.cfFavToggle(chat, mid, acc, zid, rid)
		}

	// version
	case data == "vers":
		r.versHome(chat, mid)
	case data == "verlatest":
		r.verLatest(chat, mid)
	case data == "verlist":
		r.verList(chat, mid)
	case strings.HasPrefix(data, "verdoYes:"):
		r.verDoYes(chat, mid, strings.TrimPrefix(data, "verdoYes:"))
	case strings.HasPrefix(data, "verdo:"):
		r.verDo(chat, mid, strings.TrimPrefix(data, "verdo:"))
	case strings.HasPrefix(data, "updlater:"):
		r.updLater(chat, mid, strings.TrimPrefix(data, "updlater:"))
	}
}
