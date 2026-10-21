package router

import (
	"fmt"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/aknuun/cg-server/internal/arvan"
	"github.com/aknuun/cg-server/internal/hostfilter"
)

type arvHuntItem struct {
	Name  string
	ID    string
	IP    string
	Phase string
	Err   string
	Clean bool
}

type arvHuntState struct {
	mu        sync.Mutex
	chat      int64
	acc       int
	region    string
	fw        string
	sizeID    string
	sizeName  string
	imageID   string
	imageName string
	msgID     int64
	stop      bool
	done      bool
	stage     string
	round     int
	maxRounds int
	batch     int
	started   time.Time
	items     []arvHuntItem
	created   int
	deleted   int
	kept      []arvHuntItem
	summary   string
}

var arvHunts sync.Map

func (h *arvHuntState) set(fn func(*arvHuntState)) {
	h.mu.Lock()
	fn(h)
	h.mu.Unlock()
}

func (r *Router) arvHuntRegions(chat, mid int64, i int) {
	kb := r.arvRegionButtons(i, "arvhunt")
	kb = append(kb, arvNav(i)...)
	r.Show(chat, mid, "🎯 شکار IP تمیز — آروان\n\n📍 ریجن را انتخاب کن:", kb)
}

func (r *Router) arvHuntRunning(chat int64) *arvHuntState {
	if v, ok := arvHunts.Load(chat); ok {
		if h, ok := v.(*arvHuntState); ok && !h.done {
			return h
		}
	}
	return nil
}

func arvHuntElapsed(t time.Time) string {
	s := int(time.Since(t).Seconds())
	return fmt.Sprintf("%02d:%02d", s/60, s%60)
}

func (r *Router) arvHuntRender(h *arvHuntState) {
	h.mu.Lock()
	if h.done || h.msgID == 0 {
		h.mu.Unlock()
		return
	}
	var b strings.Builder
	b.WriteString("🎯 <b>شکار IP تمیز — آروان</b>\n")
	b.WriteString(fmt.Sprintf("⏱ <b>%s</b> · دور %d/%d\n", arvHuntElapsed(h.started), h.round, h.maxRounds))
	b.WriteString(fmt.Sprintf("📍 %s · 📦 %s · 💿 %s\n", Esc(r.arvRegionName(h.region)), Esc(h.sizeName), Esc(h.imageName)))
	b.WriteString("🛡 فایروال: <code>" + Esc(h.fw) + "</code>\n")
	stageFa := map[string]string{
		"create": "در حال ساخت سرورها",
		"wait":   "در انتظار IP عمومی",
		"check":  "در حال پینگ از ایران و آلمان",
		"clean":  "آماده‌سازی سرورهای تمیز",
		"done":   "پایان",
	}[h.stage]
	if stageFa == "" {
		stageFa = h.stage
	}
	b.WriteString("🧭 مرحله: " + stageFa + "\n\n")
	for _, it := range h.items {
		line := "• " + Esc(it.Name) + ": " + Esc(it.Phase)
		if it.IP != "" {
			line += " · <code>" + Esc(it.IP) + "</code>"
		}
		if it.Err != "" {
			line += "\n   ↳ " + Esc(truncRune(it.Err, 80))
		}
		b.WriteString(line + "\n")
	}
	b.WriteString(fmt.Sprintf("\n📊 ساخته‌شده: %d · حذف: %d · تمیز: %d", h.created, h.deleted, len(h.kept)))
	msgID := h.msgID
	h.mu.Unlock()
	kb := [][]Btn{{btnS("⏹ توقف", "arvhuntstop", "danger")}}
	if h.done {
		kb = [][]Btn{{btn("🖥 سرورها", "arvsrv")}}
	}
	r.Bot.Edit(h.chat, msgID, b.String(), kb)
}

func (r *Router) arvHuntTicker(h *arvHuntState) {
	for {
		time.Sleep(3 * time.Second)
		h.mu.Lock()
		done := h.done
		h.mu.Unlock()
		if done {
			return
		}
		r.arvHuntRender(h)
	}
}

