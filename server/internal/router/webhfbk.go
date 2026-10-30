package router

import (
	"fmt"
	"net/http"
	"time"

	"github.com/aknuun/cg-server/internal/hostfilter"
)

func (r *Router) webHfBk(w http.ResponseWriter, req *http.Request, uid int64) {
	list := r.hfBackupsLoad()
	if req.Method == http.MethodGet {
		out := make([]map[string]any, 0, len(list))
		for _, b := range list {
			out = append(out, map[string]any{"id": b.ID, "ts": b.TS, "label": b.Label, "count": b.Count})
		}
		webOK(w, map[string]any{"items": out})
		return
	}
	var in struct {
		Op string `json:"op"`
		ID string `json:"id"`
	}
	if !webDecode(w, req, &in) {
		return
	}
	switch in.Op {
	case "new":
		var hosts []map[string]any
		for pi, p := range r.panels() {
			c := p.Client()
			tok, err := c.Login()
			if err != nil {
				continue
			}
			lst, err := c.Hosts(tok)
			if err != nil {
				continue
			}
			for _, hv := range lst {
				h, ok := hv.(map[string]any)
				if !ok {
					continue
				}
				hosts = append(hosts, map[string]any{
					"panel": pi, "id": strv(h["id"]), "remark": strv(h["remark"]),
					"address": toList(h["address"]), "sni": toList(h["sni"]), "host": toList(h["host"]),
				})
			}
		}
		keep := hostfilter.GetCfg(r.Store).BackupKeep
		if keep < 1 {
			keep = 5
		}
		entry := hfBackup{ID: newBkID(), TS: time.Now().UTC().Format(time.RFC3339), Label: "دستی", Count: len(hosts), Hosts: hosts}
		next := append([]hfBackup{entry}, list...)
		if len(next) > keep {
			next = next[:keep]
		}
		r.hfBackupsStore(next)
		webOK(w, map[string]any{"ok": true, "count": len(hosts)})
	case "del":
		if in.ID == "" {
			webErr(w, http.StatusBadRequest, "شناسه لازم است")
			return
		}
		out := make([]hfBackup, 0, len(list))
		for _, b := range list {
			if b.ID != in.ID {
				out = append(out, b)
			}
		}
		r.hfBackupsStore(out)
		webOK(w, map[string]any{"ok": true})
	case "restore":
		var entry *hfBackup
		for _, b := range list {
			if b.ID == in.ID {
				bb := b
				entry = &bb
				break
			}
		}
		if entry == nil {
			webErr(w, http.StatusNotFound, "بکاپ پیدا نشد")
			return
		}
		panels := r.panels()
		byPanel := map[int][]map[string]any{}
		for _, snap := range entry.Hosts {
			pi := intOrAny(snap["panel"])
			byPanel[pi] = append(byPanel[pi], snap)
		}
		okN, fail := 0, 0
		for pi, snaps := range byPanel {
			if pi < 0 || pi >= len(panels) {
				fail += len(snaps)
				continue
			}
			c := panels[pi].Client()
			tok, err := c.Login()
			if err != nil {
				fail += len(snaps)
				continue
			}
			lst, err := c.Hosts(tok)
			if err != nil {
				fail += len(snaps)
				continue
			}
			for _, snap := range snaps {
				var host map[string]any
				for _, hv := range lst {
					h, _ := hv.(map[string]any)
					if strv(h["id"]) == strv(snap["id"]) {
						host = h
						break
					}
				}
				if host == nil {
					fail++
					continue
				}
				for _, f := range []string{"address", "sni", "host"} {
					if arr, ok := snap[f].([]any); ok {
						host[f] = arr
					}
				}
				if err := c.PutHost(tok, strv(host["id"]), host); err != nil {
					fail++
				} else {
					okN++
				}
			}
		}
		webOK(w, map[string]any{"ok": okN, "fail": fail,
			"message": fmt.Sprintf("موفق: %d — ناموفق: %d", okN, fail)})
	default:
		webErr(w, http.StatusBadRequest, "عملیات نامعتبر")
	}
}
