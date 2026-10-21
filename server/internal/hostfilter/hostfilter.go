// Package hostfilter ports the automatic filtered-host swap (check-host.net +
// Cloudflare clone + panel host repoint).
package hostfilter

import (
	"bytes"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/url"
	"regexp"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/aknuun/cg-server/internal/cf"
	"github.com/aknuun/cg-server/internal/pg"
	"github.com/aknuun/cg-server/internal/store"
	"github.com/aknuun/cg-server/internal/tg"
)

const checkHost = "https://check-host.net"

var httpUA = "Mozilla/5.0 (Windows NT 10.0; Win64; x64)"

type node struct{ ID, City string }

var irNodes = []node{
	{"ir1.node.check-host.net", "Tehran"},
	{"ir5.node.check-host.net", "Tehran"},
	{"ir7.node.check-host.net", "Tehran"},
	{"ir8.node.check-host.net", "Tehran"},
	{"ir2.node.check-host.net", "Isfahan"},
	{"ir3.node.check-host.net", "Shiraz"},
	{"ir4.node.check-host.net", "Shiraz"},
}

var irCities = []string{"Tehran", "Isfahan", "Shiraz"}

var foreignNodes = []node{
	{"de2.node.check-host.net", "Frankfurt"},
	{"de1.node.check-host.net", "Nuremberg"},
	{"nl1.node.check-host.net", "Amsterdam"},
	{"nl2.node.check-host.net", "Meppel"},
}

// ipDownFails is the minimum foreign probe failures that mean the IP is down.
const ipDownFails = 4

var runLock sync.Mutex

// Cfg mirrors host_filter_cfg in the Python store.
type Cfg struct {
	Enabled       *bool    `json:"enabled"`
	IntervalMin   int      `json:"intervalMin"`
	Cities        int      `json:"cities"`
	CitiesSel     []string `json:"citiesSel"`
	MaxOk         int      `json:"maxOk"`
	RecheckCount  int      `json:"recheckCount"`
	RecheckMin    int      `json:"recheckMin"`
	Exceptions    []string `json:"exceptions"`
	Batch         int      `json:"batch"`
	MaxChanges    int      `json:"maxChanges"`
	BackupKeep    int      `json:"backupKeep"`
	RealityRotate bool     `json:"realityRotate"`
	LastRun       string   `json:"last_run"`
	LastSummary   string   `json:"last_summary"`
}

func (c Cfg) IsEnabled() bool { return c.Enabled == nil || *c.Enabled }

var (
	domRe = regexp.MustCompile(`(?i)^[a-z0-9][a-z0-9.-]*\.[a-z]{2,}$`)
	ipRe  = regexp.MustCompile(`^\d{1,3}(\.\d{1,3}){3}$`)
)

func GetCfg(st *store.Store) Cfg {
	en := true
	c := Cfg{Enabled: &en, IntervalMin: 25, Cities: 2,
		CitiesSel: append([]string{}, irCities...), MaxOk: 0,
		RecheckCount: 1, RecheckMin: 1, Batch: 60, MaxChanges: 8, BackupKeep: 5}
	var raw map[string]any
	if st.GetJSON("host_filter_cfg", &raw) {
		if v, ok := raw["enabled"].(bool); ok {
			c.Enabled = &v
		}
		c.IntervalMin = intOr(raw["intervalMin"], c.IntervalMin)
		c.Cities = intOr(raw["cities"], c.Cities)
		c.MaxOk = intOr(raw["maxOk"], c.MaxOk)
		c.RecheckCount = intOr(raw["recheckCount"], c.RecheckCount)
		c.RecheckMin = intOr(raw["recheckMin"], c.RecheckMin)
		c.Batch = intOr(raw["batch"], c.Batch)
		c.MaxChanges = intOr(raw["maxChanges"], c.MaxChanges)
		c.BackupKeep = intOr(raw["backupKeep"], c.BackupKeep)
		if v, ok := raw["realityRotate"].(bool); ok {
			c.RealityRotate = v
		}
		if v, ok := raw["last_run"].(string); ok {
			c.LastRun = v
		}
		if v, ok := raw["last_summary"].(string); ok {
			c.LastSummary = v
		}
		if arr, ok := raw["citiesSel"].([]any); ok {
			var sel []string
			for _, x := range arr {
				if s, ok := x.(string); ok {
					sel = append(sel, s)
				}
			}
			c.CitiesSel = sel
		}
		if arr, ok := raw["exceptions"].([]any); ok {
			c.Exceptions = nil
			for _, x := range arr {
				if s, ok := x.(string); ok {
					c.Exceptions = append(c.Exceptions, s)
				}
			}
		}
	}
	sel := []string{}
	for _, x := range c.CitiesSel {
		if contains(irCities, x) {
			sel = append(sel, x)
		}
	}
	if len(sel) == 0 {
		sel = append([]string{}, irCities...)
	}
	c.CitiesSel = sel
	return c
}

