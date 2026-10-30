// Package alerts ports the classic pghook expiry/volume pipeline.
package alerts

import (
	"math"
	"strconv"
	"strings"
	"time"

	"github.com/aknuun/cg-server/internal/jalali"
	"github.com/aknuun/cg-server/internal/pg"
	"github.com/aknuun/cg-server/internal/store"
	"github.com/aknuun/cg-server/internal/tg"
)

const (
	QuietMS  = 45 * 60000
	UrgWinMS = 3 * 60000
)

var pgStatusFA = map[string][2]string{
	"active": {"🟢", "فعال"}, "expired": {"🔴", "منقضی"},
	"limited": {"🟠", "محدود"}, "on_hold": {"⏸", "تعلیق"},
	"disabled": {"⛔", "غیرفعال"},
}

type PanelCfg struct {
	Secret       string `json:"secret,omitempty"`
	Renewbot     string `json:"renewbot,omitempty"`
	RenewbotAuto bool   `json:"renewbotAuto,omitempty"`
	Dashpath     string `json:"dashpath,omitempty"`
}

type Cfg struct {
	Enabled  bool                 `json:"enabled"`
	Days     []float64            `json:"days"`
	Usage    []float64            `json:"usage"`
	Panels   map[string]*PanelCfg `json:"panels"`
	SilentMe bool                 `json:"silent_me"`
}

func nums(v any, lo, hi float64, dflt []float64) []float64 {
	arr, ok := v.([]any)
	if !ok {
		if f, ok := v.([]float64); ok {
			return f
		}
		return dflt
	}
	out := []float64{}
	for _, x := range arr {
		f, ok := toF(x)
		if !ok {
			continue
		}
		if f >= lo && f <= hi {
			out = append(out, f)
		}
	}
	if len(out) == 0 || len(out) > 10 {
		if len(out) > 10 {
			return out[:10]
		}
		return dflt
	}
	return out
}

func GetCfg(st *store.Store) Cfg {
	var raw map[string]any
	st.GetJSON("pghook_cfg", &raw)
	c := Cfg{
		Enabled: true,
		Days:    []float64{3},
		Usage:   []float64{90, 100},
		Panels:  map[string]*PanelCfg{},
	}
	if raw == nil {
		return c
	}
	if v, ok := raw["enabled"].(bool); ok {
		c.Enabled = v
	}
	if v, ok := raw["silent_me"].(bool); ok {
		c.SilentMe = v
	}
	c.Days = nums(raw["days"], 0, 100000, []float64{3})
	c.Usage = nums(raw["usage"], 0.0001, 100, []float64{90, 100})
	if pm, ok := raw["panels"].(map[string]any); ok {
		for k, v := range pm {
			m, _ := v.(map[string]any)
			pc := &PanelCfg{}
			if m != nil {
				pc.Secret, _ = m["secret"].(string)
				pc.Renewbot, _ = m["renewbot"].(string)
				pc.RenewbotAuto, _ = m["renewbotAuto"].(bool)
				pc.Dashpath, _ = m["dashpath"].(string)
			}
			c.Panels[k] = pc
		}
	}
	return c
}

func SaveCfg(st *store.Store, c Cfg) {
	st.PutJSON("pghook_cfg", c, 0)
}

type Event struct {
	Username      string
	Owner         string
	OwnerTg       int64
	Days          *float64
	Usage         *float64
	DaysExplicit  bool
	UsageExplicit bool
	ExpireTs      *int64
	Status        string
}

func toF(v any) (float64, bool) {
	switch t := v.(type) {
	case float64:
		return t, true
	case int:
		return float64(t), true
	case int64:
		return float64(t), true
	case string:
		f, err := strconv.ParseFloat(t, 64)
		return f, err == nil
	}
	return 0, false
}

func num(v any) *float64 {
	f, ok := toF(v)
	if !ok || math.IsInf(f, 0) || math.IsNaN(f) {
		return nil
	}
	return &f
}

func expireTS(exp any) *int64 {
	if exp == nil {
		return nil
	}
	switch t := exp.(type) {
	case float64:
		if t == 0 {
			return nil
		}
		ms := int64(t * 1000)
		if t >= 1e12 {
			ms = int64(t)
		}
		return &ms
	case string:
		if t == "" {
			return nil
		}
		if ms, ok := parseISO(t); ok {
			return &ms
		}
	}
	return nil
}

