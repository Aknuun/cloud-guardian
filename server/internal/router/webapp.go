package router

import (
	"crypto/hmac"
	"crypto/sha256"
	"crypto/subtle"
	"embed"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io/fs"
	"math/rand"
	"net/http"
	"net/url"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/aknuun/cg-server/internal/hz"
)

//go:embed webui
var webuiFS embed.FS

// WebApp returns the Telegram Mini App surface: static files under /app and a
// small JSON API under /api. It is mounted on the existing health HTTP server,
// so no extra port/service is needed.
func (r *Router) WebApp() http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("/api/auth", r.webAuthAPI)
	mux.HandleFunc("/api/login/start", r.webLoginStart)
	mux.HandleFunc("/api/login/state", r.webLoginState)
	mux.HandleFunc("/api/login/ticket", r.webLoginTicket)
	mux.HandleFunc("/api/me", r.webAuthed(r.webMe))
	mux.HandleFunc("/api/hz/accounts", r.webAuthed(r.webHzAccounts))
	mux.HandleFunc("/api/hz/servers", r.webAuthed(r.webHzServers))
	mux.HandleFunc("/api/hz/server", r.webAuthed(r.webHzServer))
	mux.HandleFunc("/api/hz/snapshots", r.webAuthed(r.webHzSnapshots))
	mux.HandleFunc("/api/hz/options", r.webAuthed(r.webHzOptions))
	mux.HandleFunc("/api/hz/ips", r.webAuthed(r.webHzIPs))
	mux.HandleFunc("/api/hz/action", r.webAuthed(r.webHzAction))
	mux.HandleFunc("/api/hz/create", r.webAuthed(r.webHzCreate))
	mux.HandleFunc("/api/cf/accounts", r.webAuthed(r.webCfAccounts))
	mux.HandleFunc("/api/cf/zones", r.webAuthed(r.webCfZones))
	mux.HandleFunc("/api/cf/records", r.webAuthed(r.webCfRecords))
	mux.HandleFunc("/api/cf/record", r.webAuthed(r.webCfRecordOp))
	mux.HandleFunc("/api/cf/bulk", r.webAuthed(r.webCfBulk))
	mux.HandleFunc("/api/cf/settings", r.webAuthed(r.webCfSettings))
	mux.HandleFunc("/api/cf/email", r.webAuthed(r.webCfEmail))
	mux.HandleFunc("/api/cf/traffic", r.webAuthed(r.webCfTraffic))
	mux.HandleFunc("/api/cf/favs", r.webAuthed(r.webCfFavs))
	mux.HandleFunc("/api/cf/search", r.webAuthed(r.webCfSearch))
	mux.HandleFunc("/api/cf/refs", r.webAuthed(r.webCfRefs))
	mux.HandleFunc("/api/cf/lbgroups", r.webAuthed(r.webCfLbGroups))
	mux.HandleFunc("/api/cf/lb", r.webAuthed(r.webCfLb))
	mux.HandleFunc("/api/arv/accounts", r.webAuthed(r.webArvAccounts))
	mux.HandleFunc("/api/arv/accounts/add", r.webAuthed(r.webArvAccAdd))
	mux.HandleFunc("/api/arv/accounts/del", r.webAuthed(r.webArvAccDel))
	mux.HandleFunc("/api/arv/domains", r.webAuthed(r.webArvDomains))
	mux.HandleFunc("/api/arv/records", r.webAuthed(r.webArvRecords))
	mux.HandleFunc("/api/arv/record", r.webAuthed(r.webArvRecordOp))
	mux.HandleFunc("/api/arv/servers", r.webAuthed(r.webArvServers))
	mux.HandleFunc("/api/arv/options", r.webAuthed(r.webArvOptions))
	mux.HandleFunc("/api/arv/snapshots", r.webAuthed(r.webArvSnapshots))
	mux.HandleFunc("/api/arv/action", r.webAuthed(r.webArvAction))
	mux.HandleFunc("/api/arv/info", r.webAuthed(r.webArvServerInfo))
	mux.HandleFunc("/api/arv/ipcheck", r.webAuthed(r.webArvIPCheck))
	mux.HandleFunc("/api/arv/plans", r.webAuthed(r.webArvPlans))
	mux.HandleFunc("/api/arv/upgrade", r.webAuthed(r.webArvUpgrade))
	mux.HandleFunc("/api/arv/hunt", r.webAuthed(r.webArvHuntStatus))
	mux.HandleFunc("/api/arv/hunt/start", r.webAuthed(r.webArvHuntStart))
	mux.HandleFunc("/api/arv/hunt/stop", r.webAuthed(r.webArvHuntStop))
	mux.HandleFunc("/api/arv/create", r.webAuthed(r.webArvCreate))
	mux.HandleFunc("/api/tools/srv", r.webAuthed(r.webToolsSrv))
	mux.HandleFunc("/api/tools/rem", r.webAuthed(r.webToolsRem))
	mux.HandleFunc("/api/tools/pg", r.webAuthed(r.webToolsPg))
	mux.HandleFunc("/api/tools/pg/panel/add", r.webAuthed(r.webPgPanelAdd))
	mux.HandleFunc("/api/tools/pg/panel/edit", r.webAuthed(r.webPgPanelEdit))
	mux.HandleFunc("/api/tools/pg/panel/del", r.webAuthed(r.webPgPanelDel))
	mux.HandleFunc("/api/tools/pg/mon/add", r.webAuthed(r.webPgMonAdd))
	mux.HandleFunc("/api/tools/pg/mon/toggle", r.webAuthed(r.webPgMonToggle))
	mux.HandleFunc("/api/tools/pg/mon/del", r.webAuthed(r.webPgMonDel))
	mux.HandleFunc("/api/tools/pg/mon", r.webAuthed(r.webToolsNd))
	mux.HandleFunc("/api/tools/pg/mon/sync", r.webAuthed(r.webNdSync))
	mux.HandleFunc("/api/tools/pg/mon/test", r.webAuthed(r.webNdTest))
	mux.HandleFunc("/api/tools/pg/mon/exc", r.webAuthed(r.webNdExc))
	mux.HandleFunc("/api/tools/pg/mon/cfg", r.webAuthed(r.webNdCfg))
	mux.HandleFunc("/api/tools/um", r.webAuthed(r.webToolsUm))
	mux.HandleFunc("/api/tools/dom", r.webAuthed(r.webToolsDom))
	mux.HandleFunc("/api/tools/dom/ssl", r.webAuthed(r.webDomSslOp))
	mux.HandleFunc("/api/tools/dom/add", r.webAuthed(r.webDomAdd))
	mux.HandleFunc("/api/tools/dom/del", r.webAuthed(r.webDomDel))
	mux.HandleFunc("/api/tools/dom/date", r.webAuthed(r.webDomDate))
	mux.HandleFunc("/api/tools/dom/refresh", r.webAuthed(r.webDomRefresh))
	mux.HandleFunc("/api/tools/dom/one", r.webAuthed(r.webDomOne))
	mux.HandleFunc("/api/tools/dom/sslcfg", r.webAuthed(r.webDomSslCfg))
	mux.HandleFunc("/api/tools/dom/cfg", r.webAuthed(func(w http.ResponseWriter, req *http.Request, uid int64) {
		switch req.Method {
		case http.MethodGet:
			r.webDomCfgGet(w, req, uid)
		case http.MethodPost:
			r.webDomCfgSet(w, req, uid)
		default:
			webErr(w, http.StatusMethodNotAllowed, "method not allowed")
		}
	}))
	mux.HandleFunc("/api/tools/dom/cfg/test", r.webAuthed(r.webDomCfgTest))
	mux.HandleFunc("/api/tools/hf", r.webAuthed(r.webToolsHf))
	mux.HandleFunc("/api/tools/hf/cfg", r.webAuthed(r.webHfCfg))
	mux.HandleFunc("/api/tools/hf/hosts", r.webAuthed(r.webHfHosts))
	mux.HandleFunc("/api/tools/hf/exc", r.webAuthed(r.webHfExc))
	mux.HandleFunc("/api/tools/hf/run", r.webAuthed(r.webHfRun))
	mux.HandleFunc("/api/tools/hf/log", r.webAuthed(r.webHfLog))
	mux.HandleFunc("/api/tools/hf/revert", r.webAuthed(r.webHfRevert))
	mux.HandleFunc("/api/tools/hf/bk", r.webAuthed(r.webHfBk))
	mux.HandleFunc("/api/tools/settings", r.webAuthed(r.webToolsSettings))
	mux.HandleFunc("/api/tools/settings/check", r.webAuthed(r.webSetCheck))
	mux.HandleFunc("/api/tools/settings/update", r.webAuthed(r.webSetUpdate))
	mux.HandleFunc("/api/tools/settings/admin", r.webAuthed(r.webSetAdmin))
	mux.HandleFunc("/api/tools/mail", r.webAuthed(r.webToolsMail))
	mux.HandleFunc("/api/tools/mail/zone", r.webAuthed(r.webMailZone))
	mux.HandleFunc("/api/tools/mail/set", r.webAuthed(r.webMailSet))
	mux.HandleFunc("/api/tools/mail/off", r.webAuthed(r.webMailOff))
	mux.HandleFunc("/api/tools/mail/dest", r.webAuthed(r.webMailDestAdd))
	mux.HandleFunc("/api/tools/hfd", r.webAuthed(r.webToolsHfd))
	mux.HandleFunc("/api/tools/hfd/cfg", r.webAuthed(r.webHfdCfg))
	mux.HandleFunc("/api/tools/hfd/hosts", r.webAuthed(r.webHfdHosts))
	mux.HandleFunc("/api/tools/hfd/field", r.webAuthed(r.webHfdField))
	mux.HandleFunc("/api/tools/hfd/run", r.webAuthed(r.webHfdRun))
	mux.HandleFunc("/api/tools/hfd/log", r.webAuthed(r.webHfdLog))
	mux.HandleFunc("/api/tools/hfd/revert", r.webAuthed(r.webHfdRevert))
	mux.HandleFunc("/api/tools/hook", r.webAuthed(r.webToolsHook))
	mux.HandleFunc("/api/tools/hook/cfg", r.webAuthed(r.webHookCfg))
	mux.HandleFunc("/api/tools/hook/panel", r.webAuthed(r.webHookPanel))
	mux.HandleFunc("/api/tools/hook/test", r.webAuthed(r.webHookTest))
	mux.HandleFunc("/api/tools/qa/search", r.webAuthed(r.webQaSearch))
	mux.HandleFunc("/api/tools/qa/fav", r.webAuthed(r.webQaFav))
	mux.HandleFunc("/api/tools/qa/del", r.webAuthed(r.webQaDel))
	mux.HandleFunc("/api/tools/qa/add", r.webAuthed(r.webQaAdd))
	mux.HandleFunc("/api/tools/quota", r.webAuthed(r.webToolsQuota))
	mux.HandleFunc("/api/tools/quota/cfg", r.webAuthed(r.webQuotaCfg))
	mux.HandleFunc("/api/tools/hub", r.webAuthed(r.webToolsHub))
	mux.HandleFunc("/api/hub/join", func(w http.ResponseWriter, req *http.Request) {
		r.webHubJoin(w, req, 0)
	})
	mux.HandleFunc("/api/hub/ping", func(w http.ResponseWriter, req *http.Request) {
		r.webHubPing(w, req, 0)
	})
	mux.HandleFunc("/api/hub/feed", func(w http.ResponseWriter, req *http.Request) {
		r.webHubFeed(w, req, 0)
	})
	mux.HandleFunc("/api/hub/send", func(w http.ResponseWriter, req *http.Request) {
		r.webHubSend2(w, req, 0)
	})
	mux.HandleFunc("/api/tools/hub/members", r.webAuthed(r.webHubMembers))
	mux.HandleFunc("/api/tools/hub/cast-installs", r.webAuthed(r.webHubCastInstalls))
	mux.HandleFunc("/api/tools/hub/history", r.webAuthed(r.webHubHistory))
	mux.HandleFunc("/api/tools/hub/history/del", r.webAuthed(r.webHubDel))
	mux.HandleFunc("/api/tools/hub/cfg", r.webAuthed(r.webHubCfg))
	mux.HandleFunc("/api/tools/hub/ann", r.webAuthed(r.webHubAnn))
	mux.HandleFunc("/api/tools/hub/cast", r.webAuthed(r.webHubCast))
	mux.HandleFunc("/api/tools/fo", r.webAuthed(r.webToolsFo))
	mux.HandleFunc("/api/tools/fo/setup", r.webAuthed(r.webFoSetup))
	mux.HandleFunc("/api/tools/fo/enable", r.webAuthed(r.webFoEnable))
	mux.HandleFunc("/api/tools/fo/disable", r.webAuthed(r.webFoDisable))
	mux.HandleFunc("/api/tools/fo/delete", r.webAuthed(r.webFoDelete))
	mux.HandleFunc("/api/tools/fo/refresh", r.webAuthed(r.webFoRefresh))
	mux.HandleFunc("/api/tools/hook/apply", r.webAuthed(r.webHookApply))
	mux.HandleFunc("/api/hz/accounts/add", r.webAuthed(r.webHzAccAdd))
	mux.HandleFunc("/api/hz/accounts/del", r.webAuthed(r.webHzAccDel))
	mux.HandleFunc("/api/tools/srv/mon", r.webAuthed(r.webSrvMon))
	mux.HandleFunc("/api/tools/srv/reboot", r.webAuthed(r.webSrvReboot))
	mux.HandleFunc("/api/tools/srv/upd", r.webAuthed(r.webSrvUpdate))
	mux.HandleFunc("/api/tools/srv/info", r.webAuthed(r.webSrvInfo))
	mux.HandleFunc("/api/tools/srv/disk", r.webAuthed(r.webSrvDisk))
	mux.HandleFunc("/api/tools/srv/moncfg", r.webAuthed(r.webSrvMonCfg))
	mux.HandleFunc("/api/tools/srv/import", r.webAuthed(r.webSrvImport))
	mux.HandleFunc("/api/tools/srv/pw", r.webAuthed(r.webSrvPw))
	mux.HandleFunc("/api/tools/srv/subadd", r.webAuthed(r.webSrvSubAdd))
	mux.HandleFunc("/api/tools/srv/add", r.webAuthed(r.webToolsSrvAdd))
	mux.HandleFunc("/api/tools/srv/edit", r.webAuthed(r.webToolsSrvEdit))
	mux.HandleFunc("/api/tools/srv/del", r.webAuthed(r.webToolsSrvDel))
	mux.HandleFunc("/api/tools/rem/add", r.webAuthed(r.webToolsRemAdd))
	mux.HandleFunc("/api/tools/rem/del", r.webAuthed(r.webToolsRemDel))
	mux.HandleFunc("/api/tools/rem/done", r.webAuthed(r.webToolsRemDone))
	mux.Handle("/app/", r.webStatic())
	mux.HandleFunc("/app", func(w http.ResponseWriter, req *http.Request) {
		http.Redirect(w, req, "/app/", http.StatusFound)
	})
	return mux
}

