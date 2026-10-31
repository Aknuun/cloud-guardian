package router

import (
	"encoding/json"
	"fmt"
	"net"
	"net/http"
	"sort"
	"strings"
	"sync"

	"github.com/aknuun/cg-server/internal/cf"
)

// ---------- shared helpers ----------

func (r *Router) webCfTraffic(w http.ResponseWriter, req *http.Request, uid int64) {
	q := req.URL.Query()
	acc := atoiSafe(q.Get("acc"))
	zid := strings.TrimSpace(q.Get("zid"))
	days := atoiSafe(q.Get("days"))
	if days != 1 && days != 7 && days != 30 {
		days = 1
	}
	accs := r.cfAccs()
	if acc < 0 || acc >= len(accs) {
		webErr(w, http.StatusBadRequest, "اکانت پیدا نشد")
		return
	}
	t := cf.FetchTrafficCounts(accs[acc].Token, zid, days)
	if t.NeedPerm {
		webErr(w, http.StatusForbidden, "دسترسی Analytics لازم است (Zone → Analytics → Read)")
		return
	}
	if t.Error != "" {
		webErr(w, http.StatusBadGateway, t.Error)
		return
	}
	total := 0
	for _, n := range t.Counts {
		total += n
	}
	type row struct {
		Name  string `json:"name"`
		Count int    `json:"count"`
	}
	rowsAll := make([]row, 0, len(t.Counts))
	for n, c := range t.Counts {
		rowsAll = append(rowsAll, row{Name: n, Count: c})
	}
	sort.SliceStable(rowsAll, func(i, j int) bool { return rowsAll[i].Count > rowsAll[j].Count })
	top := rowsAll
	if len(top) > 20 {
		top = top[:20]
	}
	webOK(w, map[string]any{
		"zone": r.webCfZoneInfo(acc, zid, cf.New(accs[acc].Token)),
		"days": days, "total": total, "top": top, "counts": t.Counts,
	})
}

// ---------- favorites ----------

func (r *Router) webCfFavs(w http.ResponseWriter, req *http.Request, uid int64) {
	if req.Method == http.MethodPost {
		var in struct {
			Rid string `json:"rid"`
		}
		_ = json.NewDecoder(http.MaxBytesReader(w, req.Body, 1<<20)).Decode(&in)
		if in.Rid == "" {
			webErr(w, http.StatusBadRequest, "شناسهٔ رکورد لازم است")
			return
		}
		r.webCfSaveFav(0, "", in.Rid, "", false)
		webOK(w, map[string]any{"message": "از منتخب حذف شد"})
		return
	}
	favs := r.favs()
	if len(favs) == 0 {
		webOK(w, map[string]any{"items": []any{}})
		return
	}
	accs := r.cfAccs()
	zoneRecs := map[string][]cf.Record{}
	zoneNames := map[string]string{}
	for _, f := range favs {
		acc := intOrAny(f["acc"])
		zid := strv(f["zid"])
		key := fmt.Sprintf("%d|%s", acc, zid)
		if _, ok := zoneRecs[key]; ok {
			continue
		}
		if acc >= 0 && acc < len(accs) {
			cli := cf.New(accs[acc].Token)
			if recs, err := cli.Records(zid); err == nil {
				zoneRecs[key] = recs
			}
			if z, err := cli.Zone(zid); err == nil {
				zoneNames[key] = strv(z["name"])
			}
		}
	}
	out := []map[string]any{}
	for _, f := range favs {
		acc := intOrAny(f["acc"])
		zid := strv(f["zid"])
		rid := strv(f["rid"])
		key := fmt.Sprintf("%d|%s", acc, zid)
		zname := r.cfZoneName(acc, zid)
		if zname == "" {
			zname = zoneNames[key]
		}
		item := map[string]any{"acc": acc, "zid": zid, "rid": rid, "zone": zname, "label": strv(f["name"]), "missing": true}
		for _, rec := range zoneRecs[key] {
			if rec.ID == rid {
				item["missing"] = false
				item["type"] = strings.ToUpper(rec.Type)
				item["content"] = rec.Content
				item["name"] = rec.Name
				if zname == "" {
					item["zone"] = zname
				}
				break
			}
		}
		out = append(out, item)
	}
	webOK(w, map[string]any{"items": out})
}

// ---------- global search ----------

