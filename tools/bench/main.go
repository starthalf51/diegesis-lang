// bench compares, for one idea, A (the model writes the novel directly: outline, then scene by scene with the
// outline and the previous scene) against B (`dieg write`), on token cost; `judge` runs the blind continuity judge.
//
//	go run ./tools/bench a     -idea f.txt -n 7 -out examples/bench/x
//	go run ./tools/bench b     -idea f.txt -out examples/bench/x [-rounds 3] [-scenes 5] [-no-render]
//	go run ./tools/bench judge -n 7 -out examples/bench/x
//	go run ./tools/bench sum   examples/bench/x ...
//
// -dry swaps every model call (and the B subprocess) for a fake. Run from the repo root.
package main

import (
	"bufio"
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"sort"
	"strings"

	"diegesis/internal/render"
)

// System is the demo's fixed system prompt (examples/idea-to-novel/prompts/call.sh), used by A and the judge.
const System = "You are a careful Korean novelist and story engineer. Follow the user's instructions exactly. Output only what is asked."

// requirements is examples/idea-to-novel/prompts/common-requirements.txt with the scene count as a parameter.
const requirements = `## 이야기 요구 사항 (두 방식 공통)
- 아래 아이디어 하나로 한국어 중편 소설을 만든다. 장면 %d개, 등장인물 6–8명.
- 긴 이야기의 일관성이 핵심이다: 비밀, 일부 인물만 믿는 거짓말, 누가 언제 무엇을 아는가,
  시각·알리바이, 앞에서 심어 둔 물건이 끝에서 회수되는 것.
- 인물의 이름·나이·관계·물건·시각은 처음 정한 대로 끝까지 지킨다.
`

// OutlinePrompt is examples/idea-to-novel/prompts/a1-outline.txt for n scenes.
func OutlinePrompt(idea string, n int) string {
	return "# 과제: 아이디어로 소설을 쓴다 — 1단계 개요\n\n" +
		"너는 이 소설을 장면 단위로 직접 쓸 것이다. 먼저 짧은 개요를 쓴다. 이후 장면을 하나씩 쓸 때마다 이 개요와 직전 장면만 보게 된다.\n\n" +
		fmt.Sprintf(requirements, n) + "\n## 아이디어\n" + strings.TrimSpace(idea) + "\n\n" +
		"## 개요에 담을 것 (전체 1200자 안팎, 마크다운)\n" +
		"- 등장인물: 이름, 나이, 관계, 비밀\n" +
		"- 사건의 진상과 시각표(누가 몇 시에 어디 있었는가, 무엇이 거짓인가)\n" +
		"- 심어 두고 회수할 물건·단서\n" +
		fmt.Sprintf("- 장면 1–%d: 장면마다 두세 줄(장소, 시점 인물, 일어나는 일, 누가 무엇을 새로 알게 되는가)\n\n", n) +
		"개요만 출력한다.\n"
}

// ScenePrompt is examples/idea-to-novel/prompts/a-scenes.sh: requirements, idea, outline, previous scene, instruction.
func ScenePrompt(idea, outline, prev string, i, n int) string {
	var b strings.Builder
	fmt.Fprintf(&b, "# 과제: 소설의 장면 %d/%d 를 쓴다\n\n", i, n)
	fmt.Fprintf(&b, requirements, n)
	b.WriteString("\n## 아이디어\n" + strings.TrimSpace(idea) + "\n")
	b.WriteString("\n## 개요\n\n" + strings.TrimSpace(outline) + "\n")
	if i > 1 {
		fmt.Fprintf(&b, "\n## 직전 장면 (장면 %d)\n\n%s\n", i-1, strings.TrimSpace(prev))
	}
	fmt.Fprintf(&b, "\n## 지시\n개요의 장면 %d 를 한국어 산문으로 쓴다. 2500자 안팎. 해라체 서술. 제목·머리말·설명 없이 산문만 출력한다.\n", i)
	return b.String()
}

// meter opens <dir>/calls.jsonl for appending and wraps c.
func meter(c render.Completer, dir string) (*render.Meter, io.Closer, error) {
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return nil, nil, err
	}
	f, err := os.OpenFile(filepath.Join(dir, "calls.jsonl"), os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0o644)
	if err != nil {
		return nil, nil, err
	}
	return render.NewMeter(c, f), f, nil
}

func write(path, s string) error {
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	return os.WriteFile(path, []byte(strings.TrimSpace(s)+"\n"), 0o644)
}

// call refuses a non-end_turn result so a truncated scene is not passed on as the next scene's context.
func call(ctx context.Context, m *render.Meter, label, user string) (string, error) {
	r, err := m.Label(label).Complete(ctx, System, user)
	if err != nil {
		return "", fmt.Errorf("%s: %w", label, err)
	}
	if r.StopReason != "end_turn" {
		return r.Text, fmt.Errorf("%s: stop_reason %q", label, r.StopReason)
	}
	return r.Text, nil
}

