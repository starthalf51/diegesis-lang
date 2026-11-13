package render

import (
	"regexp"
	"strings"
)

var (
	proseQuote = regexp.MustCompile(`"[^"]*"|“[^”]*”|「[^」]*」|『[^』]*』`)
	sentence   = regexp.MustCompile(`[^.!?。\n]+[.!?。]*`)
	koTail     = regexp.MustCompile(`(이다|을 말한다|를 말한다|라고 부른다|이라 부른다|이라고 부른다|뜻한다)[.!?。]*$`)
)

// ProseDefinitions returns the narration sentences (proseQuote dialogue excluded) that define a term in dictionary form:
// the surface is the subject of the sentence. ko: "<surface>(은|는|이란|란) … (이다|을/를 말한다|라고 부른다)";
// en: "<surface> is/was a(n)/the …" or "… is called <surface>". A surface used inside a metaphor or as another
// part of the sentence is not flagged. Lang other than ko/en checks both.
// ponytail: copular metaphors ("돔은 감옥이다") are flagged too; narrowing needs a parser.
func ProseDefinitions(prose, lang string, surfaces []string) []string {
	var out []string
	for _, sent := range sentence.FindAllString(proseQuote.ReplaceAllString(prose, " _ "), -1) {
		sent = strings.TrimSpace(sent)
		for _, s := range surfaces {
			q := regexp.QuoteMeta(s)
			hit := false
			if lang != "en" {
				hit = hit || regexp.MustCompile(`^`+q+`(은|는|이란|란)\s`).MatchString(sent) && koTail.MatchString(sent)
			}
			if lang != "ko" {
				hit = hit || regexp.MustCompile(`(?i)^(the\s+|an?\s+)?`+q+`\s+(is|was|are|were)\s+(an?|the)\s`).MatchString(sent) ||
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

// UnrenderedTerms returns the surfaces that do not appear in the prose at all.
func UnrenderedTerms(prose string, surfaces []string) []string {
	var out []string
	for _, s := range surfaces {
		if !strings.Contains(prose, s) {
			out = append(out, s)
		}
	}
	return out
}
