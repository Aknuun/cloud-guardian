package router

import (
	"fmt"
	"strconv"
	"strings"

	"github.com/aknuun/cg-server/internal/hz"
)

func (r *Router) hzOp(chat, mid int64, i int, sid, step string) {
	accs := r.hzAccs()
	if i < 0 || i >= len(accs) {
		r.Show(chat, mid, "❌ اکانت پیدا نشد.", HomeKB())
		return
	}
	cli := hz.New(strv(accs[i]["token"]))
	d, err := cli.Call("/servers/"+sid, "GET", nil)
	if err != nil {
		r.Show(chat, mid, "❌ "+Esc(err.Error()), HomeKB())
		return
	}
	s, _ := d["server"].(map[string]any)
	if s == nil {
		r.Show(chat, mid, "❌ سرور پیدا نشد.", HomeKB())
		return
	}
	back := fmt.Sprintf("hzsi:%d:%s", i, sid)
	home := []Btn{btn("🔙 بازگشت", "back:"+back), btn("🏠 خانه", "menu")}
	if step == "rename" {
		r.setWizard(chat, map[string]any{"kind": "hz_srv_rename", "acc": i, "sid": sid})
		r.Show(chat, mid, "✏️ نام جدید سرور را بفرستید:", [][]Btn{{btn("🔙 انصراف", back)}})
		return
	}
	if step == "rebuild" {
		imgs := r.hzImagesCached(cli, i)
		arch := ""
		if im, ok := s["image"].(map[string]any); ok {
			arch = strv(im["architecture"])
		}
		if arch == "" {
			if st, _ := s["server_type"].(map[string]any); st != nil {
				arch = strv(st["architecture"])
			}
		}
		var avail []map[string]any
		for _, im := range imgs {
			t := strv(im["type"])
			if (t == "system" || t == "snapshot") && (arch == "" || strv(im["architecture"]) == arch) {
				avail = append(avail, im)
			}
		}
		if len(avail) == 0 {
			r.Show(chat, mid, "❌ تصویری پیدا نشد.", HomeKB())
			return
		}
		var rows []Btn
		for j, im := range avail {
			if j >= 50 {
				break
			}
			label := strv(im["name"])
			if label == "" {
				label = strv(im["description"])
			}
			if label == "" {
				label = strv(im["id"])
			}
			rows = append(rows, btnS(label, fmt.Sprintf("hzsv:%d:%s:rebuild:%s", i, sid, strv(im["id"])), "plain"))
		}
		kb := gridBTN(rows, 2)
		kb = append(kb, home)
		r.Show(chat, mid, "🛠️ ریبیلد\n⚠️ همهٔ داده‌های سرور پاک می‌شود.\n\nتصویر جدید را انتخاب کنید:", kb)
		return
	}
	if step == "upgrade" {
		types := r.hzTypesCached(cli, i)
		cur, _ := s["server_type"].(map[string]any)
		locName := ""
		if lm, _ := s["location"].(map[string]any); lm != nil {
			locName = strv(lm["name"])
		}
		var up []map[string]any
		for _, t := range types {
			if strv(t["architecture"]) != strv(cur["architecture"]) || strv(t["id"]) == strv(cur["id"]) {
				continue
			}
			if !hzPlanInLoc(t, locName) {
				continue
			}
			bigger := intOfAny(t["memory"]) > intOfAny(cur["memory"]) ||
				intOfAny(t["cores"]) > intOfAny(cur["cores"]) ||
				intOfAny(t["disk"]) > intOfAny(cur["disk"])
			notSmaller := intOfAny(t["memory"]) >= intOfAny(cur["memory"]) &&
				intOfAny(t["cores"]) >= intOfAny(cur["cores"]) &&
				intOfAny(t["disk"]) >= intOfAny(cur["disk"])
			if bigger && notSmaller {
				up = append(up, t)
			}
		}
		if len(up) == 0 {
			r.Show(chat, mid, "❌ پلن ارتقای بالاتری موجود نیست.", HomeKB())
			return
		}
		sortByTypeSize(up)
		var rows []Btn
		for j, t := range up {
			if j >= 40 {
				break
			}
			rows = append(rows, btnS(fmt.Sprintf("%dC/%dG/%dGB", intOfAny(t["cores"]), intOfAny(t["memory"]), intOfAny(t["disk"])),
				fmt.Sprintf("hzsv:%d:%s:upgrade:%s", i, sid, strv(t["id"])), "plain"))
		}
		kb := gridBTN(rows, 2)
		kb = append(kb, home)
		warn := ""
		if strv(s["status"]) != "off" {
			warn = "⚠️ برای ارتقا اول سرور را خاموش کن.\n"
		}
		r.Show(chat, mid, "⬆️ ارتقا\n"+warn+"پلن جدید را انتخاب کنید:", kb)
		return
	}
	if step == "a4" || step == "a6" {
		typ := "ipv4"
		if step == "a6" {
			typ = "ipv6"
		}
		pn, _ := s["public_net"].(map[string]any)
		if pn != nil && pn[typ] != nil {
			r.Show(chat, mid, fmt.Sprintf("⚠️ این سرور از قبل %s اختصاصی دارد؛ اول آن را حذف کن.", typ), HomeKB())
			return
		}
		ips, _ := cli.Call("/primary_ips", "GET", nil)
		list := hzList(ips, "primary_ips")
		var avail []map[string]any
		for _, ip := range list {
			if ip["assignee_id"] == nil && strv(ip["type"]) == typ {
				avail = append(avail, ip)
			}
		}
		if len(avail) == 0 {
			r.Show(chat, mid, fmt.Sprintf("❌ آی‌پی %s خالی پیدا نشد.", typ), HomeKB())
			return
		}
		var rows []Btn
		for j, ip := range avail {
			if j >= 40 {
				break
			}
			label := strings.TrimSpace(strv(ip["name"]) + " " + strv(ip["ip"]))
			rows = append(rows, btnS(label, fmt.Sprintf("hzsv:%d:%s:%s:%s", i, sid, step, strv(ip["id"])), "plain"))
		}
		kb := gridBTN(rows, 2)
		kb = append(kb, home)
		r.Show(chat, mid, fmt.Sprintf("🔗 آی‌پی %s برای اختصاص (سرور خاموش می‌شود):", strings.ToUpper(typ)), kb)
		return
	}
	if step == "dsnap" {
		imgs := r.hzImagesCached(cli, i)
		sidInt, _ := strconv.Atoi(sid)
		var snaps []map[string]any
		for _, im := range imgs {
			if strv(im["type"]) != "snapshot" {
				continue
			}
			if cf, ok := im["created_from"].(map[string]any); ok && intOfAny(cf["id"]) == sidInt {
				snaps = append(snaps, im)
			}
		}
		if len(snaps) == 0 {
			r.Show(chat, mid, "❌ اسنپ‌شاتی برای این سرور پیدا نشد.", HomeKB())
			return
		}
		var rows []Btn
		for j, im := range snaps {
			if j >= 40 {
				break
			}
			label := strv(im["name"])
			if label == "" {
				label = strv(im["description"])
			}
			if label == "" {
				label = strv(im["id"])
			}
			rows = append(rows, btnS(label, fmt.Sprintf("hzsv:%d:%s:dsnap:%s", i, sid, strv(im["id"])), "plain"))
		}
		kb := gridBTN(rows, 2)
		kb = append(kb, home)
		r.Show(chat, mid, "🗑 اسنپ‌شات برای حذف:", kb)
		return
	}
	labels := map[string]string{"on": "روشن کردن", "off": "خاموش کردن", "reset": "ریست شدید",
		"reboot": "ریبوت", "pass": "ریست رمز", "del": "حذف سرور",
		"snap": "ساخت اسنپ‌شات", "u4": "حذف اختصاص IPv4", "u6": "حذف اختصاص IPv6",
		"r4": "جایگزینی IPv4 با یکی تازه (قدیمی حذف و سرور دوباره روشن می‌شود)"}
	label, ok := labels[step]
	if !ok {
		r.Show(chat, mid, "❓ عملیات ناشناخته.", HomeKB())
		return
	}
	r.Show(chat, mid, fmt.Sprintf("⚠️ مطمئنید «%s» روی «%s» انجام شود؟", label, strv(s["name"])),
		[][]Btn{{btn("✅ بله", fmt.Sprintf("hzok:%d:%s:%s", i, sid, step)), btn("❌ انصراف", back)}})
}