func parseISO(s string) (int64, bool) {
	for _, layout := range []string{time.RFC3339, "2006-01-02T15:04:05"} {
		if t, err := time.Parse(layout, s); err == nil {
			return t.UnixMilli(), true
		}
	}
	return 0, false
}

func parseEvent(p map[string]any) Event {
	if p == nil {
		p = map[string]any{}
	}
	u, _ := p["user"].(map[string]any)
	by, _ := p["by"].(map[string]any)
	if u == nil {
		u = map[string]any{}
	}
	if by == nil {
		by = map[string]any{}
	}
	s := func(v any) string {
		if v == nil {
			return ""
		}
		return trimSpace(toString(v))
	}
	ev := Event{}
	ev.Username = s(first(p["username"], p["user_name"], u["username"], p["name"]))
	ev.Owner = s(first(by["username"], p["admin"], p["owner"]))
	if ev.Owner == "" {
		if adm, ok := u["admin"].(map[string]any); ok {
			ev.Owner = s(adm["username"])
		} else {
			ev.Owner = s(u["admin"])
		}
	}
	ev.OwnerTg, _ = toI64(first(p["telegram_id"], by["telegram_id"]))
	days := num(first(p["days_left"], p["daysLeft"], p["expire_days"], p["remaining_days"]))
	ev.DaysExplicit = days != nil
	if days == nil {
		if ets := expireTS(first(u["expire"], u["expire_date"], p["expire"])); ets != nil {
			d := float64(*ets-time.Now().UnixMilli()) / 86400000
			days = &d
		}
	}
	ev.Days = days
	usage := num(first(p["usage_percent"], p["used_percent"], p["usage_percentage"]))
	ev.UsageExplicit = usage != nil
	if usage == nil {
		used := num(first(u["used_traffic"], p["used_traffic"]))
		lim := num(first(u["data_limit"], p["data_limit"]))
		if used != nil && lim != nil && *lim > 0 {
			v := *used / *lim * 100
			usage = &v
		}
	}
	ev.Usage = usage
	ev.ExpireTs = expireTS(first(u["expire"], u["expire_date"], p["expire"]))
	ev.Status = s(first(u["status"], p["status"]))
	return ev
}

func isUrgent(kind string, v float64) bool {
	switch kind {
	case "days":
		return v > 0 && v <= 1
	case "usage":
		return v >= 95 && v < 100
	}
	return false
}

type Job struct {
	Kind   string
	Value  float64
	Bucket float64
}

func eventJobs(ev Event) []Job {
	out := []Job{}
	if ev.Days != nil {
		dv := math.Floor(*ev.Days*10) / 10
		if ev.DaysExplicit || (dv > 0 && dv <= 1) {
			out = append(out, Job{"days", dv, 1})
		}
	}
	if ev.Usage != nil {
		uv := math.Floor(*ev.Usage*10) / 10
		if ev.UsageExplicit || (uv >= 95 && uv < 96) {
			out = append(out, Job{"usage", uv, 95})
		}
	}
	return out
}

func gateOK(st *store.Store, pid, username, kind string, bucket float64) bool {
	raw := st.Get("pgq:" + pid + ":" + username + ":" + kind)
	if raw == "" {
		return true
	}
	i := lastIndexByte(raw, '.')
	if i < 0 {
		return true
	}
	ts, ok := parseFloat(raw[:i])
	if !ok {
		return true
	}
	if float64(time.Now().UnixMilli())-ts < QuietMS {
		return false
	}
	b := raw[i+1:]
	return b != numStr(bucket)
}

func gateMark(st *store.Store, pid, username, kind string, bucket float64) {
	st.Put("pgq:"+pid+":"+username+":"+kind,
		numStr(float64(time.Now().UnixMilli()))+"."+numStr(bucket), 7*86400)
}

