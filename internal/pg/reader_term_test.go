package pg_test

import (
	"context"
	"slices"
	"strings"
	"testing"
)

const terms = `term jokbo "족보" "족보 창" = "모든 시민의 신원과 배급을 묶은 국가 기록"
term chip "칩" = "손목에 박힌 신분 칩"
fact fj "족보가 불탔다" true
`

// reader-unknown-term: a term surface reaches the reader (shown scene text) before any explain at or before that narration position.
func TestReaderUnknownTerm(t *testing.T) {
	pool := open(t)
	scenes := func(s1extra, s3extra string) string {
		return terms + `scene 1
  enter a, b
` + s1extra + `  a tells b: fj
scene 2
  enter a, b
  a says "그는 칩거에 들어갔다"
scene 3
  enter a
` + s3extra
	}
	explain := "  render\n    explain jokbo\n"
	for _, tc := range []struct {
		name, s1, s3 string
		want         []string
	}{
		{"unexplained", "", "", []string{"t.s/1#a1/error"}},
		{"explained in the same scene", explain, "", nil},
		{"explained only later", "", explain, []string{"t.s/1#a1/error"}},
		{"discourse after moves the explaining scene first", "  discourse after 3\n", explain, nil},
		{"focus text counts; longest surface", "", "  render\n    focus \"족보 창을 본다\"\n", []string{"t.s/1#a1/error", "t.s/3/error"}},
	} {
		id := compileLoad(t, pool, scenes(tc.s1, tc.s3))
		ds := lint(t, pool, id)
		if got := of(ds, "reader-unknown-term", id); !slices.Equal(got, tc.want) {
			t.Errorf("%s: got %v, want %v", tc.name, got, tc.want)
		}
		for _, d := range ds {
			surface := map[string]string{"act": "족보", "directive": "족보 창"}[d.TargetKind]
			if d.Rule == "reader-unknown-term" && (d.Message != `term "`+surface+`" reaches the reader before it is explained` ||
				!strings.Contains(string(d.Data), `"surface": "`+surface+`"`)) {
				t.Errorf("%s: %s %s", tc.name, d.Message, d.Data)
			}
		}
	}
}

// term_explained_at gives the first narration position of each explained term (Task 3 reads it).
func TestTermExplainedAt(t *testing.T) {
	pool := open(t)
	id := compileLoad(t, pool, terms+"scene 1\n  enter a\nscene 2\n  enter a\n  render\n    explain jokbo, chip\nscene 3\n  render\n    explain jokbo\n")
	rows, err := pool.Query(context.Background(), `SELECT lint_short(term_id) || '@' || ord FROM public.term_explained_at($1) ORDER BY 1`, id)
	if err != nil {
		t.Fatal(err)
	}
	defer rows.Close()
	var got []string
	for rows.Next() {
		var s string
		if err := rows.Scan(&s); err != nil {
			t.Fatal(err)
		}
		got = append(got, s)
	}
	if want := []string{"chip@2", "jokbo@2"}; !slices.Equal(got, want) {
		t.Errorf("got %v, want %v", got, want)
	}
}

// Ruling T1-R2: an error cannot be waived. The matching pragma leaves it unwaived and the CLI form says so.
func TestReaderUnknownTermNotWaivable(t *testing.T) {
	pool := open(t)
	id := compileLoad(t, pool, terms+"scene 1\n  enter a, b\n  #! allow reader-unknown-term: \"house style\"\n  a tells b: fj\n")
	ds := lint(t, pool, id)
	if got := of(ds, "reader-unknown-term", id); !slices.Equal(got, []string{"t.s/1#a1/error"}) {
		t.Fatalf("got %v", got)
	}
	for _, d := range ds {
		if d.Rule == "reader-unknown-term" && !strings.HasSuffix(d.String(), "(error: cannot be waived)") {
			t.Errorf("String() = %s", d)
		}
	}
}
