package router

import (
	"fmt"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/aknuun/cg-server/internal/srvmon"
)

const webSrvInfoCmd = `printf 'OS: '; (. /etc/os-release 2>/dev/null && echo "$PRETTY_NAME") || echo '?'; ` +
	`printf 'Kernel: '; uname -r; ` +
	`printf 'CPU: '; grep -m1 'model name' /proc/cpuinfo | cut -d: -f2 | sed 's/^ *//'; ` +
	`printf 'Cores: '; nproc; ` +
	`printf 'Uptime: '; uptime -p 2>/dev/null || uptime; ` +
	`printf 'Load: '; cut -d' ' -f1-3 /proc/loadavg; ` +
	`printf 'Mem: '; free -h 2>/dev/null | awk '/Mem:/{print $3" / "$2}'`

func (r *Router) webSrvInfo(w http.ResponseWriter, req *http.Request, uid int64) {
	s := r.webSrvGet(req.URL.Query().Get("id"))
	if s == nil {
		webErr(w, http.StatusNotFound, "سرور پیدا نشد")
		return
	}
	res := r.webSrvRun(s, webSrvInfoCmd, 20)
	if !res.Ok {
		webErr(w, http.StatusBadGateway, "وصل نشد: "+res.Err)
		return
	}
	webOK(w, map[string]any{"text": res.Out})
}

func (r *Router) webSrvDisk(w http.ResponseWriter, req *http.Request, uid int64) {
	s := r.webSrvGet(req.URL.Query().Get("id"))
	if s == nil {
		webErr(w, http.StatusNotFound, "سرور پیدا نشد")
		return
	}
	res := r.webSrvRun(s, "df -h -x tmpfs -x devtmpfs 2>/dev/null || df -h", 20)
	if !res.Ok {
		webErr(w, http.StatusBadGateway, "وصل نشد: "+res.Err)
		return
	}
	webOK(w, map[string]any{"text": res.Out})
}

func (r *Router) webSrvMonCfg(w http.ResponseWriter, req *http.Request, uid int64) {
	if req.Method == http.MethodPost {
		var in struct {
			Enabled     *bool    `json:"enabled"`
			CPU         *float64 `json:"cpuPct"`
			Mem         *float64 `json:"memPct"`
			Disk        *float64 `json:"diskPct"`
			CooldownMin *float64 `json:"cooldownMin"`
		}
		if !webDecode(w, req, &in) {
			return
		}
		cfg := srvmon.Cfg(r.Store)
		if in.Enabled != nil {
			cfg["enabled"] = *in.Enabled
		}
		if in.CPU != nil && *in.CPU > 0 && *in.CPU <= 100 {
			cfg["cpuPct"] = *in.CPU
		}
		if in.Mem != nil && *in.Mem > 0 && *in.Mem <= 100 {
			cfg["memPct"] = *in.Mem
		}
		if in.Disk != nil && *in.Disk > 0 && *in.Disk <= 100 {
			cfg["diskPct"] = *in.Disk
		}
		if in.CooldownMin != nil && *in.CooldownMin >= 1 {
			cfg["cooldownMin"] = *in.CooldownMin
		}
		srvmon.Save(r.Store, cfg)
	}
	webOK(w, srvmon.Cfg(r.Store))
}

