package router

import (
	"net/http"
	"strings"

	"github.com/aknuun/cg-server/internal/domexpiry"
)

// webDomCfgGet returns the domain-expiry lookup settings (key never leaves the server).
func (r *Router) webDomCfgGet(w http.ResponseWriter, req *http.Request, uid int64) {
	provider, key := r.domexpCfg()
	webOK(w, map[string]any{
		"provider": provider,
		"hasKey":   key != "",
		"keyHint":  hintKey(key),
		"providers": []map[string]string{
			{"id": "whoisxmlapi_domaininfo", "label": "WhoisXML API (کلید لازم)"},
			{"id": "rdap", "label": "RDAP / whois آزاد (بدون کلید)"},
		},
	})
}

func hintKey(k string) string {
	if len(k) <= 6 {
		return strings.Repeat("•", len(k))
	}
	return k[:3] + "…" + k[len(k)-3:]
}

type webDomCfgIn struct {
	Provider string `json:"provider"`
	Key      string `json:"key"`
	Clear    bool   `json:"clear"`
}

// webDomCfgSet saves provider/key; a blank key keeps the stored one.
func (r *Router) webDomCfgSet(w http.ResponseWriter, req *http.Request, uid int64) {
	var in webDomCfgIn
	if !webDecode(w, req, &in) {
		return
	}
	provider, key := r.domexpCfg()
	if in.Provider != "" {
		provider = in.Provider
	}
	if in.Clear {
		key = ""
	} else if k := strings.TrimSpace(in.Key); k != "" {
		key = k
	}
	r.Store.PutJSON("dom_expiry_cfg", map[string]any{"provider": provider, "key": key}, 0)
	webOK(w, map[string]any{"provider": provider, "hasKey": key != ""})
}

// webDomCfgTest runs one real lookup so the stored key can be verified.
func (r *Router) webDomCfgTest(w http.ResponseWriter, req *http.Request, uid int64) {
	var in struct {
		Domain string `json:"domain"`
	}
	if !webDecode(w, req, &in) {
		return
	}
	dom := strings.TrimSpace(in.Domain)
	if dom == "" {
		dom = "pcapps.ir"
	}
	_, key := r.domexpCfg()
	res := domexpiry.FetchDomainExpiry(dom, key)
	out := map[string]any{"status": res.Status, "domain": dom, "message": res.Message}
	if res.Expiry != nil {
		out["expiry"] = *res.Expiry
		out["remaining"] = domexpiry.Remaining(res.Expiry, domexpiry.NowMS())
	}
	webOK(w, out)
}
