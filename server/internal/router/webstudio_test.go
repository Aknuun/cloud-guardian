package router

import "testing"

func TestHsRenderDefault(t *testing.T) {
	tpl := hsDefaultTemplates()[0]
	host := map[string]any{
		"id": "1", "remark": "🇫🇮 old", "address": []any{"1.2.3.4"}, "priority": float64(0),
	}
	got, warns := hsRender(tpl, host, "direct", "🇫🇮 old")
	want := "🇫🇮 🌍 𝙳𝚒𝚛𝚎𝚌𝚝 | 𝗔𝗶 | 𝑁𝑜𝐴𝑑𝑠"
	if got != want {
		t.Fatalf("render = %q, want %q", got, want)
	}
	if len(warns) != 0 {
		t.Fatalf("unexpected warnings: %v", warns)
	}
}

func TestHsRenderUnknownVar(t *testing.T) {
	tpl := hsDefaultTemplates()[0]
	tpl.Segments = []string{"{nope} {group_name}"}
	got, warns := hsRender(tpl, map[string]any{"id": "1"}, "cdn", "")
	if got == "" || len(warns) != 1 {
		t.Fatalf("got %q warns %v", got, warns)
	}
}

func TestHsAssignPriorities(t *testing.T) {
	rows := []hsRow{
		{ID: "a", Group: "direct", Priority: 10},
		{ID: "b", Group: "tunnel", Priority: 20},
		{ID: "c", Group: "tunnel", Priority: 30},
	}
	all := func(hsRow) bool { return true }
	hsAssignPriorities(rows, all, false, "", "")
	seen := map[int]bool{}
	for _, r := range rows {
		if seen[r.NewPriority] {
			t.Fatalf("duplicate priority %d", r.NewPriority)
		}
		seen[r.NewPriority] = true
	}
	if rows[0].NewPriority != 0 || rows[1].NewPriority != 10 || rows[2].NewPriority != 20 {
		t.Fatalf("full assign = %d,%d,%d", rows[0].NewPriority, rows[1].NewPriority, rows[2].NewPriority)
	}

	// group apply: only tunnel, must reuse the slots those hosts already own
	rows[0].NewPriority = 10
	rows[1].Priority, rows[2].Priority = 30, 20
	rows[1].NewPriority, rows[2].NewPriority = 30, 20
	tunnel := func(r hsRow) bool { return r.Group == "tunnel" }
	hsAssignPriorities(rows, tunnel, true, "", "")
	if rows[0].NewPriority != 10 {
		t.Fatalf("outsider changed: %d", rows[0].NewPriority)
	}
	if rows[1].NewPriority != 20 || rows[2].NewPriority != 30 {
		t.Fatalf("group slots not permuted: %d,%d", rows[1].NewPriority, rows[2].NewPriority)
	}
	if rows[0].NewPriority == rows[1].NewPriority || rows[1].NewPriority == rows[2].NewPriority {
		t.Fatalf("duplicate priorities after group apply")
	}
}
