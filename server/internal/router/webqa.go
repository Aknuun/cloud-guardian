package router

import (
	"net/http"
	"strings"
	"time"

	"github.com/aknuun/cg-server/internal/cf"
)

// webQaSearch finds every DNS record (all CF accounts) whose value equals the given IP.
func (r *Router) webQaSearch(w http.ResponseWriter, req *http.Request, uid int64) {
	ip := strings.TrimSpace(req.URL.Query().Get("ip"))
	if ip == "" {
		webErr(w, http.StatusBadRequest, "آی‌پی لازم است")
		return
	}
	var hits []qaHit
	for i, a := range r.cfAccs() {
		cli := cf.New(a.Token)
		zs, err := cli.Zones()
		if err != nil {
			continue
		}
		for _, z := range zs {
			if z.Status != "active" {
				continue
			}
			recs, err := cli.Records(z.ID)
			if err != nil {
				continue
			}
			for k := range recs {
				if strings.TrimSpace(recs[k].Content) == ip {
					hits = append(hits, qaHit{Acc: i, ZID: z.ID, ZName: z.Name,
						RID: recs[k].ID, Name: recs[k].Name, Type: recs[k].Type})
				}
			}
		}
		if len(hits) >= 200 {
			break
		}
	}
	if hits == nil {
		hits = []qaHit{}
	}
	webOK(w, map[string]any{"ip": ip, "hits": hits})
}

type webQaBulkIn struct {
	Hits []qaHit `json:"hits"`
}

// webQaFav stars every given record.
func (r *Router) webQaFav(w http.ResponseWriter, req *http.Request, uid int64) {
	var in webQaBulkIn
	if !webDecode(w, req, &in) {
		return
	}
	favs := r.favs()
	added := 0
	for _, h := range in.Hits {
		exists := false
		for _, f := range favs {
			if strvVal(f["rid"]) == h.RID {
				exists = true
				break
			}
		}
		if exists {
			continue
		}
		favs = append(favs, map[string]any{"acc": h.Acc, "zid": h.ZID, "rid": h.RID,
			"name": cfRecLabel(h.Name, h.Type, h.ZName)})
		added++
	}
	r.Store.PutJSON("favs", toAnyList(favs), 0)
	webOK(w, map[string]any{"added": added})
}

// webQaDel deletes every given record.
func (r *Router) webQaDel(w http.ResponseWriter, req *http.Request, uid int64) {
	var in webQaBulkIn
	if !webDecode(w, req, &in) {
		return
	}
	ok, fail := 0, 0
	for _, h := range in.Hits {
		cli, found := r.cfClientFor(h.Acc)
		if !found || cli.DelRecord(h.ZID, h.RID) != nil {
			fail++
			continue
		}
		ok++
	}
	webOK(w, map[string]any{"ok": ok, "fail": fail})
}

// webQaAdd creates a record in the chosen zone (quick "add sub for this IP").
func (r *Router) webQaAdd(w http.ResponseWriter, req *http.Request, uid int64) {
	var in struct {
		Acc  int    `json:"acc"`
		ZID  string `json:"zid"`
		Name string `json:"name"`
		IP   string `json:"ip"`
	}
	if !webDecode(w, req, &in) {
		return
	}
	if in.Name == "" || in.IP == "" {
		webErr(w, http.StatusBadRequest, "نام و آی‌پی لازم است")
		return
	}
	cli, ok := r.cfClientFor(in.Acc)
	if !ok {
		webErr(w, http.StatusNotFound, "اکانت پیدا نشد")
		return
	}
	rec, err := cli.AddRecord(in.ZID, "A", in.Name, in.IP, 1, false, nil)
	if err != nil {
		webErr(w, http.StatusBadGateway, "افزودن رکورد ناموفق: "+err.Error())
		return
	}
	webOK(w, map[string]any{"ok": true, "id": rec.ID})
}

// ---------- quota ----------

func (r *Router) webToolsQuota(w http.ResponseWriter, req *http.Request, uid int64) {
	c := r.quotaCfg()
	webOK(w, map[string]any{
		"limit":      c.Limit,
		"autoStop":   c.AutoStop,
		"alerts":     c.Alerts,
		"paused":     c.CronPausedUntil > time.Now().UnixMilli(),
		"pausedTill": c.CronPausedUntil,
		"usage":      r.quotaUsageLines(c.Limit),
	})
}

func (r *Router) webQuotaCfg(w http.ResponseWriter, req *http.Request, uid int64) {
	var in struct {
		Limit    *int  `json:"limit"`
		AutoStop *bool `json:"autoStop"`
		Alerts   *bool `json:"alerts"`
		Pause    *bool `json:"pause"`
	}
	if !webDecode(w, req, &in) {
		return
	}
	c := r.quotaCfg()
	if in.Limit != nil {
		if *in.Limit < 1000 {
			webErr(w, http.StatusBadRequest, "حداقل سقف ۱۰۰۰ است")
			return
		}
		c.Limit = *in.Limit
	}
	if in.AutoStop != nil {
		c.AutoStop = *in.AutoStop
	}
	if in.Alerts != nil {
		c.Alerts = *in.Alerts
	}
	if in.Pause != nil {
		if *in.Pause {
			c.CronPausedUntil = time.Now().Add(24 * time.Hour).UnixMilli()
		} else {
			c.CronPausedUntil = 0
		}
	}
	r.quotaSave(c)
	webOK(w, map[string]any{"ok": true})
}
