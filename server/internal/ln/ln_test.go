package ln

import (
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

func testClient(t *testing.T, status int, body string) *Client {
	t.Helper()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(status)
		_, _ = w.Write([]byte(body))
	}))
	t.Cleanup(srv.Close)
	c := New("tok")
	c.base = srv.URL
	c.hc = &http.Client{Timeout: 5 * time.Second}
	return c
}

func TestCallEmptyErrorBodyIsError(t *testing.T) {
	retryDelay = time.Millisecond
	t.Cleanup(func() { retryDelay = 2 * time.Second })
	c := testClient(t, http.StatusBadGateway, "")
	if _, err := c.Call("GET", "/linode/instances", nil); err == nil {
		t.Fatal("expected error for 502 with empty body")
	}
}

func TestCallAPIErrorIsReturned(t *testing.T) {
	c := testClient(t, http.StatusUnauthorized, `{"errors":[{"reason":"Invalid Token"}]}`)
	_, err := c.Call("GET", "/account", nil)
	if err == nil || err.Error() != "Invalid Token" {
		t.Fatalf("err = %v", err)
	}
}

func TestOnceOK(t *testing.T) {
	c := testClient(t, http.StatusOK, `{"data":[]}`)
	out, status, err := c.once("GET", "/linode/instances", nil)
	if err != nil || status != http.StatusOK {
		t.Fatalf("err=%v status=%d", err, status)
	}
	if _, ok := out["data"].([]any); !ok {
		t.Fatalf("unexpected body: %v", out)
	}
}

func TestOnceNonJSONError(t *testing.T) {
	c := testClient(t, http.StatusNotFound, "<html>nope</html>")
	_, _, err := c.once("GET", "/x", nil)
	if err == nil {
		t.Fatal("expected error for non-JSON 404")
	}
}
