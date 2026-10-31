package router

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"net"
	"net/http"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/aknuun/cg-server/internal/pg"
)

// Host Name Studio: rename/reorder PasarGuard hosts without touching anything
// but remark and priority. Full host body is always sent back to the panel.

var hsLock sync.Mutex

const (
	hsTplKey = "hs_templates"
	hsOvKey  = "hs_overrides"
	hsGrpKey = "hs_groups"
	hsBkKey  = "hs_backups"
	hsAudKey = "hs_audit"
)

type hsAnchor struct {
	Enabled bool   `json:"enabled"`
	Slot    string `json:"slot"`
	HostID  any    `json:"host_id"`
}

type hsTpl struct {
	ID             string                    `json:"id"`
	Name           string                    `json:"name"`
	Description    string                    `json:"description"`
	IsDefault      bool                      `json:"is_default"`
	Scope          string                    `json:"scope"`
	Segments       []string                  `json:"segments"`
	Variables      []string                  `json:"variables"`
	Fonts          map[string]string         `json:"fonts"`
	Emojis         map[string]string         `json:"emojis"`
	Separator      string                    `json:"separator"`
	Spacing        string                    `json:"spacing"`
	GroupOrder     []string                  `json:"group_order"`
	Anchor         hsAnchor                  `json:"anchor"`
	GroupOverrides map[string]map[string]any `json:"group_overrides"`
	HostOverrides  map[string]map[string]any `json:"host_overrides"`
	Version        int                       `json:"version"`
}

type hsOv struct {
	Panel          int    `json:"panel"`
	HostID         string `json:"host_id"`
	Excluded       bool   `json:"excluded"`
	LockedName     bool   `json:"locked_name"`
	LockedPriority bool   `json:"locked_priority"`
	CustomRemark   string `json:"custom_remark"`
	Reason         string `json:"reason"`
	Enabled        bool   `json:"enabled"`
}

type hsRow struct {
	Panel       int      `json:"panel"`
	ID          string   `json:"id"`
	Remark      string   `json:"remark"`
	New         string   `json:"new"`
	Priority    int      `json:"priority"`
	NewPriority int      `json:"newPriority"`
	Group       string   `json:"group"`
	GroupName   string   `json:"groupName"`
	Locked      bool     `json:"locked"`
	Excluded    bool     `json:"excluded"`
	Disabled    bool     `json:"disabled"`
	Changed     bool     `json:"changed"`
	Warnings    []string `json:"warnings,omitempty"`
}

type hsBackup struct {
	ID    string           `json:"id"`
	TS    int64            `json:"ts"`
	Panel int              `json:"panel"`
	Count int              `json:"count"`
	Hash  string           `json:"hash"`
	Hosts []map[string]any `json:"hosts"`
}

type hsGroupDef struct {
	ID      string `json:"id"`
	Label   string `json:"label"`
	Emoji   string `json:"emoji"`
	Service string `json:"service"`
}

var hsGroupDefs = []hsGroupDef{
	{"direct", "Direct", "🌍", "Ai"},
	{"tunnel", "IRAN", "🇮🇷", "Gemini"},
	{"fastly", "Fastly", "⚡", "Ai"},
	{"cdn", "CDN", "☁️", "Gemini"},
	{"status", "Status", "📢", "Status"},
	{"unknown", "Unknown", "❔", "Ai"},
}

// ---------- templates ----------

