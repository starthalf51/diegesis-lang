package syntax

import "testing"

// attr <entity>.<key> = <int | string | id> [src …]: 최상위·선언 블록(배경)과 장면 안(그 시점).
func TestAttrParse(t *testing.T) {
	src := "```sl\nmodule m\nattr hamlet.age = 20\ncharacter watson \"W\"\n  attr watson.wound = shoulder\n" +
		"scene 1\n  attr hamlet.age = 30 src \"5.1.150\"\n  attr watson.name = \"John\" src a12\n" +
		"  attr = x\n  attr hamlet = 3\n  attr tells b: p\n```\n"
	f, diags := ParseFile("t.dieg.md", []byte(src))
	var got []want
	for _, d := range diags {
		got = append(got, want{d.Line, d.Col, d.Rule})
	}
	if wantD := []want{{9, 8, "syntax"}, {10, 8, "syntax"}}; len(got) != 2 || got[0] != wantD[0] || got[1] != wantD[1] {
		t.Fatalf("diags = %v (%v), want %v", got, diags, wantD)
	}
	top := f.Items[0].(*Attribute)
	if top.Entity.Name != "hamlet" || top.Key != "age" || top.Kind != "int" || top.Value != "20" {
		t.Errorf("top attr = %+v", top)
	}
	in := f.Items[1].(*Decl).Attrs[0].(*Attribute)
	if in.Entity.Name != "watson" || in.Key != "wound" || in.Kind != "id" || in.Value != "shoulder" {
		t.Errorf("decl attr = %+v", in)
	}
	body := f.Items[2].(*Scene).Body
	a, b := body[0].(*Attribute), body[1].(*Attribute)
	if a.Kind != "int" || a.Value != "30" || a.Src != "5.1.150" || a.Pos != (Pos{7, 3}) || a.Entity.Pos != (Pos{7, 8}) {
		t.Errorf("scene attr = %+v", a)
	}
	if b.Kind != "str" || b.Value != "John" || b.Key != "name" || b.Src != "a12" {
		t.Errorf("string attr = %+v", b)
	}
	// `attr` 는 예약어가 아니다: attr 라는 인물의 행위는 그대로 행위다
	if act, ok := body[2].(*Act); !ok || act.Actor.Name != "attr" || act.Verb != "tells" {
		t.Errorf("act by attr = %#v", body[2])
	}
}
