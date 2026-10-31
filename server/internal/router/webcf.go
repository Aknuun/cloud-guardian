package router

import (
	"encoding/json"
	"fmt"
	"net"
	"net/http"
	"sort"
	"strings"
	"time"

	"github.com/aknuun/cg-server/internal/cf"
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
	// plan is an object in the CF API; fmt.Sprint on it used to leak the whole
	// subscription blob into the zone header.
	plan := ""
	if p, ok := z["plan"].(map[string]any); ok {
		plan = strv(p["name"])
	} else {
		plan = strv(z["plan"])
	}
	return map[string]any{
		"id": zid, "name": name, "status": strv(z["status"]),
		"paused": strv(z["status"]) == "paused", "plan": plan,
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
		// The editor picks A/CNAME/AAAA from what was typed, so let one save change the type too.
		if t := strings.ToUpper(strings.TrimSpace(in.Type)); t != "" && !strings.EqualFold(t, cur.Type) {
			changes["type"] = t
		}
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
		gone := r.cfFind(in.Acc, in.Zid, in.Rid)
		if err := cli.DelRecord(in.Zid, in.Rid); err != nil {
			webErr(w, http.StatusBadGateway, err.Error())
			return
		}
		label := in.Rid
		if gone != nil {
			label = gone.Type + " " + gone.Name
		}
		r.cfAuditAdd(uid, in.Acc, in.Zid, "", "حذف رکورد", label)
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
	if in.Op == "del" && okN > 0 {
		r.cfAuditAdd(uid, in.Acc, zid, zname, "حذف گروهی", fmt.Sprintf("%d رکورد", okN))
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
			r.cfAuditAdd(uid, in.Acc, in.Zid, "", "پاک‌کردن کش", "همهٔ کش دامنه")
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
				r.cfAuditAdd(uid, in.Acc, in.Zid, "", "توقف دامنه", "کل دامنه متوقف شد")
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
