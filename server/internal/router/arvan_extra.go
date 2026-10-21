package router

import (
	"fmt"
	"strings"
	"sync"
	"time"

	"github.com/aknuun/cg-server/internal/hostfilter"
)

// ---------- Arvan ECC: addresses, all-regions list, traffic, resize ----------

func arvAddrEntries(v any) []map[string]any {
	var out []map[string]any
	switch t := v.(type) {
	case []any:
		for _, x := range t {
			if m, ok := x.(map[string]any); ok {
				out = append(out, m)
			}
		}
	case map[string]any:
		out = append(out, t)
	}
	return out
}

func arvServerIPs(s map[string]any) (pub, priv []string) {
	add := func(ip string, public bool) {
		if ip == "" {
			return
		}
		if public {
			if !containsStr(pub, ip) {
				pub = append(pub, ip)
			}
			return
		}
		if !containsStr(priv, ip) && !containsStr(pub, ip) {
			priv = append(priv, ip)
		}
	}
	if addrs, ok := s["addresses"].(map[string]any); ok {
		for k, v := range addrs {
			lk := strings.ToLower(k)
			public := !strings.Contains(lk, "priv") && !strings.Contains(lk, "vpc") && !strings.Contains(lk, "internal")
			for _, e := range arvAddrEntries(v) {
				ip := firstOf(e["addr"], firstOf(e["ip"], strv(e["address"])))
				flag := public
				if b, ok := e["is_public"].(bool); ok {
					flag = b
				}
				if strings.EqualFold(strv(e["type"]), "private") || boolVal(e["is_vpc"]) {
					flag = false
				}
				add(ip, flag)
			}
		}
	} else if arr, ok := s["addresses"].([]any); ok {
		for _, x := range arr {
			m, _ := x.(map[string]any)
			if m == nil {
				continue
			}
			flag := true
			if b, ok := m["is_public"].(bool); ok {
				flag = b
			}
			if strings.EqualFold(strv(m["type"]), "private") || boolVal(m["is_vpc"]) {
				flag = false
			}
			add(firstOf(m["addr"], firstOf(m["ip"], strv(m["address"]))), flag)
		}
	}
	for _, x := range toList(s["networks"]) {
		m, _ := x.(map[string]any)
		if m == nil {
			continue
		}
		ip := firstOf(m["ip"], firstOf(m["addr"], strv(m["address"])))
		if ip == "" || containsStr(pub, ip) || containsStr(priv, ip) {
			continue
		}
		flag := true
		if b, ok := m["public"].(bool); ok {
			flag = b
		}
		add(ip, flag)
	}
	return pub, priv
}

type arvAllCache struct {
	T    int64            `json:"t"`
	List []map[string]any `json:"list"`
}

func (r *Router) arvFetchAll(i int) []map[string]any {
	cli, ok := r.arvCli(i)
	if !ok {
		return nil
	}
	var mu sync.Mutex
	var wg sync.WaitGroup
	var out []map[string]any
	for _, rg := range r.arvRegions(i, false) {
		wg.Add(1)
		go func(code string) {
			defer wg.Done()
			var rows []map[string]any
			for _, s := range cli.EccList(code, "/servers") {
				n := arvNormalize(s)
				if n == nil {
					continue
				}
				n["region"] = code
				rows = append(rows, n)
			}
			mu.Lock()
			out = append(out, rows...)
			mu.Unlock()
		}(rg.Code)
	}
	wg.Wait()
	for a := 0; a < len(out); a++ {
		for b := a + 1; b < len(out); b++ {
			if strv(out[b]["name"]) < strv(out[a]["name"]) {
				out[a], out[b] = out[b], out[a]
			}
		}
	}
	return out
}