func hsDefaultTemplates() []hsTpl {
	mk := func(id, name, desc string, segs []string, fonts map[string]string) hsTpl {
		return hsTpl{
			ID: id, Name: name, Description: desc, Scope: "global",
			Segments: segs, Variables: []string{},
			Fonts: fonts,
			Emojis: map[string]string{
				"direct": "🌍", "tunnel": "🇮🇷", "fastly": "⚡", "cdn": "☁️", "status": "📢", "unknown": "❔",
			},
			Separator: " | ", Spacing: "spaced",
			GroupOrder:     []string{"direct", "tunnel", "fastly", "cdn", "unknown", "status"},
			Anchor:         hsAnchor{Enabled: true, Slot: "first"},
			GroupOverrides: map[string]map[string]any{}, HostOverrides: map[string]map[string]any{},
			Version: 1,
		}
	}
	three := mk("tpl_default", "سه‌بخشی پیش‌فرض", "قالب پیش‌فرض Direct/Tunnel/Fastly/CDN",
		[]string{"{flag} {group_icon} {group_name}", "{service}", "{noads}"},
		map[string]string{"0": "mono", "1": "bold", "2": "serif"})
	three.IsDefault = true
	mini := mk("tpl_minimal", "مینیمال", "فقط گروه و سرویس",
		[]string{"{flag} {group_icon} {group_name}", "{service}"},
		map[string]string{"0": "mono", "1": "bold"})
	proto := mk("tpl_proto", "پروتکل‌محور", "گروه بر اساس پروتکل",
		[]string{"{flag} {group_icon} {protocol}", "{service}", "{noads}"},
		map[string]string{"0": "mono", "1": "bold", "2": "serif"})
	user := mk("tpl_user", "یوزرنیم‌محور", "نام کاربر و حجم باقی‌مانده",
		[]string{"{flag} {group_icon} {USERNAME}", "{protocol}", "{DATA_LEFT}"},
		map[string]string{"0": "bold", "1": "mono", "2": "serif"})
	classic := mk("tpl_classic", "کلاسیک", "نام گروه، آدرس و سرویس",
		[]string{"{group_name}", "{address}:{port}", "{service}"},
		map[string]string{"0": "mono", "1": "mono", "2": "bold"})
	mono := mk("tpl_mono", "تک‌فونت / مونو", "همه بخش‌ها مونو",
		[]string{"{flag} {group_icon} {group_name}", "{service}", "{noads}"},
		map[string]string{"0": "mono", "1": "mono", "2": "mono"})
	return []hsTpl{three, mini, proto, user, classic, mono}
}

func (r *Router) hsTemplates() []hsTpl {
	var lst []hsTpl
	if !r.Store.GetJSON(hsTplKey, &lst) || len(lst) == 0 {
		lst = hsDefaultTemplates()
		r.Store.PutJSON(hsTplKey, lst, 0)
	}
	return lst
}

func (r *Router) hsTplByID(id string) (hsTpl, bool) {
	for _, t := range r.hsTemplates() {
		if t.ID == id {
			return t, true
		}
	}
	return hsTpl{}, false
}

func (r *Router) hsDefaultTpl() hsTpl {
	lst := r.hsTemplates()
	for _, t := range lst {
		if t.IsDefault {
			return t
		}
	}
	return lst[0]
}

// ---------- overrides / groups ----------

func (r *Router) hsOverrides() []hsOv {
	var lst []hsOv
	r.Store.GetJSON(hsOvKey, &lst)
	return lst
}

func (r *Router) hsOvGet(panel int, id string) *hsOv {
	for _, o := range r.hsOverrides() {
		if o.Panel == panel && o.HostID == id {
			c := o
			return &c
		}
	}
	return nil
}

func (r *Router) hsGroupsManual() map[string]string {
	m := map[string]string{}
	r.Store.GetJSON(hsGrpKey, &m)
	return m
}

// ---------- grouping ----------

