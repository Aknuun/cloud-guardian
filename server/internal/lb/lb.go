package lb

import (
	"crypto/rand"
	"encoding/hex"
	"sort"
	"strings"

	"github.com/aknuun/cg-server/internal/cf"
	"github.com/aknuun/cg-server/internal/store"
)

var LbTypes = []string{"A", "AAAA", "CNAME"}

func IsLbRec(t string) bool {
	switch strings.ToUpper(t) {
	case "A", "AAAA", "CNAME":
		return true
	}
	return false
}

func KeyFor(zid, name string) string { return "lb:cf:" + zid + ":" + name }

type Spare struct {
	Type    string `json:"type"`
	Content string `json:"content"`
}

type Disabled struct {
	Type    string `json:"type"`
	Content string `json:"content"`
	Proxied bool   `json:"proxied"`
	TTL     any    `json:"ttl"`
}

type Config struct {
	Weights  map[string]float64 `json:"weights"`
	Disabled []Disabled         `json:"disabled"`
	Spares   map[string]Spare   `json:"spares"`
}

func GetCfg(s *store.Store, key string) Config {
	var cfg Config
	s.GetJSON(key, &cfg)
	if cfg.Weights == nil {
		cfg.Weights = map[string]float64{}
	}
	if cfg.Disabled == nil {
		cfg.Disabled = []Disabled{}
	}
	if cfg.Spares == nil {
		cfg.Spares = map[string]Spare{}
	}
	return cfg
}

func SaveCfg(s *store.Store, key string, cfg Config) {
	s.PutJSON(key, cfg, 0)
}

type Entry struct {
	On      bool
	ID      string
	Type    string
	Content string
	Proxied bool
	TTL     any
	Weight  float64
}

func weightOf(cfg Config, content string) float64 {
	w := cfg.Weights[content]
	if w == 0 {
		return 1
	}
	return w
}

func EntriesOf(group []cf.Record, cfg Config) []Entry {
	var out []Entry
	for _, r := range group {
		out = append(out, Entry{
			On:      true,
			ID:      r.ID,
			Type:    strings.ToUpper(r.Type),
			Content: r.Content,
			Proxied: boolOf(r.Proxied),
			TTL:     r.TTL,
			Weight:  weightOf(cfg, r.Content),
		})
	}
	for _, d := range cfg.Disabled {
		out = append(out, Entry{
			On:      false,
			Type:    strings.ToUpper(d.Type),
			Content: d.Content,
			Proxied: d.Proxied,
			TTL:     d.TTL,
			Weight:  weightOf(cfg, d.Content),
		})
	}
	sort.SliceStable(out, func(i, j int) bool { return out[i].Content < out[j].Content })
	return out
}

func SpareOf(cfg Config, content string) *Spare {
	if sp, ok := cfg.Spares[content]; ok {
		return &sp
	}
	return nil
}

func NewGid(s *store.Store, acc int, zid, name string) string {
	b := make([]byte, 4)
	_, _ = rand.Read(b)
	gid := hex.EncodeToString(b)
	s.PutJSON("lbg:"+gid, map[string]any{"acc": acc, "zid": zid, "name": name}, 86400)
	return gid
}

func boolOf(v any) bool {
	switch t := v.(type) {
	case bool:
		return t
	case string:
		return t == "true" || t == "1"
	case float64:
		return t != 0
	}
	return false
}
