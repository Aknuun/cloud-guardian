package router

import (
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"net"
	"net/http"
	"strconv"
	"strings"
	"sync"
	"time"
)

// Browser login: the panel asks the bot for approval instead of asking for a
// password. A code lives 2 minutes, is single-use, and only the admin it was
// sent to can approve it.

type webLoginReq struct {
	uid   int64
	exp   time.Time
	state int // 0 pending, 1 approved, 2 denied
}

var (
	webLoginMu   sync.Mutex
	webLoginReqs = map[string]*webLoginReq{}
	webLoginHits = map[string][]time.Time{}
)

const (
	webLoginTTL   = 2 * time.Minute
	webLoginSess  = 24 * time.Hour
	webLoginLimit = 5 // starts per IP per minute
)

func webClientIP(req *http.Request) string {
	if fwd := req.Header.Get("X-Forwarded-For"); fwd != "" {
		return strings.TrimSpace(strings.Split(fwd, ",")[0])
	}
	if real := req.Header.Get("X-Real-IP"); real != "" {
		return real
	}
	host, _, err := net.SplitHostPort(req.RemoteAddr)
	if err != nil {
		return req.RemoteAddr
	}
	return host
}

func webLoginAllow(ip string) bool {
	now := time.Now()
	webLoginMu.Lock()
	defer webLoginMu.Unlock()
	kept := webLoginHits[ip][:0]
	for _, t := range webLoginHits[ip] {
		if now.Sub(t) < time.Minute {
			kept = append(kept, t)
		}
	}
	if len(kept) >= webLoginLimit {
		webLoginHits[ip] = kept
		return false
	}
	webLoginHits[ip] = append(kept, now)
	return true
}

