package narrate

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"reflect"
	"strings"
	"testing"

	"diegesis/internal/render"
)

// pipeFake 는 프롬프트 종류로 답을 고른다. 비트 k 의 글은 "B<k> 가온이 보았다." 이고, fail[k] 면 "B<k> 실패했다." 다.
// 재추출은 글에 "B<k> 가온이" 가 있으면 비트 k 의 행위를, "환각" 이 있으면 없는 행위를 하나 더 낸다.
type pipeFake struct {
	fail       map[int]bool
	stitchAdds string
}

var pipeFacts = []string{"f1", "f3", "f4"}

func (f *pipeFake) Complete(_ context.Context, system, user string) (render.Result, error) {
	ok := func(t string) (render.Result, error) { return render.Result{Text: t, StopReason: "end_turn"}, nil }
	switch {
	case strings.Contains(system, "한 비트를 쓴다"):
		acts := strings.SplitN(strings.SplitN(user, "## 이 비트의 행위\n", 2)[1], "\n\n", 2)[0]
		for k, id := range pipeFacts {
			if strings.Contains(acts, pipeText[id]) {
				if f.fail[k+1] {
					return ok(fmt.Sprintf("B%d 실패했다.", k+1))
				}
				return ok(fmt.Sprintf("B%d 가온이 보았다.", k+1))
			}
		}
		return ok("?")
	case strings.Contains(system, "문체를 바꾼다"):
		return ok(strings.TrimPrefix(user, "## 바꿀 글\n"))
	case strings.Contains(system, "이음새만 다듬는다"):
		return ok(strings.TrimPrefix(user, "## 비트\n") + f.stitchAdds)
	}
	e := render.Extracted{Acts: []render.ExtractedAct{}, FactsStated: []string{}, NewEvents: []string{}}
	for k, id := range pipeFacts {
		if strings.Contains(user, fmt.Sprintf("B%d 가온이", k+1)) {
			e.Acts = append(e.Acts, render.ExtractedAct{Actor: "a", Verb: "sees", Targets: []string{}, Fact: id})
			e.FactsStated = append(e.FactsStated, id)
		}
	}
	if strings.Contains(user, "환각") {
		e.Acts = append(e.Acts, render.ExtractedAct{Actor: "a", Verb: "thinks", Targets: []string{}, Fact: "f2"})
	}
	b, _ := json.Marshal(e)
	return ok(string(b))
}

var pipeText = map[string]string{"f1": "문이 열렸다", "f3": "등불이 꺼졌다", "f4": "바람이 불었다"}

func pipeScene() render.Scene {
	return render.Scene{
		ID:      "s",
		Cast:    []render.Entity{{ID: "a", Names: map[string][]string{"ko": {"가온"}}}},
		OnStage: []string{"a"},
		Facts: []render.Fact{{ID: "f1", Text: pipeText["f1"], True: true}, {ID: "f2", Text: "열쇠는 서랍에 있다", True: false},
			{ID: "f3", Text: pipeText["f3"], True: true}, {ID: "f4", Text: pipeText["f4"], True: true}},
		Beats: []render.Beat{
			{ID: "b1", Acts: []render.Act{{Actor: "a", Verb: "sees", Fact: "f1"}}},
			{ID: "b2", Acts: []render.Act{{Actor: "a", Verb: "sees", Fact: "f3"}}},
			{ID: "b3", Acts: []render.Act{{Actor: "a", Verb: "sees", Fact: "f4"}}},
		},
	}
}

func pipeRun(t *testing.T, f *pipeFake) (render.Render, LayerStats, []string) {
	t.Helper()
	var buf bytes.Buffer
	m := render.NewMeter(f, &buf)
	r, st, err := RenderLayered(context.Background(), Roles{Beat: m, Verify: m, Style: m, Stitch: m}, "w", pipeScene(),
		render.Backend{ID: "ko-haera", Lang: "ko", Tone: "haera"}, DefaultOpts())
	if err != nil {
		t.Fatal(err)
	}
	var labels []string
	for _, l := range strings.Split(strings.TrimSpace(buf.String()), "\n") {
		var rec struct{ Label string }
		json.Unmarshal([]byte(l), &rec)
		labels = append(labels, rec.Label)
	}
	return r, st, labels
}

func TestRenderLayeredHappyPath(t *testing.T) {
	r, st, labels := pipeRun(t, &pipeFake{})
	if !render.Passed(r) || st.FinalFrom != "stitched" || !st.Stitched || !st.StyleOK || st.Beats != 3 || len(st.BeatFailed) != 0 {
		t.Fatalf("%+v %+v", r.Report, st)
	}
	if !reflect.DeepEqual(st.BeatAttempts, []int{1, 1, 1}) || r.Tones == nil || !strings.Contains(r.Prose, "B3 가온이") {
		t.Fatalf("%+v %v %q", st, r.Tones, r.Prose)
	}
	want := []string{"beat-s-b1-a1", "bverify-s-b1-a1", "beat-s-b2-a1", "bverify-s-b2-a1", "beat-s-b3-a1", "bverify-s-b3-a1",
		"style-s", "stitch-s", "final-s"}
	if !reflect.DeepEqual(labels, want) {
		t.Fatalf("labels %v", labels)
	}
}

// Review Focus 4: 연결 층이 사실을 더하면 최종 대조가 잡고 연결 전 결과를 쓴다.
func TestRenderLayeredStitchAddsFact(t *testing.T) {
	r, st, labels := pipeRun(t, &pipeFake{stitchAdds: " 그는 환각을 보았다."})
	if !render.Passed(r) || st.FinalFrom != "unstitched" || strings.Contains(r.Prose, "환각") {
		t.Fatalf("%+v %+v %q", r.Report, st, r.Prose)
	}
	if labels[len(labels)-1] != "final-s" || labels[len(labels)-2] != "final-s" {
		t.Fatalf("labels %v", labels)
	}
}

// Review Focus 5: 비트 2 가 끝까지 실패해도 장면은 조립되고 기록된다.
func TestRenderLayeredPartialBeatFailure(t *testing.T) {
	r, st, _ := pipeRun(t, &pipeFake{fail: map[int]bool{2: true}})
	if !reflect.DeepEqual(st.BeatFailed, []int{2}) || !reflect.DeepEqual(st.BeatAttempts, []int{1, 3, 1}) {
		t.Fatalf("%+v", st)
	}
	if !strings.Contains(r.Prose, "B2 실패했다") || !strings.Contains(r.Prose, "B3 가온이") {
		t.Fatalf("prose %q", r.Prose)
	}
	if len(r.Report.Missing) != 1 {
		t.Fatalf("%+v", r.Report)
	}
}

type errFake struct{}

func (errFake) Complete(context.Context, string, string) (render.Result, error) {
	return render.Result{}, fmt.Errorf("boom")
}

// Restyle 오류면 문체 안 바꾼 글을 쓴다("" 이 아니다).
func TestRenderLayeredStyleErrorKeepsText(t *testing.T) {
	m := &pipeFake{}
	r, st, err := RenderLayered(context.Background(), Roles{Beat: m, Verify: m, Style: errFake{}, Stitch: m}, "w", pipeScene(),
		render.Backend{ID: "ko-haera", Lang: "ko", Tone: "haera"}, DefaultOpts())
	if err != nil || st.StyleOK || !render.Passed(r) || !strings.Contains(r.Prose, "B1 가온이") {
		t.Fatalf("%v %+v %q", err, st, r.Prose)
	}
}
