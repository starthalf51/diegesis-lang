package pg_test

import (
	"context"
	"fmt"
	"slices"
	"strings"
	"testing"

	"github.com/jackc/pgx/v5/pgxpool"
)

// readerRows collects reader_knows(work, fact) as "from_ord=value".
func readerRows(t *testing.T, pool *pgxpool.Pool, work, fact string) []string {
	t.Helper()
	rows, err := pool.Query(context.Background(),
		`SELECT from_ord, value FROM public.reader_knows($1, $1 || '/t.s/' || $2) ORDER BY from_ord`, work, fact)
	if err != nil {
		t.Fatal(err)
	}
	defer rows.Close()
	var out []string
	for rows.Next() {
		var o int
		var v bool
		if err := rows.Scan(&o, &v); err != nil {
			t.Fatal(err)
		}
		out = append(out, fmt.Sprintf("%d=%v", o, v))
	}
	return out
}

func readerState(t *testing.T, pool *pgxpool.Pool, work string, ord int) []string {
	t.Helper()
	rows, err := pool.Query(context.Background(),
		`SELECT lint_short(fact_id) || '=' || value FROM public.reader_state($1, $2) ORDER BY 1`, work, ord)
	if err != nil {
		t.Fatal(err)
	}
	defer rows.Close()
	var out []string
	for rows.Next() {
		var s string
		if err := rows.Scan(&s); err != nil {
			t.Fatal(err)
		}
		out = append(out, s)
	}
	return out
}

// Review Focus 2: the reader does not know during withhold, and knows from until's narration position (a blocked disclosure is deferred to that position).
func TestReaderWithhold(t *testing.T) {
	pool := open(t)
	id := compileLoad(t, pool, `scene 1
  enter a, b
  withhold p until 3
  a tells b: p
scene 2
  enter a
scene 3
  enter a
`)
	if got, want := readerRows(t, pool, id, "p"), []string{"3=true"}; !slices.Equal(got, want) {
		t.Errorf("reader_knows(p) = %v, want %v", got, want)
	}
	if got := readerState(t, pool, id, 2); slices.Contains(got, "p=true") {
		t.Errorf("reader_state(2) = %v, p is still withheld", got)
	}
	if got := readerState(t, pool, id, 3); !slices.Contains(got, "p=true") {
		t.Errorf("reader_state(3) = %v, want p=true", got)
	}
}

// Review Focus 2: an aside is received only by the reader. The reader knows at that scene's position, and the characters on stage form no belief.
func TestReaderAsideOnly(t *testing.T) {
	pool := open(t)
	id := compileLoad(t, pool, `scene 1
  enter a, b
  a aside: p
`)
	if got, want := readerRows(t, pool, id, "p"), []string{"1=true"}; !slices.Equal(got, want) {
		t.Errorf("reader_knows(p) = %v, want %v", got, want)
	}
	var n int
	if err := pool.QueryRow(context.Background(), `SELECT count(*) FROM believes WHERE work_id = $1`, id).Scan(&n); err != nil {
		t.Fatal(err)
	}
	if n != 0 {
		t.Errorf("aside made %d character beliefs, want 0", n)
	}
}

// The reader believes what was heard (lies too), and if the opposite value is disclosed later it is updated at that position.
// Acts in unseen events (event) are not disclosed, and `reveal P` discloses at that scene's position.
// Within the same scene the last act wins (scene granularity).
func TestReaderBelievesAsToldAndUpdates(t *testing.T) {
	pool := open(t)
	id := compileLoad(t, pool, `fact q "Q" false
event e "off stage"
  when after start
  a tells b: p
scene 1
  enter a, b, c
  a lies b: q
scene 2
  enter a, b, c
  c tells b: not q
  a tells b: q
  c tells b: not q
scene 3
  enter a
  reveal p
`)
	if got, want := readerRows(t, pool, id, "q"), []string{"1=true", "2=false"}; !slices.Equal(got, want) {
		t.Errorf("reader_knows(q) = %v, want %v", got, want)
	}
	if got, want := readerRows(t, pool, id, "p"), []string{"3=true"}; !slices.Equal(got, want) {
		t.Errorf("reader_knows(p) = %v, want %v (event acts are not shown)", got, want)
	}
	if got, want := readerState(t, pool, id, 1), []string{"q=true"}; !slices.Equal(got, want) {
		t.Errorf("reader_state(1) = %v, want %v", got, want)
	}
}

// Fix round 1: the reader does not believe a deception it already knows is false.
//   Macbeth type: a lie np after learning p by aside does not change the reader's belief.
//   Othello type: if the liar first told ¬q by thought, neither the lie nor the words of someone deceived only by that lie are believed.
//   Control: without an earlier disclosure the reader believes what was heard.
func TestReaderSeesThroughLies(t *testing.T) {
	pool := open(t)
	id := compileLoad(t, pool, `scene 1
  enter a, b
  a aside: p
scene 2
  when after 1
  enter a, b
  a lies b: np
`)
	if got, want := readerRows(t, pool, id, "p"), []string{"1=true"}; !slices.Equal(got, want) {
		t.Errorf("macbeth-style: reader_knows(p) = %v, want %v", got, want)
	}
	othello := `fact q "Q" false
scene 1
  enter a
  a thinks: not q
scene 2
  when after 1
  enter a, b
  a lies b: q
scene 3
  when after 2
  enter b, c
  b tells c: q
`
	id = compileLoad(t, pool, othello)
	if got, want := readerRows(t, pool, id, "q"), []string{"1=false"}; !slices.Equal(got, want) {
		t.Errorf("othello-style: reader_knows(q) = %v, want %v", got, want)
	}
	id = compileLoad(t, pool, strings.Replace(othello, "  a thinks: not q\n", "", 1))
	if got, want := readerRows(t, pool, id, "q"), []string{"2=true"}; !slices.Equal(got, want) {
		t.Errorf("no earlier disclosure: reader_knows(q) = %v, want %v", got, want)
	}
	// Reading a forged document (a document written knowing it was false — 007's inferred deception) is also a deceptive disclosure
	id = compileLoad(t, pool, `fact q "Q" false
artifact forged "F"
  contains q
scene 1
  enter a, b
  a sees: not q
event e "forgery"
  when after 1
  when before 2
  a writes forged
scene 2
  when after 1
  enter b
  b reads forged
`)
	if got, want := readerRows(t, pool, id, "q"), []string{"1=false"}; !slices.Equal(got, want) {
		t.Errorf("forged document: reader_knows(q) = %v, want %v", got, want)
	}
}
