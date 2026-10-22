package router

import (
	"encoding/json"
	"fmt"
	"net"
	"net/http"
	"net/url"
	"sort"
	"strings"
	"time"

	"github.com/aknuun/cg-server/internal/ln"
)

func (r *Router) lnWebCli(acc int) (*ln.Client, error) {
	accs := r.lnAccs()
	if acc < 0 || acc >= len(accs) {
		return nil, fmt.Errorf("اکانت پیدا نشد")
	}
	return ln.New(strv(accs[acc]["token"])), nil
}

func lnWebStatus(st string) string {
	switch strings.ToLower(st) {
	case "running":
		return "running"
	case "offline", "stopped", "shutdown":
		return "off"
	case "booting", "starting", "provisioning":
		return "starting"
	case "shutting_down", "stopping":
		return "stopping"
	case "rebooting", "rebuild", "rebuilding", "migrating", "cloning":
		return "rebuilding"
	case "deleting":
		return "deleting"
	}
	return st
}

func lnWebServerObj(s map[string]any) map[string]any {
	specs, _ := s["specs"].(map[string]any)
	ipv4 := selStrings(s["ipv4"])
	ip6 := strv(s["ipv6"])
	if ip6 == "" {
		for _, a := range ipv4 {
			if strings.Contains(a, ":") {
				ip6 = a
				break
			}
		}
	}
	ram := intOfAny(specs["memory"])
	disk := intOfAny(specs["disk"])
	return map[string]any{
		"id":         strv(s["id"]),
		"name":       strv(s["label"]),
		"status":     lnWebStatus(strv(s["status"])),
		"raw_status": strv(s["status"]),
		"ipv4":       strings.Join(ipv4, " · "),
		"ipv4_list":  ipv4,
		"ipv6":       ip6,
		"region":     strv(s["region"]),
		"type":       strv(s["type"]),
		"image":      strv(s["image"]),
		"created":    strv(s["created"]),
		"updated":    strv(s["updated"]),
		"group":      strv(s["group"]),
		"tags":       s["tags"],
		"hypervisor": strv(s["hypervisor"]),
		"specs": map[string]any{
			"vcpus":    intOfAny(specs["vcpus"]),
			"ram_mb":   ram,
			"disk_mb":  disk,
			"disk_gb":  float64(int64(float64(disk)/1024*10+0.5)) / 10,
			"transfer": intOfAny(specs["transfer"]),
			"gpus":     intOfAny(specs["gpus"]),
		},
	}
}

func (r *Router) lnWebRegions(acc int) []map[string]any {
	cli, err := r.lnWebCli(acc)
	if err != nil {
		return nil
	}
	v, _ := arvCached(fmt.Sprintf("mlnreg:%d", acc), 6*time.Hour, func() any {
		rows := []map[string]any{}
		for _, rg := range cli.All("/regions") {
			ok := false
			for _, c := range toList(rg["capabilities"]) {
				if strv(c) == "Linodes" {
					ok = true
					break
				}
			}
			if !ok {
				continue
			}
			rows = append(rows, map[string]any{
				"id": strv(rg["id"]), "label": strv(rg["label"]),
				"country": strv(rg["country"]), "site_type": strv(rg["site_type"]),
				"status": strv(rg["status"]),
			})
		}
		sort.Slice(rows, func(i, j int) bool { return strv(rows[i]["label"]) < strv(rows[j]["label"]) })
		return rows
	})
	out, _ := v.([]map[string]any)
	return out
}

func (r *Router) lnWebTypes(acc int) []map[string]any {
	cli, err := r.lnWebCli(acc)
	if err != nil {
		return nil
	}
	v, _ := arvCached(fmt.Sprintf("mlntyp:%d", acc), 30*time.Minute, func() any {
		rows := []map[string]any{}
		for _, t := range cli.All("/linode/types") {
			if boolVal(t["deprecated"]) {
				continue
			}
			rows = append(rows, t)
		}
		sort.Slice(rows, func(i, j int) bool {
			if floatVal(rows[i]["memory"]) != floatVal(rows[j]["memory"]) {
				return floatVal(rows[i]["memory"]) < floatVal(rows[j]["memory"])
			}
			return strv(rows[i]["label"]) < strv(rows[j]["label"])
		})
		return rows
	})
	out, _ := v.([]map[string]any)
	return out
}

