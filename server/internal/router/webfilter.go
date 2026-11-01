package router

import (
	"fmt"
	"net/http"
	"strings"

	"github.com/aknuun/cg-server/internal/hostfilter"
)

func hfExcSet(cfg hostfilter.Cfg) map[string]bool {
	m := map[string]bool{}
	for _, x := range cfg.Exceptions {
		m[x] = true
	}
	return m
}

func hfExcList(set map[string]bool) []string {
	out := make([]string, 0, len(set))
	for k := range set {
		out = append(out, k)
	}
	sortStrings(out)
	return out
}

func hfList(v any) []string {
	if out := selStrings(v); len(out) > 0 {
		return out
	}
	return []string{}
}

func sortStrings(a []string) {
	for i := 1; i < len(a); i++ {
		for j := i; j > 0 && a[j] < a[j-1]; j-- {
			a[j], a[j-1] = a[j-1], a[j]
		}
	}
}

// webToolsHf returns the host-filter config plus the panel list.
func (r *Router) webToolsHf(w http.ResponseWriter, req *http.Request, uid int64) {
	cfg := hostfilter.GetCfg(r.Store)
	var panels []map[string]any
	for i, p := range r.panels() {
		panels = append(panels, map[string]any{"idx": i, "name": pname(p), "url": p.URL})
	}
	webOK(w, map[string]any{
		"cfg": map[string]any{
			"enabled": cfg.IsEnabled(), "intervalMin": cfg.IntervalMin,
			"cities": cfg.Cities, "citiesSel": cfg.CitiesSel, "maxOk": cfg.MaxOk,
			"recheckCount": cfg.RecheckCount, "recheckMin": cfg.RecheckMin,
			"batch": cfg.Batch, "maxChanges": cfg.MaxChanges, "backupKeep": cfg.BackupKeep,
			"realityRotate": cfg.RealityRotate, "lastRun": cfg.LastRun,
			"lastSummary": cfg.LastSummary, "excCount": len(cfg.Exceptions),
		},
		"panels": panels,
	})
}

type webHfCfgIn struct {
	Enabled     *bool `json:"enabled"`
	IntervalMin *int  `json:"intervalMin"`
	Cities      *int  `json:"cities"`
	MaxOk       *int  `json:"maxOk"`
	RecheckCnt  *int  `json:"recheckCount"`
	RecheckMin  *int  `json:"recheckMin"`
	Batch       *int  `json:"batch"`
	MaxChanges  *int  `json:"maxChanges"`
}

func (r *Router) webHfCfg(w http.ResponseWriter, req *http.Request, uid int64) {
	var in webHfCfgIn
	if !webDecode(w, req, &in) {
		return
	}
	cfg := hostfilter.GetCfg(r.Store)
	if in.Enabled != nil {
		v := *in.Enabled
		cfg.Enabled = &v
	}
	set := func(dst *int, src *int, lo, hi int, label string) bool {
		if src == nil {
			return true
		}
		if *src < lo || *src > hi {
			webErr(w, http.StatusBadRequest, fmt.Sprintf("%s باید بین %d و %d باشد", label, lo, hi))
			return false
		}
		*dst = *src
		return true
	}
	if !set(&cfg.IntervalMin, in.IntervalMin, 1, 1440, "فاصلهٔ اجرا") ||
		!set(&cfg.Cities, in.Cities, 1, 10, "حداقل شهر") ||
		!set(&cfg.MaxOk, in.MaxOk, 0, 4, "حداکثر پینگ موفق") ||
		!set(&cfg.RecheckCount, in.RecheckCnt, 0, 10, "تعداد بررسی مجدد") ||
		!set(&cfg.RecheckMin, in.RecheckMin, 1, 60, "فاصلهٔ بررسی مجدد") ||
		!set(&cfg.Batch, in.Batch, 1, 500, "تعداد هر اجرا") ||
		!set(&cfg.MaxChanges, in.MaxChanges, 1, 100, "حداکثر تعویض") {
		return
	}
	hostfilter.SaveCfg(r.Store, cfg)
	webOK(w, map[string]any{"ok": true})
}