func SaveCfg(st *store.Store, c Cfg) {
	cur := GetCfg(st)
	a, _ := json.Marshal(c)
	b, _ := json.Marshal(cur)
	if string(a) != string(b) {
		st.PutJSON("host_filter_cfg", c, 0)
	}
}

// Result summarizes a run.
type Result struct {
	Checked int    `json:"checked"`
	Changed int    `json:"changed"`
	Msg     string `json:"msg"`
}

// Run performs a full pass. It never panics; failures land in Msg.
func Run(bot *tg.Client, st *store.Store, panels []pg.Panel, adminID int64,
	cfAccs []cf.Account, force bool, only string) (res Result) {
	if !runLock.TryLock() {
		return Result{Msg: "run in progress"}
	}
	defer runLock.Unlock()
	defer func() {
		if e := recover(); e != nil {
			res.Msg = trunc(fmt.Sprint(e), 200)
		}
	}()
	return doRun(bot, st, panels, adminID, cfAccs, force, only)
}

type pingRes struct {
	City  string
	Ok    int
	Total int
}

func doRun(bot *tg.Client, st *store.Store, panels []pg.Panel, adminID int64,
	cfAccs []cf.Account, force bool, only string) Result {
	res := Result{}
	cfg := GetCfg(st)
	nowMS := time.Now().UnixMilli()
	if !cfg.IsEnabled() && !force {
		return res
	}
	if !force {
		last := parseISO(cfg.LastRun)
		if last != 0 && nowMS-last < int64(max(1, cfg.IntervalMin))*60000 {
			return res
		}
	}

	type pair struct {
		panel   pg.Panel
		token   string
		host    map[string]any
		domains []string
	}
	var pairs []pair
	for pi, p := range panels {
		c := p.Client()
		token, err := c.Login()
		if err != nil {
			continue
		}
		hosts, err := c.Hosts(token)
		if err != nil {
			continue
		}
		for _, hv := range hosts {
			h, ok := hv.(map[string]any)
			if !ok {
				continue
			}
			if b, ok := h["is_disabled"].(bool); ok && b {
				continue
			}
			hid := strv(h["id"])
			// Exception keys are stored by the UI as "<panelIndex>:<hostID>[:<field>]"
			// (see hfAllKeys). Also accept the legacy "<panelURL>:<hostID>" form.
			base := fmt.Sprintf("%d:%s", pi, hid)
			legacyBase := p.URL + ":" + hid
			excSet := map[string]bool{}
			for _, f := range []string{"host", "address", "sni"} {
				if contains(cfg.Exceptions, base+":"+f) || contains(cfg.Exceptions, base) ||
					contains(cfg.Exceptions, legacyBase+":"+f) || contains(cfg.Exceptions, legacyBase) {
					excSet[f] = true
				}
			}
			var allowed []string
			for _, f := range []string{"address", "sni", "host"} {
				if excSet[f] {
					continue
				}
				for _, x := range asList(h[f]) {
					if d := domOf(x); d != "" {
						allowed = append(allowed, d)
					}
				}
			}
			allowed = sortedSet(allowed)
			if len(allowed) > 0 {
				pairs = append(pairs, pair{panel: p, token: token, host: h, domains: allowed})
			}
		}
	}

	latches := map[string]int64{}
	st.GetJSON("host_filter_latch", &latches)
	latchChanged := false

	changed, checked := 0, 0
	for _, item := range pairs {
		if checked >= cfg.Batch {
			break
		}
		for _, dv := range item.domains {
			if checked >= cfg.Batch {
				break
			}
			if only != "" && only != dv {
				continue
			}
			checked++
			ping := checkPing(dv, nodesFor(cfg.CitiesSel))
			bc := blockedCities(ping, cfg.CitiesSel, cfg.MaxOk)
			if bc < cfg.Cities {
				continue
			}
			resolved := resolveHost(dv)
			if len(resolved) == 0 {
				fmt.Printf("hf: %s blocked=%d but no DNS resolve, skip\n", dv, bc)
				continue
			}
			fmt.Printf("hf: %s blocked=%d resolved=%v -> rechecking\n", dv, bc, resolved)
			ok := true
			for i := 0; i < cfg.RecheckCount; i++ {
				time.Sleep(time.Duration(max(1, cfg.RecheckMin)) * time.Minute)
				ping2 := checkPing(dv, nodesFor(cfg.CitiesSel))
				if blockedCities(ping2, cfg.CitiesSel, cfg.MaxOk) < cfg.Cities {
					ok = false
					break
				}
			}
			if !ok {
				fmt.Printf("hf: %s recheck says not blocked, skip\n", dv)
				continue
			}
			diag := diagIP(resolved[0], cfg)
			if diag.Handled {
				fmt.Printf("hf: %s ip-diagnosis handled ip=%s, skip swap\n", dv, diag.IP)
				kind := "blocked"
				if diag.IPDown {
					kind = "down"
				} else if diag.IranAccess {
					kind = "iran"
				}
				lk := kind + "|" + diag.IP
				last := latches[lk]
				fresh := last > 0 && nowMS-last < 24*3600*1000
				if force || !fresh {
					notify(bot, adminID, ipDiagMsg(panelName(item.panel), item.host, dv, diag))
					latches[lk] = nowMS
					latchChanged = true
				} else {
					fmt.Printf("hf: %s suppressed diag notice (latch %s)\n", dv, lk)
				}
				continue
			}
			if changed >= cfg.MaxChanges {
				break
			}
			if swapHost(bot, st, item.panel, item.token, item.host, dv, cfAccs, adminID) {
				changed++
			}
		}
		if changed >= cfg.MaxChanges {
			break
		}
	}
	cfg.LastRun = time.Now().UTC().Format(time.RFC3339Nano)
	cfg.LastSummary = fmt.Sprintf("بررسی %d دامنه — %d تعویض.", checked, changed)
	SaveCfg(st, cfg)
	if latchChanged {
		st.PutJSON("host_filter_latch", latches, 0)
	}
	res.Checked = checked
	res.Changed = changed
	return res
}

