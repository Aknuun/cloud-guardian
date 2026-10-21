package router

import (
	"fmt"
	"math/rand"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/aknuun/cg-server/internal/cf"
	"github.com/aknuun/cg-server/internal/jalali"
)

var hfdAtRe = regexp.MustCompile(`^([01]\d|2[0-3]):[0-5]\d$`)

type hfdBase struct {
	Base     string `json:"base"`
	ZoneName string `json:"zoneName"`
}

type hfdItem struct {
	Fields    map[string]bool     `json:"fields"`
	Style     string              `json:"style"`
	Sep       string              `json:"sep"`
	Prefix    string              `json:"prefix"`
	Counters  map[string]int      `json:"counters"`
	Base      map[string]hfdBase  `json:"base"`
	Prev      map[string][]string `json:"prev"`
	Current   map[string][]string `json:"current"`
	LastBase  map[string]hfdBase  `json:"lastBase"`
	RefSub    string              `json:"refSub"`
	LastRun   string              `json:"last_run"`
	LastNames []string            `json:"last_names"`
}

type hfdCfg struct {
	Enabled      bool               `json:"enabled"`
	IntervalH    int                `json:"intervalH"`
	At           string             `json:"at"`
	NextRun      string             `json:"next_run"`
	TotalCreated int                `json:"totalCreated"`
	Items        map[string]hfdItem `json:"items"`
}

func hfdNewItem() hfdItem {
	return hfdItem{
		Fields:   map[string]bool{"address": false, "sni": false, "host": false},
		Style:    "num",
		Sep:      "-",
		Counters: map[string]int{"address": 1, "sni": 1, "host": 1},
		Base:     map[string]hfdBase{},
		Prev:     map[string][]string{},
		Current:  map[string][]string{},
		LastBase: map[string]hfdBase{},
	}
}

func (r *Router) hfdCfg() hfdCfg {
	c := hfdCfg{IntervalH: 24, At: "03:00", Items: map[string]hfdItem{}}
	_ = r.Store.GetJSON("dailyrot_cfg", &c)
	if c.IntervalH < 1 || c.IntervalH > 720 {
		c.IntervalH = 24
	}
	if !hfdAtRe.MatchString(c.At) {
		c.At = "03:00"
	}
	if c.Items == nil {
		c.Items = map[string]hfdItem{}
	}
	return c
}

func (r *Router) hfdCfgSave(c hfdCfg) { r.Store.PutJSON("dailyrot_cfg", c, 0) }

func hfdKey(pi int, hid string) string { return fmt.Sprintf("%d:%s", pi, hid) }

func hfdKeyParse(key string) (int, string) {
	i := strings.IndexByte(key, ':')
	if i < 0 {
		return -1, ""
	}
	return intOrAny(key[:i]), key[i+1:]
}

func hfdAtOK(s string) bool { return hfdAtRe.MatchString(s) }

// hfdNextRunISO computes the next anchor time (Tehran) as RFC3339.
func hfdNextRunISO(at string, intervalH int, nowMs int64) string {
	hh, mm := 3, 0
	if hfdAtRe.MatchString(at) {
		fmt.Sscanf(at, "%d:%d", &hh, &mm)
	}
	if intervalH < 1 || intervalH > 720 {
		intervalH = 24
	}
	const off = int64(3*3600+30*60) * 1000
	const day = int64(86400000)
	step := int64(intervalH) * 3600000
	tNow := nowMs + off
	dayStart := (tNow / day) * day
	cand := dayStart + int64(hh*60+mm)*60000 - off
	for cand <= nowMs {
		cand += step
	}
	return time.UnixMilli(cand).UTC().Format(time.RFC3339)
}

func hfdAlpha(n int) string {
	if n < 1 {
		n = 1
	}
	s := ""
	x := n
	for x > 0 {
		r := (x - 1) % 26
		s = string(rune('a'+r)) + s
		x = (x - 1) / 26
	}
	return s
}

