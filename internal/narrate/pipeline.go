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
	FinalFrom    string `json:"final_from"` // "stitched" | "unstitched"
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
// 연결한 글이 대조를 통과하지 못하면 연결 전 글로 다시 대조해 나은 쪽을 쓴다(FinalFrom).
// 오류는 설계 실패와 비트 생성·검증·최종 대조의 Completer 오류뿐이다.
func RenderLayered(ctx context.Context, r Roles, work string, s render.Scene, b render.Backend, o Opts) (render.Render, LayerStats, error) {
	p, err := PlanScene(s, o.Plan)
	if err != nil {
		return render.Render{}, LayerStats{}, err
	}
	st := LayerStats{Beats: len(p.Beats)}
	var texts []string
	prev := ""
	for i := range p.Beats {
		n, id := 0, s.ID+"-b"+strconv.Itoa(i+1)
		text, rep, att, err := realizeWithRetry(ctx, attemptLabel{r.Beat, "beat-" + id, &n, true},
			attemptLabel{r.Verify, "bverify-" + id, &n, false}, p, i, b.Lang, prev, o.BeatRetries)
		if err != nil {
			return render.Render{}, st, fmt.Errorf("%s beat %d: %w", s.ID, i+1, err)
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
	plain := strings.Join(texts, "\n\n")

	styled, ok, err := Restyle(ctx, lab(r.Style, "style-"+s.ID), plain, b)
	if err != nil || styled == "" {
		styled, ok = plain, false
	}
	st.StyleOK = ok

	cand, from := styled, "unstitched"
	if o.Stitch && len(texts) > 1 {
		if t, err := Stitch(ctx, lab(r.Stitch, "stitch-"+s.ID), []string{styled}, b.Lang); err == nil && t != "" {
			cand, from, st.Stitched = t, "stitched", true
		}
	}
	res, err := finalCheck(ctx, r.Verify, work, s, b, cand)
	if err != nil {
		return res, st, err
	}
	if st.Stitched && !render.Passed(res) {
		alt, err := finalCheck(ctx, r.Verify, work, s, b, styled)
		if err != nil {
			return res, st, err
		}
		if badness(alt) < badness(res) {
			res, from = alt, "unstitched"
		}
	}
	st.FinalFrom = from
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
