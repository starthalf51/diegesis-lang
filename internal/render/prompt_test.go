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

// 지각 액트는 행위자가 보거나 듣는다고 써야 하고, 사실이 붙은 액트는 그 사실을 전해야 한다(live 판정: "닭이 울더라" 만
// 쓰고 베드로가 들었다는 것을 빼 재추출이 지각을 잃었다).
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

// 산문은 명세의 액트만 쓴다: 원작의 다른 대사를 간접화법으로 끼워 넣지 않는다(live 판정: "borrowed robes" 가 끼어들었다).
func TestBuildPromptForbidsSourceLines(t *testing.T) {
	sys, _, _ := BuildPrompt(mustLoad(t, "testdata/min.json"), VariantIROnly, "en", nil)
	if !strings.Contains(sys, "Do not add any speech, even reported speech, that is not an act") {
		t.Error("system prompt lacks the no-extra-speech rule")
	}
}
