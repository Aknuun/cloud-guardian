package router

import (
	"fmt"
	"net/http"
	"time"
)

// webToolsHfd returns the daily sub-rotation config plus panel list.
func (r *Router) webToolsHfd(w http.ResponseWriter, req *http.Request, uid int64) {
	cfg := r.hfdCfg()
	var panels []map[string]any
	for i, p := range r.panels() {
		panels = append(panels, map[string]any{"idx": i, "name": pname(p)})
	}
	items := []map[string]any{}
	for key, it := range cfg.Items {
		pi, hid := hfdKeyParse(key)
		if pi < 0 {
			continue
		}
		items = append(items, map[string]any{
			"key": key, "panel": pi, "host": hid,
			"fields": it.Fields, "prev": it.Prev,
		})
	}
	webOK(w, map[string]any{
		"cfg": map[string]any{
			"enabled": cfg.Enabled, "intervalH": cfg.IntervalH, "at": cfg.At,
			"nextRun": cfg.NextRun, "totalCreated": cfg.TotalCreated,
		},
		"panels": panels,
		"items":  items,
	})
}

func (r *Router) webHfdCfg(w http.ResponseWriter, req *http.Request, uid int64) {
	var in struct {
		Enabled   *bool  `json:"enabled"`
		IntervalH *int   `json:"intervalH"`
		At        string `json:"at"`
	}
	if !webDecode(w, req, &in) {
		return
	}
	cfg := r.hfdCfg()
	if in.Enabled != nil {
		cfg.Enabled = *in.Enabled
	}
	if in.IntervalH != nil {
		if *in.IntervalH < 1 || *in.IntervalH > 720 {
			webErr(w, http.StatusBadRequest, "فاصله باید بین ۱ تا ۷۲۰ ساعت باشد")
			return
		}
		cfg.IntervalH = *in.IntervalH
	}
	if in.At != "" {
		if !hfdAtOK(in.At) {
			webErr(w, http.StatusBadRequest, "ساعت باید شکل HH:MM باشد")
			return
		}
		cfg.At = in.At
	}
	cfg.NextRun = hfdNextRunISO(cfg.At, cfg.IntervalH, time.Now().UnixMilli())
	r.hfdCfgSave(cfg)
	webOK(w, map[string]any{"ok": true, "nextRun": cfg.NextRun})
}

// webHfdHosts lists a panel's hosts with their rotation field marks.
func (r *Router) webHfdHosts(w http.ResponseWriter, req *http.Request, uid int64) {
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
	cfg := r.hfdCfg()
	var out []map[string]any
	for _, hv := range hosts {
		h, ok := hv.(map[string]any)
		if !ok {
			continue
		}
		hid := strv(h["id"])
		it := cfg.Items[hfdKey(pi, hid)]
		fields := map[string]bool{}
		for _, f := range []string{"address", "sni", "host"} {
			fields[f] = it.Fields[f]
		}
		out = append(out, map[string]any{
			"id": hid, "remark": strv(h["remark"]),
			"disabled": boolVal(h["is_disabled"]), "reality": hfReality(h),
			"fields": fields,
		})
	}
	webOK(w, map[string]any{"hosts": out})
}

func (r *Router) webHfdField(w http.ResponseWriter, req *http.Request, uid int64) {
	var in struct {
		Panel int    `json:"panel"`
		Host  string `json:"host"`
		Field string `json:"field"`
	}
	if !webDecode(w, req, &in) {
		return
	}
	if in.Field != "address" && in.Field != "sni" && in.Field != "host" {
		webErr(w, http.StatusBadRequest, "فیلد نامعتبر است")
		return
	}
	cfg := r.hfdCfg()
	key := hfdKey(in.Panel, in.Host)
	it, ok := cfg.Items[key]
	if !ok {
		it = hfdNewItem()
	}
	if it.Fields == nil {
		it.Fields = map[string]bool{}
	}
	it.Fields[in.Field] = !it.Fields[in.Field]
	if !it.Fields["address"] && !it.Fields["sni"] && !it.Fields["host"] {
		delete(cfg.Items, key)
	} else {
		cfg.Items[key] = it
	}
	r.hfdCfgSave(cfg)
	webOK(w, map[string]any{"ok": true, "on": it.Fields[in.Field]})
}