func hsGroupOf(panel int, id string, h map[string]any, manual map[string]string) string {
	if g := manual[fmt.Sprintf("%d:%s", panel, id)]; g != "" {
		return g
	}
	remark := strings.ToLower(strv(h["remark"]))
	if boolVal(h["is_disabled"]) {
		return "status"
	}
	for _, k := range []string{"status", "وضعیت", "تمدید", "اخطار", "renew", "expire"} {
		if strings.Contains(remark, k) {
			return "status"
		}
	}
	addr := strings.ToLower(strings.Join(append(append(hfList(h["address"]), hfList(h["host"])...), hfList(h["sni"])...), " "))
	switch {
	case strings.Contains(addr, "fastly"):
		return "fastly"
	case strings.Contains(addr, "tun") || strings.Contains(addr, ".ir") || strings.Contains(addr, "iran"):
		return "tunnel"
	case strings.Contains(addr, "cloudflare") || strings.Contains(addr, "cdn") || strings.Contains(addr, "cd1") || strings.Contains(addr, "cd2"):
		return "cdn"
	}
	first := ""
	if a := hfList(h["address"]); len(a) > 0 {
		first = a[0]
	}
	if net.ParseIP(first) != nil {
		return "direct"
	}
	if first != "" {
		return "direct"
	}
	return "unknown"
}

func hsGroupMeta(id string) (label, emoji, service string) {
	for _, g := range hsGroupDefs {
		if g.ID == id {
			return g.Label, g.Emoji, g.Service
		}
	}
	return "Unknown", "❔", "Ai"
}

// ---------- rendering ----------

func hsFont(s, kind string) string {
	if kind == "" || kind == "sans" {
		return s
	}
	var up, low, dig rune
	switch kind {
	case "bold":
		up, low, dig = 0x1D5D4, 0x1D5EE, 0x1D7EC
	case "mono":
		up, low, dig = 0x1D670, 0x1D68A, 0x1D7F6
	case "serif":
		up, low, dig = 0x1D434, 0x1D44E, 0
	default:
		return s
	}
	var b strings.Builder
	for _, r := range s {
		switch {
		case r >= 'A' && r <= 'Z':
			b.WriteRune(up + (r - 'A'))
		case r >= 'a' && r <= 'z':
			if kind == "serif" && r == 'h' {
				b.WriteRune('ℎ')
			} else {
				b.WriteRune(low + (r - 'a'))
			}
		case r >= '0' && r <= '9' && dig != 0:
			b.WriteRune(dig + (r - '0'))
		default:
			b.WriteRune(r)
		}
	}
	return b.String()
}

func hsLeadingFlag(s string) string {
	rs := []rune(strings.TrimSpace(s))
	for i := 0; i < len(rs)-1; i++ {
		if rs[i] >= 0x1F1E6 && rs[i] <= 0x1F1FF && rs[i+1] >= 0x1F1E6 && rs[i+1] <= 0x1F1FF {
			return string(rs[i : i+2])
		}
		if i > 3 {
			break
		}
	}
	return ""
}

var hsAllowedVars = map[string]bool{
	"flag": true, "location_flag": true, "iran_flag": true, "group_icon": true,
	"group_name": true, "protocol": true, "transport": true, "inbound_tag": true,
	"address": true, "port": true, "path": true, "sni": true, "USERNAME": true,
	"STATUS_EMOJI": true, "TIME_LEFT": true, "DATA_LEFT": true, "service": true,
	"noads": true, "priority": true, "host_id": true,
}

