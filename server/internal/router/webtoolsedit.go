package router

import (
	"encoding/json"
	"fmt"
	"math/rand"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/aknuun/cg-server/internal/jalali"
)

// Write endpoints for the browser panel's tool pages (servers + reminders).

type webSrvIn struct {
	ID     string `json:"id"`
	Name   string `json:"name"`
	Host   string `json:"host"`
	User   string `json:"user"`
	Port   int    `json:"port"`
	Secret string `json:"secret"`
	Note   string `json:"note"`
}

func webDecode(w http.ResponseWriter, req *http.Request, out any) bool {
	if req.Method != http.MethodPost {
		webErr(w, http.StatusMethodNotAllowed, "method not allowed")
		return false
	}
	if err := json.NewDecoder(http.MaxBytesReader(w, req.Body, 1<<20)).Decode(out); err != nil {
		webErr(w, http.StatusBadRequest, "درخواست نامعتبر است")
		return false
	}
	return true
}

// webSplitHost accepts "host", "host:2222" or a separate port.
func webSplitHost(raw string, port int) (string, int) {
	raw = strings.TrimSpace(raw)
	if i := strings.LastIndex(raw, ":"); i > 0 && !strings.Contains(raw[i+1:], ":") {
		if p, err := strconv.Atoi(raw[i+1:]); err == nil && p > 0 && p < 65536 {
			return raw[:i], p
		}
	}
	if port <= 0 || port > 65535 {
		port = 22
	}
	return raw, port
}

func (r *Router) webToolsSrvAdd(w http.ResponseWriter, req *http.Request, uid int64) {
	var in webSrvIn
	if !webDecode(w, req, &in) {
		return
	}
	host, port := webSplitHost(in.Host, in.Port)
	if host == "" || strings.ContainsAny(host, " \t") {
		webErr(w, http.StatusBadRequest, "هاست معتبر نیست")
		return
	}
	user := orDefault(strings.TrimSpace(in.User), "root")
	secret := strings.TrimSpace(in.Secret)
	if secret == "" {
		// try the saved passwords one by one, like the bot does
		for _, p := range r.srvPwList() {
			if s := strings.TrimSpace(strv(p)); s != "" && webSrvPing(host, user, port, s) {
				secret = s
				break
			}
		}
		if secret == "" {
			webErr(w, http.StatusBadRequest,
				"اتصال SSH برقرار نشد: هیچ‌کدام از رمزهای ذخیره‌شده جواب نداد. رمز را وارد کن.")
			return
		}
	} else if !webSrvPing(host, user, port, secret) {
		webErr(w, http.StatusBadRequest, "اتصال SSH با این رمز/کلید برقرار نشد؛ مقدار را بررسی کن.")
		return
	}
	auth := "pass"
	if strings.HasPrefix(secret, "-----BEGIN") {
		auth = "key"
	}
	name := strings.TrimSpace(in.Name)
	if name == "" {
		name = host
	}
	rec := srvRec(fmt.Sprintf("s%d%d", time.Now().UnixNano(), rand.Intn(900)+100),
		name, host, user, port, auth, secret, strings.TrimSpace(in.Note))
	r.srvSave(append(r.srvList(), rec))
	webOK(w, map[string]any{"id": rec["id"]})
}

func (r *Router) webToolsSrvEdit(w http.ResponseWriter, req *http.Request, uid int64) {
	var in webSrvIn
	if !webDecode(w, req, &in) {
		return
	}
	i, cur := r.srvGet(strings.TrimSpace(in.ID))
	if cur == nil {
		webErr(w, http.StatusNotFound, "سرور پیدا نشد")
		return
	}
	host, port := webSplitHost(in.Host, in.Port)
	if host == "" || strings.ContainsAny(host, " \t") {
		webErr(w, http.StatusBadRequest, "هاست معتبر نیست")
		return
	}
	lst := r.srvList()
	name := strings.TrimSpace(in.Name)
	if name == "" {
		name = host
	}
	auth := strv(cur["auth"])
	secret := strv(cur["secret"])
	if s := strings.TrimSpace(in.Secret); s != "" {
		secret = s
		auth = "pass"
		if strings.HasPrefix(s, "-----BEGIN") {
			auth = "key"
		}
	}
	lst[i] = srvRec(strv(cur["id"]), name, host, orDefault(strings.TrimSpace(in.User), "root"),
		port, auth, secret, strings.TrimSpace(in.Note))
	r.srvSave(lst)
	webOK(w, map[string]any{"id": strv(cur["id"])})
}