func (r *Router) arvAllServersSWR(i int) ([]map[string]any, bool) {
	key := fmt.Sprintf("msrv:%d", i)
	if _, ok := arvMem.Load(key); !ok {
		var ch arvAllCache
		if r.Store.GetJSON(fmt.Sprintf("arvall:%d", i), &ch) && ch.List != nil {
			arvMem.Store(key, &arvCacheBox{v: ch.List, at: time.Unix(ch.T, 0)})
		}
	}
	v, stale := arvCached(key, 90*time.Second, func() any {
		l := r.arvFetchAll(i)
		if l == nil {
			l = []map[string]any{}
		}
		r.Store.PutJSON(fmt.Sprintf("arvall:%d", i), arvAllCache{T: time.Now().Unix(), List: l}, 0)
		return l
	})
	out, _ := v.([]map[string]any)
	return out, stale
}

func (r *Router) arvAllServers(i int, force bool) []map[string]any {
	key := fmt.Sprintf("arvall:%d", i)
	if !force {
		var ch arvAllCache
		if r.Store.GetJSON(key, &ch) && ch.List != nil && time.Now().Unix()-ch.T < 120 {
			return ch.List
		}
	}
	list := r.arvFetchAll(i)
	if list == nil {
		list = []map[string]any{}
	}
	r.Store.PutJSON(key, arvAllCache{T: time.Now().Unix(), List: list}, 0)
	arvMem.Store(fmt.Sprintf("msrv:%d", i), &arvCacheBox{v: list, at: time.Now()})
	return list
}

// ---------- in-memory SWR cache ----------

type arvCacheBox struct {
	mu      sync.Mutex
	v       any
	at      time.Time
	loading bool
}

var arvMem sync.Map

var arvHomeBusy sync.Map

var arvDetailBusy sync.Map

// arvCached returns a cached value and whether it is stale. A stale value is
// served immediately while a background refresh runs (stale-while-revalidate).
func arvCached(key string, ttl time.Duration, load func() any) (any, bool) {
	e, _ := arvMem.LoadOrStore(key, &arvCacheBox{})
	b := e.(*arvCacheBox)
	b.mu.Lock()
	if b.v != nil {
		if time.Since(b.at) < ttl {
			v := b.v
			b.mu.Unlock()
			return v, false
		}
		v := b.v
		if !b.loading {
			b.loading = true
			go func() {
				nv := load()
				b.mu.Lock()
				if nv != nil {
					b.v, b.at = nv, time.Now()
				}
				b.loading = false
				b.mu.Unlock()
			}()
		}
		b.mu.Unlock()
		return v, true
	}
	b.loading = true
	b.mu.Unlock()
	nv := load()
	b.mu.Lock()
	if nv != nil {
		b.v, b.at = nv, time.Now()
	}
	b.loading = false
	b.mu.Unlock()
	return nv, false
}

// arvCachedAsync serves a cached value without blocking; a missing value is
// loaded in the background and the second return value is false until ready.
func arvCachedAsync(key string, ttl time.Duration, load func() any) (any, bool) {
	e, _ := arvMem.LoadOrStore(key, &arvCacheBox{})
	b := e.(*arvCacheBox)
	b.mu.Lock()
	if b.v != nil {
		v := b.v
		if time.Since(b.at) >= ttl && !b.loading {
			b.loading = true
			go func() {
				nv := load()
				b.mu.Lock()
				if nv != nil {
					b.v, b.at = nv, time.Now()
				}
				b.loading = false
				b.mu.Unlock()
			}()
		}
		b.mu.Unlock()
		return v, true
	}
	if !b.loading {
		b.loading = true
		go func() {
			nv := load()
			b.mu.Lock()
			if nv != nil {
				b.v, b.at = nv, time.Now()
			}
			b.loading = false
			b.mu.Unlock()
		}()
	}
	b.mu.Unlock()
	return nil, false
}

// arvNav is the standard Arvan navigation bar: one tap to any main screen.
func arvNav(i int) [][]Btn {
	return [][]Btn{
		{btn("🖥 سرورها", "arvsrv"), btn("➕ خرید سرور", fmt.Sprintf("arvbuy:%d", i))},
		{btn("🎯 شکار IP تمیز", fmt.Sprintf("arvhunt:%d", i)), btn("📍 ریجن‌ها", fmt.Sprintf("arvregs:%d", i))},
	}
}