// ---------- auth ----------

func (r *Router) webSessionSecret() []byte {
	m := hmac.New(sha256.New, []byte("cg-web-session"))
	m.Write([]byte(r.BotToken))
	return m.Sum(nil)
}

func (r *Router) webSignSession(uid int64, exp int64) string {
	msg := fmt.Sprintf("%d.%d", uid, exp)
	m := hmac.New(sha256.New, r.webSessionSecret())
	m.Write([]byte(msg))
	return msg + "." + hex.EncodeToString(m.Sum(nil))
}

func (r *Router) webCheckSession(tok string) (int64, bool) {
	parts := strings.Split(tok, ".")
	if len(parts) != 3 {
		return 0, false
	}
	uid, err1 := strconv.ParseInt(parts[0], 10, 64)
	exp, err2 := strconv.ParseInt(parts[1], 10, 64)
	if err1 != nil || err2 != nil || uid <= 0 || exp < time.Now().Unix() {
		return 0, false
	}
	want := r.webSignSession(uid, exp)
	if subtle.ConstantTimeCompare([]byte(want), []byte(tok)) != 1 {
		return 0, false
	}
	return uid, true
}

// validateInitData verifies Telegram WebApp initData and returns the user id.
func validateInitData(initData, botToken string, maxAge time.Duration) (int64, error) {
	q, err := url.ParseQuery(strings.TrimSpace(initData))
	if err != nil {
		return 0, fmt.Errorf("initData نامعتبر است")
	}
	got := q.Get("hash")
	if got == "" {
		return 0, fmt.Errorf("initData ناقص است")
	}
	keys := make([]string, 0, len(q))
	for k := range q {
		if k != "hash" {
			keys = append(keys, k)
		}
	}
	sort.Strings(keys)
	lines := make([]string, 0, len(keys))
	for _, k := range keys {
		lines = append(lines, k+"="+q.Get(k))
	}
	secret := hmac.New(sha256.New, []byte("WebAppData"))
	secret.Write([]byte(botToken))
	m := hmac.New(sha256.New, secret.Sum(nil))
	m.Write([]byte(strings.Join(lines, "\n")))
	want := hex.EncodeToString(m.Sum(nil))
	if subtle.ConstantTimeCompare([]byte(want), []byte(got)) != 1 {
		return 0, fmt.Errorf("امضای initData نامعتبر است")
	}
	if ad, err := strconv.ParseInt(q.Get("auth_date"), 10, 64); err == nil && maxAge > 0 {
		if time.Since(time.Unix(ad, 0)) > maxAge {
			return 0, fmt.Errorf("نشست منقضی شده؛ ربات را دوباره باز کن")
		}
	}
	var u struct {
		ID int64 `json:"id"`
	}
	if err := json.Unmarshal([]byte(q.Get("user")), &u); err != nil || u.ID <= 0 {
		return 0, fmt.Errorf("کاربر initData پیدا نشد")
	}
	return u.ID, nil
}