func (r *Router) webLoginStart(w http.ResponseWriter, req *http.Request) {
	if req.Method != http.MethodPost {
		webErr(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}
	if !webLoginAllow(webClientIP(req)) {
		webErr(w, http.StatusTooManyRequests, "درخواست زیاد است؛ یک دقیقه بعد تلاش کن")
		return
	}
	buf := make([]byte, 6)
	if _, err := rand.Read(buf); err != nil {
		webErr(w, http.StatusInternalServerError, "ساخت کد ناموفق بود")
		return
	}
	code := hex.EncodeToString(buf)
	now := time.Now()
	webLoginMu.Lock()
	for k, v := range webLoginReqs {
		if now.After(v.exp) {
			delete(webLoginReqs, k)
		}
	}
	webLoginReqs[code] = &webLoginReq{uid: r.AdminID, exp: now.Add(webLoginTTL)}
	webLoginMu.Unlock()

	kb := [][]Btn{{
		btnS("✅ تأیید ورود", "wblogin:ok:"+code, "success"),
		btnS("🚫 رد", "wblogin:no:"+code, "danger"),
	}}
	r.Bot.Send(r.AdminID,
		"🔐 <b>درخواست ورود به پنل وب</b>\n\n"+
			"کد: <code>"+strings.ToUpper(code[:6])+"</code>\n"+
			"اگر خودت درخواست دادی، تأیید کن. اعتبار: ۲ دقیقه.",
		kb)

	webOK(w, map[string]any{"code": code, "expires_in": int(webLoginTTL.Seconds())})
}

func (r *Router) webLoginState(w http.ResponseWriter, req *http.Request) {
	code := strings.TrimSpace(req.URL.Query().Get("c"))
	if code == "" {
		webErr(w, http.StatusBadRequest, "کد لازم است")
		return
	}
	webLoginMu.Lock()
	e := webLoginReqs[code]
	if e == nil {
		webLoginMu.Unlock()
		webOK(w, map[string]any{"status": "expired"})
		return
	}
	if e.state == 0 && time.Now().After(e.exp) {
		delete(webLoginReqs, code)
		webLoginMu.Unlock()
		webOK(w, map[string]any{"status": "expired"})
		return
	}
	switch e.state {
	case 1:
		uid := e.uid
		delete(webLoginReqs, code) // single use
		webLoginMu.Unlock()
		exp := time.Now().Add(webLoginSess).Unix()
		webOK(w, map[string]any{
			"status": "ok",
			"token":  r.webSignSession(uid, exp, r.webSessCreate(uid, clientIPOf(req), req.UserAgent(), exp)),
		})
	case 2:
		delete(webLoginReqs, code)
		webLoginMu.Unlock()
		webOK(w, map[string]any{"status": "denied"})
	default:
		webLoginMu.Unlock()
		webOK(w, map[string]any{"status": "pending"})
	}
}

// webLoginCallback handles wblogin:ok:<code> / wblogin:no:<code>.
func (r *Router) webLoginCallback(chat, mid int64, data string) {
	rest := strings.TrimPrefix(data, "wblogin:")
	action, code, ok := strings.Cut(rest, ":")
	if !ok || code == "" {
		return
	}
	webLoginMu.Lock()
	e := webLoginReqs[code]
	if e == nil || e.uid != chat || time.Now().After(e.exp) {
		delete(webLoginReqs, code)
		webLoginMu.Unlock()
		r.Show(chat, mid, "⌛️ این درخواست ورود منقضی شده است.", nil)
		return
	}
	if action == "ok" {
		e.state = 1
	} else {
		e.state = 2
	}
	webLoginMu.Unlock()

	verdict := "✅ ورود تأیید شد. تا ۲۴ ساعت فعال است."
	if action != "ok" {
		verdict = "🚫 درخواست ورود رد شد."
	}
	r.Show(chat, mid, "🔐 <b>ورود به پنل وب</b>\n\n"+verdict, nil)
}

// ---------- one-time link tickets ----------
// The bot menu opens the panel on the admin's own device, so asking for a
// Telegram approval there is pointless. The link carries a signed ticket that
// lives 10 minutes and works once; nothing else changes.

var (
	webTicketMu   sync.Mutex
	webTicketUsed = map[string]time.Time{}
)

func (r *Router) webMakeTicket() string {
	var buf [6]byte
	_, _ = rand.Read(buf[:])
	nonce := hex.EncodeToString(buf[:])
	exp := time.Now().Add(10 * time.Minute).Unix()
	msg := fmt.Sprintf("%d.%d.%s", r.AdminID, exp, nonce)
	m := hmac.New(sha256.New, []byte("cg-web-ticket"))
	m.Write(r.webSessionSecret())
	m.Write([]byte(msg))
	return msg + "." + hex.EncodeToString(m.Sum(nil))
}

func (r *Router) webCheckTicket(t string) bool {
	parts := strings.Split(t, ".")
	if len(parts) != 4 {
		return false
	}
	uid, err1 := strconv.ParseInt(parts[0], 10, 64)
	exp, err2 := strconv.ParseInt(parts[1], 10, 64)
	if err1 != nil || err2 != nil || uid != r.AdminID || exp < time.Now().Unix() {
		return false
	}
	msg := strings.Join(parts[:3], ".")
	m := hmac.New(sha256.New, []byte("cg-web-ticket"))
	m.Write(r.webSessionSecret())
	m.Write([]byte(msg))
	if !hmac.Equal([]byte(hex.EncodeToString(m.Sum(nil))), []byte(parts[3])) {
		return false
	}
	now := time.Now()
	webTicketMu.Lock()
	defer webTicketMu.Unlock()
	for k, v := range webTicketUsed {
		if now.After(v) {
			delete(webTicketUsed, k)
		}
	}
	if _, used := webTicketUsed[parts[2]]; used {
		return false
	}
	webTicketUsed[parts[2]] = time.Unix(exp, 0)
	return true
}

func (r *Router) webLoginTicket(w http.ResponseWriter, req *http.Request) {
	if req.Method != http.MethodPost {
		webErr(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}
	var in struct {
		K string `json:"k"`
	}
	if err := json.NewDecoder(http.MaxBytesReader(w, req.Body, 1<<16)).Decode(&in); err != nil {
		webErr(w, http.StatusBadRequest, "بدنهٔ درخواست نامعتبر است")
		return
	}
	if !r.webCheckTicket(strings.TrimSpace(in.K)) {
		webErr(w, http.StatusUnauthorized, "لینک ورود منقضی یا استفاده‌شده است")
		return
	}
	exp := time.Now().Add(webLoginSess).Unix()
	webOK(w, map[string]any{
		"token": r.webSignSession(r.AdminID, exp, r.webSessCreate(r.AdminID, clientIPOf(req), req.UserAgent(), exp)),
		"user":  map[string]any{"id": r.AdminID, "isMain": true},
	})
}