// RunA writes <out>/a/{outline.md, prompts/, scenes/s<i>.md, calls.jsonl}. It resumes: an existing outline or
// scene file is reused, not re-asked.
func RunA(ctx context.Context, c render.Completer, idea string, n int, out string) error {
	dir := filepath.Join(out, "a")
	m, f, err := meter(c, dir)
	if err != nil {
		return err
	}
	defer f.Close()
	get := func(path, label, user string) (string, error) {
		if b, err := os.ReadFile(path); err == nil {
			return string(b), nil
		}
		if err := write(filepath.Join(dir, "prompts", label+".txt"), user); err != nil {
			return "", err
		}
		t, err := call(ctx, m, label, user)
		if err != nil {
			return "", err
		}
		return t, write(path, t)
	}
	outline, err := get(filepath.Join(dir, "outline.md"), "outline", OutlinePrompt(idea, n))
	if err != nil {
		return err
	}
	prev := ""
	for i := 1; i <= n; i++ {
		p := filepath.Join(dir, "scenes", fmt.Sprintf("s%d.md", i))
		if prev, err = get(p, fmt.Sprintf("scene-%d", i), ScenePrompt(idea, outline, prev, i, n)); err != nil {
			return err
		}
	}
	return nil
}

// runB runs `dieg write` into <out>/b and keeps its stdout+stderr in <out>/b.log. A swap point for -dry.
var runB = func(ctx context.Context, ideaFile, out string, args []string) error {
	dir := filepath.Join(out, "b")
	if err := os.MkdirAll(out, 0o755); err != nil {
		return err
	}
	log, err := os.Create(filepath.Join(out, "b.log"))
	if err != nil {
		return err
	}
	defer log.Close()
	cmd := exec.CommandContext(ctx, "go", append([]string{"run", "./cmd/dieg", "write", ideaFile, "-o", dir}, args...)...)
	cmd.Stdout, cmd.Stderr = io.MultiWriter(log, os.Stdout), io.MultiWriter(log, os.Stderr)
	err = cmd.Run()
	if ee, ok := err.(*exec.ExitError); ok {
		fmt.Fprintf(log, "bench: dieg write exit %d\n", ee.ExitCode()) // 1 = code not clean, 3 = scenes failed verify
		return nil
	}
	return err
}

// Totals sums one calls.jsonl.
type Totals struct {
	render.Usage
	Calls, Errs int
	Ms          int64
	ByLabel     map[string]render.Usage
}

// In is the whole prompt: the claude CLI puts most of it in the cache fields.
func In(u render.Usage) int64 { return u.Input + u.CacheRead + u.CacheWrite }

func ReadCalls(path string) (Totals, error) {
	t := Totals{ByLabel: map[string]render.Usage{}}
	f, err := os.Open(path)
	if err != nil {
		return t, err
	}
	defer f.Close()
	sc := bufio.NewScanner(f)
	sc.Buffer(make([]byte, 1<<20), 1<<20)
	for sc.Scan() {
		var r struct {
			render.Usage
			Label, Err string
			Ms         int64
		}
		if err := json.Unmarshal(sc.Bytes(), &r); err != nil {
			return t, fmt.Errorf("%s: %w", path, err)
		}
		t.Calls++
		if r.Err != "" {
			t.Errs++
		}
		t.Ms += r.Ms
		t.Input += r.Input
		t.Output += r.Output
		t.CacheRead += r.CacheRead
		t.CacheWrite += r.CacheWrite
		u := t.ByLabel[r.Label]
		u.Input, u.Output, u.CacheRead, u.CacheWrite = u.Input+r.Input, u.Output+r.Output, u.CacheRead+r.CacheRead, u.CacheWrite+r.CacheWrite
		t.ByLabel[r.Label] = u
	}
	return t, sc.Err()
}

var summaryRE = regexp.MustCompile(`write: rounds=(\d+) clean=(\w+) scenes=(\d+) failed=(\d+)(\[[^\]]*\])`)

