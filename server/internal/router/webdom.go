package router

import (
	"net/http"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/aknuun/cg-server/internal/domexpiry"
	"github.com/aknuun/cg-server/internal/jalali"
	"github.com/aknuun/cg-server/internal/sslmon"
)

type webDomSsl struct {
	Host   string `json:"host"`
	OK     bool   `json:"ok"`
	Days   int    `json:"days"`
	Expiry string `json:"expiry"`
	Error  string `json:"error"`
}

type webDomItem struct {
	Domain    string `json:"domain"`
	Status    string `json:"status"`
	Expiry    *int64 `json:"expiry"`
	Manual    bool   `json:"manual"`
	Remaining string `json:"remaining"`
}

type webDomState struct {
	SSL  []webDomSsl  `json:"ssl"`
	Doms []webDomItem `json:"doms"`
	Key  bool         `json:"key"`
}

func (r *Router) webDomState() webDomState {
	st := webDomState{SSL: []webDomSsl{}, Doms: []webDomItem{}}
	mons := sslmon.GetMonitors(r.Store)
	out := make([]webDomSsl, len(mons))
	var wg sync.WaitGroup
	for i, m := range mons {
		wg.Add(1)
		go func(i int, host string) {
			defer wg.Done()
			res := sslmon.CheckHost(host, 10*time.Second)
			out[i] = webDomSsl{Host: res.Host, OK: res.OK, Days: res.Days, Expiry: res.Expiry, Error: res.Error}
		}(i, m.Host)
	}
	wg.Wait()
	st.SSL = out

	_, key := r.domexpCfg()
	st.Key = key != ""
	now := domexpiry.NowMS()
	lst := r.domexpList()
	sort.SliceStable(lst, func(i, j int) bool {
		ei, ej := lst[i].Expiry, lst[j].Expiry
		if ei == nil {
			return false
		}
		if ej == nil {
			return true
		}
		return *ei < *ej
	})
	for _, it := range lst {
		st.Doms = append(st.Doms, webDomItem{Domain: it.Domain, Status: it.Status, Expiry: it.Expiry, Manual: it.Manual, Remaining: domexpiry.Remaining(it.Expiry, now)})
	}
	return st
}

// webToolsDom returns SSL certificate status and domain expiry records.
func (r *Router) webToolsDom(w http.ResponseWriter, req *http.Request, uid int64) {
	webOK(w, r.webDomState())
}

func webHostName(raw string) string {
	h := strings.ToLower(strings.TrimSpace(raw))
	h = strings.TrimPrefix(strings.TrimPrefix(h, "https://"), "http://")
	if i := strings.IndexAny(h, "/:"); i >= 0 {
		h = h[:i]
	}
	return strings.TrimSpace(h)
}

func (r *Router) webDomSslOp(w http.ResponseWriter, req *http.Request, uid int64) {
	var in struct {
		Host string `json:"host"`
	}
	if !webDecode(w, req, &in) {
		return
	}
	host := webHostName(in.Host)
	if host == "" || !strings.Contains(host, ".") {
		webErr(w, http.StatusBadRequest, "نام دامنه معتبر نیست")
		return
	}
	mons := sslmon.GetMonitors(r.Store)
	next := make([]sslmon.Monitor, 0, len(mons)+1)
	found := false
	for _, m := range mons {
		if m.Host == host {
			found = true
			continue
		}
		next = append(next, m)
	}
	if !found {
		next = append(next, sslmon.Monitor{Host: host})
	}
	r.Store.PutJSON("ssl_monitor", next, 0)
	webOK(w, r.webDomState())
}

func (r *Router) webDomItemOp(w http.ResponseWriter, req *http.Request, uid int64, mode string) {
	var in struct {
		Domain string `json:"domain"`
		Date   int64  `json:"date"`
		When   string `json:"when"` // jalali "1404/08/07"
	}
	if !webDecode(w, req, &in) {
		return
	}
	domain := webHostName(in.Domain)
	lst := r.domexpList()
	idx := -1
	for i := range lst {
		if lst[i].Domain == domain {
			idx = i
		}
	}
	switch mode {
	case "add":
		if domain == "" || !domexpiry.IsValidDomain(domain) {
			webErr(w, http.StatusBadRequest, "نام دامنه معتبر نیست")
			return
		}
		if idx >= 0 {
			webErr(w, http.StatusBadRequest, "این دامنه از قبل ثبت شده")
			return
		}
		_, key := r.domexpCfg()
		res := domexpiry.FetchDomainExpiry(domain, key)
		it := domexpiry.Item{Domain: domain, Status: res.Status, Expiry: res.Expiry, Added: domexpiry.NowMS()}
		lst = append(lst, it)
	case "del":
		if idx < 0 {
			webErr(w, http.StatusNotFound, "دامنه پیدا نشد")
			return
		}
		lst = append(lst[:idx:idx], lst[idx+1:]...)
	case "date":
		if idx < 0 {
			webErr(w, http.StatusNotFound, "دامنه پیدا نشد")
			return
		}
		ms := in.Date
		if raw := strings.TrimSpace(in.When); raw != "" {
			if !strings.Contains(raw, " ") {
				raw += " 12:00"
			}
			v, err := jalali.ParseDatetime(jalali.ParseFA(raw))
			if err != nil {
				webErr(w, http.StatusBadRequest, "تاریخ نامعتبر است (مثال: 1405/06/20)")
				return
			}
			ms = v
		}
		if ms <= domexpiry.NowMS() {
			webErr(w, http.StatusBadRequest, "تاریخ باید در آینده باشد")
			return
		}
		d := ms
		lst[idx].Manual = true
		lst[idx].Status = "ok"
		lst[idx].Expiry = &d
	default:
		webErr(w, http.StatusBadRequest, "عملیات نامعتبر")
		return
	}
	r.Store.PutJSON("dom_expiry", lst, 0)
	webOK(w, r.webDomState())
}

func (r *Router) webDomAdd(w http.ResponseWriter, req *http.Request, uid int64) {
	r.webDomItemOp(w, req, uid, "add")
}

func (r *Router) webDomDel(w http.ResponseWriter, req *http.Request, uid int64) {
	r.webDomItemOp(w, req, uid, "del")
}

func (r *Router) webDomDate(w http.ResponseWriter, req *http.Request, uid int64) {
	r.webDomItemOp(w, req, uid, "date")
}

// webDomRefresh re-checks every monitored domain (slow: ~1s each).
func (r *Router) webDomRefresh(w http.ResponseWriter, req *http.Request, uid int64) {
	if req.Method != http.MethodPost {
		webErr(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}
	lst := r.domexpList()
	_, key := r.domexpCfg()
	var wg sync.WaitGroup
	for i := range lst {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			res := domexpiry.FetchDomainExpiry(lst[i].Domain, key)
			if res.Status == "ok" && res.Expiry != nil {
				lst[i].Status = "ok"
				lst[i].Expiry = res.Expiry
			} else if res.Status != "" {
				lst[i].Status = res.Status
			}
		}(i)
	}
	wg.Wait()
	r.Store.PutJSON("dom_expiry", lst, 0)
	webOK(w, r.webDomState())
}
