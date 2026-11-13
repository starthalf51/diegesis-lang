// readcheck is the model-reader gate: a first-time-reader test plus a pairwise comparison against a
// reference text, with fixed thresholds and a call budget.
//
//	go run ./tools/readcheck --ref corpus-eval/readcheck/ref-unsu.txt [--model opus] [--budget N] [--cold-only] ep.md...
//
// Per episode: error <= maxError (2), term == 0, and the episode wins >= 1 of 2 order-swapped pairs against the
// reference. vocab and artifact confusions are ignored; `continue` is recorded only. READCHECK_CLAUDE
// replaces the `claude` command (tests).
package main

import (
	"bytes"
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"os"
	"os/exec"
	"regexp"
	"strings"
	"time"
)

// maxError is fixed: user decision 2026-11-13 (value 2 now applies to `error`; `open` is recorded only). Do not change.
const maxError = 2

const coldPrompt = `다음은 한국어 소설의 한 회차입니다. 당신은 이 작품에 대해 아무것도 모르는 일반 독자입니다. 사전 지식이나 추측으로 메우지 말고, 글에 쓰인 것만으로 답하세요.
1) 무슨 일이 일어났는지 3~5문장으로 요약하세요.
2) 읽다가 헷갈린 지점을 모두 나열하고, 각각 아래 다섯 중 하나로 분류하세요. 없으면 빈 배열.
   - error: 진짜 결함. 서술끼리의 모순(수·장소·시간·누가 무엇을 가졌는지), 시점 위반(서술자가 알 수 없는 것을 말함), 이미 떠난 장소에 인물이 있거나 장면 연속성이 끊김, 누가 말하는지 불분명함, 인용 안에 서술이 끼어듦. 예: 한 문단에서는 문이 잠겼다고 하고 바로 뒤에서는 아무 일 없이 열고 들어간다.
   - open: 아직 답이 나오지 않은 의문, 또는 뒤에서 풀릴 수 있는 일시적 모호함(궁금증을 끄는 장치, 작가의 곁말, "아직 설명되지 않음"). 예: 인물이 "그 약속"을 언급하지만 무슨 약속인지 아직 안 나온다.
   - term: 작품 고유 용어·설정이 설명 없이 나옴
   - vocab: 고어, 방언, 일반 어휘
   - artifact: 원문 추출 흔적, 편집 기호
3) 다음 회를 읽고 싶은지 true/false 로 답하세요.
설명 없이 JSON 객체 하나만 출력하세요(코드 펜스 없이):
{"summary": "...", "confusions": [{"kind": "error|open|term|vocab|artifact", "text": "..."}], "continue": true}
---
`

const pairIntro = "Below are two excerpts (A and B) from novels, of similar length. You are an ordinary adult reader with no context. Which excerpt would you rather keep reading? Judge only how much each pulls you forward, not literary prestige. Reply on line 1 with exactly 'A' or 'B', then one sentence why.\n\n"

type confusion struct {
	Kind string `json:"kind"`
	Text string `json:"text"`
}
type coldRead struct {
	Summary    *string      `json:"summary"`
	Confusions *[]confusion `json:"confusions"`
	Continue   bool         `json:"continue"`
}

// parseCold takes the first {...} object in s (CLI warnings and ``` fences around it are skipped).
func parseCold(s string) (coldRead, bool) {
	for i := 0; i < len(s); i++ {
		if s[i] != '{' {
			continue
		}
		var c coldRead
		if json.NewDecoder(strings.NewReader(s[i:])).Decode(&c) == nil && c.valid() {
			return c, true
		}
	}
	return coldRead{}, false
}

var pickRe = regexp.MustCompile("^[*'\"` ]*([AB])[*'\"` ]*[.!:]?$")

// parsePick decides on the first non-empty line only (known CLI warning lines dropped); it must be exactly A or B.
func parsePick(s string) (string, bool) {
	for _, l := range strings.Split(s, "\n") {
		l = strings.TrimSpace(l)
		if l == "" || strings.HasPrefix(l, "Warning:") || strings.HasPrefix(l, "Permission allow rule") {
			continue
		}
		if m := pickRe.FindStringSubmatch(l); m != nil {
			return m[1], true
		}
		return "", false
	}
	return "", false
}

type gate struct {
	model        string
	used, budget int
	raw          []map[string]string
}

func (g *gate) ask(prompt string) (string, error) {
	bin := os.Getenv("READCHECK_CLAUDE")
	if bin == "" {
		bin = "claude"
	}
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Minute)
	defer cancel()
	dir, err := os.MkdirTemp("", "readcheck-")
	if err != nil {
		return "", err
	}
	defer os.RemoveAll(dir)
	// Reader isolation: empty cwd, no tools, no MCP, no settings/hooks, no skills, no saved session.
	cmd := exec.CommandContext(ctx, bin, "-p", "--model", g.model, "--tools", "", "--strict-mcp-config",
		"--setting-sources", "", "--disable-slash-commands", "--no-session-persistence")
	cmd.Dir = dir
	cmd.Stdin = strings.NewReader(prompt)
	var out bytes.Buffer
	cmd.Stdout = &out
	cmd.Stderr = io.Discard // warnings live here; the answer is stdout
	g.used++
	err = cmd.Run()
	return out.String(), err
}

