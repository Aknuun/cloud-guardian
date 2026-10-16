package cf

import (
	"bytes"
	"encoding/json"
	"io"
	"net/http"
	"regexp"
	"strings"
	"time"
)

// TrafficResult holds DNS query counts or a failure reason.
type TrafficResult struct {
	Counts   map[string]int
	NeedPerm bool
	Error    string
}

var permRe = regexp.MustCompile(`(?i)permission|authz|analytics\.read`)

func gql(token, query string) (map[string]any, error) {
	body, _ := json.Marshal(map[string]any{"query": query})
	req, err := http.NewRequest("POST", api+"/graphql", bytes.NewReader(body))
	if err != nil {
		return nil, &Error{err.Error()}
	}
	req.Header.Set("Authorization", "Bearer "+token)
	req.Header.Set("Content-Type", "application/json")
	hc := &http.Client{Timeout: 25 * time.Second}
	resp, err := hc.Do(req)
	if err != nil {
		return nil, &Error{trunc(err.Error(), 150)}
	}
	defer resp.Body.Close()
	b, _ := io.ReadAll(resp.Body)
	var d struct {
		Data   map[string]any   `json:"data"`
		Errors []map[string]any `json:"errors"`
	}
	if err := json.Unmarshal(b, &d); err != nil {
		return nil, &Error{trunc(err.Error(), 150)}
	}
	if len(d.Errors) > 0 {
		msg := strAny(d.Errors[0]["message"])
		return nil, &Error{trunc(msg, 150)}
	}
	return d.Data, nil
}

// FetchTrafficCounts returns DNS query counts per name over the last `days`.
func FetchTrafficCounts(token, zoneID string, days int) TrafficResult {
	if days != 1 && days != 7 && days != 30 {
		days = 7
	}
	now := time.Now().UTC()
	start := now.Add(-time.Duration(days) * 24 * time.Hour)
	f := func(t time.Time) string { return t.Format("2006-01-02T15:04:05Z") }
	q := `{viewer{zones(filter:{zoneTag:"` + zoneID + `"}){dnsAnalyticsAdaptiveGroups(limit:1000,` +
		`filter:{datetime_geq:"` + f(start) + `",datetime_leq:"` + f(now) + `"},` +
		`orderBy:[count_DESC]){count dimensions{queryName}}}}}`
	d, err := gql(token, q)
	if err != nil {
		if permRe.MatchString(err.Error()) {
			return TrafficResult{NeedPerm: true}
		}
		return TrafficResult{Error: trunc(err.Error(), 200)}
	}
	return parseTraffic(d)
}

func parseTraffic(d map[string]any) TrafficResult {
	counts := map[string]int{}
	viewer, _ := d["viewer"].(map[string]any)
	if viewer == nil {
		return TrafficResult{Counts: counts}
	}
	zones, _ := viewer["zones"].([]any)
	if len(zones) == 0 {
		return TrafficResult{Counts: counts}
	}
	z, _ := zones[0].(map[string]any)
	groups, _ := z["dnsAnalyticsAdaptiveGroups"].([]any)
	for _, gv := range groups {
		g, _ := gv.(map[string]any)
		if g == nil {
			continue
		}
		dims, _ := g["dimensions"].(map[string]any)
		qn := strings.ToLower(strings.TrimRight(strAny(dims["queryName"]), "."))
		if qn == "" {
			continue
		}
		counts[qn] += intOr(g["count"], 0)
	}
	return TrafficResult{Counts: counts}
}

// AccountUsage returns today's Worker invocations and KV operation counts.
func AccountUsage(token string) map[string]any {
	day := time.Now().UTC().Format("2006-01-02")
	out := map[string]any{"requests": "?", "kv": map[string]int{}}
	q1 := `{viewer{accounts{workersInvocationsAdaptive(` +
		`filter:{date_geq:"` + day + `",date_leq:"` + day + `"},limit:10){sum{requests}}}}}`
	if d, err := gql(token, q1); err == nil {
		if viewer, ok := d["viewer"].(map[string]any); ok {
			if arr, ok := viewer["accounts"].([]any); ok && len(arr) > 0 {
				if acct, ok := arr[0].(map[string]any); ok {
					if groups, ok := acct["workersInvocationsAdaptive"].([]any); ok {
						total := 0
						for _, g := range groups {
							if m, ok := g.(map[string]any); ok {
								if s, ok := m["sum"].(map[string]any); ok {
									total += intOr(s["requests"], 0)
								}
							}
						}
						out["requests"] = total
					}
				}
			}
		}
	}
	q2 := `{viewer{accounts{kvOperationsAdaptiveGroups(limit:50,` +
		`filter:{date_geq:"` + day + `",date_leq:"` + day + `"})` +
		`{dimensions{actionType} sum{requests}}}}}`
	if d, err := gql(token, q2); err == nil {
		kv := map[string]int{}
		if viewer, ok := d["viewer"].(map[string]any); ok {
			if arr, ok := viewer["accounts"].([]any); ok && len(arr) > 0 {
				if acct, ok := arr[0].(map[string]any); ok {
					if groups, ok := acct["kvOperationsAdaptiveGroups"].([]any); ok {
						for _, g := range groups {
							if m, ok := g.(map[string]any); ok {
								dims, _ := m["dimensions"].(map[string]any)
								sum, _ := m["sum"].(map[string]any)
								kv[strings.ToLower(strAny(dims["actionType"]))] = intOr(sum["requests"], 0)
							}
						}
					}
				}
			}
		}
		out["kv"] = kv
	}
	return out
}

func intOr(v any, def int) int {
	switch t := v.(type) {
	case float64:
		return int(t)
	case int:
		return t
	case int64:
		return int(t)
	}
	return def
}
