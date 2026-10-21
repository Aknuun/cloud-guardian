package arvan

import "testing"

func TestIdempotent(t *testing.T) {
	for _, m := range []string{"GET", "HEAD", "PUT", "DELETE"} {
		if !idempotent(m) {
			t.Fatalf("%s should be retryable", m)
		}
	}
	for _, m := range []string{"POST", "PATCH"} {
		if idempotent(m) {
			t.Fatalf("%s must not be retried (duplicate side effects)", m)
		}
	}
}

func TestNormKey(t *testing.T) {
	if got := NormKey("  Apikey  abc-123 "); got != "abc-123" {
		t.Fatalf("got %q", got)
	}
	if got := NormKey("abc-123"); got != "abc-123" {
		t.Fatalf("got %q", got)
	}
}
