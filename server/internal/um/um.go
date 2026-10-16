// Package um is the bad-consumption ("usage monitor") detector.
package um

import (
	"strconv"
	"time"

	"github.com/aknuun/cg-server/internal/jalali"
	"github.com/aknuun/cg-server/internal/pg"
	"github.com/aknuun/cg-server/internal/store"
	"github.com/aknuun/cg-server/internal/tg"
)

const (
	MinMS          = 60 * 60000
	RecentKeep     = 6
	RecentWindowMS = 70 * 60000
	DayMinMS       = 23 * 3600000
	DayMaxMS       = 48 * 3600000
	ReportMS       = 24 * 3600000
)

type Config struct {
	Enabled   bool    `json:"enabled"`
	HourlyGb  float64 `json:"hourlyGb"`
	DailyGb   float64 `json:"dailyGb"`
	CooldownH float64 `json:"cooldownH"`
	ActiveMin float64 `json:"activeMin"`
	MinGb     float64 `json:"minGb"`
	Report    bool    `json:"report"`
}

func defaults() Config {
	return Config{Enabled: true, HourlyGb: 7, DailyGb: 25,
		CooldownH: 6, ActiveMin: 75, MinGb: 1, Report: true}
}

func Cfg(st *store.Store) Config {
	c := defaults()
	var m map[string]any
	if st.GetJSON("usage_monitor_cfg", &m) {
		if v, ok := m["enabled"].(bool); ok {
			c.Enabled = v
		}
		if v, ok := m["report"].(bool); ok {
			c.Report = v
		}
		c.HourlyGb = fget(m, "hourlyGb", c.HourlyGb)
		c.DailyGb = fget(m, "dailyGb", c.DailyGb)
		c.CooldownH = fget(m, "cooldownH", c.CooldownH)
		c.ActiveMin = fget(m, "activeMin", c.ActiveMin)
		c.MinGb = fget(m, "minGb", c.MinGb)
	}
	return c
}

func SaveCfg(st *store.Store, c Config) { st.PutJSON("usage_monitor_cfg", c, 0) }

func fget(m map[string]any, k string, dflt float64) float64 {
	switch v := m[k].(type) {
	case float64:
		return v
	case int:
		return float64(v)
	case string:
		if f, err := strconv.ParseFloat(v, 64); err == nil {
			return f
		}
	}
	return dflt
}

type sample struct {
	T int64
	U map[string]float64
}

func findSample(recent []sample, target int64, maxDiff int64) *sample {
	var best *sample
	var bestDiff int64 = -1
	for i := range recent {
		d := recent[i].T - target
		if d < 0 {
			d = -d
		}
		if d <= maxDiff && (bestDiff < 0 || d < bestDiff) {
			best = &recent[i]
			bestDiff = d
		}
	}
	return best
}

func pattern(recent []sample, uid string) bool {
	var vals []float64
	for _, s := range recent {
		if v, ok := s.U[uid]; ok {
			vals = append(vals, v)
		}
	}
	if len(vals) < 3 {
		return false
	}
	active := 0
	for i := 1; i < len(vals); i++ {
		if vals[i]-vals[i-1] > 0 {
			active++
		}
	}
	need := 4
	if len(vals)-1 < need {
		need = len(vals) - 1
	}
	return active >= need
}

