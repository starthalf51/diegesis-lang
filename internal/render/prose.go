package render

import (
	"regexp"
	"strings"
)

var (
	proseQuote = regexp.MustCompile(`"[^"]*"|“[^”]*”|「[^」]*」|『[^』]*』`)
	sentence   = regexp.MustCompile(`[^.!?。\n]+[.!?。]*`)
	koTail     = regexp.MustCompile(`(이다|[을를] 말(한다|했다)|(이)?라고 (부른다|불렀다|불린다|불렸다)|이라 부른다|뜻(한다|했다))[.!?。]*$`)
)

// ProseDefinitions returns the narration sentences (quoted dialogue excluded) that define a term in dictionary form:
// the surface is the subject of the sentence. ko: "<surface>(은|는|이란|란) … (이다|을/를 말한다/말했다|(이)라고 부른다/불렀다/불린다/불렸다|뜻한다/뜻했다)";
// en: "<surface> is/are a(n)/the …" or "… is called <surface>". A surface used inside a metaphor or as another
// part of the sentence is not flagged. Lang other than ko/en checks both.
// Precision over recall (a hit costs a retry): past copulas ("돔은 …이었다", "was a") are deliberately not flagged, and
// a present copular metaphor ("돔은 감옥이다") still is.
func ProseDefinitions(prose, lang string, surfaces []string) []string {
	var out []string
	for _, sent := range sentence.FindAllString(proseQuote.ReplaceAllString(prose, " "), -1) {
		sent = strings.TrimSpace(sent)
		for _, s := range surfaces {
			q := regexp.QuoteMeta(s)
			hit := false
			if lang != "en" {
				hit = hit || regexp.MustCompile(`^`+q+`(은|는|이란|란)\s`).MatchString(sent) && koTail.MatchString(sent)
			}
			if lang != "ko" {
				hit = hit || regexp.MustCompile(`(?i)^(the\s+|an?\s+)?`+q+`\s+(is|are)\s+(an?|the)\s`).MatchString(sent) ||
					regexp.MustCompile(`(?i)\bis called\s+(the\s+)?`+q+`\b`).MatchString(sent)
			}
			if hit {
				out = append(out, sent)
				break
			}
		}
	}
	return out
}

// UnrenderedTerms returns the surfaces that do not appear in the prose at all (case-insensitive for latin text).
func UnrenderedTerms(prose string, surfaces []string) []string {
	low := strings.ToLower(prose)
	var out []string
	for _, s := range surfaces {
		if !strings.Contains(low, strings.ToLower(s)) {
			out = append(out, s)
		}
	}
	return out
}