func (r *Router) webAuthAPI(w http.ResponseWriter, req *http.Request) {
	if req.Method != http.MethodPost {
		webErr(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}
	var in struct {
		InitData string `json:"initData"`
	}
	if err := json.NewDecoder(http.MaxBytesReader(w, req.Body, 1<<20)).Decode(&in); err != nil {
		webErr(w, http.StatusBadRequest, "بدنهٔ درخواست نامعتبر است")
		return
	}
	uid, err := validateInitData(in.InitData, r.BotToken, 24*time.Hour)
	if err != nil {
		webErr(w, http.StatusUnauthorized, err.Error())
		return
	}
	if !r.isAdmin(uid) {
		webErr(w, http.StatusForbidden, "این حساب دسترسی ادمین ندارد")
		return
	}
	exp := time.Now().Add(webLoginSess).Unix()
	webOK(w, map[string]any{
		"token": r.webSignSession(uid, exp),
		"user":  map[string]any{"id": uid, "isMain": r.IsMain(uid)},
	})
}

type webHandler func(http.ResponseWriter, *http.Request, int64)

func (r *Router) webAuthed(fn webHandler) http.HandlerFunc {
	return func(w http.ResponseWriter, req *http.Request) {
		h := req.Header.Get("Authorization")
		tok := strings.TrimSpace(strings.TrimPrefix(h, "Bearer "))
		uid, ok := r.webCheckSession(tok)
		if !ok {
			webErr(w, http.StatusUnauthorized, "نشست نامعتبر یا منقضی است")
			return
		}
		if !r.isAdmin(uid) {
			webErr(w, http.StatusForbidden, "دسترسی ادمین لازم است")
			return
		}
		w.Header().Set("Cache-Control", "no-store")
		fn(w, req, uid)
	}
}

// ---------- static ----------

func (r *Router) webStatic() http.Handler {
	sub, err := fs.Sub(webuiFS, "webui")
	if err != nil {
		return http.NotFoundHandler()
	}
	fileServer := http.FileServer(http.FS(sub))
	return http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
		p := strings.TrimPrefix(req.URL.Path, "/app")
		if p == "" || p == "/" {
			p = "/index.html"
		}
		if _, err := fs.Stat(sub, strings.TrimPrefix(p, "/")); err != nil {
			p = "/index.html"
		}
		setWebHeaders(w)
		if p == "/index.html" {
			// Serve the SPA shell directly: http.FileServer would redirect
			// index.html to ./ and create a loop with /app/.
			data, err := fs.ReadFile(sub, "index.html")
			if err != nil {
				http.NotFound(w, req)
				return
			}
			w.Header().Set("Content-Type", "text/html; charset=utf-8")
			w.Header().Set("Cache-Control", "no-store, max-age=0")
			w.Header().Set("Pragma", "no-cache")
			w.Header().Set("Expires", "0")
			_, _ = w.Write(data)
			return
		}
		w.Header().Set("Cache-Control", "public, max-age=3600")
		req2 := req.Clone(req.Context())
		req2.URL.Path = p
		fileServer.ServeHTTP(w, req2)
	})
}

