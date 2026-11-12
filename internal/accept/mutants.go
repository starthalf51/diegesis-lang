package accept

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"maps"
	"regexp"
	"slices"
	"strings"

	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/starthalf51/diegesis-lang/internal/ir"
	"github.com/starthalf51/diegesis-lang/internal/mutate"
	"github.com/starthalf51/diegesis-lang/internal/pg"
	"github.com/starthalf51/diegesis-lang/rules"
)

// RunMutants loads each mutant as `<work>~m<i>`, lints it, then deletes it. base is the already loaded and linted original,
// baseDiags are its diagnostics, qs are the queries in expect.yaml. Before running, mutants are confirmed with PG (applicable): only pairs where
// "the received information is used later" with precedes = yes are kept, and move-before fills in the source points. Mutants left with nothing are NotApplicable and
// only stay in the list (excluded from the rate).
// Detection = among the new unsuppressed warns that did not exist in the original, one has an Expect rule and its target is inside the mutation site (Site) (outside the site is only noted in the note).
// Failing that, if the query results differ from the original it is DetectedByQuery (R1). If neither but the derived rows equal the original it is Equivalent,
// otherwise Survived. Load, lint and DB errors are execution failures, not mutant results — an error is returned.
// IR mutations skip compilation, so Caught (the compiler caught it) does not occur here.
func RunMutants(ctx context.Context, db *pgxpool.Pool, base *ir.Work, baseDiags []pg.Diagnostic, qs []Query, seed int64, n int,
	log io.Writer) ([]Mutant, error) {
	h, err := newHarness(ctx, db, base, baseDiags, qs, log)
	if err != nil {
		return nil, err
	}
	work := base.Works[0].ID
	out := []Mutant{}
	for i, m := range mutate.Generate(base, seed, n) {
		res, err := h.run(ctx, fmt.Sprintf("%s~m%d", work, i+1), m)
		if err != nil {
			return nil, fmt.Errorf("m%d %s %s: %w", i+1, m.Op, shortAll(work, m.Target), err)
		}
		fmt.Fprintf(log, "m%d %s %s → %s %v %s\n", i+1, m.Op, shortAll(work, m.Target), res.Outcome, res.Rules, res.Note)
		out = append(out, res)
	}
	return out, nil
}

// harness computes the baseline values on the original side once (the original's warns, the derived-row fingerprint, the query results) and uses them for every mutant.
// qs are only the queries that can be moved to a mutant and run (those whose result is the same on a copy of the original loaded under a different name — queries that use
// work constants such as the graph name are left out). baseRows is the original's result for those queries.
type harness struct {
	db        *pgxpool.Pool
	work      string
	base      *ir.Work
	before    map[string]bool
	basePrint print
	qs        []Query
	baseRows  map[string][][]string
}

func newHarness(ctx context.Context, db *pgxpool.Pool, base *ir.Work, baseDiags []pg.Diagnostic, qs []Query,
	log io.Writer) (*harness, error) {
	h := &harness{db: db, work: base.Works[0].ID, base: base, before: map[string]bool{}}
	for _, d := range baseDiags {
		if finding(d.Severity) && !d.Waived {
			h.before[d.Rule+" "+short(h.work, d.TargetID)] = true
		}
	}
	var err error
	if h.basePrint, err = fingerprint(ctx, db, h.work, base); err != nil || len(qs) == 0 {
		return h, err
	}
	rows, _ := RunQueries(ctx, db, h.work, qs)
	copyID := h.work + "~m0"
	if _, err := loadLint(ctx, db, mutate.Rename(base, copyID)); err != nil {
		return nil, fmt.Errorf("%s: %w", copyID, err)
	}
	got, _ := RunQueries(ctx, db, copyID, qs)
	if err := drop(ctx, db, copyID); err != nil {
		return nil, err
	}
	h.baseRows = map[string][][]string{}
	for _, q := range qs {
		r, ok := rows[q.Name]
		switch {
		case readsDiagnostics.MatchString(q.SQL):
			fmt.Fprintf(log, "query %s: excluded from query detection (reads the diagnostic table — lint detection, site-local only)\n", q.Name)
		case !ok || !rowsEqual(r, got[q.Name]):
			fmt.Fprintf(log, "query %s: excluded from query detection (result differs on the renamed unmutated copy %s — not transportable)\n", q.Name, copyID)
		default:
			h.qs, h.baseRows[q.Name] = append(h.qs, q), r
		}
	}
	return h, nil
}

// readsDiagnostics: a query that reads the diagnostic table looks at lint results again (bypassing the site restriction), so it is not used for query detection.
var readsDiagnostics = regexp.MustCompile(`(?i)\bdiagnostic\b`)

