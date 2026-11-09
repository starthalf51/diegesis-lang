package main

// 일회용 스파이크(H1 수리 패스, H2 추출 어휘). 프로덕션 프롬프트는 건드리지 않는다.
//   go run ./tools/corpus-ab -spike h2|h1 -budget N -out examples/corpus-ab/spike

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"unicode/utf8"

	"github.com/starthalf51/diegesis-lang/internal/render"
)

var h2Pairs = []string{"bible/mark_denial.en-doyle", "bible/mark_denial.ko-gojeon", "bible/mark_denial.ko-hapsho",
	"chibi/45.88.en-doyle", "chibi/45.88.ko-gojeon", "chibi/45.88.ko-haera", "chibi/45.88.ko-hapsho", "odyssey/19.1.ko-gojeon"}

type spikeItem struct {
	Pair                                  string
	Calls                                 int
	PassedBefore, PassedAfter             bool
	BeforeMissing, BeforeHall, BeforeLeak []string
	AfterMissing, AfterHall, AfterLeak    []string
	CharsBefore, CharsAfter               int
	EditChars                             int    `json:",omitempty"`
	Err                                   string `json:",omitempty"`
}

func nws(p string) int { return utf8.RuneCountInString(strings.Join(strings.Fields(p), "")) }

func readP1(pair string) (prose string, prev Run) {
	n := strings.ReplaceAll(pair, "/", "_")
	b, _ := os.ReadFile(filepath.Join("examples/corpus-ab/p1", n+".md"))
	j, _ := os.ReadFile(filepath.Join("examples/corpus-ab/p1", n+".json"))
	_ = json.Unmarshal(j, &prev)
	return strings.TrimSpace(string(b)), prev
}

// actLine 은 렌더 프롬프트(BuildPrompt)의 "## Acts" 한 줄과 같은 꼴.
func actLine(s render.Scene, lang string, a render.Act) string {
	name := func(id string) string {
		if e, ok := s.Entity(id); ok && len(e.Names[lang]) > 0 {
			return e.Names[lang][0]
		}
		return id
	}
	parts := []string{}
	if a.Actor != "" {
		parts = append(parts, name(a.Actor))
	}
	parts = append(parts, a.Verb)
	if a.Action != "" {
		parts = append(parts, "("+a.Action+")")
	}
	var ts []string
	for _, t := range a.Targets {
		ts = append(ts, name(t))
	}
	if len(ts) > 0 {
		parts = append(parts, strings.Join(ts, ", "))
	}
	line := strings.Join(parts, " ")
	if a.Fact != "" {
		f, _ := s.FactByID(a.Fact)
		line += ": " + f.Text
		if !f.True {
			line += " (false — the speaker believes or claims it; the narrator must not assert it)"
		}
	}
	if a.Quote != "" {
		line += fmt.Sprintf(" quote: %q", a.Quote)
	}
	return line
}

// ---- H2: 추출 프롬프트 변형 (장면의 지은이 행위 어휘를 덧붙인다)
func vocabPrompt(s render.Scene, lang, prose string) (string, string) {
	sys, user := render.BuildExtractPrompt(s, lang, prose)
	sys = strings.Replace(sys, "- Judge only from", "- Authored actions: the catalog lists this scene's authored `does` actions (action word, actor, targets). When a sentence re-tells one of these authored actions, report it with that exact action word (in new_events \"action\", with that actor and those targets), whatever words the prose uses for it.\n- Judge only from", 1)
	var b strings.Builder
	b.WriteString("\n## Authored actions (action word: actor -> targets)\n")
	for _, bt := range s.Beats {
		for _, a := range bt.Acts {
			if a.Verb == "does" && a.Action != "" {
				fmt.Fprintf(&b, "- %s: %s -> %s\n", a.Action, a.Actor, strings.Join(a.Targets, ", "))
			}
		}
	}
	return sys, strings.Replace(user, "\n## Prose\n", b.String()+"\n## Prose\n", 1)
}

func extractVocab(ctx context.Context, c render.Completer, s render.Scene, lang, prose string, k int) (render.Report, error) {
	sys, user := vocabPrompt(s, lang, prose)
	var xs []render.Extracted
	for range k {
		res, err := c.Complete(ctx, sys, user)
		if err != nil {
			return render.Report{}, err
		}
		e, err := render.ParseExtracted(res.Text)
		if err != nil {
			return render.Report{ExtractError: err.Error()}, nil
		}
		xs = append(xs, e)
	}
	return render.Diff(s, render.Vote(s, xs)), nil
}

// ---- H1: 수리 패스. 지시문은 렌더 프롬프트 관례대로 영어, 한국어 백엔드는 대사·어투 규칙 꼬리를 그대로 붙인다.
func repairPrompt(s render.Scene, b render.Backend, prose string, missing []string) (string, string) {
	sys, _, _ := b.Prompt(s)
	tail := ""
	if i := strings.Index(sys, "</style_sample>\n"); i >= 0 {
		tail = sys[i+len("</style_sample>\n"):]
	}
	ln := map[string]string{"ko": "Korean", "en": "English"}[b.Lang]
	sys = "You are a careful copy editor. You receive existing prose of a scene, a list of story acts that the prose leaves out, and facts that must stay unstated. Add only these facts with minimal edits: keep everything else as written; do not add new events or facts; keep the style and the backend voice. Output only the full revised prose in " + ln + ". No title, headings, notes, or commentary.\n" + tail
	var u strings.Builder
	u.WriteString("## Existing prose\n" + prose + "\n\n## Acts to add (the prose leaves these out; each must become clear through its actor)\n")
	need := map[string]int{}
	for _, m := range missing {
		need[m]++
	}
	for _, bt := range s.Beats {
		for _, a := range bt.Acts {
			k := a.Actor + "|" + render.VerbClass(a.Verb) + "|" + s.Canon(a.Fact)
			if a.Fact != "" && need[k] > 0 {
				need[k]--
				u.WriteString("- " + actLine(s, b.Lang, a) + "\n")
			}
		}
	}
	if len(s.Withheld) > 0 {
		u.WriteString("\n## Must not state or imply\n")
		for _, id := range s.Withheld {
			f, _ := s.FactByID(id)
			u.WriteString("- " + f.Text + "\n")
		}
	}
	return sys, u.String()
}

