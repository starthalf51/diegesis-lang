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

// Perception (seeing, hearing, recognizing) is a sees act, and a fact conveyed by implication is that act's fact too (live verdict: the rooster's crow and the identity confession were missed).
func TestExtractPromptCoversPerceptionAndImplication(t *testing.T) {
	sys, _ := BuildExtractPrompt(Scene{}, "en", "")
	for _, want := range []string{`record it as verb "sees"`, "can convey a fact without stating it"} {
		if !strings.Contains(sys, want) {
			t.Errorf("extract prompt lacks %q", want)
		}
	}
}

// Hearing what someone else said is the speaker's act: do not add sees to the hearer (live verdict: sees was attached to every line Macbeth heard and Jaccard dropped).
func TestExtractPromptHearingSpeechIsNotPerception(t *testing.T) {
	sys, _ := BuildExtractPrompt(Scene{}, "en", "")
	if !strings.Contains(sys, "Hearing what another character says is part of that speaker's act") {
		t.Error("extract prompt lacks the hearing-speech rule")
	}
}

// When one act conveys several facts, write once per fact (live verdict: the same line was split by each backend picking a different one of the facts).
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
