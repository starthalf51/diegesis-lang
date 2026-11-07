package narrate

import (
	"context"
	"strings"
	"testing"

	"diegesis/internal/render"
)

type seqFake struct {
	replies []string
	users   []string
}

func (f *seqFake) Complete(_ context.Context, _, user string) (render.Result, error) {
	f.users = append(f.users, user)
	r := f.replies[min(len(f.users)-1, len(f.replies)-1)]
	return render.Result{Text: r, StopReason: "end_turn"}, nil
}

func beatPlan(t *testing.T, acts ...render.Act) Plan {
	t.Helper()
	s := render.Scene{
		Cast:     []render.Entity{{ID: "a", Names: map[string][]string{"ko": {"가온"}}}},
		OnStage:  []string{"a"},
		Facts:    []render.Fact{{ID: "f1", Text: "문이 열렸다", True: true}, {ID: "f2", Text: "열쇠는 서랍에 있다", True: false}, {ID: "x", Text: "범인은 나루다", True: true}},
		Withheld: []string{"x"},
		Beats:    []render.Beat{{ID: "b", Acts: acts}},
	}
	p, err := PlanScene(s, PlanOpts{})
	if err != nil {
		t.Fatal(err)
	}
	return p
}

func TestBeatPromptModality(t *testing.T) {
	sys, user := BeatPrompt(beatPlan(t, render.Act{Actor: "a", Verb: "thinks", Fact: "f2"}), 0, "ko", "", nil)
	if !strings.Contains(sys+user, "인물의 말이나 생각으로만 쓰고 서술자가 사실로 단정하지 않는다") {
		t.Fatal("belief 비트에 단정 금지 문구 없음")
	}
	sys, user = BeatPrompt(beatPlan(t, render.Act{Actor: "a", Verb: "sees", Fact: "f1"}), 0, "ko", "", nil)
	if strings.Contains(sys+user, "단정하지 않는다") {
		t.Fatal("fact 비트에 단정 금지 문구가 있다")
	}
}

func TestBeatPromptForbiddenAllowed(t *testing.T) {
	p := beatPlan(t, render.Act{Actor: "a", Verb: "sees", Fact: "f1"})
	sys, user := BeatPrompt(p, 0, "ko", "", nil)
	all := sys + user
	for _, w := range []string{"다음은 쓰지도 암시하지도 않는다", "범인은 나루다", "덧붙여도 되는 것은 이것뿐이다", p.Beats[0].Allowed[0]} {
		if !strings.Contains(all, w) {
			t.Errorf("없음: %s", w)
		}
	}
}

func TestBeatPromptForms(t *testing.T) {
	p := beatPlan(t, render.Act{Actor: "a", Verb: "thinks", Fact: "f2"}, render.Act{Actor: "a", Verb: "sees", Fact: "f1"})
	_, user := BeatPrompt(p, 0, "ko", "", nil)
	for _, l := range strings.Split(user, "\n") {
		if strings.Contains(l, "thought") && !strings.Contains(l, "내면 독백") {
			t.Errorf("thought 줄: %s", l)
		}
		if strings.Contains(l, "perception") && !strings.Contains(l, "보거나 듣는") {
			t.Errorf("perception 줄: %s", l)
		}
	}
	if !strings.Contains(user, "내면 독백") || !strings.Contains(user, "보거나 듣는") {
		t.Fatal("Form 지시 없음")
	}
}

const (
	okJSON   = `{"acts":[{"actor":"a","verb":"sees","targets":[],"fact":"f1"}],"facts_stated":["f1"],"new_events":[]}`
	halJSON  = `{"acts":[{"actor":"a","verb":"sees","targets":[],"fact":"f1"},{"actor":"a","verb":"thinks","targets":[],"fact":"f2"}],"facts_stated":["f1"],"new_events":[]}`
	missJSON = `{"acts":[],"facts_stated":[],"new_events":[]}`
)

