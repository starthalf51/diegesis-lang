package syntax

import "testing"

// lasts <dur>..<dur>: 장면·사건의 길이(start→end 의 min/max). AST 는 When{Op: "lasts"}.
// 프론트매터 when 과 함께 써도 dup-decl 이 아니다(시각이 아니라 길이다).
func TestLastsParse(t *testing.T) {
	src := "---\nmodule: m\nscene: \"1\"\nwhen: after 0\n---\n```sl\nlasts 0d..12h\nlasts 1d\n```\n"
	f, diags := ParseFile("t.dieg.md", []byte(src))
	if len(diags) != 1 || diags[0].Line != 8 || diags[0].Rule != "syntax" {
		t.Fatalf("diags = %v, want one syntax error on line 8 (lasts needs a range)", diags)
	}
	var got *When
	for _, n := range f.Items[0].(*Scene).Body {
		if w, ok := n.(*When); ok && w.Op == "lasts" {
			got = w
		}
	}
	if got == nil || got.Min.Sec != 0 || got.Max.Sec != 12*3600 || got.Pos != (Pos{7, 1}) || got.Ref != nil {
		t.Fatalf("lasts = %+v", got)
	}
	if _, d := ParseFile("t.dieg", []byte("module m\ncharacter lasts \"L\"\n")); len(d) != 1 || d[0].Rule != "syntax" {
		t.Errorf("lasts as id: diags = %v, want reserved-word syntax error", d)
	}
}
