package ln

import (
	"bytes"
	"encoding/json"
	"io"
	"net/http"
	"strconv"
	"strings"
	"time"
)

const api = "https://api.linode.com/v4"

type Error struct{ Msg string }

func (e *Error) Error() string { return e.Msg }

type Client struct {
	token string
	hc    *http.Client
}

func New(token string) *Client {
	return &Client{token: token, hc: &http.Client{Timeout: 20 * time.Second}}
}

func (c *Client) Call(method, path string, body any) (map[string]any, error) {
	var rdr io.Reader
	if body != nil {
		b, _ := json.Marshal(body)
		rdr = bytes.NewReader(b)
	}
	req, err := http.NewRequest(method, api+path, rdr)
	if err != nil {
		return nil, &Error{err.Error()}
	}
	req.Header.Set("Authorization", "Bearer "+c.token)
	req.Header.Set("Content-Type", "application/json")
	resp, err := c.hc.Do(req)
	if err != nil {
		return nil, &Error{trunc(err.Error(), 200)}
	}
	defer resp.Body.Close()
	b, _ := io.ReadAll(resp.Body)
	out := map[string]any{}
	if len(b) > 0 {
		_ = json.Unmarshal(b, &out)
	}
	if errs, ok := out["errors"].([]any); ok && len(errs) > 0 {
		var msgs []string
		for _, e := range errs {
			if m, ok := e.(map[string]any); ok {
				msgs = append(msgs, strAny(m["reason"]))
			} else {
				msgs = append(msgs, strAny(e))
			}
		}
		return nil, &Error{trunc(strings.Join(msgs, "; "), 200)}
	}
	return out, nil
}

func (c *Client) All(path string) []map[string]any {
	var out []map[string]any
	page := 1
	sep := "?"
	if strings.Contains(path, "?") {
		sep = "&"
	}
	for {
		d, err := c.Call("GET", path+sep+"page="+strconv.Itoa(page)+"&page_size=500", nil)
		if err != nil {
			return out
		}
		out = append(out, mapList(d["data"])...)
		pages := intOr(d["pages"])
		if pages == 0 || page >= pages {
			return out
		}
		page++
	}
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

func strAny(v any) string {
	if s, ok := v.(string); ok {
		return s
	}
	if v == nil {
		return ""
	}
	return strings.TrimSpace(strings.ReplaceAll(strings.TrimSpace(toJSON(v)), "\"", ""))
}

func toJSON(v any) string {
	b, _ := json.Marshal(v)
	return string(b)
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

// Disk returns the primary (non-swap) disk of a Linode instance.
func (c *Client) Disk(serverID string) (map[string]any, error) {
	disks := c.All("/linode/instances/" + serverID + "/disks")
	if len(disks) == 0 {
		return nil, nil
	}
	for _, d := range disks {
		fs := strAny(d["filesystem"])
		if fs != "" && fs != "swap" {
			return d, nil
		}
	}
	return disks[0], nil
}
