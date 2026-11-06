// Package accept 는 코퍼스 합격 판정기다. expect.yaml(원작·학계 기준의 기대값)과 실제 결과
// (컴파일 오류 수, 진단, 이름 붙은 질의, 변이 탐지)를 비교한다. 판정 규칙 자체는 PG 가 낸 결과를
// 대조할 뿐이다(ADR-010: 믿음·시간·린트 판정은 PG 에 있다).
package accept

import (
	"bytes"
	"fmt"
	"slices"
	"strings"

	"gopkg.in/yaml.v3"

	"diegesis/internal/mutate"
	"diegesis/internal/pg"
)

// Expect 는 expect.yaml.
type Expect struct {
	Work                        string  `yaml:"work"`
	CompileErrors               int     `yaml:"compile_errors"`
	Expected                    []Want  `yaml:"expected"`
	Queries                     []Query `yaml:"queries"`
	MutationMinRate             float64 `yaml:"mutation_min_rate"`
	MutationMinRateWoMoveBefore float64 `yaml:"mutation_min_rate_wo_move_before"`
	RenderScene                 string  `yaml:"render_scene"` // dieg render-check 가 렌더할 장면(짧은 ID)
}

// Want 는 반드시 나와야 하는 진단 하나. Target 은 작품 ID 를 뺀 대상 ID(`macbeth.act1/1.3#a5`).
// Severity 기본값은 warn. Why 는 원작·학계 근거(보고서에 옮긴다).
type Want struct {
	Rule     string `yaml:"rule"`
	Target   string `yaml:"target"`
	Severity string `yaml:"severity"`
	Waived   bool   `yaml:"waived"`
	Why      string `yaml:"why"`
}

// Query 는 이름 붙은 SQL($1 = 작품 ID)과 기대 행(순서까지 같아야 한다).
type Query struct {
	Name string     `yaml:"name"`
	SQL  string     `yaml:"sql"`
	Rows [][]string `yaml:"rows"`
	Why  string     `yaml:"why"`
}

// Parse 는 모르는 키를 오류로 본다(오타가 조용히 기대값을 지우지 않게).
func Parse(b []byte) (*Expect, error) {
	dec := yaml.NewDecoder(bytes.NewReader(b))
	dec.KnownFields(true)
	var e Expect
	if err := dec.Decode(&e); err != nil {
		return nil, fmt.Errorf("expect.yaml: %w", err)
	}
	if e.Work == "" {
		return nil, fmt.Errorf("expect.yaml: work is required")
	}
	if e.MutationMinRateWoMoveBefore == 0 {
		e.MutationMinRateWoMoveBefore = 0.7 // ADR-014
	}
	for i := range e.Expected {
		if e.Expected[i].Severity == "" {
			e.Expected[i].Severity = "warn"
		}
	}
	return &e, nil
}

// Outcome 은 변이 하나의 결과.
type Outcome string

const (
	Detected Outcome = "detected" // 기대 규칙 중 하나가 변이 지점(Site)에서 새 warn 을 냈다
	// DetectedByQuery: 지점 안의 기대 warn 은 없지만 expect.yaml 질의 결과가 원본과 달라졌다(R1) — 탐지로 센다
	DetectedByQuery Outcome = "detected-by-query"
	Caught          Outcome = "caught"     // 컴파일러(구문·의미 분석)가 변이본을 거부했다 — 탐지로 센다. IR 변이에서는 나오지 않는다
	Equivalent      Outcome = "equivalent" // 탐지되지 않았고 파생 행이 원본과 같다 — 비율에서 뺀다
	Survived        Outcome = "survived"
	// NotApplicable: PG 로 확정해 보니 결함이 생기지 않는 변이(받은 정보를 뒤에서 쓰지 않음, 출처 없음).
	// 돌리지 않고 목록에만 남긴다. 비율에서 뺀다.
	NotApplicable Outcome = "n/a"
)

// Mutant 는 변이 하나와 결과. Rules = 변이본에서 새로 나온 억제되지 않은 warn 의 규칙들.
type Mutant struct {
	Mutation mutate.Mutation
	Outcome  Outcome
	Rules    []string
	Queries  []string
	Note     string
}

// Input 은 실제 결과. Diags 에는 린트 진단과 컴파일 경고를 함께 넣는다.
// Rows 는 질의 이름 → 행(실행하지 못한 질의는 없다). Mutants nil = 변이를 돌리지 않았다.
type Input struct {
	CompileErrors int
	Diags         []pg.Diagnostic
	Rows          map[string][][]string
	QueryErrors   map[string]string
	Mutants       []Mutant
}

type Result struct {
	Pass                                 bool
	Failures                             []string
	Missing                              []Want
	FalsePositives                       []pg.Diagnostic
	Undeclared                           []pg.Diagnostic // 억제됐지만 expect.yaml 에 없는 warn
	BadQueries                           []string
	Rate                                 float64
	Counted                              int // 비율의 분모(동등 변이 제외)
	Hit                                  int // 분자(detected + caught)
	RateWoMoveBefore                     float64
	CountedWoMoveBefore, HitWoMoveBefore int
}

