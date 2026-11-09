package pg_test

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io/fs"
	"math/rand"
	"os"
	"path/filepath"
	"testing"

	"github.com/starthalf51/diegesis-lang/internal/ir"
	"github.com/starthalf51/diegesis-lang/internal/sema"
	"github.com/starthalf51/diegesis-lang/internal/syntax"
)

// corpusIR compiles corpus/<work> and changes the work ID to a unique one (so it does not collide with a real work loaded by dieg).
func corpusIR(t *testing.T, work string) (*ir.Work, string) {
	t.Helper()
	return corpusIRMod(t, work, nil)
}

// corpusIRMod applies a source variant (mod, for negative controls) to corpusIR.
func corpusIRMod(t *testing.T, work string, mod func(src []byte) []byte) (*ir.Work, string) {
	t.Helper()
	var files []*syntax.File
	dir := filepath.Join("..", "..", "corpus", work)
	err := filepath.WalkDir(dir, func(p string, d fs.DirEntry, err error) error {
		if err != nil || d.IsDir() || !syntax.IsSource(p) {
			return err
		}
		src, err := os.ReadFile(p)
		if err != nil {
			return err
		}
		if mod != nil {
			src = mod(src)
		}
		rel, _ := filepath.Rel(dir, p)
		f, ds := syntax.ParseFile(filepath.ToSlash(rel), src)
		for _, d := range ds {
			if d.Severity == "error" {
				t.Fatalf("%s: %v", p, d)
			}
		}
		files = append(files, f)
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	w, ds := sema.Build(files)
	for _, d := range ds {
		if d.Severity == "error" {
			t.Fatalf("%s: %v", work, d)
		}
	}
	b, err := json.Marshal(w)
	if err != nil {
		t.Fatal(err)
	}
	id := fmt.Sprintf("t_%s_%d", work, rand.Int63())
	for _, p := range []string{`"`, `(`, `,`, ` `} { // down to the arguments inside a proposition ID (holds(w/x,w/y), not w/x)
		b = bytes.ReplaceAll(b, []byte(p+work+`/`), []byte(p+id+`/`))
	}
	for _, k := range []string{"id", "work_id", "owner_id"} { // short names such as directive values like "macbeth" are left alone
		b = bytes.ReplaceAll(b, []byte(`"`+k+`":"`+work+`"`), []byte(`"`+k+`":"`+id+`"`))
	}
	var out ir.Work
	if err := json.Unmarshal(b, &out); err != nil {
		t.Fatal(err)
	}
	return &out, id
}