func setWebHeaders(w http.ResponseWriter) {
	w.Header().Set("X-Content-Type-Options", "nosniff")
	w.Header().Set("Referrer-Policy", "no-referrer")
	w.Header().Set("Content-Security-Policy",
		"default-src 'self'; script-src 'self' https://telegram.org; style-src 'self' 'unsafe-inline'; "+
			"img-src 'self' data:; connect-src 'self'; frame-ancestors https://web.telegram.org https://*.telegram.org")
}

// ---------- json ----------

func webJSON(w http.ResponseWriter, code int, v any) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.Header().Set("Cache-Control", "no-store")
	w.WriteHeader(code)
	_ = json.NewEncoder(w).Encode(v)
}

func webErr(w http.ResponseWriter, code int, msg string) {
	webJSON(w, code, map[string]any{"ok": false, "error": msg})
}

func webOK(w http.ResponseWriter, v any) {
	webJSON(w, http.StatusOK, map[string]any{"ok": true, "data": v})
}

// ---------- api ----------

func (r *Router) webMe(w http.ResponseWriter, req *http.Request, uid int64) {
	webOK(w, map[string]any{"id": uid, "isMain": r.IsMain(uid)})
}

func (r *Router) webAcc(i int) (map[string]any, bool) {
	accs := r.hzAccs()
	if i < 0 || i >= len(accs) {
		return nil, false
	}
	return accs[i], true
}

func (r *Router) webHzAccounts(w http.ResponseWriter, req *http.Request, uid int64) {
	accs := r.hzAccs()
	out := make([]map[string]any, 0, len(accs))
	for i, a := range accs {
		row := map[string]any{"i": i, "name": strv(a["name"])}
		if srv, ok := r.hzServersQuick(i); ok {
			row["count"] = len(srv)
		}
		out = append(out, row)
	}
	webOK(w, map[string]any{"accounts": out})
}

// webHzAll pages through a list endpoint and reports API errors instead of
// silently returning an empty list.
func webHzAll(cli *hz.Client, key, path string) ([]map[string]any, error) {
	var out []map[string]any
	sep := "?"
	if strings.Contains(path, "?") {
		sep = "&"
	}
	page := 1
	for page <= 20 {
		d, err := cli.Call(fmt.Sprintf("%s%sper_page=50&page=%d", path, sep, page), "GET", nil)
		if err != nil {
			return nil, err
		}
		out = append(out, hzList(d, key)...)
		meta, _ := d["meta"].(map[string]any)
		pag, _ := meta["pagination"].(map[string]any)
		np := intOfAny(pag["next_page"])
		if np <= 0 || np == page {
			break
		}
		page = np
	}
	return out, nil
}

func hzWebIncluded(s map[string]any) float64 { return hz.GB(floatAny(s["included_traffic"])) }

func hzWebTraffic(s map[string]any) map[string]any {
	ib := hz.GB(floatAny(s["ingoing_traffic"]))
	ob := hz.GB(floatAny(s["outgoing_traffic"]))
	total := float64(int64((ib+ob)*1000+0.5)) / 1000
	incl := hzWebIncluded(s)
	usedPct := 0.0
	extra := 0.0
	if incl > 0 {
		usedPct = float64(int64(ob/incl*1000+0.5)) / 10
		extra = float64(int64(maxf(total-incl, 0)*1000+0.5)) / 1000
	}
	return map[string]any{
		"in_gb": round2(ib), "out_gb": round2(ob), "total_gb": round2(total),
		"included_gb": round2(incl), "used_pct": usedPct, "extra_gb": round2(extra),
	}
}

func round2(f float64) float64 { return float64(int64(f*100+0.5)) / 100 }

func hzWebServerObj(s map[string]any) map[string]any {
	pn, _ := s["public_net"].(map[string]any)
	ip4, ip6 := "", ""
	if m, _ := pn["ipv4"].(map[string]any); m != nil {
		ip4 = strv(m["ip"])
	}
	if m, _ := pn["ipv6"].(map[string]any); m != nil {
		ip6 = strv(m["ip"])
	}
	st, _ := s["server_type"].(map[string]any)
	loc, _ := s["location"].(map[string]any)
	img, _ := s["image"].(map[string]any)
	imgName := ""
	if img != nil {
		imgName = strv(img["name"])
		if imgName == "" {
			imgName = strv(img["description"])
		}
	}
	return map[string]any{
		"id": strv(s["id"]), "name": strv(s["name"]), "status": strv(s["status"]),
		"ipv4": ip4, "ipv6": ip6, "locked": boolVal(s["locked"]),
		"created": strv(s["created"]), "labels": s["labels"],
		"type": map[string]any{
			"id": strv(st["id"]), "name": strv(st["name"]),
			"cores": intOfAny(st["cores"]), "memory": intOfAny(st["memory"]),
			"disk": intOfAny(st["disk"]), "architecture": strv(st["architecture"]),
		},
		"location": map[string]any{
			"name": strv(loc["name"]), "city": strv(loc["city"]), "country": strv(loc["country"]),
		},
		"image":   imgName,
		"traffic": hzWebTraffic(s),
	}
}

func (r *Router) hzWebCli(acc int) (*hz.Client, error) {
	a, ok := r.webAcc(acc)
	if !ok {
		return nil, fmt.Errorf("اکانت پیدا نشد")
	}
	return hz.New(strv(a["token"])), nil
}

func (r *Router) webHzServers(w http.ResponseWriter, req *http.Request, uid int64) {
	acc := atoiSafe(req.URL.Query().Get("acc"))
	cli, err := r.hzWebCli(acc)
	if err != nil {
		webErr(w, http.StatusBadRequest, err.Error())
		return
	}
	list, err := webHzAll(cli, "servers", "/servers")
	if err != nil {
		webErr(w, http.StatusBadGateway, err.Error())
		return
	}
	out := make([]map[string]any, 0, len(list))
	for _, s := range list {
		out = append(out, hzWebServerObj(s))
	}
	webOK(w, map[string]any{"servers": out})
}