func hsRender(t hsTpl, h map[string]any, group, oldRemark string) (string, []string) {
	label, icon, service := hsGroupMeta(group)
	if e := t.Emojis[group]; e != "" {
		icon = e
	}
	addr := strings.Join(hfList(h["address"]), ", ")
	sni := strings.Join(hfList(h["sni"]), ", ")
	proto := strv(h["inbound_tag"])
	if proto == "" {
		proto = strv(h["security"])
	}
	if proto == "" {
		proto = "manual"
	}
	vars := map[string]string{
		"flag": hsLeadingFlag(oldRemark), "location_flag": hsLeadingFlag(oldRemark),
		"iran_flag": "🇮🇷", "group_icon": icon, "group_name": label,
		"protocol": proto, "transport": strv(h["transport"]), "inbound_tag": strv(h["inbound_tag"]),
		"address": addr, "port": strv(h["port"]), "path": strv(h["path"]), "sni": sni,
		"USERNAME": "{USERNAME}", "STATUS_EMOJI": "{STATUS_EMOJI}",
		"TIME_LEFT": "{TIME_LEFT}", "DATA_LEFT": "{DATA_LEFT}",
		"service": service, "noads": "NoAds",
		"priority": strv(h["priority"]), "host_id": strv(h["id"]),
	}
	var warns []string
	seen := map[string]bool{}
	sub := func(s string) string {
		out := s
		for k, v := range vars {
			out = strings.ReplaceAll(out, "{"+k+"}", v)
		}
		// report unknown placeholders without dropping what the user wrote
		for i := 0; i < len(out); i++ {
			if out[i] != '{' {
				continue
			}
			j := strings.IndexByte(out[i:], '}')
			if j < 0 {
				break
			}
			name := out[i+1 : i+j]
			if !hsAllowedVars[name] && !seen[name] {
				seen[name] = true
				warns = append(warns, "متغیر ناشناخته: {"+name+"}")
			}
			i += j
		}
		return out
	}
	sep := t.Separator
	if sep == "" {
		sep = " | "
	}
	parts := make([]string, 0, len(t.Segments))
	for i, seg := range t.Segments {
		v := sub(seg)
		if strings.TrimSpace(v) == "" {
			continue
		}
		if t.Spacing == "compact" {
			v = strings.TrimSpace(v)
		}
		if f := t.Fonts[strconv.Itoa(i)]; f != "" {
			v = hsFont(v, f)
		}
		parts = append(parts, v)
	}
	out := strings.Join(parts, sep)
	if t.Spacing == "compact" {
		out = strings.TrimSpace(strings.ReplaceAll(out, " "+strings.TrimSpace(sep)+" ", strings.TrimSpace(sep)))
	}
	return out, warns
}

// ---------- plan ----------

func (r *Router) hsPanelHosts(panel int) ([]map[string]any, *pg.Pg, string, error) {
	panels := r.panels()
	if panel < 0 || panel >= len(panels) {
		return nil, nil, "", fmt.Errorf("پنل پیدا نشد")
	}
	c := panels[panel].Client()
	tok, err := c.Login()
	if err != nil {
		return nil, nil, "", fmt.Errorf("ورود به پنل ناموفق")
	}
	raw, err := c.Hosts(tok)
	if err != nil {
		return nil, nil, "", fmt.Errorf("خواندن هاست‌ها ناموفق")
	}
	out := make([]map[string]any, 0, len(raw))
	for _, hv := range raw {
		if h, ok := hv.(map[string]any); ok {
			out = append(out, h)
		}
	}
	return out, c, tok, nil
}

type hsPlanIn struct {
	Panel    int      `json:"panel"`
	Template string   `json:"template"`
	IDs      []string `json:"ids"`
	Group    string   `json:"group"`
	Mode     string   `json:"mode"`
	Force    bool     `json:"force"`
}

