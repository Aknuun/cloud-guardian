package router

import (
	"net/http"
	"strings"
	"time"
)

// hubOut is one recorded hub broadcast (to installs or to local users).
type hubOut struct {
	ID        string `json:"id"`
	Text      string `json:"text"`
	TS        int64  `json:"ts"`
	Target    string `json:"target"`    // installs | users
	Members   int    `json:"members"`   // installs at send time / queued users
	Delivered int    `json:"delivered"` // installs that fetched the feed afterwards
	Done      bool   `json:"done"`
	OK        int    `json:"ok"`
	Fail      int    `json:"fail"`
	Lang      string `json:"lang,omitempty"`
}

func (r *Router) hubOutList() []hubOut {
	var l []hubOut
	r.Store.GetJSON("hub_outbox", &l)
	return l
}

func (r *Router) hubOutAdd(o hubOut) {
	l := r.hubOutList()
	l = append(l, o)
	if len(l) > 300 {
		l = l[len(l)-300:]
	}
	r.Store.PutJSON("hub_outbox", l, 0)
}

func (r *Router) hubOutPatch(id string, fn func(*hubOut)) {
	l := r.hubOutList()
	for i := range l {
		if l[i].ID == id {
			fn(&l[i])
		}
	}
	r.Store.PutJSON("hub_outbox", l, 0)
}

// webHubHistory returns the outbox with live delivery counts.
func (r *Router) webHubHistory(w http.ResponseWriter, req *http.Request, uid int64) {
	l := r.hubOutList()
	members := r.hubMembers()
	seen := map[string]int64{}
	for _, m := range members {
		seen[m.Token] = m.Feed
	}
	for i := range l {
		if l[i].Target != "installs" {
			continue
		}
		n := 0
		for _, m := range members {
			if m.Feed >= l[i].TS {
				n++
			}
		}
		l[i].Delivered = n
	}
	gone := []map[string]any{}
	for _, m := range members {
		gone = append(gone, map[string]any{
			"name": m.Name, "bot": m.Bot, "version": m.Version,
			"feed": m.Feed, "seen": m.Seen, "users": m.Users,
			"servers": m.Servers, "zones": m.Zones, "ip": m.IP,
		})
	}
	webOK(w, map[string]any{"items": l, "installs": gone})
}

func (r *Router) webHubDel(w http.ResponseWriter, req *http.Request, uid int64) {
	var in struct {
		ID  string `json:"id"`
		All bool   `json:"all"`
	}
	if !webDecode(w, req, &in) {
		return
	}
	if in.All {
		r.Store.PutJSON("hub_outbox", []hubOut{}, 0)
		webOK(w, map[string]any{"ok": true})
		return
	}
	l := r.hubOutList()
	out := l[:0]
	for _, o := range l {
		if o.ID != strings.TrimSpace(in.ID) {
			out = append(out, o)
		}
	}
	r.Store.PutJSON("hub_outbox", out, 0)
	webOK(w, map[string]any{"ok": true})
}

func hubOutID() string { return randHex(6) }

func hubNow() int64 { return time.Now().UnixMilli() }