func (r *Router) webHzServer(w http.ResponseWriter, req *http.Request, uid int64) {
	q := req.URL.Query()
	acc := atoiSafe(q.Get("acc"))
	sid := q.Get("id")
	cli, err := r.hzWebCli(acc)
	if err != nil {
		webErr(w, http.StatusBadRequest, err.Error())
		return
	}
	if sid == "" {
		webErr(w, http.StatusBadRequest, "شناسهٔ سرور لازم است")
		return
	}
	d, err := cli.Call("/servers/"+sid, "GET", nil)
	if err != nil {
		webErr(w, http.StatusBadGateway, err.Error())
		return
	}
	s, _ := d["server"].(map[string]any)
	if s == nil {
		webErr(w, http.StatusNotFound, "سرور پیدا نشد")
		return
	}
	obj := hzWebServerObj(s)
	// pricing from the cached type list (blocking on first access only)
	locName := ""
	if loc, _ := s["location"].(map[string]any); loc != nil {
		locName = strv(loc["name"])
	}
	typeID := ""
	if st, _ := s["server_type"].(map[string]any); st != nil {
		typeID = strv(st["id"])
	}
	obj["prices"] = r.hzWebPrices(cli, acc, typeID, locName)
	snaps, _ := r.hzWebSnaps(cli, acc, sid)
	obj["snapshots"] = snaps
	webOK(w, map[string]any{"server": obj})
}

func (r *Router) hzWebPrices(cli *hz.Client, acc int, typeID, loc string) map[string]any {
	if typeID == "" {
		return map[string]any{}
	}
	euro := r.hzEuroCached()
	for _, t := range r.hzTypesCached(cli, acc) {
		if strv(t["id"]) != typeID {
			continue
		}
		var p map[string]any
		for _, x := range priceList(t) {
			if strv(x["location"]) == loc {
				p = x
				break
			}
		}
		if p == nil {
			if ps := priceList(t); len(ps) > 0 {
				p = ps[0]
			}
		}
		return map[string]any{
			"hourly":      hz.PriceFmt(priceGross(p, "price_hourly"), euro),
			"monthly":     hz.PriceFmt(priceGross(p, "price_monthly"), euro),
			"net_hourly":  priceGross(p, "price_hourly"),
			"net_monthly": priceGross(p, "price_monthly"),
		}
	}
	return map[string]any{}
}

func (r *Router) hzWebSnaps(cli *hz.Client, acc int, sid string) ([]map[string]any, error) {
	sidInt, _ := strconv.Atoi(sid)
	out := make([]map[string]any, 0)
	for _, im := range r.hzImagesCached(cli, acc) {
		if strv(im["type"]) != "snapshot" {
			continue
		}
		cf, _ := im["created_from"].(map[string]any)
		if cf == nil || intOfAny(cf["id"]) != sidInt {
			continue
		}
		name := strv(im["description"])
		if name == "" {
			name = strv(im["name"])
		}
		out = append(out, map[string]any{
			"id": strv(im["id"]), "name": name,
			"size_gb": floatAny(im["size"]), "created": strv(im["created"]),
			"description": strv(im["description"]),
		})
	}
	sort.Slice(out, func(i, j int) bool {
		return strv(out[i]["created"]) > strv(out[j]["created"])
	})
	return out, nil
}

func (r *Router) webHzSnapshots(w http.ResponseWriter, req *http.Request, uid int64) {
	q := req.URL.Query()
	acc := atoiSafe(q.Get("acc"))
	cli, err := r.hzWebCli(acc)
	if err != nil {
		webErr(w, http.StatusBadRequest, err.Error())
		return
	}
	// Refresh the image cache so a snapshot created in the bot shows up here.
	arvMem.Delete(fmt.Sprintf("mhzi:%d", acc))
	snaps, err := r.hzWebSnaps(cli, acc, q.Get("id"))
	if err != nil {
		webErr(w, http.StatusBadGateway, err.Error())
		return
	}
	webOK(w, map[string]any{"snapshots": snaps})
}

func (r *Router) webHzOptions(w http.ResponseWriter, req *http.Request, uid int64) {
	q := req.URL.Query()
	acc := atoiSafe(q.Get("acc"))
	loc := q.Get("loc")
	arch := q.Get("arch")
	cli, err := r.hzWebCli(acc)
	if err != nil {
		webErr(w, http.StatusBadRequest, err.Error())
		return
	}
	euro := r.hzEuroCached()
	locations := make([]map[string]any, 0)
	for _, l := range r.hzLocsCached(cli, acc) {
		locations = append(locations, map[string]any{
			"name": strv(l["name"]), "city": strv(l["city"]),
			"country": strv(l["country"]), "description": strv(l["description"]),
		})
	}
	types := make([]map[string]any, 0)
	for _, t := range r.hzTypesCached(cli, acc) {
		if loc != "" && !hzPlanInLoc(t, loc) {
			continue
		}
		if arch != "" && strv(t["architecture"]) != arch {
			continue
		}
		row := map[string]any{
			"id": strv(t["id"]), "name": strv(t["name"]),
			"cores": intOfAny(t["cores"]), "memory": intOfAny(t["memory"]),
			"disk": intOfAny(t["disk"]), "architecture": strv(t["architecture"]),
		}
		if loc != "" {
			var p map[string]any
			for _, x := range priceList(t) {
				if strv(x["location"]) == loc {
					p = x
					break
				}
			}
			row["monthly"] = hz.PriceFmt(priceGross(p, "price_monthly"), euro)
			row["monthly_net"] = priceGross(p, "price_monthly")
		}
		types = append(types, row)
	}
	sort.Slice(types, func(i, j int) bool {
		return floatAny(types[i]["monthly_net"]) < floatAny(types[j]["monthly_net"])
	})
	images := make([]map[string]any, 0)
	for _, im := range r.hzImagesCached(cli, acc) {
		if strv(im["type"]) != "system" || strv(im["status"]) != "available" {
			continue
		}
		if arch != "" && strv(im["architecture"]) != "" && strv(im["architecture"]) != arch {
			continue
		}
		name := strv(im["name"])
		if name == "" {
			name = strv(im["description"])
		}
		images = append(images, map[string]any{
			"id": strv(im["id"]), "name": name, "description": strv(im["description"]),
			"architecture": strv(im["architecture"]), "os_flavor": strv(im["os_flavor"]),
			"os_version": strv(im["os_version"]),
		})
	}
	sort.Slice(images, func(i, j int) bool { return strv(images[i]["name"]) < strv(images[j]["name"]) })
	webOK(w, map[string]any{"locations": locations, "types": types, "images": images})
}