// Run scans panels for heavy consumers; returns alerts sent.
func Run(bot *tg.Client, st *store.Store, panels []pg.Panel, adminID int64) int {
	cfg := Cfg(st)
	if !cfg.Enabled {
		return 0
	}
	now := time.Now().UnixMilli()
	last := i64(st.Get("um_last_run"))
	if last != 0 && now-last < MinMS {
		return 0
	}
	st.Put("um_last_run", strconv.FormatInt(now, 10), 0)
	if len(panels) == 0 {
		return 0
	}
	since := time.Now().UTC().Add(-time.Duration(cfg.ActiveMin) * time.Minute)
	sinceISO := since.Format(time.RFC3339)
	hourTh := cfg.HourlyGb * 1024 * 1024 * 1024
	dayTh := cfg.DailyGb * 1024 * 1024 * 1024
	coolMS := cfg.CooldownH * 3600000
	minB := cfg.MinGb * 1024 * 1024 * 1024
	repTS := i64(st.Get("um_report_ts"))
	reportDue := cfg.Report && repTS > 0 && now-repTS >= ReportMS
	sent := 0
	for _, p := range panels {
		pid := p.URL
		c := p.Client()
		tok, err := c.Login()
		if err != nil {
			continue
		}
		users, err := c.UsersSince(tok, sinceISO)
		if err != nil || len(users) == 0 {
			continue
		}
		state := loadState(st, pid)
		dayAge := int64(0)
		dayReady := false
		if state.Day != nil {
			dayAge = now - state.Day.T
			dayReady = dayAge >= DayMinMS && dayAge <= 30*3600000
		}
		cur := map[string]float64{}
		for _, u := range users {
			used := fval(u["used"])
			if minB > 0 && used < minB {
				continue
			}
			cur[strv(u["id"])] = used
		}
		hourOld := findSample(state.Recent, now-60000, 35000)
		type alert struct {
			username string
			owner    string
			gb       float64
			label    string
			flat     bool
		}
		var alerts []alert
		for _, u := range users {
			uid := strv(u["id"])
			used := fval(u["used"])
			var best *struct {
				gb    float64
				label string
			}
			if hourOld != nil {
				if old, ok := hourOld.U[uid]; ok {
					d := used - old
					if d > 0 && d >= hourTh {
						best = &struct {
							gb    float64
							label string
						}{d / 1024 / 1024 / 1024, "ساعت اخیر"}
					}
				}
			}
			if dayReady && state.Day != nil {
				if old, ok := state.Day.U[uid]; ok {
					d := used - old
					if d > 0 && d >= dayTh {
						g := d / 1024 / 1024 / 1024
						if best == nil || g > best.gb {
							best = &struct {
								gb    float64
								label string
							}{g, "۲۴ ساعت اخیر"}
						}
					}
				}
			}
			if best == nil {
				continue
			}
			if coolMS > 0 {
				if prev, ok := state.Cool[uid]; ok && float64(now-prev) < coolMS {
					continue
				}
			}
			state.Cool[uid] = now
			flat := pattern(state.Recent, uid)
			logAppend(st, map[string]any{"t": now, "panel": p.Name,
				"username": strv(u["username"]), "owner": strv(u["owner"]),
				"gb": best.gb, "label": best.label})
			alerts = append(alerts, alert{strv(u["username"]), strv(u["owner"]), best.gb, best.label, flat})
		}
		if len(alerts) > 0 {
			for i := 0; i < len(alerts); i++ {
				for j := i + 1; j < len(alerts); j++ {
					if alerts[j].gb > alerts[i].gb {
						alerts[i], alerts[j] = alerts[j], alerts[i]
					}
				}
			}
			lines := []string{"🚨 مصرف بالا (احتمال پخش لینک)",
				"🕐 " + jalali.Fmt(now), ""}
			limit := len(alerts)
			if limit > 30 {
				limit = 30
			}
			for _, a := range alerts[:limit] {
				lines = append(lines, "▪️ <code>"+a.username+"</code> — "+
					fmtGB(a.gb)+" GB ("+a.label+")")
				pat := "مصرف جهشی ⚡"
				if a.flat {
					pat = "مصرف پیوسته 🔁"
				}
				lines = append(lines, "   ادمین: "+a.owner+" • "+pat)
			}
			if len(alerts) > 30 {
				lines = append(lines, "… و "+strconv.Itoa(len(alerts)-30)+" کاربر دیگر")
			}
			bot.Send(adminID, join(lines), nil)
			sent++
		}
		state.Recent = append(state.Recent, sample{T: now, U: cur})
		var keep []sample
		for _, s := range state.Recent {
			if now-s.T <= RecentWindowMS {
				keep = append(keep, s)
			}
		}
		if len(keep) > RecentKeep {
			keep = keep[len(keep)-RecentKeep:]
		}
		state.Recent = keep
		if state.Day == nil || dayReady || dayAge > DayMaxMS {
			state.Day = &sample{T: now, U: cur}
		}
		for uid, ts := range state.Cool {
			if now-ts > 3*24*3600000 {
				delete(state.Cool, uid)
			}
		}
		saveState(st, pid, state)
	}
	if reportDue {
		runReport(bot, st, adminID, now)
	} else if repTS == 0 {
		st.Put("um_report_ts", strconv.FormatInt(now, 10), 0)
	}
	return sent
}

