// readcheck is the model-reader gate: a first-time-reader test plus a pairwise comparison against a
// reference text, with fixed thresholds and a call budget.
//
//	go run ./tools/readcheck [--ref corpus-eval/readcheck/ref-dongbaek.txt] [--state f] [--model opus] [--budget N] [--cold-only] ep.md...
//
// An episode with a sibling <name>.state.txt (from `dieg reader-state`) is read by a reader who knows that state.
//
// Per episode: error <= maxError (2), term == 0, repeat == 0, dump <= 1, flat <= 1, and the episode wins >= 1 of 2 order-swapped pairs against the
// reference. open, vocab and artifact confusions are recorded only; `continue` is recorded only. READCHECK_CLAUDE
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
	"path/filepath"
	"regexp"
	"strings"
	"time"
)

// maxError is fixed: user decision 2026-11-13 (value 2 now applies to `error`; `open` is recorded only). Do not change.
const maxError = 2

const defaultRef = "corpus-eval/readcheck/ref-dongbaek.txt"

// Fixed thresholds (plan 16): repeat must be 0; dump and flat at most 1 each.
const maxDump, maxFlat = 1, 1

const coldHead = `다음 글은 연재물의 한 회차입니다.`

const coldFresh = ` 당신은 이 작품에 대해 아무것도 모르는 일반 독자입니다. 사전 지식이나 추측으로 메우지 말고, 글에 쓰인 것만으로 답하세요.
`

const stateIntro = "당신은 앞 회차를 읽었습니다. 다음은 지금까지 당신이 알게 된 것입니다:\n"

const stateTail = "이 밖의 사전 지식이나 추측으로 메우지 말고, 앞 회차에서 알게 된 것과 이 글에 쓰인 것만으로 답하세요.\n"

const coldBody = `1) 무슨 일이 일어났는지 3~5문장으로 요약하세요.
2) 읽다가 걸린 지점을 모두 나열하고, 각각 아래 여덟 중 하나로 분류하세요. 없으면 빈 배열.
   - error: 진짜 결함. 서술끼리의 모순(수·장소·시간·누가 무엇을 가졌는지), 시점 위반(서술자가 알 수 없는 것을 말함), 이미 떠난 장소에 인물이 있거나 장면 연속성이 끊김, 누가 말하는지 불분명함, 인용 안에 서술이 끼어듦. 예: 한 문단에서는 문이 잠겼다고 하고 바로 뒤에서는 아무 일 없이 열고 들어간다.
   - open: 아직 답이 나오지 않은 의문, 또는 뒤에서 풀릴 수 있는 일시적 모호함(궁금증을 끄는 장치, 작가의 곁말, "아직 설명되지 않음"). 예: 인물이 "그 약속"을 언급하지만 무슨 약속인지 아직 안 나온다.
   - term: 작품 고유의 설정·제도·기술·관습을 가리키는 말인데, 사건을 따라가려면 뜻을 알아야 하는데도 설명 없이 쓰임. 예: 서술이 "배급표 등급이 낮아 문이 열리지 않았다"고 하는데 등급 제도가 어디에도 설명되지 않는다. 인물·가게·집·장소·배 등의 이름은 문맥상 이름임이 분명하면 term 이 아니다(궁금하면 open 으로 쓰거나 적지 않는다). 예: 미닫이 위에 붙은 "백화당"이라는 옥호는 term 이 아니다.
   - repeat: 앞 회차에서 이미 알게 된 것(위 목록)을 이 글이 다시 설명함. 예: 이미 알려진 나루터의 규칙을 서술자가 처음인 듯 다시 풀어 줌. 앞 회차가 없으면 쓰지 않는다.
   - dump: 정의문, 설명 덩어리, 대놓고 정보를 말하는 대사. 예: 서술이 "X란 Y를 뜻하는 제도다"로 시작함, 인물들이 서로 이미 아는 사정을 독자에게 들려주려고 길게 말함.
   - flat: 중요한 사건 뒤에 인물의 반응(몸짓·말·생각)이 없어 밋밋함. 예: 가족의 부고를 들었는데 다음 문장에서 바로 저녁 메뉴 이야기로 넘어감.
   - vocab: 고어, 방언, 일반 어휘
   - artifact: 원문 추출 흔적, 편집 기호
3) 다음 회를 읽고 싶은지 true/false 로 답하세요.
설명 없이 JSON 객체 하나만 출력하세요(코드 펜스 없이):
{"summary": "...", "confusions": [{"kind": "error|open|term|repeat|dump|flat|vocab|artifact", "text": "..."}], "continue": true}
---
`

// coldPromptFor is the first-time-reader prompt, or (state != "") the cumulative reader's prompt: the same questions after
// the list of what the reader learned in earlier episodes.
func coldPromptFor(state string) string {
	if strings.TrimSpace(state) == "" {
		return coldHead + coldFresh + coldBody
	}
	return coldHead + " " + stateIntro + strings.TrimRight(state, "\n") + "\n" + stateTail + coldBody
}

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

func (g *gate) episode(path, ref, state string, coldOnly bool) (string, bool) {
	g.raw = nil
	b, err := os.ReadFile(path)
	if err != nil {
		return fmt.Sprintf("%s ERROR %v", path, err), false
	}
	ep := string(b)
	var c coldRead
	ok := g.try("cold", coldPromptFor(state)+ep, func(s string) bool { var k bool; c, k = parseCold(s); return k })
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
	n := map[string]int{}
	for _, x := range c.confusions() {
		n[x.Kind]++ // kinds already normalised by valid()
	}
	cont := map[bool]string{true: "y", false: "n"}[c.Continue]
	pass := n["error"] <= maxError && n["term"] == 0 && n["repeat"] == 0 && n["dump"] <= maxDump && n["flat"] <= maxFlat && (coldOnly || wins >= 1)
	v := map[bool]string{true: "PASS", false: "FAIL"}[pass]
	return fmt.Sprintf("%s error=%d open=%d term=%d repeat=%d dump=%d flat=%d pair=%s continue=%s → %s", path,
		n["error"], n["open"], n["term"], n["repeat"], n["dump"], n["flat"], pairs, cont, v), pass
}

func run(args []string, out io.Writer) int {
	fs := flag.NewFlagSet("readcheck", flag.ContinueOnError)
	refPath := fs.String("ref", "", "reference text")
	model := fs.String("model", "opus", "claude model")
	budget := fs.Int("budget", 0, "max model calls (default files x 3)")
	coldOnly := fs.Bool("cold-only", false, "first-time-reader test only (calibration)")
	statePath := fs.String("state", "", "reader-state file for every episode (default: <episode>.state.txt beside each episode, if present)")
	if fs.Parse(args) != nil || fs.NArg() == 0 {
		fmt.Fprintln(out, "usage: readcheck --ref ref.txt [--model opus] [--budget N] [--cold-only] episode.md...")
		return 2
	}
	if *refPath == "" {
		*refPath = defaultRef
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
		line, pass := g.episode(f, string(rb), readState(*statePath, f), *coldOnly)
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
		case "error", "open", "term", "repeat", "dump", "flat", "vocab", "artifact":
		default:
			return false
		}
	}
	return true
}

// readState is the cumulative reader state for an episode: the --state file if given, else <episode without extension>.state.txt.
func readState(explicit, ep string) string {
	p := explicit
	if p == "" {
		p = strings.TrimSuffix(ep, filepath.Ext(ep)) + ".state.txt"
	}
	b, _ := os.ReadFile(p) // absent = first-time reader
	return string(b)
}