func (r *Router) webHzIPs(w http.ResponseWriter, req *http.Request, uid int64) {
	acc := atoiSafe(req.URL.Query().Get("acc"))
	cli, err := r.hzWebCli(acc)
	if err != nil {
		webErr(w, http.StatusBadRequest, err.Error())
		return
	}
	pool := r.hzPoolList(cli, acc)
	if pool == nil {
		webErr(w, http.StatusBadGateway, "دریافت آی‌پی‌ها ناموفق بود")
		return
	}
	out := make([]map[string]any, 0, len(pool))
	for _, p := range pool {
		row := map[string]any{
			"id": p.ID, "ip": p.IP, "loc": p.Loc, "assignee": p.Assignee, "auto": p.Auto,
		}
		if p.HasCheck {
			row["check"] = map[string]any{
				"clean": p.Check.Clean, "listed": p.Check.Listed,
				"unknown": p.Check.Unknown, "answered": p.Check.Answered, "at": p.Check.At,
			}
		}
		out = append(out, row)
	}
	webOK(w, map[string]any{"ips": out})
}

func hzPrimaryIP(cli *hz.Client, id int) (map[string]any, error) {
	d, err := cli.Call(fmt.Sprintf("/primary_ips/%d", id), "GET", nil)
	if err != nil {
		return nil, err
	}
	p, _ := d["primary_ip"].(map[string]any)
	if p == nil {
		return nil, fmt.Errorf("آی‌پی پیدا نشد")
	}
	return p, nil
}

func hzWaitPrimaryState(cli *hz.Client, id, assignee int, maxMs int) bool {
	start := time.Now()
	for time.Since(start).Milliseconds() < int64(maxMs) {
		p, err := hzPrimaryIP(cli, id)
		if err == nil && intOfAny(p["assignee_id"]) == assignee {
			return true
		}
		time.Sleep(700 * time.Millisecond)
	}
	return false
}

func hzWaitServerPrimary(cli *hz.Client, sid string, ipID int, maxMs int) bool {
	start := time.Now()
	for time.Since(start).Milliseconds() < int64(maxMs) {
		d, err := cli.Call("/servers/"+sid, "GET", nil)
		if err == nil {
			s, _ := d["server"].(map[string]any)
			pn, _ := s["public_net"].(map[string]any)
			if intOfAny(hz.PrimaryIPID(pn["ipv4"])) == ipID {
				return true
			}
		}
		time.Sleep(700 * time.Millisecond)
	}
	return false
}

func hzRestorePrimary(cli *hz.Client, sid string, oldID, newID int) {
	sidInt := atoiSafe(sid)
	if newID > 0 {
		_, _ = cli.Call(fmt.Sprintf("/primary_ips/%d/actions/unassign", newID), "POST", map[string]any{})
		hzWaitPrimaryState(cli, newID, 0, 30000)
	}
	if oldID > 0 {
		_, _ = cli.Call(fmt.Sprintf("/primary_ips/%d/actions/assign", oldID), "POST",
			map[string]any{"assignee_id": sidInt, "assignee_type": "server"})
		hzWaitPrimaryState(cli, oldID, sidInt, 30000)
	}
	_, _ = cli.Call("/servers/"+sid+"/actions/poweron", "POST", map[string]any{})
}