func (r *Router) arvImageGroups(i int, region string) []map[string]any {
	v, _ := arvCached(fmt.Sprintf("mimg:%d:%s", i, region), 6*time.Hour, func() any {
		cli, ok := r.arvCli(i)
		if !ok {
			return []map[string]any{}
		}
		out := cli.EccList(region, "/images?type=distributions&per_page=50")
		if out == nil {
			out = []map[string]any{}
		}
		return out
	})
	out, _ := v.([]map[string]any)
	return out
}

// arvCreateServer builds the Arvan create payload (firewall by UUID, network
// ids discovered from existing servers) and retries with the alternate
// flavor/image keys when the API rejects them.
func (r *Router) arvCreateServer(i int, region, name, flavorID, imageID, fwName string, disk int) (map[string]any, error) {
	cli, ok := r.arvCli(i)
	if !ok {
		return nil, fmt.Errorf("account not found")
	}
	base := map[string]any{
		"name":            name,
		"enable_ipv4":     true,
		"security_groups": []map[string]any{{"name": r.arvFwID(i, region, fwName)}},
	}
	if nids := r.arvNetworkIDs(i, region); len(nids) > 0 {
		base["network_ids"] = nids
	}
	if disk > 0 {
		base["disk_size"] = disk
	}
	var last error
	for _, keys := range [][2]string{{"flavorRef", "imageRef"}, {"flavor_id", "image_id"}} {
		body := map[string]any{}
		for k, v := range base {
			body[k] = v
		}
		body[keys[0]] = flavorID
		body[keys[1]] = imageID
		d, err := cli.Ecc(region, "POST", "/servers", body)
		if err == nil {
			return d, nil
		}
		last = err
		low := strings.ToLower(err.Error())
		if !strings.Contains(low, "plan") && !strings.Contains(low, "flavor") {
			break
		}
	}
	return nil, last
}

func (r *Router) arvFwID(i int, region, name string) string {
	if name == "" {
		return name
	}
	v, _ := arvCached(fmt.Sprintf("mfw:%d:%s", i, region), 10*time.Minute, func() any {
		cli, ok := r.arvCli(i)
		if !ok {
			return map[string]string{}
		}
		out := map[string]string{}
		for _, g := range cli.EccList(region, "/securities") {
			nm, id := strv(g["name"]), strv(g["id"])
			if nm != "" && id != "" {
				out[nm] = id
			}
		}
		return out
	})
	if m, ok := v.(map[string]string); ok {
		if id := m[name]; id != "" {
			return id
		}
	}
	return name
}

func (r *Router) arvNetworkIDs(i int, region string) []string {
	v, _ := arvCached(fmt.Sprintf("mnet:%d:%s", i, region), time.Hour, func() any {
		cli, ok := r.arvCli(i)
		if !ok {
			return []string{}
		}
		out := []string{}
		for _, m := range cli.EccList(region, "/networks") {
			if id := strv(m["id"]); id != "" {
				out = append(out, id)
			}
		}
		if len(out) == 0 {
			for _, s := range r.arvAllServers(i, false) {
				if strv(s["region"]) != region {
					continue
				}
				for _, n := range toList(s["networks"]) {
					if id := strv(n); id != "" {
						out = append(out, id)
					}
				}
			}
		}
		if len(out) == 0 {
			return nil
		}
		return out
	})
	out, _ := v.([]string)
	return out
}

func (r *Router) arvRegionButtons(i int, cb string) [][]Btn {
	var kb [][]Btn
	for _, rg := range r.arvRegions(i, false) {
		label := rg.Fa
		if rg.Def {
			label += " ✅"
		}
		kb = append(kb, []Btn{btn(truncRune(label, 55), fmt.Sprintf("%s:%d:%s", cb, i, rg.Code))})
	}
	return kb
}