func (r *Router) hsPlan(in hsPlanIn) ([]hsRow, []map[string]any, error) {
	t, ok := r.hsTplByID(in.Template)
	if !ok {
		t = r.hsDefaultTpl()
	}
	hosts, _, _, err := r.hsPanelHosts(in.Panel)
	if err != nil {
		return nil, nil, err
	}
	manual := r.hsGroupsManual()
	ovMap := map[string]*hsOv{}
	for _, o := range r.hsOverrides() {
		if o.Panel == in.Panel {
			c := o
			ovMap[o.HostID] = &c
		}
	}
	want := map[string]bool{}
	for _, id := range in.IDs {
		want[id] = true
	}
	anchorID := ""
	if t.Anchor.Enabled {
		anchorID = strv(t.Anchor.HostID)
	}
	order := map[string]int{}
	for i, g := range t.GroupOrder {
		order[g] = i
	}
	rows := make([]hsRow, 0, len(hosts))
	body := map[string]map[string]any{}
	for _, h := range hosts {
		id := strv(h["id"])
		body[id] = h
		group := hsGroupOf(in.Panel, id, h, manual)
		old := strv(h["remark"])
		ov := ovMap[id]
		row := hsRow{
			Panel: in.Panel, ID: id, Remark: old, Group: group,
			Priority: intOrAny(h["priority"]), Disabled: boolVal(h["is_disabled"]),
		}
		row.GroupName, _, _ = hsGroupMeta(group)
		row.NewPriority = row.Priority
		row.New = old
		if ov != nil && ov.Enabled {
			row.Excluded = ov.Excluded
			row.Locked = ov.LockedName || ov.LockedPriority
		}
		if id == anchorID {
			row.Locked = true
		}
		if ov != nil && ov.Enabled && strings.TrimSpace(ov.CustomRemark) != "" {
			row.New = ov.CustomRemark
		} else if in.Mode != "order" {
			nv, warns := hsRender(t, h, group, old)
			row.New = nv
			row.Warnings = warns
		}
		rows = append(rows, row)
	}
	// ordering: group order, then current priority
	sort.SliceStable(rows, func(i, j int) bool {
		gi, gj := order[rows[i].Group], order[rows[j].Group]
		if gi != gj {
			return gi < gj
		}
		return rows[i].Priority < rows[j].Priority
	})
	if in.Mode != "name" && len(want) > 0 {
		// subset apply: permute the existing priority slots of the selected hosts
		// so no priority collides with hosts outside the selection
		vals := []int{}
		for i := range rows {
			if want[rows[i].ID] && !rows[i].Locked {
				vals = append(vals, rows[i].Priority)
			}
		}
		sort.Ints(vals)
		k := 0
		for i := range rows {
			if !want[rows[i].ID] || rows[i].Locked {
				continue
			}
			rows[i].NewPriority = vals[k]
			k++
		}
	} else if in.Mode != "name" {
		used := map[int]bool{}
		for i := range rows {
			if rows[i].Locked {
				rows[i].NewPriority = rows[i].Priority
				if rows[i].ID == anchorID {
					rows[i].NewPriority = 0
					if t.Anchor.Slot == "second" {
						rows[i].NewPriority = 10
					}
				}
				used[rows[i].NewPriority] = true
			}
		}
		next := 0
		for i := range rows {
			if rows[i].Locked {
				rows[i].NewPriority = rows[i].Priority
				continue
			}
			for used[next] {
				next += 10
			}
			rows[i].NewPriority = next
			used[next] = true
			next += 10
		}
	}
	// filters: selection, group, exemptions
	for i := range rows {
		row := &rows[i]
		skip := false
		if len(want) > 0 && !want[row.ID] {
			skip = true
		}
		if in.Group != "" && in.Group != "all" && row.Group != in.Group {
			skip = true
		}
		if row.Group == "status" && !in.Force {
			skip = true
			row.Excluded = true
		}
		if row.Excluded && !in.Force {
			skip = true
		}
		if ov := ovMap[row.ID]; ov != nil && ov.Enabled && !in.Force {
			if ov.LockedName {
				row.New = row.Remark
			}
			if ov.LockedPriority {
				row.NewPriority = row.Priority
			}
		}
		if skip {
			row.New = row.Remark
			row.NewPriority = row.Priority
		}
		row.Changed = row.New != row.Remark || row.NewPriority != row.Priority
	}
	return rows, hosts, nil
}

// ---------- handlers ----------

