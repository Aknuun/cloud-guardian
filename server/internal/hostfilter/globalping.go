package hostfilter

import (
	"bytes"
	"encoding/json"
	"io"
	"net/http"
	"strconv"
	"strings"
	"time"
)

// globalpingAPI is the fallback probe network used when check-host.net does not
// answer (or answers without usable samples). It reports ICMP ping results per
// probe, which is enough to fill the per-city map that check-host provides.
const globalpingAPI = "https://api.globalping.io/v1"

type gpProbe struct {
	Country string
	City    string
	Ok      int
	Total   int
}

// nodeCountry maps a probe node to the country used for the Globalping query.
func nodeCountry(n node) string {
	switch strings.ToLower(n.City) {
	case "tehran", "isfahan", "shiraz":
		return "IR"
	case "frankfurt", "nuremberg":
		return "DE"
	case "amsterdam", "meppel":
		return "NL"
	}
	switch {
	case strings.HasPrefix(n.ID, "ir"):
		return "IR"
	case strings.HasPrefix(n.ID, "de"):
		return "DE"
	case strings.HasPrefix(n.ID, "nl"):
		return "NL"
	}
	return ""
}

// gpMerge turns Globalping probes into the same per-node map used by
// checkPing. City-level data is preferred; when the fallback has no probe in
// that exact city, the country aggregate is used for every node of that
// country so a single result still produces a usable verdict.
func gpMerge(nodes []node, probes []gpProbe) map[string]pingRes {
	out := map[string]pingRes{}
	for _, n := range nodes {
		cc := nodeCountry(n)
		if cc == "" {
			continue
		}
		var city gpProbe
		for _, p := range probes {
			if p.Country != cc || p.Total <= 0 {
				continue
			}
			if p.City != "" && strings.EqualFold(p.City, n.City) {
				city.Ok += p.Ok
				city.Total += p.Total
			}
		}
		if city.Total > 0 {
			out[n.ID] = pingRes{City: n.City, Ok: city.Ok, Total: city.Total}
			continue
		}
		var country gpProbe
		for _, p := range probes {
			if p.Country == cc && p.Total > 0 {
				country.Ok += p.Ok
				country.Total += p.Total
			}
		}
		if country.Total > 0 {
			out[n.ID] = pingRes{City: n.City, Ok: country.Ok, Total: country.Total}
		}
	}
	return out
}

// globalPing probes host through Globalping and returns per-node results. It
// returns nil when the service is unavailable or reports no samples.
func globalPing(host string, nodes []node) map[string]pingRes {
	var countries []string
	for _, n := range nodes {
		if cc := nodeCountry(n); cc != "" && !contains(countries, cc) {
			countries = append(countries, cc)
		}
	}
	if len(countries) == 0 {
		return nil
	}
	locations := make([]map[string]any, 0, len(countries))
	for _, cc := range countries {
		locations = append(locations, map[string]any{"country": cc, "limit": 3})
	}
	body, _ := json.Marshal(map[string]any{
		"type":               "ping",
		"target":             host,
		"locations":          locations,
		"measurementOptions": map[string]any{"packets": 3},
	})
	var created struct {
		ID string `json:"id"`
	}
	if err := httpPostJSON(globalpingAPI+"/measurements", body, &created); err != nil || created.ID == "" {
		return nil
	}
	for attempt := 0; attempt < 10; attempt++ {
		time.Sleep(2 * time.Second)
		var res struct {
			Status  string `json:"status"`
			Results []struct {
				Probe struct {
					Country string `json:"country"`
					City    string `json:"city"`
				} `json:"probe"`
				Result struct {
					Stats struct {
						Rcv   int `json:"rcv"`
						Drop  int `json:"drop"`
						Total int `json:"total"`
					} `json:"stats"`
				} `json:"result"`
			} `json:"results"`
		}
		if err := httpGetJSON(globalpingAPI+"/measurements/"+created.ID, &res); err != nil {
			continue
		}
		if res.Status != "finished" {
			continue
		}
		probes := make([]gpProbe, 0, len(res.Results))
		for _, r := range res.Results {
			st := r.Result.Stats
			total := st.Total
			if total <= 0 {
				total = st.Rcv + st.Drop
			}
			if total <= 0 {
				continue
			}
			probes = append(probes, gpProbe{
				Country: strings.ToUpper(r.Probe.Country),
				City:    r.Probe.City,
				Ok:      st.Rcv,
				Total:   total,
			})
		}
		if len(probes) == 0 {
			return nil
		}
		return gpMerge(nodes, probes)
	}
	return nil
}

func httpPostJSON(u string, payload []byte, out any) error {
	req, err := http.NewRequest("POST", u, bytes.NewReader(payload))
	if err != nil {
		return err
	}
	req.Header.Set("User-Agent", httpUA)
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "application/json")
	hc := &http.Client{Timeout: 20 * time.Second}
	resp, err := hc.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	b, _ := io.ReadAll(resp.Body)
	if resp.StatusCode >= 400 {
		return &httpStatusError{resp.StatusCode}
	}
	return json.Unmarshal(b, out)
}

type httpStatusError struct{ code int }

func (e *httpStatusError) Error() string { return "HTTP " + strconv.Itoa(e.code) }
