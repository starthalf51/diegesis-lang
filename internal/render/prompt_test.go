package render

import (
	"strings"
	"testing"
)

func TestBuildPromptVariants(t *testing.T) {
	s := mustLoad(t, "testdata/scenes/holmes-scandal-opening.json")
	st := mustStyle(t, "testdata/styles/doyle-redheaded.json")
	sysA, userA, err := BuildPrompt(s, VariantIROnly, "en", nil)
	if err != nil {
		t.Fatal(err)
	}
	sysC, userC, err := BuildPrompt(s, VariantStyleDirectives, "en", &st)
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(sysA, st.Text[:40]) {
		t.Error("variant a must not include style sample")
	}
	if !strings.Contains(sysC, st.Text[:40]) {
		t.Error("variant c must include style sample")
	}
	if strings.Contains(userA, "## Directives") || !strings.Contains(userC, "## Directives") {
		t.Error("directives only in c")
	}
	for _, u := range []string{userA, userC} {
		if !strings.Contains(u, "## Must not state or imply") || !strings.Contains(u, factText(s, "visitor_is_king")) {
			t.Error("withheld list missing")
		}
	}
	if strings.Contains(sysA+sysC, "visitor_is_king") {
		t.Error("scene content leaked into cacheable system prompt")
	}
	// variant a ignores a non-nil style
	sysA2, _, err := BuildPrompt(s, VariantIROnly, "en", &st)
	if err != nil || sysA2 != sysA {
		t.Errorf("variant a must ignore style: err=%v", err)
	}
}

func TestBuildPromptDeterministic(t *testing.T) {
	s := mustLoad(t, "testdata/scenes/holmes-scandal-opening.json")
	st := mustStyle(t, "testdata/styles/doyle-redheaded.json")
	s1, u1, _ := BuildPrompt(s, VariantStyleDirectives, "en", &st)
	s2, u2, _ := BuildPrompt(s, VariantStyleDirectives, "en", &st)
	if s1 != s2 || u1 != u2 {
		t.Error("not deterministic")
	}
}

func TestBuildPromptRejectsStyleLangMismatch(t *testing.T) {
	s := mustLoad(t, "testdata/scenes/holmes-scandal-opening.json")
	st := mustStyle(t, "testdata/styles/doyle-redheaded.json")
	if _, _, err := BuildPrompt(s, VariantStyle, "ko", &st); err == nil {
		t.Error("want lang mismatch error")
	}
	if _, _, err := BuildPrompt(s, VariantStyle, "en", nil); err == nil {
		t.Error("want nil style error")
	}
}

func TestBuildPromptKoreanNames(t *testing.T) {
	s := mustLoad(t, "testdata/scenes/chibi-jianggan.json")
	_, u, err := BuildPrompt(s, VariantIROnly, "ko", nil)
	if err != nil {
		t.Fatal(err)
	}
	// fixture prose (previously, fact text) is English and names Cao Cao; check only the rendered cast list.
	cast := u[strings.Index(u, "## Characters"):strings.Index(u, "## Acts")]
	if !strings.Contains(u, "조조") || !strings.Contains(cast, "조조") || strings.Contains(cast, "Cao Cao") {
		t.Error("ko names expected in cast, en names forbidden")
	}
}

func TestBuildPromptNarrationAndReportedSpeech(t *testing.T) {
	h := mustLoad(t, "testdata/scenes/holmes-scandal-opening.json")
	c := mustLoad(t, "testdata/scenes/chibi-jianggan.json")
	st := mustStyle(t, "testdata/styles/doyle-redheaded.json")
	for _, v := range []Variant{VariantIROnly, VariantStyle, VariantStyleDirectives} {
		sys, u, err := BuildPrompt(h, v, "en", &st)
		if err != nil || !strings.Contains(u, "Narration: first person, as Watson") {
			t.Errorf("%s: %v\n%s", v, err, u)
		}
		if !strings.Contains(sys, "Speech acts without a quote are reported speech; never invent quoted dialogue.") || strings.Contains(sys, "read aloud to") {
			t.Errorf("%s: system prompt rules", v)
		}
	}
	_, u, _ := BuildPrompt(c, VariantIROnly, "en", nil)
	if strings.Contains(u, "Narration:") {
		t.Error("third person is the default; no narration line")
	}
}

// A perception act must be written as the actor seeing or hearing, and an act with a fact must convey that fact (live verdict: only "the rooster crowed" was
// written and that Peter heard it was dropped, so re-extraction lost the perception).
func TestBuildPromptMakesPerceptionAndFactsExplicit(t *testing.T) {
	sys, _, err := BuildPrompt(mustLoad(t, "testdata/min.json"), VariantIROnly, "ko", nil)
	if err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{"For a `sees` act, say that its actor sees or hears", "must make that fact clear"} {
		if !strings.Contains(sys, want) {
			t.Errorf("system prompt lacks %q", want)
		}
	}
}

// The prose writes only the spec's acts: do not slip in other lines of the original as indirect speech (live verdict: "borrowed robes" crept in).
func TestBuildPromptForbidsSourceLines(t *testing.T) {
	sys, _, _ := BuildPrompt(mustLoad(t, "testdata/min.json"), VariantIROnly, "en", nil)
	if !strings.Contains(sys, "Do not add any speech, even reported speech, that is not an act") {
		t.Error("system prompt lacks the no-extra-speech rule")
	}
}

