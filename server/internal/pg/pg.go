// Package pg is a read-only client for Passargad/Marzban panel APIs.
package pg

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"
)

var ua = "Mozilla/5.0 (Windows NT 10.0; Win64; x64)"

type Error struct{ Msg string }

func (e *Error) Error() string { return e.Msg }

func doJSON(req *http.Request, timeout time.Duration, out any) error {
	hc := &http.Client{Timeout: timeout}
	resp, err := hc.Do(req)
	if err != nil {
		return &Error{trunc(err.Error(), 200)}
	}
	defer resp.Body.Close()
	if resp.StatusCode != 200 {
		return &Error{fmt.Sprintf("HTTP %d", resp.StatusCode)}
	}
	b, _ := io.ReadAll(resp.Body)
	if out == nil {
		return nil
	}
	if err := json.Unmarshal(b, out); err != nil {
		return &Error{trunc(err.Error(), 200)}
	}
	return nil
}

type Pg struct {
	Base, Username, Password string
}

// Panel is a stored PasarGuard panel credential entry.
type Panel struct {
	URL      string `json:"url"`
	Username string `json:"username"`
	Password string `json:"password"`
	Name     string `json:"name"`
}

func (p Panel) Client() *Pg { return New(p.URL, p.Username, p.Password) }

// Panels loads stored panels.
func Panels(st interface{ GetJSON(string, any) bool }) []Panel {
	var lst []Panel
	st.GetJSON("panels", &lst)
	return lst
}

// StorePanelJSON persists panels.
func StorePanels(st interface{ PutJSON(string, any, int) }, lst []Panel) {
	st.PutJSON("panels", lst, 0)
}

func New(rawurl, user, pass string) *Pg {
	return &Pg{Base: strings.TrimRight(rawurl, "/"), Username: user, Password: pass}
}

func (p *Pg) get(token, path string, timeout time.Duration, out any) error {
	req, err := http.NewRequest("GET", p.Base+path, nil)
	if err != nil {
		return &Error{err.Error()}
	}
	req.Header.Set("Authorization", "Bearer "+token)
	req.Header.Set("User-Agent", ua)
	return doJSON(req, timeout, out)
}

