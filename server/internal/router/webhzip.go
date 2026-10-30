package router

import (
	"fmt"
	"net/http"
	"time"

	"github.com/aknuun/cg-server/internal/hz"
)

// webHzIPAdd creates fresh primary IPv4 addresses in the account.
func (r *Router) webHzIPAdd(w http.ResponseWriter, req *http.Request, uid int64) {
	var in struct {
		Acc   int    `json:"acc"`
		Loc   string `json:"loc"`
		Count int    `json:"count"`
	}
	if !webDecode(w, req, &in) {
		return
	}
	cli, err := r.hzWebCli(in.Acc)
	if err != nil {
		webErr(w, http.StatusBadRequest, err.Error())
		return
	}
	if in.Loc == "" {
		webErr(w, http.StatusBadRequest, "لوکیشن را انتخاب کن")
		return
	}
	if in.Count < 1 {
		in.Count = 1
	}
	if in.Count > 10 {
		webErr(w, http.StatusBadRequest, "حداکثر ۱۰ آی‌پی در هر بار")
		return
	}
	created := []string{}
	for i := 0; i < in.Count; i++ {
		d, err := cli.Call("/primary_ips", "POST", map[string]any{
			"type":        "ipv4",
			"name":        fmt.Sprintf("cg-%d", time.Now().UnixNano()%100000),
			"location":    in.Loc,
			"auto_delete": false,
		})
		if err != nil {
			if len(created) == 0 {
				webErr(w, http.StatusBadGateway, "ساخت آی‌پی ناموفق: "+err.Error())
				return
			}
			break
		}
		if p, _ := d["primary_ip"].(map[string]any); p != nil {
			created = append(created, strv(p["ip"]))
		}
	}
	webOK(w, map[string]any{"ok": true, "created": created})
}

var _ = hz.Client{}
