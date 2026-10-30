package router

import (
	"fmt"
	"net/http"
	"strings"

	"github.com/aknuun/cg-server/internal/hostfilter"
)

// webArvIdx resolves account index + server index from the query string.
func (r *Router) webArvSrv(i, idx int) (map[string]any, int, bool) {
	list := r.arvAllServers(i, false)
	if idx < 0 || idx >= len(list) {
		return nil, 0, false
	}
	return list[idx], idx, true
}

// webArvIPCheck runs the IP cleanliness check on a server's public IP.
func (r *Router) webArvIPCheck(w http.ResponseWriter, req *http.Request, uid int64) {
	i := intOrAny(req.URL.Query().Get("i"))
	idx := intOrAny(req.URL.Query().Get("idx"))
	s, _, ok := r.webArvSrv(i, idx)
	if !ok {
		webErr(w, http.StatusNotFound, "سرور پیدا نشد")
		return
	}
	ip := firstPubIP(s)
	if ip == "" {
		webErr(w, http.StatusBadRequest, "این سرور IP عمومی ندارد")
		return
	}
	clean, verdict := hostfilter.CheckIPCleanliness(ip)
	webOK(w, map[string]any{"ip": ip, "clean": clean, "verdict": verdict})
}

// webArvPlans lists upgrade sizes for the server's region.
func (r *Router) webArvPlans(w http.ResponseWriter, req *http.Request, uid int64) {
	i := intOrAny(req.URL.Query().Get("i"))
	idx := intOrAny(req.URL.Query().Get("idx"))
	s, _, ok := r.webArvSrv(i, idx)
	if !ok {
		webErr(w, http.StatusNotFound, "سرور پیدا نشد")
		return
	}
	region := strv(s["region"])
	if region == "" {
		region = strv(s["region_id"])
	}
	cur := strv(s["flavorID"])
	if cur == "" {
		cur = strv(s["flavor_id"])
	}
	var out []map[string]any
	for _, z := range r.arvSizes(i, region) {
		out = append(out, map[string]any{
			"id": z.ID, "name": z.Name, "cpu": z.CPU, "ram": z.RAM,
			"disk": z.Disk, "price": z.Price, "current": z.ID == cur,
		})
	}
	webOK(w, map[string]any{
		"region": region, "current": cur, "sizes": out,
		"status": strv(s["status"]),
	})
}

// webArvUpgrade resizes a (stopped) server to another flavor.
func (r *Router) webArvUpgrade(w http.ResponseWriter, req *http.Request, uid int64) {
	var in struct {
		I    int    `json:"i"`
		Idx  int    `json:"idx"`
		Size string `json:"size"`
	}
	if !webDecode(w, req, &in) {
		return
	}
	if strings.TrimSpace(in.Size) == "" {
		webErr(w, http.StatusBadRequest, "پلن مقصد مشخص نشده")
		return
	}
	s, _, ok := r.webArvSrv(in.I, in.Idx)
	if !ok {
		webErr(w, http.StatusNotFound, "سرور پیدا نشد")
		return
	}
	region := strv(s["region"])
	if region == "" {
		region = strv(s["region_id"])
	}
	cli, okc := r.arvCli(in.I)
	if !okc {
		webErr(w, http.StatusNotFound, "اکانت پیدا نشد")
		return
	}
	body := map[string]any{"resize": map[string]any{"flavorRef": in.Size}}
	out, err := cli.Ecc(region, "POST", "/servers/"+strv(s["id"])+"/action", body)
	if err != nil {
		out, err = cli.Ecc(region, "POST", "/servers/"+strv(s["id"])+"/action", map[string]any{"resize": map[string]any{"flavor_id": in.Size}})
	}
	if err != nil {
		webErr(w, http.StatusBadGateway, "ارتقا ناموفق: "+hfErrFA(err))
		return
	}
	_ = out
	r.arvFlushIndex(in.I)
	webOK(w, map[string]any{"ok": true})
}

// webArvHuntStatus reports the running stock hunt for the signed-in admin.
func (r *Router) webArvHuntStatus(w http.ResponseWriter, req *http.Request, uid int64) {
	h := r.arvHuntRunning(uid)
	if h == nil {
		webOK(w, map[string]any{"running": false})
		return
	}
	webOK(w, map[string]any{
		"running": true, "round": h.round, "rounds": h.maxRounds,
		"created": h.created, "batch": h.batch, "region": h.region,
		"stage": h.stage, "summary": h.summary,
	})
}

// webArvHuntStart starts the stock hunt; output goes to Telegram.
func (r *Router) webArvHuntStart(w http.ResponseWriter, req *http.Request, uid int64) {
	var in struct {
		I      int    `json:"i"`
		Region string `json:"region"`
		Idx    int    `json:"idx"`
		Rounds int    `json:"rounds"`
		Batch  int    `json:"batch"`
	}
	if !webDecode(w, req, &in) {
		return
	}
	if in.Rounds < 1 || in.Rounds > 100 || in.Batch < 1 || in.Batch > 10 {
		webErr(w, http.StatusBadRequest, "تعداد دور یا سرور هر دور نامعتبر است")
		return
	}
	if r.arvHuntRunning(uid) != nil {
		webErr(w, http.StatusConflict, "یک شکار در حال اجراست")
		return
	}
	go r.arvHuntStart(uid, 0, in.I, in.Region, in.Idx, in.Rounds, in.Batch)
	webOK(w, map[string]any{"started": true})
}

// webArvHuntStop stops the running hunt.
func (r *Router) webArvHuntStop(w http.ResponseWriter, req *http.Request, uid int64) {
	if r.arvHuntRunning(uid) == nil {
		webErr(w, http.StatusBadRequest, "شکاری در حال اجرا نیست")
		return
	}
	r.arvHuntStop(uid)
	webOK(w, map[string]any{"ok": true})
}

// webArvServerInfo returns one server's raw fields for the web detail page.
func (r *Router) webArvServerInfo(w http.ResponseWriter, req *http.Request, uid int64) {
	i := intOrAny(req.URL.Query().Get("i"))
	idx := intOrAny(req.URL.Query().Get("idx"))
	s, _, ok := r.webArvSrv(i, idx)
	if !ok {
		webErr(w, http.StatusNotFound, "سرور پیدا نشد")
		return
	}
	webOK(w, map[string]any{
		"id": strv(s["id"]), "name": strv(s["name"]),
		"status": strv(s["status"]), "ip": firstPubIP(s),
		"region": strv(s["region"]), "flavor": strv(s["flavorID"]),
		"msg": fmt.Sprintf("سرور %s", strv(s["name"])),
	})
}
