package router

import (
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"
)

const repo = "Aknuun/cloud-guardian"

func latestVersion() (string, error) {
	hc := &http.Client{Timeout: 12 * time.Second}
	urls := []string{
		fmt.Sprintf("https://raw.githubusercontent.com/%s/main/server/VERSION", repo),
		fmt.Sprintf("https://raw.githubusercontent.com/%s/master/server/VERSION", repo),
	}
	for _, u := range urls {
		resp, err := hc.Get(u)
		if err != nil {
			continue
		}
		b, _ := io.ReadAll(resp.Body)
		resp.Body.Close()
		if resp.StatusCode == 200 {
			if v := strings.TrimSpace(string(b)); v != "" {
				return v, nil
			}
		}
	}
	return "", fmt.Errorf("unreachable")
}

func (r *Router) versHome(chat, mid int64) {
	kb := [][]Btn{
		{btn("🔍 بررسی آخرین نسخه", "verlatest")},
		{btn("🔙 بازگشت", "settings"), btn("🏠 خانه", "menu")},
	}
	r.Show(chat, mid,
		fmt.Sprintf("🔄 <b>نسخه‌ها</b>\n\n🤖 نسخهٔ فعلی سرور: <code>%s</code>", Version), kb)
}

func (r *Router) verDo(chat, mid int64) {
	v, err := latestVersion()
	if err != nil {
		r.Show(chat, mid, "❌ اتصال به مخزن ممکن نشد.",
			[][]Btn{{btn("🔙 بازگشت", "vers"), btn("🏠 خانه", "menu")}})
		return
	}
	if v == Version {
		r.Show(chat, mid, "✅ روی آخرین نسخه هستی.",
			[][]Btn{{btn("🔙 بازگشت", "vers"), btn("🏠 خانه", "menu")}})
		return
	}
	r.Show(chat, mid, fmt.Sprintf("🆕 آخرین نسخه: <code>%s</code>\n(فعلی: <code>%s</code>)", v, Version),
		[][]Btn{{btn("🔙 بازگشت", "vers"), btn("🏠 خانه", "menu")}})
}