func hfdRandStr(n int) string {
	const abc = "abcdefghijklmnopqrstuvwxyz0123456789"
	b := make([]byte, n)
	for i := range b {
		b[i] = abc[rand.Intn(len(abc))]
	}
	return string(b)
}

func hfdNextSuffix(style string, counter int) (string, int) {
	c := counter
	if c < 1 {
		c = 1
	}
	switch style {
	case "alpha":
		return hfdAlpha(c), c + 1
	case "mix":
		return hfdRandStr(4), c + 1
	default:
		return strconv.Itoa(c), c + 1
	}
}

type hfdZone struct {
	cli  *cf.Client
	zone cf.Zone
}

func (r *Router) hfdZones() []hfdZone {
	var out []hfdZone
	for _, a := range r.cfAccs() {
		cli := cf.New(a.Token)
		zs, err := cli.Zones()
		if err != nil {
			continue
		}
		for _, z := range zs {
			if z.Status == "active" {
				out = append(out, hfdZone{cli, z})
			}
		}
	}
	return out
}

func hfdZoneFor(zs []hfdZone, name string) *hfdZone {
	low := strings.ToLower(name)
	var best *hfdZone
	for i := range zs {
		zn := strings.ToLower(zs[i].zone.Name)
		if zn == "" {
			continue
		}
		if low == zn || strings.HasSuffix(low, "."+zn) {
			if best == nil || len(zn) > len(best.zone.Name) {
				best = &zs[i]
			}
		}
	}
	return best
}

// hfdResolve returns the base label and zone for a live value.
func hfdResolve(val string, zs []hfdZone) (base, zoneName string, z *hfdZone, ok bool) {
	v := strings.ToLower(strings.TrimSuffix(strings.TrimSpace(val), "."))
	if v == "" || ipRe2.MatchString(v) {
		return "", "", nil, false
	}
	z = hfdZoneFor(zs, v)
	if z == nil {
		return "", "", nil, false
	}
	zn := strings.ToLower(z.zone.Name)
	rel := ""
	if v != zn {
		rel = v[:len(v)-(len(zn)+1)]
	}
	label0 := strings.Split(rel, ".")[0]
	if label0 == "" {
		label0 = "sub"
	}
	return label0, z.zone.Name, z, true
}

var ipRe2 = regexp.MustCompile(`^\d{1,3}(\.\d{1,3}){3}$`)

func hfdFreeName(names map[string]bool, base, sep, zoneName, style string, counter int) (string, int, bool) {
	c := counter
	if c < 1 {
		c = 1
	}
	for i := 0; i < 200; i++ {
		suf, nx := hfdNextSuffix(style, c)
		cand := strings.ToLower(base + sep + suf + "." + zoneName)
		if !names[cand] {
			return cand, nx, true
		}
		if style != "mix" {
			c = nx
		}
	}
	return "", c, false
}

type hfdRecCache map[string][]cf.Record

func (rc hfdRecCache) get(z *hfdZone) []cf.Record {
	if recs, ok := rc[z.zone.ID]; ok {
		return recs
	}
	recs, err := z.cli.Records(z.zone.ID)
	if err != nil {
		recs = nil
	}
	rc[z.zone.ID] = recs
	return recs
}

func hfdCreateRecords(z *hfdZone, name string, src []cf.Record, old string) error {
	if len(src) > 0 {
		for _, rec := range src {
			ttl := intOrAny(rec.TTL)
			if ttl <= 0 {
				ttl = 1
			}
			if _, err := z.cli.AddRecord(z.zone.ID, rec.Type, name, rec.Content, ttl, boolVal(rec.Proxied), nil); err != nil {
				return err
			}
		}
		return nil
	}
	_, err := z.cli.AddRecord(z.zone.ID, "CNAME", name, old, 1, false, nil)
	return err
}

