package router

import (
	"fmt"
	"strings"
	"sync"
	"time"

	"github.com/aknuun/cg-server/internal/arvan"
)

// arvanFA maps ArvanCloud region country/city/dc values to Persian names.
var arvanFA = map[string]string{
	"Iran": "ایران", "Germany": "آلمان", "Netherlands": "هلند",
	"Tehran": "تهران", "Tabriz": "تبریز", "Ahwaz": "اهواز", "Isfahan": "اصفهان",
	"Shiraz": "شیراز", "Mashhad": "مشهد", "Karlsruhe": "کارلسروهه", "Amsterdam": "آمستردام",
	"Bamdad": "بامداد", "Foroogh": "فروغ", "Forough": "فروغ", "Shahriar": "شهریار",
	"Qeysar": "قیصر", "Simin": "سیمین", "Goethe": "گوته", "Atieh": "عطیه",
}

var arvanFlagFA = map[string]string{
	"Iran": "🇮🇷", "ir": "🇮🇷", "IR": "🇮🇷",
	"Germany": "🇩🇪", "de": "🇩🇪", "DE": "🇩🇪",
	"Netherlands": "🇳🇱", "nl": "🇳🇱", "NL": "🇳🇱",
}

func arvFA(s string) string {
	if v, ok := arvanFA[s]; ok {
		return v
	}
	return s
}

// arvRegionFa builds a full Persian location label from a region object.
func arvRegionFa(m map[string]any) string {
	country := arvFA(strv(m["country"]))
	flag := arvanFlagFA[strv(m["flag"])]
	if flag == "" {
		flag = arvanFlagFA[strv(m["country"])]
	}
	city := arvFA(strv(m["city"]))
	dc := arvFA(strv(m["dc"]))
	var loc []string
	if country != "" {
		loc = append(loc, country)
	}
	if city != "" && city != country {
		loc = append(loc, city)
	}
	if dc != "" && dc != city {
		loc = append(loc, "دیتاسنتر "+dc)
	}
	if len(loc) == 0 {
		return firstOf(m["code"], strv(m["id"]))
	}
	head := strings.Join(loc, " · ")
	if flag != "" {
		return flag + " " + head
	}
	return head
}

// arvRegion is a cached Arvan region with a Persian label.
type arvRegion struct {
	Code string `json:"code"`
	Fa   string `json:"fa"`
	Def  bool   `json:"def"`
}

func arvFetchRegions(token string) []arvRegion {
	d, err := arvan.New(token).EccRegions()
	if err != nil {
		return nil
	}
	var out []arvRegion
	for _, m := range arvan.EccOneList(d) {
		if b, ok := m["visible"].(bool); ok && !b {
			continue
		}
		if b, ok := m["create"].(bool); ok && !b {
			continue
		}
		code := firstOf(m["code"], strv(m["id"]))
		if code == "" {
			continue
		}
		out = append(out, arvRegion{Code: code, Fa: arvRegionFa(m), Def: boolVal(m["default"])})
	}
	return out
}

func arvFallbackRegions() []arvRegion {
	fa := map[string]string{
		"ir-thr-ba1":      "🇮🇷 ایران · تهران · دیتاسنتر بامداد",
		"ir-thr-fr1":      "🇮🇷 ایران · تهران · دیتاسنتر فروغ",
		"ir-tbz-sh1":      "🇮🇷 ایران · تبریز · دیتاسنتر شهریار",
		"ir-thr-si1":      "🇮🇷 ایران · تهران · دیتاسنتر سیمین",
		"ir-southwest1-a": "🇮🇷 ایران · اهواز",
		"eu-west1-a":      "🇩🇪 آلمان · کارلسروهه · دیتاسنتر گوته",
	}
	var out []arvRegion
	for _, c := range arvanFallbackRegions {
		out = append(out, arvRegion{Code: c, Fa: firstOf(fa[c], c)})
	}
	return out
}

// arvRegionCache is the JSON shape stored in the KV store.
type arvRegionCache struct {
	Regs []arvRegion `json:"regs"`
	T    int64       `json:"t"`
}

// arvRegions returns an account's regions, using a 2-day cache to keep the
// first (slow) API call from repeating on every menu open.
func (r *Router) arvRegions(i int, force bool) []arvRegion {
	accs := r.arvAccounts()
	if i < 0 || i >= len(accs) {
		return nil
	}
	key := fmt.Sprintf("arv_regs:%d", i)
	if !force {
		var ch arvRegionCache
		if r.Store.GetJSON(key, &ch) && len(ch.Regs) > 0 {
			return ch.Regs
		}
	}
	regs := arvFetchRegions(strv(accs[i]["token"]))
	if len(regs) == 0 {
		regs = arvFallbackRegions()
		// Cache the fallback briefly so a failing/throttled API does not add
		// 8-16s of latency to every screen open.
		r.Store.PutJSON(key, arvRegionCache{Regs: regs, T: time.Now().Unix()}, 600)
		return regs
	}
	r.Store.PutJSON(key, arvRegionCache{Regs: regs, T: time.Now().Unix()}, 2*86400)
	return regs
}

var arvRegBusy sync.Map

// arvRegionsFast never blocks: it serves the cached regions (or the built-in
// fallback) and refreshes them in the background.
func (r *Router) arvRegionsFast(i int) []arvRegion {
	var ch arvRegionCache
	if r.Store.GetJSON(fmt.Sprintf("arv_regs:%d", i), &ch) && len(ch.Regs) > 0 {
		if time.Now().Unix()-ch.T > 86400 {
			if _, busy := arvRegBusy.LoadOrStore(i, true); !busy {
				go func() { defer arvRegBusy.Delete(i); r.arvRegions(i, true) }()
			}
		}
		return ch.Regs
	}
	if _, busy := arvRegBusy.LoadOrStore(i, true); !busy {
		go func() { defer arvRegBusy.Delete(i); r.arvRegions(i, true) }()
	}
	return arvFallbackRegions()
}

// arvRegionName returns the Persian label for a region code (from any cached
// account), falling back to the fallback map and then the raw code.
func (r *Router) arvRegionName(code string) string {
	if code == "" {
		return ""
	}
	for i := range r.arvAccounts() {
		var ch arvRegionCache
		if r.Store.GetJSON(fmt.Sprintf("arv_regs:%d", i), &ch) {
			for _, rg := range ch.Regs {
				if rg.Code == code {
					return rg.Fa
				}
			}
		}
	}
	for _, rg := range arvFallbackRegions() {
		if rg.Code == code {
			return rg.Fa
		}
	}
	return code
}

// arvStatusFa maps an Arvan status string to a Persian label.
func arvStatusFa(st string) string {
	s := strings.ToLower(st)
	switch {
	case s == "":
		return "نامشخص"
	case strings.Contains(s, "active"), strings.Contains(s, "run"):
		return "روشن"
	case strings.Contains(s, "build"), strings.Contains(s, "creat"), strings.Contains(s, "pending"):
		return "در حال ساخت"
	case strings.Contains(s, "rebuild"):
		return "ری‌بیلد"
	case strings.Contains(s, "off"), strings.Contains(s, "stop"), strings.Contains(s, "shut"):
		return "خاموش"
	case strings.Contains(s, "suspend"), strings.Contains(s, "pause"):
		return "معلق"
	case strings.Contains(s, "error"), strings.Contains(s, "fail"):
		return "خطا"
	default:
		return st
	}
}
