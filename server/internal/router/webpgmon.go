package router

import (
	"net/http"
	"sort"
	"strings"

	"github.com/aknuun/cg-server/internal/ndmon"
)

type webNdNode struct {
	Name    string `json:"name"`
	Dir     string `json:"dir"`
	Addr    string `json:"addr,omitempty"`
	Version string `json:"version,omitempty"`
	Reason  string `json:"reason,omitempty"`
	TS      string `json:"ts,omitempty"`
	Exc     bool   `json:"exc"`
}

type webNdState struct {
	ID          string      `json:"id"`
	Name        string      `json:"name"`
	Panel       string      `json:"panel"`
	IntervalMin int         `json:"interval"`
	Enabled     bool        `json:"enabled"`
	Hook        string      `json:"hook"`
	Nodes       []webNdNode `json:"nodes"`
}

func (r *Router) webNdState(m ndmon.Monitor) webNdState {
	st := webNdState{ID: m.ID, Name: m.Label(), Panel: m.PanelURL, IntervalMin: m.IntervalMin, Enabled: m.IsEnabled(), Nodes: []webNdNode{}}
	if st.IntervalMin == 0 {
		st.IntervalMin = 10
	}
	tok := r.ndEnsureToken(&m)
	st.Hook = r.webHookBase() + "/ndhook/" + tok
	exc := map[string]bool{}
	for _, e := range m.Excluded {
		exc[e] = true
	}
	names := make([]string, 0, 8)
	snap := ndmon.Nodes(r.Store, m.ID)
	for name := range snap {
		names = append(names, name)
	}
	sort.Strings(names)
	for _, name := range names {
		n := snap[name]
		st.Nodes = append(st.Nodes, webNdNode{
			Name: name, Dir: ndmon.NodeDir(n.Status), Addr: ndmon.AddrOf(n),
			Version: ndmon.VersionOf(n), Reason: ndmon.ReasonOf(n), TS: n.TS, Exc: exc[name],
		})
	}
	return st
}

func (r *Router) webNdGet(id string) *ndmon.Monitor {
	for _, m := range ndmon.Monitors(r.Store) {
		if m.ID == id {
			return &m
		}
	}
	return nil
}

// webToolsNd returns one node monitor with its node snapshot.
func (r *Router) webToolsNd(w http.ResponseWriter, req *http.Request, uid int64) {
	m := r.webNdGet(req.URL.Query().Get("id"))
	if m == nil {
		webErr(w, http.StatusNotFound, "مانیتور پیدا نشد")
		return
	}
	webOK(w, r.webNdState(*m))
}

func (r *Router) webNdSync(w http.ResponseWriter, req *http.Request, uid int64) {
	var in struct {
		ID string `json:"id"`
	}
	if !webDecode(w, req, &in) {
		return
	}
	m := r.webNdGet(in.ID)
	if m == nil {
		webErr(w, http.StatusNotFound, "مانیتور پیدا نشد")
		return
	}
	ndmon.RunNodePoll(r.Bot, r.Store, r.panels(), r.AdminID, true)
	webOK(w, r.webNdState(*m))
}

func (r *Router) webNdTest(w http.ResponseWriter, req *http.Request, uid int64) {
	var in struct {
		ID string `json:"id"`
	}
	if !webDecode(w, req, &in) {
		return
	}
	m := r.webNdGet(in.ID)
	if m == nil {
		webErr(w, http.StatusNotFound, "مانیتور پیدا نشد")
		return
	}
	msg := "🧪 پیام آزمایشی هشدار نود:\n\n" +
		"🚨 نود قطع شد!\n" +
		"🖥 پنل: <code>" + Esc(m.Label()) + "</code>\n" +
		"🖧 نود: <code>node-test</code>\n" +
		"🌐 آی‌پی: <code>1.2.3.4</code>\n" +
		"⏱ زمان: " + nowJalali() + " به وقت ایران\n" +
		"🔎 علت: <code>تست</code>"
	r.Bot.Send(uid, msg, nil)
	webOK(w, r.webNdState(*m))
}

// webNdExc toggles one node in/out of the monitor's exclusion list.
func (r *Router) webNdExc(w http.ResponseWriter, req *http.Request, uid int64) {
	var in struct {
		ID   string `json:"id"`
		Node string `json:"node"`
	}
	if !webDecode(w, req, &in) {
		return
	}
	lst := ndmon.Monitors(r.Store)
	idx := -1
	for i := range lst {
		if lst[i].ID == in.ID {
			idx = i
		}
	}
	if idx < 0 {
		webErr(w, http.StatusNotFound, "مانیتور پیدا نشد")
		return
	}
	name := strings.TrimSpace(in.Node)
	if name == "" {
		webErr(w, http.StatusBadRequest, "نام نود لازم است")
		return
	}
	excluded := make([]string, 0, len(lst[idx].Excluded)+1)
	had := false
	for _, e := range lst[idx].Excluded {
		if e == name {
			had = true
			continue
		}
		excluded = append(excluded, e)
	}
	if !had {
		excluded = append(excluded, name)
	}
	lst[idx].Excluded = excluded
	ndmon.SaveMonitors(r.Store, lst)
	webOK(w, r.webNdState(lst[idx]))
}

// webNdCfg saves interval and enabled state.
func (r *Router) webNdCfg(w http.ResponseWriter, req *http.Request, uid int64) {
	var in struct {
		ID       string `json:"id"`
		Interval int    `json:"interval"`
		Enabled  *bool  `json:"enabled"`
	}
	if !webDecode(w, req, &in) {
		return
	}
	lst := ndmon.Monitors(r.Store)
	idx := -1
	for i := range lst {
		if lst[i].ID == in.ID {
			idx = i
		}
	}
	if idx < 0 {
		webErr(w, http.StatusNotFound, "مانیتور پیدا نشد")
		return
	}
	if in.Interval < 1 || in.Interval > 1440 {
		webErr(w, http.StatusBadRequest, "فاصله باید بین ۱ تا ۱۴۴۰ دقیقه باشد")
		return
	}
	lst[idx].IntervalMin = in.Interval
	if in.Enabled != nil {
		v := *in.Enabled
		lst[idx].Enabled = &v
	}
	ndmon.SaveMonitors(r.Store, lst)
	webOK(w, r.webNdState(lst[idx]))
}
