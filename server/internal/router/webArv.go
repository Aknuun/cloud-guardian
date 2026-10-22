package router

import (
	"encoding/json"
	"fmt"
	"net/http"
	"sort"
	"strings"

	"github.com/aknuun/cg-server/internal/arvan"
)

func (r *Router) arvWebCli(acc int) (*arvan.Client, error) {
	accs := r.arvanAccList()
	if acc < 0 || acc >= len(accs) {
		return nil, fmt.Errorf("اکانت پیدا نشد")
	}
	return arvan.New(strv(accs[acc]["token"])), nil
}

func arvWebDomainObj(d map[string]any) map[string]any {
	return map[string]any{
		"id":      strv(d["id"]),
		"name":    arvanName(d),
		"domain":  firstOf(strv(d["domain"]), arvanName(d)),
		"status":  firstOf(strv(d["status"]), strv(d["state"])),
		"ns":      d["ns"],
		"created": strv(d["created_at"]),
	}
}

func arvWebRecObj(rec map[string]any) map[string]any {
	val := firstOf(strv(rec["value"]), strv(rec["content"]))
	name := firstOf(strv(rec["name"]), strv(rec["host"]))
	return map[string]any{
		"id":    strv(rec["id"]),
		"type":  strings.ToUpper(strv(rec["type"])),
		"name":  name,
		"value": val,
		"ttl":   intOrAny(rec["ttl"]),
		"cloud": arBool(rec["cloud"]) || arBool(rec["cdn"]),
	}
}

func arvWebServerObj(s map[string]any) map[string]any {
	region := strv(s["region"])
	if region == "" {
		region = strv(s["region_code"])
	}
	status := strv(s["status"])
	return map[string]any{
		"id":        strv(s["id"]),
		"name":      firstOf(strv(s["name"]), strv(s["hostname"])),
		"status":    status,
		"status_fa": arvStatusFa(status),
		"task":      strv(s["task"]),
		"flavor":    firstOf(strv(s["flavor"]), strv(s["flavorID"])),
		"flavor_id": strv(s["flavorID"]),
		"image":     strv(s["image"]),
		"os":        strv(s["os"]),
		"cpu":       intOrAny(s["cpu"]),
		"ram":       intOrAny(s["ram"]),
		"disk":      intOrAny(s["disk"]),
		"region":    region,
		"pub":       selStrings(s["pub"]),
		"priv":      selStrings(s["priv"]),
		"created":   strv(s["created"]),
		"ha":        boolVal(s["ha"]),
	}
}

func (r *Router) webArvAccounts(w http.ResponseWriter, req *http.Request, uid int64) {
	accs := r.arvanAccList()
	out := make([]map[string]any, 0, len(accs))
	for i, a := range accs {
		row := map[string]any{"i": i, "name": strv(a["name"])}
		var doms []map[string]any
		if r.Store.GetJSON(fmt.Sprintf("arvdoms:%d", i), &doms) {
			row["domains"] = len(doms)
		}
		var ch arvAllCache
		if r.Store.GetJSON(fmt.Sprintf("arvall:%d", i), &ch) && ch.List != nil {
			row["servers"] = len(ch.List)
		}
		out = append(out, row)
	}
	webOK(w, map[string]any{"accounts": out})
}

func (r *Router) webArvDomains(w http.ResponseWriter, req *http.Request, uid int64) {
	acc := atoiSafe(req.URL.Query().Get("acc"))
	accs := r.arvanAccList()
	if acc < 0 || acc >= len(accs) {
		webErr(w, http.StatusBadRequest, "اکانت پیدا نشد")
		return
	}
	tok := strv(accs[acc]["token"])
	doms := r.arvCachedMaps(fmt.Sprintf("mdom:%d", acc), fmt.Sprintf("arvdoms:%d", acc), 5*60*1e9,
		func() []map[string]any { return arvan.New(tok).Domains() })
	out := make([]map[string]any, 0, len(doms))
	for _, d := range doms {
		out = append(out, arvWebDomainObj(d))
	}
	sort.Slice(out, func(i, j int) bool { return strv(out[i]["name"]) < strv(out[j]["name"]) })
	webOK(w, map[string]any{"domains": out})
}

func (r *Router) webArvRecords(w http.ResponseWriter, req *http.Request, uid int64) {
	q := req.URL.Query()
	acc := atoiSafe(q.Get("acc"))
	dom := strings.TrimSpace(q.Get("domain"))
	if dom == "" {
		webErr(w, http.StatusBadRequest, "دامنه لازم است")
		return
	}
	if _, err := r.arvWebCli(acc); err != nil {
		webErr(w, http.StatusBadRequest, err.Error())
		return
	}
	recs := r.arvDomainRecords(acc, dom)
	out := make([]map[string]any, 0, len(recs))
	for _, rec := range recs {
		out = append(out, arvWebRecObj(rec))
	}
	webOK(w, map[string]any{"records": out, "domain": dom})
}

