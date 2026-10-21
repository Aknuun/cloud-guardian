package router

import (
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"strings"
	"time"

	"github.com/aknuun/cg-server/internal/hostfilter"
	"github.com/aknuun/cg-server/internal/jalali"
)

// hfBackup is a point-in-time snapshot of every panel host definition.
type hfBackup struct {
	ID    string           `json:"id"`
	TS    string           `json:"ts"`
	Label string           `json:"label"`
	Count int              `json:"count"`
	Hosts []map[string]any `json:"hosts"`
}

func newBkID() string {
	var b [8]byte
	rand.Read(b[:])
	return hex.EncodeToString(b[:])
}

func (r *Router) hfBackupsLoad() []hfBackup {
	var b []hfBackup
	r.Store.GetJSON("host_filter_backups", &b)
	return b
}

func (r *Router) hfBackupsStore(list []hfBackup) {
	r.Store.PutJSON("host_filter_backups", list, 0)
}

func hfBkTime(ts string) string {
	if t, err := time.Parse(time.RFC3339, ts); err == nil {
		return jalali.Fmt(t.UnixMilli())
	}
	if len(ts) > 16 {
		return strings.Replace(ts[:16], "T", " ", 1)
	}
	return ts
}

func (r *Router) hfBackupHome(chat, mid int64) {
	backups := r.hfBackupsLoad()
	lines := []string{"💾 بکاپ هاست‌ها", ""}
	if len(backups) == 0 {
		lines = append(lines, "📭 هنوز بکاپی گرفته نشده.")
	} else {
		for _, b := range backups {
			lines = append(lines, fmt.Sprintf("• %s — %s (%d هاست)", hfBkTime(b.TS), b.Label, b.Count))
		}
	}
	kb := [][]Btn{{btn("➕ گرفتن بکاپ جدید", "hfbknow")}}
	for _, b := range backups {
		kb = append(kb, []Btn{
			btn("↩️ "+hfBkTime(b.TS), "hfbkr:"+b.ID),
			btnS("🗑", "hfbkdel:"+b.ID, "danger")})
	}
	kb = append(kb, []Btn{btn("🔙 بازگشت", "back:hf"), btn("🏠 خانه", "menu")})
	r.Show(chat, mid, strings.Join(lines, "\n"), kb)
}

func (r *Router) hfBackupNow(chat, mid int64) {
	r.Show(chat, mid, "⏳ در حال گرفتن بکاپ از هاست‌ها…", nil)
	var hosts []map[string]any
	for pi, p := range r.panels() {
		c := p.Client()
		tok, err := c.Login()
		if err != nil {
			continue
		}
		lst, err := c.Hosts(tok)
		if err != nil {
			continue
		}
		for _, hv := range lst {
			h, ok := hv.(map[string]any)
			if !ok {
				continue
			}
			hosts = append(hosts, map[string]any{
				"panel":   pi,
				"id":      strv(h["id"]),
				"remark":  strv(h["remark"]),
				"address": toList(h["address"]),
				"sni":     toList(h["sni"]),
				"host":    toList(h["host"]),
			})
		}
	}
	keep := hostfilter.GetCfg(r.Store).BackupKeep
	if keep < 1 {
		keep = 5
	}
	entry := hfBackup{ID: newBkID(), TS: time.Now().UTC().Format(time.RFC3339),
		Label: "دستی", Count: len(hosts), Hosts: hosts}
	backups := append([]hfBackup{entry}, r.hfBackupsLoad()...)
	if len(backups) > keep {
		backups = backups[:keep]
	}
	r.hfBackupsStore(backups)
	r.Show(chat, mid, fmt.Sprintf("✅ بکاپ «%s» با %d هاست ساخته شد.", hfBkTime(entry.TS), entry.Count),
		navKB("hfbk"))
}

func (r *Router) hfBackupRestoreAsk(chat, mid int64, id string) {
	r.Show(chat, mid,
		"⚠️ بازگردانی این بکاپ، مقادیر address/sni/host همهٔ هاست‌ها را به آن زمان برمی‌گرداند. مطمئنی؟",
		[][]Btn{
			{btnS("✅ بله، بازگردان", "hfbkrd:"+id, "success")},
			{btn("❌ انصراف", "hfbk")}})
}

func (r *Router) hfBackupRestoreDo(chat, mid int64, id string) {
	r.Show(chat, mid, "⏳ در حال بازگردانی بکاپ…", nil)
	var entry *hfBackup
	for _, b := range r.hfBackupsLoad() {
		if b.ID == id {
			bb := b
			entry = &bb
			break
		}
	}
	if entry == nil {
		r.Show(chat, mid, "❌ بکاپ پیدا نشد.", navKB("hfbk"))
		return
	}
	panels := r.panels()
	byPanel := map[int][]map[string]any{}
	for _, snap := range entry.Hosts {
		pi := intOrAny(snap["panel"])
		byPanel[pi] = append(byPanel[pi], snap)
	}
	ok, fail := 0, 0
	for pi, snaps := range byPanel {
		if pi < 0 || pi >= len(panels) {
			fail += len(snaps)
			continue
		}
		c := panels[pi].Client()
		tok, err := c.Login()
		if err != nil {
			fail += len(snaps)
			continue
		}
		lst, err := c.Hosts(tok)
		if err != nil {
			fail += len(snaps)
			continue
		}
		for _, snap := range snaps {
			var host map[string]any
			for _, hv := range lst {
				h, _ := hv.(map[string]any)
				if strv(h["id"]) == strv(snap["id"]) {
					host = h
					break
				}
			}
			if host == nil {
				fail++
				continue
			}
			for _, f := range []string{"address", "sni", "host"} {
				if arr, ok := snap[f].([]any); ok {
					host[f] = arr
				}
			}
			if err := c.PutHost(tok, strv(host["id"]), host); err != nil {
				fail++
			} else {
				ok++
			}
		}
	}
	r.Show(chat, mid, fmt.Sprintf("✅ بازگردانی انجام شد.\nموفق: %d — ناموفق: %d", ok, fail),
		navKB("hfbk"))
}

func (r *Router) hfBackupDel(chat, mid int64, id string) {
	list := r.hfBackupsLoad()
	out := make([]hfBackup, 0, len(list))
	for _, b := range list {
		if b.ID != id {
			out = append(out, b)
		}
	}
	r.hfBackupsStore(out)
	r.hfBackupHome(chat, mid)
}