func runReport(bot *tg.Client, st *store.Store, adminID, now int64) {
	var log []map[string]any
	st.GetJSON("um_alerts_log", &log)
	type row struct {
		username, owner, panel, label string
		gb                            float64
	}
	byUser := map[string]*row{}
	for _, w := range log {
		t := i64of(w["t"])
		if t < now-ReportMS {
			continue
		}
		un := strv(w["username"])
		g := fval(w["gb"])
		prev := byUser[un]
		if prev == nil {
			byUser[un] = &row{un, strv(w["owner"]), strv(w["panel"]), strv(w["label"]), g}
			continue
		}
		if strv(w["label"]) == "۲۴ ساعت اخیر" {
			prev.gb, prev.label = g, strv(w["label"])
			if o := strv(w["owner"]); o != "" {
				prev.owner = o
			}
			if pn := strv(w["panel"]); pn != "" {
				prev.panel = pn
			}
		} else if g > prev.gb && prev.label != "۲۴ ساعت اخیر" {
			prev.gb, prev.label = g, strv(w["label"])
		}
	}
	arr := make([]*row, 0, len(byUser))
	for _, v := range byUser {
		arr = append(arr, v)
	}
	for i := 0; i < len(arr); i++ {
		for j := i + 1; j < len(arr); j++ {
			if arr[j].gb > arr[i].gb {
				arr[i], arr[j] = arr[j], arr[i]
			}
		}
	}
	if len(arr) == 0 {
		st.Put("um_report_ts", strconv.FormatInt(now, 10), 0)
		return
	}
	if len(arr) > 30 {
		arr = arr[:30]
	}
	lines := []string{"📊 گزارش پرمصرف‌های اخطارشده (۲۴ ساعت اخیر)",
		"🕐 " + jalali.Fmt(now), ""}
	for i, t := range arr {
		lines = append(lines, strconv.Itoa(i+1)+". <code>"+t.username+"</code> — "+
			fmtGB(t.gb)+" GB ("+t.label+")")
		owner := t.owner
		if owner == "" {
			owner = "—"
		}
		panel := t.panel
		if panel == "" {
			panel = "—"
		}
		lines = append(lines, "   ادمین: "+owner+" • 🖥 "+panel)
	}
	bot.Send(adminID, join(lines), nil)
	st.Put("um_report_ts", strconv.FormatInt(now, 10), 0)
}

// ---------- state ----------

type umState struct {
	Recent []sample
	Day    *sample
	Cool   map[string]int64
}

func loadState(st *store.Store, pid string) umState {
	s := umState{Cool: map[string]int64{}}
	var raw map[string]any
	if !st.GetJSON("um:"+pid, &raw) {
		return s
	}
	if rl, ok := raw["recent"].([]any); ok {
		for _, it := range rl {
			if m, ok := it.(map[string]any); ok {
				s.Recent = append(s.Recent, sample{T: i64of(m["t"]), U: toUMap(m["u"])})
			}
		}
	}
	if dm, ok := raw["day"].(map[string]any); ok {
		s.Day = &sample{T: i64of(dm["t"]), U: toUMap(dm["u"])}
	}
	if cm, ok := raw["cool"].(map[string]any); ok {
		for k, v := range cm {
			s.Cool[k] = i64of(v)
		}
	}
	return s
}

func saveState(st *store.Store, pid string, s umState) {
	recent := []any{}
	for _, r := range s.Recent {
		u := map[string]any{}
		for k, v := range r.U {
			u[k] = v
		}
		recent = append(recent, map[string]any{"t": r.T, "u": u})
	}
	var day any
	if s.Day != nil {
		u := map[string]any{}
		for k, v := range s.Day.U {
			u[k] = v
		}
		day = map[string]any{"t": s.Day.T, "u": u}
	}
	cool := map[string]any{}
	for k, v := range s.Cool {
		cool[k] = v
	}
	st.PutJSON("um:"+pid, map[string]any{"recent": recent, "day": day, "cool": cool}, 0)
}
