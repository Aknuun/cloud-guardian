package router

import (
	"net/http"
	"net/url"
	"strings"

	"github.com/aknuun/cg-server/internal/alerts"
	"github.com/aknuun/cg-server/internal/pg"
)

// webHookBase is the public origin used to build PasarGuard webhook URLs.
func (r *Router) webHookBase() string {
	// The mini app origin is the public HTTPS entry; public_url may point at a
	// loopback port PasarGuard cannot reach.
	if u, err := url.Parse(r.WebAppURL); err == nil && u.Host != "" {
		return u.Scheme + "://" + u.Host
	}
	return r.publicBase()
}

func (r *Router) webHookPanelCfg(cfg alerts.Cfg, pid string) *alerts.PanelCfg {
	pc := cfg.Panels[pid]
	if pc == nil {
		pc = &alerts.PanelCfg{}
		cfg.Panels[pid] = pc
	}
	if pc.Secret == "" {
		pc.Secret = randHex(12)
		alerts.SaveCfg(r.Store, cfg)
	}
	return pc
}

func (r *Router) webToolsHook(w http.ResponseWriter, req *http.Request, uid int64) {
	cfg := alerts.GetCfg(r.Store)
	base := r.webHookBase()
	var panels []map[string]any
	for i, p := range r.panels() {
		pc := cfg.Panels[p.URL]
		item := map[string]any{"idx": i, "name": pname(p), "url": p.URL, "set": pc != nil}
		if pc != nil {
			item["secret"] = pc.Secret
			item["dashpath"] = pc.Dashpath
			item["renewbot"] = pc.Renewbot
			item["renewbotAuto"] = pc.RenewbotAuto
			item["hook"] = base + "/pghook/" + pc.Secret
		}
		panels = append(panels, item)
	}
	webOK(w, map[string]any{
		"enabled": cfg.Enabled, "silentMe": cfg.SilentMe,
		"days": cfg.Days, "usage": cfg.Usage, "panels": panels,
	})
}

func (r *Router) webHookCfg(w http.ResponseWriter, req *http.Request, uid int64) {
	var in struct {
		Enabled  *bool     `json:"enabled"`
		SilentMe *bool     `json:"silentMe"`
		Days     []float64 `json:"days"`
		Usage    []float64 `json:"usage"`
	}
	if !webDecode(w, req, &in) {
		return
	}
	cfg := alerts.GetCfg(r.Store)
	if in.Enabled != nil {
		cfg.Enabled = *in.Enabled
	}
	if in.SilentMe != nil {
		cfg.SilentMe = *in.SilentMe
	}
	if len(in.Days) > 0 {
		cfg.Days = in.Days
	}
	if len(in.Usage) > 0 {
		cfg.Usage = in.Usage
	}
	alerts.SaveCfg(r.Store, cfg)
	webOK(w, map[string]any{"ok": true})
}

// webHookPanel updates one panel's webhook settings or rotates its secret.
func (r *Router) webHookPanel(w http.ResponseWriter, req *http.Request, uid int64) {
	var in struct {
		Idx        int    `json:"idx"`
		Dashpath   string `json:"dashpath"`
		Renewbot   string `json:"renewbot"`
		Auto       bool   `json:"renewbotAuto"`
		Regen      bool   `json:"regen"`
		TouchOnly  bool   `json:"touch"`
		ClearDash  bool   `json:"clearDash"`
		ClearRenew bool   `json:"clearRenew"`
	}
	if !webDecode(w, req, &in) {
		return
	}
	panels := r.panels()
	if in.Idx < 0 || in.Idx >= len(panels) {
		webErr(w, http.StatusNotFound, "پنل پیدا نشد")
		return
	}
	cfg := alerts.GetCfg(r.Store)
	pc := r.webHookPanelCfg(cfg, panels[in.Idx].URL)
	if in.Regen {
		pc.Secret = randHex(12)
	}
	if in.ClearDash {
		pc.Dashpath = ""
	} else if d := strings.TrimSpace(in.Dashpath); d != "" && d != "-" {
		if !strings.HasPrefix(d, "/") {
			d = "/" + d
		}
		pc.Dashpath = d
	}
	if in.ClearRenew {
		pc.Renewbot = ""
		pc.RenewbotAuto = false
	} else if rb := strings.TrimSpace(strings.TrimPrefix(in.Renewbot, "@")); rb != "" {
		pc.Renewbot = rb
		pc.RenewbotAuto = in.Auto
	}
	alerts.SaveCfg(r.Store, cfg)
	webOK(w, map[string]any{"ok": true, "secret": pc.Secret, "hook": r.webHookBase() + "/pghook/" + pc.Secret})
}

// webHookTest sends a sample expiry alert to the signed-in admin.
func (r *Router) webHookTest(w http.ResponseWriter, req *http.Request, uid int64) {
	var in struct {
		Idx int `json:"idx"`
	}
	if !webDecode(w, req, &in) {
		return
	}
	panels := r.panels()
	if in.Idx < 0 || in.Idx >= len(panels) {
		webErr(w, http.StatusNotFound, "پنل پیدا نشد")
		return
	}
	p := panels[in.Idx]
	cfg := alerts.GetCfg(r.Store)
	rb := ""
	if pc := cfg.Panels[p.URL]; pc != nil {
		rb = pc.Renewbot
	}
	items := []map[string]any{{"username": "test-user", "kind": "days",
		"value": 1, "expireTs": nil, "status": "", "owner": "-"}}
	alerts.SendAlert(r.Bot, uid, pg.Panel{Name: pname(p), URL: p.URL, Username: rb}, items, true)
	webOK(w, map[string]any{"ok": true})
}
