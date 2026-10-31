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
