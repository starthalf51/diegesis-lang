package narrate

import (
	"context"
	"strings"

	"diegesis/internal/render"
)

// toneTarget: 한국어 목표 어투 지시 (render 의 toneRule 은 비공개라 짧게 따로 둔다).
var toneTarget = map[string]string{
	"haera":  "따옴표 밖의 모든 서술 문장을 해라체 '-다'(했다, 있다)로 끝낸다.",
	"hapsho": "따옴표 밖의 모든 서술 문장을 하십시오체 '-습니다/-ㅂ니다'(했습니다, 입니다)로 끝낸다.",
	"gojeon": "따옴표 밖의 모든 서술 문장을 고전소설체 '-더라', '-니라', '-도다'(하더라, 이더라)로 끝낸다. 철자는 현대 맞춤법을 따른다.",
}

// RestylePrompt: 어투만 바꾸는 프롬프트. 문장 수·대사 문구·사실은 보존한다.
func RestylePrompt(text string, b render.Backend) (system, user string) {
	system = "너는 소설 문체를 바꾼다. 어투와 말씨만 바꾼다. 문장 수, 따옴표 안 대사의 문구, 사실, 사건 순서는 그대로 둔다. " +
		"새 사실이나 새 장면을 더하지 않고 빼지도 않는다. 본문만 출력한다."
	if b.Lang == "ko" {
		if r := toneTarget[b.Tone]; r != "" {
			system += "\n목표 어투(반드시 지킨다): " + r
		}
	}
	if b.Sample.Text != "" {
		system += "\n\n## 문체 견본 (어투와 말씨만 참고한다. 내용은 가져오지 않는다)\n" + b.Sample.Text
	}
	return system, "## 바꿀 글\n" + text
}

func toneOK(text string, b render.Backend) bool {
	if b.Lang != "ko" || b.Tone == "" {
		return true
	}
	top, share := "", 0.0
	for k, v := range render.ClassifyTone(text) {
		if v > share {
			top, share = k, v
		}
	}
	return top == b.Tone && share >= render.MinToneShare
}

// Restyle: 어투 검사 통과 여부를 함께 돌려준다. 실패하면 1회 재시도, 그래도 실패면 (원문, false).
// 잘린 출력(end_turn 아님)도 실패 시도다.
func Restyle(ctx context.Context, c render.Completer, text string, b render.Backend) (string, bool, error) {
	sys, user := RestylePrompt(text, b)
	for range 2 {
		r, err := c.Complete(ctx, sys, user)
		if err != nil {
			return "", false, err
		}
		if out := strings.TrimSpace(r.Text); r.StopReason == "end_turn" && toneOK(out, b) {
			return out, true, nil
		}
	}
	return text, false, nil
}