func nodesFor(citiesSel []string) []node {
	var out []node
	for _, n := range irNodes {
		if contains(citiesSel, n.City) {
			out = append(out, n)
		}
	}
	return out
}

// checkPing asks check-host.net for ICMP results and falls back to Globalping
// for every node that check-host did not report (service down, rate-limited or
// empty samples).
func checkPing(host string, nodes []node) map[string]pingRes {
	out := checkPingCH(host, nodes)
	if len(nodes) == 0 {
		return out
	}
	var missing []node
	for _, n := range nodes {
		if r, ok := out[n.ID]; !ok || r.Total <= 0 {
			missing = append(missing, n)
		}
	}
	if len(missing) == 0 {
		return out
	}
	if fb := globalPing(host, missing); len(fb) > 0 {
		for k, v := range fb {
			out[k] = v
		}
	}
	return out
}

func checkPingCH(host string, nodes []node) map[string]pingRes {
	out := map[string]pingRes{}
	if len(nodes) == 0 {
		return out
	}
	vals := url.Values{}
	vals.Set("host", host)
	for _, n := range nodes {
		vals.Add("node", n.ID)
	}
	var first map[string]any
	if err := httpGetJSON(checkHost+"/check-ping?"+vals.Encode(), &first); err != nil {
		return out
	}
	rid, _ := first["request_id"].(string)
	if rid == "" {
		return out
	}
	for attempt := 0; attempt < 6; attempt++ {
		if attempt == 0 {
			time.Sleep(6 * time.Second)
		} else {
			time.Sleep(3500 * time.Millisecond)
		}
		var data map[string]any
		if err := httpGetJSON(checkHost+"/check-result/"+rid, &data); err != nil {
			continue
		}
		done := true
		for _, n := range nodes {
			arr, ok := data[n.ID].([]any)
			if !ok {
				done = false
				continue
			}
			okc := 0
			for _, a := range arr {
				if aa, ok := a.([]any); ok && len(aa) > 0 {
					if s, _ := aa[0].(string); s == "OK" {
						okc++
					}
				}
			}
			out[n.ID] = pingRes{City: n.City, Ok: okc, Total: len(arr)}
		}
		if done {
			break
		}
	}
	return out
}

