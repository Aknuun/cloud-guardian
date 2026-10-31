package router

import (
	"path/filepath"
	"testing"

	"github.com/aknuun/cg-server/internal/store"
)

func TestCfAuditKeepsNewestAndCaps(t *testing.T) {
	st, err := store.Open(filepath.Join(t.TempDir(), "t.db"))
	if err != nil {
		t.Fatal(err)
	}
	r := &Router{Store: st}
	for i := 0; i < 5; i++ {
		r.cfAuditAdd(1, 0, "z1", "example.com", "حذف رکورد", "rec")
	}
	var lst []cfAuditEntry
	if !st.GetJSON("cf_audit", &lst) {
		t.Fatal("audit not stored")
	}
	if len(lst) != 5 {
		t.Fatalf("want 5 entries, got %d", len(lst))
	}
	if lst[0].Op != "حذف رکورد" || lst[0].Zid != "z1" {
		t.Fatalf("unexpected head: %+v", lst[0])
	}
}