func (r *Router) arvBuyRegions(chat, mid int64, i int) {
	kb := r.arvRegionButtons(i, "arvbuy")
	kb = append(kb, arvNav(i)...)
	r.Show(chat, mid, "➕ خرید سرور جدید\n\n📍 ریجن را انتخاب کن:", kb)
}

func (r *Router) arvBrowseRegions(chat, mid int64, i int) {
	kb := r.arvRegionButtons(i, "arvreg")
	kb = append(kb, navRow("arvsrv"))
	r.Show(chat, mid, "📍 ریجن‌ها\n\nبرای دیدن سرورهای هر ریجن انتخاب کن:", kb)
}

func (r *Router) arvPick(chat, mid int64, i, j int) {
	list := r.arvAllServers(i, false)
	if j < 0 || j >= len(list) {
		r.Show(chat, mid, "⏳ نشست منقضی شد؛ دوباره تلاش کن.", navKB("arvsrv"))
		return
	}
	sess := r.arvSess(chat)
	sess["acc"] = i
	sess["region"] = strv(list[j]["region"])
	sess["servers"] = list
	r.arvSessSave(chat, sess)
	r.arvServerDetail(chat, mid, i, j)
}

func arvFmtBytes(v float64) string {
	if v <= 0 {
		return "۰"
	}
	units := []string{"B", "KB", "MB", "GB", "TB"}
	u := 0
	for v >= 1024 && u < len(units)-1 {
		v /= 1024
		u++
	}
	if v >= 100 {
		return fmt.Sprintf("%.0f %s", v, units[u])
	}
	return fmt.Sprintf("%.1f %s", v, units[u])
}

func (r *Router) arvTraffic(i int, region, id string) (inB, outB float64, ok bool) {
	cli, good := r.arvCli(i)
	if !good {
		return 0, 0, false
	}
	now := time.Now().Unix()
	path := fmt.Sprintf("/reports/traffic?servers=%s&start=%d&end=%d", id, now-30*86400, now)
	type res struct {
		in, out float64
		ok      bool
	}
	ch := make(chan res, 1)
	go func() {
		defer func() { recover() }()
		d, err := cli.Ecc(region, "GET", path, nil)
		if err != nil || d == nil {
			ch <- res{}
			return
		}
		data, _ := d["data"].(map[string]any)
		charts, _ := data["charts"].(map[string]any)
		netw, _ := charts["network"].(map[string]any)
		if netw == nil {
			ch <- res{}
			return
		}
		var in, out float64
		seen := false
		for _, x := range toList(netw["series"]) {
			m, _ := x.(map[string]any)
			if m == nil {
				continue
			}
			name := strings.ToLower(strv(m["name"]))
			var sum float64
			for _, v := range toList(m["data"]) {
				if f, ok := v.(float64); ok {
					sum += f
				}
			}
			if sum > 0 {
				seen = true
			}
			if strings.Contains(name, "out") {
				out += sum
			} else {
				in += sum
			}
		}
		ch <- res{in: in, out: out, ok: seen}
	}()
	select {
	case v := <-ch:
		return v.in, v.out, v.ok
	case <-time.After(4 * time.Second):
		return 0, 0, false
	}
}

func (r *Router) arvTrafficLine(i int, region, id string) string {
	inB, outB, ok := r.arvTraffic(i, region, id)
	if !ok {
		return "📊 ترافیک ۳۰ روز: <i>داده‌ای ثبت نشده</i>"
	}
	return fmt.Sprintf("📊 ترافیک ۳۰ روز: ⬇️ %s · ⬆️ %s", arvFmtBytes(inB), arvFmtBytes(outB))
}

type arvTrafficVal struct {
	In, Out float64
	OK      bool
}

