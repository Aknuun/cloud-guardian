package router

import (
	"fmt"
	"net/http"
	"strings"

	"github.com/aknuun/cg-server/internal/hz"
	"github.com/aknuun/cg-server/internal/sshop"
)

// Hetzner accounts + SSH server operations for the web panel.

func (r *Router) webHzAccAdd(w http.ResponseWriter, req *http.Request, uid int64) {
	var in struct{ Name, Token string }
	if !webDecode(w, req, &in) {
		return
	}
	tok := strings.TrimSpace(in.Token)
	name := strings.TrimSpace(in.Name)
	if tok == "" {
		webErr(w, http.StatusBadRequest, "توکن لازم است")
		return
	}
	if name == "" {
		name = "hz"
	}
	if _, err := hz.New(tok).Call("/servers?per_page=1", "GET", nil); err != nil {
		webErr(w, http.StatusBadRequest, "توکن نامعتبر است: "+err.Error())
		return
	}
	accs := r.hzAccs()
	for _, a := range accs {
		if strv(a["name"]) == name {
			webErr(w, http.StatusBadRequest, "اکانتی با این نام هست")
			return
		}
	}
	accs = append(accs, map[string]any{"name": name, "token": tok})
	r.hzSaveAccs(accs)
	webOK(w, map[string]any{"name": name})
}

func (r *Router) webHzAccDel(w http.ResponseWriter, req *http.Request, uid int64) {
	var in struct{ Name string }
	if !webDecode(w, req, &in) {
		return
	}
	accs := r.hzAccs()
	out := accs[:0]
	found := false
	for _, a := range accs {
		if strv(a["name"]) == in.Name {
			found = true
			continue
		}
		out = append(out, a)
	}
	if !found {
		webErr(w, http.StatusNotFound, "اکانت پیدا نشد")
		return
	}
	r.hzSaveAccs(out)
	// indexes shift, so drop every cached server list
	for i := 0; i < 8; i++ {
		r.Store.Delete(fmt.Sprintf("hzsrv:%d", i))
	}
	webOK(w, map[string]any{"deleted": in.Name})
}

func (r *Router) webSrvGet(id string) map[string]any {
	_, s := r.srvGet(id)
	return s
}

func (r *Router) webSrvRun(s map[string]any, cmd string, secs int) sshop.Result {
	return sshop.RunCmd(strv(s["host"]), orDefault(strv(s["user"]), "root"),
		strv(s["secret"]), intOfAny(s["port"]), cmd, secs)
}

func (r *Router) webSrvMon(w http.ResponseWriter, req *http.Request, uid int64) {
	s := r.webSrvGet(req.URL.Query().Get("id"))
	if s == nil {
		webErr(w, http.StatusNotFound, "سرور پیدا نشد")
		return
	}
	st := sshop.Stats(strv(s["host"]), orDefault(strv(s["user"]), "root"),
		strv(s["secret"]), intOfAny(s["port"]))
	if e := strvVal(st["error"]); e != "" {
		webErr(w, http.StatusBadGateway, "وصل نشد: "+truncRune(e, 200))
		return
	}
	webOK(w, st)
}

func (r *Router) webSrvReboot(w http.ResponseWriter, req *http.Request, uid int64) {
	var in struct{ ID string }
	if !webDecode(w, req, &in) {
		return
	}
	s := r.webSrvGet(in.ID)
	if s == nil {
		webErr(w, http.StatusNotFound, "سرور پیدا نشد")
		return
	}
	// the SSH session usually dies with the reboot, so the error is expected
	r.webSrvRun(s, "reboot", 15)
	webOK(w, map[string]any{"ok": true})
}

func (r *Router) webSrvUpdate(w http.ResponseWriter, req *http.Request, uid int64) {
	var in struct{ ID string }
	if !webDecode(w, req, &in) {
		return
	}
	s := r.webSrvGet(in.ID)
	if s == nil {
		webErr(w, http.StatusNotFound, "سرور پیدا نشد")
		return
	}
	name := strv(s["name"])
	sv := s
	chat := r.AdminID
	go func() {
		cmd := "export DEBIAN_FRONTEND=noninteractive; apt-get update -y && apt-get upgrade -y"
		res := r.webSrvRun(sv, cmd, 300)
		if !res.Ok {
			r.Bot.Send(chat, fmt.Sprintf("❌ آپدیت «%s» ناموفق: <code>%s</code>", Esc(name), Esc(res.Err)), nil)
			return
		}
		out := res.Out
		if len(out) > 800 {
			out = out[len(out)-800:]
		}
		r.Bot.Send(chat, fmt.Sprintf("✅ آپدیت «%s» انجام شد.\n\n<pre>%s</pre>", Esc(name), Esc(out)), nil)
	}()
	webOK(w, map[string]any{"queued": true})
}
