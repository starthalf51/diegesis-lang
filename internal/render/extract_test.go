package render

import (
	"strings"
	"testing"
)

func TestParseExtracted(t *testing.T) {
	e, err := ParseExtracted("Here you go:\n```json\n{\"acts\":[{\"actor\":\"ross\",\"verb\":\"tells\",\"targets\":[\"macbeth\"],\"fact\":\"cawdor_title\"}],\"facts_stated\":[\"cawdor_title\"],\"new_events\":[]}\n```")
	if err != nil || e.Acts[0].Actor != "ross" {
		t.Fatal(err, e)
	}
	if e, err := ParseExtracted(" {\"acts\":[],\"facts_stated\":[\"a\"],\"new_events\":[]} "); err != nil || e.FactsStated[0] != "a" {
		t.Fatal("bare json", err, e)
	}
	for _, bad := range []string{"", "  \n", "not json", "```json\n{\"acts\": [\n```", `{"acts":[],"extra":1}`, "null", "{}", `{"acts":[]}`, `{"acts":null,"facts_stated":[]}`,
		"```json\n{\"acts\":[],\"facts_stated\":[]}", `{"acts":[],"facts_stated":[]} {"acts":[]}`, "```json\n{\"acts\":[],\"facts_stated\":[]} x\n```"} {
		if _, err := ParseExtracted(bad); err == nil {
			t.Errorf("want error for %q", bad)
		}
	}
}

func TestExtractPromptHidesAnswers(t *testing.T) {
	s := mustLoad(t, "testdata/scenes/chibi-jianggan.json")
	sys, user := BuildExtractPrompt(s, "ko", "장간은 …")
	all := strings.ToLower(sys + user)
	if strings.Contains(all, "must not state") || strings.Contains(all, "beat") || strings.Contains(all, "withheld") {
		t.Error("answer key leaked into extractor")
	}
	if !strings.Contains(user, "letter_forged") || !strings.Contains(user, "조조") || !strings.Contains(user, "曹操") || !strings.Contains(user, "장간은 …") {
		t.Error("catalog must list all fact ids, all names, and the prose")
	}
	for _, f := range s.Withheld {
		if !strings.Contains(user, f) || !strings.Contains(user, factText(s, f)) {
			t.Errorf("withheld fact %s must appear unlabeled in catalog", f)
		}
	}
}

func TestExtractPromptScopesNewEventsAndIgnoresWorldKnowledge(t *testing.T) {
	s := mustLoad(t, "testdata/scenes/chibi-jianggan.json")
	sys, _ := BuildExtractPrompt(s, "en", "x")
	for _, w := range []string{
		"plot events only",
		"Ignore description, gesture, setting, and manner.",
		"ignore your knowledge of any original work",
	} {
		if !strings.Contains(sys, w) {
			t.Errorf("extract system missing %q", w)
		}
	}
}

// 지각(보고 듣고 알아봄)은 sees 액트이고, 암시로 전한 사실도 그 액트의 fact 다(live 판정: 닭 울음·정체 고백 누락).
func TestExtractPromptCoversPerceptionAndImplication(t *testing.T) {
	sys, _ := BuildExtractPrompt(Scene{}, "en", "")
	for _, want := range []string{`record it as verb "sees"`, "can convey a fact without stating it"} {
		if !strings.Contains(sys, want) {
			t.Errorf("extract prompt lacks %q", want)
		}
	}
}

// 남이 한 말을 듣는 것은 말한 사람의 액트다: 듣는 쪽에 sees 를 덧붙이지 않는다(live 판정: 맥베스가 들은 말마다 sees 가 붙어 Jaccard 하락).
func TestExtractPromptHearingSpeechIsNotPerception(t *testing.T) {
	sys, _ := BuildExtractPrompt(Scene{}, "en", "")
	if !strings.Contains(sys, "Hearing what another character says is part of that speaker's act") {
		t.Error("extract prompt lacks the hearing-speech rule")
	}
}

// 한 액트가 사실 여럿을 전하면 사실마다 한 번씩 적는다(live 판정: 같은 대사를 백엔드마다 다른 사실 하나로 골라 갈라졌다).
func TestExtractPromptOneEntryPerFact(t *testing.T) {
	sys, _ := BuildExtractPrompt(Scene{}, "en", "")
	if !strings.Contains(sys, "list that act once per fact") {
		t.Error("extract prompt lacks the one-entry-per-fact rule")
	}
}

func TestParseExtractedLegacyStrings(t *testing.T) {
	e, err := ParseExtracted(`{"acts":[],"facts_stated":[],"new_events":["x happens",{"actor":"a","verb":"does","action":"hide","targets":["t"],"text":""}]}`)
	if err != nil || len(e.NewEvents) != 2 || e.NewEvents[0].Text != "x happens" || e.NewEvents[1].Action != "hide" {
		t.Fatalf("%v %+v", err, e)
	}
}