func alertText(panel pg.Panel, items []map[string]any, test bool) string {
	kinds := map[string]bool{}
	for _, a := range items {
		kinds[toString(a["kind"])] = true
	}
	kind := "days"
	if kinds["usage"] {
		kind = "usage"
	}
	head := "🔔 هشدار انقضا"
	if kind == "usage" {
		head = "📊 هشدار حجم"
	}
	pre := ""
	if test {
		pre = "🧪 تستی — "
	}
	out := []string{pre + head, "🖥 پنل: <code>" + panelName(panel) + "</code>", ""}
	now := time.Now().UnixMilli()
	for _, a := range items {
		ak := toString(a["kind"])
		if (kind == "usage") != (ak == "usage") {
			continue
		}
		out = append(out, "👤 کاربر: <code>"+toString(a["username"])+"</code>")
		if kind == "usage" {
			out = append(out, "📈 حجم مصرف شده: "+toString(a["value"])+"٪")
		} else {
			hrs := 0
			if ets, ok := toI64(a["expireTs"]); ok && ets > now {
				hrs = int((ets - now) / 3600000)
			}
			out = append(out, "⏳ انقضا در: "+itoa(maxInt(hrs, 0))+" ساعت دیگر")
		}
		if ets, ok := toI64(a["expireTs"]); ok && ets > 0 {
			out = append(out, "📅 انقضا: "+jalali.Fmt(ets))
		}
		if st := lower(toString(a["status"])); st != "" {
			if pair, ok := pgStatusFA[st]; ok {
				out = append(out, pair[0]+" وضعیت: "+pair[1])
			} else {
				out = append(out, "▪️ وضعیت: "+toString(a["status"]))
			}
		}
		if ow := toString(a["owner"]); ow != "" {
			out = append(out, "👮 ادمین پنل: "+ow)
		}
	}
	out = append(out, "⏱ "+jalali.Fmt(now))
	return joinLines(out)
}

func renewKB(panel pg.Panel, username string) [][]tg.Button {
	var row []tg.Button
	if panelURL := trimRight(panel.URL, "/"); panelURL != "" {
		row = append(row, tg.Button{Text: "🖥 تمدید با پنل",
			URL: panelURL + "/dashboard/#/users?search=" + trunc(username, 200)})
	}
	if len(row) == 0 {
		return nil
	}
	return [][]tg.Button{row}
}

func SendAlert(bot *tg.Client, to int64, panel pg.Panel, items []map[string]any, test bool) {
	name := ""
	if len(items) > 0 {
		name = toString(items[0]["username"])
	}
	bot.Send(to, alertText(panel, items, test), renewKB(panel, name))
}

// HandleEvents is the urgent webhook path; returns buffered jobs count.
func HandleEvents(bot *tg.Client, st *store.Store, panel pg.Panel, pid string, payload any, adminID int64) int {
	cfg := GetCfg(st)
	if !cfg.Enabled {
		return 0
	}
	var evs []any
	if arr, ok := payload.([]any); ok {
		evs = arr
	} else {
		evs = []any{payload}
	}
	n := 0
	for _, raw := range evs {
		p, ok := raw.(map[string]any)
		if !ok {
			continue
		}
		ev := parseEvent(p)
		if ev.Username == "" {
			continue
		}
		var batch []Job
		for _, j := range eventJobs(ev) {
			if !isUrgent(j.Kind, j.Value) {
				continue
			}
			if !gateOK(st, pid, ev.Username, j.Kind, j.Bucket) {
				continue
			}
			batch = append(batch, j)
		}
		if len(batch) == 0 {
			continue
		}
		for _, j := range batch {
			gateMark(st, pid, ev.Username, j.Kind, j.Bucket)
		}
		key := "pgub:" + pid
		var buf map[string]any
		st.GetJSON(key, &buf)
		if buf == nil {
			buf = map[string]any{"ts": time.Now().UnixMilli(), "items": []any{}}
		}
		items, _ := buf["items"].([]any)
		for _, j := range batch {
			items = append(items, map[string]any{
				"username": ev.Username, "kind": j.Kind, "value": j.Value,
				"expireTs": ev.ExpireTs, "status": ev.Status, "owner": ev.Owner,
				"to": []any{adminID},
			})
			n++
		}
		buf["items"] = items
		st.PutJSON(key, buf, 3600)
	}
	FlushBuffers(bot, st, map[string]pg.Panel{pid: panel}, adminID, false)
	return n
}

