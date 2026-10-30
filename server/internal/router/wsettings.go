package router

import (
	"net/http"
	"strings"
)

func (r *Router) webToolsSettings(w http.ResponseWriter, req *http.Request, uid int64) {
	var gs []map[string]any
	for _, k := range guideOrder {
		if g, ok := guides[k]; ok {
			gs = append(gs, map[string]any{"id": k, "title": g[0], "text": g[1]})
		}
	}
	webOK(w, map[string]any{
		"version": Version,
		"latest":  r.Store.Get("selfup_latest"),
		"isMain":  r.IsMain(uid),
		"admins":  r.Admins(),
		"guides":  gs,
	})
}

func (r *Router) webSetCheck(w http.ResponseWriter, req *http.Request, uid int64) {
	tag, err := latestTag()
	if err != nil || tag == "" {
		webErr(w, http.StatusBadGateway, "بررسی نسخه ممکن نشد (گیت‌هاب در دسترس نیست)")
		return
	}
	r.Store.Put("selfup_latest", tag, 0)
	webOK(w, map[string]any{"latest": tag, "newer": verGreater(tag, r.curTag())})
}

func (r *Router) webSetUpdate(w http.ResponseWriter, req *http.Request, uid int64) {
	if !r.IsMain(uid) {
		webErr(w, http.StatusForbidden, "فقط ادمین اصلی")
		return
	}
	var in struct {
		Tag string `json:"tag"`
	}
	if !webDecode(w, req, &in) {
		return
	}
	tag := strings.TrimSpace(in.Tag)
	if !tagRe.MatchString(tag) {
		webErr(w, http.StatusBadRequest, "نسخهٔ نامعتبر")
		return
	}
	if err := startSelfUpdate(tag); err != nil {
		webErr(w, http.StatusBadGateway, "شروع آپدیت ممکن نشد: "+err.Error())
		return
	}
	webOK(w, map[string]any{"started": true, "tag": tag})
}

func (r *Router) webSetAdmin(w http.ResponseWriter, req *http.Request, uid int64) {
	if !r.IsMain(uid) {
		webErr(w, http.StatusForbidden, "فقط ادمین اصلی")
		return
	}
	var in struct {
		Add int64 `json:"add"`
		Del int64 `json:"del"`
	}
	if !webDecode(w, req, &in) {
		return
	}
	lst := r.Admins()
	if in.Add > 0 {
		if containsI64(lst, in.Add) {
			webErr(w, http.StatusBadRequest, "این شناسه از قبل ادمین است")
			return
		}
		lst = append(lst, in.Add)
	} else if in.Del > 0 {
		if in.Del == r.AdminID {
			webErr(w, http.StatusBadRequest, "ادمین اصلی حذف نمی‌شود")
			return
		}
		var next []int64
		for _, id := range lst {
			if id != in.Del {
				next = append(next, id)
			}
		}
		if len(next) == len(lst) {
			webErr(w, http.StatusNotFound, "ادمین پیدا نشد")
			return
		}
		lst = next
	} else {
		webErr(w, http.StatusBadRequest, "شناسه لازم است")
		return
	}
	r.SetAdmins(lst)
	webOK(w, map[string]any{"admins": r.Admins()})
}
