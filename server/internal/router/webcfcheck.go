package router

import (
	"context"
	"net"
	"net/http"
	"strings"
	"time"
)

// webCfResolve tells the UI whether a CNAME target actually points anywhere.
// It never blocks the save by itself; the panel asks the user before writing.
func (r *Router) webCfResolve(w http.ResponseWriter, req *http.Request, uid int64) {
	name := strings.TrimSpace(req.URL.Query().Get("name"))
	name = strings.TrimSuffix(strings.TrimPrefix(name, "http://"), "/")
	name = strings.TrimSuffix(strings.TrimPrefix(name, "https://"), "/")
	if name == "" {
		webErr(w, http.StatusBadRequest, "نام هدف لازم است")
		return
	}
	ctx, cancel := context.WithTimeout(req.Context(), 6*time.Second)
	defer cancel()
	ips, err := net.DefaultResolver.LookupHost(ctx, name)
	msg := ""
	if err != nil || len(ips) == 0 {
		msg = "هدف ریسالو نشد"
		if err != nil {
			msg += ": " + err.Error()
		}
	}
	seen := map[string]bool{}
	out := make([]string, 0, len(ips))
	for _, ip := range ips {
		if !seen[ip] {
			seen[ip] = true
			out = append(out, ip)
		}
	}
	webOK(w, map[string]any{"name": name, "ok": len(out) > 0, "ips": out, "message": msg})
}