// arvTrafficLineAsync never blocks: it returns the cached line and whether it
// is already available; the first call starts a background fetch.
func (r *Router) arvTrafficLineAsync(i int, region, id string) (string, bool) {
	if id == "" {
		return "📊 ترافیک ۳۰ روز: <i>داده‌ای ثبت نشده</i>", true
	}
	v, ok := arvCachedAsync(fmt.Sprintf("mtrf:%d:%s", i, id), 10*time.Minute, func() any {
		in, out, good := r.arvTraffic(i, region, id)
		return arvTrafficVal{In: in, Out: out, OK: good}
	})
	if !ok {
		return "📊 ترافیک ۳۰ روز: ⏳ در حال دریافت…", false
	}
	tv, _ := v.(arvTrafficVal)
	if !tv.OK {
		return "📊 ترافیک ۳۰ روز: <i>داده‌ای ثبت نشده</i>", true
	}
	return fmt.Sprintf("📊 ترافیک ۳۰ روز: ⬇️ %s · ⬆️ %s", arvFmtBytes(tv.In), arvFmtBytes(tv.Out)), true
}

func (r *Router) arvIPCheck(chat int64, i, idx int) {
	sess := r.arvSess(chat)
	list := asMapList(sess["servers"])
	if idx < 0 || idx >= len(list) {
		r.Bot.Send(chat, "⏳ نشست منقضی شد.", navKB("arvsrv"))
		return
	}
	ip := firstPubIP(list[idx])
	if ip == "" {
		r.Bot.Send(chat, "❌ این سرور IPv4 عمومی ندارد.", navKB(fmt.Sprintf("arvs:%d:%d", i, idx)))
		return
	}
	r.Bot.Send(chat, "🧪 بررسی IP <code>"+Esc(ip)+"</code> از ایران و آلمان/هلند… (۳۰ تا ۶۰ ثانیه)", nil)
	go func() {
		defer func() { recover() }()
		clean, verdict := hostfilter.CheckIPCleanliness(ip)
		head := "🔴 IP تمیز نیست"
		if clean {
			head = "🟢 IP تمیز است"
		}
		r.Bot.Send(chat, fmt.Sprintf("%s\n🌐 <code>%s</code>\n📝 %s", head, Esc(ip), Esc(verdict)),
			navKB(fmt.Sprintf("arvs:%d:%d", i, idx)))
	}()
}

func arvToman(v any) string {
	n := intOrAny(v)
	if n <= 0 {
		return ""
	}
	n = n / 10
	s := fmt.Sprintf("%d", n)
	var parts []string
	for len(s) > 3 {
		parts = append([]string{s[len(s)-3:]}, parts...)
		s = s[:len(s)-3]
	}
	parts = append([]string{s}, parts...)
	return strings.Join(parts, ",") + " تومان/ماه"
}

type arvSize struct {
	ID, Name   string
	CPU, RAM   int
	Disk       int
	Price      int
	PricePerHr int
}

func (r *Router) arvSizes(i int, region string) []arvSize {
	key := fmt.Sprintf("msz:%d:%s", i, region)
	v, _ := arvCached(key, 30*time.Minute, func() any {
		cli, ok := r.arvCli(i)
		if !ok {
			return []arvSize{}
		}
		out := []arvSize{}
		for _, m := range cli.EccList(region, "/sizes") {
			id := strv(m["id"])
			if id == "" {
				continue
			}
			cpu := intOrAny(m["cpu_count"])
			if cpu == 0 {
				cpu = intOrAny(m["vcpus"])
			}
			if cpu == 0 {
				cpu = intOrAny(m["cpu"])
			}
			ram := intOrAny(m["memory"])
			if ram == 0 {
				ram = intOrAny(m["ram"])
			}
			if ram >= 512 {
				ram /= 1024
			}
			price := intOrAny(m["price_per_month"])
			if price == 0 {
				price = intOrAny(m["price"])
			}
			out = append(out, arvSize{
				ID: id, Name: firstOf(m["name"], id),
				CPU: cpu, RAM: ram, Disk: intOrAny(m["disk"]),
				Price: price, PricePerHr: intOrAny(m["price_per_hour"]),
			})
		}
		if len(out) == 0 {
			return nil
		}
		r.Store.PutJSON("mszdb:"+key, out, 0)
		return out
	})
	if out, ok := v.([]arvSize); ok && len(out) > 0 {
		return out
	}
	var db []arvSize
	if r.Store.GetJSON("mszdb:"+key, &db) && len(db) > 0 {
		return db
	}
	return arvFallbackSizes()
}

