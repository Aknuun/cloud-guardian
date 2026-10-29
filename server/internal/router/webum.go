package router

import (
	"net/http"

	"github.com/aknuun/cg-server/internal/um"
)

// webToolsUm reads or saves the bad-consumption ("bad usage") monitor config.
func (r *Router) webToolsUm(w http.ResponseWriter, req *http.Request, uid int64) {
	if req.Method == http.MethodPost {
		var in um.Config
		if !webDecode(w, req, &in) {
			return
		}
		if in.HourlyGb < 0 || in.DailyGb < 0 || in.CooldownH < 0 || in.MinGb < 0 || in.ActiveMin < 0 {
			webErr(w, http.StatusBadRequest, "مقدار نامعتبر است")
			return
		}
		um.SaveCfg(r.Store, in)
		webOK(w, um.Cfg(r.Store))
		return
	}
	webOK(w, um.Cfg(r.Store))
}