func hfReality(host map[string]any) bool {
	hay := strings.ToLower(strv(host["inbound_tag"]) + " " + strv(host["remark"]) + " " + strv(host["security"]))
	return strings.Contains(hay, "reality")
}

func (r *Router) hfdLogAdd(entry map[string]any) {
	var log []map[string]any
	r.Store.GetJSON("host_filter_log", &log)
	if _, ok := entry["ts"]; !ok {
		entry["ts"] = time.Now().UTC().Format(time.RFC3339)
	}
	log = append(log, entry)
	if len(log) > 60 {
		log = log[len(log)-60:]
	}
	r.Store.PutJSON("host_filter_log", log, 0)
}

// hfdRotate runs one pass. force ignores enabled/next_run.
func (r *Router) hfdRotate(force bool) (int, int, []string) {
	cfg := r.hfdCfg()
	if !cfg.Enabled && !force {
		return 0, 0, nil
	}
	nowMS := time.Now().UnixMilli()
	if !force {
		if cfg.NextRun == "" {
			cfg.NextRun = hfdNextRunISO(cfg.At, cfg.IntervalH, nowMS)
			r.hfdCfgSave(cfg)
			return 0, 0, nil
		}
		if t, err := time.Parse(time.RFC3339, cfg.NextRun); err == nil && t.UnixMilli() > nowMS {
			return 0, 0, nil
		}
	}
	if len(cfg.Items) == 0 {
		r.hfdAdvanceNext(&cfg, nowMS)
		r.hfdCfgSave(cfg)
		return 0, 0, nil
	}
	panels := r.panels()
	zones := r.hfdZones()
	recCache := hfdRecCache{}
	tokens := map[int]string{}
	changed, skipped := 0, 0
	var lines []string

	for key, it := range cfg.Items {
		pi, hid := hfdKeyParse(key)
		if pi < 0 || pi >= len(panels) {
			r.hfdLogAdd(map[string]any{"kind": "dailyrot", "key": key, "error": "panel_gone"})
			skipped++
			continue
		}
		panel := panels[pi]
		c := panel.Client()
		tok, ok := tokens[pi]
		if !ok {
			t, err := c.Login()
			if err != nil {
				r.hfdLogAdd(map[string]any{"kind": "dailyrot", "key": key, "error": "login_failed"})
				skipped++
				continue
			}
			tok = t
			tokens[pi] = t
		}
		hosts, err := c.Hosts(tok)
		if err != nil {
			r.hfdLogAdd(map[string]any{"kind": "dailyrot", "key": key, "error": "fetch_failed"})
			skipped++
			continue
		}
		var host map[string]any
		for _, hv := range hosts {
			h, _ := hv.(map[string]any)
			if strv(h["id"]) == hid {
				host = h
				break
			}
		}
		if host == nil {
			r.hfdLogAdd(map[string]any{"kind": "dailyrot", "key": key, "error": "host_gone"})
			skipped++
			continue
		}
		if boolVal(host["is_disabled"]) {
			r.hfdLogAdd(map[string]any{"kind": "dailyrot", "key": key, "error": "host_disabled"})
			skipped++
			continue
		}
		reality := hfReality(host)
		if it.Fields == nil {
			it.Fields = map[string]bool{}
		}
		if it.Counters == nil {
			it.Counters = map[string]int{}
		}
		if it.Base == nil {
			it.Base = map[string]hfdBase{}
		}
		if it.Prev == nil {
			it.Prev = map[string][]string{}
		}
		if it.Current == nil {
			it.Current = map[string][]string{}
		}
		if it.LastBase == nil {
			it.LastBase = map[string]hfdBase{}
		}
		sep := it.Sep
		if sep == "" {
			sep = "-"
		}
		style := it.Style
		if style == "" {
			style = "num"
		}
		var events []map[string]any
		for _, field := range []string{"address", "sni", "host"} {
			if !it.Fields[field] {
				continue
			}
			if reality && field != "address" {
				r.hfdLogAdd(map[string]any{"kind": "dailyrot", "key": key, "field": field, "error": "reality_sni_protected"})
				continue
			}
			cur := selStrings(host[field])
			out := make([]string, 0, len(cur))
			for _, val := range cur {
				base, zname, z, ok := hfdResolve(val, zones)
				if !ok {
					r.hfdLogAdd(map[string]any{"kind": "dailyrot", "key": key, "field": field, "from": val, "error": "external_zone"})
					out = append(out, val)
					continue
				}
				stored := it.LastBase[field]
				if stored.Base == "" {
					stored = it.Base[field]
				}
				if stored.Base != "" {
					low := strings.ToLower(val)
					sb := strings.ToLower(stored.Base)
					if low == sb || strings.HasPrefix(low, sb+sep) {
						base = stored.Base
						if stored.ZoneName != "" {
							zname = stored.ZoneName
						}
					} else {
						it.Counters[field] = 1
					}
				}
				if it.Prefix != "" {
					base = it.Prefix
				}
				recs := recCache.get(z)
				names := map[string]bool{}
				for _, rec := range recs {
					names[strings.ToLower(rec.Name)] = true
				}
				newName, counter, ok := hfdFreeName(names, base, sep, zname, style, it.Counters[field])
				if !ok {
					r.hfdLogAdd(map[string]any{"kind": "dailyrot", "key": key, "field": field, "from": val, "error": "no_free_name"})
					out = append(out, val)
					continue
				}
				var src []cf.Record
				for _, rec := range recs {
					if strings.EqualFold(rec.Name, val) &&
						(rec.Type == "A" || rec.Type == "AAAA" || rec.Type == "CNAME") {
						src = append(src, rec)
					}
				}
				if err := hfdCreateRecords(z, newName, src, val); err != nil {
					r.hfdLogAdd(map[string]any{"kind": "dailyrot", "key": key, "field": field, "from": val, "error": "create_fail: " + truncRune(err.Error(), 100)})
					out = append(out, val)
					continue
				}
				it.Counters[field] = counter
				it.LastBase[field] = hfdBase{Base: base, ZoneName: zname}
				pv := append([]string{val}, it.Prev[field]...)
				if len(pv) > 3 {
					pv = pv[:3]
				}
				it.Prev[field] = pv
				out = append(out, newName)
				events = append(events, map[string]any{"field": field, "from": val, "to": newName})
				cfg.TotalCreated++
			}
			host[field] = out
			it.Current[field] = out
		}
		if len(events) == 0 {
			cfg.Items[key] = it
			skipped++
			continue
		}
		if err := c.PutHost(tok, strv(host["id"]), host); err != nil {
			r.hfdLogAdd(map[string]any{"kind": "dailyrot", "key": key, "error": "put_fail: " + truncRune(err.Error(), 100)})
			cfg.Items[key] = it
			continue
		}
		changed++
		it.LastRun = time.Now().UTC().Format(time.RFC3339)
		var lns []string
		for _, ev := range events {
			lines = append(lines, fmt.Sprintf("• %s: %s ➜ %s", strv(ev["field"]), strv(ev["from"]), strv(ev["to"])))
			lns = append(lns, strv(ev["field"])+":"+strv(ev["to"]))
		}
		it.LastNames = lns
		cfg.Items[key] = it
		r.hfdLogAdd(map[string]any{"kind": "dailyrot", "key": key, "remark": strv(host["remark"]), "events": events})
	}
	r.hfdAdvanceNext(&cfg, nowMS)
	r.hfdCfgSave(cfg)
	return changed, skipped, lines
}