func (p *Pg) Login() (string, error) {
	form := url.Values{"username": {p.Username}, "password": {p.Password}}
	req, _ := http.NewRequest("POST", p.Base+"/api/admin/token",
		strings.NewReader(form.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.Header.Set("User-Agent", ua)
	var d struct {
		AccessToken string `json:"access_token"`
	}
	if err := doJSON(req, 20*time.Second, &d); err != nil {
		return "", err
	}
	if d.AccessToken == "" {
		return "", &Error{"ورود ناموفق (یوزر/پسورد؟)"}
	}
	return d.AccessToken, nil
}

type UserInfo struct {
	Username string  `json:"username"`
	Used     float64 `json:"used"`
	Limit    float64 `json:"limit"`
	Expire   any     `json:"expire"`
	Status   string  `json:"status"`
	Owner    string  `json:"owner"`
}

type rawUser struct {
	Username    any `json:"username"`
	ID          any `json:"id"`
	UsedTraffic any `json:"used_traffic"`
	DataLimit   any `json:"data_limit"`
	Expire      any `json:"expire"`
	ExpireDate  any `json:"expire_date"`
	Status      any `json:"status"`
	Admin       any `json:"admin"`
}

func ownerOf(adm any) string {
	switch t := adm.(type) {
	case map[string]any:
		if u, ok := t["username"].(string); ok && u != "" {
			return u
		}
	case string:
		if t != "" {
			return t
		}
	}
	return "-"
}

func (p *Pg) Users(token string, limit int) ([]UserInfo, error) {
	if limit <= 0 {
		limit = 1000
	}
	out := []UserInfo{}
	offset := 0
	for i := 0; i < 6; i++ {
		var d struct {
			Users []rawUser `json:"users"`
			Total int       `json:"total"`
		}
		path := fmt.Sprintf("/api/users?limit=%d&offset=%d", limit, offset)
		if err := p.get(token, path, 25*time.Second, &d); err != nil {
			return out, err
		}
		if len(d.Users) == 0 {
			break
		}
		for _, u := range d.Users {
			un := strOr(u.Username, u.ID)
			exp := u.Expire
			if exp == nil {
				exp = u.ExpireDate
			}
			out = append(out, UserInfo{
				Username: un,
				Used:     toF(u.UsedTraffic),
				Limit:    toF(u.DataLimit),
				Expire:   exp,
				Status:   strOr(u.Status, ""),
				Owner:    ownerOf(u.Admin),
			})
		}
		offset += len(d.Users)
		if d.Total != 0 && offset >= d.Total {
			break
		}
	}
	return out, nil
}

func (p *Pg) UsersSince(token, sinceISO string) ([]map[string]any, error) {
	out := []map[string]any{}
	offset := 0
	for i := 0; i < 6; i++ {
		var d struct {
			Users []rawUser `json:"users"`
		}
		path := "/api/users?online_after=" + url.QueryEscape(sinceISO) +
			fmt.Sprintf("&limit=5000&offset=%d", offset)
		if err := p.get(token, path, 30*time.Second, &d); err != nil {
			return out, err
		}
		if len(d.Users) == 0 {
			break
		}
		for _, u := range d.Users {
			out = append(out, map[string]any{
				"id":       strOr(u.ID, u.Username),
				"username": strOr(u.Username, ""),
				"used":     toF(u.UsedTraffic),
				"owner":    ownerOf(u.Admin),
			})
		}
		offset += len(d.Users)
	}
	return out, nil
}

type NodeInfo struct {
	Name    string `json:"name"`
	Status  string `json:"status"`
	Address any    `json:"address"`
	Version any    `json:"version"`
	Reason  any    `json:"reason"`
}

func (p *Pg) Nodes(token string) ([]NodeInfo, error) {
	var raw any
	if err := p.get(token, "/api/nodes", 15*time.Second, &raw); err != nil {
		return nil, err
	}
	arr := toArray(raw)
	out := []NodeInfo{}
	for _, n := range arr {
		m, ok := n.(map[string]any)
		if !ok {
			continue
		}
		out = append(out, NodeInfo{
			Name:    strOr(m["name"], firstNonNil(m["node_name"], m["id"])),
			Status:  strOr(m["status"], "unknown"),
			Address: m["address"],
			Version: firstNonNil(m["xray_version"], m["version"]),
			Reason:  firstNonNil(m["message"], m["error"]),
		})
	}
	return out, nil
}

func (p *Pg) Settings(token string) map[string]any {
	var d map[string]any
	if err := p.get(token, "/api/settings", 25*time.Second, &d); err != nil {
		return nil
	}
	return d
}

func (p *Pg) Hosts(token string) ([]any, error) {
	var d []any
	if err := p.get(token, "/api/hosts", 25*time.Second, &d); err != nil {
		return nil, err
	}
	return d, nil
}

// PutHost updates a single host via PUT /api/host/{id}.
func (p *Pg) PutHost(token, id string, body map[string]any) error {
	b, _ := json.Marshal(body)
	req, err := http.NewRequest("PUT",
		strings.TrimRight(p.Base, "/")+"/api/host/"+id, bytes.NewReader(b))
	if err != nil {
		return &Error{err.Error()}
	}
	req.Header.Set("Authorization", "Bearer "+token)
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("User-Agent", ua)
	hc := &http.Client{Timeout: 30 * time.Second}
	resp, err := hc.Do(req)
	if err != nil {
		return &Error{trunc(err.Error(), 200)}
	}
	defer resp.Body.Close()
	if resp.StatusCode != 200 {
		bb, _ := io.ReadAll(resp.Body)
		return &Error{fmt.Sprintf("HTTP %d: %s", resp.StatusCode, trunc(string(bb), 120))}
	}
	return nil
}

// SetHosts replaces the whole host list via PUT /api/hosts.
func (p *Pg) SetHosts(token string, hosts []map[string]any) error {
	b, _ := json.Marshal(hosts)
	req, err := http.NewRequest("PUT", p.Base+"/api/hosts", bytes.NewReader(b))
	if err != nil {
		return &Error{err.Error()}
	}
	req.Header.Set("Authorization", "Bearer "+token)
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("User-Agent", ua)
	return doJSON(req, 30*time.Second, nil)
}

func (p *Pg) User(token, username string) map[string]any {
	var d map[string]any
	if err := p.get(token, "/api/user/"+url.PathEscape(username), 20*time.Second, &d); err != nil {
		return nil
	}
	return d
}

// SetRecurrent PUTs a new webhook recurrent interval (seconds).
func (p *Pg) SetRecurrent(token string, n int) bool {
	st := p.Settings(token)
	if st == nil {
		return false
	}
	wh, _ := st["webhook"].(map[string]any)
	if wh == nil {
		wh = map[string]any{}
		st["webhook"] = wh
	}
	wh["recurrent"] = n
	b, _ := json.Marshal(st)
	req, err := http.NewRequest("PUT", p.Base+"/api/settings", bytes.NewReader(b))
	if err != nil {
		return false
	}
	req.Header.Set("Authorization", "Bearer "+token)
	req.Header.Set("Content-Type", "application/json")
	hc := &http.Client{Timeout: 25 * time.Second}
	resp, err := hc.Do(req)
	if err != nil {
		return false
	}
	defer resp.Body.Close()
	return resp.StatusCode == 200
}

// ---------- helpers ----------

const GB = 1024 * 1024 * 1024

func GBOf(v any) float64 {
	f := toF(v) / GB
	return round2(f)
}

func Pct(used, limit float64) *float64 {
	if limit > 0 {
		p := round1(used / limit * 100)
		return &p
	}
	return nil
}

func DaysLeft(expire any) *float64 {
	ms, ok := expireToMS(expire)
	if !ok {
		return nil
	}
	d := round1(float64(ms-time.Now().UnixMilli()) / 86400000)
	return &d
}

func expireToMS(exp any) (int64, bool) {
	switch t := exp.(type) {
	case nil:
		return 0, false
	case float64:
		if t == 0 {
			return 0, false
		}
		if t < 1e12 {
			return int64(t * 1000), true
		}
		return int64(t), true
	case int64:
		if t == 0 {
			return 0, false
		}
		if t < 1e12 {
			return t * 1000, true
		}
		return t, true
	case string:
		if t == "" {
			return 0, false
		}
		if ts, err := time.Parse(time.RFC3339, strings.Replace(t, "Z", "+00:00", 1)); err == nil {
			return ts.UnixMilli(), true
		}
	}
	return 0, false
}

func toF(v any) float64 {
	switch t := v.(type) {
	case float64:
		return t
	case int:
		return float64(t)
	case int64:
		return float64(t)
	case string:
		var f float64
		fmt.Sscanf(t, "%g", &f)
		return f
	}
	return 0
}

func toArray(v any) []any {
	switch t := v.(type) {
	case []any:
		return t
	case map[string]any:
		for _, k := range []string{"nodes", "items", "result"} {
			if a, ok := t[k].([]any); ok {
				return a
			}
		}
	}
	return nil
}

func strOr(v any, alt any) string {
	if s, ok := v.(string); ok && s != "" {
		return s
	}
	if s, ok := alt.(string); ok {
		return s
	}
	if v != nil {
		return fmt.Sprint(v)
	}
	if alt != nil {
		return fmt.Sprint(alt)
	}
	return ""
}

func firstNonNil(a, b any) any {
	if a != nil {
		return a
	}
	return b
}

func round1(f float64) float64 { return float64(int(f*10+0.5)) / 10 }
func round2(f float64) float64 { return float64(int(f*100+0.5)) / 100 }

func trunc(s string, n int) string {
	if len(s) > n {
		return s[:n]
	}
	return s
}
