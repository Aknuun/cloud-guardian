package router

import (
	"net/http"
	"time"
)

// Audit for destructive Cloudflare actions. Only what a user can break or lose
// is recorded: record deletes, bulk deletes, cache purge and zone pause.
type cfAuditEntry struct {
	TS     int64  `json:"ts"`
	UID    int64  `json:"uid"`
	Acc    int    `json:"acc"`
	Zid    string `json:"zid"`
	Zone   string `json:"zone"`
	Op     string `json:"op"`
	Detail string `json:"detail"`
}

func (r *Router) cfAuditAdd(uid int64, acc int, zid, zone, op, detail string) {
	var lst []cfAuditEntry
	r.Store.GetJSON("cf_audit", &lst)
	lst = append([]cfAuditEntry{{
		TS: time.Now().UnixMilli(), UID: uid, Acc: acc, Zid: zid,
		Zone: zone, Op: op, Detail: detail,
	}}, lst...)
	if len(lst) > 200 {
		lst = lst[:200]
	}
	r.Store.PutJSON("cf_audit", lst, 0)
}

func (r *Router) webCfAudit(w http.ResponseWriter, req *http.Request, uid int64) {
	var lst []cfAuditEntry
	r.Store.GetJSON("cf_audit", &lst)
	zid := req.URL.Query().Get("zid")
	out := make([]cfAuditEntry, 0, len(lst))
	for _, e := range lst {
		if zid != "" && e.Zid != zid {
			continue
		}
		out = append(out, e)
		if len(out) >= 50 {
			break
		}
	}
	webOK(w, map[string]any{"items": out})
}
