// Package cf is a minimal Cloudflare API v4 client (zones + DNS + email routing).
package cf

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"
)

const api = "https://api.cloudflare.com/client/v4"

type Error struct{ Msg string }

func (e *Error) Error() string { return e.Msg }

type Client struct {
	token string
	hc    *http.Client
}

func New(token string) *Client {
	return &Client{token: token, hc: &http.Client{Timeout: 25 * time.Second}}
}

// Account is a stored Cloudflare credential entry.
type Account struct {
	Name  string `json:"name"`
	Token string `json:"token"`
}

func req(c *Client, method, path string, body any, out any) error {
	var rd io.Reader
	if body != nil {
		b, _ := json.Marshal(body)
		rd = bytes.NewReader(b)
	}
	r, err := http.NewRequest(method, api+path, rd)
	if err != nil {
		return &Error{err.Error()}
	}
	r.Header.Set("Authorization", "Bearer "+c.token)
	r.Header.Set("Content-Type", "application/json")
	resp, err := c.hc.Do(r)
	if err != nil {
		return &Error{trunc(err.Error(), 200)}
	}
	defer resp.Body.Close()
	var envelope struct {
		Success bool `json:"success"`
		Errors  []struct {
			Message string `json:"message"`
		} `json:"errors"`
		Result json.RawMessage `json:"result"`
	}
	b, _ := io.ReadAll(resp.Body)
	if err := json.Unmarshal(b, &envelope); err != nil {
		return &Error{trunc(err.Error(), 200)}
	}
	if !envelope.Success {
		msg := "cloudflare error"
		if len(envelope.Errors) > 0 && envelope.Errors[0].Message != "" {
			msg = envelope.Errors[0].Message
		}
		return &Error{trunc(msg, 200)}
	}
	if out != nil && len(envelope.Result) > 0 {
		if err := json.Unmarshal(envelope.Result, out); err != nil {
			return &Error{trunc(err.Error(), 200)}
		}
	}
	return nil
}

type Zone struct {
	ID     string `json:"id"`
	Name   string `json:"name"`
	Status string `json:"status"`
}

type Record struct {
	ID       string `json:"id"`
	Type     string `json:"type"`
	Name     string `json:"name"`
	Content  string `json:"content"`
	TTL      any    `json:"ttl"`
	Proxied  any    `json:"proxied"`
	Priority any    `json:"priority,omitempty"`
}

// Zones returns every zone of the account (follows pagination).
func (c *Client) Zones() ([]Zone, error) {
	var all []Zone
	for page := 1; page <= 20; page++ {
		var z []Zone
		err := req(c, "GET", fmt.Sprintf("/zones?per_page=50&page=%d", page), nil, &z)
		if err != nil {
			return all, err
		}
		all = append(all, z...)
		if len(z) < 50 {
			break
		}
	}
	return all, nil
}

// Records returns every DNS record of a zone (follows pagination).
func (c *Client) Records(zoneID string) ([]Record, error) {
	var all []Record
	for page := 1; page <= 20; page++ {
		var r []Record
		err := req(c, "GET", fmt.Sprintf("/zones/%s/dns_records?per_page=100&page=%d", zoneID, page), nil, &r)
		if err != nil {
			return all, err
		}
		all = append(all, r...)
		if len(r) < 100 {
			break
		}
	}
	return all, nil
}

func (c *Client) AddRecord(zoneID, rtype, name, content string, ttl int, proxied bool, priority *int) (*Record, error) {
	body := map[string]any{
		"type": rtype, "name": name, "content": content,
		"ttl": ttl, "proxied": proxied,
	}
	if priority != nil {
		body["priority"] = *priority
	}
	var r Record
	err := req(c, "POST", "/zones/"+zoneID+"/dns_records", body, &r)
	return &r, err
}

func (c *Client) DelRecord(zoneID, recID string) error {
	return req(c, "DELETE", "/zones/"+zoneID+"/dns_records/"+recID, nil, nil)
}

func (c *Client) UpdateRecord(zoneID string, rec Record, changes map[string]any) (*Record, error) {
	body := map[string]any{
		"type": rec.Type, "name": rec.Name, "content": rec.Content,
		"ttl": ttlOf(rec.TTL),
	}
	// proxied is only valid for proxyable types; sending it for TXT/MX/NS is
	// rejected by the API, so keep it conditional.
	switch strings.ToUpper(rec.Type) {
	case "A", "AAAA", "CNAME":
		body["proxied"] = boolOf(rec.Proxied)
	}
	if rec.Priority != nil {
		body["priority"] = rec.Priority
	}
	for k, v := range changes {
		if k == "proxied" {
			switch strings.ToUpper(fmt.Sprint(body["type"])) {
			case "A", "AAAA", "CNAME":
			default:
				continue
			}
		}
		body[k] = v
	}
	var r Record
	err := req(c, "PUT", "/zones/"+zoneID+"/dns_records/"+rec.ID, body, &r)
	return &r, err
}

