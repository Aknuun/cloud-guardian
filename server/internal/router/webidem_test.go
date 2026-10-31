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