func blockedCities(ping map[string]pingRes, citiesSel []string, maxOk int) int {
	type cc struct{ nodes, blocked int }
	byCity := map[string]*cc{}
	for _, r := range ping {
		if !contains(citiesSel, r.City) || r.Total <= 0 {
			// A node that returned no samples (timeout/API hiccup) must not be
			// counted as blocked, otherwise a healthy domain gets swapped.
			continue
		}
		c := byCity[r.City]
		if c == nil {
			c = &cc{}
			byCity[r.City] = c
		}
		c.nodes++
		if r.Ok <= maxOk {
			c.blocked++
		}
	}
	n := 0
	for _, c := range byCity {
		if c.nodes > 0 && c.blocked == c.nodes {
			n++
		}
	}
	return n
}

func resolveHost(host string) []string {
	ips, err := net.LookupHost(host)
	if err != nil {
		return nil
	}
	return ips
}

// ipDiag is the verdict for a resolved IP (ported from the Worker's
// hfVerdictIp): a swap only happens when the IP itself is healthy both in Iran
// and abroad — so a filtered/down IP is never mistaken for a filtered domain.
type ipDiag struct {
	IP             string
	IranMeasured   bool
	IranBlocked    bool
	IPBlocked      bool
	IPDown         bool
	IranAccess     bool
	ForeignUnknown bool
	ForeignFail    int
	Handled        bool
}

func diagIP(ip string, cfg Cfg) ipDiag {
	iran := checkPing(ip, nodesFor(cfg.CitiesSel))
	fgn := checkPing(ip, foreignNodes)
	return verdictIP(ip, iran, fgn, cfg)
}

func verdictIP(ip string, ipp, fp map[string]pingRes, cfg Cfg) ipDiag {
	d := ipDiag{IP: ip}
	iranMeasured := len(ipp) > 0
	iranBlocked := iranMeasured && blockedCities(ipp, cfg.CitiesSel, cfg.MaxOk) >= cfg.Cities
	total, fail := 0, 0
	for _, r := range fp {
		total += r.Total
		if r.Total > r.Ok {
			fail += r.Total - r.Ok
		}
	}
	measured := len(fp) > 0 && total > 0
	foreignOk := measured && (total-fail)*2 >= total
	foreignDown := measured && fail >= ipDownFails
	foreignUnknown := !foreignOk && !foreignDown
	d.IranMeasured = iranMeasured
	d.IranBlocked = iranBlocked
	d.ForeignFail = fail
	d.ForeignUnknown = foreignUnknown
	// "Down" must be backed by at least one real sample; a total outage of both
	// probe services is unknown, not a down verdict.
	d.IPDown = (foreignDown || foreignUnknown) && (iranMeasured || measured) && (!iranMeasured || iranBlocked)
	d.IranAccess = foreignDown && iranMeasured && !iranBlocked
	d.IPBlocked = iranMeasured && iranBlocked && foreignOk && !foreignDown
	d.Handled = d.IPDown || d.IranAccess || d.IPBlocked
	return d
}