func (c *Client) Zone(zoneID string) (map[string]any, error) {
	var z map[string]any
	err := req(c, "GET", "/zones/"+zoneID, nil, &z)
	return z, err
}

// Settings returns the zone settings as a map of id -> value.
func (c *Client) Settings(zoneID string) (map[string]any, error) {
	var list []map[string]any
	if err := req(c, "GET", "/zones/"+zoneID+"/settings", nil, &list); err != nil {
		return nil, err
	}
	out := map[string]any{}
	for _, s := range list {
		out[strAny(s["id"])] = s["value"]
	}
	return out, nil
}

func (c *Client) SetSetting(zoneID, key string, value any) error {
	return req(c, "PATCH", "/zones/"+zoneID+"/settings/"+key,
		map[string]any{"value": value}, nil)
}

func (c *Client) SetZonePaused(zoneID string, paused bool) error {
	return req(c, "PATCH", "/zones/"+zoneID, map[string]any{"paused": paused}, nil)
}

func (c *Client) PurgeCache(zoneID string) error {
	return req(c, "POST", "/zones/"+zoneID+"/purge_cache",
		map[string]any{"purge_everything": true}, nil)
}

func (c *Client) AccountID(zoneID string) string {
	z, _ := c.Zone(zoneID)
	if acct, ok := z["account"].(map[string]any); ok {
		return strAny(acct["id"])
	}
	return ""
}

func (c *Client) EmailRouting(zoneID string) map[string]any {
	var m map[string]any
	req(c, "GET", "/zones/"+zoneID+"/email/routing", nil, &m)
	return m
}

func (c *Client) EmailRules(zoneID string) ([]map[string]any, error) {
	var r []map[string]any
	err := req(c, "GET", "/zones/"+zoneID+"/email/routing/rules", nil, &r)
	return r, err
}

func (c *Client) EmailRuleAdd(zoneID, name, addr, dest string) error {
	if len(name) > 60 {
		name = name[:60]
	}
	body := map[string]any{
		"name": name, "enabled": true,
		"matchers": []any{map[string]any{"type": "literal", "field": "to", "value": addr}},
		"actions":  []any{map[string]any{"type": "forward", "value": []string{dest}}},
	}
	return req(c, "POST", "/zones/"+zoneID+"/email/routing/rules", body, nil)
}

func (c *Client) EmailRuleDel(zoneID, ruleID string) error {
	return req(c, "DELETE", "/zones/"+zoneID+"/email/routing/rules/"+ruleID, nil, nil)
}

func (c *Client) EmailCatchAll(zoneID string) map[string]any {
	var m map[string]any
	req(c, "GET", "/zones/"+zoneID+"/email/routing/rules/catch_all", nil, &m)
	return m
}

func (c *Client) EmailCatchAllSet(zoneID string, body map[string]any) error {
	return req(c, "PUT", "/zones/"+zoneID+"/email/routing/rules/catch_all", body, nil)
}

func (c *Client) EmailEnable(zoneID string) error {
	return req(c, "POST", "/zones/"+zoneID+"/email/routing/enable", map[string]any{}, nil)
}

func (c *Client) EmailDests(accountID string) ([]map[string]any, error) {
	var r []map[string]any
	err := req(c, "GET", "/accounts/"+accountID+"/email/routing/addresses", nil, &r)
	return r, err
}

func (c *Client) EmailDestAdd(accountID, email string) error {
	return req(c, "POST", "/accounts/"+accountID+"/email/routing/addresses",
		map[string]any{"email": email}, nil)
}

// ---------- helpers ----------

func ttlOf(v any) int {
	switch t := v.(type) {
	case float64:
		if t < 1 {
			return 1
		}
		return int(t)
	case int:
		return t
	}
	return 1
}

func boolOf(v any) bool {
	b, ok := v.(bool)
	return ok && b
}

func strAny(v any) string {
	if s, ok := v.(string); ok {
		return s
	}
	if v != nil {
		return fmt.Sprint(v)
	}
	return ""
}

func trunc(s string, n int) string {
	if len(s) > n {
		return s[:n]
	}
	return s
}
