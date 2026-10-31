package router

import (
	"crypto/tls"
	"fmt"
	"net"
	"net/http"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/aknuun/cg-server/internal/cf"
)

// A record health row is what the checker page renders: the record, what it
// currently says, what the network actually answers, and a verdict.
type cfHealthRow struct {
	Acc     int    `json:"acc"`
	Zid     string `json:"zid"`
	Zone    string `json:"zone"`
	Rid     string `json:"rid"`
	Name    string `json:"name"`
	Type    string `json:"type"`
	Content string `json:"content"`
	Proxied bool   `json:"proxied"`
	Status  string `json:"status"` // ok | proxy | nxdomain | timeout | bad
	Detail  string `json:"detail"`
	Latency int    `json:"latency"` // ms, 0 when not measured
	HTTP    int    `json:"http"`
	TLSDays int    `json:"tlsDays"`
}

type cfHealthCfg struct {
	Enabled     bool  `json:"enabled"`
	IntervalMin int   `json:"intervalMin"`
	LastRun     int64 `json:"lastRun"`
}

type cfHealthLast struct {
	At    int64         `json:"at"`
	Rows  []cfHealthRow `json:"rows"`
	Total int           `json:"total"`
	Bad   int           `json:"bad"`
}

const (
	cfHealthCfgKey  = "cf_check_cfg"
	cfHealthLastKey = "cf_check_last"
)

func (r *Router) cfHealthCfg() cfHealthCfg {
	c := cfHealthCfg{Enabled: false, IntervalMin: 60}
	r.Store.GetJSON(cfHealthCfgKey, &c)
	if c.IntervalMin < 5 {
		c.IntervalMin = 60
	}
	return c
}

func (r *Router) cfHealthLastGet() cfHealthLast {
	var l cfHealthLast
	r.Store.GetJSON(cfHealthLastKey, &l)
	if l.Rows == nil {
		l.Rows = []cfHealthRow{}
	}
	return l
}

func dialAny(host string, ports []int, ms int) (int, int) {
	for _, p := range ports {
		t := time.Now()
		c, err := net.DialTimeout("tcp", net.JoinHostPort(host, strconv.Itoa(p)), time.Duration(ms)*time.Millisecond)
		if err == nil {
			c.Close()
			return p, int(time.Since(t).Milliseconds())
		}
	}
	return 0, 0
}

func tlsDaysLeft(host string) int {
	d := &net.Dialer{Timeout: 4 * time.Second}
	conn, err := tls.DialWithDialer(d, "tcp", net.JoinHostPort(host, "443"), &tls.Config{ServerName: host, InsecureSkipVerify: true})
	if err != nil {
		return 0
	}
	defer conn.Close()
	cs := conn.ConnectionState()
	if len(cs.PeerCertificates) == 0 {
		return 0
	}
	return int(time.Until(cs.PeerCertificates[0].NotAfter).Hours() / 24)
}

func httpProbe(host string) int {
	cl := &http.Client{Timeout: 6 * time.Second, Transport: &http.Transport{
		TLSClientConfig: &tls.Config{InsecureSkipVerify: true},
	}}
	resp, err := cl.Head("https://" + host + "/")
	if err != nil {
		resp, err = cl.Get("https://" + host + "/")
		if err != nil {
			return 0
		}
	}
	defer resp.Body.Close()
	return resp.StatusCode
}

// cfHealthCheckRow probes one record from this server (Germany).
func cfHealthCheckRow(row cfHealthRow) cfHealthRow {
	t := strings.ToUpper(row.Type)
	switch t {
	case "A", "AAAA":
		if net.ParseIP(row.Content) == nil {
			row.Status, row.Detail = "bad", "مقدار آی‌پی معتبر نیست"
			return row
		}
		if row.Proxied {
			row.Status, row.Detail = "proxy", "پشت کلادفلر"
			return row
		}
		port, lat := dialAny(row.Content, []int{443, 80, 22}, 4000)
		if port == 0 {
			row.Status, row.Detail = "timeout", "هیچ پورتی جواب نداد"
			return row
		}
		row.Latency = lat
		row.Status, row.Detail = "ok", fmt.Sprintf("پورت %d پاسخ داد", port)
		return row
	case "CNAME":
		fqdn := row.Name
		if row.Proxied {
			row.Status, row.Detail = "proxy", "پشت کلادفلر"
			return row
		}
		ips, err := net.LookupHost(fqdn)
		if err != nil || len(ips) == 0 {
			row.Status, row.Detail = "nxdomain", "ریسالو نمی‌شود"
			return row
		}
		port, lat := dialAny(ips[0], []int{443, 80}, 4000)
		row.Latency = lat
		row.HTTP = httpProbe(fqdn)
		row.TLSDays = tlsDaysLeft(fqdn)
		if port == 0 {
			row.Status, row.Detail = "timeout", "آی‌پی «"+ips[0]+"» جواب نداد"
			return row
		}
		row.Status, row.Detail = "ok", "به «"+ips[0]+"» می‌رسد"
		return row
	}
	row.Status, row.Detail = "ok", "بررسی نمی‌شود"
	return row
}

