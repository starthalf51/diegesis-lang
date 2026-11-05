package render

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

const stylesDir = "../../styles"

func mustBackends(t *testing.T) []Backend {
	t.Helper()
	bs, err := LoadBackends(stylesDir)
	if err != nil {
		t.Fatal(err)
	}
	return bs
}

func TestLoadBackends(t *testing.T) {
	bs := mustBackends(t)
	var ids []string
	for _, b := range bs {
		ids = append(ids, b.ID+":"+b.Tone)
		if b.Sample.Text == "" || !strings.HasPrefix(b.Sample.Source, "https://") {
			t.Errorf("%s: incomplete sample", b.ID)
		}
	}
	if got := strings.Join(ids, " "); got != "en-doyle: ko-gojeon:gojeon ko-haera:haera ko-hapsho:hapsho" {
		t.Errorf("backends: %s", got)
	}
	// 한국어 백엔드의 시스템 프롬프트는 목표 어투를 밝힌다. 영어 백엔드는 어투 지시가 없다.
	s := mustLoad(t, "testdata/scenes/macbeth-1.3.json")
	want := map[string]string{"ko-haera": "해라체", "ko-hapsho": "하십시오체", "ko-gojeon": "고전소설체"}
	for _, b := range bs {
		sys, _, err := b.Prompt(s)
		if err != nil {
			t.Fatal(err)
		}
		if !strings.Contains(sys, b.Sample.Text[:60]) {
			t.Errorf("%s: sample missing from system prompt", b.ID)
		}
		if w := want[b.ID]; w != "" && !strings.Contains(sys, w) {
			t.Errorf("%s: system prompt lacks %q", b.ID, w)
		}
		if b.Lang == "en" && strings.Contains(sys, "체") {
			t.Errorf("%s: unexpected tone rule", b.ID)
		}
	}
}

// recorder 는 받은 프롬프트를 남기고 차례대로 답한다.
type recorder struct {
	replies []string
	prompts []string
}

func (r *recorder) Complete(_ context.Context, system, user string) (Result, error) {
	r.prompts = append(r.prompts, system+"\n"+user)
	text := r.replies[0]
	r.replies = r.replies[1:]
	return Result{Text: text, StopReason: "end_turn"}, nil
}

func perfectJSON(t *testing.T, s Scene) string {
	t.Helper()
	b, err := json.Marshal(extractedFromScene(s))
	if err != nil {
		t.Fatal(err)
	}
	return "```json\n" + string(b) + "\n```"
}

// Review Focus 5: 재추출 프롬프트에는 백엔드 ID·어투·문체 샘플이 없고, 원작 지식 무시 지시가 있다.
func TestExtractPromptHasNoBackendInfo(t *testing.T) {
	s := mustLoad(t, "testdata/scenes/macbeth-1.3.json")
	for _, b := range mustBackends(t) {
		r := &recorder{replies: []string{"산문입니다.", perfectJSON(t, s)}}
		if _, err := RenderOne(ctx, r, "macbeth", s, b); err != nil {
			t.Fatal(err)
		}
		x := r.prompts[1]
		bad := []string{b.ID, b.Sample.Author, b.Sample.Work, b.Sample.Source, b.Sample.Text[:40], "해라체", "하십시오체", "고전소설체", "style", "tone"}
		if b.Tone != "" {
			bad = append(bad, b.Tone)
		}
		for _, w := range bad {
			if strings.Contains(x, w) {
				t.Errorf("%s: extract prompt contains %q", b.ID, w)
			}
		}
		if !strings.Contains(x, "ignore your knowledge of any original work") || !strings.Contains(x, "산문입니다.") {
			t.Errorf("%s: extract prompt lacks the ignore-source line or the prose", b.ID)
		}
	}
}

var toneProse = map[string]string{
	"haera":  "맥베스가 황야에 섰다. 마녀들이 그를 불렀다. 그는 놀랐다.",
	"hapsho": "맥베스가 황야에 섰습니다. 마녀들이 그를 불렀습니다. 그는 놀랐습니다.",
	"gojeon": "맥베스가 황야에 섰더라. 마녀들이 그를 부르니라. 그가 크게 놀라더라.",
	"":       "Macbeth stood upon the heath.",
}

