package syntax

import "testing"

// enacts 는 예약어다: 선언 id 로 쓸 수 없다.
func TestEnactsReserved(t *testing.T) {
	if _, d := ParseFile("t.dieg", []byte("module m\ncharacter enacts \"E\"\n")); len(d) != 1 || d[0].Rule != "syntax" {
		t.Errorf("enacts as id: diags = %v, want reserved-word syntax error", d)
	}
}