// try asks up to twice (one retry on unparsable output), each call counted and bounded by the budget.
func (g *gate) try(kind, prompt string, ok func(string) bool) bool {
	for attempt := 0; attempt < 2 && g.used < g.budget; attempt++ {
		resp, _ := g.ask(prompt)
		g.raw = append(g.raw, map[string]string{"kind": kind, "response": resp})
		if ok(resp) {
			return true
		}
	}
	return false
}

func (g *gate) episode(path, ref string, coldOnly bool) (string, bool) {
	g.raw = nil
	b, err := os.ReadFile(path)
	if err != nil {
		return fmt.Sprintf("%s ERROR %v", path, err), false
	}
	ep := string(b)
	var c coldRead
	ok := g.try("cold", coldPrompt+ep, func(s string) bool { var k bool; c, k = parseCold(s); return k })
	wins, pairs := 0, "-"
	if ok && !coldOnly {
		for round := 0; round < 2 && ok; round++ {
			a, bb := ep, ref
			if round == 1 {
				a, bb = ref, ep
			}
			p := pairIntro + "=== A ===\n" + a + "\n\n=== B ===\n" + bb + "\n"
			want := map[int]string{0: "A", 1: "B"}[round]
			ok = g.try("pair", p, func(s string) bool {
				v, k := parsePick(s)
				if k && v == want {
					wins++ // set only on the accepted (last) call: a retry happens only when !k
				}
				return k
			})
		}
		pairs = fmt.Sprintf("%d/2", wins)
	}
	if rb, err := json.MarshalIndent(g.raw, "", " "); err == nil {
		os.WriteFile(path+".readcheck.json", rb, 0o644)
	}
	if !ok {
		return fmt.Sprintf("%s UNDECIDED (unparsable or budget exhausted)", path), false
	}
	e, o, t := 0, 0, 0
	for _, x := range c.confusions() {
		switch x.Kind { // kinds already normalised by valid()
		case "error":
			e++
		case "open":
			o++
		case "term":
			t++
		}
	}
	cont := map[bool]string{true: "y", false: "n"}[c.Continue]
	pass := e <= maxError && t == 0 && (coldOnly || wins >= 1)
	v := map[bool]string{true: "PASS", false: "FAIL"}[pass]
	return fmt.Sprintf("%s error=%d open=%d term=%d pair=%s continue=%s → %s", path, e, o, t, pairs, cont, v), pass
}

func run(args []string, out io.Writer) int {
	fs := flag.NewFlagSet("readcheck", flag.ContinueOnError)
	refPath := fs.String("ref", "", "reference text")
	model := fs.String("model", "opus", "claude model")
	budget := fs.Int("budget", 0, "max model calls (default files x 3)")
	coldOnly := fs.Bool("cold-only", false, "first-time-reader test only (calibration)")
	if fs.Parse(args) != nil || *refPath == "" || fs.NArg() == 0 {
		fmt.Fprintln(out, "usage: readcheck --ref ref.txt [--model opus] [--budget N] [--cold-only] episode.md...")
		return 2
	}
	rb, err := os.ReadFile(*refPath)
	if err != nil {
		fmt.Fprintln(out, err)
		return 2
	}
	if *budget == 0 {
		*budget = fs.NArg() * 3
	}
	g := &gate{model: *model, budget: *budget}
	need := 3
	if *coldOnly {
		need = 1
	}
	allPass := true
	for _, f := range fs.Args() {
		if g.budget-g.used < need {
			fmt.Fprintf(out, "%s SKIPPED(budget)\n", f)
			allPass = false
			continue
		}
		line, pass := g.episode(f, string(rb), *coldOnly)
		fmt.Fprintln(out, line)
		allPass = allPass && pass
	}
	fmt.Fprintf(out, "calls=%d/%d\n", g.used, g.budget)
	if allPass {
		return 0
	}
	return 1
}

func main() { os.Exit(run(os.Args[1:], os.Stdout)) }

func (c coldRead) confusions() []confusion {
	if c.Confusions == nil {
		return nil
	}
	return *c.Confusions
}

// valid requires a summary and a confusions array whose kinds are all known; kinds are normalised in place.
func (c *coldRead) valid() bool {
	if c.Summary == nil || strings.TrimSpace(*c.Summary) == "" || c.Confusions == nil {
		return false
	}
	cs := *c.Confusions
	for i := range cs {
		cs[i].Kind = strings.ToLower(strings.TrimSpace(cs[i].Kind))
		switch cs[i].Kind {
		case "error", "open", "term", "vocab", "artifact":
		default:
			return false
		}
	}
	return true
}
