package router

import (
	"fmt"
	"strconv"
	"strings"
)

func (r *Router) adminsMenu(chat, mid int64) {
	if !r.IsMain(r.curFrom) {
		r.Show(chat, mid, "👥 فقط ادمین اصلی.", HomeKB())
		return
	}
	var b strings.Builder
	b.WriteString("👥 <b>مدیریت ادمین‌ها</b>\n\n")
	b.WriteString(fmt.Sprintf("👑 ادمین اصلی: <code>%d</code>\n", r.AdminID))
	extra := r.Admins()
	if len(extra) <= 1 {
		b.WriteString("— ادمین دیگری ثبت نشده.\n")
	} else {
		for i, id := range extra[1:] {
			b.WriteString(fmt.Sprintf("%d) <code>%d</code>\n", i+1, id))
		}
	}
	b.WriteString("\n🔗 شناسه: با <code>/myid</code>.")
	kb := [][]Btn{
		{btn("➕ افزودن ادمین", "adminadd"), btn("➖ حذف ادمین", "admindel")},
		{btn("🔙 بازگشت", "settings"), btn("🏠 خانه", "menu")},
	}
	r.Show(chat, mid, b.String(), kb)
}

func (r *Router) adminAdd(chat, mid int64) {
	r.setWizard(chat, map[string]any{"kind": "admin_add"})
	r.Show(chat, mid, "➕ شناسهٔ عددی ادمین جدید را بفرست:", [][]Btn{{btn("🔙 لغو", "admins_menu")}})
}

func (r *Router) adminDelList(chat, mid int64) {
	extra := r.Admins()
	var kb [][]Btn
	for _, id := range extra[1:] {
		kb = append(kb, []Btn{btn(fmt.Sprintf("➖ %d", id), "admindely:"+strconv.FormatInt(id, 10))})
	}
	kb = append(kb, []Btn{btn("🔙 بازگشت", "admins_menu"), btn("🏠 خانه", "menu")})
	if len(kb) == 1 {
		r.Show(chat, mid, "— ادمینی برای حذف نیست.", [][]Btn{{btn("🔙 بازگشت", "admins_menu")}})
		return
	}
	r.Show(chat, mid, "➖ کدام ادمین حذف شود؟", kb)
}

func (r *Router) adminDelDo(chat, mid int64, s string) {
	id, err := strconv.ParseInt(strings.TrimSpace(s), 10, 64)
	if err != nil {
		r.Show(chat, mid, "❌", HomeKB())
		return
	}
	cur := r.Admins()
	var next []int64
	for _, x := range cur {
		if x != id {
			next = append(next, x)
		}
	}
	r.SetAdmins(next)
	r.Show(chat, mid, fmt.Sprintf("✅ ادمین <code>%d</code> حذف شد.", id), HomeKB())
}