// A fact is conveyed only by the act that has it: do not repeat it as a character's thought or reaction (live verdict: the classical-novel tone restated the fact after a line with
// "in Macbeth's thought …" and re-extracted thinks acts increased).
func TestBuildPromptNoFactsThroughAddedThoughts(t *testing.T) {
	sys, _, _ := BuildPrompt(mustLoad(t, "testdata/min.json"), VariantIROnly, "ko", nil)
	if !strings.Contains(sys, "Do not restate a fact through a character's thoughts or reactions") {
		t.Error("system prompt lacks the no-restated-facts rule")
	}
}

// An act holding a false fact (says/lies) is marked so the narrator does not state it as fact (demo s4 "the bell rang").
func TestPromptMarksFalseFacts(t *testing.T) {
	const tag = "(false — the speaker believes or claims it; the narrator must not assert it)"
	s := Scene{
		Facts: []Fact{{ID: "bell_rang", Text: "the bell rang", True: false}, {ID: "door_open", Text: "the door is open", True: true}},
		Beats: []Beat{{Acts: []Act{
			{Actor: "a", Verb: "says", Fact: "bell_rang"},
			{Actor: "a", Verb: "lies", Fact: "bell_rang"},
			{Actor: "a", Verb: "says", Fact: "door_open"},
		}}},
	}
	sys, user, err := BuildPrompt(s, VariantIROnly, "en", nil)
	if err != nil {
		t.Fatal(err)
	}
	var lines []string
	for _, l := range strings.Split(user, "\n") {
		if strings.HasPrefix(l, "- ") && strings.Contains(l, ": the ") {
			lines = append(lines, l)
		}
	}
	if len(lines) != 3 {
		t.Fatalf("want 3 act lines, got %q", lines)
	}
	if !strings.HasSuffix(lines[0], tag) || !strings.Contains(lines[1], "lies") || !strings.HasSuffix(lines[1], tag) {
		t.Errorf("false facts not marked: %q", lines[:2])
	}
	if strings.Contains(lines[2], "false") {
		t.Errorf("true fact marked: %q", lines[2])
	}
	rule := "- A fact marked false is only what a character says, believes or pretends. Never state it in the narrator's own voice as what happened."
	if !strings.Contains(sys, rule) {
		t.Error("system prompt lacks the false-fact rule")
	}
}

const oldDialogueRule = "- Dialogue may only use the provided `quote` strings. Speech acts without a quote are reported speech; never invent quoted dialogue.\n"
const newDialogueRule = "- A speech act may be voiced as dialogue you write. It must say only that act's fact (or, without a fact, only small talk that states no fact). Use a provided quote verbatim when there is one.\n"

func TestPromptFreeDialogue(t *testing.T) {
	s := mustLoad(t, "testdata/scenes/holmes-scandal-opening.json")
	off, _, err := BuildPrompt(s, VariantIROnly, "en", nil)
	if err != nil {
		t.Fatal(err)
	}
	s.FreeDialogue = true
	on, _, err := BuildPrompt(s, VariantIROnly, "en", nil)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(off, oldDialogueRule) || strings.Contains(off, newDialogueRule) {
		t.Error("false must keep the old rule")
	}
	if !strings.Contains(on, newDialogueRule) || strings.Contains(on, oldDialogueRule) {
		t.Error("true must swap in the new rule")
	}
	if strings.Replace(off, oldDialogueRule, newDialogueRule, 1) != on {
		t.Error("only the rule line may differ")
	}
}

func TestPromptTerms(t *testing.T) {
	s := mustLoad(t, "testdata/scenes/holmes-scandal-opening.json")
	_, base, _ := BuildPrompt(s, VariantIROnly, "en", nil)
	const hn = "## New terms — introduce each to a reader who has never heard it, inside the story, using its meaning\n"
	const hk = "## Terms the reader already knows\n"
	if strings.Contains(base, hn) || strings.Contains(base, hk) {
		t.Fatal("term sections present without terms")
	}
	s.TermsNew = []Term{{"족보", "국가 기록"}}
	_, u, _ := BuildPrompt(s, VariantIROnly, "en", nil)
	if !strings.Contains(u, hn+"- 족보: 국가 기록\n") || strings.Contains(u, hk) {
		t.Errorf("new only:\n%s", u)
	}
	s.TermsNew, s.TermsKnown = nil, []Term{{"칩", "신분 칩"}}
	_, u, _ = BuildPrompt(s, VariantIROnly, "en", nil)
	if !strings.Contains(u, hk+"- 칩\n") || strings.Contains(u, hn) {
		t.Errorf("known only:\n%s", u)
	}
	const jargon = "Do not coin new jargon: any work-specific word must be one of the listed terms."
	const nest = "Never nest narration inside a quote; a quote is only what the character says aloud."
	const thoughts = "In first-person narration, never report another character's unspoken thoughts."
	for _, tc := range []struct {
		name                   string
		terms                  bool
		narration              string
		jargon, nest, thoughts bool
	}{
		{"plain", false, "", false, false, false},
		{"third person", false, "third-person", false, true, false},
		{"first person", false, "first-person", false, true, true},
		{"terms", true, "", true, true, false},
		{"terms first person", true, "first-person", true, true, true},
	} {
		c := s
		c.TermsNew, c.TermsKnown, c.Narration = nil, nil, tc.narration
		if tc.terms {
			c.TermsKnown = []Term{{"칩", "신분 칩"}}
		}
		sys, _, _ := BuildPrompt(c, VariantIROnly, "en", nil)
		for r, want := range map[string]bool{jargon: tc.jargon, nest: tc.nest, thoughts: tc.thoughts} {
			if strings.Contains(sys, r) != want {
				t.Errorf("%s: rule %q present=%v, want %v", tc.name, r, !want, want)
			}
		}
	}
}
