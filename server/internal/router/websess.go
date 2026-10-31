package router

import (
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"net/http"
	"sort"
	"strings"
	"sync"
	"time"
)

// Server-side web sessions: a signed token is only valid while its row exists,
// so an admin can revoke one device or every device at once.

const (
	webSessKey     = "web_sessions"
	webSessMax     = 20
	webSessTouchAt = 5 * time.Minute
)

type webSess struct {
	ID      string `json:"id"`
	UID     int64  `json:"uid"`
	Created int64  `json:"created"`
	Exp     int64  `json:"exp"`
	Last    int64  `json:"last"`
	IP      string `json:"ip"`
	UA      string `json:"ua"`
	Device  string `json:"device"`
}

var webSessMu sync.Mutex

func (r *Router) webSessAll() map[string][]webSess {
	m := map[string][]webSess{}
	r.Store.GetJSON(webSessKey, &m)
	return m
}

func (r *Router) webSessSave(m map[string][]webSess) {
	r.Store.PutJSON(webSessKey, m, 0)
}

func webSessID() string {
	b := make([]byte, 12)
	if _, err := rand.Read(b); err != nil {
		return fmt.Sprintf("%x", time.Now().UnixNano())
	}
	return hex.EncodeToString(b)
}

// webDeviceLabel turns a User-Agent into something short and readable.
func webDeviceLabel(ua string) string {
	l := strings.ToLower(ua)
	os := ""
	switch {
	case strings.Contains(l, "iphone"):
		os = "iPhone"
	case strings.Contains(l, "ipad"):
		os = "iPad"
	case strings.Contains(l, "android"):
		os = "Android"
	case strings.Contains(l, "windows"):
		os = "Windows"
	case strings.Contains(l, "macintosh") || strings.Contains(l, "mac os"):
		os = "macOS"
	case strings.Contains(l, "linux"):
		os = "Linux"
	}
	br := ""
	switch {
	case strings.Contains(l, "edg/"):
		br = "Edge"
	case strings.Contains(l, "chrome") || strings.Contains(l, "crios"):
		br = "Chrome"
	case strings.Contains(l, "firefox") || strings.Contains(l, "fxios"):
		br = "Firefox"
	case strings.Contains(l, "safari"):
		br = "Safari"
	}
	switch {
	case os != "" && br != "":
		return os + " · " + br
	case os != "":
		return os
	case br != "":
		return br
	case strings.Contains(l, "telegram"):
		return "Telegram"
	}
	return "نامشخص"
}

func (r *Router) webSessCreate(uid int64, ip, ua string, exp int64) string {
	now := time.Now().Unix()
	s := webSess{
		ID: webSessID(), UID: uid, Created: now, Exp: exp, Last: now,
		IP: ip, UA: trimUA(ua), Device: webDeviceLabel(ua),
	}
	webSessMu.Lock()
	defer webSessMu.Unlock()
	all := r.webSessAll()
	key := fmt.Sprint(uid)
	lst := make([]webSess, 0, len(all[key])+1)
	for _, x := range all[key] {
		if x.Exp > now {
			lst = append(lst, x)
		}
	}
	lst = append([]webSess{s}, lst...)
	if len(lst) > webSessMax {
		lst = lst[:webSessMax]
	}
	all[key] = lst
	r.webSessSave(all)
	return s.ID
}

func trimUA(ua string) string {
	ua = strings.TrimSpace(ua)
	if len(ua) > 180 {
		return ua[:180]
	}
	return ua
}

// webSessValid checks the row and refreshes its last-seen stamp (throttled).
func (r *Router) webSessValid(uid int64, sid string) bool {
	now := time.Now().Unix()
	webSessMu.Lock()
	defer webSessMu.Unlock()
	all := r.webSessAll()
	key := fmt.Sprint(uid)
	lst, ok := all[key]
	if !ok {
		return false
	}
	touch := false
	for i := range lst {
		if lst[i].ID != sid {
			continue
		}
		if lst[i].Exp < now {
			return false
		}
		if now-lst[i].Last >= int64(webSessTouchAt.Seconds()) {
			lst[i].Last = now
			touch = true
		}
		if touch {
			all[key] = lst
			r.webSessSave(all)
		}
		return true
	}
	return false
}

func (r *Router) webSessRevoke(uid int64, sid string) {
	webSessMu.Lock()
	defer webSessMu.Unlock()
	all := r.webSessAll()
	key := fmt.Sprint(uid)
	lst, ok := all[key]
	if !ok {
		return
	}
	out := make([]webSess, 0, len(lst))
	for _, x := range lst {
		if x.ID != sid {
			out = append(out, x)
		}
	}
	all[key] = out
	r.webSessSave(all)
}

func (r *Router) webSessRevokeAll(uid int64) {
	webSessMu.Lock()
	defer webSessMu.Unlock()
	all := r.webSessAll()
	delete(all, fmt.Sprint(uid))
	r.webSessSave(all)
}

func (r *Router) webSessList(uid int64) []webSess {
	now := time.Now().Unix()
	webSessMu.Lock()
	defer webSessMu.Unlock()
	all := r.webSessAll()
	out := make([]webSess, 0)
	for _, x := range all[fmt.Sprint(uid)] {
		if x.Exp > now {
			out = append(out, x)
		}
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Last > out[j].Last })
	return out
}

func webReqSid(req *http.Request) string {
	tok := strings.TrimSpace(strings.TrimPrefix(req.Header.Get("Authorization"), "Bearer "))
	parts := strings.Split(tok, ".")
	if len(parts) != 4 {
		return ""
	}
	return parts[2]
}

// ---------- HTTP ----------

func (r *Router) webSessions(w http.ResponseWriter, req *http.Request, uid int64) {
	cur := webReqSid(req)
	lst := r.webSessList(uid)
	out := make([]map[string]any, 0, len(lst))
	for _, s := range lst {
		out = append(out, map[string]any{
			"id": s.ID, "created": s.Created, "last": s.Last, "exp": s.Exp,
			"ip": s.IP, "device": s.Device, "current": s.ID == cur,
		})
	}
	webOK(w, map[string]any{"sessions": out, "current": cur})
}

func (r *Router) webSessionsRevoke(w http.ResponseWriter, req *http.Request, uid int64) {
	var in struct {
		ID  string `json:"id"`
		All bool   `json:"all"`
	}
	if !webDecode(w, req, &in) {
		return
	}
	if in.All {
		r.webSessRevokeAll(uid)
		webOK(w, map[string]any{"revoked": "all"})
		return
	}
	if strings.TrimSpace(in.ID) == "" {
		webErr(w, http.StatusBadRequest, "شناسهٔ نشست لازم است")
		return
	}
	r.webSessRevoke(uid, in.ID)
	webOK(w, map[string]any{"revoked": in.ID})
}
