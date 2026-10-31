package cf

import (
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"
)

func TestRetryOnRateLimit(t *testing.T) {
	var calls int32
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		n := atomic.AddInt32(&calls, 1)
		if n < 3 {
			w.WriteHeader(http.StatusTooManyRequests)
			_, _ = w.Write([]byte(`{"success":false,"errors":[{"message":"rate limited"}]}`))
			return
		}
		_, _ = w.Write([]byte(`{"success":true,"result":[{"id":"z1","name":"example.com","status":"active"}],"result_info":{"page":1,"total_pages":1}}`))
	}))
	defer ts.Close()

	zones, err := NewAt(ts.URL, "tok").Zones()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(zones) != 1 || zones[0].Name != "example.com" {
		t.Fatalf("unexpected zones: %+v", zones)
	}
	if calls != 3 {
		t.Fatalf("expected 3 attempts, got %d", calls)
	}
}

func TestNoRetryOnBadToken(t *testing.T) {
	var calls int32
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		atomic.AddInt32(&calls, 1)
		_, _ = w.Write([]byte(`{"success":false,"errors":[{"message":"Invalid token"}]}`))
	}))
	defer ts.Close()

	if _, err := NewAt(ts.URL, "bad").Zones(); err == nil {
		t.Fatal("expected error")
	}
	if calls != 1 {
		t.Fatalf("auth errors must not be retried, got %d calls", calls)
	}
}
