package router

import (
	"encoding/json"
	"fmt"
	"net"
	"net/http"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/aknuun/cg-server/internal/cf"
	"github.com/aknuun/cg-server/internal/lb"
)

// ---------- shared helpers ----------

func (r *Router) webCfClient(acc int) (*cf.Client, error) {
	accs := r.cfAccs()
	if acc < 0 || acc >= len(accs) {
		return nil, fmt.Errorf("اکانت کلادفلر پیدا نشد")
	}
	return cf.New(accs[acc].Token), nil
}

func (r *Router) webCfAccNames() []map[string]any {
	accs := r.cfAccs()
	out := make([]map[string]any, 0, len(accs))
	for i, a := range accs {
		out = append(out, map[string]any{"i": i, "name": a.Name})
	}
	return out
}

func cfTTLNum(v any) int {
	switch t := v.(type) {
	case float64:
		return int(t)
	case int:
		return t
	case int64:
		return int(t)
	case string:
		return atoiSafe(t)
	}
	return 1
}

func cfBoolOf(v any) bool {
	b, _ := v.(bool)
	return b
}

func cfFQDN(name, zname string) string {
	n := strings.ToLower(strings.TrimSpace(name))
	switch {
	case n == "":
		return ""
	case n == "@":
		return zname
	case n == zname, strings.HasSuffix(n, "."+zname):
		return n
	}
	return n + "." + zname
}

func cfWebRecord(rec cf.Record, zname string, fav bool) map[string]any {
	return map[string]any{
		"id": rec.ID, "type": strings.ToUpper(rec.Type), "name": rec.Name,
		"content": rec.Content, "ttl": cfTTLNum(rec.TTL),
		"proxied": cfBoolOf(rec.Proxied), "priority": rec.Priority,
		"label": cfRecLabel(rec.Name, rec.Type, zname), "fav": fav,
	}
}

func cfWinZone(z cfZone, accs []cf.Account) map[string]any {
	name := ""
	if z.Acc >= 0 && z.Acc < len(accs) {
		name = accs[z.Acc].Name
	}
	return map[string]any{
		"id": z.Zone.ID, "name": z.Zone.Name, "status": z.Zone.Status,
		"acc": z.Acc, "accName": name,
	}
}

func (r *Router) webCfZoneInfo(acc int, zid string, cli *cf.Client) map[string]any {
	z, err := cli.Zone(zid)
	if err != nil {
		return map[string]any{"id": zid, "name": r.cfZoneName(acc, zid)}
	}
	ns := []string{}
	if arr, ok := z["name_servers"].([]any); ok {
		for _, n := range arr {
			ns = append(ns, strv(n))
		}
	}
	created := strv(z["created_on"])
	if len(created) > 10 {
		created = created[:10]
	}
	accName := ""
	if a, ok := z["account"].(map[string]any); ok {
		accName = strv(a["name"])
	}
	name := strv(z["name"])
	if name != "" {
		cfZnMu.Lock()
		cfZnCache[fmt.Sprintf("%d:%s", acc, zid)] = cfZnEntry{name: name, at: time.Now()}
		cfZnMu.Unlock()
	}
	return map[string]any{
		"id": zid, "name": name, "status": strv(z["status"]),
		"paused": strv(z["status"]) == "paused", "plan": strv(z["plan"]),
		"created": created, "account": accName, "ns": ns,
	}
}

func (r *Router) webCfFavSet() map[string]bool {
	out := map[string]bool{}
	for _, f := range r.favs() {
		out[strv(f["rid"])] = true
	}
	return out
}

func (r *Router) webCfSaveFav(acc int, zid, rid, label string, add bool) {
	favs := r.favs()
	var out []map[string]any
	found := false
	for _, f := range favs {
		if strv(f["rid"]) == rid {
			found = true
			if !add {
				continue
			}
		}
		out = append(out, f)
	}
	if add && !found {
		out = append(out, map[string]any{"acc": acc, "zid": zid, "rid": rid, "name": label})
	}
	r.Store.PutJSON("favs", toAnyList(out), 0)
}

// ---------- accounts ----------

