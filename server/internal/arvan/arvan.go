package arvan

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strconv"
	"strings"
	"time"
)

const api = "https://napi.arvancloud.ir/cdn/4.0"
const eccBase = "https://napi.arvancloud.ir/ecc/v1"

type Error struct{ Msg string }

func (e *Error) Error() string { return e.Msg }

type Client struct {
	token string
	hc    *http.Client
}

func New(token string) *Client {
	return &Client{token: token, hc: &http.Client{Timeout: 15 * time.Second}}
}

func NormKey(raw string) string {
	t := strings.TrimSpace(raw)
	if strings.HasPrefix(strings.ToLower(t), "apikey") {
		t = strings.TrimSpace(t[6:])
	}
	return t
}

func (c *Client) Call(method, path string, body any) (map[string]any, error) {
	return c.do(api, method, path, body)
}

func (c *Client) do(base, method, path string, body any) (map[string]any, error) {
	var raw []byte
	if body != nil {
		raw, _ = json.Marshal(body)
	}
	var lastErr string
	for attempt := 1; attempt <= 2; attempt++ {
		var rdr io.Reader
		if raw != nil {
			rdr = bytes.NewReader(raw)
		}
		req, err := http.NewRequest(method, base+path, rdr)
		if err != nil {
			return nil, &Error{err.Error()}
		}
		req.Header.Set("Authorization", "Apikey "+c.token)
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("Accept", "application/json")
		resp, err := c.hc.Do(req)
		if err != nil {
			lastErr = trunc(err.Error(), 200)
			if attempt < 2 {
				time.Sleep(time.Duration(attempt) * 500 * time.Millisecond)
				continue
			}
			return nil, &Error{lastErr}
		}
		b, _ := io.ReadAll(resp.Body)
		resp.Body.Close()
		if resp.StatusCode == 429 || resp.StatusCode == 500 || resp.StatusCode == 502 || resp.StatusCode == 503 || resp.StatusCode == 504 {
			lastErr = trunc(string(b), 200)
			if attempt < 2 {
				time.Sleep(time.Duration(attempt) * 500 * time.Millisecond)
				continue
			}
			return nil, &Error{fmt.Sprintf("HTTP %d: %s", resp.StatusCode, lastErr)}
		}
		if resp.StatusCode < 200 || resp.StatusCode >= 300 {
			return nil, &Error{fmt.Sprintf("HTTP %d: %s", resp.StatusCode, trunc(string(b), 200))}
		}
		if len(b) == 0 {
			return map[string]any{}, nil
		}
		var out map[string]any
		if err := json.Unmarshal(b, &out); err != nil {
			return map[string]any{}, nil
		}
		return out, nil
	}
	return nil, &Error{lastErr}
}

// ---------- ECC (cloud server) ----------

// EccRegions lists the available regions on the ECC API.
func (c *Client) EccRegions() (map[string]any, error) {
	return c.do(eccBase, "GET", "/regions", nil)
}

// Ecc performs a request scoped to a region on the ECC API.
func (c *Client) Ecc(region, method, path string, body any) (map[string]any, error) {
	return c.do(eccBase, method, "/regions/"+region+path, body)
}

// EccOneList extracts a list of objects from an ECC response envelope.
func EccOneList(v map[string]any) []map[string]any {
	if v == nil {
		return nil
	}
	if arr, ok := v["data"].([]any); ok {
		return mapList(arr)
	}
	if arr, ok := v["result"].([]any); ok {
		return mapList(arr)
	}
	return nil
}

func eccList(v any) []map[string]any {
	if m, ok := v.(map[string]any); ok {
		return EccOneList(m)
	}
	return nil
}

// EccList fetches a paginated list from a region-scoped ECC path.
func (c *Client) EccList(region, path string) []map[string]any {
	var out []map[string]any
	switch path {
	case "/sizes", "/securities", "/networks":
		if d, err := c.Ecc(region, "GET", path, nil); err == nil {
			return eccList(d)
		}
		return nil
	}
	page := 1
	sep := "?"
	if strings.Contains(path, "?") {
		sep = "&"
	}
	for {
		d, err := c.Ecc(region, "GET", path+sep+"per_page=50&page="+strconv.Itoa(page), nil)
		if err != nil {
			if page == 1 && sep == "?" {
				if d2, err2 := c.Ecc(region, "GET", path, nil); err2 == nil {
					return eccList(d2)
				}
			}
			return out
		}
		arr := eccList(d)
		out = append(out, arr...)
		meta, _ := d["meta"].(map[string]any)
		pag, _ := meta["pagination"].(map[string]any)
		if np := intOr(pag["next_page"]); np > 0 {
			page = np
			continue
		}
		if pag == nil && len(arr) >= 50 {
			page++
			continue
		}
		return out
	}
}

// EccOne extracts a single object from an ECC response.
func EccOne(d map[string]any) map[string]any {
	if d == nil {
		return nil
	}
	if one, ok := d["data"].(map[string]any); ok {
		return one
	}
	if one, ok := d["result"].(map[string]any); ok {
		return one
	}
	return nil
}

func (c *Client) AllPages(path, key string) []map[string]any {
	var out []map[string]any
	page := 1
	sep := "?"
	if strings.Contains(path, "?") {
		sep = "&"
	}
	for {
		d, err := c.Call("GET", path+sep+"per_page=50&page="+strconv.Itoa(page), nil)
		if err != nil {
			return out
		}
		arr := mapList(d[key])
		out = append(out, arr...)
		meta, _ := d["meta"].(map[string]any)
		pag, _ := meta["pagination"].(map[string]any)
		if np := intOr(pag["next_page"]); np > 0 {
			page = np
			continue
		}
		if meta == nil && len(arr) >= 50 {
			page++
			continue
		}
		return out
	}
}

func (c *Client) Domains() []map[string]any { return c.AllPages("/domains", "data") }

func (c *Client) Records(domain string) []map[string]any {
	return c.AllPages("/domains/"+domain+"/dns-records", "data")
}

func mapList(v any) []map[string]any {
	arr, _ := v.([]any)
	var out []map[string]any
	for _, x := range arr {
		if m, ok := x.(map[string]any); ok {
			out = append(out, m)
		}
	}
	return out
}

func intOr(v any) int {
	switch t := v.(type) {
	case float64:
		return int(t)
	case int:
		return t
	case string:
		n, _ := strconv.Atoi(t)
		return n
	}
	return 0
}

func trunc(s string, n int) string {
	if len(s) > n {
		return s[:n]
	}
	return s
}
