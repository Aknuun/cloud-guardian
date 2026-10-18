// Package router dispatches Telegram updates to feature handlers.
package router

import (
	"fmt"
	"html"
	"strings"
	"time"

	"github.com/aknuun/cg-server/internal/jalali"
	"github.com/aknuun/cg-server/internal/store"
	"github.com/aknuun/cg-server/internal/tg"
)

const Version = "4.5.2"

// Btn is an inline keyboard button (alias of tg.Button).
type Btn = tg.Button

func btn(text, data string) Btn         { return Btn{Text: text, Data: data} }
func btnS(text, data, style string) Btn { return Btn{Text: text, Data: data, Style: style} }
func HomeKB() [][]Btn                   { return [][]Btn{{btn("🏠 خانه", "menu")}} }

func Esc(s any) string {
	return html.EscapeString(fmt.Sprint(s))
}

type Router struct {
	Bot      *tg.Client
	Store    *store.Store
	AdminID  int64
	FO       map[string]string
	CFToken  string
	BotToken string
	DBPath   string
	curFrom  int64
}

func New(bot *tg.Client, st *store.Store, adminID int64, fo map[string]string) *Router {
	if fo == nil {
		fo = map[string]string{}
	}
	var saved map[string]string
	if st.GetJSON(foCfgKey, &saved) && saved != nil {
		for k, v := range saved {
			fo[k] = v
		}
	}
	return &Router{Bot: bot, Store: st, AdminID: adminID, FO: fo}
}

// ---------- admins ----------

func (r *Router) Admins() []int64 {
	out := []int64{r.AdminID}
	var raw []int64
	if r.Store.GetJSON("admins", &raw) {
		for _, x := range raw {
			if x != r.AdminID && !containsI64(out, x) {
				out = append(out, x)
			}
		}
	}
	return out
}

func (r *Router) SetAdmins(lst []int64) {
	clean := []int64{}
	for _, x := range lst {
		if x != r.AdminID && !containsI64(clean, x) {
			clean = append(clean, x)
		}
	}
	r.Store.PutJSON("admins", clean, 0)
}

func (r *Router) IsMain(id int64) bool { return id == r.AdminID }

func (r *Router) isAdmin(id int64) bool { return containsI64(r.Admins(), id) }

func containsI64(a []int64, x int64) bool {
	for _, v := range a {
		if v == x {
			return true
		}
	}
	return false
}

// ---------- entry ----------

func (r *Router) Handle(upd tg.Update) {
	defer func() {
		if e := recover(); e != nil {
			fmt.Println("panic:", e)
		}
	}()
	if upd.CallbackQuery != nil {
		fmt.Printf("cb from=%d data=%q\n", idOf(upd.CallbackQuery.From), upd.CallbackQuery.Data)
		r.onCallback(upd.CallbackQuery)
	} else if upd.Message != nil {
		fmt.Printf("msg from=%d text=%q\n", idOf(upd.Message.From), upd.Message.Text)
		r.onMessage(upd.Message)
	}
}