func (r *Router) webToolsStudio(w http.ResponseWriter, req *http.Request, uid int64) {
	panel := intOrAny(req.URL.Query().Get("panel"))
	panels := r.panels()
	out := map[string]any{
		"panels":    []map[string]any{},
		"templates": r.hsTemplates(),
		"hosts":     []hsRow{},
		"overrides": r.hsOverrides(),
		"backups":   r.hsBackupsLite(),
		"audit":     r.hsAudit(),
		"groups":    hsGroupDefs,
	}
	pl := make([]map[string]any, 0, len(panels))
	for i, p := range panels {
		pl = append(pl, map[string]any{"idx": i, "name": p.Name, "url": p.URL})
	}
	if len(panels) == 0 {
		webOK(w, out)
		return
	}
	out["panels"] = pl
	if panel < 0 || panel >= len(panels) {
		panel = 0
	}
	hosts, _, _, err := r.hsPanelHosts(panel)
	if err != nil {
		out["error"] = err.Error()
		webOK(w, out)
		return
	}
	manual := r.hsGroupsManual()
	rows := make([]hsRow, 0, len(hosts))
	for _, h := range hosts {
		id := strv(h["id"])
		g := hsGroupOf(panel, id, h, manual)
		row := hsRow{Panel: panel, ID: id, Remark: strv(h["remark"]), Group: g, Priority: intOrAny(h["priority"]), Disabled: boolVal(h["is_disabled"])}
		row.GroupName, _, _ = hsGroupMeta(g)
		if ov := r.hsOvGet(panel, id); ov != nil && ov.Enabled {
			row.Excluded = ov.Excluded
			row.Locked = ov.LockedName || ov.LockedPriority
		}
		rows = append(rows, row)
	}
	out["hosts"] = rows
	out["panel"] = panel
	webOK(w, out)
}

func (r *Router) webStudioPreview(w http.ResponseWriter, req *http.Request, uid int64) {
	var in hsPlanIn
	if !webDecode(w, req, &in) {
		return
	}
	if in.Mode == "" {
		in.Mode = "both"
	}
	rows, _, err := r.hsPlan(in)
	if err != nil {
		webErr(w, http.StatusBadGateway, err.Error())
		return
	}
	changed := 0
	for _, x := range rows {
		if x.Changed {
			changed++
		}
	}
	webOK(w, map[string]any{"rows": rows, "changed": changed})
}

func (r *Router) webStudioApply(w http.ResponseWriter, req *http.Request, uid int64) {
	var in struct {
		hsPlanIn
		Confirm string `json:"confirm"`
	}
	if !webDecode(w, req, &in) {
		return
	}
	if strings.TrimSpace(in.Confirm) != "APPLY" {
		webErr(w, http.StatusBadRequest, "برای اعمال باید APPLY را تایپ کنی")
		return
	}
	if in.Mode == "" {
		in.Mode = "both"
	}
	hsLock.Lock()
	defer hsLock.Unlock()

	rows, hosts, err := r.hsPlan(in.hsPlanIn)
	if err != nil {
		webErr(w, http.StatusBadGateway, err.Error())
		return
	}
	bk := r.hsBackupNew(in.Panel, hosts)
	_, c, tok, err := r.hsPanelHosts(in.Panel)
	if err != nil {
		webErr(w, http.StatusBadGateway, err.Error())
		return
	}
	orig := map[string]map[string]any{}
	for _, h := range hosts {
		orig[strv(h["id"])] = h
	}
	done := make([]string, 0)
	fail := ""
	for _, row := range rows {
		if !row.Changed {
			continue
		}
		body := map[string]any{}
		for k, v := range orig[row.ID] {
			body[k] = v
		}
		body["remark"] = row.New
		body["priority"] = row.NewPriority
		if err := c.PutHost(tok, row.ID, body); err != nil {
			fail = row.ID
			break
		}
		done = append(done, row.ID)
	}
	if fail != "" {
		for i := len(done) - 1; i >= 0; i-- {
			_ = c.PutHost(tok, done[i], orig[done[i]])
		}
		r.hsAuditAdd(uid, in.Panel, in.Template, len(done), false, "خطا در "+fail+" و بازگردانی کامل")
		webErr(w, http.StatusBadGateway, "اعمال ناموفق بود؛ همهٔ تغییرات برگشت داده شد")
		return
	}
	r.hsAuditAdd(uid, in.Panel, in.Template, len(done), true, "")
	webOK(w, map[string]any{"changed": len(done), "backup": bk.ID})
}