func (r *Router) lnWebImages(acc int) []map[string]any {
	cli, err := r.lnWebCli(acc)
	if err != nil {
		return nil
	}
	v, _ := arvCached(fmt.Sprintf("mlnimg:%d", acc), 10*time.Minute, func() any {
		rows := []map[string]any{}
		for _, im := range cli.All("/images?is_public=true") {
			if strv(im["status"]) != "available" || boolVal(im["deprecated"]) {
				continue
			}
			rows = append(rows, im)
		}
		sort.Slice(rows, func(i, j int) bool {
			return strings.ToLower(strv(rows[i]["label"])) < strings.ToLower(strv(rows[j]["label"]))
		})
		return rows
	})
	out, _ := v.([]map[string]any)
	return out
}

func lnWebPrice(t map[string]any, region string) (monthly, hourly string) {
	p := lnRegionPrice(t, region)
	m, h := strv(p["monthly"]), strv(p["hourly"])
	if m == "" {
		if base, _ := t["price"].(map[string]any); base != nil {
			m, h = strv(base["monthly"]), strv(base["hourly"])
		}
	}
	return m, h
}

func (r *Router) lnWebPrices(acc int, typeID, region string) map[string]any {
	if typeID == "" {
		return map[string]any{}
	}
	for _, t := range r.lnWebTypes(acc) {
		if strv(t["id"]) != typeID {
			continue
		}
		m, h := lnWebPrice(t, region)
		return map[string]any{"monthly": m, "hourly": h, "label": strv(t["label"])}
	}
	return map[string]any{}
}

// lnWebTransferGB normalizes Linode transfer values. The API has returned
// both GB and byte-sized numbers over time; quota size picks the unit.
func lnWebTransferGB(v, quota float64) float64 {
	if v <= 0 {
		return 0
	}
	switch {
	case quota >= 100_000_000:
		return v / (1024 * 1024 * 1024)
	case quota >= 1_000_000:
		return v / 1024
	default:
		return v
	}
}

func (r *Router) lnWebTraffic(cli *ln.Client, sid string) map[string]any {
	d, err := cli.Call("GET", "/linode/instances/"+sid+"/transfer", nil)
	if err != nil {
		return map[string]any{"ok": false}
	}
	used := floatVal(d["used"])
	quota := floatVal(d["quota"])
	usedGB := lnWebTransferGB(used, quota)
	quotaGB := lnWebTransferGB(quota, quota)
	pct := 0.0
	if quotaGB > 0 {
		pct = float64(int64(usedGB/quotaGB*1000+0.5)) / 10
	}
	return map[string]any{
		"ok": true, "used_gb": round2(usedGB), "quota_gb": round2(quotaGB),
		"billable_gb": round2(lnWebTransferGB(floatVal(d["billable"]), quota)), "used_pct": pct,
	}
}

func (r *Router) lnWebSnaps(cli *ln.Client) []map[string]any {
	out := []map[string]any{}
	for _, im := range cli.All("/images?is_public=false") {
		if strv(im["status"]) != "available" {
			continue
		}
		out = append(out, map[string]any{
			"id": strv(im["id"]), "name": strv(im["label"]), "description": strv(im["description"]),
			"size_gb": floatVal(im["size"]), "created": strv(im["created"]),
			"created_by": strv(im["created_by"]), "status": strv(im["status"]),
		})
	}
	sort.Slice(out, func(i, j int) bool { return strv(out[i]["created"]) > strv(out[j]["created"]) })
	return out
}

func (r *Router) webLnAccounts(w http.ResponseWriter, req *http.Request, uid int64) {
	accs := r.lnAccs()
	out := make([]map[string]any, 0, len(accs))
	for i, a := range accs {
		row := map[string]any{"i": i, "name": strv(a["name"])}
		if list := r.lnServersCached(i); list != nil {
			row["count"] = len(list)
		}
		out = append(out, row)
	}
	webOK(w, map[string]any{"accounts": out})
}

func (r *Router) webLnServers(w http.ResponseWriter, req *http.Request, uid int64) {
	acc := atoiSafe(req.URL.Query().Get("acc"))
	if _, err := r.lnWebCli(acc); err != nil {
		webErr(w, http.StatusBadRequest, err.Error())
		return
	}
	list := r.lnServersCached(acc)
	out := make([]map[string]any, 0, len(list))
	for _, s := range list {
		out = append(out, lnWebServerObj(s))
	}
	webOK(w, map[string]any{"servers": out})
}