type arvHuntPrep struct {
	SizeID, SizeName, SizeLabel, ImageID, ImageName string
	Price                                           int
	Disk                                            int
}

func (r *Router) arvHuntSizes(i int, region string) []arvSize {
	sizes := r.arvSizes(i, region)
	sort.SliceStable(sizes, func(a, b int) bool {
		if sizes[a].Price > 0 && sizes[b].Price > 0 {
			return sizes[a].Price < sizes[b].Price
		}
		if sizes[a].CPU != sizes[b].CPU {
			return sizes[a].CPU < sizes[b].CPU
		}
		return sizes[a].RAM < sizes[b].RAM
	})
	return sizes
}

func (r *Router) arvHuntPrep(i int, region string, idx int) (arvHuntPrep, string) {
	var p arvHuntPrep
	if _, ok := r.arvCli(i); !ok {
		return p, "اکانت پیدا نشد"
	}
	sizes := r.arvHuntSizes(i, region)
	if len(sizes) == 0 {
		return p, "لیست پلن‌ها از آروان نیامد (خطای موقت API یا ریجن نامعتبر)"
	}
	if idx < 0 || idx >= len(sizes) {
		idx = 0
	}
	sz := sizes[idx]
	p.SizeID, p.SizeName, p.Price, p.Disk = sz.ID, sz.Name, sz.Price, sz.Disk
	if sz.CPU > 0 || sz.RAM > 0 {
		p.SizeLabel = fmt.Sprintf("%s (%dvCPU/%dGB)", sz.Name, sz.CPU, sz.RAM)
	} else {
		p.SizeLabel = sz.Name
	}
	var debID, debName, ubuID, ubuName string
	for _, m := range r.arvImageGroups(i, region) {
		grp := strings.ToLower(strv(m["name"]))
		for _, im := range asMapList(m["images"]) {
			nm := strings.ToLower(strv(im["name"]))
			id := strv(im["id"])
			if id == "" {
				continue
			}
			if debID == "" && grp == "debian" && (nm == "12" || nm == "13") {
				debID, debName = id, "Debian "+strv(im["name"])
			}
			if ubuID == "" && grp == "ubuntu" && (nm == "22.04" || nm == "24.04" || nm == "26.04") {
				ubuID, ubuName = id, "Ubuntu "+strv(im["name"])
			}
		}
		if debID != "" {
			break
		}
	}
	switch {
	case debID != "":
		p.ImageID, p.ImageName = debID, debName
	case ubuID != "":
		p.ImageID, p.ImageName = ubuID, ubuName
	}
	if p.ImageID == "" {
		return p, "ایمیج Debian/Ubuntu پیدا نشد"
	}
	return p, ""
}

// arvHuntPlans lets the user choose the plan (with price) before hunting.
func (r *Router) arvHuntPlans(chat, mid int64, i int, region string) {
	sizes := r.arvHuntSizes(i, region)
	if len(sizes) == 0 {
		r.Show(chat, mid, "❌ لیست پلن‌ها نیامد؛ چند لحظه بعد دوباره تلاش کن.", navKB("arvsrv"))
		return
	}
	var kb [][]Btn
	for j, sz := range sizes {
		if j >= 25 {
			break
		}
		label := fmt.Sprintf("📦 %s (%dvCPU/%dGB", sz.Name, sz.CPU, sz.RAM)
		if sz.Disk > 0 {
			label += fmt.Sprintf("/%dGB", sz.Disk)
		}
		label += ")"
		if p := arvToman(sz.Price); p != "" {
			label += " · 💰 " + p
		}
		kb = append(kb, []Btn{btn(truncRune(label, 60), fmt.Sprintf("arvhuntp:%d:%s:%d", i, region, j))})
	}
	kb = append(kb, arvNav(i)...)
	r.Show(chat, mid, fmt.Sprintf("🎯 شکار IP تمیز — %s\n\n📦 پلن مورد نظر را انتخاب کن (قیمت ماهانه):", Esc(r.arvRegionName(region))), kb)
}