func (r *Router) hsBackupsLite() []map[string]any {
	var lst []hsBackup
	r.Store.GetJSON(hsBkKey, &lst)
	out := make([]map[string]any, 0, len(lst))
	for _, b := range lst {
		out = append(out, map[string]any{"id": b.ID, "ts": b.TS, "panel": b.Panel, "count": b.Count, "hash": b.Hash})
	}
	return out
}

func (r *Router) hsBackupNew(panel int, hosts []map[string]any) hsBackup {
	var lst []hsBackup
	r.Store.GetJSON(hsBkKey, &lst)
	raw, _ := json.Marshal(hosts)
	sum := sha256.Sum256(raw)
	b := hsBackup{
		ID: fmt.Sprintf("bk%d", time.Now().UnixNano()), TS: time.Now().UnixMilli(),
		Panel: panel, Count: len(hosts), Hash: hex.EncodeToString(sum[:]), Hosts: hosts,
	}
	lst = append([]hsBackup{b}, lst...)
	if len(lst) > 10 {
		lst = lst[:10]
	}
	r.Store.PutJSON(hsBkKey, lst, 0)
	return b
}

func (r *Router) hsAudit() []map[string]any {
	var lst []map[string]any
	r.Store.GetJSON(hsAudKey, &lst)
	return lst
}

func (r *Router) hsAuditAdd(uid int64, panel int, tpl string, n int, ok bool, note string) {
	lst := r.hsAudit()
	entry := map[string]any{
		"ts": time.Now().UnixMilli(), "uid": uid, "panel": panel,
		"template": tpl, "hosts": n, "ok": ok,
	}
	if note != "" {
		entry["note"] = note
	}
	lst = append([]map[string]any{entry}, lst...)
	if len(lst) > 200 {
		lst = lst[:200]
	}
	r.Store.PutJSON(hsAudKey, lst, 0)
}

func (r *Router) webStudioBackup(w http.ResponseWriter, req *http.Request, uid int64) {
	var in struct {
		Op string `json:"op"`
		ID string `json:"id"`
	}
	if !webDecode(w, req, &in) {
		return
	}
	if in.Op == "new" {
		panel := 0
		hosts, _, _, err := r.hsPanelHosts(panel)
		if err != nil {
			webErr(w, http.StatusBadGateway, err.Error())
			return
		}
		b := r.hsBackupNew(panel, hosts)
		webOK(w, map[string]any{"id": b.ID, "hash": b.Hash})
		return
	}
	if in.Op != "restore" {
		webErr(w, http.StatusBadRequest, "عملیات نامعتبر")
		return
	}
	var lst []hsBackup
	r.Store.GetJSON(hsBkKey, &lst)
	var b *hsBackup
	for i := range lst {
		if lst[i].ID == in.ID {
			b = &lst[i]
			break
		}
	}
	if b == nil {
		webErr(w, http.StatusNotFound, "بکاپ پیدا نشد")
		return
	}
	hsLock.Lock()
	defer hsLock.Unlock()
	_, c, tok, err := r.hsPanelHosts(b.Panel)
	if err != nil {
		webErr(w, http.StatusBadGateway, err.Error())
		return
	}
	n := 0
	for _, h := range b.Hosts {
		if err := c.PutHost(tok, strv(h["id"]), h); err != nil {
			webErr(w, http.StatusBadGateway, "بازگردانی ناموفق: "+strv(h["id"]))
			return
		}
		n++
	}
	r.hsAuditAdd(uid, b.Panel, "restore", n, true, "restore "+b.ID)
	webOK(w, map[string]any{"restored": n})
}