func FlushBuffers(bot *tg.Client, st *store.Store, panels map[string]pg.Panel, adminID int64, force bool) int {
	cfg := GetCfg(st)
	if !cfg.Enabled {
		return 0
	}
	sent := 0
	for pid, panel := range panels {
		key := "pgub:" + pid
		var buf map[string]any
		st.GetJSON(key, &buf)
		if buf == nil {
			continue
		}
		items, _ := buf["items"].([]any)
		if len(items) == 0 {
			continue
		}
		ts, _ := toI64(buf["ts"])
		if !force && time.Now().UnixMilli()-ts < UrgWinMS {
			continue
		}
		st.Delete(key)
		for _, kind := range []string{"usage", "days"} {
			var part []map[string]any
			for _, it := range items {
				if m, ok := it.(map[string]any); ok && toString(m["kind"]) == kind {
					part = append(part, m)
				}
			}
			if len(part) > 0 {
				if cfg.SilentMe {
					SendAlertUsers(bot, panel, part)
				} else {
					SendAlert(bot, adminID, panel, part, false)
				}
				sent++
			}
		}
	}
	return sent
}

// webhookURLs extracts every push-webhook URL configured on a panel.
func webhookURLs(wh map[string]any) []string {
	var urls []string
	if s, ok := wh["url"].(string); ok && strings.TrimSpace(s) != "" {
		urls = append(urls, strings.TrimSpace(s))
	}
	if arr, ok := wh["webhooks"].([]any); ok {
		for _, x := range arr {
			if m, ok := x.(map[string]any); ok {
				if s, ok := m["url"].(string); ok && strings.TrimSpace(s) != "" {
					urls = append(urls, strings.TrimSpace(s))
				}
			}
		}
	}
	return urls
}

// ownsDigest reports whether this install is the panel's designated alert
// receiver. When a panel has an active push webhook the pull-digest must run
// only on that panel's webhook owner (matched by our /pghook/<secret> path);
// otherwise every install that merely added the panel would duplicate reports.
func ownsDigest(cfg Cfg, p pg.Panel, stg map[string]any) bool {
	wh, _ := stg["webhook"].(map[string]any)
	if wh == nil {
		return true
	}
	enabled, _ := wh["enable"].(bool)
	urls := webhookURLs(wh)
	if !enabled || len(urls) == 0 {
		return true
	}
	secret := ""
	if pc := cfg.Panels[p.URL]; pc != nil {
		secret = pc.Secret
	}
	if secret == "" {
		return false
	}
	want := "/pghook/" + secret
	for _, u := range urls {
		if strings.Contains(u, want) {
			return true
		}
	}
	return false
}