func (r *Router) arvHuntPlanConfirm(chat, mid int64, i int, region string, idx int) {
	p, reason := r.arvHuntPrep(i, region, idx)
	if reason != "" {
		r.Show(chat, mid, "❌ "+Esc(reason)+"\n\nچند لحظه بعد دوباره تلاش کن.", navKB("arvsrv"))
		return
	}
	price := ""
	if t := arvToman(p.Price); t != "" {
		price = "\n💰 قیمت: <b>" + Esc(t) + "</b>"
	}
	r.Show(chat, mid, fmt.Sprintf("🎯 <b>شکار IP تمیز — آروان</b>\n\n"+
		"📍 ریجن: <code>%s</code>\n📦 پلن: <code>%s</code>%s\n💿 ایمیج: <code>%s</code>\n"+
		"🛡 فایروال: <code>Default</code>\n\n"+
		"🧪 معیار تمیزی: پینگ موفق از ایران و آلمان/هلند (check-host).\n"+
		"⚠️ هر دور ۳ سرور موقت ساخته می‌شود؛ کثیف‌ها خودکار حذف می‌شوند.\n"+
		"✅ سرورهای تمیز می‌مانند (هزینهٔ ساعتی دارند) و جست‌وجو متوقف می‌شود.\n\n"+
		"🔢 تعداد دورها را انتخاب کن:",
		Esc(r.arvRegionName(region)), Esc(p.SizeLabel), price, Esc(p.ImageName)),
		[][]Btn{
			{btnS("۳ دور", fmt.Sprintf("arvhuntgo:%d:%s:%d:3", i, region, idx), "primary"),
				btnS("۵ دور", fmt.Sprintf("arvhuntgo:%d:%s:%d:5", i, region, idx), "primary")},
			{btnS("۱۰ دور", fmt.Sprintf("arvhuntgo:%d:%s:%d:10", i, region, idx), "success")},
			{btn("🔙 پلن‌ها", fmt.Sprintf("arvhuntc:%d:%s", i, region)), btn("🖥 سرورها", "arvsrv")},
		})
}

func (r *Router) arvHuntStart(chat, mid int64, i int, region string, idx, rounds int) {
	if r.arvHuntRunning(chat) != nil {
		r.Show(chat, mid, "⏳ یک شکار IP در حال اجراست؛ اول آن را متوقف کن.", navKB("arvsrv"))
		return
	}
	p, reason := r.arvHuntPrep(i, region, idx)
	if reason != "" {
		r.Show(chat, mid, "❌ "+Esc(reason)+"\n\nچند لحظه بعد دوباره تلاش کن.", navKB("arvsrv"))
		return
	}
	if rounds <= 0 {
		rounds = 10
	}
	if rounds > 20 {
		rounds = 20
	}
	h := &arvHuntState{
		chat: chat, acc: i, region: region, fw: "Default",
		sizeID: p.SizeID, sizeName: p.SizeLabel,
		imageID: p.ImageID, imageName: p.ImageName,
		maxRounds: rounds, batch: 3, started: time.Now(), stage: "create",
	}
	arvHunts.Store(chat, h)
	msgID := r.Bot.Send(chat, "🎯 آماده‌سازی شکار IP…", [][]Btn{{btnS("⏹ توقف", "arvhuntstop", "danger")}})
	h.msgID = msgID
	go r.arvHuntTicker(h)
	go r.arvHuntRun(h)
}

func (r *Router) arvHuntStop(chat int64) {
	v, ok := arvHunts.Load(chat)
	if !ok {
		return
	}
	h, ok := v.(*arvHuntState)
	if !ok {
		return
	}
	h.set(func(h *arvHuntState) { h.stop = true })
	r.Bot.Send(chat, "⏹ در حال توقف… سرورهای موقت حذف می‌شوند.", nil)
}

