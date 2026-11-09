package pg_test

import (
	"bytes"
	"context"
	"testing"

	"github.com/starthalf51/diegesis-lang/internal/pg"
)

// Hamlet's age conflict (20 in 1.2 vs 30 in 5.1) relies on the loose 1-year upper bound pirates→landing (act4/offstage):
// with an elapsed-time upper bound it can judge that a 10-year gap cannot be bridged. Removing that bound must make the conflict vanish (negative control).
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
		t.Fatalf("original attribute-conflict = %d, want 1", n)
	}
	drop := func(b []byte) []byte {
		return bytes.ReplaceAll(b, []byte("when after pirates by 0d..1y"), []byte("when after pirates"))
	}
	if n := count(drop); n != 0 {
		t.Errorf("attribute-conflict after removing the bound = %d, want 0", n)
	}
}
