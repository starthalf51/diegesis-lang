package render

import (
	"regexp"
	"strings"
	"unicode"
)

// 따옴표 쌍 안(대사)은 어투 판정에서 뺀다. 대사는 원작 언어의 인용이라 백엔드가 고르는 서술 어투가 아니다.
var quoted = regexp.MustCompile(`“[^”]*”|"[^"]*"|「[^」]*」|『[^』]*』|‘[^’]*’|'[^']*'`)

var sentenceEnd = regexp.MustCompile(`[.!?…\n]+`)

var (
	hapshoEnd = regexp.MustCompile(`(습니다|습니까|십시오|세요)$`)
	gojeonEnd = regexp.MustCompile(`(더라|니라|도다|노라|로다|하매|하니|이로다)$`)
	haeraEnd  = regexp.MustCompile(`(다|라|냐|자)$`)
)

// ClassifyTone 은 서술 문장의 종결어미 분포(비율)를 낸다: hapsho(하십시오체) → gojeon(고전소설체) → haera(해라체) → other
// 순서로 문장의 마지막 어절을 분류한다. 한글이 없는 문장은 세지 않는다. 빈 결과 = 셀 문장이 없다.
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

// bieupNida: 받침 ㅂ 음절 + 니다/니까(합니다, 갑니까 — 정규식의 `ㅂ니다` 를 완성형 음절에서 본다).
func bieupNida(w string) bool {
	r := []rune(w)
	if len(r) < 3 || !(strings.HasSuffix(w, "니다") || strings.HasSuffix(w, "니까")) {
		return false
	}
	s := r[len(r)-3]
	return s >= 0xAC00 && s <= 0xD7A3 && (s-0xAC00)%28 == 17
}

// lastEojeol 은 문장의 마지막 어절에서 끝의 문장부호를 떼어 낸다. 한글이 없으면 "".
func lastEojeol(s string) string {
	f := strings.Fields(s)
	if len(f) == 0 {
		return ""
	}
	w := strings.TrimRightFunc(f[len(f)-1], func(r rune) bool { return !unicode.IsLetter(r) })
	if !strings.ContainsFunc(w, func(r rune) bool { return unicode.Is(unicode.Hangul, r) }) {
		return ""
	}
	return w
}
