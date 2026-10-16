package srvmon

import (
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/aknuun/cg-server/internal/store"
	"github.com/aknuun/cg-server/internal/tg"
)

var Defaults = map[string]any{
	"enabled": true, "cpuPct": 90.0, "memPct": 90.0,
	"diskPct": 90.0, "cooldownMin": 60.0,
}

func Cfg(st *store.Store) map[string]any {
	out := map[string]any{}
	for k, v := range Defaults {
		out[k] = v
	}
	var c map[string]any
	if st.GetJSON("srvmon_cfg", &c) {
		for k, v := range c {
			out[k] = v
		}
	}
	return out
}

func Save(st *store.Store, cfg map[string]any) {
	st.PutJSON("srvmon_cfg", cfg, 0)
}

// FmtGB formats a megabyte value as GB/MB.
func FmtGB(mb any) string {
	f, err := strconv.ParseFloat(strings.TrimSpace(fmt.Sprint(mb)), 64)
	if err != nil {
		return "—"
	}
	if f >= 1024 {
		return fmt.Sprintf("%.1f GB", f/1024)
	}
	return fmt.Sprintf("%d MB", int(f))
}

func num(cfg map[string]any, key string, def float64) float64 {
	switch t := cfg[key].(type) {
	case float64:
		return t
	case int:
		return float64(t)
	case string:
		f, err := strconv.ParseFloat(t, 64)
		if err == nil {
			return f
		}
	}
	return def
}

func truthy(v any) bool {
	switch t := v.(type) {
	case bool:
		return t
	case float64:
		return t != 0
	case string:
		return t == "true" || t == "1"
	}
	// default: enabled unless explicitly false
	return true
}

func Run(st *store.Store, servers []map[string]any, adminID int64,
	stats func(map[string]any) map[string]any, bot *tg.Client, manual bool) int {
	cfg := Cfg(st)
	if !truthy(cfg["enabled"]) && !manual {
		return 0
	}
	if len(servers) == 0 {
		return 0
	}
	nowMs := time.Now().UnixMilli()
	if !manual {
		last, _ := strconv.ParseInt(strings.TrimSpace(st.Get("srv_mon_last")), 10, 64)
		if last != 0 && nowMs-last < 60*60000 {
			return 0
		}
		st.Put("srv_mon_last", strconv.FormatInt(nowMs, 10), 0)
	}
	var alerts []string
	rows := [][]tg.Button{{
		{Text: "🌐 IP", Data: "noop", Style: ""},
		{Text: "⚙️ CPU", Data: "noop", Style: ""},
		{Text: "🧠 RAM", Data: "noop", Style: ""}}}
	for si, s := range servers {
		stt := stats(s)
		shortIP := substr(strv(s["host"], s["name"]), 16)
		if e := strAny(stt["error"]); e != "" {
			alerts = append(alerts, fmt.Sprintf("🔴 <code>%s</code> — خطای اتصال SSH (<code>%s</code>)", strv(s["name"]), substr(e, 120)))
			rows = append(rows, []tg.Button{
				{Text: "🔴 " + shortIP, Data: fmt.Sprintf("srvopen:%d", si)},
				{Text: "—", Data: "noop", Style: ""},
				{Text: "—", Data: "noop", Style: ""}})
			continue
		}
		mem, _ := stt["mem"].(map[string]any)
		disk, _ := stt["disk"].(map[string]any)
		cpu, _ := stt["cpu"].(map[string]any)
		memPct := 0
		if tm := intOf(mem["totalMb"]); tm != 0 {
			memPct = int(round1(float64(intOf(mem["usedMb"])) / float64(tm) * 100))
		}
		diskPct := intOf(strings.TrimRight(fmt.Sprint(disk["pct"]), "%"))
		if strings.Contains(fmt.Sprint(disk["pct"]), "—") {
			diskPct = 0
		}
		cpuPct := intOf(cpu["pct"])
		var issues []string
		if float64(cpuPct) >= num(cfg, "cpuPct", 90) {
			issues = append(issues, fmt.Sprintf("CPU %d%%", cpuPct))
		}
		if float64(memPct) >= num(cfg, "memPct", 90) {
			issues = append(issues, fmt.Sprintf("RAM %d%%", memPct))
		}
		if float64(diskPct) >= num(cfg, "diskPct", 90) {
			issues = append(issues, fmt.Sprintf("دیسک %d%%", diskPct))
		}
		dot := "🟢"
		if len(issues) > 0 {
			dot = "🟡"
		}
		rows = append(rows, []tg.Button{
			{Text: fmt.Sprintf("%s %s", dot, shortIP), Data: fmt.Sprintf("srvopen:%d", si)},
			{Text: fmt.Sprintf("%d%%", cpuPct), Data: "noop", Style: ""},
			{Text: fmt.Sprintf("%d%%", memPct), Data: "noop", Style: ""}})
		if len(issues) > 0 {
			coolKey := fmt.Sprintf("srv_cool:%s", strv(s["id"], s["host"]))
			lastA, _ := strconv.ParseInt(strings.TrimSpace(st.Get(coolKey)), 10, 64)
			coolMs := num(cfg, "cooldownMin", 60) * 60000
			if manual || lastA == 0 || float64(nowMs-lastA) > coolMs {
				ttl := int(maxf(3600, num(cfg, "cooldownMin", 60)*120))
				st.Put(coolKey, strconv.FormatInt(nowMs, 10), ttl)
				var pref []string
				for _, x := range issues {
					pref = append(pref, "⚠️ "+x)
				}
				alerts = append(alerts, fmt.Sprintf("<code>%s</code> — %s", strv(s["name"]), strings.Join(pref, " · ")))
			}
		}
	}
	if len(alerts) == 0 && !manual {
		return 0
	}
	msg := "📊 گزارش مانیتور سرورها\n\n"
	if len(alerts) > 0 {
		msg += strings.Join(alerts, "\n") + "\n"
	}
	if len(alerts) == 0 && manual {
		msg += "✅ همهٔ سرورها سالم هستند."
	}
	kb := append(rows, []tg.Button{{Text: "📊 مانیتور سرورها", Data: "srvmon"}},
		[]tg.Button{{Text: "🏠 خانه", Data: "menu"}})
	bot.Send(adminID, msg, kb)
	return len(alerts)
}

func maxf(a, b float64) float64 {
	if a > b {
		return a
	}
	return b
}

func round1(v float64) float64 { return float64(int64(v*10+0.5)) / 10 }

func strv(vals ...any) string {
	for _, v := range vals {
		if s, ok := v.(string); ok && s != "" {
			return s
		}
	}
	return ""
}

func strAny(v any) string {
	if v == nil {
		return ""
	}
	if s, ok := v.(string); ok {
		return s
	}
	return fmt.Sprint(v)
}

func intOf(v any) int {
	switch t := v.(type) {
	case float64:
		return int(t)
	case int:
		return t
	case string:
		n, _ := strconv.Atoi(strings.TrimSpace(t))
		return n
	}
	return 0
}

func substr(s string, n int) string {
	r := []rune(s)
	if len(r) > n {
		return string(r[:n])
	}
	return s
}
