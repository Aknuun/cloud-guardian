package router

import (
	"net/http"
	"strings"

	"github.com/aknuun/cg-server/internal/cf"
	"github.com/aknuun/cg-server/internal/cfworkers"
)

func (r *Router) webFoState() map[string]any {
	c := r.foGet()
	wh := r.foWebhookURL()
	owner := "server"
	if c["worker_name"] != "" && strings.Contains(wh, c["worker_name"]) {
		owner = "worker"
	} else if wh != "" {
		owner = "other"
	}
	health := c["health_url"]
	pub := strings.TrimRight(c["public_url"], "/")
	probe := health
	if probe == "" && pub != "" {
		probe = pub + "/health"
	}
	up, code := foPing(probe)
	if !up && pub != "" && probe != pub+"/health" {
		if ok, c2 := foPing(pub + "/health"); ok {
			up, probe, code = true, pub+"/health", c2
		}
	}
	enabled := c["worker_name"] != "" && c["enabled"] != "0"
	return map[string]any{
		"configured": r.foConfigured(),
		"enabled":    enabled,
		"worker":     c["worker_name"],
		"url":        c["worker_url"],
		"account":    c["account_name"],
		"namespace":  c["fo_namespace_id"],
		"health":     probe,
		"publicUrl":  pub,
		"owner":      owner,
		"serverUp":   up,
		"probeCode":  code,
		"webhook":    wh,
	}
}

func (r *Router) webToolsFo(w http.ResponseWriter, req *http.Request, uid int64) {
	webOK(w, r.webFoState())
}

func (r *Router) webFoSetup(w http.ResponseWriter, req *http.Request, uid int64) {
	var in struct {
		Token string `json:"token"`
		Name  string `json:"name"`
	}
	if !webDecode(w, req, &in) {
		return
	}
	tok := strings.TrimSpace(in.Token)
	if tok == "" {
		webErr(w, http.StatusBadRequest, "توکن کلادفلر لازم است")
		return
	}
	name := strings.TrimSpace(in.Name)
	go func() {
		cfg, err := r.DeployFailover(tok, name)
		if err != nil {
			r.Bot.Send(uid, failMsg("ساخت ورکر", err), nil)
			return
		}
		r.Bot.Send(uid, "✅ ورکر پشتیبان ساخته شد: <code>"+Esc(cfg["worker_name"])+"</code>", nil)
	}()
	webOK(w, map[string]any{"started": true})
}

func (r *Router) webFoDisable(w http.ResponseWriter, req *http.Request, uid int64) {
	c := r.foGet()
	if c["worker_name"] == "" {
		webErr(w, http.StatusBadRequest, "فیلور تنظیم نشده")
		return
	}
	cl := cfworkers.New(c["cf_api_token"])
	if err := cl.SetSchedules(c["cf_account_id"], c["worker_name"], nil); err != nil {
		webErr(w, http.StatusBadGateway, "غیرفعال‌سازی ناموفق: "+err.Error())
		return
	}
	c["enabled"] = "0"
	r.foSet(c)
	r.SyncPush()
	webOK(w, map[string]any{"ok": true})
}

func (r *Router) webFoEnable(w http.ResponseWriter, req *http.Request, uid int64) {
	c := r.foGet()
	if c["worker_name"] == "" {
		webErr(w, http.StatusBadRequest, "فیلور تنظیم نشده")
		return
	}
	cl := cfworkers.New(c["cf_api_token"])
	if err := cl.SetSchedules(c["cf_account_id"], c["worker_name"], []string{"*/5 * * * *"}); err != nil {
		webErr(w, http.StatusBadGateway, "فعال‌سازی ناموفق: "+err.Error())
		return
	}
	c["enabled"] = "1"
	r.foSet(c)
	r.SyncPush()
	webOK(w, map[string]any{"ok": true})
}

func (r *Router) webFoDelete(w http.ResponseWriter, req *http.Request, uid int64) {
	c := r.foGet()
	if c["worker_name"] == "" {
		webErr(w, http.StatusBadRequest, "فیلور تنظیم نشده")
		return
	}
	cl := cfworkers.New(c["cf_api_token"])
	if c["worker_name"] != "" && c["cf_account_id"] != "" {
		_ = cl.DeleteScript(c["cf_account_id"], c["worker_name"])
	}
	if c["fo_namespace_id"] != "" && c["cf_account_id"] != "" {
		_ = cl.DeleteKV(c["cf_account_id"], c["fo_namespace_id"])
	}
	if c["health_dns_rec"] != "" && c["health_dns_zone"] != "" {
		_ = cf.New(c["cf_api_token"]).DelRecord(c["health_dns_zone"], c["health_dns_rec"])
	}
	r.Store.Delete(foCfgKey)
	r.foClear("cf_api_token", "cf_account_id", "account_name", "worker_name", "worker_url",
		"fo_namespace_id", "health_url", "public_url", "health_dns_zone", "health_dns_rec", "health_dns_name")
	webOK(w, map[string]any{"ok": true})
}

// webFoRefresh pushes a fresh credential snapshot to the worker KV.
func (r *Router) webFoRefresh(w http.ResponseWriter, req *http.Request, uid int64) {
	if !r.foConfigured() {
		webErr(w, http.StatusBadRequest, "فیلور تنظیم نشده")
		return
	}
	r.SyncPush()
	webOK(w, r.webFoState())
}
