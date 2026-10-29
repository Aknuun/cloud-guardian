package router

import (
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/aknuun/cg-server/internal/sslmon"
)

// sslCfg is the certificate alert config (store key "ssl_cfg").
type sslCfg struct {
	Enabled  bool `json:"enabled"`
	WarnDays int  `json:"warnDays"`
}

func (r *Router) sslCfgGet() sslCfg {
	c := sslCfg{Enabled: true, WarnDays: 7}
	var m map[string]any
	if !r.Store.GetJSON("ssl_cfg", &m) || m == nil {
		return c
	}
	if v, ok := m["enabled"].(bool); ok {
		c.Enabled = v
	}
	if v, ok := m["warnDays"].(float64); ok && v >= 1 && v <= 365 {
		c.WarnDays = int(v)
	}
	return c
}

func (r *Router) sslCfgSave(c sslCfg) {
	r.Store.PutJSON("ssl_cfg", c, 0)
}

// webDomSslCfg updates the certificate warning threshold.
func (r *Router) webDomSslCfg(w http.ResponseWriter, req *http.Request, uid int64) {
	var in sslCfg
	if !webDecode(w, req, &in) {
		return
	}
	if in.WarnDays < 1 || in.WarnDays > 365 {
		webErr(w, http.StatusBadRequest, "تعداد روز هشدار باید بین ۱ تا ۳۶۵ باشد")
		return
	}
	r.sslCfgSave(in)
	webOK(w, r.webDomState())
}

// SslTick checks every monitored certificate and alerts the admin once a day
// for hosts at or under the warning threshold (or unreachable).
func (r *Router) SslTick() {
	cfg := r.sslCfgGet()
	if !cfg.Enabled {
		return
	}
	mons := sslmon.GetMonitors(r.Store)
	if len(mons) == 0 {
		return
	}
	var sent map[string]string
	r.Store.GetJSON("ssl_alert", &sent)
	if sent == nil {
		sent = map[string]string{}
	}
	today := time.Now().UTC().Format("2006-01-02")
	var lines []string
	for _, m := range mons {
		res := sslmon.CheckHost(m.Host, 15*time.Second)
		due := !res.OK || res.Days <= cfg.WarnDays
		if !due || sent[m.Host] == today {
			continue
		}
		sent[m.Host] = today
		if !res.OK {
			lines = append(lines, fmt.Sprintf("🔴 <code>%s</code> — خطا: %s", sslmon.Escape(res.Host), sslmon.Escape(res.Error)))
			continue
		}
		lines = append(lines, fmt.Sprintf("⚠️ <code>%s</code> — %d روز مانده (انقضا: %s)", sslmon.Escape(res.Host), res.Days, sslmon.Escape(res.Expiry)))
	}
	r.Store.PutJSON("ssl_alert", sent, 0)
	if len(lines) == 0 {
		return
	}
	msg := "🔐 هشدار گواهی SSL\n\n" + strings.Join(lines, "\n")
	if r.AdminID != 0 {
		r.Bot.Send(r.AdminID, msg, nil)
	}
}