// run loads, lints and judges one mutant by id, then deletes it.
func (h *harness) run(ctx context.Context, id string, m mutate.Mutation) (Mutant, error) {
	res := Mutant{Mutation: m}
	ok, why, err := applicable(ctx, h.db, h.work, h.base, &res.Mutation)
	if err != nil {
		return res, err
	}
	if !ok {
		res.Outcome, res.Note = NotApplicable, why
		return res, nil
	}
	hit, off := false, []string(nil)
	ds, err := loadLint(ctx, h.db, mutate.Rename(mutate.Apply(h.base, res.Mutation), id))
	if err == nil {
		for _, d := range ds {
			if !finding(d.Severity) || d.Waived || h.before[d.Rule+" "+short(id, d.TargetID)] {
				continue
			}
			if !slices.Contains(res.Rules, d.Rule) {
				res.Rules = append(res.Rules, d.Rule)
			}
			if hits(res.Mutation, h.work, id, d) {
				hit = true
			} else if slices.Contains(m.Expect, d.Rule) {
				off = append(off, d.Rule+"@"+short(id, d.TargetID))
			}
		}
		slices.Sort(res.Rules)
		// R1: if the result of a query the author wrote in the source changes, it is detection regardless of site (counted separately)
		rows, _ := RunQueries(ctx, h.db, id, h.qs)
		for _, q := range h.qs {
			if r, ok := rows[q.Name]; ok && !rowsEqual(r, h.baseRows[q.Name]) { // an execution failure is not counted as detection
				res.Queries = append(res.Queries, q.Name)
			}
		}
		var fp print
		fp, err = fingerprint(ctx, h.db, id, h.base)
		switch {
		case err != nil:
		case hit:
			res.Outcome = Detected
		case len(res.Queries) > 0:
			res.Outcome, res.Note = DetectedByQuery, "query: "+strings.Join(res.Queries, ", ")
		case fp.equal(h.basePrint):
			res.Outcome, res.Note = Equivalent, "derived rows (believes, uses, role_span, bounds of shared points, order of edge endpoints) equal the original"
		default:
			res.Outcome, res.Note = Survived, h.basePrint.diff(fp)
		}
		if len(off) > 0 {
			res.Note = strings.TrimSpace(res.Note + " · outside the site (not counted): " + strings.Join(off, ", "))
		}
	}
	if derr := drop(ctx, h.db, id); err == nil {
		err = derr
	}
	return res, err
}

// hits: did the new warn d detect mutant m — an expected rule and the target is inside m.Site. For time-cycle the target is only
// one representative statement of the cycle, so it is enough that one of data.points (the points the cycle passes through, 002/008) is in Site (an edge through the mutated point).
func hits(m mutate.Mutation, work, id string, d pg.Diagnostic) bool {
	if !slices.Contains(m.Expect, d.Rule) {
		return false
	}
	in := func(x string) bool {
		return slices.ContainsFunc(m.Site, func(s string) bool { return short(work, s) == short(id, x) })
	}
	if in(d.TargetID) {
		return true
	}
	var data struct {
		Points []string `json:"points"`
	}
	if d.Rule == "time-cycle" && json.Unmarshal(d.Data, &data) == nil {
		return slices.ContainsFunc(data.Points, in)
	}
	return false
}

// applicable confirms the mutant with PG. If Pairs exist, at least one pair with precedes(received, used) = yes is needed.
// move-before fills Before with the points at which the speaker (and groups) obtained the proposition they use (believes.from_pt, their own decrees).
func applicable(ctx context.Context, db *pgxpool.Pool, work string, base *ir.Work, m *mutate.Mutation) (bool, string, error) {
	if len(m.Pairs) > 0 {
		var a, b []string
		for _, p := range m.Pairs {
			a, b = append(a, p[0]), append(b, p[1])
		}
		// if the user withdrew all the beliefs received (rows from that act) by doubt before using them, that pair cannot make a defect (006).
		rows, err := strRows(ctx, db, `SELECT coalesce(bool_or(precedes($1, a, b) = 'yes' AND NOT coalesce((
		    SELECT bool_and(bl.until_pt IS NOT NULL AND bl.until_pt <> b AND precedes($1, bl.until_pt, b) = 'yes')
		      FROM believes bl JOIN act u ON u.id = b
		     WHERE bl.work_id = $1 AND bl.cause_act = a AND bl.entity_id = u.resolved_actor_id AND bl.value), false)), false)::text
		  FROM unnest($2::text[], $3::text[]) t(a, b)`, work, a, b)
		if err != nil {
			return false, "", err
		}
		if rows[0][0] != "true" {
			return false, "every act that uses the received information is earlier, has undetermined order, or the belief was withdrawn by doubt before use", nil
		}
	}
	if m.Op != "move-before" {
		return true, "", nil
	}
	actor := ""
	for _, a := range base.Acts {
		if a.ID == m.Target {
			actor = a.ActorID
		}
	}
	// Uses is Go's one-step alias resolution. Realign it to the PG canonical form (008: aliases inside negation).
	rows, err := strRows(ctx, db, `
	  WITH c AS (SELECT lint_canon($1, u) AS c FROM unnest($4::text[]) u)
	  SELECT DISTINCT b.from_pt FROM believes b JOIN lint_self_and_groups($1, $2) u ON u.id = b.entity_id
	   WHERE b.work_id = $1 AND b.value AND b.from_pt IS NOT NULL AND b.from_pt <> $3 AND lint_canon($1, b.prop_id) IN (SELECT c FROM c)
	  UNION
	  SELECT x.pt FROM act_content x JOIN lint_self_and_groups($1, $2) u ON u.id = x.actor_id
	   WHERE x.work_id = $1 AND x.verb = 'decrees' AND x.pt <> $3 AND lint_canon($1, x.prop_id) IN (SELECT c FROM c)
	  ORDER BY 1`, work, actor, m.Target, m.Uses)
	if err != nil {
		return false, "", err
	}
	for _, r := range rows {
		m.Before = append(m.Before, r[0])
	}
	if len(m.Before) == 0 {
		return false, "the proposition used has no source point (background knowledge only)", nil
	}
	return true, "", nil
}