// Judge: 컴파일 오류 수 일치 ∧ expected 전부 있음 ∧ expected 밖의 warn = 0(억제되지 않은 것은 오탐,
// 억제된 것은 선언되지 않은 억제)
// ∧ 질의 결과 일치 ∧ 변이 탐지율 ≥ 기준.
func Judge(e *Expect, in Input) Result {
	var r Result
	fail := func(f string, a ...any) { r.Failures = append(r.Failures, fmt.Sprintf(f, a...)) }
	if in.CompileErrors != e.CompileErrors {
		fail("compile errors: %d, want %d", in.CompileErrors, e.CompileErrors)
	}
	matched := make([]bool, len(in.Diags))
	for _, w := range e.Expected {
		found := false
		for i, d := range in.Diags {
			if d.Rule == w.Rule && short(e.Work, d.TargetID) == w.Target && d.Severity == w.Severity && d.Waived == w.Waived {
				found, matched[i] = true, true
			}
		}
		if !found {
			r.Missing = append(r.Missing, w)
			fail("missing expected %s %s (%s, waived=%v)", w.Rule, w.Target, w.Severity, w.Waived)
		}
	}
	for i, d := range in.Diags {
		switch {
		case matched[i] || d.Severity != "warn":
		case !d.Waived:
			r.FalsePositives = append(r.FalsePositives, d)
			fail("false positive: %s", d)
		default: // 모든 억제는 기대값으로 선언돼야 한다(Review Focus 1)
			r.Undeclared = append(r.Undeclared, d)
			fail("waived warn not declared in expect.yaml: %s", d)
		}
	}
	for _, q := range e.Queries {
		got, ok := in.Rows[q.Name]
		if !ok || !rowsEqual(got, q.Rows) {
			r.BadQueries = append(r.BadQueries, q.Name)
			fail("query %s: got %v, want %v %s", q.Name, got, q.Rows, in.QueryErrors[q.Name])
		}
	}
	for _, m := range in.Mutants {
		hit, counted := 0, 0
		switch m.Outcome {
		case Detected, DetectedByQuery, Caught:
			hit, counted = 1, 1
		case Survived:
			counted = 1
		}
		r.Hit, r.Counted = r.Hit+hit, r.Counted+counted
		if m.Mutation.Op != "move-before" {
			r.HitWoMoveBefore, r.CountedWoMoveBefore = r.HitWoMoveBefore+hit, r.CountedWoMoveBefore+counted
		}
	}
	if r.Counted > 0 {
		r.Rate = float64(r.Hit) / float64(r.Counted)
	}
	if r.CountedWoMoveBefore > 0 {
		r.RateWoMoveBefore = float64(r.HitWoMoveBefore) / float64(r.CountedWoMoveBefore)
	}
	if e.MutationMinRate > 0 && (r.Counted == 0 || r.Rate < e.MutationMinRate) {
		fail("mutation detection %d/%d = %.2f, want ≥ %.2f", r.Hit, r.Counted, r.Rate, e.MutationMinRate)
	}
	// move-before 는 거의 언제나 잡힌다(출처 앞으로 옮기면 모른다). 나머지 연산자만으로도 기준을 넘어야 한다(ADR-014)
	if e.MutationMinRate > 0 && (r.CountedWoMoveBefore == 0 || r.RateWoMoveBefore < e.MutationMinRateWoMoveBefore) {
		fail("mutation detection without move-before %d/%d = %.2f, want ≥ %.2f", r.HitWoMoveBefore, r.CountedWoMoveBefore,
			r.RateWoMoveBefore, e.MutationMinRateWoMoveBefore)
	}
	r.Pass = len(r.Failures) == 0
	return r
}

func short(work, id string) string { return strings.TrimPrefix(id, work+"/") }

func rowsEqual(a, b [][]string) bool {
	return slices.EqualFunc(a, b, func(x, y []string) bool { return slices.Equal(x, y) })
}

