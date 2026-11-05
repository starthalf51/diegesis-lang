package pg_test

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io/fs"
	"math/rand"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"story-lang/internal/ir"
	"story-lang/internal/sema"
	"story-lang/internal/syntax"
)

// corpusIR 는 corpus/<work> 를 컴파일하고 작품 ID 를 고유한 것으로 바꾼다(slc 가 적재한 실제 작품과 겹치지 않게).
func corpusIR(t *testing.T, work string) (*ir.Work, string) {
	t.Helper()
	var files []*syntax.File
	dir := filepath.Join("..", "..", "corpus", work)
	err := filepath.WalkDir(dir, func(p string, d fs.DirEntry, err error) error {
		if err != nil || d.IsDir() || !strings.HasSuffix(p, ".sl.md") {
			return err
		}
		src, err := os.ReadFile(p)
		if err != nil {
			return err
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
	for _, p := range []string{`"`, `(`, `,`, ` `} { // 명제 ID 안의 인자(holds(w/x,w/y), not w/x)까지
		b = bytes.ReplaceAll(b, []byte(p+work+`/`), []byte(p+id+`/`))
	}
	for _, k := range []string{"id", "work_id", "owner_id"} { // 지시어 값 같은 짧은 이름 "macbeth" 는 그대로 둔다
		b = bytes.ReplaceAll(b, []byte(`"`+k+`":"`+work+`"`), []byte(`"`+k+`":"`+id+`"`))
	}
	var out ir.Work
	if err := json.Unmarshal(b, &out); err != nil {
		t.Fatal(err)
	}
	return &out, id
}