func (r *Router) onMessage(m *tg.Message) {
	var frm int64
	if m.From != nil {
		frm = m.From.ID
	}
	r.curFrom = frm
	r.trackUser(m.From)
	if !r.isAdmin(frm) {
		r.nonAdminMessage(m)
		return
	}
	chat := m.Chat.ID
	text := strings.TrimSpace(m.Text)

	var pend map[string]any
	if r.Store.GetJSON(fmt.Sprintf("pend:%d", chat), &pend) && pend != nil &&
		!strings.HasPrefix(text, "/") {
		mid := int64(0)
		if v, ok := pend["mid"].(float64); ok {
			mid = int64(v)
		}
		r.wizardStep(chat, mid, pend, text)
		return
	}
	switch {
	case strings.HasPrefix(text, "/start"):
		r.Menu(chat, 0)
		if r.IsMain(frm) {
			r.foMaybePrompt(chat)
		}
	case text == "🏠 خانه":
		r.Menu(chat, 0)
	case strings.HasPrefix(text, "/zones"), strings.HasPrefix(text, "/cf"):
		r.cfZones(chat, 0, "all", 0)
	case strings.HasPrefix(text, "/newrecord"):
		r.cfAddrecZones(chat, 0)
	case strings.HasPrefix(text, "/favorites"):
		r.cfFavsPage(chat, 0)
	case strings.HasPrefix(text, "/search"):
		r.setWizard(chat, map[string]any{"kind": "cr_gsearch"})
		r.Show(chat, 0, cfSearchText, [][]Btn{{btn("❌ انصراف", "menu")}})
	case strings.HasPrefix(text, "/myid"):
		r.Show(chat, 0, fmt.Sprintf("🆔 شناسهٔ تلگرام شما: <code>%d</code>", frm), HomeKB())
	case strings.HasPrefix(text, "/admins"):
		if r.IsMain(frm) {
			r.adminsMenu(chat, 0)
		} else {
			r.Show(chat, 0, "👥 برای مدیریت ادمین‌ها با ادمین اصلی تماس بگیر.", HomeKB())
		}
	case strings.HasPrefix(text, "/hub"):
		if r.IsMain(frm) {
			r.hubHome(chat, 0)
		} else {
			r.contactKB(chat)
		}
	case strings.HasPrefix(text, "/contact"):
		r.contactKB(chat)
	default:
		switch {
		case strings.HasPrefix(text, "/"):
			if cb, ok := slashTargets[slashCmd(text)]; ok {
				r.dispatch(chat, 0, cb)
				return
			}
			r.Menu(chat, 0)
		case qaIPRe.MatchString(text):
			r.qaIPSearch(chat, 0, text)
		case cfIsHost(strings.ToLower(text)):
			r.cfGsearch(chat, 0, text)
		default:
			r.Menu(chat, 0)
		}
	}
}

func (r *Router) onCallback(cb *tg.CallbackQuery) {
	var frm int64
	if cb.From != nil {
		frm = cb.From.ID
	}
	r.curFrom = frm
	if !r.isAdmin(frm) {
		if cb.Data == "hubctmsg" && cb.Message != nil {
			r.Bot.Answer(cb.ID, "")
			r.setWizard(cb.Message.Chat.ID, map[string]any{"kind": "hub_contact"})
			r.Bot.Send(cb.Message.Chat.ID, "✉️ پیامت به سازنده را بفرست:", nil)
		}
		return
	}
	if cb.Message == nil {
		return
	}
	chat := cb.Message.Chat.ID
	mid := cb.Message.MessageID
	r.Bot.Answer(cb.ID, "")
	data := cb.Data
	r.dispatch(chat, mid, data)
}

// Show sends (mid==0) or edits a message.
func (r *Router) Show(chat, mid int64, text string, kb [][]Btn) {
	if mid == 0 || !r.Bot.Edit(chat, mid, text, kb) {
		r.Bot.Send(chat, text, kb)
	}
}

func nowMS() int64 { return time.Now().UnixMilli() }

func idOf(u *tg.User) int64 {
	if u == nil {
		return 0
	}
	return u.ID
}

// ---------- non-admin support ----------

func (r *Router) contactKB(chat int64) {
	who := fmt.Sprintf("<code>%d</code>", r.AdminID)
	r.Show(chat, 0,
		"📞 ارتباط با سازنده\n\nهر پیامی بفرستی به ادمین اصلی می‌رسد و می‌تونه جواب بده.\n"+
			"🆔 ادمین اصلی: "+who,
		[][]Btn{{btnS("✉️ فرستادن پیام", "hubctmsg", "primary")}})
}

func (r *Router) nonAdminMessage(m *tg.Message) {
	chat := m.Chat.ID
	text := strings.TrimSpace(m.Text)
	if text == "" {
		return
	}
	var pend map[string]any
	if r.Store.GetJSON(fmt.Sprintf("pend:%d", chat), &pend) && pend != nil &&
		!strings.HasPrefix(text, "/") {
		if kind, _ := pend["kind"].(string); kind == "hub_contact" {
			r.hubContactSave(chat, 0, text)
			return
		}
	}
	if strings.HasPrefix(text, "/") && !(strings.HasPrefix(text, "/start") || strings.HasPrefix(text, "/contact")) {
		r.contactKB(chat)
		return
	}
	if strings.HasPrefix(text, "/start") {
		r.contactKB(chat)
		return
	}
	r.forwardSupport(chat, text)
	r.Bot.Send(chat, "✅ پیامت به سازنده ارسال شد.", nil)
}

var _ = jalali.JNow