func (r *Router) webArvRecordOp(w http.ResponseWriter, req *http.Request, uid int64) {
	if req.Method != http.MethodPost {
		webErr(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}
	var in struct {
		Op     string `json:"op"`
		Acc    int    `json:"acc"`
		Domain string `json:"domain"`
		ID     string `json:"id"`
		Type   string `json:"type"`
		Name   string `json:"name"`
		Value  string `json:"value"`
		TTL    int    `json:"ttl"`
		Cloud  *bool  `json:"cloud"`
	}
	if err := json.NewDecoder(http.MaxBytesReader(w, req.Body, 1<<20)).Decode(&in); err != nil {
		webErr(w, http.StatusBadRequest, "بدنهٔ درخواست نامعتبر است")
		return
	}
	cli, err := r.arvWebCli(in.Acc)
	if err != nil {
		webErr(w, http.StatusBadRequest, err.Error())
		return
	}
	dom := strings.TrimSpace(in.Domain)
	if dom == "" {
		webErr(w, http.StatusBadRequest, "دامنه لازم است")
		return
	}
	base := "/domains/" + dom + "/dns-records"
	drop := func() { r.arvDropRecords(in.Acc, dom) }
	switch in.Op {
	case "add":
		typ := strings.ToUpper(strings.TrimSpace(in.Type))
		name := strings.TrimSpace(in.Name)
		value := strings.TrimSpace(in.Value)
		if typ == "" || name == "" || value == "" {
			webErr(w, http.StatusBadRequest, "نوع، نام و مقدار لازم است")
			return
		}
		body := map[string]any{"type": typ, "name": name, "value": value}
		if in.TTL > 0 {
			body["ttl"] = in.TTL
		}
		if _, err := cli.Call("POST", base, body); err != nil {
			webErr(w, http.StatusBadGateway, err.Error())
			return
		}
		drop()
		webOK(w, map[string]any{"message": "رکورد اضافه شد"})
	case "edit":
		if in.ID == "" {
			webErr(w, http.StatusBadRequest, "شناسهٔ رکورد لازم است")
			return
		}
		body := map[string]any{}
		if in.Type != "" {
			body["type"] = strings.ToUpper(strings.TrimSpace(in.Type))
		}
		if in.Name != "" {
			body["name"] = strings.TrimSpace(in.Name)
		}
		if in.Value != "" {
			body["value"] = strings.TrimSpace(in.Value)
		}
		if in.TTL > 0 {
			body["ttl"] = in.TTL
		}
		if len(body) == 0 {
			webErr(w, http.StatusBadRequest, "تغییری ارسال نشده")
			return
		}
		if _, err := cli.Call("PUT", base+"/"+in.ID, body); err != nil {
			webErr(w, http.StatusBadGateway, err.Error())
			return
		}
		drop()
		webOK(w, map[string]any{"message": "رکورد ویرایش شد"})
	case "delete":
		if in.ID == "" {
			webErr(w, http.StatusBadRequest, "شناسهٔ رکورد لازم است")
			return
		}
		if _, err := cli.Call("DELETE", base+"/"+in.ID, nil); err != nil {
			webErr(w, http.StatusBadGateway, err.Error())
			return
		}
		drop()
		webOK(w, map[string]any{"message": "رکورد حذف شد"})
	case "cloud":
		if in.ID == "" || in.Cloud == nil {
			webErr(w, http.StatusBadRequest, "شناسه و وضعیت ابری لازم است")
			return
		}
		if _, err := cli.Call("PUT", base+"/"+in.ID, map[string]any{"cloud": *in.Cloud}); err != nil {
			webErr(w, http.StatusBadGateway, err.Error())
			return
		}
		drop()
		state := "روشن"
		if !*in.Cloud {
			state = "خاموش"
		}
		webOK(w, map[string]any{"message": "حالت ابری " + state + " شد"})
	default:
		webErr(w, http.StatusBadRequest, "عملیات ناشناخته")
	}
}

func (r *Router) webArvServers(w http.ResponseWriter, req *http.Request, uid int64) {
	acc := atoiSafe(req.URL.Query().Get("acc"))
	if _, err := r.arvWebCli(acc); err != nil {
		webErr(w, http.StatusBadRequest, err.Error())
		return
	}
	list, stale := r.arvAllServersSWR(acc)
	if list == nil {
		list = []map[string]any{}
	}
	out := make([]map[string]any, 0, len(list))
	for _, s := range list {
		row := arvWebServerObj(s)
		row["region_name"] = r.arvRegionName(strv(row["region"]))
		out = append(out, row)
	}
	sort.Slice(out, func(i, j int) bool {
		ri, rj := strv(out[i]["region"]), strv(out[j]["region"])
		if ri != rj {
			return ri < rj
		}
		return strings.ToLower(strv(out[i]["name"])) < strings.ToLower(strv(out[j]["name"]))
	})
	webOK(w, map[string]any{"servers": out, "stale": stale})
}

func (r *Router) webArvOptions(w http.ResponseWriter, req *http.Request, uid int64) {
	q := req.URL.Query()
	acc := atoiSafe(q.Get("acc"))
	region := strings.TrimSpace(q.Get("region"))
	cli, err := r.arvWebCli(acc)
	if err != nil {
		webErr(w, http.StatusBadRequest, err.Error())
		return
	}
	regions := make([]map[string]any, 0)
	for _, rg := range r.arvRegionsFast(acc) {
		regions = append(regions, map[string]any{"id": rg.Code, "name": rg.Fa})
	}
	sizes := make([]map[string]any, 0)
	images := make([]map[string]any, 0)
	firewalls := make([]map[string]any, 0)
	if region != "" {
		for _, sz := range r.arvSizes(acc, region) {
			sizes = append(sizes, map[string]any{
				"id": sz.ID, "name": sz.Name, "cpu": sz.CPU, "ram": sz.RAM,
				"disk": sz.Disk, "price": sz.Price, "price_text": arvToman(sz.Price),
			})
		}
		for _, im := range r.arvImages(acc, region) {
			m, _ := im.(map[string]any)
			if m != nil {
				images = append(images, map[string]any{"id": strv(m["id"]), "name": strv(m["name"])})
			}
		}
		for _, g := range cli.EccList(region, "/securities") {
			if id := strv(g["id"]); id != "" {
				firewalls = append(firewalls, map[string]any{"id": id, "name": firstOf(strv(g["name"]), id)})
			}
		}
	}
	webOK(w, map[string]any{"regions": regions, "sizes": sizes, "images": images, "firewalls": firewalls})
}

func (r *Router) webArvSnapshots(w http.ResponseWriter, req *http.Request, uid int64) {
	q := req.URL.Query()
	acc := atoiSafe(q.Get("acc"))
	region := strings.TrimSpace(q.Get("region"))
	cli, err := r.arvWebCli(acc)
	if err != nil {
		webErr(w, http.StatusBadRequest, err.Error())
		return
	}
	if region == "" {
		webErr(w, http.StatusBadRequest, "منطقه لازم است")
		return
	}
	out := make([]map[string]any, 0)
	for _, s := range cli.EccList(region, "/snapshots") {
		out = append(out, map[string]any{
			"id": strv(s["id"]), "name": firstOf(strv(s["name"]), strv(s["id"])),
			"size": intOrAny(s["size"]), "status": strv(s["status"]), "created": strv(s["created"]),
		})
	}
	webOK(w, map[string]any{"snapshots": out})
}

func (r *Router) webArvCreate(w http.ResponseWriter, req *http.Request, uid int64) {
	if req.Method != http.MethodPost {
		webErr(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}
	var in struct {
		Acc      int    `json:"acc"`
		Region   string `json:"region"`
		Name     string `json:"name"`
		Size     string `json:"size"`
		Image    string `json:"image"`
		Firewall string `json:"firewall"`
	}
	if err := json.NewDecoder(http.MaxBytesReader(w, req.Body, 1<<20)).Decode(&in); err != nil {
		webErr(w, http.StatusBadRequest, "بدنهٔ درخواست نامعتبر است")
		return
	}
	name := arvSanitizeName(in.Name)
	if name == "" || len([]rune(name)) > 60 {
		webErr(w, http.StatusBadRequest, "نام سرور نامعتبر است (حروف انگلیسی، عدد و خط تیره)")
		return
	}
	if in.Region == "" || in.Size == "" || in.Image == "" {
		webErr(w, http.StatusBadRequest, "منطقه، پلن و تصویر لازم است")
		return
	}
	if _, err := r.arvWebCli(in.Acc); err != nil {
		webErr(w, http.StatusBadRequest, err.Error())
		return
	}
	disk := 0
	for _, sz := range r.arvSizes(in.Acc, in.Region) {
		if sz.ID == in.Size {
			disk = sz.Disk
			break
		}
	}
	fw := firstOf(strings.TrimSpace(in.Firewall), "Default")
	d, err := r.arvCreateServer(in.Acc, in.Region, name, in.Size, in.Image, fw, disk)
	if err != nil {
		if msg := arvCreditErr(err.Error()); msg != "" {
			webErr(w, http.StatusPaymentRequired, msg)
			return
		}
		webErr(w, http.StatusBadGateway, err.Error())
		return
	}
	if msg := strv(d["message"]); msg == "Unauthenticated." {
		webErr(w, http.StatusForbidden, "کلید دسترسی این عمل را ندارد")
		return
	}
	one := arvan.EccOne(d)
	pw := ""
	if one != nil {
		pw = firstOf(one["password"], strv(one["root_password"]))
	}
	r.Store.Delete(fmt.Sprintf("arvall:%d", in.Acc))
	arvMem.Delete(fmt.Sprintf("msrv:%d", in.Acc))
	webOK(w, map[string]any{"message": "درخواست ساخت سرور ارسال شد", "password": pw})
}

func (r *Router) webArvAction(w http.ResponseWriter, req *http.Request, uid int64) {
	if req.Method != http.MethodPost {
		webErr(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}
	var in struct {
		Acc     int    `json:"acc"`
		Region  string `json:"region"`
		ID      string `json:"id"`
		Action  string `json:"action"`
		Arg     string `json:"arg"`
		Confirm bool   `json:"confirm"`
	}
	if err := json.NewDecoder(http.MaxBytesReader(w, req.Body, 1<<20)).Decode(&in); err != nil {
		webErr(w, http.StatusBadRequest, "بدنهٔ درخواست نامعتبر است")
		return
	}
	if in.Region == "" || in.ID == "" || in.Action == "" {
		webErr(w, http.StatusBadRequest, "پارامتر ناقص")
		return
	}
	cli, err := r.arvWebCli(in.Acc)
	if err != nil {
		webErr(w, http.StatusBadRequest, err.Error())
		return
	}
	destructive := map[string]bool{
		"delete": true, "snapshot": true, "snapshot_delete": true,
		"rename": true, "reset_root": true,
	}
	if destructive[in.Action] && !in.Confirm {
		webErr(w, http.StatusPreconditionRequired, "این عملیات نیاز به تأیید دارد")
		return
	}
	drop := func() {
		r.Store.Delete(fmt.Sprintf("arvall:%d", in.Acc))
		arvMem.Delete(fmt.Sprintf("msrv:%d", in.Acc))
	}
	switch in.Action {
	case "power-on", "power-off", "reboot":
		if _, err := cli.Ecc(in.Region, "POST", "/servers/"+in.ID+"/"+in.Action, nil); err != nil {
			webErr(w, http.StatusBadGateway, err.Error())
			return
		}
		drop()
		webOK(w, map[string]any{"message": "دستور ارسال شد"})
	case "delete":
		if _, err := cli.Ecc(in.Region, "DELETE", "/servers/"+in.ID, nil); err != nil {
			webErr(w, http.StatusBadGateway, err.Error())
			return
		}
		drop()
		webOK(w, map[string]any{"message": "سرور حذف شد"})
	case "rename":
		name := arvSanitizeName(in.Arg)
		if name == "" {
			webErr(w, http.StatusBadRequest, "نام نامعتبر است")
			return
		}
		if _, err := cli.Ecc(in.Region, "POST", "/servers/"+in.ID+"/rename", map[string]any{"name": name}); err != nil {
			webErr(w, http.StatusBadGateway, err.Error())
			return
		}
		drop()
		webOK(w, map[string]any{"message": "نام تغییر کرد"})
	case "snapshot":
		name := strings.TrimSpace(in.Arg)
		if name == "" {
			name = "snap-" + in.ID
		}
		if _, err := cli.Ecc(in.Region, "POST", "/servers/"+in.ID+"/snapshot", map[string]any{"name": name}); err != nil {
			webErr(w, http.StatusBadGateway, err.Error())
			return
		}
		webOK(w, map[string]any{"message": "درخواست اسنپ‌شات ارسال شد"})
	case "snapshot_delete":
		if in.Arg == "" {
			webErr(w, http.StatusBadRequest, "شناسهٔ اسنپ‌شات لازم است")
			return
		}
		if _, err := cli.Ecc(in.Region, "DELETE", "/snapshots/"+in.Arg, nil); err != nil {
			webErr(w, http.StatusBadGateway, err.Error())
			return
		}
		webOK(w, map[string]any{"message": "اسنپ‌شات حذف شد"})
	case "reset_root":
		d, err := cli.Ecc(in.Region, "POST", "/servers/"+in.ID+"/reset-root-password", map[string]any{})
		if err != nil {
			webErr(w, http.StatusBadGateway, err.Error())
			return
		}
		pw := ""
		if one := arvan.EccOne(d); one != nil {
			pw = firstOf(one["password"], strv(one["root_password"]))
		}
		webOK(w, map[string]any{"message": "درخواست ریست رمز ارسال شد", "password": pw})
	default:
		webErr(w, http.StatusBadRequest, "عملیات ناشناخته")
	}
}
