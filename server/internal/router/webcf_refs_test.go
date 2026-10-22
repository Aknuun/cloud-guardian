package router

import (
	"testing"

	"github.com/aknuun/cg-server/internal/cf"
)

func TestCFCnameResolver(t *testing.T) {
	records := []cf.Record{
		{Name: "example.com", Type: "A", Content: "1.2.3.4"},
		{Name: "www.example.com", Type: "CNAME", Content: "example.com"},
		{Name: "cdn.example.com", Type: "CNAME", Content: "www.example.com"},
		{Name: "loop1.example.com", Type: "CNAME", Content: "loop2.example.com"},
		{Name: "loop2.example.com", Type: "CNAME", Content: "loop1.example.com"},
		{Name: "external.example.com", Type: "CNAME", Content: "outside.example.net"},
	}
	targets := map[string]bool{"example.com": true}
	byName := map[string][]cf.Record{}
	for _, rec := range records {
		byName[cfNormHost(rec.Name)] = append(byName[cfNormHost(rec.Name)], rec)
	}
	reaches := cfCnameResolver(targets, byName)
	for _, name := range []string{"example.com", "www.example.com", "cdn.example.com"} {
		if !reaches(name) {
			t.Errorf("expected %s to reach target", name)
		}
	}
	for _, name := range []string{"loop1.example.com", "loop2.example.com", "external.example.com", "missing.example.com"} {
		if reaches(name) {
			t.Errorf("expected %s not to reach target", name)
		}
	}
}