// renderAll 은 백엔드마다 tone 의 산문과 extract 의 추출 결과를 내는 가짜 Completer 로 렌더한다.
func renderAll(t *testing.T, s Scene, tone func(Backend) string, extract func(Backend) string) []Render {
	t.Helper()
	var rs []Render
	for _, b := range mustBackends(t) {
		r := &recorder{replies: []string{toneProse[tone(b)], extract(b)}}
		x, err := RenderOne(ctx, r, "macbeth", s, b)
		if err != nil {
			t.Fatal(err)
		}
		rs = append(rs, x)
	}
	return rs
}

func TestRenderCheckVerdict(t *testing.T) {
	s := mustLoad(t, "testdata/scenes/chibi-jianggan.json")
	own := func(b Backend) string { return b.Tone }
	perfect := func(Backend) string { return perfectJSON(t, s) }

	v := Judge("chibi", s, renderAll(t, s, own, perfect))
	if !v.Pass || len(v.Jaccard) != 6 {
		t.Fatalf("want pass with 6 pairs: %+v", v)
	}

	// 어투 실패: 하십시오체 백엔드가 해라체로 썼다 → 목표 비율 0 이고 1위 어투가 겹친다.
	v = Judge("chibi", s, renderAll(t, s, func(b Backend) string {
		if b.ID == "ko-hapsho" {
			return "haera"
		}
		return b.Tone
	}, perfect))
	if v.Pass || !strings.Contains(strings.Join(v.Why, ";"), "same top tone") || !failed(v, "ko-hapsho", "hapsho share") {
		t.Errorf("tone failure not caught: %+v", v)
	}

	// 내용 실패: 한 백엔드의 재추출이 사실 둘을 빠뜨리고 숨길 사실을 말했다 → missing 2, leaked, Jaccard < 0.9.
	e := extractedFromScene(s)
	e.Acts = e.Acts[2:]
	e.FactsStated = append(e.FactsStated, s.Withheld...)
	lossy, _ := json.Marshal(e)
	v = Judge("chibi", s, renderAll(t, s, own, func(b Backend) string {
		if b.ID == "ko-gojeon" {
			return string(lossy)
		}
		return perfectJSON(t, s)
	}))
	if v.Pass || !failed(v, "ko-gojeon", "missing") || !failed(v, "ko-gojeon", "leaked") ||
		!strings.Contains(strings.Join(v.Why, ";"), "jaccard") {
		t.Errorf("content failure not caught: %+v", v)
	}
	if len(s.Withheld) == 0 {
		t.Fatal("fixture needs a withheld fact")
	}
}

func failed(v SceneVerdict, backend, why string) bool {
	for _, r := range v.Rows {
		if r.Backend == backend && !r.Pass && strings.Contains(strings.Join(r.Why, ";"), why) {
			return true
		}
	}
	return false
}

func TestRenderCachedSkips(t *testing.T) {
	s := mustLoad(t, "testdata/min.json")
	b := mustBackends(t)[2] // ko-haera
	dir := t.TempDir()
	r := &recorder{replies: []string{toneProse["haera"], perfectJSON(t, s)}}
	if _, err := RenderCached(ctx, r, dir, "w", s, b, false); err != nil {
		t.Fatal(err)
	}
	got, err := RenderCached(ctx, r, dir, "w", s, b, false) // 답이 남아 있지 않다: 호출하면 패닉
	if err != nil || got.Prose != toneProse["haera"] {
		t.Fatalf("cached: %v %+v", err, got)
	}
	md, _ := os.ReadFile(filepath.Join(dir, "w", "s1.ko-haera.md"))
	if !strings.Contains(string(md), toneProse["haera"]) {
		t.Errorf("prose file: %q", md)
	}
}

