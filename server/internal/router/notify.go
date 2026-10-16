package router

import (
	"fmt"

	"github.com/aknuun/cg-server/internal/jalali"
)

// FireDue sends any reminder whose time has arrived.
func (r *Router) FireDue() {
	list := r.rems()
	now := nowMS()
	changed := false
	for i := range list {
		rm := &list[i]
		if rm.Notified || rm.When == 0 || rm.When > now {
			continue
		}
		text := fmt.Sprintf("🔔 <b>یادآور</b>\n\n📝 %s\n🕓 %s", Esc(rm.Text), jalali.Fmt(rm.When))
		if rm.Note != "" {
			text += "\n📌 " + Esc(rm.Note)
		}
		targets := r.Admins()
		if rm.By != 0 && !containsI64(targets, rm.By) {
			targets = append(targets, rm.By)
		}
		for _, t := range targets {
			r.Bot.Send(t, text, [][]Btn{{btn("✅ باشه", "remack:"+rm.ID)}})
		}
		rm.Notified = true
		changed = true
	}
	if changed {
		r.saveRems(list)
	}
}
