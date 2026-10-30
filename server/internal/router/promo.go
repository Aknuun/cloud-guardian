package router

import (
	"fmt"
	"io"
	"net/http"
	"os"
	"strings"
	"sync"
	"time"
)

// Promo/ad line shown under the home menu. It can be changed without shipping a
// new release: the central promo.txt in the repo is fetched (and cached 12h), or
// overridden per-install with /promoset.
const (
	promoDefault    = "" // disabled: no built-in ad line
	promoURLDefault = "https://raw.githubusercontent.com/Aknuun/cloud-guardian/main/promo.txt"
	promoMaxRunes   = 200
)

type promoCache struct {
	T int64  `json:"t"`
	V string `json:"v"`
}

var (
	promoFetchMu  sync.Mutex
	promoFetching bool
)

func promoURL() string {
	if u := strings.TrimSpace(os.Getenv("CG_PROMO_URL")); u != "" {
		return u
	}
	return promoURLDefault
}

func fetchPromo(url string) string {
	cl := &http.Client{Timeout: 8 * time.Second}
	resp, err := cl.Get(url)
	if err != nil {
		return ""
	}
	defer resp.Body.Close()
	if resp.StatusCode != 200 {
		return ""
	}
	b, err := io.ReadAll(io.LimitReader(resp.Body, 4096))
	if err != nil {
		return ""
	}
	v := strings.TrimSpace(string(b))
	if v == "" || len([]rune(v)) > promoMaxRunes {
		return ""
	}
	return v
}

// refreshPromo fetches the central promo in the background (at most one at a time).
func (r *Router) refreshPromo() {
	promoFetchMu.Lock()
	if promoFetching {
		promoFetchMu.Unlock()
		return
	}
	promoFetching = true
	promoFetchMu.Unlock()
	go func() {
		defer func() {
			promoFetchMu.Lock()
			promoFetching = false
			promoFetchMu.Unlock()
		}()
		if v := fetchPromo(promoURL()); v != "" {
			r.Store.PutJSON("promo:cache", promoCache{T: time.Now().UnixMilli(), V: v}, 86400)
		}
	}()
}

// PromoText returns the current promo line. Order: local override → cached
// central promo.txt → env fallback → built-in default.
func (r *Router) PromoText() string {
	if v := strings.TrimSpace(r.Store.Get("promo:local")); v != "" {
		return v
	}
	if promoDefault == "" {
		return ""
	}
	var c promoCache
	have := r.Store.GetJSON("promo:cache", &c) && strings.TrimSpace(c.V) != ""
	if have && time.Now().UnixMilli()-c.T < 12*3600*1000 {
		return c.V
	}
	r.refreshPromo()
	if have {
		return c.V
	}
	if v := strings.TrimSpace(os.Getenv("CG_PROMO_TEXT")); v != "" {
		return v
	}
	return promoDefault
}

func (r *Router) promoHome(chat, mid int64) {
	cur := r.PromoText()
	src := "پیش‌فرض"
	if strings.TrimSpace(r.Store.Get("promo:local")) != "" {
		src = "محلی این نصب"
	}
	kb := [][]Btn{
		{btnS("✏️ تنظیم تبلیغ", "promoset", "primary"), btnS("♻️ بازنشانی", "promoreset", "danger")},
		navRow("settings"),
	}
	r.Show(chat, mid,
		fmt.Sprintf("📢 تبلیغ منوی اصلی\n\nمتن فعلی (%s):\n%s\n\n"+
			"این متن زیر پیام «خوش اومدی» نشان داده می‌شود. اگر نسخهٔ مرکزی مخزن به‌روز شود، بدون آپدیت هم عوض می‌شود.",
			src, Esc(cur)), kb)
}

func (r *Router) promoSetAsk(chat, mid int64) {
	r.setWizard(chat, map[string]any{"kind": "promo_text", "mid": mid})
	r.Show(chat, mid, "✍️ متن تبلیغ را بفرست (حداکثر ۲۰۰ کاراکتر):\n\nمی‌توانی از ایموجی و لینک استفاده کنی.",
		[][]Btn{{btn("❌ انصراف", "promo")}})
}

func (r *Router) promoSetSave(chat, mid int64, text string) {
	r.clearWizard(chat)
	v := strings.TrimSpace(text)
	if v == "" {
		r.Show(chat, mid, "⚠️ متن خالی است.", navKB("promo"))
		return
	}
	if len([]rune(v)) > promoMaxRunes {
		v = string([]rune(v)[:promoMaxRunes])
	}
	r.Store.Put("promo:local", v, 0)
	r.Show(chat, mid, "✅ تبلیغ محلی روی «خوش اومدی» تنظیم شد.", [][]Btn{navRow("promo")})
}

func (r *Router) promoReset(chat, mid int64) {
	r.Store.Delete("promo:local")
	r.Store.Delete("promo:cache")
	r.Show(chat, mid, "♻️ تبلیغ به حالت مرکزی/پیش‌فرض برگشت.", [][]Btn{navRow("promo")})
}