func (r *Router) webLnServer(w http.ResponseWriter, req *http.Request, uid int64) {
	q := req.URL.Query()
	acc := atoiSafe(q.Get("acc"))
	sid := q.Get("id")
	cli, err := r.lnWebCli(acc)
	if err != nil {
		webErr(w, http.StatusBadRequest, err.Error())
		return
	}
	if sid == "" {
		webErr(w, http.StatusBadRequest, "شناسهٔ سرور لازم است")
		return
	}
	s, err := cli.Call("GET", "/linode/instances/"+sid, nil)
	if err != nil {
		webErr(w, http.StatusBadGateway, err.Error())
		return
	}
	if strv(s["id"]) == "" {
		webErr(w, http.StatusNotFound, "سرور پیدا نشد")
		return
	}
	obj := lnWebServerObj(s)
	obj["traffic"] = r.lnWebTraffic(cli, sid)
	obj["prices"] = r.lnWebPrices(acc, strv(s["type"]), strv(s["region"]))
	obj["snapshots"] = r.lnWebSnaps(cli)
	snaps, _ := obj["snapshots"].([]map[string]any)
	obj["snapshot_count"] = len(snaps)
	webOK(w, map[string]any{"server": obj})
}

func (r *Router) webLnOptions(w http.ResponseWriter, req *http.Request, uid int64) {
	q := req.URL.Query()
	acc := atoiSafe(q.Get("acc"))
	region := q.Get("region")
	if _, err := r.lnWebCli(acc); err != nil {
		webErr(w, http.StatusBadRequest, err.Error())
		return
	}
	regions := r.lnWebRegions(acc)
	types := make([]map[string]any, 0)
	for _, t := range r.lnWebTypes(acc) {
		m, h := lnWebPrice(t, region)
		types = append(types, map[string]any{
			"id": strv(t["id"]), "name": strv(t["label"]),
			"vcpus": intOfAny(t["vcpus"]), "ram_mb": intOfAny(t["memory"]),
			"disk_gb": intOfAny(t["disk"]), "transfer": intOfAny(t["transfer"]),
			"class": strv(t["class"]), "monthly": m, "hourly": h,
		})
	}
	images := make([]map[string]any, 0)
	for _, im := range r.lnWebImages(acc) {
		images = append(images, map[string]any{
			"id": strv(im["id"]), "name": strv(im["label"]),
			"description": strv(im["description"]), "vendor": strv(im["vendor"]),
			"created": strv(im["created"]), "expiry": strv(im["expiry"]),
		})
	}
	webOK(w, map[string]any{"regions": regions, "types": types, "images": images})
}