// editSize: 단어 LCS 기준으로 추가·삭제된 단어의 글자 수 합.
func editSize(a, b string) int {
	x, y := strings.Fields(a), strings.Fields(b)
	l := make([][]int, len(x)+1)
	for i := range l {
		l[i] = make([]int, len(y)+1)
	}
	for i := len(x) - 1; i >= 0; i-- {
		for j := len(y) - 1; j >= 0; j-- {
			if x[i] == y[j] {
				l[i][j] = l[i+1][j+1] + 1
			} else {
				l[i][j] = max(l[i+1][j], l[i][j+1])
			}
		}
	}
	i, j, n := 0, 0, 0
	for i < len(x) && j < len(y) {
		switch {
		case x[i] == y[j]:
			i++
			j++
		case l[i+1][j] >= l[i][j+1]:
			n += utf8.RuneCountInString(x[i])
			i++
		default:
			n += utf8.RuneCountInString(y[j])
			j++
		}
	}
	for ; i < len(x); i++ {
		n += utf8.RuneCountInString(x[i])
	}
	for ; j < len(y); j++ {
		n += utf8.RuneCountInString(y[j])
	}
	return n
}

func runSpike(ctx context.Context, h string, budget int, out string, scs []scene, bs []render.Backend, only string) {
	_ = os.MkdirAll(out, 0o755)
	lf, _ := os.OpenFile(filepath.Join(out, "calls.jsonl"), os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0o644)
	defer lf.Close()
	pairs := h2Pairs
	if only != "" {
		pairs = strings.Split(only, ",")
	}
	if h == "h1" && only == "" {
		pairs = nil
		for _, sc := range scs {
			for _, b := range bs {
				p := sc.work + "/" + sc.s.ID + "." + b.ID
				if _, prev := readP1(p); !prev.Passed && len(prev.Missing) > 0 {
					pairs = append(pairs, p)
				}
			}
		}
	}
	used := 0
	for _, p := range pairs {
		for _, sc := range scs {
			for _, b := range bs {
				if p != sc.work+"/"+sc.s.ID+"."+b.ID {
					continue
				}
				prose, prev := readP1(p)
				it := spikeItem{Pair: p, PassedBefore: prev.Passed, BeforeMissing: prev.Missing, BeforeHall: prev.Hallucinated, BeforeLeak: prev.Leaked, CharsBefore: nws(prose)}
				if used+4 > budget {
					it.Err = "budget"
					fmt.Fprintln(os.Stderr, p, "skipped: budget")
					saveSpike(out, h, it, "")
					continue
				}
				n := 0
				xm := render.NewMeter(counter{render.NewClaudeCLIWith(sonnet, "low"), &n, &used, budget}, lf).Label(h + "-extract-" + p)
				after, newProse := prose, ""
				if h == "h1" {
					rm := render.NewMeter(counter{render.NewClaudeCLIWith(haiku, "medium"), &n, &used, budget}, lf).Label("h1-repair-" + p)
					sys, user := repairPrompt(sc.s, b, prose, prev.Missing)
					res, err := rm.Complete(ctx, sys, user)
					if err != nil || res.StopReason != "end_turn" {
						it.Err = fmt.Sprint("repair: ", err, " ", res.StopReason)
						it.Calls = n
						saveSpike(out, h, it, "")
						continue
					}
					newProse = strings.TrimSpace(res.Text)
					after = newProse
					it.EditChars = editSize(prose, after)
				}
				it.CharsAfter = nws(after)
				var rep render.Report
				var err error
				if h == "h2" {
					rep, err = extractVocab(ctx, xm, sc.s, b.Lang, after, 3)
				} else {
					var e render.Extracted
					e, _, err = render.ExtractK(ctx, xm, sc.s, b.Lang, after, 3)
					rep = render.Diff(sc.s, e)
				}
				if err != nil {
					it.Err = err.Error()
				}
				it.AfterMissing, it.AfterHall, it.AfterLeak = rep.Missing, rep.Hallucinated, rep.Leaked
				it.PassedAfter = err == nil && render.Passed(render.Render{Report: rep})
				it.Calls = n
				fmt.Fprintf(os.Stderr, "%-30s %s before=%v after=%v calls=%d used=%d %s\n", p, h, it.PassedBefore, it.PassedAfter, n, used, it.Err)
				saveSpike(out, h, it, newProse)
			}
		}
	}
	fmt.Fprintln(os.Stderr, h, "total calls", used)
}

func saveSpike(dir, h string, it spikeItem, prose string) {
	b, _ := json.MarshalIndent(it, "", "  ")
	n := h + "_" + strings.ReplaceAll(it.Pair, "/", "_")
	_ = os.WriteFile(filepath.Join(dir, n+".json"), append(b, '\n'), 0o644)
	if prose != "" {
		_ = os.WriteFile(filepath.Join(dir, n+".md"), []byte(prose+"\n"), 0o644)
	}
}
