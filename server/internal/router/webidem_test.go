package router

import (
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"testing"

	"github.com/aknuun/cg-server/internal/store"
)

func TestWebIdemReplay(t *testing.T) {
	st, err := store.Open(filepath.Join(t.TempDir(), "t.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer st.Close()
	r := &Router{Store: st}
	calls := 0
	fn := func(w http.ResponseWriter, req *http.Request, uid int64) {
		calls++
		webOK(w, map[string]any{"calls": calls})
	}
	mk := func() *http.Request {
		req := httptest.NewRequest("POST", "/x", nil)
		req.Header.Set("Idempotency-Key", "k1")
		return req
	}
	w1 := httptest.NewRecorder()
	r.webIdemRun(w1, mk(), 1, fn)
	w2 := httptest.NewRecorder()
	r.webIdemRun(w2, mk(), 1, fn)
	if calls != 1 {
		t.Fatalf("handler ran %d times, want 1", calls)
	}
	if w2.Header().Get("X-Idempotent-Replay") != "1" {
		t.Fatal("missing replay header")
	}
	if strings.TrimSpace(w1.Body.String()) != strings.TrimSpace(w2.Body.String()) {
		t.Fatalf("body mismatch: %q vs %q", w1.Body.String(), w2.Body.String())
	}
	req3 := httptest.NewRequest("POST", "/x", nil)
	req3.Header.Set("Idempotency-Key", "k2")
	r.webIdemRun(httptest.NewRecorder(), req3, 1, fn)
	if calls != 2 {
		t.Fatalf("new key did not run: %d", calls)
	}
}

func TestWebIdemKeyBoundToBody(t *testing.T) {
	st, err := store.Open(filepath.Join(t.TempDir(), "t2.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer st.Close()
	r := &Router{Store: st}
	hits := 0
	fn := func(w http.ResponseWriter, req *http.Request, uid int64) {
		hits++
		webOK(w, map[string]any{"n": hits})
	}
	call := func(path, body string) *http.Request {
		req := httptest.NewRequest(http.MethodPost, path, strings.NewReader(body))
		req.Header.Set("Idempotency-Key", "same-key")
		return req
	}
	// first request reserves the key for /a with body {"x":1}
	w1 := httptest.NewRecorder()
	r.webIdemRun(w1, call("/a", `{"x":1}`), 1, fn)
	// same key, different path -> must not replay the first response
	w2 := httptest.NewRecorder()
	r.webIdemRun(w2, call("/b", `{"x":1}`), 1, fn)
	if w2.Code != http.StatusUnprocessableEntity {
		t.Fatalf("different path: code = %d, want 422", w2.Code)
	}
	// same key, same path, different body -> 422
	w3 := httptest.NewRecorder()
	r.webIdemRun(w3, call("/a", `{"x":2}`), 1, fn)
	if w3.Code != http.StatusUnprocessableEntity {
		t.Fatalf("different body: code = %d, want 422", w3.Code)
	}
	// identical retry -> replayed, handler not run again
	w4 := httptest.NewRecorder()
	r.webIdemRun(w4, call("/a", `{"x":1}`), 1, fn)
	if hits != 1 {
		t.Fatalf("handler ran %d times, want 1", hits)
	}
	if w4.Header().Get("X-Idempotent-Replay") != "1" {
		t.Fatalf("expected replay header")
	}
}