func (r *Router) webLnAction(w http.ResponseWriter, req *http.Request, uid int64) {
	if req.Method != http.MethodPost {
		webErr(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}
	var in struct {
		Acc     int    `json:"acc"`
		ID      string `json:"id"`
		Action  string `json:"action"`
		Arg     string `json:"arg"`
		Confirm bool   `json:"confirm"`
	}
	if err := json.NewDecoder(http.MaxBytesReader(w, req.Body, 1<<20)).Decode(&in); err != nil {
		webErr(w, http.StatusBadRequest, "بدنهٔ درخواست نامعتبر است")
		return
	}
	if in.ID == "" || in.Action == "" {
		webErr(w, http.StatusBadRequest, "پارامتر ناقص")
		return
	}
	mutating := map[string]bool{
		"boot": true, "shutdown": true, "reboot": true, "rename": true, "rebuild": true,
		"resize": true, "delete": true, "reset_password": true, "snapshot_create": true,
		"snapshot_delete": true, "ip_add4": true, "ip_add6": true, "ip_del4": true, "ip_del6": true,
	}
	if mutating[in.Action] && !in.Confirm {
		webErr(w, http.StatusPreconditionRequired, "این عملیات نیاز به تأیید دارد")
		return
	}
	cli, err := r.lnWebCli(in.Acc)
	if err != nil {
		webErr(w, http.StatusBadRequest, err.Error())
		return
	}
	sid := in.ID
	back := func() { r.lnDropServers(in.Acc) }
	switch in.Action {
	case "boot", "shutdown", "reboot":
		path := "/linode/instances/" + sid + "/" + in.Action
		if _, err := cli.Call("POST", path, map[string]any{}); err != nil {
			webErr(w, http.StatusBadGateway, err.Error())
			return
		}
		back()
		webOK(w, map[string]any{"message": "دستور ارسال شد"})
	case "rename":
		name := strings.TrimSpace(in.Arg)
		if name == "" || len(name) > 64 {
			webErr(w, http.StatusBadRequest, "نام سرور نامعتبر است")
			return
		}
		if _, err := cli.Call("PUT", "/linode/instances/"+sid, map[string]any{"label": name}); err != nil {
			webErr(w, http.StatusBadGateway, err.Error())
			return
		}
		back()
		webOK(w, map[string]any{"message": "نام تغییر کرد"})
	case "reset_password":
		disk, err := cli.Disk(sid)
		if err != nil {
			webErr(w, http.StatusBadGateway, err.Error())
			return
		}
		if disk == nil {
			webErr(w, http.StatusNotFound, "دیسکی پیدا نشد")
			return
		}
		pw := lnRandomPass()
		if _, err := cli.Call("POST", "/linode/instances/"+sid+"/disks/"+strv(disk["id"])+"/password",
			map[string]any{"password": pw}); err != nil {
			webErr(w, http.StatusBadGateway, err.Error()+" (برای تغییر رمز، سرور باید خاموش باشد)")
			return
		}
		webOK(w, map[string]any{"password": pw, "message": "رمز جدید ساخته شد؛ سرور را خاموش و روشن کن"})
	case "snapshot_create":
		s, err := cli.Call("GET", "/linode/instances/"+sid, nil)
		if err != nil {
			webErr(w, http.StatusBadGateway, err.Error())
			return
		}
		disk, err := cli.Disk(sid)
		if err != nil {
			webErr(w, http.StatusBadGateway, err.Error())
			return
		}
		if disk == nil {
			webErr(w, http.StatusNotFound, "دیسکی پیدا نشد")
			return
		}
		label := strings.TrimSpace(in.Arg)
		if label == "" {
			label = fmt.Sprintf("snap-%s-%d", lnLabelPart(strv(s["label"])), time.Now().Unix())
		}
		if _, err := cli.Call("POST", "/images", map[string]any{"disk_id": disk["id"], "label": label}); err != nil {
			webErr(w, http.StatusBadGateway, err.Error())
			return
		}
		arvMem.Delete(fmt.Sprintf("mlnimg:%d", in.Acc))
		webOK(w, map[string]any{"message": "اسنپ‌شات ساخته شد: " + label})
	case "snapshot_delete":
		if in.Arg == "" {
			webErr(w, http.StatusBadRequest, "شناسهٔ اسنپ‌شات لازم است")
			return
		}
		if _, err := cli.Call("DELETE", "/images/"+in.Arg, nil); err != nil {
			webErr(w, http.StatusBadGateway, err.Error())
			return
		}
		webOK(w, map[string]any{"message": "اسنپ‌شات حذف شد"})
	case "rebuild":
		if in.Arg == "" {
			webErr(w, http.StatusBadRequest, "تصویر لازم است")
			return
		}
		pw := lnRandomPass()
		if _, err := cli.Call("POST", "/linode/instances/"+sid+"/rebuild",
			map[string]any{"image": in.Arg, "root_pass": pw, "booted": true}); err != nil {
			webErr(w, http.StatusBadGateway, err.Error())
			return
		}
		back()
		webOK(w, map[string]any{"password": pw, "message": "ریبیلد شروع شد"})
	case "resize":
		if in.Arg == "" {
			webErr(w, http.StatusBadRequest, "پلن جدید لازم است")
			return
		}
		if _, err := cli.Call("POST", "/linode/instances/"+sid+"/resize",
			map[string]any{"type": in.Arg, "allow_auto_disk_resize": true}); err != nil {
			webErr(w, http.StatusBadGateway, err.Error())
			return
		}
		back()
		webOK(w, map[string]any{"message": "ارتقا شروع شد"})
	case "delete":
		if _, err := cli.Call("DELETE", "/linode/instances/"+sid, nil); err != nil {
			webErr(w, http.StatusBadGateway, err.Error())
			return
		}
		back()
		webOK(w, map[string]any{"message": "سرور حذف شد"})
	case "ip_add4", "ip_add6":
		typ := "ipv4"
		if in.Action == "ip_add6" {
			typ = "ipv6"
		}
		if _, err := cli.Call("POST", "/linode/instances/"+sid+"/ips",
			map[string]any{"type": typ, "public": true}); err != nil {
			webErr(w, http.StatusBadGateway, err.Error())
			return
		}
		back()
		webOK(w, map[string]any{"message": "آی‌پی " + strings.ToUpper(typ) + " اضافه شد"})
	case "ip_del4":
		s, err := cli.Call("GET", "/linode/instances/"+sid, nil)
		if err != nil {
			webErr(w, http.StatusBadGateway, err.Error())
			return
		}
		var extra []string
		for _, a := range selStrings(s["ipv4"]) {
			if ip := net.ParseIP(a); ip != nil && !ip.IsPrivate() {
				extra = append(extra, a)
			}
		}
		if len(extra) < 2 {
			webErr(w, http.StatusConflict, "آی‌پی IPv4 اضافه‌ای برای حذف وجود ندارد")
			return
		}
		addr := extra[len(extra)-1]
		if _, err := cli.Call("DELETE", "/networking/ips/"+url.PathEscape(addr), nil); err != nil {
			webErr(w, http.StatusBadGateway, err.Error())
			return
		}
		back()
		webOK(w, map[string]any{"message": "آی‌پی حذف شد: " + addr})
	case "ip_del6":
		var addrs []string
		for _, x := range cli.All("/networking/ips") {
			if strv(x["type"]) == "ipv6" && strv(x["linode_id"]) == sid {
				addrs = append(addrs, strv(x["address"]))
			}
		}
		if len(addrs) == 0 {
			webErr(w, http.StatusNotFound, "آی‌پی IPv6 قابل حذف پیدا نشد")
			return
		}
		addr := addrs[len(addrs)-1]
		if _, err := cli.Call("DELETE", "/networking/ips/"+url.PathEscape(addr), nil); err != nil {
			webErr(w, http.StatusBadGateway, err.Error())
			return
		}
		back()
		webOK(w, map[string]any{"message": "آی‌پی IPv6 حذف شد: " + addr})
	default:
		webErr(w, http.StatusBadRequest, "عملیات ناشناخته")
	}
}

func (r *Router) webLnCreate(w http.ResponseWriter, req *http.Request, uid int64) {
	if req.Method != http.MethodPost {
		webErr(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}
	var in struct {
		Acc    int    `json:"acc"`
		Name   string `json:"name"`
		Region string `json:"region"`
		Type   string `json:"type"`
		Image  string `json:"image"`
	}
	if err := json.NewDecoder(http.MaxBytesReader(w, req.Body, 1<<20)).Decode(&in); err != nil {
		webErr(w, http.StatusBadRequest, "بدنهٔ درخواست نامعتبر است")
		return
	}
	in.Name = strings.TrimSpace(in.Name)
	if in.Name == "" || len(in.Name) > 64 || strings.ContainsAny(in.Name, " \t/") {
		webErr(w, http.StatusBadRequest, "نام سرور نامعتبر است (بدون فاصله، حداکثر ۶۴ کاراکتر)")
		return
	}
	if in.Region == "" || in.Type == "" || in.Image == "" {
		webErr(w, http.StatusBadRequest, "منطقه، پلن و تصویر لازم است")
		return
	}
	cli, err := r.lnWebCli(in.Acc)
	if err != nil {
		webErr(w, http.StatusBadRequest, err.Error())
		return
	}
	pw := lnRandomPass()
	res, err := cli.Call("POST", "/linode/instances", map[string]any{
		"label": in.Name, "region": in.Region, "type": in.Type,
		"image": in.Image, "root_pass": pw, "booted": true,
	})
	if err != nil {
		webErr(w, http.StatusBadGateway, err.Error())
		return
	}
	if strv(res["id"]) == "" {
		webErr(w, http.StatusBadGateway, "ساخت سرور ناموفق بود")
		return
	}
	r.lnDropServers(in.Acc)
	ip := lnFirstIP(res)
	if ip != "" && ip != "—" {
		lst := r.srvList()
		lst = append(lst, srvRec(fmt.Sprintf("s%d%d%d", time.Now().Unix(), len(lst), time.Now().Nanosecond()%900+100),
			strv(res["label"]), ip, "root", 22, "pass", pw, "Linode: "+strv(res["label"])))
		r.srvSave(lst)
	}
	webOK(w, map[string]any{
		"message": "سرور ساخته شد", "password": pw, "ip": ip,
		"server": lnWebServerObj(res),
	})
}