func (r *Router) webHfdRun(w http.ResponseWriter, req *http.Request, uid int64) {
	go func() {
		changed, skipped, lines := r.hfdRotate(true)
		msg := fmt.Sprintf("✅ چرخش روزانه: %d هاست چرخید", changed)
		if skipped > 0 {
			msg += fmt.Sprintf(" · ⏭ %d رد شد", skipped)
		}
		if len(lines) > 0 {
			if len(lines) > 20 {
				lines = lines[:20]
			}
			msg += "\n\n" + joinLines(lines)
		}
		r.Bot.Send(uid, msg, nil)
	}()
	webOK(w, map[string]any{"started": true})
}

func joinLines(v []string) string {
	out := ""
	for i, s := range v {
		if i > 0 {
			out += "\n"
		}
		out += s
	}
	return out
}

func (r *Router) webHfdLog(w http.ResponseWriter, req *http.Request, uid int64) {
	var log []map[string]any
	r.Store.GetJSON("host_filter_log", &log)
	var out []map[string]any
	for i := len(log) - 1; i >= 0 && len(out) < 40; i-- {
		if strv(log[i]["kind"]) != "dailyrot" && strv(log[i]["kind"]) != "revert" {
			continue
		}
		out = append(out, log[i])
	}
	webOK(w, map[string]any{"log": out})
}

// webHfdRevert lists restorable entries (GET) or restores one (POST).
func (r *Router) webHfdRevert(w http.ResponseWriter, req *http.Request, uid int64) {
	cfg := r.hfdCfg()
	if req.Method == http.MethodGet {
		var out []map[string]any
		for key, it := range cfg.Items {
			pi, hid := hfdKeyParse(key)
			if pi < 0 {
				continue
			}
			n := 0
			for _, f := range []string{"address", "sni", "host"} {
				n += len(it.Prev[f])
			}
			if n == 0 {
				continue
			}
			out = append(out, map[string]any{"key": key, "panel": pi, "host": hid, "prev": n})
		}
		webOK(w, map[string]any{"items": out})
		return
	}
	var in struct {
		Panel int    `json:"panel"`
		Host  string `json:"host"`
	}
	if !webDecode(w, req, &in) {
		return
	}
	panels := r.panels()
	if in.Panel < 0 || in.Panel >= len(panels) {
		webErr(w, http.StatusNotFound, "پنل پیدا نشد")
		return
	}
	key := hfdKey(in.Panel, in.Host)
	it, ok := cfg.Items[key]
	if !ok {
		webErr(w, http.StatusNotFound, "موردی برای بازگردانی نیست")
		return
	}
	c := panels[in.Panel].Client()
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
	var host map[string]any
	for _, hv := range hosts {
		h, _ := hv.(map[string]any)
		if strv(h["id"]) == in.Host {
			host = h
			break
		}
	}
	if host == nil {
		webErr(w, http.StatusNotFound, "هاست پیدا نشد")
		return
	}
	var fields []string
	for _, f := range []string{"address", "sni", "host"} {
		if pv := it.Prev[f]; len(pv) > 0 {
			host[f] = []string{pv[len(pv)-1]}
			it.Prev[f] = pv[:len(pv)-1]
			fields = append(fields, f)
		}
	}
	if len(fields) == 0 {
		webErr(w, http.StatusBadRequest, "چیزی برای بازگردانی نیست")
		return
	}
	if err := c.PutHost(tok, strv(host["id"]), host); err != nil {
		webErr(w, http.StatusBadGateway, "بازگردانی ناموفق: "+hfErrFA(err))
		return
	}
	cfg.Items[key] = it
	r.hfdCfgSave(cfg)
	r.hfdLogAdd(map[string]any{"kind": "revert", "key": key, "remark": strv(host["remark"])})
	webOK(w, map[string]any{"ok": true, "fields": fields})
}