func TestRealizeWithRetryFixesOnlyThatBeat(t *testing.T) {
	p := beatPlan(t, render.Act{Actor: "a", Verb: "sees", Fact: "f1"})
	gen := &seqFake{replies: []string{"첫 글.", "둘째 글."}}
	ver := &seqFake{replies: []string{halJSON, okJSON}}
	text, rep, n, err := realizeWithRetry(context.Background(), gen, ver, p, 0, "ko", "", 2)
	if err != nil || n != 2 || text != "둘째 글." || !BeatPassed(rep) {
		t.Fatalf("%q %+v %d %v", text, rep, n, err)
	}
	if strings.Contains(gen.users[0], "직전 시도에서 고칠 것") {
		t.Error("첫 프롬프트에 피드백")
	}
	u := gen.users[1]
	if !strings.Contains(u, "## 직전 시도에서 고칠 것") || !strings.Contains(u, "지울 것") || !strings.Contains(u, "열쇠는 서랍에 있다") {
		t.Errorf("두 번째 프롬프트에 지울 것 없음:\n%s", u)
	}
}

func TestRealizeWithRetryGivesUp(t *testing.T) {
	p := beatPlan(t, render.Act{Actor: "a", Verb: "sees", Fact: "f1"})
	gen := &seqFake{replies: []string{"가", "나", "다"}}
	ver := &seqFake{replies: []string{missJSON}}
	text, rep, n, err := realizeWithRetry(context.Background(), gen, ver, p, 0, "ko", "", 2)
	if err != nil || n != 3 || text != "다" || BeatPassed(rep) || len(rep.Missing) == 0 {
		t.Fatalf("%q %+v %d %v", text, rep, n, err)
	}
}

func TestBeatPromptForbidsRepeatingPrevTail(t *testing.T) {
	sys, _ := BeatPrompt(beatPlan(t, render.Act{Actor: "a", Verb: "sees", Fact: "f1"}), 0, "ko", "앞 문단", nil)
	if !strings.Contains(sys, "직전 문단의 사건을 다시 쓰지 않는다") {
		t.Fatal("직전 문단 반복 금지 문구 없음")
	}
}

func TestRealizeWithRetryReverifiesExtractError(t *testing.T) {
	p := beatPlan(t, render.Act{Actor: "a", Verb: "sees", Fact: "f1"})
	gen := &seqFake{replies: []string{"글."}}
	ver := &seqFake{replies: []string{"쓰레기", okJSON}}
	text, rep, n, err := realizeWithRetry(context.Background(), gen, ver, p, 0, "ko", "", 2)
	if err != nil || n != 1 || len(gen.users) != 1 || len(ver.users) != 2 || text != "글." || !BeatPassed(rep) {
		t.Fatalf("%q %+v %d %d %v", text, rep, n, len(gen.users), err)
	}
}

func TestRealizeWithRetryNegativeRetries(t *testing.T) {
	p := beatPlan(t, render.Act{Actor: "a", Verb: "sees", Fact: "f1"})
	gen := &seqFake{replies: []string{"글."}}
	ver := &seqFake{replies: []string{missJSON}}
	_, _, n, err := realizeWithRetry(context.Background(), gen, ver, p, 0, "ko", "", -3)
	if err != nil || n != 1 {
		t.Fatalf("%d %v", n, err)
	}
}

type truncFake struct{ calls int }

func (f *truncFake) Complete(_ context.Context, _, _ string) (render.Result, error) {
	f.calls++
	if f.calls == 1 {
		return render.Result{Text: "잘린 글", StopReason: "max_tokens"}, nil
	}
	return render.Result{Text: "온전한 글.", StopReason: "end_turn"}, nil
}

func TestRealizeWithRetryTruncatedNotVerified(t *testing.T) {
	p := beatPlan(t, render.Act{Actor: "a", Verb: "sees", Fact: "f1"})
	gen := &truncFake{}
	ver := &seqFake{replies: []string{okJSON}}
	text, rep, n, err := realizeWithRetry(context.Background(), gen, ver, p, 0, "ko", "", 2)
	if err != nil || n != 2 || text != "온전한 글." || len(ver.users) != 1 || !BeatPassed(rep) {
		t.Fatalf("%q %+v %d %d %v", text, rep, n, len(ver.users), err)
	}
}