// 판정은 기록된 Report·Tones 를 믿지 않고 산문·재추출에서 다시 센다: 분류기나 Diff 가 바뀌면 Completer 없이 판정이 바뀐다.
func TestJudgeRescoresCachedRecords(t *testing.T) {
	s := mustLoad(t, "testdata/min.json")
	b := mustBackends(t)[2] // ko-haera
	dir := t.TempDir()
	r := &recorder{replies: []string{toneProse["hapsho"], `{"acts":[],"facts_stated":[],"new_events":[]}`}}
	if _, err := RenderCached(ctx, r, dir, "w", s, b, false); err != nil {
		t.Fatal(err)
	}
	// 옛 분류기·옛 Diff 가 남긴 듯한 기록: 어투 haera 1.0, 빠진 것 없음.
	rec := filepath.Join(dir, "w", "s1.ko-haera.json")
	var x Render
	raw, _ := os.ReadFile(rec)
	if err := json.Unmarshal(raw, &x); err != nil {
		t.Fatal(err)
	}
	x.Tones, x.Report = map[string]float64{"haera": 1}, Report{}
	raw, _ = json.Marshal(x)
	if err := os.WriteFile(rec, raw, 0o644); err != nil {
		t.Fatal(err)
	}
	got, err := RenderCached(ctx, r, dir, "w", s, b, false) // 답이 남아 있지 않다: 호출하면 패닉
	if err != nil {
		t.Fatal(err)
	}
	v := Judge("w", s, []Render{got})
	if v.Pass || !failed(v, "ko-haera", "haera share 0.00") {
		t.Errorf("stale record trusted: %+v", v)
	}
	if v.Rows[0].Tones["hapsho"] != 1 || len(v.Rows[0].Report.Missing) != 1 {
		t.Errorf("not rescored: tones %v report %+v", v.Rows[0].Tones, v.Rows[0].Report)
	}
}

// 장면(곧 렌더·재추출 프롬프트)이 바뀌면 캐시를 버리고 다시 부른다.
func TestRenderCachedInvalidatesOnPromptChange(t *testing.T) {
	s := mustLoad(t, "testdata/min.json")
	b := mustBackends(t)[2]
	dir := t.TempDir()
	r := &recorder{replies: []string{toneProse["haera"], perfectJSON(t, s), toneProse["haera"], perfectJSON(t, s)}}
	first, err := RenderCached(ctx, r, dir, "w", s, b, false)
	if err != nil || first.PromptHash == "" {
		t.Fatalf("%v %q", err, first.PromptHash)
	}
	if _, err := RenderCached(ctx, r, dir, "w", s, b, false); err != nil || len(r.prompts) != 2 {
		t.Fatalf("same scene must hit the cache: %v, %d calls", err, len(r.prompts))
	}
	s.Facts[0].Text = "The king lives."
	again, err := RenderCached(ctx, r, dir, "w", s, b, false)
	if err != nil || len(r.prompts) != 4 || again.PromptHash == first.PromptHash {
		t.Fatalf("changed scene must re-render: %v, %d calls", err, len(r.prompts))
	}
}

// 사실은 지은이의 ID 로 싣고, 비교만 정규 명제(canon)로 한다: 별칭 둘 중 어느 것을 재추출해도 같은 액트다.
func TestDiffCanonicalisesAliases(t *testing.T) {
	s := Scene{
		Facts: []Fact{{ID: "king_hereafter", Text: "Macbeth is the King", Canon: "holds(king,macbeth)"},
			{ID: "holds(king,macbeth)", Text: "Macbeth is the King"}, {ID: "secret", Canon: "not open"}},
		Beats:    []Beat{{Acts: []Act{{Actor: "witch3", Verb: "hails", Targets: []string{"macbeth"}, Fact: "king_hereafter"}}}},
		Withheld: []string{"secret"},
	}
	a := Extracted{Acts: []ExtractedAct{{Actor: "witch3", Verb: "says", Fact: "holds(king,macbeth)"}}, FactsStated: []string{}}
	b := Extracted{Acts: []ExtractedAct{{Actor: "witch3", Verb: "hails", Fact: "king_hereafter"}}, FactsStated: []string{"secret"}}
	if r := Diff(s, a); len(r.Missing) != 0 {
		t.Errorf("alias not matched: %+v", r)
	}
	if r := Diff(s, b); len(r.Missing) != 0 || len(r.Leaked) != 1 {
		t.Errorf("authored id: %+v", r)
	}
	if j := Jaccard(ContentKeys(s, a, false), ContentKeys(s, b, false)); j != 1 {
		t.Errorf("jaccard over aliases = %.2f", j)
	}
}
