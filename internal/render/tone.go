package render

import (
	"regexp"
	"strings"
	"unicode"
)

// Inside paired quotes (dialogue) is excluded from the tone judgment. Dialogue is a quotation in the source language, not the narration tone the backend chooses.
// An ASCII single quote cannot be told apart from an apostrophe (Peter's), so it is not treated as a dialogue quote.
var quoted = regexp.MustCompile(`“[^”]*”|"[^"]*"|「[^」]*」|『[^』]*』|‘[^’]*’`)

var sentenceEnd = regexp.MustCompile(`[.!?…\n]+`)

var (
	hapshoEnd = regexp.MustCompile(`(습니다|습니까|십시오)$`)
	gojeonEnd = regexp.MustCompile(`(더라|니라|도다|노라|로다|하매|하니|이로다)$`)
	haeraEnd  = regexp.MustCompile(`(다|라|냐|자)$`)
)

// ClassifyTone produces the distribution (ratio) of sentence-ending forms of narration sentences: it classifies the last word of a sentence in the order hapsho (formal polite) → gojeon (classical novel) → haera (plain) → other.
// Sentences with no Hangul, and connective clauses ending in a comma or colon (a dialogue placed on the next line
// "…he said,") are not counted. An empty result = no sentence to count.
func ClassifyTone(prose string) map[string]float64 {
	counts := map[string]int{}
	n := 0
	for _, s := range sentenceEnd.Split(quoted.ReplaceAllString(prose, " "), -1) {
		w := lastEojeol(s)
		if w == "" {
			continue
		}
		counts[toneOf(w)]++
		n++
	}
	out := map[string]float64{}
	for k, c := range counts {
		out[k] = float64(c) / float64(n)
	}
	return out
}

func toneOf(w string) string {
	switch {
	case hapshoEnd.MatchString(w) || bieupNida(w):
		return "hapsho"
	case gojeonEnd.MatchString(w):
		return "gojeon"
	case haeraEnd.MatchString(w):
		return "haera"
	}
	return "other"
}

// bieupNida: a syllable with final ㅂ + 니다/니까 (합니다, 갑니까 — sees the regex's `ㅂ니다` on precomposed syllables).
func bieupNida(w string) bool {
	r := []rune(w)
	if len(r) < 3 || !(strings.HasSuffix(w, "니다") || strings.HasSuffix(w, "니까")) {
		return false
	}
	s := r[len(r)-3]
	return s >= 0xAC00 && s <= 0xD7A3 && (s-0xAC00)%28 == 17
}

// lastEojeol strips the trailing punctuation from the last word of a sentence. "" if there is no Hangul or it is a connective clause (ends in , : 、).
func lastEojeol(s string) string {
	f := strings.Fields(s)
	if len(f) == 0 || strings.ContainsAny(lastRune(f[len(f)-1]), ",:、，：") {
		return ""
	}
	w := strings.TrimRightFunc(f[len(f)-1], func(r rune) bool { return !unicode.IsLetter(r) })
	if !strings.ContainsFunc(w, func(r rune) bool { return unicode.Is(unicode.Hangul, r) }) {
		return ""
	}
	return w
}

func lastRune(s string) string {
	r := []rune(s)
	return string(r[len(r)-1])
}
