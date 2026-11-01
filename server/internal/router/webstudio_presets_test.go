package router

import (
	"encoding/json"
	"path/filepath"
	"strings"
	"testing"

	"github.com/aknuun/cg-server/internal/store"
)

// The seeded presets must render cleanly for every install: no unknown
// placeholder warnings, a non-empty name, and the custom group label in use.
func TestHsPresetsV2Render(t *testing.T) {
	host := map[string]any{
		"id": "7", "remark": "🇩🇪 DE-01", "address": []any{"49.12.88.14"},
		"inbound_tag": "vless", "transport": "ws", "port": 443, "path": "/ws",
		"sni": []any{"example.com"}, "priority": 1,
	}
	for _, tpl := range hsPresetsV2() {
		out, warns := hsRender(tpl, host, "tunnel", "🇩🇪 DE-01")
		if len(warns) > 0 {
			t.Fatalf("%s: unknown variables %v", tpl.ID, warns)
		}
		if strings.TrimSpace(out) == "" {
			t.Fatalf("%s: rendered empty", tpl.ID)
		}
		if _, ok := tpl.GroupLabels["tunnel"]; ok {
			plain := tpl
			plain.GroupLabels = nil
			base, _ := hsRender(plain, host, "tunnel", "🇩🇪 DE-01")
			if base == out {
				t.Fatalf("%s: group label ignored (%q)", tpl.ID, out)
			}
		}
	}
}

// Templates saved before a field existed must not reach the app as null maps.
func TestHsTemplatesNeverNilMaps(t *testing.T) {
	st, err := store.Open(filepath.Join(t.TempDir(), "t.db"))
	if err != nil {
		t.Fatal(err)
	}
	r := &Router{Store: st}
	for _, tpl := range r.hsTemplates() {
		raw, err := json.Marshal(tpl)
		if err != nil {
			t.Fatal(err)
		}
		for _, key := range []string{"fonts", "emojis", "group_labels", "group_overrides", "host_overrides"} {
			if strings.Contains(string(raw), `"`+key+`":null`) {
				t.Fatalf("%s: %s is null in %s", tpl.ID, key, raw)
			}
		}
	}
}
