package main

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"strings"

	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/starthalf51/diegesis-lang/internal/pg"
)

// readerSt is what the reader has been given strictly before a scene, in telling order (scene_told_ord, so
// `discourse after` counts): explained terms, characters who appeared, facts disclosed.
type readerSt struct {
	Terms []map[string]string `json:"terms"` // {surface, gloss}
	Cast  []string            `json:"cast"`
	Facts []rsFact            `json:"facts"`
}

// rsFact is one statement the reader holds. Value is false only when the statement is false in the work and the reader
// was already shown that before --until (it came by `lies`, or the truth was disclosed earlier). A deception whose
// reveal is still ahead is listed with Value true, as the reader holds it.
type rsFact struct {
	Text  string `json:"text"`
	Value bool   `json:"value"`
}

// Text is the plain-text state file that readcheck prepends to the reader prompt ("" when nothing is known yet).
func (s readerSt) Text() string {
	var b strings.Builder
	for _, t := range s.Terms {
		fmt.Fprintf(&b, "용어 %s: %s\n", t["surface"], t["gloss"])
	}
	if len(s.Cast) > 0 {
		fmt.Fprintf(&b, "등장인물: %s\n", strings.Join(s.Cast, ", "))
	}
	for _, f := range s.Facts {
		if f.Value {
			fmt.Fprintf(&b, "알려진 사실: %s\n", f.Text)
		} else {
			fmt.Fprintf(&b, "알려진 사실: (사실 아님) %s\n", f.Text)
		}
	}
	return b.String()
}

func readerState(ctx context.Context, pool *pgxpool.Pool, work, until string) (readerSt, error) {
	s := readerSt{Terms: []map[string]string{}, Cast: []string{}, Facts: []rsFact{}}
	var o int
	if err := pool.QueryRow(ctx, `SELECT o.ord FROM scene_told_ord($1) o JOIN scene x ON x.id = o.scene_id
		WHERE x.id = $2 OR lint_short(x.id) = $2`, work, until).Scan(&o); err != nil {
		return s, fmt.Errorf("scene %q is not a shown scene of %s: %w", until, work, err)
	}
	q := func(sql string, f func(a, b string)) error {
		rows, err := pool.Query(ctx, sql, work, o)
		if err != nil {
			return err
		}
		defer rows.Close()
		for rows.Next() {
			var a, b string
			if err := rows.Scan(&a, &b); err != nil {
				return err
			}
			f(a, b)
		}
		return rows.Err()
	}
	if err := q(`SELECT render_name($1, t.term_id, 'en'), coalesce(g.value, '') FROM term_explained_at($1) t
		LEFT JOIN attribute g ON g.id = t.term_id || '#gloss' WHERE t.ord < $2 ORDER BY t.ord, t.term_id`,
		func(a, b string) { s.Terms = append(s.Terms, map[string]string{"surface": a, "gloss": b}) }); err != nil {
		return s, err
	}
	if err := q(`SELECT render_name($1, p.entity_id, 'en'), '' FROM presence p JOIN entity e ON e.id = p.entity_id
		JOIN scene_told_ord($1) o ON o.scene_id = p.scene_id
		WHERE e.kind = 'character' AND o.ord < $2 GROUP BY p.entity_id ORDER BY min(o.ord), p.entity_id`,
		func(a, _ string) { s.Cast = append(s.Cast, a) }); err != nil {
		return s, err
	}
	// reader_knows.from_ord is a scene's discourse_ord; keep the changes whose scene is told before `until`, latest per fact.
	// A held value is marked false only if the reader was shown its falsity before `until`: a `lies` disclosure of it, or a
	// disclosure of the opposite value (reveal, narration). fact.truth is timeless, so it never decides alone (I2).
	err := q(`WITH told AS (SELECT sc.discourse_ord FROM scene sc JOIN scene_told_ord($1) o ON o.scene_id = sc.id WHERE o.ord < $2)
		SELECT CASE WHEN k.value THEN '' ELSE '[아님] ' END || render_fact_text($1, k.fact_id),
		 (NOT coalesce(render_fact_true($1, k.fact_id) <> k.value, false) OR NOT EXISTS (SELECT 1 FROM reader_events($1) e
		   WHERE e.fact_id = k.fact_id AND e.ord IN (SELECT * FROM told) AND (e.value <> k.value OR e.via = 'lies')))::text FROM (
		SELECT DISTINCT ON (k.fact_id) k.fact_id, k.value, k.from_ord FROM reader_knows($1) k
		 WHERE k.from_ord IN (SELECT * FROM told)
		 ORDER BY k.fact_id, k.from_ord DESC) k ORDER BY k.from_ord, k.fact_id`,
		func(a, b string) {
			s.Facts = append(s.Facts, rsFact{Text: a, Value: b == "true"})
		})
	return s, err
}

// cmdReaderState runs `dieg reader-state <work|dir> --until <scene> [--json]`.
func cmdReaderState(args []string) int {
	var target, until string
	asJSON := false
	for i := 0; i < len(args); i++ {
		switch a := args[i]; {
		case a == "--json":
			asJSON = true
		case a == "--until" && i+1 < len(args):
			i++
			until = args[i]
		case target == "" && !strings.HasPrefix(a, "-"):
			target = a
		default:
			return usage()
		}
	}
	if target == "" || until == "" {
		return usage()
	}
	work := target
	if _, err := os.Stat(target); err == nil {
		var err error
		if work, err = load(target, os.Stderr); err != nil {
			fmt.Fprintln(os.Stderr, "dieg:", err)
			return 1
		}
	}
	ctx := context.Background()
	pool, err := pg.Connect(ctx)
	if err != nil {
		fmt.Fprintln(os.Stderr, "dieg:", err)
		return 1
	}
	defer pool.Close()
	s, err := readerState(ctx, pool, work, until)
	if err != nil {
		fmt.Fprintln(os.Stderr, "dieg:", err)
		return 1
	}
	if asJSON {
		b, _ := json.MarshalIndent(s, "", "  ")
		fmt.Printf("%s\n", b)
	} else {
		fmt.Print(s.Text())
	}
	return 0
}