// HfdTick runs the daily rotation when due and notifies admins.
func (r *Router) HfdTick() {
	cfg := r.hfdCfg()
	if !cfg.Enabled {
		return
	}
	nowMS := time.Now().UnixMilli()
	if cfg.NextRun == "" {
		cfg.NextRun = hfdNextRunISO(cfg.At, cfg.IntervalH, nowMS)
		r.hfdCfgSave(cfg)
		return
	}
	if t, err := time.Parse(time.RFC3339, cfg.NextRun); err == nil && t.UnixMilli() > nowMS {
		return
	}
	changed, skipped, lines := r.hfdRotate(false)
	if changed == 0 && len(lines) == 0 {
		return
	}
	msg := fmt.Sprintf("🔁 چرخش خودکار ساب‌ها\n✅ %d هاست چرخید", changed)
	if skipped > 0 {
		msg += fmt.Sprintf(" · ⏭ %d رد شد", skipped)
	}
	if len(lines) > 0 {
		if len(lines) > 25 {
			lines = lines[:25]
		}
		msg += "\n\n" + strings.Join(lines, "\n")
	}
	for _, t := range r.Admins() {
		r.Bot.Send(t, msg, [][]Btn{{btn("🔁 چرخش روزانه", "hfd")}})
	}
}

func (r *Router) hfdAdvanceNext(cfg *hfdCfg, nowMS int64) {
	step := int64(cfg.IntervalH) * 3600000
	if step < 3600000 {
		step = 24 * 3600000
	}
	nxt := nowMS
	if cfg.NextRun != "" {
		if t, err := time.Parse(time.RFC3339, cfg.NextRun); err == nil {
			nxt = t.UnixMilli()
		}
	}
	for nxt <= nowMS {
		nxt += step
	}
	cfg.NextRun = time.UnixMilli(nxt).UTC().Format(time.RFC3339)
}