func (r *Router) webToolsSrvDel(w http.ResponseWriter, req *http.Request, uid int64) {
	var in struct {
		ID string `json:"id"`
	}
	if !webDecode(w, req, &in) {
		return
	}
	var out []map[string]any
	for _, s := range r.srvList() {
		if strv(s["id"]) != in.ID {
			out = append(out, s)
		}
	}
	r.srvSave(out)
	webOK(w, map[string]any{"ok": true})
}

type webRemIn struct {
	ID     string `json:"id"`
	Text   string `json:"text"`
	When   string `json:"when"` // jalali "1404/08/07 14:30" or empty = when back online
	Server string `json:"server"`
	Note   string `json:"note"`
	Rel    int    `json:"rel"` // minutes from now (used when When is empty)
}

func (r *Router) webToolsRemAdd(w http.ResponseWriter, req *http.Request, uid int64) {
	var in webRemIn
	if !webDecode(w, req, &in) {
		return
	}
	text := strings.TrimSpace(in.Text)
	if text == "" {
		webErr(w, http.StatusBadRequest, "متن یادآور خالی است")
		return
	}
	var when int64
	if raw := strings.TrimSpace(in.When); raw != "" {
		ms, err := jalali.ParseDatetime(jalali.ParseFA(raw))
		if err != nil {
			webErr(w, http.StatusBadRequest, "تاریخ نامعتبر است (مثال: 1404/08/07 14:30)")
			return
		}
		if ms < time.Now().UnixMilli() {
			webErr(w, http.StatusBadRequest, "تاریخ گذشته است")
			return
		}
		when = ms
	}
	if when == 0 && in.Rel > 0 {
		if in.Rel > 60*24*365 {
			webErr(w, http.StatusBadRequest, "زمان نسبی بیش از حد بزرگ است")
			return
		}
		when = time.Now().Add(time.Duration(in.Rel) * time.Minute).UnixMilli()
	}
	rm := Reminder{ID: newID(), Text: text, When: when, Created: nowMS(), By: uid,
		Server: strings.TrimSpace(in.Server), Note: strings.TrimSpace(in.Note)}
	r.saveRems(append(r.rems(), rm))
	webOK(w, rm)
}

func (r *Router) webToolsRemDel(w http.ResponseWriter, req *http.Request, uid int64) {
	var in struct {
		ID string `json:"id"`
	}
	if !webDecode(w, req, &in) {
		return
	}
	var out []Reminder
	for _, x := range r.rems() {
		if x.ID != in.ID {
			out = append(out, x)
		}
	}
	if out == nil {
		out = []Reminder{}
	}
	r.saveRems(out)
	webOK(w, map[string]any{"ok": true})
}

func (r *Router) webToolsRemDone(w http.ResponseWriter, req *http.Request, uid int64) {
	var in struct {
		ID string `json:"id"`
	}
	if !webDecode(w, req, &in) {
		return
	}
	lst := r.rems()
	found := false
	for i := range lst {
		if lst[i].ID == in.ID {
			lst[i].Ack = true
			found = true
		}
	}
	if !found {
		webErr(w, http.StatusNotFound, "یادآور پیدا نشد")
		return
	}
	r.saveRems(lst)
	webOK(w, map[string]any{"ok": true})
}
