// Package ndmon implements the PasarGuard node monitor: polling node state
// on panels and firing up/down alerts to admins.
package ndmon

import (
	"strings"
	"time"

	"github.com/aknuun/cg-server/internal/jalali"
	"github.com/aknuun/cg-server/internal/pg"
	"github.com/aknuun/cg-server/internal/store"
	"github.com/aknuun/cg-server/internal/tg"
)

const (
	alertCooldownMS = 30 * 60000
	quietMS         = 10 * 60000
	stateResaveMS   = 5 * 60000
)

var upStatus = map[string]bool{
	"connected": true, "up": true, "online": true, "running": true, "healthy": true,
}

var downStatus = map[string]bool{
	"error": true, "down": true, "offline": true, "stopped": true,
	"unreachable": true, "off": true,
}

func NodeDir(status any) string {
	s := strings.ToLower(strv(status))
	if upStatus[s] {
		return "up"
	}
	if downStatus[s] {
		return "down"
	}
	return "mid"
}

// Monitor is a stored node monitor entry (JSON keys mirror the Python store).
type Monitor struct {
	ID          string   `json:"id"`
	PanelURL    string   `json:"panel_url"`
	Name        string   `json:"name"`
	IntervalMin int      `json:"intervalMin"`
	Enabled     *bool    `json:"enabled"`
	Excluded    []string `json:"excluded"`
	HookToken   string   `json:"hook_token"`
}

func (m Monitor) IsEnabled() bool { return m.Enabled == nil || *m.Enabled }

func (m Monitor) Label() string {
	if m.Name != "" {
		return m.Name
	}
	return m.ID
}

func Monitors(st *store.Store) []Monitor {
	var lst []Monitor
	st.GetJSON("node_monitors", &lst)
	return lst
}

func SaveMonitors(st *store.Store, lst []Monitor) { st.PutJSON("node_monitors", lst, 0) }

// NodeSt is the per-node snapshot kept in ndst:<id>.
type NodeSt struct {
	Name      string `json:"name"`
	Status    any    `json:"status"`
	Address   any    `json:"address"`
	Version   any    `json:"version"`
	Reason    any    `json:"reason"`
	TS        string `json:"ts"`
	FirstSeen string `json:"first_seen"`
	LastAlert string `json:"last_alert"`
	LastDir   string `json:"last_dir"`
}

type nodeState struct {
	Nodes map[string]NodeSt `json:"nodes"`
	TS    string            `json:"ts"`
}

func stateOf(st *store.Store, id string) nodeState {
	var s nodeState
	if st.GetJSON("ndst:"+id, &s) && s.Nodes != nil {
		return s
	}
	return nodeState{Nodes: map[string]NodeSt{}}
}

func Nodes(st *store.Store, id string) map[string]NodeSt {
	return stateOf(st, id).Nodes
}

// ---------- poll ----------

type pollEvent struct {
	Name string
	Dir  string
	Node pg.NodeInfo
}

type pollResult struct {
	Next   map[string]NodeSt
	Events []pollEvent
	HasNew bool
}

func applyPoll(prev map[string]NodeSt, lst []pg.NodeInfo, nowISO string) pollResult {
	nxt := map[string]NodeSt{}
	var events []pollEvent
	seen := map[string]bool{}
	hasNew := false
	for _, n := range lst {
		name := strings.TrimSpace(n.Name)
		if name == "" || seen[name] {
			continue
		}
		seen[name] = true
		p, had := prev[name]
		d := NodeDir(n.Status)
		if !had {
			hasNew = true
		} else {
			pd := NodeDir(p.Status)
			if pd != "" && pd != d && (d == "up" || d == "down") {
				events = append(events, pollEvent{Name: name, Dir: d, Node: n})
			}
		}
		firstSeen := nowISO
		lastAlert, lastDir := "", ""
		if had {
			if p.FirstSeen != "" {
				firstSeen = p.FirstSeen
			}
			lastAlert = p.LastAlert
			lastDir = p.LastDir
		}
		nxt[name] = NodeSt{
			Name: name, Status: n.Status, Address: n.Address, Version: n.Version,
			Reason: n.Reason, TS: nowISO, FirstSeen: firstSeen,
			LastAlert: lastAlert, LastDir: lastDir,
		}
	}
	return pollResult{Next: nxt, Events: events, HasNew: hasNew}
}

func shouldAlert(prev NodeSt, nowMS int64, dir string) bool {
	t := parseISO(prev.LastAlert)
	if t <= 0 {
		return true
	}
	if dir == "up" && prev.LastDir != "up" {
		return true
	}
	return nowMS-t >= alertCooldownMS
}

