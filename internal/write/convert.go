package write

import (
	"context"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"regexp"
	"strings"
)

const maxConvertRunes = 12000

// blankLine separates paragraphs: a line with only spaces or tabs counts as blank.
var blankLine = regexp.MustCompile(`\n[ \t]*\n`)

// ConvertIdea turns a public-domain story text into a write.Loop idea: a Korean instruction followed by the
// text with its paragraphs numbered ¶1, ¶2, … and returns the paragraph count. Leading `#` header lines
// (tools/fetchtext) are dropped.
func ConvertIdea(title, text string) (string, int, error) {
	lines := strings.Split(strings.ReplaceAll(text, "\r\n", "\n"), "\n")
	for len(lines) > 0 && strings.HasPrefix(lines[0], "#") {
		lines = lines[1:]
	}
	body := strings.TrimSpace(strings.Join(lines, "\n"))
	if n := len([]rune(body)); n > maxConvertRunes {
		return "", 0, fmt.Errorf("convert: %d characters, too long for one pass (max %d)", n, maxConvertRunes)
	}
	var sb strings.Builder
	fmt.Fprintf(&sb, "아래 공개 도메인 단편소설 「%s」를 디제시스 코드로 충실하게 변환하라. "+
		"원문에 없는 사건이나 인물을 만들지 말고, 장면은 원문 순서대로 둔다. "+
		"각 행위 줄 끝에 해당 문단 번호를 `src <문단번호>` 로 붙인다(예: `src 12`).\n\n"+
		"변환 규칙:\n"+
		"- 문단을 건너뛰지 말고 원문 사건마다 행위 한 줄을 쓴다. 장면을 줄여서 오류를 피하지 않는다.\n"+
		"- cast 파일 맨 앞에 `work <id> \"%s\"` 선언이 반드시 있어야 한다(시간 기준). 모든 장소는 cast 에 `place` 로 선언한다.\n"+
		"- 장면 머리말에 `when:` 은 쓰지 않는다(장면 순서로 충분하다).\n"+
		"- 죽음은 `dies <id>` 로 쓴다(`<id> dies` 가 아니다). 동사는 참고 문서에 있는 것만 쓴다.\n\n"+
		"## 원문\n", title, title)
	n := 0
	for _, p := range blankLine.Split(body, -1) {
		if p = strings.TrimSpace(p); p != "" {
			n++
			fmt.Fprintf(&sb, "\n¶%d %s\n", n, p)
		}
	}
	return sb.String(), n, nil
}

var srcRef = regexp.MustCompile(`\bsrc (\d+)\b`)

// SrcCoverage counts the distinct source paragraphs (`src N`) cited by the .dieg.md files under dir.
func SrcCoverage(dir string) int {
	seen := map[string]bool{}
	filepath.WalkDir(dir, func(p string, d fs.DirEntry, err error) error {
		if err == nil && !d.IsDir() && strings.HasSuffix(p, ".dieg.md") {
			b, _ := os.ReadFile(p)
			for _, m := range srcRef.FindAllStringSubmatch(string(b), -1) {
				seen[m[1]] = true
			}
		}
		return nil
	})
	return len(seen)
}

// CoverageChecker adds a "coverage" error to check's diagnostics when the code cites fewer than a fifth of
// the source's paragraphs, so a round cannot pass by dropping scenes.
func CoverageChecker(check Checker, paragraphs int) Checker {
	return func(ctx context.Context, dir string) ([]Diag, error) {
		ds, err := check(ctx, dir)
		if err != nil {
			return ds, err
		}
		if n := SrcCoverage(dir); n*5 < paragraphs {
			ds = append(ds, Diag{Severity: "error", Rule: "coverage",
				Msg: fmt.Sprintf("code cites %d of %d source paragraphs (src); convert every event, at least a fifth of the paragraphs", n, paragraphs)})
		}
		return ds, nil
	}
}