// CheckIPCleanliness probes ip from check-host.net Iranian and foreign nodes and
// returns whether the IP looks clean (reachable from Iran and abroad) plus a
// short Persian summary suitable for a Telegram message.
func CheckIPCleanliness(ip string) (bool, string) {
	ip = strings.TrimSpace(ip)
	if ip == "" {
		return false, "⚪ آی‌پی مشخص نیست."
	}
	cfg := Cfg{Cities: 2, CitiesSel: append([]string{}, irCities...), MaxOk: 0}
	d := diagIP(ip, cfg)
	switch {
	case !d.IranMeasured && d.ForeignUnknown:
		// Neither check-host nor Globalping produced a single sample; do not
		// report the IP as clean just because nothing could be measured.
		return false, "⚪ بررسی ممکن نشد — نه check-host و نه Globalping پاسخ ندادند؛ دوباره تلاش کن."
	case d.IPBlocked:
		return false, "🔴 تمیز نیست — از داخل ایران فیلتر است (خارج پاسخ می‌دهد)."
	case d.IPDown:
		return false, "🛑 از داخل و خارج پاسخ نداد (سرور خاموش یا هنوز بالا نیامده)."
	case d.IranAccess:
		return false, "🟡 «ایران‌اکسس» — از ایران پاسخ می‌دهد ولی از خارج قطع است."
	case d.IranBlocked:
		return false, "🔴 تمیز نیست — از داخل ایران فیلتر است."
	case d.ForeignUnknown:
		return true, "🟢 از داخل ایران تمیز است (بررسی خارج نامشخص)."
	default:
		return true, "🟢 تمیز — از داخل ایران و خارج پاسخ می‌دهد."
	}
}

// reHostPlaceholder matches panel remark template tokens like {USERNAME},
// {TIME_LEFT}, {STATUS_EMOJI}, {DATA_LEFT} that the panel expands at
// subscription time. They are meaningless in a raw notification, so drop them.
var reHostPlaceholder = regexp.MustCompile(`\{[^{}]*\}`)

func cleanHostLabel(s string) string {
	s = reHostPlaceholder.ReplaceAllString(s, "")
	s = strings.ReplaceAll(s, "|", "·")
	s = strings.Join(strings.Fields(s), " ")
	return strings.Trim(s, " ·-|")
}

func ipDiagMsg(panel string, host map[string]any, dv string, d ipDiag) string {
	head := "🚫 آی‌پی فیلتر شده است (نه دامنه) — تعویض انجام نشد"
	tail := "این آی‌پی از خارج (آلمان/هلند) پاسخ می‌دهد ولی از داخل ایران فیلتر است و زیر این هاست قرار دارد."
	if d.IPDown {
		head = "🛑 آی‌پی/سرور خاموش است — تعویض انجام نشد"
		tail = "از ایران و آلمان/هلند پاسخی نیامد؛ احتمالاً سرور/IP خاموش است."
	} else if d.IranAccess {
		head = "🇮🇷 آی‌پی «ایران‌اکسس» شد — تعویض انجام نشد"
		tail = "این آی‌پی از داخل ایران پاسخ می‌دهد ولی از آلمان/هلند در دسترس نیست."
	}
	remark := cleanHostLabel(strv(host["remark"]))
	if remark == "" {
		remark = strv(host["id"])
	}
	return head + "\n🖥 پنل: " + panel + "\n📄 هاست: " + trunc(remark, 60) +
		"\n🌐 آی‌پی: " + d.IP + "\n🔗 زیرساب‌دامنه: " + dv +
		"\n📝 " + tail + "\n🛠 مشکل از خود آی‌پی است نه دامنه؛ تا سالم‌شدن آن تعویض نمی‌کنم."
}

func logEntry(st *store.Store, entry map[string]any) {
	var log []map[string]any
	st.GetJSON("host_filter_log", &log)
	if _, ok := entry["ts"]; !ok {
		entry["ts"] = time.Now().UTC().Format(time.RFC3339Nano)
	}
	log = append(log, entry)
	if len(log) > 40 {
		log = log[len(log)-40:]
	}
	st.PutJSON("host_filter_log", log, 0)
}

