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
		Acc  *int     `json:"acc"`
		Zid  string   `json:"zid"`
		Rid  string   `json:"rid"`
		Zids []string `json:"zids"`
	}
	if !webDecode(w, req, &in) {
		return
	}
	acc := -1
	if in.Acc != nil && *in.Acc >= 0 && *in.Acc < len(r.cfAccs()) {
		acc = *in.Acc
	}
	var zids map[string]bool
	if len(in.Zids) > 0 {
		zids = map[string]bool{}
		for _, z := range in.Zids {
			if z != "" {
				zids[z] = true
			}
		}
	}
	last := r.cfHealthScan(acc, in.Zid, in.Rid, zids)
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

// webCfHealthBulk applies one fix to many rows at once.
func (r *Router) webCfHealthBulk(w http.ResponseWriter, req *http.Request, uid int64) {
	type item struct {
		Acc int    `json:"acc"`
		Zid string `json:"zid"`
		Rid string `json:"rid"`
	}
	var in struct {
		Op      string `json:"op"` // del | edit
		Content string `json:"content"`
		Type    string `json:"type"`
		Items   []item `json:"items"`
	}
	if !webDecode(w, req, &in) {
		return
	}
	if in.Op != "del" && in.Op != "edit" {
		webErr(w, http.StatusBadRequest, "عملیات نامعتبر است")
		return
	}
	if in.Op == "edit" && strings.TrimSpace(in.Content) == "" {
		webErr(w, http.StatusBadRequest, "مقدار جدید را وارد کن")
		return
	}
	accs := r.cfAccs()
	ok, fail := 0, 0
	for _, it := range in.Items {
		if it.Acc < 0 || it.Acc >= len(accs) || it.Zid == "" || it.Rid == "" {
			fail++
			continue
		}
		cli := cf.New(accs[it.Acc].Token)
		var err error
		if in.Op == "del" {
			err = cli.DelRecord(it.Zid, it.Rid)
		} else {
			cur := r.cfFind(it.Acc, it.Zid, it.Rid)
			if cur == nil {
				fail++
				continue
			}
			ch := map[string]any{"content": strings.TrimSpace(in.Content)}
			if t := strings.ToUpper(strings.TrimSpace(in.Type)); t != "" {
				ch["type"] = t
			}
			_, err = cli.UpdateRecord(it.Zid, *cur, ch)
		}
		if err != nil {
			fail++
		} else {
			ok++
		}
	}
	webOK(w, map[string]any{"ok": ok, "fail": fail})
}
