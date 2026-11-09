package pg_test

import (
	"bytes"
	"context"
	"testing"

	"github.com/starthalf51/diegesis-lang/internal/pg"
)

// 햄릿 나이 충돌(1.2 의 20세 vs 5.1 의 30세)은 해적→상륙의 느슨한 상한 1년(act4/offstage)에 기댄다:
// 경과 상한이 있어야 10년 차이를 메울 수 없다고 판정한다. 그 상한을 지우면 충돌이 사라져야 한다(음성 대조).
func TestHamletAgeConflictNeedsLandingBound(t *testing.T) {
	pool := open(t)
	count := func(mod func([]byte) []byte) int {
		w, id := corpusIRMod(t, "hamlet", mod)
		cleanup(t, pool, id)
		if err := pg.Load(context.Background(), pool, w); err != nil {
			t.Fatal(err)
		}
		return len(of(lint(t, pool, id), "attribute-conflict", id))
	}
	if n := count(nil); n != 1 {
		t.Fatalf("원본 attribute-conflict = %d, want 1", n)
	}
	drop := func(b []byte) []byte {
		return bytes.ReplaceAll(b, []byte("when after pirates by 0d..1y"), []byte("when after pirates"))
	}
	if n := count(drop); n != 0 {
		t.Errorf("상한을 지운 뒤 attribute-conflict = %d, want 0", n)
	}
}