// RunDigest performs hourly(usage)/daily(days) full-panel scans.
func RunDigest(bot *tg.Client, st *store.Store, panels []pg.Panel, kind string, adminID int64) int {
	cfg := GetCfg(st)
	if !cfg.Enabled {
		return 0
	}
	sent := 0
	for _, p := range panels {
		pid := p.URL
		c := p.Client()
		tok, err := c.Login()
		if err != nil {
			continue
		}
		if !ownsDigest(cfg, p, c.Settings(tok)) {
			continue
		}
		users, err := c.Users(tok, 1000)
		if err != nil {
			continue
		}
		skey := "pgdr:" + pid + ":" + kind
		var stobj map[string]any
		st.GetJSON(skey, &stobj)
		rep := map[string]float64{}
		if stobj != nil {
			if rm, ok := stobj["reported"].(map[string]any); ok {
				for k, v := range rm {
					if f, ok := toF(v); ok {
						rep[k] = f
					}
				}
			}
		}
		var fresh []map[string]any
		for _, u := range users {
			if u.Username == "" {
				continue
			}
			prev, has := rep[u.Username]
			if kind == "usage" {
				if !(u.Limit > 0) || u.Used < 0 {
					continue
				}
				pc := u.Used / u.Limit * 100
				switch {
				case pc >= 100:
					if !has || prev != 100 {
						rep[u.Username] = 100
					}
				case pc >= 95:
					if !has || prev != 95 {
						fresh = append(fresh, map[string]any{"username": u.Username,
							"kind": "usage", "value": round1(pc), "expireTs": nil,
							"status": "", "owner": u.Owner})
						rep[u.Username] = 95
					}
				case pc < 90 && has:
					delete(rep, u.Username)
				}
			} else {
				dl := pg.DaysLeft(u.Expire)
				if dl == nil {
					continue
				}
				switch {
				case *dl <= 0:
					if !has || prev != -1 {
						rep[u.Username] = -1
					}
				case *dl <= 1:
					if !has || prev != 1 {
						fresh = append(fresh, map[string]any{"username": u.Username,
							"kind": "days", "value": round1(*dl), "expireTs": nil,
							"status": "", "owner": u.Owner})
						rep[u.Username] = 1
					}
				case *dl > 2 && has:
					delete(rep, u.Username)
				}
			}
		}
		st.PutJSON(skey, map[string]any{"reported": rep, "lastSent": 0, "lastCount": 0}, 0)
		for _, a := range fresh {
			bucket := 1.0
			if kind == "usage" {
				bucket = 95
			}
			gateMark(st, pid, toString(a["username"]), kind, bucket)
		}
		if len(fresh) > 0 {
			panel := pg.Panel{Name: p.Name, URL: p.URL}
			if pc, ok := cfg.Panels[pid]; ok {
				panel.Name = p.Name
				_ = pc
			}
			if cfg.SilentMe {
				SendAlertUsers(bot, p, fresh)
			} else {
				SendAlert(bot, adminID, pg.Panel{Name: p.Name, URL: p.URL,
					Username: "", Password: ""}, fresh, false)
			}
			sent++
		}
	}
	lk := "pgdlast:days"
	if kind == "usage" {
		lk = "pgdlast:usage"
	}
	st.PutJSON(lk, map[string]any{"ts": time.Now().UnixMilli(), "count": sent}, 0)
	return sent
}

func panelName(p pg.Panel) string {
	if p.Name != "" {
		return p.Name
	}
	if p.URL != "" {
		return p.URL
	}
	return "?"
}

// userText renders a per-user notice instead of the admin digest.
func userText(panel pg.Panel, items []map[string]any) string {
	lines := []string{"🔔 سرویس «" + panelName(panel) + "»"}
	for _, it := range items {
		kind := toString(it["kind"])
		val := toString(it["value"])
		switch kind {
		case "usage":
			lines = append(lines, "⛔️ حجم سرویس شما به "+val+"٪ رسیده است.")
		case "days":
			lines = append(lines, "⏰ سرویس شما تا "+val+" روز دیگر منقضی می‌شود.")
		default:
			lines = append(lines, "• "+kind+": "+val)
		}
	}
	lines = append(lines, "", "برای تمدید با پشتیبانی تماس بگیرید.")
	return strings.Join(lines, "\n")
}

// SendAlertUsers delivers each item to that user's own Telegram id when the
// panel has one; it reports how many users were reached.
func SendAlertUsers(bot *tg.Client, p pg.Panel, items []map[string]any) int {
	if len(items) == 0 {
		return 0
	}
	c := p.Client()
	tok, err := c.Login()
	if err != nil {
		return 0
	}
	byUser := map[string][]map[string]any{}
	var order []string
	for _, it := range items {
		u := toString(it["username"])
		if u == "" {
			continue
		}
		if _, ok := byUser[u]; !ok {
			order = append(order, u)
		}
		byUser[u] = append(byUser[u], it)
	}
	sent := 0
	for _, u := range order {
		d := c.User(tok, u)
		if d == nil {
			continue
		}
		var id int64
		switch v := d["telegram_id"].(type) {
		case float64:
			id = int64(v)
		case int64:
			id = v
		case string:
			id = int64(atoiSafe(v))
		}
		if id <= 0 {
			continue
		}
		if bot.Send(id, "👤 "+u+"\n\n"+userText(p, byUser[u]), nil) != 0 {
			sent++
		}
	}
	return sent
}

func atoiSafe(s string) int {
	n := 0
	for _, r := range strings.TrimSpace(s) {
		if r < '0' || r > '9' {
			return 0
		}
		n = n*10 + int(r-'0')
	}
	return n
}