func swapHost(bot *tg.Client, st *store.Store, panel pg.Panel, panelToken string,
	host map[string]any, oldName string, cfAccs []cf.Account, adminID int64) bool {
	low := strings.ToLower(oldName)
	id := strv(host["id"])
	for _, a := range cfAccs {
		cli := cf.New(a.Token)
		zones, err := cli.Zones()
		if err != nil {
			continue
		}
		for _, z := range zones {
			if z.Status != "active" {
				continue
			}
			zn := strings.ToLower(z.Name)
			if low != zn && !strings.HasSuffix(low, "."+zn) {
				continue
			}
			rel := ""
			if low != zn {
				rel = low[:len(low)-(len(zn)+1)]
			}
			recs, err := cli.Records(z.ID)
			if err != nil {
				continue
			}
			names := map[string]bool{}
			for _, r := range recs {
				names[strings.ToLower(r.Name)] = true
			}
			newName := ""
			for n := 1; n <= 60; n++ {
				cand := fmt.Sprintf("h%d.%s", n, zn)
				if rel != "" {
					cand = fmt.Sprintf("%s%d.%s", rel, n, zn)
				}
				if !names[cand] {
					newName = cand
					break
				}
			}
			if newName == "" {
				continue
			}
			var src []cf.Record
			for _, r := range recs {
				if strings.ToLower(r.Name) == low &&
					(r.Type == "A" || r.Type == "AAAA" || r.Type == "CNAME") {
					src = append(src, r)
				}
			}
			err = func() error {
				if len(src) > 0 {
					for _, r := range src {
						ttl := 1
						if t := intOr(r.TTL, 0); t > 0 {
							ttl = t
						}
						if _, err := cli.AddRecord(z.ID, r.Type, newName, r.Content, ttl, boolOf(r.Proxied), nil); err != nil {
							return err
						}
					}
				} else {
					if _, err := cli.AddRecord(z.ID, "CNAME", newName, oldName, 1, false, nil); err != nil {
						return err
					}
				}
				repl := map[string]string{low: newName}
				body := deepCopyMap(host)
				for _, field := range []string{"address", "sni", "host"} {
					v, ok := body[field]
					if !ok {
						continue
					}
					switch t := v.(type) {
					case []any:
						nv := make([]any, len(t))
						for i, x := range t {
							nv[i] = repl[strings.ToLower(strv(x))]
							if nv[i] == "" {
								nv[i] = x
							}
						}
						body[field] = nv
					default:
						if r := repl[strings.ToLower(strv(v))]; r != "" {
							body[field] = r
						}
					}
				}
				delete(body, "id")
				return putHost(panel.URL, panelToken, id, body)
			}()
			if err != nil {
				notify(bot, adminID, trunc(fmt.Sprintf("❌ تعویض %s نشد: %v", oldName, err), 300))
				return false
			}
			logEntry(st, map[string]any{"kind": "swap", "panel": panelName(panel),
				"from": oldName, "to": newName})
			sid := saveHostState(st, panel, id, host, oldName, newName)
			notifyKB(bot, adminID,
				fmt.Sprintf("🔄 تعویض خودکار\n<code>%s</code> → <code>%s</code>", oldName, newName),
				[][]tg.Button{{{Text: "↩️ دامنهٔ قبلی را جایگزین کن", Data: "hfrev:" + sid}}})
			return true
		}
	}
	return false
}

func putHost(base, token, id string, body map[string]any) error {
	b, _ := json.Marshal(body)
	req, err := http.NewRequest("PUT",
		strings.TrimRight(base, "/")+"/api/host/"+id, bytes.NewReader(b))
	if err != nil {
		return err
	}
	req.Header.Set("Authorization", "Bearer "+token)
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("User-Agent", httpUA)
	hc := &http.Client{Timeout: 30 * time.Second}
	resp, err := hc.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode != 200 {
		bb, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("HTTP %d: %s", resp.StatusCode, trunc(string(bb), 120))
	}
	return nil
}

// ---------- helpers ----------

func httpGetJSON(u string, out any) error {
	req, err := http.NewRequest("GET", u, nil)
	if err != nil {
		return err
	}
	req.Header.Set("User-Agent", httpUA)
	req.Header.Set("Accept", "application/json")
	hc := &http.Client{Timeout: 25 * time.Second}
	resp, err := hc.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	b, _ := io.ReadAll(resp.Body)
	if resp.StatusCode >= 400 {
		return &httpStatusError{resp.StatusCode}
	}
	return json.Unmarshal(b, out)
}

func domOf(v any) string {
	s := strv(v)
	if domRe.MatchString(s) && !ipRe.MatchString(s) {
		return strings.ToLower(s)
	}
	return ""
}

func asList(v any) []any {
	if v == nil {
		return nil
	}
	if arr, ok := v.([]any); ok {
		return arr
	}
	return []any{v}
}

