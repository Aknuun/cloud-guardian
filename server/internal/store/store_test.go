package store

import (
	"path/filepath"
	"testing"
	"time"
)

func TestRoundTrip(t *testing.T) {
	s, err := Open(filepath.Join(t.TempDir(), "data.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()

	s.Put("k", "v", 0)
	if s.Get("k") != "v" {
		t.Fatal("get")
	}
	if _, ok := s.GetOK("missing"); ok {
		t.Fatal("missing should be absent")
	}
	s.Put("t", "x", 1)
	time.Sleep(1100 * time.Millisecond)
	if _, ok := s.GetOK("t"); ok {
		t.Fatal("ttl should expire")
	}
}

func TestJSONAndHook(t *testing.T) {
	s, err := Open(filepath.Join(t.TempDir(), "data.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()

	var seen string
	s.OnChange = func(k string) { seen = k }
	s.PutJSON("list", []int{1, 2, 3}, 0)
	var got []int
	if !s.GetJSON("list", &got) || len(got) != 3 {
		t.Fatal("json")
	}
	if seen != "list" {
		t.Fatalf("hook got %q", seen)
	}
}
