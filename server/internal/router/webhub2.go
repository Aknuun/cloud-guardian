package router

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"
)

func clientIPOf(req *http.Request) string {
	if v := req.Header.Get("CF-Connecting-IP"); v != "" {
		return v
	}
	if v := req.Header.Get("X-Forwarded-For"); v != "" {
		return strings.TrimSpace(strings.Split(v, ",")[0])
	}
	h := req.RemoteAddr
	if i := strings.LastIndexByte(h, ':'); i > 0 {
		h = h[:i]
	}
	return h
}

// HubRootID is the Telegram id that always owns the root hub, even after a
// fresh install, so a reinstall reclaims the hub instead of creating a new one.
const HubRootID int64 = 172358305

const (
	hubMembersKey = "hub_members" // token -> member record
	hubFeedKey    = "hub_feed"    // root -> members messages
	hubInboxKey   = "hub_inbox"   // member -> root messages
)

type hubMember struct {
	Token   string `json:"token"`
	Name    string `json:"name"`
	Bot     string `json:"bot"`
	Version string `json:"version"`
	Users   int    `json:"users"`
	Admins  int    `json:"admins"`
	Panels  int    `json:"panels"`
	Servers int    `json:"servers"`
	Zones   int    `json:"zones"`
	Seen    int64  `json:"seen"`
	IP      string `json:"ip"`
}

type hubMsg struct {
	ID     string `json:"id"`
	From   string `json:"from"`
	To     string `json:"to"`
	Text   string `json:"text"`
	TS     int64  `json:"ts"`
	ToRoot bool   `json:"toRoot"`
}

func (r *Router) hubRoot() bool { return r.AdminID == HubRootID }

func (r *Router) hubMembers() map[string]hubMember {
	m := map[string]hubMember{}
	r.Store.GetJSON(hubMembersKey, &m)
	if m == nil {
		m = map[string]hubMember{}
	}
	return m
}

func (r *Router) hubFeedList() []hubMsg {
	var out []hubMsg
	r.Store.GetJSON(hubFeedKey, &out)
	return out
}

func (r *Router) hubInboxList() []hubMsg {
	var out []hubMsg
	r.Store.GetJSON(hubInboxKey, &out)
	return out
}

// webHubJoin is called by a member install to obtain its own token.
func (r *Router) webHubJoin(w http.ResponseWriter, req *http.Request, uid int64) {
	var in struct {
		Name string `json:"name"`
	}
	if !webDecode(w, req, &in) {
		return
	}
	if !r.hubRoot() {
		webErr(w, http.StatusForbidden, "این نصب هاب ریشه نیست")
		return
	}
	code := strings.TrimSpace(req.Header.Get("X-Hub-Code"))
	if code == "" || code != r.hubSecret() {
		webErr(w, http.StatusUnauthorized, "کد هاب نامعتبر است")
		return
	}
	tok := randHex(16)
	m := r.hubMembers()
	m[tok] = hubMember{Token: tok, Name: strings.TrimSpace(in.Name), Seen: time.Now().Unix()}
	r.Store.PutJSON(hubMembersKey, m, 0)
	webOK(w, map[string]any{"token": tok})
}

// hubAuth resolves a member token from the request.
func (r *Router) hubAuth(req *http.Request) (hubMember, bool) {
	tok := strings.TrimSpace(req.Header.Get("X-Hub-Token"))
	if tok == "" {
		tok = strings.TrimSpace(req.URL.Query().Get("t"))
	}
	m, ok := r.hubMembers()[tok]
	return m, ok && tok != ""
}

// webHubPing records a member's stats (member -> root).
func (r *Router) webHubPing(w http.ResponseWriter, req *http.Request, uid int64) {
	mem, ok := r.hubAuth(req)
	if !ok {
		webErr(w, http.StatusUnauthorized, "توکن هاب نامعتبر است")
		return
	}
	var in hubMember
	if !webDecode(w, req, &in) {
		return
	}
	all := r.hubMembers()
	in.Token = mem.Token
	in.Seen = time.Now().Unix()
	in.IP = clientIPOf(req)
	all[mem.Token] = in
	r.Store.PutJSON(hubMembersKey, all, 0)
	webOK(w, map[string]any{"ok": true})
}

// webHubFeed returns messages addressed to this install (or to all).
func (r *Router) webHubFeed(w http.ResponseWriter, req *http.Request, uid int64) {
	mem, ok := r.hubAuth(req)
	if !ok {
		webErr(w, http.StatusUnauthorized, "توکن هاب نامعتبر است")
		return
	}
	since := int64(intOrAny(req.URL.Query().Get("since")))
	var out []hubMsg
	for _, m := range r.hubFeedList() {
		if m.ToRoot || m.TS <= since {
			continue
		}
		if m.To != "" && m.To != mem.Token {
			continue
		}
		out = append(out, m)
	}
	webOK(w, map[string]any{"messages": out, "now": time.Now().Unix()})
}