// Summary prints one idea's table: per arm the four token fields, total input, calls, errors, minutes; B's
// write summary line; and the per-scene input curve (A scene-i vs B render-<scene>-a* and extract-<scene>-a*).
func Summary(w io.Writer, out string) error {
	a, errA := ReadCalls(filepath.Join(out, "a", "calls.jsonl"))
	b, errB := ReadCalls(filepath.Join(out, "b", "calls.jsonl"))
	fmt.Fprintf(w, "### %s\n\n| 팔 | 호출 | 실패 | input | cache_read | cache_write | 총 input | output | 분 |\n|---|---|---|---|---|---|---|---|---|\n", filepath.Base(out))
	for _, r := range []struct {
		n   string
		t   Totals
		err error
	}{{"A", a, errA}, {"B", b, errB}} {
		if r.err != nil {
			fmt.Fprintf(w, "| %s | (없음: %v) |\n", r.n, r.err)
			continue
		}
		fmt.Fprintf(w, "| %s | %d | %d | %d | %d | %d | %d | %d | %.1f |\n", r.n, r.t.Calls, r.t.Errs, r.t.Input, r.t.CacheRead,
			r.t.CacheWrite, In(r.t.Usage), r.t.Output, float64(r.t.Ms)/60000)
	}
	if log, err := os.ReadFile(filepath.Join(out, "b.log")); err == nil {
		if s := summaryRE.FindSubmatch(log); s != nil {
			fmt.Fprintf(w, "\nB: rounds=%s clean=%s scenes=%s verify-failed=%s %s\n", s[1], s[2], s[3], s[4], s[5])
		}
	}
	if b.ByLabel == nil {
		return nil
	}
	var code []string
	for _, l := range sortedKeys(b.ByLabel) {
		if strings.HasPrefix(l, "code-") || strings.HasPrefix(l, "fix-") {
			u := b.ByLabel[l]
			code = append(code, fmt.Sprintf("%s in=%d out=%d", l, In(u), u.Output))
		}
	}
	fmt.Fprintf(w, "\nB 코드: %s\n\n| 장면 | A 입력 | A 출력 | B render 입력 | B extract 입력 | B 출력 |\n|---|---|---|---|---|---|\n", strings.Join(code, ", "))
	ua := a.ByLabel["outline"]
	fmt.Fprintf(w, "| 개요 | %d | %d | | | |\n", In(ua), ua.Output)
	for i := 1; ; i++ {
		ua, okA := a.ByLabel[fmt.Sprintf("scene-%d", i)]
		var rin, ein, bout int64
		okB := false
		for l, u := range b.ByLabel {
			s := fmt.Sprintf("s%d-a", i)
			if strings.HasPrefix(l, "render-"+s) {
				rin, bout, okB = rin+In(u), bout+u.Output, true
			} else if strings.HasPrefix(l, "extract-"+s) {
				ein, bout = ein+In(u), bout+u.Output
			}
		}
		if !okA && !okB {
			break
		}
		fmt.Fprintf(w, "| s%d | %d | %d | %d | %d | %d |\n", i, In(ua), ua.Output, rin, ein, bout)
	}
	return nil
}

func sortedKeys(m map[string]render.Usage) []string {
	var ks []string
	for k := range m {
		ks = append(ks, k)
	}
	sort.Strings(ks)
	return ks
}

func main() {
	if len(os.Args) < 2 {
		fmt.Fprintln(os.Stderr, "usage: bench a|b|judge|sum ...")
		os.Exit(2)
	}
	if err := run(context.Background(), os.Args[1], os.Args[2:], os.Stdout); err != nil {
		fmt.Fprintln(os.Stderr, "bench:", err)
		os.Exit(1)
	}
}

func run(ctx context.Context, cmd string, args []string, w io.Writer) error {
	fs := flag.NewFlagSet(cmd, flag.ContinueOnError)
	ideaFile := fs.String("idea", "", "idea file")
	n := fs.Int("n", 7, "target scenes")
	out := fs.String("out", "", "idea output dir")
	rounds := fs.Int("rounds", 0, "B: dieg write --rounds")
	scenes := fs.Int("scenes", 0, "B: dieg write --scenes (render only the first N)")
	noRender := fs.Bool("no-render", false, "B: dieg write --no-render")
	dry := fs.Bool("dry", false, "fake every model call")
	if err := fs.Parse(args); err != nil {
		return err
	}
	var c render.Completer = render.NewClaudeCLI()
	if *dry {
		c = fake{}
		runB = fakeB
	}
	switch cmd {
	case "sum":
		for _, d := range fs.Args() {
			if err := Summary(w, d); err != nil {
				return err
			}
			fmt.Fprintln(w)
		}
		return nil
	case "judge":
		return RunJudge(ctx, c, *n, *out)
	}
	idea, err := os.ReadFile(*ideaFile)
	if err != nil || *out == "" {
		return fmt.Errorf("need -idea and -out (%v)", err)
	}
	switch cmd {
	case "a":
		return RunA(ctx, c, string(idea), *n, *out)
	case "b":
		var bargs []string
		if *rounds > 0 {
			bargs = append(bargs, "--rounds", fmt.Sprint(*rounds))
		}
		if *scenes > 0 {
			bargs = append(bargs, "--scenes", fmt.Sprint(*scenes))
		}
		if *noRender {
			bargs = append(bargs, "--no-render")
		}
		return runB(ctx, *ideaFile, *out, bargs)
	}
	return fmt.Errorf("unknown command %q", cmd)
}