// ---------- UI ----------

func (r *Router) hfdHome(chat, mid int64) {
	cfg := r.hfdCfg()
	status := "⏸ غیرفعال"
	if cfg.Enabled {
		status = "▶️ فعال"
	}
	next := "—"
	if cfg.NextRun != "" {
		if t, err := time.Parse(time.RFC3339, cfg.NextRun); err == nil {
			next = jalali.Fmt(t.UnixMilli()) + " به وقت ایران"
		}
	}
	toggle := "▶️ فعال‌سازی"
	if cfg.Enabled {
		toggle = "⏸ غیرفعال‌سازی"
	}
	lines := []string{"🔁 چرخش روزانه ساب‌ها", "",
		"وضعیت: " + status,
		fmt.Sprintf("⏱ هر %d ساعت · 🕐 لنگر: %s (تهران)", cfg.IntervalH, cfg.At),
		"⏭ اجرای بعدی: " + next,
		fmt.Sprintf("🖥 هاست‌های منتخب: %d · 🆕 ساب ساخته‌شده: %d", len(cfg.Items), cfg.TotalCreated),
		"",
		"روش: هر اجرا، فیلدهای فعال هر هاست منتخب یک ساب‌دامنهٔ تازه می‌گیرند؛ ساب قبلی از DNS حذف نمی‌شود و «برگشت به قبلی» هم هست. در هاست REALITY فقط address می‌چرخد."}
	kb := [][]Btn{
		{btn(toggle, "hfdtg")},
		{btn("⏱ بازه", "hfdint"), btn("🕐 ساعت لنگر", "hfdat")},
		{btn("🖥 هاست‌ها", "hfdhosts"), btn("⚡ اجرای دستی", "hfdrun")},
		{btn("📜 گزارش", "hfdlog"), btn("↩️ بازگردانی", "hfdrvlist")},
		{btn("🔙 بازگشت", "hf"), btn("🏠 خانه", "menu")},
	}
	r.Show(chat, mid, strings.Join(lines, "\n"), kb)
}

func (r *Router) hfdToggle(chat, mid int64) {
	cfg := r.hfdCfg()
	cfg.Enabled = !cfg.Enabled
	if cfg.Enabled && cfg.NextRun == "" {
		cfg.NextRun = hfdNextRunISO(cfg.At, cfg.IntervalH, time.Now().UnixMilli())
	}
	r.hfdCfgSave(cfg)
	r.hfdHome(chat, mid)
}

