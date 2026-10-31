package router

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"sync"
	"time"
)

// Idempotent POSTs: the client sends Idempotency-Key and gets the first result
// replayed for 5 minutes, so a retried delete/create never runs twice.

const (
	webIdemTTL = 5 * time.Minute
	webIdemMax = 300
)

type webIdemEntry struct {
	TS   int64           `json:"ts"`
	Code int             `json:"code"`
	Body json.RawMessage `json:"body"`
	Busy bool            `json:"busy"`
	Hash string          `json:"hash"`
}

var webIdemMu sync.Mutex

func (r *Router) webIdemLoad() map[string]webIdemEntry {
	m := map[string]webIdemEntry{}
	r.Store.GetJSON("web_idem", &m)
	return m
}

func (r *Router) webIdemStore(m map[string]webIdemEntry) {
	cut := time.Now().Add(-webIdemTTL).Unix()
	out := make(map[string]webIdemEntry, len(m))
	for k, v := range m {
		if v.TS >= cut {
			out[k] = v
		}
	}
	if len(out) > webIdemMax {
		oldest := ""
		var ts int64
		for k, v := range out {
			if oldest == "" || v.TS < ts {
				oldest, ts = k, v.TS
			}
		}
		delete(out, oldest)
	}
	r.Store.PutJSON("web_idem", out, 0)
}

// webIdemReserve returns an existing entry when the key was already used.
func (r *Router) webIdemReserve(key, hash string) (webIdemEntry, bool) {
	webIdemMu.Lock()
	defer webIdemMu.Unlock()
	m := r.webIdemLoad()
	if e, ok := m[key]; ok && time.Since(time.Unix(e.TS, 0)) < webIdemTTL {
		return e, true
	}
	m[key] = webIdemEntry{TS: time.Now().Unix(), Busy: true, Hash: hash}
	r.webIdemStore(m)
	return webIdemEntry{}, false
}

func (r *Router) webIdemFinish(key, hash string, code int, body []byte) {
	webIdemMu.Lock()
	defer webIdemMu.Unlock()
	m := r.webIdemLoad()
	m[key] = webIdemEntry{TS: time.Now().Unix(), Code: code, Hash: hash, Body: append(json.RawMessage(nil), body...)}
	r.webIdemStore(m)
}

type webRec struct {
	http.ResponseWriter
	code int
	buf  bytes.Buffer
}

func (w *webRec) WriteHeader(c int) {
	if w.code == 0 {
		w.code = c
	}
}
func (w *webRec) Write(b []byte) (int, error) { return w.buf.Write(b) }

// webIdemRun executes fn once per key and replays the stored response.
func (r *Router) webIdemRun(w http.ResponseWriter, req *http.Request, uid int64, fn webHandler) {
	key := strings.TrimSpace(req.Header.Get("Idempotency-Key"))
	if key == "" || req.Method != http.MethodPost {
		fn(w, req, uid)
		return
	}
	// The key is scoped to the caller, the path and the body: reusing a key for a
	// different request must not replay someone else's response.
	var raw []byte
	if req.Body != nil {
		raw, _ = io.ReadAll(io.LimitReader(req.Body, 1<<20))
		req.Body = io.NopCloser(bytes.NewReader(raw))
	}
	sum := sha256.Sum256(append([]byte(req.URL.Path+"\n"), raw...))
	hash := hex.EncodeToString(sum[:])
	key = fmt.Sprintf("%d|%s", uid, key)
	if e, ok := r.webIdemReserve(key, hash); ok {
		if e.Busy {
			webErr(w, http.StatusConflict, "همین درخواست در حال انجام است؛ چند لحظه صبر کن")
			return
		}
		if e.Hash != "" && e.Hash != hash {
			webErr(w, http.StatusUnprocessableEntity, "این کلید قبلاً برای درخواست دیگری استفاده شده")
			return
		}
		w.Header().Set("Content-Type", "application/json; charset=utf-8")
		w.Header().Set("X-Idempotent-Replay", "1")
		w.WriteHeader(e.Code)
		_, _ = w.Write(e.Body)
		return
	}
	rec := &webRec{ResponseWriter: w}
	fn(rec, req, uid)
	code := rec.code
	if code == 0 {
		code = http.StatusOK
	}
	// 5xx is a failed attempt: let the client retry with the same key.
	if code < 500 {
		r.webIdemFinish(key, hash, code, rec.buf.Bytes())
	}
	w.WriteHeader(code)
	_, _ = w.Write(rec.buf.Bytes())
}