func (r *Router) webCfSearch(w http.ResponseWriter, req *http.Request, uid int64) {
	q := strings.ToLower(strings.TrimSpace(req.URL.Query().Get("q")))
	if q == "" {
		webOK(w, map[string]any{"zones": []any{}, "records": []any{}})
		return
	}
	all := r.cfAllZones()
	accs := r.cfAccs()
	zones := []map[string]any{}
	zonesToScan := []cfZone{}
	for _, z := range all {
		if strings.Contains(strings.ToLower(z.Zone.Name), q) && len(zones) < 20 {
			zones = append(zones, cfWinZone(z, accs))
		}
		zonesToScan = append(zonesToScan, z)
	}
	if net.ParseIP(q) != nil {
		r.webCfSearchIP(w, q, zones, zonesToScan)
		return
	}
	var (
		mu  sync.Mutex
		wg  sync.WaitGroup
		sem = make(chan struct{}, 8)
	)
	records := []map[string]any{}
	for _, z := range zonesToScan {
		z := z
		wg.Add(1)
		go func() {
			defer wg.Done()
			sem <- struct{}{}
			defer func() { <-sem }()
			cli, ok := r.cfClientFor(z.Acc)
			if !ok {
				return
			}
			recs, err := cli.Records(z.Zone.ID)
			if err != nil {
				return
			}
			favs := r.webCfFavSet()
			for _, rec := range recs {
				if !strings.Contains(strings.ToLower(rec.Name), q) && !strings.Contains(strings.ToLower(rec.Content), q) {
					continue
				}
				mu.Lock()
				if len(records) < 40 {
					m := cfWebRecord(rec, z.Zone.Name, favs[rec.ID])
					m["acc"], m["zid"], m["zone"] = z.Acc, z.Zone.ID, z.Zone.Name
					records = append(records, m)
				}
				mu.Unlock()
			}
		}()
	}
	wg.Wait()
	webOK(w, map[string]any{"zones": zones, "records": records})
}

func (r *Router) webCfSearchIP(w http.ResponseWriter, q string, zones []map[string]any, zonesToScan []cfZone) {
	type zoneRecs struct {
		z    cfZone
		recs []cf.Record
	}
	zrs := make([]zoneRecs, len(zonesToScan))
	var wg sync.WaitGroup
	sem := make(chan struct{}, 8)
	for i, z := range zonesToScan {
		i, z := i, z
		wg.Add(1)
		go func() {
			defer wg.Done()
			sem <- struct{}{}
			defer func() { <-sem }()
			cli, ok := r.cfClientFor(z.Acc)
			if !ok {
				return
			}
			recs, err := cli.Records(z.Zone.ID)
			if err != nil {
				return
			}
			zrs[i] = zoneRecs{z: z, recs: recs}
		}()
	}
	wg.Wait()

	qip := net.ParseIP(q)
	byName := map[string][]cf.Record{}
	targets := map[string]bool{}
	for _, zr := range zrs {
		for _, rec := range zr.recs {
			byName[cfNormHost(rec.Name)] = append(byName[cfNormHost(rec.Name)], rec)
			if (strings.EqualFold(rec.Type, "A") || strings.EqualFold(rec.Type, "AAAA")) && cfSameIP(rec.Content, qip) {
				targets[cfNormHost(rec.Name)] = true
			}
		}
	}

	reaches := cfCnameResolver(targets, byName)

	records := []map[string]any{}
	favs := r.webCfFavSet()
	seen := map[string]bool{}
	add := func(z cfZone, rec cf.Record, match string) {
		if len(records) >= 40 {
			return
		}
		key := fmt.Sprintf("%d|%s|%s", z.Acc, z.Zone.ID, rec.ID)
		if seen[key] {
			return
		}
		seen[key] = true
		m := cfWebRecord(rec, z.Zone.Name, favs[rec.ID])
		m["acc"], m["zid"], m["zone"] = z.Acc, z.Zone.ID, z.Zone.Name
		if match != "" {
			m["match"] = match
		}
		records = append(records, m)
	}
	for _, zr := range zrs {
		for _, rec := range zr.recs {
			if (strings.EqualFold(rec.Type, "A") || strings.EqualFold(rec.Type, "AAAA")) && cfSameIP(rec.Content, qip) {
				add(zr.z, rec, "")
			}
		}
	}
	for _, zr := range zrs {
		for _, rec := range zr.recs {
			if strings.EqualFold(rec.Type, "CNAME") && reaches(rec.Content) {
				add(zr.z, rec, "cname")
			}
		}
	}
	webOK(w, map[string]any{"zones": zones, "records": records})
}