func (r *Router) hfdIntervalMenu(chat, mid int64) {
	cfg := r.hfdCfg()
	var row []Btn
	for _, h := range []int{12, 24, 48} {
		mark := ""
		if cfg.IntervalH == h {
			mark = "✅ "
		}
		row = append(row, btn(fmt.Sprintf("%sهر %d ساعت", mark, h), fmt.Sprintf("hfdintv:%d", h)))
	}
	kb := [][]Btn{row,
		{btn("✏️ ساعت دلخواه", "hfdintc")},
		{btn("🔙 بازگشت", "hfd")}}
	r.Show(chat, mid, fmt.Sprintf("⏱ بازهٔ چرخش (فعلی: هر %d ساعت):", cfg.IntervalH), kb)
}

func (r *Router) hfdIntervalSet(chat, mid int64, h int) {
	if h < 1 || h > 720 {
		r.Show(chat, mid, "⚠️ بازه نامعتبر (۱ تا ۷۲۰).", navKB("hfd"))
		return
	}
	cfg := r.hfdCfg()
	cfg.IntervalH = h
	cfg.NextRun = hfdNextRunISO(cfg.At, cfg.IntervalH, time.Now().UnixMilli())
	r.hfdCfgSave(cfg)
	r.hfdHome(chat, mid)
}

func (r *Router) hfdAtAsk(chat, mid int64) {
	cfg := r.hfdCfg()
	r.wizStart(chat, mid, map[string]any{"kind": "hfd_at"})
	r.Show(chat, mid, fmt.Sprintf("🕐 ساعت لنگر را به وقت تهران بفرست (HH:MM مثل 03:00؛ فعلی: %s):", cfg.At),
		[][]Btn{{btn("⬅️ انصراف", "hfd")}})
}

func (r *Router) hfdAtSave(chat, mid int64, pend map[string]any, text string) {
	r.clearWizard(chat)
	a := strings.TrimSpace(jalali.ParseFA(text))
	if !hfdAtOK(a) {
		r.Show(chat, mid, "⚠️ قالب ساعت نامعتبر (مثل 03:00).", navKB("hfd"))
		return
	}
	cfg := r.hfdCfg()
	cfg.At = a
	cfg.NextRun = hfdNextRunISO(cfg.At, cfg.IntervalH, time.Now().UnixMilli())
	r.hfdCfgSave(cfg)
	r.hfdHome(chat, mid)
}

func (r *Router) hfdIntAsk(chat, mid int64) {
	r.wizStart(chat, mid, map[string]any{"kind": "hfd_int"})
	r.Show(chat, mid, "⏱ بازه را به ساعت بفرست (۱ تا ۷۲۰):", [][]Btn{{btn("⬅️ انصراف", "hfd")}})
}

func (r *Router) hfdIntSave(chat, mid int64, pend map[string]any, text string) {
	r.clearWizard(chat)
	h := intOrAny(strings.TrimSpace(jalali.ParseFA(text)))
	if h < 1 || h > 720 {
		r.Show(chat, mid, "⚠️ بازه نامعتبر (۱ تا ۷۲۰).", navKB("hfd"))
		return
	}
	r.hfdIntervalSet(chat, mid, h)
}

func (r *Router) hfdHostsHome(chat, mid int64) {
	panels := r.panels()
	if len(panels) == 0 {
		r.Show(chat, mid, "📭 پنلی ثبت نشده.", navKB("hfd"))
		return
	}
	var items []Btn
	for i, p := range panels {
		items = append(items, btnS("🖥 "+pname(p), fmt.Sprintf("hfdpanel:%d", i), "plain"))
	}
	kb := gridBTN(items, 2)
	kb = append(kb, navRow("hfd"))
	r.Show(chat, mid, "🖥 انتخاب هاست‌ها برای چرخش\n\nیک پنل را انتخاب کن:", kb)
}