// RunNodePoll polls all enabled monitors whose interval has elapsed and returns
// the number of alerts sent. force=true ignores the interval gate.
func RunNodePoll(bot *tg.Client, st *store.Store, panels []pg.Panel, adminID int64, force bool) int {
	mons := Monitors(st)
	if len(panels) == 0 || len(mons) == 0 {
		return 0
	}
	byPanel := map[string]pg.Panel{}
	for _, p := range panels {
		byPanel[p.URL] = p
	}
	nowMS := time.Now().UnixMilli()
	nowISO := isoNow()
	sent := 0
	type creds struct {
		c   *pg.Pg
		tok string
		ok  bool
	}
	tokens := map[string]*creds{}
	for _, m := range mons {
		if !m.IsEnabled() {
			continue
		}
		interval := m.IntervalMin
		if interval < 1 {
			interval = 10
		}
		s := stateOf(st, m.ID)
		if !force {
			last := parseISO(s.TS)
			if last != 0 && nowMS-last < int64(interval)*60000 {
				continue
			}
		}
		panel, ok := byPanel[m.PanelURL]
		if !ok {
			continue
		}
		cr := tokens[panel.URL]
		if cr == nil {
			c := panel.Client()
			tok, err := c.Login()
			if err != nil {
				tokens[panel.URL] = &creds{}
				continue
			}
			cr = &creds{c: c, tok: tok, ok: true}
			tokens[panel.URL] = cr
		}
		if !cr.ok {
			continue
		}
		lst, err := cr.c.Nodes(cr.tok)
		if err != nil {
			continue
		}
		poll := applyPoll(s.Nodes, lst, nowISO)
		dirty, fired := poll.HasNew, false
		excluded := map[string]bool{}
		for _, e := range m.Excluded {
			excluded[e] = true
		}
		for _, ev := range poll.Events {
			if excluded[ev.Name] {
				continue
			}
			dirty = true
			prev := s.Nodes[ev.Name]
			if !shouldAlert(prev, nowMS, ev.Dir) {
				continue
			}
			gk := "ndq:" + panel.URL + ":" + ev.Name + ":" + ev.Dir
			lastG := int64(0)
			if v, ok := st.GetOK(gk); ok {
				lastG = parseI64(v)
			}
			if lastG > 0 && nowMS-lastG < quietMS {
				continue
			}
			st.Put(gk, itoa64(nowMS), 3600)
			msg := alertMsg(panel, ev)
			if bot.Send(adminID, msg, nil) != 0 {
				sent++
				fired = true
				ns := poll.Next[ev.Name]
				ns.LastAlert = nowISO
				ns.LastDir = ev.Dir
				poll.Next[ev.Name] = ns
			}
		}
		if s.Nodes == nil {
			s.Nodes = map[string]NodeSt{}
		}
		for name, nd := range poll.Next {
			s.Nodes[name] = nd
		}
		lastSave := parseISO(s.TS)
		if dirty && (fired || nowMS-lastSave >= stateResaveMS) {
			s.TS = nowISO
			st.PutJSON("ndst:"+m.ID, s, 0)
		}
	}
	return sent
}

func alertMsg(panel pg.Panel, ev pollEvent) string {
	ts := jalali.Fmt(time.Now().UnixMilli())
	if ev.Dir == "down" {
		msg := "🚨 نود قطع شد!\n🖥 پنل: <code>" + esc(panelName(panel)) + "</code>\n" +
			"🖧 نود: <code>" + esc(ev.Name) + "</code>"
		if a := strv(ev.Node.Address); a != "" {
			msg += "\n🌐 آی‌پی: <code>" + esc(a) + "</code>"
		}
		reason := strv(ev.Node.Reason)
		if reason == "" {
			reason = "—"
		}
		return msg + "\n⏱ زمان: " + ts + " به وقت ایران\n🔎 علت: " + esc(reason)
	}
	return "✅ نود وصل شد!\n🖥 پنل: <code>" + esc(panelName(panel)) + "</code>\n" +
		"🖧 نود: <code>" + esc(ev.Name) + "</code>\n⏱ زمان: " + ts + " به وقت ایران"
}

// HandleNodeHook fires a single event-driven node alert from a panel webhook.
func HandleNodeHook(bot *tg.Client, m Monitor, payload map[string]any, adminID int64) bool {
	ev := strings.ToLower(strv(payload["event"]))
	name := strv(payload["node"])
	if name == "" {
		name = strv(payload["name"])
	}
	if name == "" {
		name = "?"
	}
	reason := strv(payload["reason"])
	ts := jalali.Fmt(time.Now().UnixMilli())
	pname := m.Name
	if pname == "" {
		pname = m.PanelURL
	}
	var msg string
	if ev == "up" {
		msg = "✅ نود وصل شد!\n🖥 پنل: <code>" + esc(pname) + "</code>\n" +
			"🖧 نود: <code>" + esc(name) + "</code>\n⏱ زمان: " + ts + " به وقت ایران"
	} else {
		msg = "🚨 نود قطع شد!\n🖥 پنل: <code>" + esc(pname) + "</code>\n" +
			"🖧 نود: <code>" + esc(name) + "</code>\n⏱ زمان: " + ts + " به وقت ایران"
		if reason != "" {
			msg += "\n🔎 علت: <code>" + esc(reason) + "</code>"
		}
	}
	return bot.Send(adminID, msg, nil) != 0
}
