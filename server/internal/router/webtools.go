package router

import (
	"net/http"
	"sort"

	"github.com/aknuun/cg-server/internal/ndmon"
)

// Read-only endpoints for the browser panel's tool pages.

func (r *Router) webToolsSrv(w http.ResponseWriter, req *http.Request, uid int64) {
	out := []map[string]any{}
	for _, s := range r.srvList() {
		out = append(out, map[string]any{
			"id":   s["id"],
			"name": s["name"],
			"host": s["host"],
			"user": s["user"],
			"port": s["port"],
			"note": s["note"],
			"key":  s["auth"] == "key",
		})
	}
	webOK(w, out)
}

func (r *Router) webToolsRem(w http.ResponseWriter, req *http.Request, uid int64) {
	list := r.rems()
	sort.Slice(list, func(i, j int) bool { return list[i].When < list[j].When })
	if list == nil {
		list = []Reminder{}
	}
	webOK(w, list)
}

func (r *Router) webToolsPg(w http.ResponseWriter, req *http.Request, uid int64) {
	panels := []map[string]any{}
	for i, p := range r.panels() {
		panels = append(panels, map[string]any{
			"idx":  i,
			"name": p.Name,
			"url":  p.URL,
			"user": p.Username,
		})
	}
	mons := []map[string]any{}
	for _, m := range ndmon.Monitors(r.Store) {
		mons = append(mons, map[string]any{
			"id":       m.ID,
			"name":     m.Label(),
			"panel":    m.PanelURL,
			"interval": m.IntervalMin,
			"enabled":  m.IsEnabled(),
		})
	}
	webOK(w, map[string]any{"panels": panels, "monitors": mons})
}