func (r *Router) hfdPanel(chat, mid int64, pi int) {
	panels := r.panels()
	if pi < 0 || pi >= len(panels) {
		r.Show(chat, mid, "❌ پنل پیدا نشد.", HomeKB())
		return
	}
	panel := panels[pi]
	c := panel.Client()
	tok, err := c.Login()
	if err != nil {
		r.Show(chat, mid, "⚠️ ورود به پنل ناموفق.", navKB("hfdhosts"))
		return
	}
	hosts, err := c.Hosts(tok)
	if err != nil {
		r.Show(chat, mid, "⚠️ خواندن هاست‌ها ناموفق.", navKB("hfdhosts"))
		return
	}
	cfg := r.hfdCfg()
	lines := []string{fmt.Sprintf("🖥 هاست‌های «%s»", pname(panel)), "", "دکمه‌های A/S/H = address/sni/host"}
	var kb [][]Btn
	n := 0
	for _, hv := range hosts {
		h, ok := hv.(map[string]any)
		if !ok || boolVal(h["is_disabled"]) {
			continue
		}
		if n >= 25 {
			break
		}
		n++
		hid := strv(h["id"])
		it, has := cfg.Items[hfdKey(pi, hid)]
		mark := func(f string) string {
			if has && it.Fields[f] {
				return "✅"
			}
			return "▫️"
		}
		kb = append(kb, []Btn{
			btn(truncRune(strv(h["remark"]), 20), "noop"),
			btn("A"+mark("address"), fmt.Sprintf("hfdhf:%d:%s:address", pi, hid)),
			btn("S"+mark("sni"), fmt.Sprintf("hfdhf:%d:%s:sni", pi, hid)),
			btn("H"+mark("host"), fmt.Sprintf("hfdhf:%d:%s:host", pi, hid)),
		})
	}
	if n == 0 {
		lines = append(lines, "📭 هاستی نیست.")
	}
	kb = append(kb, []Btn{btn("🔙 بازگشت", "hfdhosts"), btn("🏠 خانه", "menu")})
	r.Show(chat, mid, strings.Join(lines, "\n"), kb)
}

func (r *Router) hfdHostField(chat, mid int64, pi int, hid, field string) {
	if field != "address" && field != "sni" && field != "host" {
		return
	}
	cfg := r.hfdCfg()
	key := hfdKey(pi, hid)
	it, ok := cfg.Items[key]
	if !ok {
		it = hfdNewItem()
	}
	if it.Fields == nil {
		it.Fields = map[string]bool{}
	}
	it.Fields[field] = !it.Fields[field]
	if !it.Fields["address"] && !it.Fields["sni"] && !it.Fields["host"] {
		delete(cfg.Items, key)
	} else {
		cfg.Items[key] = it
	}
	r.hfdCfgSave(cfg)
	r.hfdPanel(chat, mid, pi)
}

func (r *Router) hfdRunNow(chat, mid int64) {
	r.Show(chat, mid, "⏳ در حال چرخش ساب‌ها…", navKB(""))
	changed, skipped, lines := r.hfdRotate(true)
	msg := fmt.Sprintf("✅ %d هاست چرخید", changed)
	if skipped > 0 {
		msg += fmt.Sprintf(" · ⏭ %d رد شد", skipped)
	}
	if len(lines) > 0 {
		if len(lines) > 20 {
			lines = lines[:20]
		}
		msg += "\n\n" + strings.Join(lines, "\n")
	}
	r.Show(chat, mid, msg, navKB("hfd"))
}

