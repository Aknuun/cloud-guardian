package router

import (
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/aknuun/cg-server/internal/ndmon"
	"github.com/aknuun/cg-server/internal/pg"
)

type webPgPanelIn struct {
	Index int    `json:"idx"`
	Name  string `json:"name"`
	URL   string `json:"url"`
	User  string `json:"user"`
	Pass  string `json:"pass"`
}

// webPgTest logs in with the given credentials so a bad panel never gets saved.
func webPgTest(p webPgPanelIn) (string, error) {
	u := strings.TrimSpace(p.URL)
	if u == "" || strings.TrimSpace(p.User) == "" || p.Pass == "" {
		return "", fmt.Errorf("آدرس، نام کاربری و رمز لازم است")
	}
	client := pg.New(u, strings.TrimSpace(p.User), p.Pass)
	if _, err := client.Login(); err != nil {
		return "", fmt.Errorf("ورود به پنل ناموفق: %s", err)
	}
	return u, nil
}

func (r *Router) webPgPanelAdd(w http.ResponseWriter, req *http.Request, uid int64) {
	var in webPgPanelIn
	if !webDecode(w, req, &in) {
		return
	}
	u, err := webPgTest(in)
	if err != nil {
		webErr(w, http.StatusBadRequest, err.Error())
		return
	}
	lst := pg.Panels(r.Store)
	name := strings.TrimSpace(in.Name)
	if name == "" {
		name = strings.TrimPrefix(strings.TrimPrefix(u, "https://"), "http://")
	}
	for _, p := range lst {
		if strings.EqualFold(strings.TrimSpace(p.URL), u) {
			webErr(w, http.StatusBadRequest, "این پنل قبلاً ثبت شده")
			return
		}
	}
	lst = append(lst, pg.Panel{URL: u, Username: strings.TrimSpace(in.User), Password: in.Pass, Name: name})
	pg.StorePanels(r.Store, lst)
	webOK(w, map[string]any{"ok": true})
}

func (r *Router) webPgPanelEdit(w http.ResponseWriter, req *http.Request, uid int64) {
	var in webPgPanelIn
	if !webDecode(w, req, &in) {
		return
	}
	lst := pg.Panels(r.Store)
	if in.Index < 0 || in.Index >= len(lst) {
		webErr(w, http.StatusBadRequest, "پنل پیدا نشد")
		return
	}
	cur := lst[in.Index]
	if strings.TrimSpace(in.URL) == "" {
		in.URL = cur.URL
	}
	if strings.TrimSpace(in.User) == "" {
		in.User = cur.Username
	}
	if in.Pass == "" {
		// keep the stored password, still verify the connection
		in.Pass = cur.Password
	}
	u, err := webPgTest(in)
	if err != nil {
		webErr(w, http.StatusBadRequest, err.Error())
		return
	}
	cur.URL = u
	cur.Username = strings.TrimSpace(in.User)
	cur.Password = in.Pass
	if n := strings.TrimSpace(in.Name); n != "" {
		cur.Name = n
	}
	lst[in.Index] = cur
	pg.StorePanels(r.Store, lst)
	webOK(w, map[string]any{"ok": true})
}

func (r *Router) webPgPanelDel(w http.ResponseWriter, req *http.Request, uid int64) {
	var in struct {
		Index int `json:"idx"`
	}
	if !webDecode(w, req, &in) {
		return
	}
	lst := pg.Panels(r.Store)
	if in.Index < 0 || in.Index >= len(lst) {
		webErr(w, http.StatusBadRequest, "پنل پیدا نشد")
		return
	}
	gone := lst[in.Index].URL
	lst = append(lst[:in.Index], lst[in.Index+1:]...)
	pg.StorePanels(r.Store, lst)

	mons := ndmon.Monitors(r.Store)
	kept := mons[:0]
	for _, m := range mons {
		if !strings.EqualFold(m.PanelURL, gone) {
			kept = append(kept, m)
		}
	}
	ndmon.SaveMonitors(r.Store, kept)
	webOK(w, map[string]any{"ok": true})
}

func (r *Router) webPgMonAdd(w http.ResponseWriter, req *http.Request, uid int64) {
	var in struct {
		PanelURL string  `json:"panel_url"`
		Name     string  `json:"name"`
		Interval float64 `json:"interval"`
	}
	if !webDecode(w, req, &in) {
		return
	}
	panel := strings.TrimSpace(in.PanelURL)
	if panel == "" {
		webErr(w, http.StatusBadRequest, "پنل را انتخاب کن")
		return
	}
	found := false
	for _, p := range pg.Panels(r.Store) {
		if strings.EqualFold(strings.TrimSpace(p.URL), panel) {
			found = true
			break
		}
	}
	if !found {
		webErr(w, http.StatusBadRequest, "پنل پیدا نشد")
		return
	}
	interval := int(in.Interval)
	if interval < 1 {
		interval = 5
	}
	on := true
	m := ndmon.Monitor{
		ID:          fmt.Sprintf("m%d", time.Now().UnixNano()),
		PanelURL:    panel,
		Name:        strings.TrimSpace(in.Name),
		IntervalMin: interval,
		Enabled:     &on,
	}
	lst := append(ndmon.Monitors(r.Store), m)
	ndmon.SaveMonitors(r.Store, lst)
	webOK(w, map[string]any{"id": m.ID})
}

func (r *Router) webPgMonToggle(w http.ResponseWriter, req *http.Request, uid int64) {
	var in struct {
		ID      string `json:"id"`
		Enabled bool   `json:"enabled"`
	}
	if !webDecode(w, req, &in) {
		return
	}
	lst := ndmon.Monitors(r.Store)
	for i := range lst {
		if lst[i].ID == in.ID {
			on := in.Enabled
			lst[i].Enabled = &on
			ndmon.SaveMonitors(r.Store, lst)
			webOK(w, map[string]any{"ok": true})
			return
		}
	}
	webErr(w, http.StatusNotFound, "مانیتور پیدا نشد")
}

func (r *Router) webPgMonDel(w http.ResponseWriter, req *http.Request, uid int64) {
	var in struct {
		ID string `json:"id"`
	}
	if !webDecode(w, req, &in) {
		return
	}
	lst := ndmon.Monitors(r.Store)
	kept := lst[:0]
	hit := false
	for _, m := range lst {
		if m.ID == in.ID {
			hit = true
			continue
		}
		kept = append(kept, m)
	}
	if !hit {
		webErr(w, http.StatusNotFound, "مانیتور پیدا نشد")
		return
	}
	ndmon.SaveMonitors(r.Store, kept)
	webOK(w, map[string]any{"ok": true})
}