// arvFallbackSizes keeps the create wizard usable when the sizes API is
// throttled or returns 500.
func arvFallbackSizes() []arvSize {
	return []arvSize{
		{ID: "eco-1-1-0", Name: "eco-small1", CPU: 1, RAM: 1, Disk: 25},
		{ID: "eco-2-1-0", Name: "eco-small2", CPU: 2, RAM: 1, Disk: 25},
		{ID: "eco-2-2-0", Name: "eco-small4", CPU: 2, RAM: 2, Disk: 30},
		{ID: "std-2-2-0", Name: "std-small1", CPU: 2, RAM: 2, Disk: 75},
		{ID: "g4-2-2-0", Name: "g4-small2", CPU: 2, RAM: 2, Disk: 75},
		{ID: "g4-4-2-0", Name: "c4-medium1", CPU: 4, RAM: 2, Disk: 25},
	}
}

func (r *Router) arvUpgrade(chat, mid int64, i, idx int) {
	sess := r.arvSess(chat)
	region := strv(sess["region"])
	list := asMapList(sess["servers"])
	if idx < 0 || idx >= len(list) {
		r.Show(chat, mid, "⏳ نشست منقضی شد.", navKB("arvsrv"))
		return
	}
	s := list[idx]
	st := strings.ToLower(strv(s["status"]))
	if strings.Contains(st, "run") || strings.Contains(st, "active") && !strings.Contains(st, "off") {
		r.Show(chat, mid, "⬆️ ارتقای سرور\n\n⚠️ برای تغییر پلن، سرور باید خاموش باشد.\nاول خاموشش کن، بعد دوباره «ارتقا» را بزن.",
			[][]Btn{
				{btnS("🔌 خاموش کن", fmt.Sprintf("arvoff:%d:%d", i, idx), "danger")},
				{btn("⬆️ ادامهٔ ارتقا", fmt.Sprintf("arvup:%d:%d", i, idx))},
				navRow(fmt.Sprintf("arvs:%d:%d", i, idx)),
			})
		return
	}
	r.Show(chat, mid, "⏳ در حال بارگذاری پلن‌ها…", nil)
	sizes := r.arvSizes(i, region)
	if len(sizes) == 0 {
		r.Show(chat, mid, "❌ پلنی پیدا نشد.", navKB(fmt.Sprintf("arvs:%d:%d", i, idx)))
		return
	}
	var szl []any
	for _, sz := range sizes {
		szl = append(szl, map[string]any{"id": sz.ID, "name": sz.Name, "cpu": sz.CPU, "ram": sz.RAM, "disk": sz.Disk, "price": sz.Price})
	}
	sess["sizes"] = szl
	r.arvSessSave(chat, sess)
	curID := strv(s["flavor"])
	var kb [][]Btn
	for j, sz := range sizes {
		if j >= 30 {
			break
		}
		label := fmt.Sprintf("%s · %dvCPU/%dGB/%dGB", sz.Name, sz.CPU, sz.RAM, sz.Disk)
		if sz.ID == curID {
			label = "✅ " + label + " (فعلی)"
		}
		if p := arvToman(sz.Price); p != "" {
			label += " · " + p
		}
		kb = append(kb, []Btn{btn(truncRune(label, 58), fmt.Sprintf("arvupd:%d:%d:%d", i, idx, j))})
	}
	kb = append(kb, navRow(fmt.Sprintf("arvs:%d:%d", i, idx)))
	r.Show(chat, mid, fmt.Sprintf("⬆️ ارتقا/تغییر پلن «%s»\n📦 فعلی: <code>%s</code>\n\nپلن جدید را انتخاب کن:",
		Esc(strv(s["name"])), Esc(orDash(curID))), kb)
}