func (r *Router) arvHuntFinish(h *arvHuntState, summary string) {
	h.set(func(h *arvHuntState) {
		h.done = true
		h.stage = "done"
		h.summary = summary
	})
	r.arvHuntRender(h)
	var b strings.Builder
	b.WriteString(summary + "\n\n")
	if len(h.kept) > 0 {
		b.WriteString("🟢 سرورهای تمیز نگه‌داشته‌شده:\n")
		for _, it := range h.kept {
			b.WriteString("• <code>" + Esc(it.Name) + "</code> — <code>" + Esc(it.IP) + "</code>\n")
		}
	}
	kb := [][]Btn{{btn("🖥 سرورها", "arvsrv")}, {btn("🏠 خانه", "menu")}}
	r.Bot.Send(h.chat, b.String(), kb)
}

func (r *Router) arvHuntRun(h *arvHuntState) {
	defer func() {
		if rec := recover(); rec != nil {
			r.arvHuntFinish(h, "❌ خطای داخلی: "+fmt.Sprint(rec))
		}
	}()
	cli, ok := r.arvCli(h.acc)
	if !ok {
		r.arvHuntFinish(h, "❌ اکانت پیدا نشد.")
		return
	}
	for round := 1; round <= h.maxRounds; round++ {
		h.set(func(h *arvHuntState) {
			if h.stop {
				return
			}
			h.round = round
			h.stage = "create"
			h.items = nil
			for k := 0; k < h.batch; k++ {
				h.items = append(h.items, arvHuntItem{
					Name:  fmt.Sprintf("clean-%d-%d-%d", time.Now().Unix(), round, k+1),
					Phase: "در حال ساخت",
				})
			}
		})
		h.mu.Lock()
		if h.stop {
			h.mu.Unlock()
			break
		}
		items := append([]arvHuntItem{}, h.items...)
		h.mu.Unlock()
		r.arvHuntRender(h)

		var wg sync.WaitGroup
		var mu sync.Mutex
		fwErr := ""
		lastErr := ""
		roundCreated := 0
		for k := range items {
			wg.Add(1)
			go func(k int) {
				defer wg.Done()
				name := items[k].Name
				d, err := r.arvCreateServer(h.acc, h.region, name, h.sizeID, h.imageID, h.fw, 0)
				if err != nil {
					low := strings.ToLower(err.Error())
					mu.Lock()
					lastErr = err.Error()
					mu.Unlock()
					if strings.Contains(low, "firewall") {
						mu.Lock()
						fwErr = "آروان بدون گروه فایروال سرور نمی‌سازد. در پنل آروان یک گروه فایروال بساز و از صفحهٔ «ساخت سرور» نامش را وارد کن."
						mu.Unlock()
					}
					h.set(func(h *arvHuntState) {
						if k < len(h.items) {
							h.items[k].Phase = "❌ خطای ساخت"
							h.items[k].Err = err.Error()
						}
					})
					return
				}
				sid := ""
				if one := arvan.EccOne(d); one != nil {
					sid = strv(one["id"])
				}
				h.set(func(h *arvHuntState) {
					if k < len(h.items) {
						h.items[k].ID = sid
						h.items[k].Phase = "⏳ در انتظار IP"
					}
					h.created++
				})
				mu.Lock()
				roundCreated++
				mu.Unlock()
			}(k)
		}
		wg.Wait()
		if fwErr != "" {
			r.arvHuntFinish(h, "❌ "+fwErr)
			return
		}
		if roundCreated == 0 {
			if msg := arvCreditErr(lastErr); msg != "" {
				r.arvHuntFinish(h, msg)
				return
			}
			r.arvHuntFinish(h, "❌ هیچ سروری ساخته نشد (دور "+fmt.Sprint(round)+"): "+truncRune(lastErr, 300))
			return
		}

		h.set(func(h *arvHuntState) { h.stage = "wait" })
		for k := range items {
			wg.Add(1)
			go func(k int) {
				defer wg.Done()
				h.mu.Lock()
				sid := ""
				if k < len(h.items) {
					sid = h.items[k].ID
				}
				h.mu.Unlock()
				if sid == "" {
					return
				}
				for attempt := 0; attempt < 40; attempt++ {
					h.mu.Lock()
					stopped := h.stop
					h.mu.Unlock()
					if stopped {
						return
					}
					d, err := cli.Ecc(h.region, "GET", "/servers/"+sid, nil)
					if err == nil {
						if one := arvan.EccOne(d); one != nil {
							n := arvNormalize(one)
							ip := firstPubIP(n)
							st := strings.ToLower(strv(n["status"]))
							if ip != "" {
								h.set(func(h *arvHuntState) {
									if k < len(h.items) {
										h.items[k].IP = ip
										h.items[k].Phase = "🧪 در حال بررسی"
									}
								})
								return
							}
							h.set(func(h *arvHuntState) {
								if k < len(h.items) {
									h.items[k].Phase = "⏳ " + orDash(st)
								}
							})
						}
					}
					time.Sleep(6 * time.Second)
				}
			}(k)
		}
		wg.Wait()

		h.set(func(h *arvHuntState) { h.stage = "check" })
		r.arvHuntRender(h)
		results := make([]bool, len(items))
		verdicts := make([]string, len(items))
		for k := range items {
			h.mu.Lock()
			ip := ""
			if k < len(h.items) {
				ip = h.items[k].IP
			}
			h.mu.Unlock()
			if ip == "" {
				verdicts[k] = "بدون IP"
				continue
			}
			wg.Add(1)
			go func(k int, ip string) {
				defer wg.Done()
				clean, verdict := hostfilter.CheckIPCleanliness(ip)
				results[k] = clean
				verdicts[k] = verdict
				h.set(func(h *arvHuntState) {
					if k < len(h.items) {
						h.items[k].Clean = clean
						if clean {
							h.items[k].Phase = "🟢 تمیز"
						} else {
							h.items[k].Phase = "🔴 کثیف"
						}
					}
				})
			}(k, ip)
		}
		wg.Wait()
		r.arvHuntRender(h)

		h.mu.Lock()
		stopped := h.stop
		cur := append([]arvHuntItem{}, h.items...)
		h.mu.Unlock()
		var cleanOnes []arvHuntItem
		for k := range cur {
			if results[k] {
				cleanOnes = append(cleanOnes, cur[k])
			}
		}
		if stopped {
			for k := range cur {
				if cur[k].ID != "" {
					cli.Ecc(h.region, "DELETE", "/servers/"+cur[k].ID, nil)
					h.set(func(h *arvHuntState) { h.deleted++ })
				}
			}
			r.arvHuntFinish(h, "⏹ متوقف شد.")
			return
		}
		if len(cleanOnes) > 0 {
			h.set(func(h *arvHuntState) { h.stage = "clean"; h.kept = cleanOnes })
			for k := range cur {
				if results[k] || cur[k].ID == "" {
					continue
				}
				cli.Ecc(h.region, "DELETE", "/servers/"+cur[k].ID, nil)
				h.set(func(h *arvHuntState) {
					h.deleted++
					if k < len(h.items) {
						h.items[k].Phase += " → حذف"
					}
				})
			}
			r.arvHuntRender(h)
			r.arvHuntFinish(h, fmt.Sprintf("✅ %d آی‌پی تمیز پیدا شد (دور %d).", len(cleanOnes), round))
			return
		}
		for k := range cur {
			if cur[k].ID == "" {
				continue
			}
			cli.Ecc(h.region, "DELETE", "/servers/"+cur[k].ID, nil)
			h.set(func(h *arvHuntState) {
				h.deleted++
				if k < len(h.items) {
					h.items[k].Phase += " → حذف"
				}
			})
		}
		r.arvHuntRender(h)
	}
	r.arvHuntFinish(h, "❌ در سقف تلاش‌ها آی‌پی تمیزی پیدا نشد.")
}