func (r *Router) webCfAccounts(w http.ResponseWriter, req *http.Request, uid int64) {
	if req.Method == http.MethodPost {
		var in struct {
			Op    string `json:"op"`
			Name  string `json:"name"`
			Token string `json:"token"`
			I     int    `json:"i"`
		}
		if err := json.NewDecoder(http.MaxBytesReader(w, req.Body, 1<<20)).Decode(&in); err != nil {
			webErr(w, http.StatusBadRequest, "بدنهٔ درخواست نامعتبر است")
			return
		}
		accs := r.cfAccs()
		switch in.Op {
		case "add":
			in.Name = strings.TrimSpace(in.Name)
			in.Token = strings.TrimSpace(in.Token)
			if in.Name == "" || in.Token == "" {
				webErr(w, http.StatusBadRequest, "نام و توکن الزامی است")
				return
			}
			accs = append(accs, cf.Account{Name: in.Name, Token: in.Token})
		case "del":
			if in.I < 0 || in.I >= len(accs) {
				webErr(w, http.StatusBadRequest, "اکانت پیدا نشد")
				return
			}
			accs = append(accs[:in.I], accs[in.I+1:]...)
			r.cfDropZoneNames(in.I)
			var keep []map[string]any
			for _, f := range r.favs() {
				fa := intOrAny(f["acc"])
				if fa == in.I {
					continue
				}
				if fa > in.I {
					f["acc"] = fa - 1
				}
				keep = append(keep, f)
			}
			r.Store.PutJSON("favs", toAnyList(keep), 0)
		default:
			webErr(w, http.StatusBadRequest, "عملیات نامعتبر است")
			return
		}
		r.Store.PutJSON("cf_accounts", accs, 0)
		webOK(w, map[string]any{"accounts": r.webCfAccNames()})
		return
	}
	accs := r.webCfAccNames()
	for _, a := range accs {
		i := intOrAny(a["i"])
		n := -1
		if cli, err := r.webCfClient(i); err == nil {
			if zs, err := cli.Zones(); err == nil {
				n = len(zs)
			}
		}
		a["zones"] = n
	}
	webOK(w, map[string]any{"accounts": accs})
}

// ---------- zones ----------

func (r *Router) webCfZones(w http.ResponseWriter, req *http.Request, uid int64) {
	all, err := r.cfAllZonesE()
	if err != nil && len(all) == 0 {
		webErr(w, http.StatusBadGateway, err.Error())
		return
	}
	accs := r.cfAccs()
	out := make([]map[string]any, 0, len(all))
	for _, z := range all {
		out = append(out, cfWinZone(z, accs))
	}
	sort.SliceStable(out, func(i, j int) bool {
		return strv(out[i]["name"]) < strv(out[j]["name"])
	})
	webOK(w, map[string]any{"zones": out, "accounts": r.webCfAccNames(), "favs": len(r.favs())})
}

// ---------- records ----------

func (r *Router) webCfRecords(w http.ResponseWriter, req *http.Request, uid int64) {
	q := req.URL.Query()
	acc := atoiSafe(q.Get("acc"))
	zid := strings.TrimSpace(q.Get("zid"))
	cli, err := r.webCfClient(acc)
	if err != nil {
		webErr(w, http.StatusBadRequest, err.Error())
		return
	}
	if zid == "" {
		webErr(w, http.StatusBadRequest, "شناسهٔ دامنه لازم است")
		return
	}
	recs, err := cli.Records(zid)
	if err != nil {
		webErr(w, http.StatusBadGateway, err.Error())
		return
	}
	info := r.webCfZoneInfo(acc, zid, cli)
	zname := strv(info["name"])
	favs := r.webCfFavSet()
	labels := cfDupLabels(recs, zname)
	out := make([]map[string]any, 0, len(recs))
	for _, rec := range recs {
		o := cfWebRecord(rec, zname, favs[rec.ID])
		o["label"] = labels[rec.ID]
		o["history"] = r.cfRecHistory(acc, zid, rec.ID)
		out = append(out, o)
	}
	webOK(w, map[string]any{"zone": info, "records": out})
}

func cfNormHost(s string) string {
	return strings.ToLower(strings.TrimRight(strings.TrimSpace(s), "."))
}

func cfSameIP(value string, want net.IP) bool {
	if want == nil {
		return false
	}
	ip := net.ParseIP(strings.TrimSpace(value))
	return ip != nil && ip.Equal(want)
}

// cfCnameResolver returns a memoized predicate that reports whether a hostname
// points, directly or through CNAME records, at one of the IP targets.
func cfCnameResolver(targets map[string]bool, byName map[string][]cf.Record) func(string) bool {
	memo := map[string]bool{}
	visiting := map[string]bool{}
	var reaches func(string) bool
	reaches = func(name string) bool {
		n := cfNormHost(name)
		if targets[n] {
			return true
		}
		if v, ok := memo[n]; ok {
			return v
		}
		if visiting[n] {
			return false
		}
		visiting[n] = true
		ok := false
		for _, rec := range byName[n] {
			if strings.EqualFold(rec.Type, "CNAME") && reaches(rec.Content) {
				ok = true
				break
			}
		}
		delete(visiting, n)
		memo[n] = ok
		return ok
	}
	return reaches
}

func (r *Router) cfHistKey(acc int, zid, rid string) string {
	return fmt.Sprintf("cfhist:%d:%s:%s", acc, zid, rid)
}

func (r *Router) cfRecHistory(acc int, zid, rid string) []string {
	out := []string{}
	if rid == "" {
		return out
	}
	r.Store.GetJSON(r.cfHistKey(acc, zid, rid), &out)
	if out == nil {
		return []string{}
	}
	return out
}

func (r *Router) cfPushHistory(acc int, zid, rid, value string) {
	value = strings.TrimSpace(value)
	if rid == "" || value == "" {
		return
	}
	old := r.cfRecHistory(acc, zid, rid)
	next := []string{value}
	for _, v := range old {
		if v != value && len(next) < 8 {
			next = append(next, v)
		}
	}
	r.Store.PutJSON(r.cfHistKey(acc, zid, rid), next, 0)
}