func (r *Router) arvUpgradeAsk(chat, mid int64, i, idx, sj int) {
	sess := r.arvSess(chat)
	sizes := asMapList(sess["sizes"])
	if sj < 0 || sj >= len(sizes) {
		r.Show(chat, mid, "❌ پلن پیدا نشد.", navKB(fmt.Sprintf("arvs:%d:%d", i, idx)))
		return
	}
	sz := sizes[sj]
	price := arvToman(sz["price"])
	if price == "" {
		price = "—"
	}
	r.Show(chat, mid, fmt.Sprintf("⬆️ تأیید ارتقا\n\n📦 پلن جدید: <code>%s</code> (%dvCPU / %dGB RAM / %dGB دیسک)\n💰 %s\n\n⚠️ سرور باید خاموش باشد.",
		Esc(strv(sz["name"])), intOrAny(sz["cpu"]), intOrAny(sz["ram"]), intOrAny(sz["disk"]), Esc(price)),
		[][]Btn{
			{btnS("✅ بله، ارتقا بده", fmt.Sprintf("arvupdo:%d:%d:%d", i, idx, sj), "success")},
			navRow(fmt.Sprintf("arvup:%d:%d", i, idx)),
		})
}

func (r *Router) arvUpgradeDo(chat, mid int64, i, idx, sj int) {
	sess := r.arvSess(chat)
	sizes := asMapList(sess["sizes"])
	list := asMapList(sess["servers"])
	region := strv(sess["region"])
	if sj < 0 || sj >= len(sizes) || idx < 0 || idx >= len(list) {
		r.Show(chat, mid, "⏳ نشست منقضی شد.", navKB("arvsrv"))
		return
	}
	cli, ok := r.arvCli(i)
	if !ok {
		r.Show(chat, mid, "❌ اکانت پیدا نشد.", navKB("arvsrv"))
		return
	}
	sizeID := strv(sizes[sj]["id"])
	sid := strv(list[idx]["id"])
	d, err := cli.Ecc(region, "POST", "/servers/"+sid+"/action", map[string]any{"resize": map[string]any{"flavorRef": sizeID}})
	if err != nil {
		d, err = cli.Ecc(region, "POST", "/servers/"+sid+"/action", map[string]any{"resize": map[string]any{"flavor_id": sizeID}})
	}
	if err != nil {
		r.Show(chat, mid, "❌ خطا: "+Esc(err.Error()), navKB(fmt.Sprintf("arvs:%d:%d", i, idx)))
		return
	}
	r.Bot.Send(chat, "✅ درخواست ارتقا ارسال شد.\n⏳ چند دقیقه طول می‌کشد؛ بعد از آماده‌شدن «تأیید ارتقا» را بزن.",
		[][]Btn{
			{btn("✅ تأیید ارتقا", fmt.Sprintf("arvupok:%d:%d", i, idx))},
			navRow(fmt.Sprintf("arvs:%d:%d", i, idx)),
		})
	_ = d
}

func (r *Router) arvUpgradeConfirm(chat, mid int64, i, idx int) {
	sess := r.arvSess(chat)
	list := asMapList(sess["servers"])
	region := strv(sess["region"])
	if idx < 0 || idx >= len(list) {
		r.Show(chat, mid, "⏳ نشست منقضی شد.", navKB("arvsrv"))
		return
	}
	cli, ok := r.arvCli(i)
	if !ok {
		r.Show(chat, mid, "❌ اکانت پیدا نشد.", navKB("arvsrv"))
		return
	}
	sid := strv(list[idx]["id"])
	_, err := cli.Ecc(region, "POST", "/servers/"+sid+"/action", map[string]any{"confirmResize": nil})
	if err != nil {
		r.Show(chat, mid, "❌ خطا: "+Esc(err.Error()), navKB(fmt.Sprintf("arvs:%d:%d", i, idx)))
		return
	}
	r.Show(chat, mid, "✅ ارتقا تأیید شد.", [][]Btn{{btn("🔄 تازه‌سازی", fmt.Sprintf("arvs:%d:%d", i, idx)), btn("🏠 خانه", "menu")}})
}
