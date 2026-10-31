package router

import (
	"encoding/json"
	"net/http"
	"strings"
)

// ---------- shared helpers ----------

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