// Report 는 out/accept/<work>.md 의 본문.
func Report(e *Expect, in Input, r Result) string {
	var b strings.Builder
	p := func(f string, a ...any) { fmt.Fprintf(&b, f, a...) }
	verdict := "FAIL"
	if r.Pass {
		verdict = "PASS"
	}
	p("# 합격 판정 — %s: **%s**\n\n", e.Work, verdict)
	p("| 항목 | 결과 | 기준 |\n|---|---|---|\n")
	p("| 컴파일 오류 | %d | %d |\n", in.CompileErrors, e.CompileErrors)
	p("| 기대 진단 | %d/%d | 전부 |\n", len(e.Expected)-len(r.Missing), len(e.Expected))
	p("| 오탐(기대 밖 억제되지 않은 warn) | %d | 0 |\n", len(r.FalsePositives))
	p("| 선언되지 않은 억제(기대 밖 억제된 warn) | %d | 0 |\n", len(r.Undeclared))
	p("| 질의 | %d/%d | 전부 |\n", len(e.Queries)-len(r.BadQueries), len(e.Queries))
	p("| 변이 탐지율 | %d/%d = %.2f | ≥ %.2f |\n", r.Hit, r.Counted, r.Rate, e.MutationMinRate)
	p("| 변이 탐지율(move-before 제외) | %d/%d = %.2f | ≥ %.2f |\n\n", r.HitWoMoveBefore, r.CountedWoMoveBefore,
		r.RateWoMoveBefore, e.MutationMinRateWoMoveBefore)
	if len(r.Failures) > 0 {
		p("## 불합격 사유\n\n")
		for _, f := range r.Failures {
			p("- %s\n", f)
		}
		p("\n")
	}
	p("## 기대 진단\n\n| 규칙 | 대상 | 등급 | 억제 | 결과 | 근거 |\n|---|---|---|---|---|---|\n")
	for _, w := range e.Expected {
		st := "있음"
		if slices.Contains(r.Missing, w) {
			st = "**없음**"
		}
		p("| %s | `%s` | %s | %v | %s | %s |\n", w.Rule, w.Target, w.Severity, w.Waived, st, cell(w.Why))
	}
	p("\n## 전체 진단\n\n")
	if len(in.Diags) == 0 {
		p("(없음)\n")
	}
	for _, d := range in.Diags {
		p("- `%s` — 대상 `%s`\n", d, short(e.Work, d.TargetID))
	}
	p("\n## 질의\n\n")
	for _, q := range e.Queries {
		st := "일치"
		if slices.Contains(r.BadQueries, q.Name) {
			st = "**불일치**"
		}
		p("### %s — %s\n\n%s\n\n```sql\n%s\n```\n\n기대 `%v` · 실제 `%v` %s\n\n", q.Name, st, q.Why, strings.TrimSpace(q.SQL),
			q.Rows, in.Rows[q.Name], in.QueryErrors[q.Name])
	}
	p("## 변이\n\n")
	if in.Mutants == nil {
		p("(돌리지 않았다)\n")
		return b.String()
	}
	mutantTables(&b, e.Work, in.Mutants)
	return b.String()
}

// mutantTables 는 연산자별 집계(생성/탐지/동등/생존)와 변이별 표.
func mutantTables(b *strings.Builder, work string, ms []Mutant) {
	type tally struct{ gen, det, query, caught, eq, surv, na int }
	var ops []string
	by := map[string]*tally{}
	for _, m := range ms {
		t := by[m.Mutation.Op]
		if t == nil {
			t = &tally{}
			by[m.Mutation.Op] = t
			ops = append(ops, m.Mutation.Op)
		}
		t.gen++
		switch m.Outcome {
		case Detected:
			t.det++
		case DetectedByQuery:
			t.query++
		case Caught:
			t.caught++
		case Equivalent:
			t.eq++
		case NotApplicable:
			t.na++
		default:
			t.surv++
		}
	}
	fmt.Fprintf(b, "| 연산자 | 생성 | 해당 없음(제외) | 탐지(규칙, 지점 안) | 탐지(질의) | 탐지(컴파일러) | 동등(제외) | 생존 |\n|---|---|---|---|---|---|---|---|\n")
	var all, noMove tally
	for _, op := range ops {
		t := by[op]
		fmt.Fprintf(b, "| %s | %d | %d | %d | %d | %d | %d | %d |\n", op, t.gen, t.na, t.det, t.query, t.caught, t.eq, t.surv)
		for _, s := range []*tally{&all, &noMove} {
			if s == &noMove && op == "move-before" {
				continue
			}
			s.det, s.caught, s.surv = s.det+t.det+t.query, s.caught+t.caught, s.surv+t.surv
		}
	}
	rate := func(t tally) string {
		n := t.det + t.caught + t.surv
		if n == 0 {
			return "-"
		}
		return fmt.Sprintf("%d/%d = %.2f", t.det+t.caught, n, float64(t.det+t.caught)/float64(n))
	}
	fmt.Fprintf(b, "\n탐지율: 전체 %s · move-before 제외 %s\n", rate(all), rate(noMove))
	fmt.Fprintf(b, "\n| # | 연산자 | 대상 | 기대 규칙 | 결과 | 새 warn 규칙 | 비고 |\n|---|---|---|---|---|---|---|\n")
	for i, m := range ms {
		fmt.Fprintf(b, "| m%d | %s | `%s` | %s | %s | %s | %s |\n", i+1, m.Mutation.Op, cell(shortAll(work, m.Mutation.Target)),
			strings.Join(m.Mutation.Expect, ", "), m.Outcome, strings.Join(m.Rules, ", "), cell(m.Note))
	}
}

func shortAll(work, s string) string { return strings.ReplaceAll(s, work+"/", "") }

func cell(s string) string { return strings.NewReplacer("|", "\\|", "\n", " ").Replace(s) }