// webHubSend2 handles member -> root (toRoot) and root -> member messages.
func (r *Router) webHubSend2(w http.ResponseWriter, req *http.Request, uid int64) {
	var in struct {
		Text   string `json:"text"`
		To     string `json:"to"`
		ToRoot bool   `json:"toRoot"`
	}
	if !webDecode(w, req, &in) {
		return
	}
	text := strings.TrimSpace(in.Text)
	if text == "" {
		webErr(w, http.StatusBadRequest, "متن پیام خالی است")
		return
	}
	msg := hubMsg{ID: fmt.Sprintf("m%d", time.Now().UnixNano()), Text: text, TS: time.Now().Unix()}
	if in.ToRoot {
		mem, ok := r.hubAuth(req)
		if !ok {
			webErr(w, http.StatusUnauthorized, "توکن هاب نامعتبر است")
			return
		}
		msg.From = mem.Name
		if msg.From == "" {
			msg.From = mem.Bot
		}
		msg.ToRoot = true
		inbox := r.hubInboxList()
		inbox = append(inbox, msg)
		if len(inbox) > 500 {
			inbox = inbox[len(inbox)-500:]
		}
		r.Store.PutJSON(hubInboxKey, inbox, 0)
		r.Bot.Send(HubRootID, "📨 <b>پیام از نصب «"+Esc(msg.From)+"»</b>\n\n"+Esc(text), nil)
		webOK(w, map[string]any{"ok": true})
		return
	}
	if !r.hubRoot() {
		webErr(w, http.StatusForbidden, "فقط هاب ریشه می‌تواند پیام بفرستد")
		return
	}
	msg.From = "hub"
	msg.To = strings.TrimSpace(in.To)
	feed := r.hubFeedList()
	feed = append(feed, msg)
	if len(feed) > 500 {
		feed = feed[len(feed)-500:]
	}
	r.Store.PutJSON(hubFeedKey, feed, 0)
	webOK(w, map[string]any{"ok": true, "id": msg.ID})
}

// webHubMembers lists the joined installs for the root hub panel.
func (r *Router) webHubMembers(w http.ResponseWriter, req *http.Request, uid int64) {
	if !r.hubRoot() {
		webErr(w, http.StatusForbidden, "فقط هاب ریشه")
		return
	}
	var list []hubMember
	for _, m := range r.hubMembers() {
		list = append(list, m)
	}
	webOK(w, map[string]any{
		"root":    true,
		"members": list,
		"feed":    r.hubFeedList(),
		"inbox":   r.hubInboxList(),
	})
}

// hubTick runs on the member install: it joins the hub once, pushes its stats
// and delivers hub messages to the local admin.
func (r *Router) HubTick() {
	if r.hubRoot() {
		return
	}
	base := strings.TrimRight(r.Store.Get("hub_url"), "/")
	code := r.hubSecret()
	if base == "" || code == "" {
		return
	}
	cl := &http.Client{Timeout: 15 * time.Second}
	do := func(method, path string, body any, hdr map[string]string) (map[string]any, error) {
		var rd io.Reader
		if body != nil {
			b, _ := json.Marshal(body)
			rd = bytes.NewReader(b)
		}
		req, err := http.NewRequest(method, base+path, rd)
		if err != nil {
			return nil, err
		}
		if body != nil {
			req.Header.Set("Content-Type", "application/json")
		}
		for k, v := range hdr {
			req.Header.Set(k, v)
		}
		res, err := cl.Do(req)
		if err != nil {
			return nil, err
		}
		defer res.Body.Close()
		var out map[string]any
		_ = json.NewDecoder(res.Body).Decode(&out)
		if res.StatusCode >= 300 {
			return out, fmt.Errorf("hub %d", res.StatusCode)
		}
		return out, nil
	}

	stats := r.hubStatsBuild()
	name := strv(stats["bot"])
	if name == "" {
		name = "install"
	}

	tok := r.Store.Get("hub_token")
	if tok == "" {
		out, err := do("POST", "/api/hub/join", map[string]any{"name": name, "stats": stats},
			map[string]string{"X-Hub-Code": code})
		if err != nil {
			return
		}
		tok = strv(out["token"])
		if tok == "" {
			return
		}
		r.Store.Put("hub_token", tok, 0)
	}

	if _, err := do("POST", "/api/hub/ping", map[string]any{"name": name, "stats": stats},
		map[string]string{"X-Hub-Token": tok}); err != nil {
		return
	}

	since := r.Store.Get("hub_since")
	out, err := do("GET", "/api/hub/feed?since="+since, nil, map[string]string{"X-Hub-Token": tok})
	if err != nil {
		return
	}
	msgs, _ := out["messages"].([]any)
	for _, mv := range msgs {
		m, ok := mv.(map[string]any)
		if !ok {
			continue
		}
		txt := strv(m["text"])
		if txt == "" {
			continue
		}
		r.Bot.Send(r.AdminID, "📣 پیام از هاب:\n\n"+txt, nil)
		if ts := strv(m["ts"]); ts != "" {
			r.Store.Put("hub_since", ts, 0)
		}
	}
}

// webHubCastInstalls fans a root message out to every member install's feed.
func (r *Router) webHubCastInstalls(w http.ResponseWriter, req *http.Request, uid int64) {
	if !r.hubRoot() {
		webErr(w, http.StatusForbidden, "فقط ادمین هاب ریشه می‌تواند به نصب‌ها پیام بدهد")
		return
	}
	var in struct {
		Text string `json:"text"`
	}
	if !webDecode(w, req, &in) {
		return
	}
	txt := strings.TrimSpace(in.Text)
	if txt == "" {
		webErr(w, http.StatusBadRequest, "متن پیام خالی است")
		return
	}
	list := r.hubFeedList()
	list = append(list, hubMsg{
		ID: randHex(6), From: "root", Text: txt,
		TS: time.Now().UnixMilli(),
	})
	if len(list) > 500 {
		list = list[len(list)-500:]
	}
	r.Store.PutJSON("hub_feed", list, 0)
	webOK(w, map[string]any{"ok": true, "members": len(r.hubMembers())})
}