func (r *Router) webHzAction(w http.ResponseWriter, req *http.Request, uid int64) {
	if req.Method != http.MethodPost {
		webErr(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}
	var in struct {
		Acc     int    `json:"acc"`
		ID      string `json:"id"`
		Action  string `json:"action"`
		Arg     string `json:"arg"`
		IPID    int    `json:"ipId"`
		Confirm bool   `json:"confirm"`
	}
	if err := json.NewDecoder(http.MaxBytesReader(w, req.Body, 1<<20)).Decode(&in); err != nil {
		webErr(w, http.StatusBadRequest, "بدنهٔ درخواست نامعتبر است")
		return
	}
	if in.ID == "" || in.Action == "" {
		webErr(w, http.StatusBadRequest, "پارامتر ناقص")
		return
	}
	cli, err := r.hzWebCli(in.Acc)
	if err != nil {
		webErr(w, http.StatusBadRequest, err.Error())
		return
	}
	destructive := map[string]bool{
		"reset_password": true, "rename": true, "rebuild": true, "resize": true,
		"delete": true, "ip_assign": true, "ip_unassign": true, "ip_replace": true, "ip_delete": true,
		"snapshot_create": true, "snapshot_delete": true,
	}
	if destructive[in.Action] && !in.Confirm {
		webErr(w, http.StatusPreconditionRequired, "این عملیات نیاز به تأیید دارد")
		return
	}
	sid := in.ID
	power := map[string]string{"poweron": "poweron", "poweroff": "poweroff", "reboot": "reboot", "reset": "reset"}
	if act, ok := power[in.Action]; ok {
		if _, err := cli.Call("/servers/"+sid+"/actions/"+act, "POST", map[string]any{}); err != nil {
			webErr(w, http.StatusBadGateway, err.Error())
			return
		}
		r.hzDropServers(in.Acc)
		webOK(w, map[string]any{"message": "دستور ارسال شد"})
		return
	}
	switch in.Action {
	case "reset_password":
		res, err := cli.Call("/servers/"+sid+"/actions/reset_password", "POST", map[string]any{})
		if err != nil {
			webErr(w, http.StatusBadGateway, err.Error())
			return
		}
		pw := ""
		if a, ok := res["action"].(map[string]any); ok {
			pw = strv(a["root_password"])
		}
		if pw == "" {
			pw = strv(res["root_password"])
		}
		webOK(w, map[string]any{"password": pw, "message": "رمز جدید ساخته شد"})
	case "rename":
		name := strings.TrimSpace(in.Arg)
		if name == "" {
			webErr(w, http.StatusBadRequest, "نام خالی است")
			return
		}
		if _, err := cli.Call("/servers/"+sid, "PUT", map[string]any{"name": name}); err != nil {
			webErr(w, http.StatusBadGateway, err.Error())
			return
		}
		r.hzDropServers(in.Acc)
		webOK(w, map[string]any{"message": "نام تغییر کرد"})
	case "snapshot_create":
		desc := strings.TrimSpace(in.Arg)
		if desc == "" {
			desc = "snap-" + sid + "-" + time.Now().Format("20060102-1504")
		}
		if _, err := cli.Call("/servers/"+sid+"/actions/create_image", "POST",
			map[string]any{"description": desc, "type": "snapshot"}); err != nil {
			webErr(w, http.StatusBadGateway, err.Error())
			return
		}
		r.hzDropImages(in.Acc)
		webOK(w, map[string]any{"message": "ساخت اسنپ‌شات شروع شد: " + desc})
	case "snapshot_delete":
		if in.Arg == "" {
			webErr(w, http.StatusBadRequest, "شناسهٔ اسنپ‌شات لازم است")
			return
		}
		if _, err := cli.Call("/images/"+in.Arg, "DELETE", nil); err != nil {
			webErr(w, http.StatusBadGateway, err.Error())
			return
		}
		r.hzDropImages(in.Acc)
		webOK(w, map[string]any{"message": "اسنپ‌شات حذف شد"})
	case "rebuild":
		if in.Arg == "" {
			webErr(w, http.StatusBadRequest, "تصویر لازم است")
			return
		}
		if _, err := cli.Call("/servers/"+sid+"/actions/rebuild", "POST",
			map[string]any{"image": in.Arg}); err != nil {
			webErr(w, http.StatusBadGateway, err.Error())
			return
		}
		r.hzDropServers(in.Acc)
		webOK(w, map[string]any{"message": "ریبیلد شروع شد"})
	case "resize":
		if in.Arg == "" {
			webErr(w, http.StatusBadRequest, "پلن جدید لازم است")
			return
		}
		d, err := cli.Call("/servers/"+sid, "GET", nil)
		if err != nil {
			webErr(w, http.StatusBadGateway, err.Error())
			return
		}
		s, _ := d["server"].(map[string]any)
		if strv(s["status"]) != "off" {
			webErr(w, http.StatusConflict, "برای ارتقا اول سرور را خاموش کن")
			return
		}
		if _, err := cli.Call("/servers/"+sid+"/actions/change_type", "POST",
			map[string]any{"server_type": in.Arg, "upgrade_disk": true}); err != nil {
			webErr(w, http.StatusBadGateway, err.Error())
			return
		}
		r.hzDropServers(in.Acc)
		webOK(w, map[string]any{"message": "ارتقا شروع شد"})
	case "delete":
		if _, err := cli.Call("/servers/"+sid, "DELETE", nil); err != nil {
			webErr(w, http.StatusBadGateway, err.Error())
			return
		}
		r.hzDropServers(in.Acc)
		webOK(w, map[string]any{"message": "سرور حذف شد"})
	case "ip_delete":
		if in.IPID <= 0 {
			webErr(w, http.StatusBadRequest, "آی‌پی لازم است")
			return
		}
		p, err := hzPrimaryIP(cli, in.IPID)
		if err != nil {
			webErr(w, http.StatusBadGateway, err.Error())
			return
		}
		if intOfAny(p["assignee_id"]) > 0 {
			webErr(w, http.StatusConflict, "این آی‌پی به یک سرور متصل است؛ اول آن را جدا کن")
			return
		}
		if _, err := cli.Call(fmt.Sprintf("/primary_ips/%d", in.IPID), "DELETE", nil); err != nil {
			webErr(w, http.StatusBadGateway, err.Error())
			return
		}
		checks := r.hzChecks(in.Acc)
		delete(checks, strconv.Itoa(in.IPID))
		r.hzSaveChecks(in.Acc, checks)
		webOK(w, map[string]any{"message": "آی‌پی حذف شد"})
	case "ip_assign":
		if in.IPID <= 0 {
			webErr(w, http.StatusBadRequest, "آی‌پی لازم است")
			return
		}
		if !hz.EnsureOff(cli, sid, 60000) {
			webErr(w, http.StatusConflict, "سرور در ۶۰ ثانیه خاموش نشد؛ دوباره تلاش کن")
			return
		}
		if _, err := cli.Call(fmt.Sprintf("/primary_ips/%d/actions/assign", in.IPID), "POST",
			map[string]any{"assignee_id": atoiSafe(sid), "assignee_type": "server"}); err != nil {
			webErr(w, http.StatusBadGateway, err.Error())
			return
		}
		if !hzWaitPrimaryState(cli, in.IPID, atoiSafe(sid), 45000) ||
			!hzWaitServerPrimary(cli, sid, in.IPID, 45000) {
			webErr(w, http.StatusGatewayTimeout, "آی‌پی به سرور متصل نشد؛ وضعیت را بررسی کن")
			return
		}
		_, _ = cli.Call("/servers/"+sid+"/actions/poweron", "POST", map[string]any{})
		r.hzDropServers(in.Acc)
		webOK(w, map[string]any{"message": "آی‌پی اختصاص یافت و سرور روشن شد"})
	case "ip_unassign":
		d, err := cli.Call("/servers/"+sid, "GET", nil)
		if err != nil {
			webErr(w, http.StatusBadGateway, err.Error())
			return
		}
		s, _ := d["server"].(map[string]any)
		pn, _ := s["public_net"].(map[string]any)
		key := "ipv4"
		if in.Arg == "ipv6" {
			key = "ipv6"
		}
		ipid := 0
		if m, _ := pn[key].(map[string]any); m != nil {
			ipid = intOfAny(m["id"])
		}
		if ipid <= 0 {
			webErr(w, http.StatusNotFound, "آی‌پی اختصاصی برای حذف وجود ندارد")
			return
		}
		if !hz.EnsureOff(cli, sid, 60000) {
			webErr(w, http.StatusConflict, "سرور در ۶۰ ثانیه خاموش نشد؛ دوباره تلاش کن")
			return
		}
		if _, err := cli.Call(fmt.Sprintf("/primary_ips/%d/actions/unassign", ipid), "POST",
			map[string]any{}); err != nil {
			webErr(w, http.StatusBadGateway, err.Error())
			return
		}
		if !hzWaitPrimaryState(cli, ipid, 0, 45000) {
			webErr(w, http.StatusGatewayTimeout, "آی‌پی در زمان مقرر جدا نشد؛ دوباره تلاش کن")
			return
		}
		r.hzDropServers(in.Acc)
		webOK(w, map[string]any{"message": "اختصاص آی‌پی حذف شد"})
	case "ip_replace":
		d, err := cli.Call("/servers/"+sid, "GET", nil)
		if err != nil {
			webErr(w, http.StatusBadGateway, err.Error())
			return
		}
		s, _ := d["server"].(map[string]any)
		loc, _ := s["location"].(map[string]any)
		locName := strv(loc["name"])
		if locName == "" {
			webErr(w, http.StatusConflict, "لوکیشن سرور مشخص نیست")
			return
		}
		pn, _ := s["public_net"].(map[string]any)
		oldID := 0
		oldIP := ""
		if m, _ := pn["ipv4"].(map[string]any); m != nil {
			oldID = intOfAny(m["id"])
			oldIP = strv(m["ip"])
		}
		sidInt := atoiSafe(sid)
		if !hz.EnsureOff(cli, sid, 60000) {
			webErr(w, http.StatusConflict, "سرور در ۶۰ ثانیه خاموش نشد؛ دوباره تلاش کن")
			return
		}
		if oldID > 0 {
			if _, err := cli.Call(fmt.Sprintf("/primary_ips/%d/actions/unassign", oldID), "POST", map[string]any{}); err != nil {
				_, _ = cli.Call("/servers/"+sid+"/actions/poweron", "POST", map[string]any{})
				webErr(w, http.StatusBadGateway, "جدا کردن IPv4 قبلی ناموفق بود: "+err.Error())
				return
			}
			if !hzWaitPrimaryState(cli, oldID, 0, 45000) {
				_, _ = cli.Call(fmt.Sprintf("/primary_ips/%d/actions/assign", oldID), "POST",
					map[string]any{"assignee_id": sidInt, "assignee_type": "server"})
				_, _ = cli.Call("/servers/"+sid+"/actions/poweron", "POST", map[string]any{})
				webErr(w, http.StatusGatewayTimeout, "IPv4 قبلی در زمان مقرر جدا نشد؛ دوباره تلاش کن")
				return
			}
		}
		c, err := cli.Call("/primary_ips", "POST", map[string]any{
			"type": "ipv4", "name": "replace-" + sid, "location": locName, "auto_delete": true})
		if err != nil {
			hzRestorePrimary(cli, sid, oldID, 0)
			webErr(w, http.StatusBadGateway, err.Error())
			return
		}
		nip, _ := c["primary_ip"].(map[string]any)
		newID := 0
		if nip != nil {
			newID = intOfAny(nip["id"])
		}
		if newID <= 0 {
			hzRestorePrimary(cli, sid, oldID, 0)
			webErr(w, http.StatusBadGateway, "ساخت IPv4 تازه ناموفق شد")
			return
		}
		if _, err := cli.Call(fmt.Sprintf("/primary_ips/%d/actions/assign", newID), "POST",
			map[string]any{"assignee_id": sidInt, "assignee_type": "server"}); err != nil {
			hzRestorePrimary(cli, sid, oldID, newID)
			_, _ = cli.Call(fmt.Sprintf("/primary_ips/%d", newID), "DELETE", nil)
			webErr(w, http.StatusBadGateway, "اتصال IPv4 تازه ناموفق بود: "+err.Error())
			return
		}
		if !hzWaitPrimaryState(cli, newID, sidInt, 45000) || !hzWaitServerPrimary(cli, sid, newID, 45000) {
			hzRestorePrimary(cli, sid, oldID, newID)
			_, _ = cli.Call(fmt.Sprintf("/primary_ips/%d", newID), "DELETE", nil)
			webErr(w, http.StatusGatewayTimeout, "IPv4 تازه در زمان مقرر روی سرور فعال نشد؛ IPv4 قبلی برگردانده شد")
			return
		}
		if oldID > 0 {
			_, _ = cli.Call(fmt.Sprintf("/primary_ips/%d", oldID), "DELETE", nil)
			checks := r.hzChecks(in.Acc)
			delete(checks, strconv.Itoa(oldID))
			r.hzSaveChecks(in.Acc, checks)
		}
		_, _ = cli.Call("/servers/"+sid+"/actions/poweron", "POST", map[string]any{})
		r.hzDropServers(in.Acc)
		newIP := strv(nip["ip"])
		msg := "IPv4 سرور تعویض شد: " + newIP
		if oldIP != "" {
			msg += " (قبلی: " + oldIP + ")"
		}
		webOK(w, map[string]any{"message": msg, "ip": newIP})
	default:
		webErr(w, http.StatusBadRequest, "عملیات ناشناخته")
	}
}

func (r *Router) webHzCreate(w http.ResponseWriter, req *http.Request, uid int64) {
	if req.Method != http.MethodPost {
		webErr(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}
	var in struct {
		Acc      int    `json:"acc"`
		Name     string `json:"name"`
		Type     string `json:"type"`
		Image    string `json:"image"`
		Location string `json:"location"`
	}
	if err := json.NewDecoder(http.MaxBytesReader(w, req.Body, 1<<20)).Decode(&in); err != nil {
		webErr(w, http.StatusBadRequest, "بدنهٔ درخواست نامعتبر است")
		return
	}
	in.Name = strings.TrimSpace(in.Name)
	if in.Name == "" || len(in.Name) > 63 || strings.ContainsAny(in.Name, " \t/") {
		webErr(w, http.StatusBadRequest, "نام سرور نامعتبر است (بدون فاصله، حداکثر ۶۳ کاراکتر)")
		return
	}
	if in.Type == "" || in.Image == "" || in.Location == "" {
		webErr(w, http.StatusBadRequest, "پلن، تصویر و لوکیشن لازم است")
		return
	}
	cli, err := r.hzWebCli(in.Acc)
	if err != nil {
		webErr(w, http.StatusBadRequest, err.Error())
		return
	}
	td, err := cli.Call("/server_types/"+in.Type, "GET", nil)
	if err != nil {
		webErr(w, http.StatusBadGateway, err.Error())
		return
	}
	st, _ := td["server_type"].(map[string]any)
	if st == nil {
		webErr(w, http.StatusBadRequest, "پلن پیدا نشد")
		return
	}
	if !hzPlanInLoc(st, in.Location) {
		webErr(w, http.StatusBadRequest, "این پلن در لوکیشن انتخاب‌شده موجود نیست")
		return
	}
	b := make([]byte, 16)
	for k := range b {
		b[k] = randChars[rand.Intn(len(randChars))]
	}
	pw := string(b)
	cloudInit := "#cloud-config\nssh_pwauth: true\nchpasswd:\n  list: |\n    root:" + pw + "\n  expire: False\n"
	resp, err := cli.Call("/servers", "POST", map[string]any{
		"name": in.Name, "server_type": in.Type, "image": in.Image, "location": in.Location,
		"start_after_create": true, "user_data": cloudInit,
	})
	if err != nil {
		webErr(w, http.StatusBadGateway, err.Error())
		return
	}
	r.hzDropServers(in.Acc)
	hs, _ := resp["server"].(map[string]any)
	ip := ""
	if hs != nil {
		if pn, _ := hs["public_net"].(map[string]any); pn != nil {
			if m, _ := pn["ipv4"].(map[string]any); m != nil {
				ip = strv(m["ip"])
			}
		}
	}
	if ip != "" {
		name := in.Name
		if hs != nil && strv(hs["name"]) != "" {
			name = strv(hs["name"])
		}
		lst := r.srvList()
		lst = append(lst, srvRec(fmt.Sprintf("s%d%d", time.Now().Unix(), rand.Intn(90)+10),
			name, ip, "root", 22, "pass", pw, "Hetzner: "+name))
		r.srvSave(lst)
	}
	out := map[string]any{"password": pw, "ip": ip}
	if hs != nil {
		out["server"] = hzWebServerObj(hs)
	}
	webOK(w, out)
}

// ---------- helpers ----------

// hzDropImages invalidates only the image cache (snapshots create/delete).
func (r *Router) hzDropImages(i int) {
	arvMem.Delete(fmt.Sprintf("mhzi:%d", i))
	r.Store.Delete(fmt.Sprintf("hzimg:%d", i))
}
