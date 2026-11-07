package narrate

import (
	"context"
	"fmt"
	"strconv"
	"strings"

	"diegesis/internal/render"
)

// Roles: 층별 모델. Beat·Style·Stitch 는 쓰고, Verify 는 비트 검증과 최종 대조를 한다.
type Roles struct{ Beat, Verify, Style, Stitch render.Completer }

type Opts struct {
	Plan        PlanOpts
	BeatRetries int
	Stitch      bool
}

func DefaultOpts() Opts { return Opts{Plan: PlanOpts{2500, 3}, BeatRetries: 2, Stitch: true} }

// LayerStats: 비트 번호는 1부터다(호출 라벨 b<i> 와 같다). BeatAttempts[i] 는 비트 i+1 의 생성 시도 수.
type LayerStats struct {
	Beats        int    `json:"beats"`
	BeatAttempts []int  `json:"beat_attempts"`
	BeatFailed   []int  `json:"beat_failed"`
	StyleOK      bool   `json:"style_ok"`
	Stitched     bool   `json:"stitched"`
	FinalFrom    string `json:"final_from"` // "stitched" | "unstitched" | "plain"(문체 전)
}

func lab(c render.Completer, l string) render.Completer {
	if m, ok := c.(*render.Meter); ok {
		return m.Label(l)
	}
	return c
}

// attemptLabel: bump 이면 부를 때마다 시도 번호를 올리고(생성), 아니면 같은 번호를 쓴다(검증).
type attemptLabel struct {
	c      render.Completer
	prefix string
	n      *int
	bump   bool
}

func (a attemptLabel) Complete(ctx context.Context, system, user string) (render.Result, error) {
	if a.bump {
		*a.n++
	}
	return lab(a.c, a.prefix+"-a"+strconv.Itoa(*a.n)).Complete(ctx, system, user)
}

func lastPara(t string) string {
	ps := strings.Split(strings.TrimSpace(t), "\n\n")
	return strings.TrimSpace(ps[len(ps)-1])
}

// RenderLayered: [1] 설계 → [2] 비트마다 생성·검증·재시도 → [3] 문체 → [4] 연결 → 장면 전체 재추출 1회로 최종 대조.
// 실패한 비트도 장면에 넣고 BeatFailed 에 남긴다. 문체·연결 호출이 실패하면 그 층을 건너뛴다.
// 연결한 글이 대조를 통과하지 못하면 연결 전 글로, 그것도 못하면(문체가 글을 바꿨을 때) 문체 전 글로 다시 대조해 가장 나은 쪽을 쓴다(FinalFrom).
// 오류는 설계 실패와 비트 생성·검증·최종 대조의 Completer 오류뿐이다.
func RenderLayered(ctx context.Context, r Roles, work string, s render.Scene, b render.Backend, o Opts) (render.Render, LayerStats, error) {
	texts, st, err := Beats(ctx, r, s, b, o)
	if err != nil {
		return render.Render{}, st, err
	}
	return Finish(ctx, r, work, s, b, o, texts, st)
}

// Beats 는 [1]·[2] 만 돈다. 비트 글은 b.Lang 의 중립 문체라 같은 언어의 다른 백엔드로 Finish 에 다시 넘길 수 있다(문체 교체).
func Beats(ctx context.Context, r Roles, s render.Scene, b render.Backend, o Opts) ([]string, LayerStats, error) {
	p, err := PlanScene(s, o.Plan)
	if err != nil {
		return nil, LayerStats{}, err
	}
	st := LayerStats{Beats: len(p.Beats), BeatAttempts: []int{}, BeatFailed: []int{}}
	var texts []string
	prev := ""
	for i := range p.Beats {
		n, id := 0, s.ID+"-b"+strconv.Itoa(i+1)
		text, rep, att, err := realizeWithRetry(ctx, attemptLabel{r.Beat, "beat-" + id, &n, true},
			attemptLabel{r.Verify, "bverify-" + id, &n, false}, p, i, b.Lang, prev, o.BeatRetries)
		if err != nil {
			return nil, st, fmt.Errorf("%s beat %d: %w", s.ID, i+1, err)
		}
		st.BeatAttempts = append(st.BeatAttempts, att)
		if !BeatPassed(rep) {
			st.BeatFailed = append(st.BeatFailed, i+1)
		}
		if text != "" {
			texts = append(texts, text)
			prev = lastPara(text)
		}
	}
	return texts, st, nil
}

// Finish 는 Beats 의 글로 [3] 문체 → [4] 연결 → 최종 대조를 돈다. st 는 Beats 가 돌려준 것이다.
func Finish(ctx context.Context, r Roles, work string, s render.Scene, b render.Backend, o Opts, texts []string, st LayerStats) (render.Render, LayerStats, error) {
	plain := strings.Join(texts, "\n\n")

	styled, ok, err := Restyle(ctx, lab(r.Style, "style-"+s.ID), plain, b)
	if err != nil || styled == "" {
		styled, ok = plain, false
	}
	st.StyleOK = ok

	// 후보를 바깥 층부터 대조한다: 연결본 → 연결 전(문체본) → 문체 전(비트 원문). 통과하면 멈추고, 아니면 가장 나은 것.
	type cand struct{ text, from string }
	var cs []cand
	if o.Stitch && len(texts) > 1 {
		if t, err := Stitch(ctx, lab(r.Stitch, "stitch-"+s.ID), []string{styled}, b.Lang); err == nil && t != "" {
			cs, st.Stitched = append(cs, cand{t, "stitched"}), true
		}
	}
	cs = append(cs, cand{styled, "unstitched"})
	if styled != plain {
		cs = append(cs, cand{plain, "plain"})
	}
	var res render.Render
	for k, c := range cs {
		alt, err := finalCheck(ctx, r.Verify, work, s, b, c.text)
		if err != nil {
			return alt, st, err
		}
		if k == 0 || badness(alt) < badness(res) {
			res, st.FinalFrom = alt, c.from
		}
		if render.Passed(res) {
			break
		}
	}
	return res, st, nil
}

// finalCheck: 장면 전체를 1회 재추출해 Report·Tones 를 채운다. 깨진 답은 ExtractError 다.
func finalCheck(ctx context.Context, c render.Completer, work string, s render.Scene, b render.Backend, prose string) (render.Render, error) {
	r := render.Render{Work: work, Scene: s.ID, Backend: b.ID, Lang: b.Lang, Tone: b.Tone, Prose: prose}
	if b.Lang == "ko" {
		r.Tones = render.ClassifyTone(prose)
	}
	e, xs, err := render.ExtractK(ctx, lab(c, "final-"+s.ID), s, b.Lang, prose, 1)
	r.Extractions = xs
	switch {
	case render.IsBadExtract(err):
		r.Report.ExtractError = err.Error()
	case err != nil:
		return r, fmt.Errorf("%s final: %w", s.ID, err)
	default:
		r.Extracted, r.Report = e, render.Diff(s, e)
	}
	return r, nil
}

// badness: 깨진 재추출 > 환각·누출 > 누락 순으로 나쁘다. 통과한 결과는 실패한 결과보다 항상 작다.
func badness(r render.Render) int {
	if r.Report.ExtractError != "" {
		return 1 << 30
	}
	return 1000*(len(r.Report.Hallucinated)+len(r.Report.Leaked)) + len(r.Report.Missing)
}