func deepCopyMap(m map[string]any) map[string]any {
	out := make(map[string]any, len(m))
	for k, v := range m {
		out[k] = v
	}
	return out
}

func sortedSet(a []string) []string {
	set := map[string]bool{}
	for _, x := range a {
		set[x] = true
	}
	out := make([]string, 0, len(set))
	for k := range set {
		out = append(out, k)
	}
	sort.Strings(out)
	return out
}

func contains(a []string, x string) bool {
	for _, v := range a {
		if v == x {
			return true
		}
	}
	return false
}

func parseISO(s string) int64 {
	s = strings.TrimSpace(s)
	if s == "" {
		return 0
	}
	if t, err := time.Parse(time.RFC3339Nano, s); err == nil {
		return t.UnixMilli()
	}
	if t, err := time.Parse(time.RFC3339, strings.Replace(s, "Z", "+00:00", 1)); err == nil {
		return t.UnixMilli()
	}
	return 0
}

func intOr(v any, def int) int {
	switch t := v.(type) {
	case float64:
		return int(t)
	case int:
		return t
	case int64:
		return int(t)
	}
	return def
}

func boolOf(v any) bool { b, ok := v.(bool); return ok && b }

func strv(v any) string {
	switch t := v.(type) {
	case nil:
		return ""
	case string:
		return t
	case float64:
		if t == float64(int64(t)) {
			return fmt.Sprintf("%d", int64(t))
		}
		return fmt.Sprintf("%g", t)
	case bool:
		if t {
			return "true"
		}
		return "false"
	default:
		return fmt.Sprint(v)
	}
}

func panelName(p pg.Panel) string {
	if p.Name != "" {
		return p.Name
	}
	return p.URL
}

func notify(bot *tg.Client, adminID int64, text string) {
	if bot != nil {
		bot.Send(adminID, text, nil)
	}
}

func notifyKB(bot *tg.Client, adminID int64, text string, kb [][]tg.Button) {
	if bot != nil {
		bot.Send(adminID, text, kb)
	}
}

// ---------- host state (swap backup for revert) ----------

func newSID() string {
	b := make([]byte, 6)
	_, _ = rand.Read(b)
	return hex.EncodeToString(b)
}

func loadStates(st *store.Store) map[string]map[string]any {
	m := map[string]map[string]any{}
	st.GetJSON("hf_host_state", &m)
	if m == nil {
		m = map[string]map[string]any{}
	}
	return m
}

// saveHostState records the host's original address/sni/host before a swap and
// returns a short id used as the revert button payload.
func saveHostState(st *store.Store, panel pg.Panel, id string, host map[string]any,
	oldName, newName string) string {
	m := loadStates(st)
	var orig map[string]any
	for k, e := range m {
		if strv(e["panel"]) == panel.URL && strv(e["hid"]) == id {
			if o, ok := e["original"].(map[string]any); ok {
				orig = o
			}
			delete(m, k)
			break
		}
	}
	if orig == nil {
		orig = map[string]any{
			"address": host["address"],
			"sni":     host["sni"],
			"host":    host["host"],
		}
	}
	sid := newSID()
	m[sid] = map[string]any{
		"panel":    panel.URL,
		"hid":      id,
		"remark":   strv(host["remark"]),
		"from":     oldName,
		"to":       newName,
		"original": orig,
		"ts":       time.Now().UTC().Format(time.RFC3339),
	}
	st.PutJSON("hf_host_state", m, 0)
	return sid
}

// StateEntries lists pending swap states (for the revert menu).
func StateEntries(st *store.Store) []map[string]any {
	m := loadStates(st)
	var out []map[string]any
	for sid, e := range m {
		cp := map[string]any{}
		for k, v := range e {
			cp[k] = v
		}
		cp["sid"] = sid
		out = append(out, cp)
	}
	sort.Slice(out, func(i, j int) bool { return strv(out[i]["ts"]) > strv(out[j]["ts"]) })
	return out
}