func loadLint(ctx context.Context, db *pgxpool.Pool, w *ir.Work) ([]pg.Diagnostic, error) {
	if err := pg.Load(ctx, db, w); err != nil {
		return nil, fmt.Errorf("load: %w", err)
	}
	return pg.Lint(ctx, db, w.Works[0].ID, rules.FS)
}

func drop(ctx context.Context, db *pgxpool.Pool, work string) error {
	if _, err := db.Exec(ctx, `DELETE FROM public.work WHERE id = $1`, work); err != nil {
		return fmt.Errorf("drop %s: %w", work, err)
	}
	_, err := db.Exec(ctx, `SELECT ag_catalog.drop_graph(name, true) FROM ag_catalog.ag_graph WHERE name = public.graph_name($1)`, work)
	if err != nil {
		return fmt.Errorf("drop graph %s: %w", work, err)
	}
	return nil
}

// print is the derived-row fingerprint of a work (strings with the work ID prefix removed). rows = multisets of believes, uses, role_span,
// points = point → bounds, and for each edge of the original IR the order of its two endpoints (`order a b` → precedes(a,b)|precedes(b,a), PG) —
// so a mutant that only changes order also changes the derived result. Only points and pairs present on both sides are compared (the point of a deleted act is not compared).
type print struct {
	rows   []string
	points map[string]string
}

const fingerprintSQL = `
WITH r(x) AS (
  SELECT concat_ws('|', 'believes', entity_id, prop_id, value, from_pt, cause_act, declared, until_pt) FROM believes WHERE work_id = $1
  UNION ALL SELECT concat_ws('|', 'uses', entity_id, prop_id, at_pt, act_id) FROM uses WHERE work_id = $1
  UNION ALL SELECT concat_ws('|', 'role_span', role_id, entity_id, from_pt, until_pt, span, shared) FROM role_span WHERE work_id = $1
  UNION ALL SELECT concat_ws('|', 'point', id, coalesce(earliest::text, '-'), coalesce(latest::text, '-')) FROM point WHERE work_id = $1
  UNION ALL SELECT concat_ws('|', 'point', 'order ' || a || ' ' || b, precedes($1, $1 || '/' || a, $1 || '/' || b),
                            precedes($1, $1 || '/' || b, $1 || '/' || a))
    FROM unnest($2::text[], $3::text[]) e(a, b)
   WHERE EXISTS (SELECT 1 FROM point WHERE id = $1 || '/' || a) AND EXISTS (SELECT 1 FROM point WHERE id = $1 || '/' || b))
SELECT replace(x, $1 || '/', '') FROM r ORDER BY 1`

func fingerprint(ctx context.Context, db *pgxpool.Pool, work string, base *ir.Work) (print, error) {
	var a, b []string
	for _, e := range base.TimeEdges {
		a, b = append(a, short(base.Works[0].ID, e.From)), append(b, short(base.Works[0].ID, e.To))
	}
	rows, err := strRows(ctx, db, fingerprintSQL, work, a, b)
	if err != nil {
		return print{}, err
	}
	p := print{points: map[string]string{}}
	for _, r := range rows {
		if rest, ok := strings.CutPrefix(r[0], "point|"); ok {
			id, bounds, _ := strings.Cut(rest, "|")
			p.points[id] = bounds
			continue
		}
		p.rows = append(p.rows, r[0])
	}
	return p, nil
}

func (p print) equal(q print) bool { return p.diff(q) == "" }

// diff is the first difference as one line ("" if equal).
func (p print) diff(q print) string {
	if !slices.Equal(p.rows, q.rows) {
		for _, r := range p.rows {
			if !slices.Contains(q.rows, r) {
				return "only in original: " + r
			}
		}
		for _, r := range q.rows {
			if !slices.Contains(p.rows, r) {
				return "only in mutant: " + r
			}
		}
		return "row counts differ"
	}
	for _, id := range slices.Sorted(maps.Keys(p.points)) {
		if b, ok := q.points[id]; ok && b != p.points[id] {
			return fmt.Sprintf("point %s: %s → %s", id, p.points[id], b)
		}
	}
	return ""
}
