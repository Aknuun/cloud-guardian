package router

import (
	"encoding/json"
	"net"
	"net/http"
	"sort"
	"strings"

	"github.com/aknuun/cg-server/internal/cf"
	"github.com/aknuun/cg-server/internal/lb"
)

// ---------- shared helpers ----------

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
