package render

import (
	"encoding/json"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
)

// rep 는 렌더 한 번 + 재추출 DefaultK 번의 답이다.
func rep(prose, extract string) []string {
	out := []string{prose}
	for range DefaultK {
		out = append(out, extract)
	}
	return out
}

func xjson(t *testing.T, e Extracted) Result {
	t.Helper()
	b, err := json.Marshal(e)
	if err != nil {
		t.Fatal(err)
	}
	return Result{Text: "```json\n" + string(b) + "\n```", StopReason: "end_turn"}
}

func voteScene() Scene {
	return Scene{
		Facts:    []Fact{{ID: "f1"}, {ID: "f2"}, {ID: "w"}},
		Withheld: []string{"w"},
		Beats:    []Beat{{Acts: []Act{{Actor: "a", Verb: "says", Fact: "f1"}, {Actor: "a", Verb: "says", Fact: "f1"}}}},
	}
}

func keys(s Scene, e Extracted) map[string]int { return ContentKeys(s, e, true) }

// R1: 3회 중 2회 이상 나온 액트 키·facts_stated 만 채택한다. 숨긴 사실은 1회만 나와도 누출이다(fail-closed).
func TestExtractKMajority(t *testing.T) {
	s := voteScene()
	act := func(actor, verb, fact string) ExtractedAct { return ExtractedAct{Actor: actor, Verb: verb, Fact: fact} }
	runs := []Extracted{
		{Acts: []ExtractedAct{act("a", "says", "f1"), act("a", "tells", "f1"), act("a", "says", "f2")}, FactsStated: []string{"f1", "f2"}, NewEvents: []NewEvent{{Text: "x happens"}}},
		{Acts: []ExtractedAct{act("a", "hails", "f1"), act("a", "says", "f1")}, FactsStated: []string{"f1"}, NewEvents: []NewEvent{}},
		{Acts: []ExtractedAct{act("a", "says", "f1"), act("b", "sees", "f2"), act("b", "aside", "w")}, FactsStated: []string{"f1", "f2"}, NewEvents: []NewEvent{{Text: "y happens"}}},
	}
	f := &fakeCompleter{}
	for _, e := range runs {
		f.replies = append(f.replies, xjson(t, e))
	}
	got, raws, err := ExtractK(ctx, f, s, "en", "prose", 3)
	if err != nil {
		t.Fatal(err)
	}
	if f.calls != 3 || len(raws) != 3 || !reflect.DeepEqual(raws[1], runs[1]) {
		t.Fatalf("calls %d, raws %d", f.calls, len(raws))
	}
	// a|assert|f1 은 세 번 모두 두 번씩 → 2. a|assert|f2(1회), b|perceive|f2(1회), b|assert|w(1회) 는 탈락.
	if k := keys(s, got); !reflect.DeepEqual(k, map[string]int{"a|assert|f1": 2}) {
		t.Errorf("acts: %v", k)
	}
	// f1(3회)·f2(2회) 채택, w 는 액트로 1회 → 누출 판정용으로 채택. new_events 는 2회 이상 나온 개수(1)만.
	if want := []string{"f1", "f2", "w"}; !reflect.DeepEqual(got.FactsStated, want) {
		t.Errorf("facts_stated %v", got.FactsStated)
	}
	if len(got.NewEvents) != 1 {
		t.Errorf("new_events %v", got.NewEvents)
	}
	r := Diff(s, got)
	if !reflect.DeepEqual(r.Leaked, []string{"w"}) || len(r.Missing) != 0 {
		t.Errorf("diff %+v", r)
	}
	// 캐시에서 다시 판정하면 같은 다수결이 나온다(호출 없음).
	if again := Vote(s, raws); !reflect.DeepEqual(again, got) {
		t.Errorf("vote not reproducible: %+v vs %+v", again, got)
	}
}

// 재추출 하나라도 깨지면 ExtractError 다(다수결을 일부로 하지 않는다). Completer 오류는 그대로 돌려준다.
func TestExtractKFailures(t *testing.T) {
	s := voteScene()
	f := &fakeCompleter{replies: []Result{xjson(t, Extracted{Acts: []ExtractedAct{}, FactsStated: []string{}}), {Text: "not json", StopReason: "end_turn"}, xjson(t, Extracted{Acts: []ExtractedAct{}, FactsStated: []string{}})}}
	_, _, err := ExtractK(ctx, f, s, "en", "p", 3)
	if !IsBadExtract(err) {
		t.Errorf("want bad extract, got %v", err)
	}
	f = &fakeCompleter{replies: []Result{{Text: "x", StopReason: "max_tokens"}}}
	if _, _, err := ExtractK(ctx, f, s, "en", "p", 3); !IsBadExtract(err) || !strings.Contains(err.Error(), "max_tokens") {
		t.Errorf("want stop error, got %v", err)
	}
	f = &fakeCompleter{err: os.ErrDeadlineExceeded}
	if _, _, err := ExtractK(ctx, f, s, "en", "p", 3); err == nil || IsBadExtract(err) {
		t.Errorf("completer error must pass through: %v", err)
	}
}

