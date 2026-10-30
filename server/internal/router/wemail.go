package router

import (
	"fmt"
	"net/http"
	"strings"
)

// webToolsMail lists catch-all email settings per zone across every CF account.
func (r *Router) webToolsMail(w http.ResponseWriter, req *http.Request, uid int64) {
	sc := r.fmailScan(uid)
	fwd := asMapList(sc["fwd"])
	worker := asMapList(sc["worker"])
	if fwd == nil {
		fwd = []map[string]any{}
	}
	if worker == nil {
		worker = []map[string]any{}
	}
	webOK(w, map[string]any{"fwd": fwd, "worker": worker})
}

func (r *Router) webMailZone(w http.ResponseWriter, req *http.Request, uid int64) {
	acc := atoiSafe(req.URL.Query().Get("acc"))
	zid := strings.TrimSpace(req.URL.Query().Get("zid"))
	cli, ok := r.cfClientFor(acc)
	if !ok {
		webErr(w, http.StatusNotFound, "اکانت پیدا نشد")
		return
	}
	list, err := cli.EmailDests(cli.AccountID(zid))
	if err != nil {
		webErr(w, http.StatusBadGateway, err.Error())
		return
	}
	var verified, pending []map[string]any
	for _, d := range mailDestsSorted(list) {
		if b, _ := d["verified"].(bool); b {
			verified = append(verified, d)
		} else {
			pending = append(pending, d)
		}
	}
	if verified == nil {
		verified = []map[string]any{}
	}
	if pending == nil {
		pending = []map[string]any{}
	}
	webOK(w, map[string]any{"verified": verified, "pending": pending})
}

func (r *Router) webMailSet(w http.ResponseWriter, req *http.Request, uid int64) {
	var in struct {
		Acc  int    `json:"acc"`
		Zid  string `json:"zid"`
		Dest string `json:"dest"`
	}
	if !webDecode(w, req, &in) {
		return
	}
	dest := strings.TrimSpace(in.Dest)
	if dest == "" || !strings.Contains(dest, "@") {
		webErr(w, http.StatusBadRequest, "ایمیل مقصد نامعتبر است")
		return
	}
	cli, ok := r.cfClientFor(in.Acc)
	if !ok {
		webErr(w, http.StatusNotFound, "اکانت پیدا نشد")
		return
	}
	if err := cli.EmailCatchAllSet(in.Zid, map[string]any{
		"enabled": true,
		"actions": []any{map[string]any{"type": "forward", "value": []string{dest}}},
	}); err != nil {
		webErr(w, http.StatusBadGateway, err.Error())
		return
	}
	r.Store.Delete(fmt.Sprintf("fmail_scan:%d", uid))
	webOK(w, map[string]any{"ok": true})
}

func (r *Router) webMailOff(w http.ResponseWriter, req *http.Request, uid int64) {
	var in struct {
		Acc int    `json:"acc"`
		Zid string `json:"zid"`
	}
	if !webDecode(w, req, &in) {
		return
	}
	cli, ok := r.cfClientFor(in.Acc)
	if !ok {
		webErr(w, http.StatusNotFound, "اکانت پیدا نشد")
		return
	}
	if err := cli.EmailCatchAllSet(in.Zid, map[string]any{
		"enabled": false,
		"actions": []any{map[string]any{"type": "drop", "value": []string{}}},
	}); err != nil {
		webErr(w, http.StatusBadGateway, err.Error())
		return
	}
	r.Store.Delete(fmt.Sprintf("fmail_scan:%d", uid))
	webOK(w, map[string]any{"ok": true})
}

func (r *Router) webMailDestAdd(w http.ResponseWriter, req *http.Request, uid int64) {
	var in struct {
		Acc   int    `json:"acc"`
		Zid   string `json:"zid"`
		Email string `json:"email"`
	}
	if !webDecode(w, req, &in) {
		return
	}
	email := strings.TrimSpace(in.Email)
	if !strings.Contains(email, "@") {
		webErr(w, http.StatusBadRequest, "ایمیل نامعتبر است")
		return
	}
	cli, ok := r.cfClientFor(in.Acc)
	if !ok {
		webErr(w, http.StatusNotFound, "اکانت پیدا نشد")
		return
	}
	if err := cli.EmailDestAdd(cli.AccountID(in.Zid), email); err != nil {
		webErr(w, http.StatusBadGateway, err.Error())
		return
	}
	webOK(w, map[string]any{"ok": true, "email": email})
}