// cfHealthCollect gathers A/AAAA/CNAME rows for one account (acc<0 = all) or one zone.
func (r *Router) cfHealthCollect(acc int, zid string) []cfHealthRow {
	var out []cfHealthRow
	accs := r.cfAccs()
	for i, a := range accs {
		if acc >= 0 && acc != i {
			continue
		}
		cli := cf.New(a.Token)
		zones, err := cli.Zones()
		if err != nil {
			continue
		}
		for _, z := range zones {
			if zid != "" && z.ID != zid {
				continue
			}
			recs, err := cli.Records(z.ID)
			if err != nil {
				continue
			}
			for _, rec := range recs {
				t := strings.ToUpper(rec.Type)
				if t != "A" && t != "AAAA" && t != "CNAME" {
					continue
				}
				fqdn := rec.Name
				if fqdn == "@" {
					fqdn = z.Name
				} else if !strings.HasSuffix(fqdn, z.Name) {
					fqdn = fqdn + "." + z.Name
				}
				out = append(out, cfHealthRow{
					Acc: i, Zid: z.ID, Zone: z.Name, Rid: rec.ID,
					Name: fqdn, Type: t, Content: rec.Content, Proxied: rec.Proxied == true,
				})
			}
		}
	}
	return out
}

func (r *Router) cfHealthScan(acc int, zid string, only string) cfHealthLast {
	rows := r.cfHealthCollect(acc, zid)
	if only != "" {
		kept := rows[:0]
		for _, x := range rows {
			if x.Rid == only {
				kept = append(kept, x)
			}
		}
		rows = kept
	}
	if len(rows) > 400 {
		rows = rows[:400]
	}
	out := make([]cfHealthRow, len(rows))
	sem := make(chan struct{}, 8)
	var wg sync.WaitGroup
	for i := range rows {
		wg.Add(1)
		sem <- struct{}{}
		go func(i int) {
			defer wg.Done()
			defer func() { <-sem }()
			out[i] = cfHealthCheckRow(rows[i])
		}(i)
	}
	wg.Wait()
	sort.SliceStable(out, func(i, j int) bool {
		if out[i].Zone != out[j].Zone {
			return out[i].Zone < out[j].Zone
		}
		return out[i].Name < out[j].Name
	})
	bad := 0
	for _, x := range out {
		if x.Status != "ok" && x.Status != "proxy" {
			bad++
		}
	}
	last := cfHealthLast{At: time.Now().UnixMilli(), Rows: out, Total: len(out), Bad: bad}
	r.Store.PutJSON(cfHealthLastKey, last, 0)
	cfg := r.cfHealthCfg()
	cfg.LastRun = last.At
	r.Store.PutJSON(cfHealthCfgKey, cfg, 0)
	return last
}

// CfHealthTick runs the scheduled scan and reports regressions to the admin.
func (r *Router) CfHealthTick(force bool) {
	cfg := r.cfHealthCfg()
	if !cfg.Enabled && !force {
		return
	}
	if !force && time.Since(time.UnixMilli(cfg.LastRun)) < time.Duration(cfg.IntervalMin)*time.Minute {
		return
	}
	prev := r.cfHealthLastGet()
	prevBad := map[string]bool{}
	for _, x := range prev.Rows {
		if x.Status != "ok" && x.Status != "proxy" {
			prevBad[x.Zid+"|"+x.Rid] = true
		}
	}
	last := r.cfHealthScan(-1, "", "")
	var fresh []string
	for _, x := range last.Rows {
		k := x.Zid + "|" + x.Rid
		if x.Status != "ok" && x.Status != "proxy" && !prevBad[k] {
			fresh = append(fresh, "• "+x.Name+" ("+x.Type+") — "+x.Detail)
		}
	}
	if len(fresh) == 0 || r.Bot == nil {
		return
	}
	if len(fresh) > 25 {
		fresh = append(fresh[:25], "…")
	}
	r.Bot.Send(r.AdminID, "🩺 <b>سلامت رکوردهای کلادفلر</b>\n\n"+fmt.Sprint(strings.Join(fresh, "\n")), nil)
}