// webHfHosts lists one panel's hosts with their per-field exception state.
func (r *Router) webHfHosts(w http.ResponseWriter, req *http.Request, uid int64) {
	pi := intOrAny(req.URL.Query().Get("panel"))
	panels := r.panels()
	if pi < 0 || pi >= len(panels) {
		webErr(w, http.StatusNotFound, "پنل پیدا نشد")
		return
	}
	c := panels[pi].Client()
	tok, err := c.Login()
	if err != nil {
		webErr(w, http.StatusBadGateway, "ورود به پنل ناموفق")
		return
	}
	hosts, err := c.Hosts(tok)
	if err != nil {
		webErr(w, http.StatusBadGateway, "خواندن هاست‌ها ناموفق")
		return
	}
	cfg := hostfilter.GetCfg(r.Store)
	exc := hfExcSet(cfg)
	var out []map[string]any
	for _, hv := range hosts {
		h, ok := hv.(map[string]any)
		if !ok {
			continue
		}
		hid := strv(h["id"])
		base := fmt.Sprintf("%d:%s", pi, hid)
		auto := hostfilter.IsAutoHost(h)
		forced := map[string]bool{}
		for _, f := range []string{"address", "sni", "host"} {
			forced[f] = hostfilter.ForcedKey(cfg, pi, hid, f) || hostfilter.ForcedKey(cfg, pi, hid, "")
		}
		skip := map[string]bool{}
		for _, f := range []string{"address", "sni", "host"} {
			skip[f] = exc[base+":"+f] || exc[base] || (auto && !forced[f])
		}
		out = append(out, map[string]any{
			"id": hid, "remark": strv(h["remark"]),
			"disabled": boolVal(h["is_disabled"]), "reality": hfReality(h), "auto": auto,
			"address": hfList(h["address"]), "sni": hfList(h["sni"]),
			"host": hfList(h["host"]), "skip": skip,
			"all": exc[base],
		})
	}
	webOK(w, map[string]any{"hosts": out})
}

type webHfExcIn struct {
	Panel int    `json:"panel"`
	Host  string `json:"host"`
	Field string `json:"field"`
	All   bool   `json:"all"`
	Clear bool   `json:"clear"`
	// Allow toggles the force-back list for auto-excluded hosts.
	Allow bool `json:"allow"`
}

func hfToggleForce(list []string, key string) []string {
	for i, k := range list {
		if k == key {
			return append(append([]string{}, list[:i]...), list[i+1:]...)
		}
	}
	return append(append([]string{}, list...), key)
}

func (r *Router) webHfExc(w http.ResponseWriter, req *http.Request, uid int64) {
	var in webHfExcIn
	if !webDecode(w, req, &in) {
		return
	}
	cfg := hostfilter.GetCfg(r.Store)
	set := hfExcSet(cfg)
	if in.Allow && in.Host != "" {
		// auto-excluded hosts (disabled / status text) are put back field by field
		if in.All {
			cfg.Forced = hfToggleForce(cfg.Forced, fmt.Sprintf("%d:%s", in.Panel, in.Host))
		} else {
			cfg.Forced = hfToggleForce(cfg.Forced, fmt.Sprintf("%d:%s:%s", in.Panel, in.Host, in.Field))
		}
		delete(set, fmt.Sprintf("%d:%s:%s", in.Panel, in.Host, in.Field))
		cfg.Exceptions = hfExcList(set)
		hostfilter.SaveCfg(r.Store, cfg)
		webOK(w, map[string]any{"ok": true})
		return
	}
	if in.Clear {
		pre := fmt.Sprintf("%d:", in.Panel)
		for k := range set {
			if strings.HasPrefix(k, pre) {
				delete(set, k)
			}
		}
	} else if in.Host != "" {
		key := fmt.Sprintf("%d:%s", in.Panel, in.Host)
		if !in.All {
			key += ":" + in.Field
		}
		if set[key] {
			delete(set, key)
		} else {
			set[key] = true
		}
	} else {
		webErr(w, http.StatusBadRequest, "هاست مشخص نشده")
		return
	}
	cfg.Exceptions = hfExcList(set)
	hostfilter.SaveCfg(r.Store, cfg)
	webOK(w, map[string]any{"ok": true})
}

func (r *Router) webHfRun(w http.ResponseWriter, req *http.Request, uid int64) {
	go func() {
		res := hostfilter.Run(r.Bot, r.Store, r.panels(), uid, r.cfAccs(), true, "")
		msg := fmt.Sprintf("✅ بررسی هاست‌ها تمام شد: %d دامنه بررسی، %d تعویض.", res.Checked, res.Changed)
		if res.Msg != "" {
			msg = "⚠️ " + res.Msg
		}
		r.Bot.Send(uid, msg, nil)
	}()
	webOK(w, map[string]any{"started": true})
}

func (r *Router) webHfLog(w http.ResponseWriter, req *http.Request, uid int64) {
	var log []map[string]any
	r.Store.GetJSON("host_filter_log", &log)
	if len(log) > 40 {
		log = log[len(log)-40:]
	}
	webOK(w, map[string]any{"log": log})
}

func (r *Router) webHfRevert(w http.ResponseWriter, req *http.Request, uid int64) {
	if req.Method == http.MethodGet {
		hostfilter.EnsureStateFromLog(r.Store, r.panels())
		webOK(w, map[string]any{"items": hostfilter.StateEntries(r.Store)})
		return
	}
	var in struct {
		SID string `json:"sid"`
	}
	if !webDecode(w, req, &in) {
		return
	}
	if strings.TrimSpace(in.SID) == "" {
		webErr(w, http.StatusBadRequest, "شناسه لازم است")
		return
	}
	if _, err := hostfilter.Revert(r.Store, r.panels(), in.SID); err != nil {
		webErr(w, http.StatusBadRequest, "بازگردانی ناموفق: "+hfErrFA(err))
		return
	}
	webOK(w, map[string]any{"ok": true})
}
