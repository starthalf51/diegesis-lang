package narrate

import (
	"context"
	"fmt"
	"strings"

	"diegesis/internal/render"
)

var formGuide = map[Form]string{
	FormDialogue:   "따옴표 대사로 쓴다",
	FormThought:    "내면 독백으로 쓴다",
	FormPerception: "인물이 보거나 듣는 것으로 쓴다",
	FormAction:     "행동 서술로 쓴다",
	FormNarration:  "간접화법 요약으로 쓴다",
}

func name(s render.Scene, lang, id string) string {
	if e, ok := s.Entity(id); ok && len(e.Names[lang]) > 0 {
		return e.Names[lang][0]
	}
	return id
}

// BeatPrompt 는 비트 i 하나를 중립 문체로 쓰게 하는 프롬프트. fix 가 있으면 고칠 것 절을 붙인다.
func BeatPrompt(p Plan, i int, lang, prevTail string, fix *render.Report) (system, user string) {
	b := p.Beats[i]
	style := "해라체 평서문"
	if lang == "en" {
		style = "plain past tense"
	}
	system = fmt.Sprintf("너는 소설의 한 비트를 쓴다. 이 비트의 행위만 순서대로 쓴다. 문체는 중립 평서(%s)로 쓰고, 문단은 %d개로 쓴다. 본문만 출력한다.", style, b.Paragraphs)
	var u strings.Builder
	u.WriteString("## 이 비트의 행위\n")
	for _, a := range b.Acts {
		who := name(p.Scene, lang, a.Act.Actor)
		fmt.Fprintf(&u, "- [%s] %s %s: %s", a.Form, who, a.Act.Verb, formGuide[a.Form])
		if a.FactText != "" {
			fmt.Fprintf(&u, " — %s", a.FactText)
		}
		if a.Act.Quote != "" {
			fmt.Fprintf(&u, " (대사: %q)", a.Act.Quote)
		}
		if a.Modality != ModFact {
			fmt.Fprintf(&u, " (%s: 인물의 말이나 생각으로만 쓰고 서술자가 사실로 단정하지 않는다)", a.Modality)
		}
		u.WriteString("\n")
	}
	if len(b.Forbidden) > 0 {
		u.WriteString("\n## 다음은 쓰지도 암시하지도 않는다\n")
		for _, f := range b.Forbidden {
			u.WriteString("- " + f + "\n")
		}
	}
	if len(b.Allowed) > 0 {
		u.WriteString("\n## 덧붙여도 되는 것은 이것뿐이다\n")
		for _, f := range b.Allowed {
			u.WriteString("- " + f + "\n")
		}
	}
	if prevTail != "" {
		u.WriteString("\n## 직전 문단 (흐름용, 다시 쓰지 않는다)\n" + prevTail + "\n")
	}
	if fix != nil {
		u.WriteString(beatFeedback(p.Scene, lang, *fix))
	}
	return system, u.String()
}

// beatFeedback: render.feedback 과 같은 내용을 한국어로. 이름·사실 문장으로 풀어 쓴다.
func beatFeedback(s render.Scene, lang string, r render.Report) string {
	var rm, inc []string
	for _, h := range r.Hallucinated {
		rm = append(rm, describeKey(s, lang, strings.TrimPrefix(strings.TrimPrefix(h, "extra-act:"), "unknown-fact:")))
	}
	for _, l := range r.Leaked {
		rm = append(rm, describeKey(s, lang, l)+" (숨김: 드러내면 안 된다)")
	}
	for _, m := range r.Missing {
		inc = append(inc, describeKey(s, lang, m))
	}
	var b strings.Builder
	b.WriteString("\n## 직전 시도에서 고칠 것\n")
	if len(rm) > 0 {
		b.WriteString("지울 것 (이 비트에 없는 내용을 썼다):\n")
		for _, x := range rm {
			b.WriteString("- " + x + "\n")
		}
	}
	if len(inc) > 0 {
		b.WriteString("넣을 것 (빠뜨린 행위):\n")
		for _, x := range inc {
			b.WriteString("- " + x + "\n")
		}
	}
	if r.ExtractError != "" {
		b.WriteString("재추출에 실패했다. 더 분명하게 쓴다.\n")
	}
	return b.String()
}

// describeKey: "actor|class|fact" → "이름 (class): 사실문장", 사실 ID 하나면 문장. 모르면 그대로.
func describeKey(s render.Scene, lang, key string) string {
	text := func(id string) string {
		if f, ok := s.FactByID(id); ok && f.Text != "" {
			return f.Text
		}
		return id
	}
	p := strings.Split(key, "|")
	if len(p) != 3 {
		return text(key)
	}
	return name(s, lang, p[0]) + " (" + p[1] + "): " + text(p[2])
}

func RealizeBeat(ctx context.Context, c render.Completer, p Plan, i int, lang, prevTail string, fix *render.Report) (string, error) {
	sys, user := BeatPrompt(p, i, lang, prevTail, fix)
	r, err := c.Complete(ctx, sys, user)
	if err != nil {
		return "", err
	}
	return strings.TrimSpace(r.Text), nil
}

// VerifyBeat: 비트 소장면으로 재추출 1회(K=1) 후 Diff. 파싱 실패는 ExtractError 로 보고한다.
func VerifyBeat(ctx context.Context, c render.Completer, p Plan, i int, lang, text string) (render.Report, error) {
	bs := p.BeatScene(i)
	sys, user := render.BuildExtractPrompt(bs, lang, text)
	r, err := c.Complete(ctx, sys, user)
	if err != nil {
		return render.Report{}, err
	}
	e, err := render.ParseExtracted(r.Text)
	if err != nil {
		return render.Report{ExtractError: err.Error()}, nil
	}
	return render.Diff(bs, e), nil
}

// BeatPassed: 비트는 행위 ≤ 3개라 누락도 허용하지 않는다. 장면 단위 허용(≤1)은 최종 대조가 맡는다.
func BeatPassed(r render.Report) bool {
	return len(r.Hallucinated) == 0 && len(r.Leaked) == 0 && len(r.Missing) == 0 && r.ExtractError == ""
}

// realizeWithRetry: 통과하거나 1+retries 번 시도할 때까지. 상한을 넘기면 마지막 결과를 돌려준다(오류 아님).
func realizeWithRetry(ctx context.Context, gen, ver render.Completer, p Plan, i int, lang, prevTail string, retries int) (text string, rep render.Report, attempts int, err error) {
	var fix *render.Report
	for attempts < 1+retries {
		attempts++
		if text, err = RealizeBeat(ctx, gen, p, i, lang, prevTail, fix); err != nil {
			return
		}
		if rep, err = VerifyBeat(ctx, ver, p, i, lang, text); err != nil {
			return
		}
		if BeatPassed(rep) {
			return
		}
		r := rep
		fix = &r
	}
	return
}
