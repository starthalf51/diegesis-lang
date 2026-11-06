package pg_test

import (
	"context"
	"fmt"
	"slices"
	"strings"
	"testing"

	"github.com/jackc/pgx/v5/pgxpool"
)

// readerRows 는 reader_knows(작품, 사실) 를 "from_ord=value" 로 모은다.
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

// Review Focus 2: withhold 동안 독자는 모르고, until 의 서술 위치부터 안다(막힌 공개는 그 위치로 미뤄진다).
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

// Review Focus 2: 방백은 독자만 받는다. 독자는 그 장면의 위치에서 알고, 무대 위 인물에게는 믿음이 생기지 않는다.
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

// 독자는 들은 대로 믿고(거짓말도), 나중에 반대 값이 공개되면 그 위치에서 갱신된다.
// 보이지 않는 사건(event)의 행위는 공개하지 않고, `reveal P` 는 그 장면 위치에서 공개한다.
// 같은 장면 안에서는 마지막 행위가 이긴다(장면 단위).
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

// 수정 1회차: 독자는 이미 거짓인 줄 아는 기만을 믿지 않는다.
//   맥베스형: 방백으로 p 를 안 뒤의 거짓말 np 는 독자 믿음을 바꾸지 않는다.
//   오셀로형: 거짓말한 사람이 생각으로 ¬q 를 먼저 알렸으면, 거짓말도, 그 거짓말에만 속은 사람의 말도 믿지 않는다.
//   대조: 앞선 공개가 없으면 독자는 들은 대로 믿는다.
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
	// 위조 문서(쓴 사람이 거짓인 줄 알고 쓴 문서 — 007 의 추론된 기만)를 읽는 것도 기만 공개다
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