// R4: 원본보다 많이 뽑힌 사실 액트는 초과분만큼 환각이다(원본에 없는 사실 액트도).
func TestDiffExtraActs(t *testing.T) {
	s := voteScene()
	e := Extracted{
		Acts:        []ExtractedAct{{Actor: "a", Verb: "says", Fact: "f1"}, {Actor: "a", Verb: "tells", Fact: "f1"}, {Actor: "a", Verb: "hails", Fact: "f1"}, {Actor: "a", Verb: "tells", Fact: "f1"}, {Actor: "c", Verb: "tells", Fact: "f2"}, {Actor: "c", Verb: "tells"}, {Verb: "enter", Targets: []string{"c"}}},
		FactsStated: []string{"f1", "f2"},
	}
	r := Diff(s, e)
	want := []string{"extra-act:a|assert|f1", "extra-act:a|assert|f1", "extra-act:c|assert|f2"}
	if !reflect.DeepEqual(r.Hallucinated, want) {
		t.Errorf("hallucinated %v", r.Hallucinated)
	}
	if r := Diff(s, extractedFromScene(s)); len(r.Hallucinated) != 0 {
		t.Errorf("exact extraction: %v", r.Hallucinated)
	}
}

// R3: 방백·생각의 명제는 그 인물의 단언이다.
func TestExtractPromptAsides(t *testing.T) {
	sys, _ := BuildExtractPrompt(Scene{}, "en", "")
	if !strings.Contains(sys, "An aside or a character's thoughts assert their proposition") {
		t.Error("extract prompt lacks the aside line")
	}
}

// --force-extract: 산문은 캐시를 쓰고 재추출만 다시 한다. 재추출이 없거나 깨진 기록은 플래그 없이도 다시 뽑는다.
func TestRenderCachedForceExtract(t *testing.T) {
	s := mustLoad(t, "testdata/min.json")
	b := mustBackends(t)[2] // ko-haera
	dir := t.TempDir()
	r := &recorder{replies: rep(toneProse["haera"], perfectJSON(t, s))}
	if _, err := RenderCached(ctx, r, dir, "w", s, b, Force{}); err != nil {
		t.Fatal(err)
	}
	r.replies = []string{perfectJSON(t, s), perfectJSON(t, s), perfectJSON(t, s)}
	got, err := RenderCached(ctx, r, dir, "w", s, b, Force{Extract: true})
	if err != nil || len(r.prompts) != 1+2*DefaultK || got.Prose != toneProse["haera"] || len(got.Extractions) != DefaultK {
		t.Fatalf("force-extract: %v, %d prompts, %+v", err, len(r.prompts), got)
	}
	for _, p := range r.prompts[1+DefaultK:] {
		if !strings.Contains(p, "## Prose\n"+toneProse["haera"]) {
			t.Errorf("re-extract must use the cached prose: %q", p)
		}
	}
	// 재추출이 하나뿐인 옛 기록: 플래그 없이 재추출만 다시 한다.
	rec := filepath.Join(dir, "w", "s1.ko-haera.json")
	var x Render
	raw, _ := os.ReadFile(rec)
	if err := json.Unmarshal(raw, &x); err != nil {
		t.Fatal(err)
	}
	x.Extractions = x.Extractions[:1]
	raw, _ = json.Marshal(x)
	if err := os.WriteFile(rec, raw, 0o644); err != nil {
		t.Fatal(err)
	}
	r.replies = []string{perfectJSON(t, s), perfectJSON(t, s), perfectJSON(t, s)}
	got, err = RenderCached(ctx, r, dir, "w", s, b, Force{})
	if err != nil || len(r.prompts) != 1+3*DefaultK || len(got.Extractions) != DefaultK || got.Prose != toneProse["haera"] {
		t.Fatalf("incomplete extraction not redone: %v, %d prompts", err, len(r.prompts))
	}
	// 재추출 프롬프트가 바뀌어도 산문은 그대로다(렌더 해시는 같다).
	x.ExtractHash = "old"
	x.Extractions = got.Extractions
	raw, _ = json.Marshal(x)
	_ = os.WriteFile(rec, raw, 0o644)
	r.replies = []string{perfectJSON(t, s), perfectJSON(t, s), perfectJSON(t, s)}
	if got, err = RenderCached(ctx, r, dir, "w", s, b, Force{}); err != nil || len(r.prompts) != 1+4*DefaultK || got.Prose != toneProse["haera"] {
		t.Fatalf("extract-prompt change must re-extract only: %v, %d prompts", err, len(r.prompts))
	}
}

// 판정은 기록의 Extracted 를 믿지 않고 원본 재추출들로 다수결을 다시 한다.
func TestScoreRevotesFromExtractions(t *testing.T) {
	s := mustLoad(t, "testdata/min.json")
	e := extractedFromScene(s)
	empty := Extracted{Acts: []ExtractedAct{}, FactsStated: []string{}}
	r := Score(s, Render{Extractions: []Extracted{e, e, empty}, Extracted: empty})
	if len(r.Report.Missing) != 0 || !reflect.DeepEqual(keys(s, r.Extracted), keys(s, e)) {
		t.Errorf("not re-voted: %+v", r)
	}
}

func TestVoteAuthoredEventCannotHideInvention(t *testing.T) {
	s := Scene{Cast: []Entity{{ID: "a"}}, Beats: []Beat{{Acts: []Act{{Actor: "a", Verb: "does", Action: "hide"}}}}}
	auth := NewEvent{Actor: "a", Verb: "does", Action: "hide"}
	inv := NewEvent{Text: "someone dies"}
	got := Vote(s, []Extracted{{NewEvents: []NewEvent{auth}}, {NewEvents: []NewEvent{inv}}, {NewEvents: []NewEvent{inv}}})
	if len(got.NewEvents) != 1 || got.NewEvents[0].Text != "someone dies" {
		t.Fatalf("%+v", got.NewEvents)
	}
}
