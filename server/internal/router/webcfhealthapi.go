package router

import (
	"net/http"
	"strings"

	"github.com/aknuun/cg-server/internal/cf"
)

// webToolsCfCheck serves the checker page state.
func (r *Router) webToolsCfCheck(w http.ResponseWriter, req *http.Request, uid int64) {
	cfg := r.cfHealthCfg()
	last := r.cfHealthLastGet()
	accs := r.cfAccs()
	names := make([]string, 0, len(accs))
	for _, a := range accs {
		names = append(names, a.Name)
	}
	webOK(w, map[string]any{
		"enabled":     cfg.Enabled,
		"intervalMin": cfg.IntervalMin,
		"lastRun":     cfg.LastRun,
		"accounts":    names,
		"last":        last,
	})
}

// webCfHealthRun checks everything, one zone, or a single record.
func (r *Router) webCfHealthRun(w http.ResponseWriter, req *http.Request, uid int64) {
	var in struct {
		Acc  int    `json:"acc"`
		Zid  string `json:"zid"`
		Rid  string `json:"rid"`
		Zone string `json:"zone"`
	}
	if !webDecode(w, req, &in) {
		return
	}
	acc := in.Acc
	if acc < 0 || acc >= len(r.cfAccs()) {
		acc = -1
	}
	last := r.cfHealthScan(acc, in.Zid, in.Rid)
	webOK(w, last)
}

// webCfHealthCfg saves the monitor settings.
func (r *Router) webCfHealthCfgSave(w http.ResponseWriter, req *http.Request, uid int64) {
	var in struct {
		Enabled     bool `json:"enabled"`
		IntervalMin int  `json:"intervalMin"`
	}
	if !webDecode(w, req, &in) {
		return
	}
	if in.IntervalMin < 5 {
		in.IntervalMin = 5
	}
	if in.IntervalMin > 1440 {
		in.IntervalMin = 1440
	}
	cfg := r.cfHealthCfg()
	cfg.Enabled = in.Enabled
	cfg.IntervalMin = in.IntervalMin
	r.Store.PutJSON(cfHealthCfgKey, cfg, 0)
	webOK(w, map[string]any{"ok": true})
}

// webCfHealthFix applies a repair chosen on a result row.
func (r *Router) webCfHealthFix(w http.ResponseWriter, req *http.Request, uid int64) {
	var in struct {
		Acc     int    `json:"acc"`
		Zid     string `json:"zid"`
		Rid     string `json:"rid"`
		Content string `json:"content"`
		Type    string `json:"type"`
		Del     bool   `json:"del"`
	}
	if !webDecode(w, req, &in) {
		return
	}
	accs := r.cfAccs()
	if in.Acc < 0 || in.Acc >= len(accs) {
		webErr(w, http.StatusBadRequest, "اکانت نامعتبر است")
		return
	}
	cli := cf.New(accs[in.Acc].Token)
	if in.Del {
		if err := cli.DelRecord(in.Zid, in.Rid); err != nil {
			webErr(w, http.StatusBadGateway, "حذف نشد: "+err.Error())
			return
		}
		webOK(w, map[string]any{"ok": true})
		return
	}
	cur := r.cfFind(in.Acc, in.Zid, in.Rid)
	if cur == nil {
		webErr(w, http.StatusNotFound, "رکورد پیدا نشد")
		return
	}
	changes := map[string]any{}
	if c := strings.TrimSpace(in.Content); c != "" {
		changes["content"] = c
	}
	if t := strings.ToUpper(strings.TrimSpace(in.Type)); t != "" {
		changes["type"] = t
	}
	if len(changes) == 0 {
		webErr(w, http.StatusBadRequest, "چیزی برای تغییر نیست")
		return
	}
	if _, err := cli.UpdateRecord(in.Zid, *cur, changes); err != nil {
		webErr(w, http.StatusBadGateway, "ذخیره نشد: "+err.Error())
		return
	}
	webOK(w, map[string]any{"ok": true})
}