func (r *Router) webStudioTpl(w http.ResponseWriter, req *http.Request, uid int64) {
	var in struct {
		Op        string  `json:"op"`
		ID        string  `json:"id"`
		Template  *hsTpl  `json:"template"`
		Templates []hsTpl `json:"templates"`
	}
	if !webDecode(w, req, &in) {
		return
	}
	lst := r.hsTemplates()
	switch in.Op {
	case "save":
		if in.Template == nil {
			webErr(w, http.StatusBadRequest, "قالب خالی است")
			return
		}
		t := *in.Template
		if len(t.Segments) == 0 {
			t.Segments = []string{"{flag} {group_icon} {group_name}", "{service}", "{noads}"}
		}
		if t.Separator == "" {
			t.Separator = " | "
		}
		if t.GroupOrder == nil {
			t.GroupOrder = []string{"direct", "tunnel", "fastly", "cdn", "unknown", "status"}
		}
		if t.ID == "" {
			t.ID = fmt.Sprintf("tpl_%d", time.Now().UnixNano())
			t.Version = 1
		} else {
			t.Version++
		}
		found := false
		for i := range lst {
			if lst[i].ID == t.ID {
				t.IsDefault = lst[i].IsDefault
				lst[i] = t
				found = true
			}
		}
		if !found {
			lst = append(lst, t)
		}
	case "del":
		out := make([]hsTpl, 0, len(lst))
		for _, t := range lst {
			if t.ID != in.ID {
				out = append(out, t)
			}
		}
		if len(out) == 0 {
			webErr(w, http.StatusBadRequest, "حداقل یک قالب لازم است")
			return
		}
		any := false
		for i := range out {
			if out[i].IsDefault {
				any = true
			}
		}
		if !any {
			out[0].IsDefault = true
		}
		lst = out
	case "dup":
		for _, t := range lst {
			if t.ID == in.ID {
				c := t
				c.ID = fmt.Sprintf("tpl_%d", time.Now().UnixNano())
				c.Name = t.Name + " (کپی)"
				c.IsDefault = false
				c.Version = 1
				lst = append(lst, c)
			}
		}
	case "default":
		for i := range lst {
			lst[i].IsDefault = lst[i].ID == in.ID
		}
	case "import":
		for _, t := range in.Templates {
			if t.ID == "" {
				t.ID = fmt.Sprintf("tpl_%d", time.Now().UnixNano())
			}
			t.IsDefault = false
			lst = append(lst, t)
		}
	case "reset":
		lst = hsDefaultTemplates()
	default:
		webErr(w, http.StatusBadRequest, "عملیات نامعتبر")
		return
	}
	r.Store.PutJSON(hsTplKey, lst, 0)
	webOK(w, map[string]any{"templates": lst})
}

func (r *Router) webStudioOverride(w http.ResponseWriter, req *http.Request, uid int64) {
	var in struct {
		Op             string `json:"op"`
		Panel          int    `json:"panel"`
		HostID         string `json:"host_id"`
		Excluded       bool   `json:"excluded"`
		LockedName     bool   `json:"locked_name"`
		LockedPriority bool   `json:"locked_priority"`
		CustomRemark   string `json:"custom_remark"`
		Reason         string `json:"reason"`
		Enabled        bool   `json:"enabled"`
		Group          string `json:"group"`
	}
	if !webDecode(w, req, &in) {
		return
	}
	lst := r.hsOverrides()
	out := make([]hsOv, 0, len(lst)+1)
	for _, o := range lst {
		if !(o.Panel == in.Panel && o.HostID == in.HostID) {
			out = append(out, o)
		}
	}
	if in.Op != "del" && in.HostID != "" {
		out = append(out, hsOv{
			Panel: in.Panel, HostID: in.HostID, Excluded: in.Excluded,
			LockedName: in.LockedName, LockedPriority: in.LockedPriority,
			CustomRemark: in.CustomRemark, Reason: in.Reason, Enabled: in.Enabled,
		})
	}
	r.Store.PutJSON(hsOvKey, out, 0)
	// manual group change is part of the same editor
	if in.Op == "group" {
		m := r.hsGroupsManual()
		key := fmt.Sprintf("%d:%s", in.Panel, in.HostID)
		if in.Group != "" {
			m[key] = in.Group
		} else {
			delete(m, key)
		}
		r.Store.PutJSON(hsGrpKey, m, 0)
	}
	webOK(w, map[string]any{"overrides": out})
}
