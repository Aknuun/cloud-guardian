package router

import "testing"

func TestWebRateLimit(t *testing.T) {
	uid := int64(999001)
	for i := 0; i < webRateLimit; i++ {
		if !webRateAllow(uid) {
			t.Fatalf("blocked too early at %d", i)
		}
	}
	if webRateAllow(uid) {
		t.Fatal("expected the request over the limit to be blocked")
	}
	// other admins keep their own budget
	if !webRateAllow(uid + 1) {
		t.Fatal("limit leaked across users")
	}
}