// webSrvImport takes "host user password [port]" lines, one server per line.
func (r *Router) webSrvImport(w http.ResponseWriter, req *http.Request, uid int64) {
	var in struct {
		Lines string `json:"lines"`
	}
	if !webDecode(w, req, &in) {
		return
	}
	lst := r.srvList()
	added, skipped := 0, 0
	for _, line := range strings.Split(in.Lines, "\n") {
		f := strings.Fields(strings.TrimSpace(line))
		if len(f) == 0 || strings.HasPrefix(f[0], "#") {
			continue
		}
		if len(f) < 2 {
			skipped++
			continue
		}
		host, port := f[0], 22
		host, port = webSplitHost(f[0], port)
		user := f[1]
		secret := ""
		if len(f) >= 3 && f[2] != "-" {
			secret = f[2]
		}
		if len(f) >= 4 {
			if p, err := strconv.Atoi(f[3]); err == nil && p > 0 && p < 65536 {
				port = p
			}
		}
		auth := "none"
		switch {
		case secret == "":
		case strings.HasPrefix(secret, "-----BEGIN"):
			auth = "key"
		default:
			auth = "pass"
		}
		lst = append(lst, srvRec(fmt.Sprintf("s%d%d", time.Now().UnixNano(), added),
			host, host, user, port, auth, secret, ""))
		added++
	}
	if added > 0 {
		r.srvSave(lst)
	}
	webOK(w, map[string]any{"added": added, "skipped": skipped})
}

func (r *Router) webSrvPw(w http.ResponseWriter, req *http.Request, uid int64) {
	if req.Method == http.MethodPost {
		var in struct {
			Secret string `json:"secret"`
			Del    *int   `json:"del"`
		}
		if !webDecode(w, req, &in) {
			return
		}
		pws := r.srvPwList()
		if in.Del != nil {
			i := *in.Del
			if i < 0 || i >= len(pws) {
				webErr(w, http.StatusBadRequest, "رمز پیدا نشد")
				return
			}
			pws = append(pws[:i], pws[i+1:]...)
		} else {
			s := strings.TrimSpace(in.Secret)
			if s == "" {
				webErr(w, http.StatusBadRequest, "رمز خالی است")
				return
			}
			pws = append(pws, s)
		}
		r.Store.PutJSON("srv_passwords", pws, 0)
	}
	list := r.srvPwList()
	out := make([]string, 0, len(list))
	for _, p := range list {
		s := strv(p)
		if len(s) > 4 {
			s = s[:2] + "…" + s[len(s)-2:]
		} else {
			s = "…"
		}
		out = append(out, s)
	}
	webOK(w, map[string]any{"count": len(out), "masked": out})
}

// webSrvSubAdd registers servers picked from Cloudflare subdomains.
func (r *Router) webSrvSubAdd(w http.ResponseWriter, req *http.Request, uid int64) {
	var in struct {
		Items    []struct{ Name, Host string } `json:"items"`
		Password string                        `json:"password"`
		PwIndex  *int                          `json:"pwIndex"`
	}
	if !webDecode(w, req, &in) {
		return
	}
	if len(in.Items) == 0 {
		webErr(w, http.StatusBadRequest, "هیچ موردی انتخاب نشده")
		return
	}
	if in.PwIndex != nil {
		if l := r.srvPwList(); *in.PwIndex >= 0 && *in.PwIndex < len(l) {
			in.Password = strv(l[*in.PwIndex])
		}
	}
	pw := strings.TrimSpace(in.Password)
	auth := "none"
	switch {
	case pw == "":
	case strings.HasPrefix(pw, "-----BEGIN"):
		auth = "key"
	default:
		auth = "pass"
	}
	lst := r.srvList()
	have := map[string]bool{}
	for _, s := range lst {
		have[strings.ToLower(strv(s["name"]))] = true
	}
	added, dup := 0, 0
	for _, it := range in.Items {
		name := strings.TrimSpace(it.Name)
		host := strings.TrimSpace(it.Host)
		if name == "" || host == "" {
			continue
		}
		key := strings.ToLower(name)
		if have[key] {
			dup++
			continue
		}
		have[key] = true
		lst = append(lst, srvRec(fmt.Sprintf("s%d%d", time.Now().UnixNano(), added),
			truncRune(name, 60), host, "root", 22, auth, pw, "افزوده از ساب‌دامنه "+name))
		added++
	}
	if added > 0 {
		r.srvSave(lst)
	}
	webOK(w, map[string]any{"added": added, "dup": dup})
}
