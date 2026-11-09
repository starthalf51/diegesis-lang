package write

import (
	"fmt"
	"strings"
)

const maxConvertRunes = 12000

// ConvertIdea turns a public-domain story text into a write.Loop idea: a Korean instruction followed by the
// text with its paragraphs numbered ¶1, ¶2, … Leading `#` header lines (tools/fetchtext) are dropped.
func ConvertIdea(title, text string) (string, error) {
	lines := strings.Split(strings.ReplaceAll(text, "\r\n", "\n"), "\n")
	for len(lines) > 0 && strings.HasPrefix(lines[0], "#") {
		lines = lines[1:]
	}
	body := strings.TrimSpace(strings.Join(lines, "\n"))
	if n := len([]rune(body)); n > maxConvertRunes {
		return "", fmt.Errorf("convert: %d characters, too long for one pass (max %d)", n, maxConvertRunes)
	}
	var sb strings.Builder
	fmt.Fprintf(&sb, "아래 공개 도메인 단편소설 「%s」를 디제시스 코드로 충실하게 변환하라. "+
		"원문에 없는 사건이나 인물을 만들지 말고, 장면은 원문 순서대로 둔다. "+
		"각 행위 줄 끝에 해당 문단 번호를 `src <문단번호>` 로 붙인다(예: `src 12`).\n\n## 원문\n", title)
	n := 0
	for _, p := range strings.Split(body, "\n\n") {
		if p = strings.TrimSpace(p); p != "" {
			n++
			fmt.Fprintf(&sb, "\n¶%d %s\n", n, p)
		}
	}
	return sb.String(), nil
}