// Revert restores a swapped host's original address/sni/host from stored state.
func Revert(st *store.Store, panels []pg.Panel, sid string) (string, error) {
	m := loadStates(st)
	e, ok := m[sid]
	if !ok {
		return "", fmt.Errorf("state_not_found")
	}
	panelURL := strv(e["panel"])
	hid := strv(e["hid"])
	orig, _ := e["original"].(map[string]any)
	var panel *pg.Panel
	for i := range panels {
		if panels[i].URL == panelURL {
			panel = &panels[i]
			break
		}
	}
	if panel == nil {
		return "", fmt.Errorf("panel_not_found")
	}
	c := panel.Client()
	token, err := c.Login()
	if err != nil {
		return "", fmt.Errorf("login_failed")
	}
	hosts, err := c.Hosts(token)
	if err != nil {
		return "", fmt.Errorf("fetch_failed")
	}
	var host map[string]any
	for _, hv := range hosts {
		if h, ok := hv.(map[string]any); ok && strv(h["id"]) == hid {
			host = h
			break
		}
	}
	if host == nil {
		return "", fmt.Errorf("host_not_found")
	}
	for _, f := range []string{"address", "sni", "host"} {
		if v, ok := orig[f]; ok && v != nil {
			host[f] = v
		}
	}
	delete(host, "id")
	if err := putHost(panel.URL, token, hid, host); err != nil {
		return "", err
	}
	delete(m, sid)
	st.PutJSON("hf_host_state", m, 0)
	logEntry(st, map[string]any{"kind": "revert", "panel": panelName(*panel), "hid": hid})
	return strv(e["remark"]), nil
}

func replaceDomain(v any, from, to string) (any, bool) {
	lfrom := strings.ToLower(from)
	switch t := v.(type) {
	case []any:
		changed := false
		nv := make([]any, len(t))
		for i, x := range t {
			if strings.ToLower(strv(x)) == lfrom {
				nv[i] = to
				changed = true
			} else {
				nv[i] = x
			}
		}
		if changed {
			return nv, true
		}
		return v, false
	default:
		if strings.ToLower(strv(v)) == lfrom {
			return to, true
		}
		return v, false
	}
}

// EnsureStateFromLog backfills revert state for swaps that happened before state
// storage existed, so recent swaps in the history log become revertible too.
func EnsureStateFromLog(st *store.Store, panels []pg.Panel) {
	var log []map[string]any
	st.GetJSON("host_filter_log", &log)
	if len(log) == 0 {
		return
	}
	states := loadStates(st)
	seen := map[string]bool{}
	for _, e := range states {
		seen[strv(e["panel"])+"|"+strv(e["from"])+"|"+strv(e["to"])] = true
	}
	for i := len(log) - 1; i >= 0; i-- {
		e := log[i]
		if strv(e["kind"]) != "swap" {
			continue
		}
		from, to, pname := strv(e["from"]), strv(e["to"]), strv(e["panel"])
		if from == "" || to == "" {
			continue
		}
		var panel *pg.Panel
		for j := range panels {
			if panelName(panels[j]) == pname || panels[j].URL == pname {
				panel = &panels[j]
				break
			}
		}
		if panel == nil || seen[panel.URL+"|"+from+"|"+to] {
			continue
		}
		c := panel.Client()
		token, err := c.Login()
		if err != nil {
			continue
		}
		hosts, err := c.Hosts(token)
		if err != nil {
			continue
		}
		for _, hv := range hosts {
			h, ok := hv.(map[string]any)
			if !ok {
				continue
			}
			orig := map[string]any{}
			found := false
			for _, f := range []string{"address", "sni", "host"} {
				if nv, ch := replaceDomain(h[f], to, from); ch {
					orig[f] = nv
					found = true
				}
			}
			if !found {
				continue
			}
			sid := newSID()
			states[sid] = map[string]any{
				"panel":    panel.URL,
				"hid":      strv(h["id"]),
				"remark":   strv(h["remark"]),
				"from":     from,
				"to":       to,
				"original": orig,
				"ts":       strv(e["ts"]),
			}
			seen[panel.URL+"|"+from+"|"+to] = true
			break
		}
	}
	st.PutJSON("hf_host_state", states, 0)
}

func max(a, b int) int {
	if a > b {
		return a
	}
	return b
}

func trunc(s string, n int) string {
	if len(s) > n {
		return s[:n]
	}
	return s
}
