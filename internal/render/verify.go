package render

import (
	"context"
	"strings"
)

// VerifyOpts: K 는 재추출 횟수, Retries 는 첫 렌더 뒤 최대 재렌더 횟수.
// Verifier "card" 이면 재추출 대신 체크리스트 채점(ScoreCard, Card 옵션), 그 밖(""·"open")은 열린 재추출.
type VerifyOpts struct {
	K, Retries int
	Verifier   string
	Card       CardOpts
}

// Passed 는 사실 기준 통과 여부. 열린 경로는 ADR-019(환각 0, 누출 0, 누락 ≤ MaxMissing), 카드 경로는 ADR-028(누출 0, 누락+종류 틀림 ≤ 1, G2 는 경고만 — Hallucinated 가 늘 비어 있다).
// render-check 의 ADR-012(누락 ≤ 1 허용)보다 열린 경로가 엄격하다. 채점이 깨졌으면(ExtractError) 통과가 아니다.
func Passed(r Render) bool {
	p := r.Report
	return p.ExtractError == "" && len(p.Hallucinated) == 0 && len(p.Leaked) == 0 && len(p.Missing) <= MaxMissing
}

// RenderVerified 는 렌더 → 재추출 → diff 를 통과할 때까지 최대 1+Retries 번 반복한다. 실패한 diff 는 다음 렌더의
// 사용자 프롬프트 끝에 피드백으로 붙는다. 상한에 닿으면 마지막 렌더를 nil 오류로 돌려준다(통과 여부는 Passed 로 본다).
// Completer 오류는 그대로 돌려준다.
func RenderVerified(ctx context.Context, c Completer, work string, s Scene, b Backend, o VerifyOpts) (Render, int, error) {
	return RenderVerified2(ctx, Completers{c, c}, work, s, b, o)
}

// Completers 는 역할별 모델: 렌더 호출은 Render, 재추출 호출은 Extract 로 간다.
type Completers struct{ Render, Extract Completer }

func RenderVerified2(ctx context.Context, cs Completers, work string, s Scene, b Backend, o VerifyOpts) (Render, int, error) {
	if o.K < 1 {
		o.K = 1
	}
	var r Render
	extra := ""
	for n := 1; ; n++ {
		var err error
		if r, err = renderWith(ctx, cs.Render, cs.Extract, work, s, b, o.K, extra, o.card()); err != nil {
			return r, n, err
		}
		if Passed(r) || n > o.Retries {
			return r, n, nil
		}
		extra = feedback(s, b.Lang, r.Report)
	}
}

// feedback 은 사람이 읽을 사실 문장·인물 이름으로 쓴 "## Fix from the previous draft" 절.
func feedback(s Scene, lang string, rep Report) string {
	var rm, inc []string
	for _, h := range rep.Hallucinated {
		rm = append(rm, describe(s, lang, strings.TrimPrefix(strings.TrimPrefix(h, "extra-act:"), "unknown-fact:")))
	}
	for _, l := range rep.Leaked {
		rm = append(rm, describe(s, lang, l)+" (withheld: the narration must not reveal it)")
	}
	for _, m := range rep.Missing {
		inc = append(inc, describe(s, lang, m))
	}
	var b strings.Builder
	b.WriteString("\n\n## Fix from the previous draft\n")
	if len(rm) > 0 {
		b.WriteString("The previous draft stated things that are not in the scene. Items to remove:\n")
		for _, x := range rm {
			b.WriteString("- " + x + "\n")
		}
	}
	if len(inc) > 0 {
		b.WriteString("The previous draft left out these acts. Items to include:\n")
		for _, x := range inc {
			b.WriteString("- " + x + "\n")
		}
	}
	return b.String()
}

// describe 는 "actor|class|fact" 키를 "이름 (class): 사실문장" 으로, 사실 ID 하나면 문장으로 풀어 쓴다. 모르면 그대로.
func describe(s Scene, lang, key string) string {
	p := strings.Split(key, "|")
	text := func(id string) string {
		if f, ok := s.FactByID(id); ok && f.Text != "" {
			return f.Text
		}
		return id
	}
	if len(p) != 3 {
		return text(key)
	}
	name := p[0]
	if e, ok := s.Entity(p[0]); ok {
		if ns := e.Names[lang]; len(ns) > 0 {
			name = ns[0]
		}
	}
	return name + " (" + p[1] + "): " + text(p[2])
}

func (o VerifyOpts) card() *CardOpts {
	if o.Verifier == "card" {
		return &o.Card
	}
	return nil
}
