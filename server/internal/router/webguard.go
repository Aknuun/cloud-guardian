package router

import (
	"sync"
	"time"
)

// Cheap per-admin request limiter: a stuck UI or a retry loop should not hammer
// PasarGuard/Cloudflare with hundreds of writes. 300 requests per minute is well
// above normal use (each page load is a handful of GETs).
const webRateLimit = 300

type webRateWin struct {
	start time.Time
	n     int
}

var (
	webRateMu sync.Mutex
	webRate   = map[int64]*webRateWin{}
)

func webRateAllow(uid int64) bool {
	now := time.Now()
	webRateMu.Lock()
	defer webRateMu.Unlock()
	w := webRate[uid]
	if w == nil || now.Sub(w.start) > time.Minute {
		webRate[uid] = &webRateWin{start: now, n: 1}
		return true
	}
	if w.n >= webRateLimit {
		return false
	}
	w.n++
	return true
}
