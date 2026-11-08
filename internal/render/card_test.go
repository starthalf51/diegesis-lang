package render

import (
	"reflect"
	"strings"
	"testing"
)

func cardScene() Scene {
	return Scene{
		Cast:  []Entity{{ID: "a", Names: map[string][]string{"ko": {"가"}}}, {ID: "b", Names: map[string][]string{"ko": {"나"}}}},
		Facts: []Fact{{ID: "fa", Text: "A fact"}, {ID: "fb", Text: "B fact"}, {ID: "fc", Text: "C fact"}, {ID: "fw", Text: "Secret"}},
		Beats: []Beat{{Acts: []Act{
			{Actor: "a", Verb: "tells", Targets: []string{"b"}, Fact: "fa"},
			{Actor: "a", Verb: "says", Targets: []string{"b"}, Fact: "fb"},
			{Actor: "b", Verb: "tells", Targets: []string{"a"}, Fact: "fc"},
		}}},
		Withheld: []string{"fw"},
	}
}

func tierReply(text string) Result { return Result{Text: "```json\n" + text + "\n```", StopReason: "end_turn"} }

const t1Body = `{"items":[
{"id":"E1","fact_present":"yes","as_stated":"yes","quote":"q"},
{"id":"E2","fact_present":"no","as_stated":"no","quote":""},
{"id":"E3","fact_present":"yes","as_stated":"no","quote":"q"}],
"withheld":[{"id":"W1","revealed":"yes","quote":"leak"}],
"other":[{"actor":"a","verb":"tells","targets":["b"],"text":"새 일","quote":"x"}]}`

func near(a, b float64) bool { return a-b < 1e-9 && b-a < 1e-9 }

func TestCardItemsMatchDiff(t *testing.T) {
	s := Scene{
		Facts: []Fact{{ID: "f1"}, {ID: "f2"}},
		Beats: []Beat{{Acts: []Act{
			{Actor: "a", Verb: "tells", Fact: "f1"}, {Actor: "a", Verb: "says", Fact: "f1"},
			{Actor: "b", Verb: "sees", Fact: "f2"},
			{Actor: "a", Verb: "enter"}, {Actor: "a", Verb: "does", Action: "x"},
		}}},
	}
	want := Diff(s, Extracted{}).Missing
	if got := CardItems(s); !reflect.DeepEqual(got, want) || len(got) != 3 {
		t.Fatalf("got %v want %v", got, want)
	}
}

func TestScoreCardMapsReport(t *testing.T) {
	s := cardScene()
	f := &fakeCompleter{replies: []Result{tierReply(t1Body)}}
	r, err := ScoreCard(ctx, f, s, Backend{Lang: "en"}, "prose", CardOpts{Tier2: "off"})
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(r.Missing, []string{"a|assert|fb", "b|assert|fc"}) || !reflect.DeepEqual(r.Leaked, []string{"fw"}) ||
		len(r.Hallucinated) != 0 || len(r.Warnings) != 1 || r.ExtractError != "" {
		t.Fatalf("%+v", r)
	}
	if !near(r.Scores.Coverage, 2.0/3) || !near(r.Scores.Kind, 1.0/3) {
		t.Fatalf("%+v", r.Scores)
	}
}

func TestScoreCardRetryThenError(t *testing.T) {
	f := &fakeCompleter{replies: []Result{tierReply("{broken"), tierReply(`{"items":[{"id":"E1","fact_present":"yes","as_stated":"yes"}],"withheld":[]}`)}}
	r, err := ScoreCard(ctx, f, cardScene(), Backend{Lang: "en"}, "prose", CardOpts{Tier2: "off"})
	if err != nil || f.calls != 2 || r.ExtractError == "" || Passed(Render{Report: r}) {
		t.Fatalf("err=%v calls=%d %+v", err, f.calls, r)
	}
}

func TestScoreCardNoWithheld(t *testing.T) {
	s := cardScene()
	s.Withheld = nil
	_, user := BuildCardPrompt(s, "en", "prose")
	if strings.Contains(user, "Withheld") {
		t.Fatal(user)
	}
	f := &fakeCompleter{replies: []Result{tierReply(`{"items":[{"id":"E1","fact_present":"yes","as_stated":"yes"},{"id":"E2","fact_present":"yes","as_stated":"yes"},{"id":"E3","fact_present":"yes","as_stated":"yes"}],"other":[]}`)}}
	r, err := ScoreCard(ctx, f, s, Backend{Lang: "en"}, "prose", CardOpts{Tier2: "off"})
	if err != nil || len(r.Leaked) != 0 || len(r.Missing) != 0 || r.ExtractError != "" {
		t.Fatalf("%v %+v", err, r)
	}
}

func TestScoreCardTier2(t *testing.T) {
	re := tierReply(`{"recheck":[{"id":"E2","fact_present":"yes","as_stated":"yes","quote":"q"},{"id":"E3","fact_present":"yes","as_stated":"no","quote":"q"}]}`)
	f := &fakeCompleter{replies: []Result{tierReply(t1Body), re}}
	r, err := ScoreCard(ctx, f, cardScene(), Backend{Lang: "en"}, "prose", CardOpts{Tier2: "absent-only"})
	if err != nil || f.calls != 2 || !reflect.DeepEqual(r.Missing, []string{"b|assert|fc"}) || len(r.Warnings) != 1 {
		t.Fatalf("%v calls=%d %+v", err, f.calls, r)
	}
	f = &fakeCompleter{replies: []Result{tierReply(t1Body)}}
	if _, err = ScoreCard(ctx, f, cardScene(), Backend{Lang: "en"}, "prose", CardOpts{Tier2: "off"}); err != nil || f.calls != 1 {
		t.Fatalf("%v calls=%d", err, f.calls)
	}
	f = &fakeCompleter{replies: []Result{tierReply(t1Body), re}} // "" means the default, absent-only
	if _, err = ScoreCard(ctx, f, cardScene(), Backend{Lang: "en"}, "prose", CardOpts{}); err != nil || f.calls != 2 {
		t.Fatalf("%v calls=%d", err, f.calls)
	}
}

func TestScoresLengthDropped(t *testing.T) {
	f := &fakeCompleter{replies: []Result{tierReply(t1Body)}}
	r, _ := ScoreCard(ctx, f, cardScene(), Backend{Lang: "en"}, "prose", CardOpts{Tier2: "off"})
	sc := r.Scores
	want := (0.6*sc.Coverage + 0.2*sc.Kind + 0.1*sc.Tone) / 0.9
	if sc.Length != 0 || !near(sc.Score, want) {
		t.Fatalf("%+v want %v", sc, want)
	}
	f = &fakeCompleter{replies: []Result{tierReply(t1Body)}}
	r, _ = ScoreCard(ctx, f, cardScene(), Backend{Lang: "en"}, "ab cd", CardOpts{Tier2: "off", TargetChars: 8})
	if !near(r.Scores.Length, 0.5) {
		t.Fatalf("%+v", r.Scores)
	}
}
