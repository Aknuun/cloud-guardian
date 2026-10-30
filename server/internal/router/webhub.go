package router

import (
	"fmt"
	"net/http"
	"strings"
	"time"
)

// webToolsHub returns local install stats, hub settings and hub-wide installs.
func (r *Router) webToolsHub(w http.ResponseWriter, req *http.Request, uid int64) {
	s := r.hubStatsBuild()
	out := map[string]any{
		"hubUrl":   r.hubURL(),
		"annUrl":   r.announceURL(),
		"secret":   r.hubSecret() != "",
		"stats":    s,
		"root":     r.hubRoot(),
		"hubOk":    false,
		"installs": []any{},
	}
	if u := r.hubURL(); u != "" {
		var agg map[string]any
		if err := httpJSON("GET", u+"/installs/"+r.hubSecret(), nil, &agg, 8*time.Second); err == nil {
			if arr, ok := agg["installs"].([]any); ok {
				out["installs"] = arr
				out["hubOk"] = true
			}
		} else {
			out["hubErr"] = err.Error()
		}
	}
	webOK(w, out)
}

func (r *Router) webHubCfg(w http.ResponseWriter, req *http.Request, uid int64) {
	var in struct {
		HubURL string `json:"hubUrl"`
		AnnURL string `json:"annUrl"`
		Secret string `json:"secret"`
	}
	if !webDecode(w, req, &in) {
		return
	}
	if v := strings.TrimSpace(in.HubURL); v != "" {
		r.Store.Put("hub_url", strings.TrimRight(v, "/"), 0)
	} else {
		r.Store.Delete("hub_url")
	}
	if v := strings.TrimSpace(in.AnnURL); v != "" {
		r.Store.Put("announce_url", v, 0)
	} else {
		r.Store.Delete("announce_url")
	}
	if strings.TrimSpace(in.Secret) != "" {
		r.Store.Put("hub_secret", strings.TrimSpace(in.Secret), 0)
	}
	webOK(w, map[string]any{"ok": true})
}

// webHubAnn lists announcements from the configured source.
func (r *Router) webHubAnn(w http.ResponseWriter, req *http.Request, uid int64) {
	list, err := r.hubAnnFetch()
	if err != nil {
		webErr(w, http.StatusBadGateway, "دریافت اعلان‌ها ناموفق: "+err.Error())
		return
	}
	if list == nil {
		list = []announcement{}
	}
	webOK(w, map[string]any{"items": list, "src": r.announceURL()})
}

// webHubCast sends a broadcast message to every admin and known user.
func (r *Router) webHubCast(w http.ResponseWriter, req *http.Request, uid int64) {
	var in struct {
		Text string `json:"text"`
	}
	if !webDecode(w, req, &in) {
		return
	}
	msg := strings.TrimSpace(in.Text)
	if msg == "" {
		webErr(w, http.StatusBadRequest, "متن پیام خالی است")
		return
	}
	targets := map[int64]bool{}
	var order []int64
	for _, a := range r.Admins() {
		if !targets[a] {
			targets[a] = true
			order = append(order, a)
		}
	}
	for _, u := range r.users() {
		if !targets[u.ID] {
			targets[u.ID] = true
			order = append(order, u.ID)
		}
	}
	go func() {
		ok, fail := 0, 0
		for _, id := range order {
			if r.Bot.Send(id, "📢 "+msg, nil) != 0 {
				ok++
			} else {
				fail++
			}
			time.Sleep(40 * time.Millisecond)
		}
		r.Bot.Send(uid, fmt.Sprintf("✅ پیام همگانی: %d موفق · %d ناموفق", ok, fail), nil)
	}()
	oid := hubOutID()
	r.hubOutAdd(hubOut{ID: oid, Text: msg, TS: hubNow(), Target: "users", Members: len(order)})
	webOK(w, map[string]any{"queued": len(order), "id": oid})
}