func (r *Router) webCfRecordOp(w http.ResponseWriter, req *http.Request, uid int64) {
	if req.Method != http.MethodPost {
		webErr(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}
	var in struct {
		Op       string `json:"op"`
		Acc      int    `json:"acc"`
		Zid      string `json:"zid"`
		Rid      string `json:"rid"`
		Type     string `json:"type"`
		Name     string `json:"name"`
		Content  string `json:"content"`
		TTL      int    `json:"ttl"`
		Proxied  bool   `json:"proxied"`
		Priority *int   `json:"priority"`
		Force    bool   `json:"force"`
	}
	if err := json.NewDecoder(http.MaxBytesReader(w, req.Body, 1<<20)).Decode(&in); err != nil {
		webErr(w, http.StatusBadRequest, "بدنهٔ درخواست نامعتبر است")
		return
	}
	cli, err := r.webCfClient(in.Acc)
	if err != nil {
		webErr(w, http.StatusBadRequest, err.Error())
		return
	}
	if in.Zid == "" {
		webErr(w, http.StatusBadRequest, "شناسهٔ دامنه لازم است")
		return
	}
	zname := r.cfZoneName(in.Acc, in.Zid)
	msg := "انجام شد"
	var rec *cf.Record

	switch in.Op {
	case "add":
		name := cfFQDN(in.Name, zname)
		rtype := strings.ToUpper(strings.TrimSpace(in.Type))
		content := strings.TrimSpace(in.Content)
		if name == "" || rtype == "" || content == "" {
			webErr(w, http.StatusBadRequest, "نام، نوع و مقدار الزامی است")
			return
		}
		if !in.Force && cfDuplicate(cli, in.Zid, name, rtype, content, zname) {
			webErr(w, http.StatusConflict, "رکورد تکراری است (همان نام/نوع/مقدار)")
			return
		}
		ttl := in.TTL
		proxied := in.Proxied
		switch rtype {
		case "A", "AAAA", "CNAME":
		default:
			proxied = false
		}
		if ttl <= 0 {
			ttl = 1
		}
		rec, err = cli.AddRecord(in.Zid, rtype, name, content, ttl, proxied, in.Priority)
		if err != nil {
			webErr(w, http.StatusBadGateway, err.Error())
			return
		}
		msg = "رکورد اضافه شد"
	case "edit":
		cur := r.cfFind(in.Acc, in.Zid, in.Rid)
		if cur == nil {
			webErr(w, http.StatusNotFound, "رکورد پیدا نشد")
			return
		}
		changes := map[string]any{}
		if c := strings.TrimSpace(in.Content); c != "" && c != cur.Content {
			r.cfPushHistory(in.Acc, in.Zid, cur.ID, cur.Content)
			changes["content"] = c
		}
		if in.TTL > 0 {
			changes["ttl"] = in.TTL
		}
		if strings.EqualFold(cur.Type, "A") || strings.EqualFold(cur.Type, "AAAA") || strings.EqualFold(cur.Type, "CNAME") {
			changes["proxied"] = in.Proxied
		}
		if in.Priority != nil {
			changes["priority"] = *in.Priority
		}
		rec, err = cli.UpdateRecord(in.Zid, *cur, changes)
		if err != nil {
			webErr(w, http.StatusBadGateway, err.Error())
			return
		}
		msg = "رکورد ویرایش شد"
	case "del":
		if err := cli.DelRecord(in.Zid, in.Rid); err != nil {
			webErr(w, http.StatusBadGateway, err.Error())
			return
		}
		r.webCfSaveFav(in.Acc, in.Zid, in.Rid, "", false)
		msg = "رکورد حذف شد"
	case "proxy":
		cur := r.cfFind(in.Acc, in.Zid, in.Rid)
		if cur == nil {
			webErr(w, http.StatusNotFound, "رکورد پیدا نشد")
			return
		}
		switch strings.ToUpper(cur.Type) {
		case "A", "AAAA", "CNAME":
		default:
			webErr(w, http.StatusBadRequest, "پروکسی فقط برای A / AAAA / CNAME است")
			return
		}
		rec, err = cli.UpdateRecord(in.Zid, *cur, map[string]any{"proxied": in.Proxied})
		if err != nil {
			webErr(w, http.StatusBadGateway, err.Error())
			return
		}
		msg = "پروکسی تغییر کرد"
	case "ttl":
		cur := r.cfFind(in.Acc, in.Zid, in.Rid)
		if cur == nil {
			webErr(w, http.StatusNotFound, "رکورد پیدا نشد")
			return
		}
		next := in.TTL
		if next <= 0 {
			steps := []int{1, 120, 300, 1800, 3600, 21600, 86400}
			next = 1
			curTTL := cfTTLNum(cur.TTL)
			for i, s := range steps {
				if s == curTTL {
					next = steps[(i+1)%len(steps)]
					break
				}
			}
		}
		rec, err = cli.UpdateRecord(in.Zid, *cur, map[string]any{"ttl": next})
		if err != nil {
			webErr(w, http.StatusBadGateway, err.Error())
			return
		}
		msg = "TTL تغییر کرد"
	case "type":
		cur := r.cfFind(in.Acc, in.Zid, in.Rid)
		if cur == nil {
			webErr(w, http.StatusNotFound, "رکورد پیدا نشد")
			return
		}
		target := strings.ToUpper(strings.TrimSpace(in.Type))
		if target == "" {
			webErr(w, http.StatusBadRequest, "نوع جدید لازم است")
			return
		}
		content := strings.TrimSpace(in.Content)
		if content == "" {
			webErr(w, http.StatusBadRequest, "مقدار جدید برای تبدیل نوع لازم است")
			return
		}
		switch target {
		case "A":
			ip := net.ParseIP(content)
			if ip == nil || ip.To4() == nil {
				webErr(w, http.StatusBadRequest, "برای رکورد A یک آی‌پی معتبر IPv4 وارد کن")
				return
			}
		case "CNAME":
			if !cfIsHost(content) {
				webErr(w, http.StatusBadRequest, "برای رکورد CNAME یک دامنهٔ معتبر وارد کن")
				return
			}
		default:
			webErr(w, http.StatusBadRequest, "تبدیل نوع فقط برای A و CNAME پشتیبانی می‌شود")
			return
		}
		r.cfPushHistory(in.Acc, in.Zid, cur.ID, cur.Content)
		rec, err = cli.UpdateRecord(in.Zid, *cur, map[string]any{"type": target, "content": content})
		if err != nil {
			webErr(w, http.StatusBadGateway, err.Error())
			return
		}
		msg = "نوع رکورد تغییر کرد"
	case "fav":
		cur := r.cfFind(in.Acc, in.Zid, in.Rid)
		if cur == nil {
			webErr(w, http.StatusNotFound, "رکورد پیدا نشد")
			return
		}
		add := !r.webCfFavSet()[in.Rid]
		r.webCfSaveFav(in.Acc, in.Zid, in.Rid, cfRecLabel(cur.Name, cur.Type, zname), add)
		if add {
			msg = "به منتخب اضافه شد"
		} else {
			msg = "از منتخب حذف شد"
		}
	default:
		webErr(w, http.StatusBadRequest, "عملیات نامعتبر است")
		return
	}

	out := map[string]any{"message": msg}
	if rec != nil {
		m := cfWebRecord(*rec, zname, r.webCfFavSet()[rec.ID])
		m["history"] = r.cfRecHistory(in.Acc, in.Zid, rec.ID)
		out["record"] = m
	}
	webOK(w, out)
}

func (r *Router) webCfBulk(w http.ResponseWriter, req *http.Request, uid int64) {
	if req.Method != http.MethodPost {
		webErr(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}
	var in struct {
		Op      string   `json:"op"`
		Acc     int      `json:"acc"`
		Zid     string   `json:"zid"`
		IDs     []string `json:"ids"`
		Type    string   `json:"type"`
		Content string   `json:"content"`
	}
	if err := json.NewDecoder(http.MaxBytesReader(w, req.Body, 1<<20)).Decode(&in); err != nil {
		webErr(w, http.StatusBadRequest, "بدنهٔ درخواست نامعتبر است")
		return
	}
	cli, err := r.webCfClient(in.Acc)
	if err != nil {
		webErr(w, http.StatusBadRequest, err.Error())
		return
	}
	zid := in.Zid
	if zid == "" || len(in.IDs) == 0 {
		webErr(w, http.StatusBadRequest, "دامنه و رکوردها لازم است")
		return
	}
	info := r.webCfZoneInfo(in.Acc, zid, cli)
	zname := strv(info["name"])
	recs, err := cli.Records(zid)
	if err != nil {
		webErr(w, http.StatusBadGateway, err.Error())
		return
	}
	byID := map[string]cf.Record{}
	for _, rec := range recs {
		byID[rec.ID] = rec
	}
	okN, fail := 0, 0
	lastErr := ""
	for _, rid := range in.IDs {
		cur, exists := byID[rid]
		if !exists {
			fail++
			continue
		}
		var err error
		switch in.Op {
		case "del":
			err = cli.DelRecord(zid, rid)
			if err == nil {
				r.webCfSaveFav(in.Acc, zid, rid, "", false)
			}
		case "edit":
			_, err = cli.UpdateRecord(zid, cur, map[string]any{"content": strings.TrimSpace(in.Content)})
		case "type":
			_, err = cli.UpdateRecord(zid, cur, map[string]any{"type": strings.ToUpper(in.Type)})
		case "fav":
			r.webCfSaveFav(in.Acc, zid, rid, cfRecLabel(cur.Name, cur.Type, zname), true)
		default:
			webErr(w, http.StatusBadRequest, "عملیات نامعتبر است")
			return
		}
		if err != nil {
			fail++
			lastErr = err.Error()
		} else {
			okN++
		}
	}
	msg := fmt.Sprintf("%d مورد انجام شد", okN)
	if fail > 0 {
		msg += fmt.Sprintf(" · %d ناموفق", fail)
		if lastErr != "" {
			msg += ": " + lastErr
		}
	}
	webOK(w, map[string]any{"message": msg, "ok": okN, "failed": fail})
}

// ---------- settings ----------

func (r *Router) webCfSettings(w http.ResponseWriter, req *http.Request, uid int64) {
	if req.Method == http.MethodPost {
		var in struct {
			Op    string `json:"op"`
			Acc   int    `json:"acc"`
			Zid   string `json:"zid"`
			Key   string `json:"key"`
			Value any    `json:"value"`
		}
		if err := json.NewDecoder(http.MaxBytesReader(w, req.Body, 1<<20)).Decode(&in); err != nil {
			webErr(w, http.StatusBadRequest, "بدنهٔ درخواست نامعتبر است")
			return
		}
		cli, err := r.webCfClient(in.Acc)
		if err != nil {
			webErr(w, http.StatusBadRequest, err.Error())
			return
		}
		msg := "انجام شد"
		switch in.Op {
		case "set":
			info, ok := cfSetInfo[in.Key]
			if !ok {
				webErr(w, http.StatusBadRequest, "تنظیم ناشناخته")
				return
			}
			var val any = in.Value
			if info.Enum {
				s := strv(in.Value)
				valid := false
				for _, v := range info.Values {
					if v.V == s {
						valid = true
						break
					}
				}
				if !valid {
					webErr(w, http.StatusBadRequest, "مقدار نامعتبر است")
					return
				}
				val = s
			} else {
				on := false
				switch v := in.Value.(type) {
				case bool:
					on = v
				case string:
					on = v == "on" || v == "true" || v == "1"
				}
				if on {
					val = "on"
				} else {
					val = "off"
				}
			}
			if err := cli.SetSetting(in.Zid, in.Key, val); err != nil {
				webErr(w, http.StatusBadGateway, err.Error())
				return
			}
			msg = "تنظیم ذخیره شد"
		case "purge":
			if err := cli.PurgeCache(in.Zid); err != nil {
				webErr(w, http.StatusBadGateway, err.Error())
				return
			}
			msg = "کل کش دامنه پاک شد"
		case "pause":
			z, err := cli.Zone(in.Zid)
			if err != nil {
				webErr(w, http.StatusBadGateway, err.Error())
				return
			}
			active := strv(z["status"]) == "active"
			if err := cli.SetZonePaused(in.Zid, active); err != nil {
				webErr(w, http.StatusBadGateway, err.Error())
				return
			}
			if active {
				msg = "دامنه متوقف شد"
			} else {
				msg = "دامنه فعال شد"
			}
		case "dev":
			smap, err := cli.Settings(in.Zid)
			if err != nil {
				webErr(w, http.StatusBadGateway, err.Error())
				return
			}
			next := "on"
			if strv(smap["development_mode"]) == "on" {
				next = "off"
			}
			if err := cli.SetSetting(in.Zid, "development_mode", next); err != nil {
				webErr(w, http.StatusBadGateway, err.Error())
				return
			}
			if next == "on" {
				msg = "حالت توسعه روشن شد"
			} else {
				msg = "حالت توسعه خاموش شد"
			}
		default:
			webErr(w, http.StatusBadRequest, "عملیات نامعتبر است")
			return
		}
		webOK(w, map[string]any{"message": msg})
		return
	}

	q := req.URL.Query()
	acc := atoiSafe(q.Get("acc"))
	zid := strings.TrimSpace(q.Get("zid"))
	cli, err := r.webCfClient(acc)
	if err != nil {
		webErr(w, http.StatusBadRequest, err.Error())
		return
	}
	if zid == "" {
		webErr(w, http.StatusBadRequest, "شناسهٔ دامنه لازم است")
		return
	}
	smap, err := cli.Settings(zid)
	if err != nil {
		webErr(w, http.StatusBadGateway, err.Error())
		return
	}
	info := r.webCfZoneInfo(acc, zid, cli)
	groups := []map[string]any{}
	for _, g := range []struct {
		key   string
		title string
		keys  []string
	}{
		{"ssl", "SSL / TLS", cfSSLKeys},
		{"sec", "امنیت", cfSECKeys},
		{"perf", "کارایی", cfPERFKeys},
	} {
		items := []map[string]any{}
		for _, k := range g.keys {
			si := cfSetInfo[k]
			cur := strv(smap[k])
			it := map[string]any{"key": k, "label": si.Label, "value": cur, "kind": "bool"}
			if si.Enum {
				it["kind"] = "enum"
				vals := []map[string]any{}
				for _, v := range si.Values {
					vals = append(vals, map[string]any{"v": v.V, "l": v.L})
				}
				it["values"] = vals
			}
			items = append(items, it)
		}
		groups = append(groups, map[string]any{"key": g.key, "title": g.title, "items": items})
	}
	webOK(w, map[string]any{
		"zone": info, "groups": groups,
		"dev":    strv(smap["development_mode"]) == "on",
		"paused": strv(info["status"]) == "paused",
	})
}

// ---------- email routing ----------

func (r *Router) webCfEmail(w http.ResponseWriter, req *http.Request, uid int64) {
	if req.Method == http.MethodPost {
		var in struct {
			Op   string `json:"op"`
			Acc  int    `json:"acc"`
			Zid  string `json:"zid"`
			Addr string `json:"addr"`
			Dest string `json:"dest"`
			Rid  string `json:"rid"`
			On   bool   `json:"on"`
		}
		if err := json.NewDecoder(http.MaxBytesReader(w, req.Body, 1<<20)).Decode(&in); err != nil {
			webErr(w, http.StatusBadRequest, "بدنهٔ درخواست نامعتبر است")
			return
		}
		cli, err := r.webCfClient(in.Acc)
		if err != nil {
			webErr(w, http.StatusBadRequest, err.Error())
			return
		}
		msg := "انجام شد"
		switch in.Op {
		case "enable":
			if err := cli.EmailEnable(in.Zid); err != nil {
				webErr(w, http.StatusBadGateway, err.Error())
				return
			}
			msg = "ایمیل فعال شد"
		case "add":
			local := strings.TrimSpace(in.Addr)
			if strings.Contains(local, "@") {
				local = strings.SplitN(local, "@", 2)[0]
			}
			dest := strings.TrimSpace(in.Dest)
			if local == "" || dest == "" {
				webErr(w, http.StatusBadRequest, "آدرس و مقصد الزامی است")
				return
			}
			zname := strv(r.webCfZoneInfo(in.Acc, in.Zid, cli)["name"])
			addr := local + "@" + zname
			if err := cli.EmailRuleAdd(in.Zid, local, addr, dest); err != nil {
				webErr(w, http.StatusBadGateway, err.Error())
				return
			}
			msg = "قانون اضافه شد: " + addr + " → " + dest
		case "del":
			if err := cli.EmailRuleDel(in.Zid, in.Rid); err != nil {
				webErr(w, http.StatusBadGateway, err.Error())
				return
			}
			msg = "قانون حذف شد"
		case "catchall":
			if in.On {
				dest := strings.TrimSpace(in.Dest)
				if dest == "" {
					webErr(w, http.StatusBadRequest, "مقصد الزامی است")
					return
				}
				if err := cli.EmailCatchAllSet(in.Zid, map[string]any{
					"enabled": true,
					"actions": []any{map[string]any{"type": "forward", "value": []string{dest}}},
				}); err != nil {
					webErr(w, http.StatusBadGateway, err.Error())
					return
				}
				msg = "catch-all فعال شد"
			} else {
				if err := cli.EmailCatchAllSet(in.Zid, map[string]any{
					"enabled": false,
					"actions": []any{map[string]any{"type": "drop", "value": []string{}}},
				}); err != nil {
					webErr(w, http.StatusBadGateway, err.Error())
					return
				}
				msg = "catch-all غیرفعال شد"
			}
		case "dest_add":
			aid := cli.AccountID(in.Zid)
			if aid == "" {
				webErr(w, http.StatusBadRequest, "شناسهٔ اکانت پیدا نشد")
				return
			}
			if err := cli.EmailDestAdd(aid, strings.TrimSpace(in.Dest)); err != nil {
				webErr(w, http.StatusBadGateway, err.Error())
				return
			}
			msg = "مقصد اضافه شد؛ لینک تأیید به ایمیل رفت"
		default:
			webErr(w, http.StatusBadRequest, "عملیات نامعتبر است")
			return
		}
		webOK(w, map[string]any{"message": msg})
		return
	}

	q := req.URL.Query()
	acc := atoiSafe(q.Get("acc"))
	zid := strings.TrimSpace(q.Get("zid"))
	cli, err := r.webCfClient(acc)
	if err != nil {
		webErr(w, http.StatusBadRequest, err.Error())
		return
	}
	st := cli.EmailRouting(zid)
	if st == nil {
		webOK(w, map[string]any{"available": false, "zone": r.webCfZoneInfo(acc, zid, cli)})
		return
	}
	enabled, _ := st["enabled"].(bool)
	rulesOut := []map[string]any{}
	if enabled {
		rules, _ := cli.EmailRules(zid)
		for _, rule := range rules {
			isAll := false
			for _, m := range asMapList(rule["matchers"]) {
				if strv(m["type"]) == "all" {
					isAll = true
				}
			}
			if isAll {
				continue
			}
			label := strv(rule["name"])
			for _, m := range asMapList(rule["matchers"]) {
				if strv(m["type"]) == "literal" {
					label = strv(m["value"])
				}
			}
			rulesOut = append(rulesOut, map[string]any{
				"id": strv(rule["id"]), "label": label,
				"dest": r.ruleDest(rule), "enabled": rule["enabled"],
			})
		}
	}
	ca := cli.EmailCatchAll(zid)
	dests := []map[string]any{}
	if aid := cli.AccountID(zid); aid != "" {
		if lst, err := cli.EmailDests(aid); err == nil {
			for _, d := range mailDestsSorted(lst) {
				dests = append(dests, map[string]any{
					"email": strv(d["email"]), "verified": d["verified"],
				})
			}
		}
	}
	webOK(w, map[string]any{
		"available": true, "enabled": enabled, "status": strv(st["status"]),
		"zone": r.webCfZoneInfo(acc, zid, cli), "rules": rulesOut,
		"catchAll": r.catchAllDesc(ca), "dests": dests,
	})
}

// ---------- traffic ----------

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

func (r *Router) webCfLbGroups(w http.ResponseWriter, req *http.Request, uid int64) {
	q := req.URL.Query()
	acc := atoiSafe(q.Get("acc"))
	zid := strings.TrimSpace(q.Get("zid"))
	cli, err := r.webCfClient(acc)
	if err != nil {
		webErr(w, http.StatusBadRequest, err.Error())
		return
	}
	recs, err := cli.Records(zid)
	if err != nil {
		webErr(w, http.StatusBadGateway, err.Error())
		return
	}
	names := map[string]bool{}
	for _, rec := range recs {
		if lb.IsLbRec(rec.Type) {
			names[rec.Name] = true
		}
	}
	sorted := make([]string, 0, len(names))
	for n := range names {
		sorted = append(sorted, n)
	}
	sort.Strings(sorted)
	out := []map[string]any{}
	for _, name := range sorted {
		cfg := lb.GetCfg(r.Store, lb.KeyFor(zid, name))
		on, off := 0, 0
		for _, rec := range recs {
			if rec.Name == name && lb.IsLbRec(rec.Type) {
				on++
			}
		}
		off = len(cfg.Disabled)
		out = append(out, map[string]any{"name": name, "on": on, "off": off, "total": on + off})
	}
	webOK(w, map[string]any{"zone": r.webCfZoneInfo(acc, zid, cli), "groups": out})
}

func (r *Router) webCfLb(w http.ResponseWriter, req *http.Request, uid int64) {
	if req.Method == http.MethodPost {
		var in struct {
			Op      string  `json:"op"`
			Acc     int     `json:"acc"`
			Zid     string  `json:"zid"`
			Name    string  `json:"name"`
			Type    string  `json:"type"`
			Content string  `json:"content"`
			TTL     int     `json:"ttl"`
			Proxied bool    `json:"proxied"`
			Weight  float64 `json:"weight"`
			On      bool    `json:"on"`
			Spare   string  `json:"spare"`
		}
		if err := json.NewDecoder(http.MaxBytesReader(w, req.Body, 1<<20)).Decode(&in); err != nil {
			webErr(w, http.StatusBadRequest, "بدنهٔ درخواست نامعتبر است")
			return
		}
		if in.Zid == "" || in.Name == "" {
			webErr(w, http.StatusBadRequest, "دامنه و نام لازم است")
			return
		}
		cli, err := r.webCfClient(in.Acc)
		if err != nil {
			webErr(w, http.StatusBadRequest, err.Error())
			return
		}
		key := lb.KeyFor(in.Zid, in.Name)
		cfg := lb.GetCfg(r.Store, key)
		msg := "انجام شد"
		switch in.Op {
		case "add":
			rtype := strings.ToUpper(strings.TrimSpace(in.Type))
			content := strings.TrimSpace(in.Content)
			if rtype == "" || content == "" {
				webErr(w, http.StatusBadRequest, "نوع و مقدار لازم است")
				return
			}
			if _, err := cli.AddRecord(in.Zid, rtype, in.Name, content, 1, false, nil); err != nil {
				webErr(w, http.StatusBadGateway, err.Error())
				return
			}
			msg = "آی‌پی اضافه شد"
		case "toggle":
			group, err := cli.Records(in.Zid)
			if err != nil {
				webErr(w, http.StatusBadGateway, err.Error())
				return
			}
			var entry *cf.Record
			for i := range group {
				if group[i].Name == in.Name && group[i].Content == in.Content && lb.IsLbRec(group[i].Type) {
					entry = &group[i]
					break
				}
			}
			if in.On {
				var done bool
				for _, d := range cfg.Disabled {
					if d.Content != in.Content {
						continue
					}
					if _, err := cli.AddRecord(in.Zid, d.Type, in.Name, d.Content, cfTTLNum(d.TTL), d.Proxied, nil); err != nil {
						webErr(w, http.StatusBadGateway, err.Error())
						return
					}
					var nd []lb.Disabled
					for _, x := range cfg.Disabled {
						if x.Content != in.Content {
							nd = append(nd, x)
						}
					}
					cfg.Disabled = nd
					done = true
					break
				}
				if !done {
					webErr(w, http.StatusNotFound, "مورد غیرفعال پیدا نشد")
					return
				}
				msg = "آی‌پی فعال شد"
			} else {
				if entry == nil {
					webErr(w, http.StatusNotFound, "آی‌پی فعال پیدا نشد")
					return
				}
				if err := cli.DelRecord(in.Zid, entry.ID); err != nil {
					webErr(w, http.StatusBadGateway, err.Error())
					return
				}
				cfg.Disabled = append(cfg.Disabled, lb.Disabled{
					Type: entry.Type, Content: entry.Content, Proxied: cfBoolOf(entry.Proxied), TTL: entry.TTL,
				})
				msg = "آی‌پی غیرفعال شد"
			}
		case "del":
			content := strings.TrimSpace(in.Content)
			if content == "" {
				webErr(w, http.StatusBadRequest, "مقدار آی‌پی لازم است")
				return
			}
			group, err := cli.Records(in.Zid)
			if err != nil {
				webErr(w, http.StatusBadGateway, err.Error())
				return
			}
			for _, rec := range group {
				if rec.Name != in.Name || rec.Content != content || !lb.IsLbRec(rec.Type) {
					continue
				}
				if err := cli.DelRecord(in.Zid, rec.ID); err != nil {
					webErr(w, http.StatusBadGateway, err.Error())
					return
				}
			}
			nd := make([]lb.Disabled, 0, len(cfg.Disabled))
			for _, d := range cfg.Disabled {
				if d.Content != content {
					nd = append(nd, d)
				}
			}
			cfg.Disabled = nd
			delete(cfg.Weights, content)
			delete(cfg.Spares, content)
			msg = "آی‌پی حذف شد"
		case "weight":
			if cfg.Weights == nil {
				cfg.Weights = map[string]float64{}
			}
			cfg.Weights[in.Content] = in.Weight
			msg = "وزن ذخیره شد"
		case "proxy":
			group, err := cli.Records(in.Zid)
			if err != nil {
				webErr(w, http.StatusBadGateway, err.Error())
				return
			}
			for _, rec := range group {
				if rec.Name != in.Name || !lb.IsLbRec(rec.Type) {
					continue
				}
				if _, err := cli.UpdateRecord(in.Zid, rec, map[string]any{"proxied": in.On}); err != nil {
					webErr(w, http.StatusBadGateway, err.Error())
					return
				}
			}
			msg = "پروکسی همه تغییر کرد"
		case "spare_set":
			spare := strings.TrimSpace(in.Spare)
			if spare == "" {
				webErr(w, http.StatusBadRequest, "مقدار زاپاس لازم است")
				return
			}
			typ := "CNAME"
			if ip := net.ParseIP(spare); ip != nil {
				if strings.Contains(spare, ".") {
					typ = "A"
				} else {
					typ = "AAAA"
				}
			}
			if cfg.Spares == nil {
				cfg.Spares = map[string]lb.Spare{}
			}
			cfg.Spares[in.Content] = lb.Spare{Type: typ, Content: spare}
			msg = "زاپاس ذخیره شد"
		case "spare_del":
			delete(cfg.Spares, in.Content)
			msg = "زاپاس حذف شد"
		case "replace":
			sp, ok := cfg.Spares[in.Content]
			if !ok {
				webErr(w, http.StatusNotFound, "زاپاسی تعیین نشده")
				return
			}
			group, err := cli.Records(in.Zid)
			if err != nil {
				webErr(w, http.StatusBadGateway, err.Error())
				return
			}
			var entry *cf.Record
			for i := range group {
				if group[i].Name == in.Name && group[i].Content == in.Content {
					entry = &group[i]
					break
				}
			}
			if entry == nil {
				webErr(w, http.StatusNotFound, "رکورد پیدا نشد")
				return
			}
			spType := sp.Type
			if spType == "" {
				spType = entry.Type
			}
			if _, err := cli.AddRecord(in.Zid, spType, in.Name, sp.Content, cfTTLNum(entry.TTL), cfBoolOf(entry.Proxied), nil); err != nil {
				webErr(w, http.StatusBadGateway, err.Error())
				return
			}
			if err := cli.DelRecord(in.Zid, entry.ID); err != nil {
				webErr(w, http.StatusBadGateway, "رکورد جدید ساخته شد ولی حذف قبلی ناموفق بود: "+err.Error())
				return
			}
			msg = "جایگزینی انجام شد"
		default:
			webErr(w, http.StatusBadRequest, "عملیات نامعتبر است")
			return
		}
		lb.SaveCfg(r.Store, key, cfg)
		webOK(w, map[string]any{"message": msg})
		return
	}

	q := req.URL.Query()
	acc := atoiSafe(q.Get("acc"))
	zid := strings.TrimSpace(q.Get("zid"))
	name := strings.TrimSpace(q.Get("name"))
	cli, err := r.webCfClient(acc)
	if err != nil {
		webErr(w, http.StatusBadRequest, err.Error())
		return
	}
	_, group, cfg, _, err := r.lbGroupOf(acc, zid, name)
	if err != nil {
		webErr(w, http.StatusBadGateway, strings.TrimPrefix(err.Error(), "❌ "))
		return
	}
	entries := lb.EntriesOf(group, cfg)
	out := make([]map[string]any, 0, len(entries))
	for _, e := range entries {
		row := map[string]any{
			"on": e.On, "id": e.ID, "type": e.Type, "content": e.Content,
			"proxied": e.Proxied, "ttl": cfTTLNum(e.TTL), "weight": e.Weight,
		}
		if sp, ok := cfg.Spares[e.Content]; ok {
			row["spare"] = map[string]any{"type": sp.Type, "content": sp.Content}
		}
		out = append(out, row)
	}
	webOK(w, map[string]any{
		"zone": r.webCfZoneInfo(acc, zid, cli), "name": name,
		"entries": out, "total": len(out),
	})
}