func (r *Router) hfdLogView(chat, mid int64) {
	var log []map[string]any
	r.Store.GetJSON("host_filter_log", &log)
	lines := []string{"📜 گزارش چرخش روزانه", ""}
	cnt := 0
	for i := len(log) - 1; i >= 0 && cnt < 15; i-- {
		e := log[i]
		if strv(e["kind"]) != "dailyrot" {
			continue
		}
		cnt++
		ts := hfBkTime(strv(e["ts"]))
		if rerr := strv(e["error"]); rerr != "" {
			lines = append(lines, fmt.Sprintf("• %s — ⚠️ %s (%s)", ts, rerr, strv(e["remark"])))
			continue
		}
		lines = append(lines, fmt.Sprintf("• %s — ✅ %s", ts, strv(e["remark"])))
	}
	if cnt == 0 {
		lines = append(lines, "📭 چیزی ثبت نشده.")
	}
	r.Show(chat, mid, strings.Join(lines, "\n"), navKB("hfd"))
}

func (r *Router) hfdRevertList(chat, mid int64) {
	cfg := r.hfdCfg()
	lines := []string{"↩️ بازگردانی چرخش روزانه", ""}
	var kb [][]Btn
	var rvItems []Btn
	for key, it := range cfg.Items {
		has := false
		for _, f := range []string{"address", "sni", "host"} {
			if len(it.Prev[f]) > 0 {
				has = true
			}
		}
		if !has {
			continue
		}
		pi, hid := hfdKeyParse(key)
		lines = append(lines, "• "+key)
		rvItems = append(rvItems, btn("↩️ "+key, fmt.Sprintf("hfdrv:%d:%s", pi, hid)))
	}
	kb = gridBTN(rvItems, 2)
	if len(rvItems) == 0 {
		lines = append(lines, "📭 چیزی برای بازگردانی نیست.")
	}
	kb = append(kb, navRow("hfd"))
	r.Show(chat, mid, strings.Join(lines, "\n"), kb)
}

func (r *Router) hfdRevert(chat, mid int64, pi int, hid string) {
	panels := r.panels()
	if pi < 0 || pi >= len(panels) {
		r.Show(chat, mid, "❌ پنل پیدا نشد.", HomeKB())
		return
	}
	cfg := r.hfdCfg()
	key := hfdKey(pi, hid)
	it, ok := cfg.Items[key]
	if !ok {
		r.Show(chat, mid, "❌ موردی نیست.", navKB("hfdrvlist"))
		return
	}
	panel := panels[pi]
	c := panel.Client()
	tok, err := c.Login()
	if err != nil {
		r.Show(chat, mid, "⚠️ ورود ناموفق.", navKB("hfdrvlist"))
		return
	}
	hosts, err := c.Hosts(tok)
	if err != nil {
		r.Show(chat, mid, "⚠️ خواندن هاست‌ها ناموفق.", navKB("hfdrvlist"))
		return
	}
	var host map[string]any
	for _, hv := range hosts {
		h, _ := hv.(map[string]any)
		if strv(h["id"]) == hid {
			host = h
			break
		}
	}
	if host == nil {
		r.Show(chat, mid, "❌ هاست پیدا نشد.", navKB("hfdrvlist"))
		return
	}
	fields := []string{}
	for _, f := range []string{"address", "sni", "host"} {
		if pv := it.Prev[f]; len(pv) > 0 {
			host[f] = []string{pv[len(pv)-1]}
			it.Prev[f] = pv[:len(pv)-1]
			fields = append(fields, f)
		}
	}
	if len(fields) == 0 {
		r.Show(chat, mid, "📭 چیزی برای بازگردانی نیست.", navKB("hfdrvlist"))
		return
	}
	if err := c.PutHost(tok, strv(host["id"]), host); err != nil {
		r.Show(chat, mid, "❌ بازگردانی ناموفق: "+Esc(err.Error()), navKB("hfdrvlist"))
		return
	}
	cfg.Items[key] = it
	r.hfdCfgSave(cfg)
	r.hfdLogAdd(map[string]any{"kind": "revert", "key": key, "remark": strv(host["remark"])})
	r.Show(chat, mid, "✅ بازگردانی انجام شد: "+strings.Join(fields, "، "),
		navKB("hfdrvlist"))
}
