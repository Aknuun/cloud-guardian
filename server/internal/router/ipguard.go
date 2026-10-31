package router

import (
	"net/http"
	"sync"
	"time"
)

// Simple per-IP fixed-window limiter for endpoints that have no session.

type ipWin struct {
	n   int
	win int64
}

var (
	ipMu   sync.Mutex
	ipHits = map[string]ipWin{}
)

// IPAllow reports whether ip may perform another request in this minute.
func IPAllow(ip string, perMin int) bool {
	if ip == "" {
		return true
	}
	now := time.Now().Unix() / 60
	ipMu.Lock()
	defer ipMu.Unlock()
	if len(ipHits) > 5000 {
		for k, v := range ipHits {
			if v.win < now-1 {
				delete(ipHits, k)
			}
		}
	}
	w := ipHits[ip]
	if w.win != now {
		w = ipWin{win: now}
	}
	if w.n >= perMin {
		ipHits[ip] = w
		return false
	}
	w.n++
	ipHits[ip] = w
	return true
}

func ipLimited(fn http.HandlerFunc, perMin int) http.HandlerFunc {
	return func(w http.ResponseWriter, req *http.Request) {
		if !IPAllow(clientIPOf(req), perMin) {
			webErr(w, http.StatusTooManyRequests, "درخواست‌های زیادی از این آی‌پی؛ کمی صبر کن")
			return
		}
		fn(w, req)
	}
}

// ClientIP exposes the proxy-aware remote address for callers outside the router package.
func ClientIP(req *http.Request) string { return clientIPOf(req) }