// ---------- CNAME refs ----------

func (r *Router) webCfRefs(w http.ResponseWriter, req *http.Request, uid int64) {
	q := req.URL.Query()
	acc := atoiSafe(q.Get("acc"))
	zid := strings.TrimSpace(q.Get("zid"))
	rid := strings.TrimSpace(q.Get("rid"))
	cli, err := r.webCfClient(acc)
	if err != nil {
		webErr(w, http.StatusBadRequest, err.Error())
		return
	}
	type zoneRecs struct {
		z    cfZone
		recs []cf.Record
	}
	zrs := make([]zoneRecs, 0, 8)
	var mu sync.Mutex
	var wg sync.WaitGroup
	sem := make(chan struct{}, 8)
	for _, z := range r.cfAllZones() {
		z := z
		wg.Add(1)
		go func() {
			defer wg.Done()
			sem <- struct{}{}
			defer func() { <-sem }()
			c, ok := r.cfClientFor(z.Acc)
			if !ok {
				return
			}
			recs, err := c.Records(z.Zone.ID)
			if err != nil {
				return
			}
			mu.Lock()
			zrs = append(zrs, zoneRecs{z: z, recs: recs})
			mu.Unlock()
		}()
	}
	wg.Wait()

	var target, rootZone string
	for _, zr := range zrs {
		if zr.z.Acc != acc || zr.z.Zone.ID != zid {
			continue
		}
		rootZone = zr.z.Zone.Name
		for _, rec := range zr.recs {
			if rec.ID == rid {
				target = cfNormHost(rec.Name)
				break
			}
		}
	}
	if target == "" {
		recs, err := cli.Records(zid)
		if err != nil {
			webErr(w, http.StatusBadGateway, err.Error())
			return
		}
		if rootZone == "" {
			rootZone = r.cfZoneName(acc, zid)
		}
		zrs = append(zrs, zoneRecs{z: cfZone{Zone: cf.Zone{ID: zid, Name: rootZone}, Acc: acc}, recs: recs})
		for _, rec := range recs {
			if rec.ID == rid {
				target = cfNormHost(rec.Name)
				break
			}
		}
	}
	if target == "" {
		webErr(w, http.StatusNotFound, "رکورد پیدا نشد")
		return
	}

	type refSeed struct {
		name    string
		zone    string
		content string
	}
	byTarget := map[string][]refSeed{}
	for _, zr := range zrs {
		for _, rec := range zr.recs {
			if !strings.EqualFold(rec.Type, "CNAME") {
				continue
			}
			t := cfNormHost(rec.Content)
			byTarget[t] = append(byTarget[t], refSeed{name: rec.Name, zone: zr.z.Zone.Name, content: rec.Content})
		}
	}
	for k := range byTarget {
		sort.SliceStable(byTarget[k], func(i, j int) bool {
			if byTarget[k][i].name == byTarget[k][j].name {
				return byTarget[k][i].zone < byTarget[k][j].zone
			}
			return byTarget[k][i].name < byTarget[k][j].name
		})
	}

	type refNode struct {
		Name     string    `json:"name"`
		Zone     string    `json:"zone"`
		Content  string    `json:"content,omitempty"`
		Children []refNode `json:"children,omitempty"`
		Cycle    bool      `json:"cycle,omitempty"`
	}
	count := 0
	var build func(string, map[string]bool, int) []refNode
	build = func(name string, path map[string]bool, depth int) []refNode {
		if depth > 8 || count >= 200 {
			return nil
		}
		seeds := byTarget[cfNormHost(name)]
		out := make([]refNode, 0, len(seeds))
		for _, s := range seeds {
			if count >= 200 {
				break
			}
			n := refNode{Name: s.name, Zone: s.zone, Content: s.content}
			count++
			key := cfNormHost(s.name)
			if path[key] {
				n.Cycle = true
				out = append(out, n)
				continue
			}
			if depth < 8 {
				next := make(map[string]bool, len(path)+1)
				for k, v := range path {
					next[k] = v
				}
				next[key] = true
				n.Children = build(s.name, next, depth+1)
			}
			out = append(out, n)
		}
		return out
	}
	tree := build(target, map[string]bool{target: true}, 0)
	webOK(w, map[string]any{"target": target, "zone": rootZone, "tree": tree, "count": count})
}

// ---------- load balancer / spare ----------
