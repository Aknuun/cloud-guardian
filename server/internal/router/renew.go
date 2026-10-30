package router

import (
	"fmt"
	"net/url"
	"strings"
	"time"

	"github.com/aknuun/cg-server/internal/jalali"
	"github.com/aknuun/cg-server/internal/pg"
)

// renewFromBot answers the «تمدید از ربات» button on user alerts:
// https://t.me/<bot>?start=renew_<username>
func (r *Router) renewFromBot(chat, mid int64, username string) {
	username = strings.TrimSpace(username)
	if username == "" {
		r.Menu(chat, 0)
		return
	}
	for _, p := range r.panels() {
		c := p.Client()
		tok, err := c.Login()
		if err != nil {
			continue
		}
		u := c.User(tok, username)
		if u == nil {
			continue
		}
		lines := []string{
			"🔄 <b>تمدید سرویس</b>",
			"",
			"سرویس: <code>" + Esc(username) + "</code>",
			"پنل: " + Esc(p.Name),
		}
		if exp := strv(u["expire"]); exp != "" {
			line := "انقضا: " + ExpiryFA(exp)
			if t, err := time.Parse(time.RFC3339, exp); err == nil {
				d := time.Until(t)
				if d > 0 {
					if d < 24*time.Hour {
						line += fmt.Sprintf(" · حدود %d ساعت", int(d.Hours()+0.5))
					} else {
						line += fmt.Sprintf(" · %d روز", int(d.Hours()/24))
					}
				} else {
					line += " · منقضی شده"
				}
			}
			lines = append(lines, line)
		}
		used := numGB(u["used_traffic"])
		limit := numGB(u["data_limit"])
		if limit > 0 {
			lines = append(lines, fmt.Sprintf("حجم: %.1f از %.1f GB", used, limit))
		} else {
			lines = append(lines, fmt.Sprintf("حجم مصرف‌شده: %.1f GB", used))
		}
		if st := strv(u["status"]); st != "" {
			lines = append(lines, "وضعیت: "+statusFA(st))
		}
		lines = append(lines, "", "برای تمدید از دکمهٔ زیر استفاده کن.")
		kb := [][]Btn{
			{btnURL("🖥 تمدید از پنل", panelUserURL(p, username))},
			{btn("🔙 بازگشت", "menu")},
		}
		r.Show(chat, mid, strings.Join(lines, "\n"), kb)
		return
	}
	r.Show(chat, mid, "❌ سرویسی با نام <code>"+Esc(username)+"</code> در پنل‌ها پیدا نشد.",
		[][]Btn{{btn("🔙 بازگشت", "menu")}})
}

func panelUserURL(p pg.Panel, username string) string {
	return strings.TrimRight(p.URL, "/") + "/dashboard/#/users?search=" + url.QueryEscape(username)
}

// ExpiryFA renders an RFC3339 panel expiry as a Jalali date.
func ExpiryFA(s string) string {
	if t, err := time.Parse(time.RFC3339, s); err == nil {
		return jalali.Fmt(t.UnixMilli())
	}
	return s
}

func numGB(v any) float64 {
	switch t := v.(type) {
	case float64:
		return t / (1024 * 1024 * 1024)
	case int64:
		return float64(t) / (1024 * 1024 * 1024)
	case int:
		return float64(t) / (1024 * 1024 * 1024)
	}
	return 0
}

func statusFA(s string) string {
	switch strings.ToLower(s) {
	case "active":
		return "فعال"
	case "expired":
		return "منقضی"
	case "limited":
		return "محدود شده"
	case "disabled":
		return "غیرفعال"
	case "on_hold":
		return "در انتظار"
	}
	return s
}

var _ = pg.Panel{}
