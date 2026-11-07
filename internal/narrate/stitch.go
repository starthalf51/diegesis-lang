package narrate

import (
	"context"
	"strings"

	"diegesis/internal/render"
)

// StitchPrompt: 비트 사이 이음새만 다듬는 프롬프트. parts 는 빈 줄로 이어 붙인다(파이프라인은 문체를 바꾼 장면 글 하나를 넘긴다).
func StitchPrompt(parts []string, lang string) (system, user string) {
	system = "너는 이어 붙인 소설 비트들의 이음새만 다듬는다. 비트 사이의 접속과 전환, 반복되는 표현만 손본다. " +
		"새 사실을 더하지 않는다. 새 대사를 더하지 않는다. 기존 대사의 문구를 고치지 않는다. 행위를 빼거나 순서를 바꾸지 않는다. " +
		"어투는 그대로 둔다. 본문 전체만 출력한다."
	if lang == "en" {
		system += " 영어 글은 영어로 출력한다."
	}
	return system, "## 비트\n" + strings.Join(parts, "\n\n")
}

func Stitch(ctx context.Context, c render.Completer, parts []string, lang string) (string, error) {
	sys, user := StitchPrompt(parts, lang)
	r, err := c.Complete(ctx, sys, user)
	if err != nil {
		return "", err
	}
	return strings.TrimSpace(r.Text), nil
}
