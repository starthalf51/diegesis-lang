package render

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
)

// 합격 기준(ADR-012). 낮추지 않는다.
const (
	MaxMissing      = 1
	MaxHallucinated = 1
	MinJaccard      = 0.9
	MinToneShare    = 0.8
)

// Render 는 (장면, 백엔드) 하나의 렌더 → 재추출 → diff 결과. out/render/<work>/<scene>.<backend>.json 에 남는다.
type Render struct {
	Work       string             `json:"work"`
	Scene      string             `json:"scene"`
	Backend    string             `json:"backend"`
	Lang       string             `json:"lang"`
	Tone       string             `json:"tone"`
	Prose      string             `json:"prose"`
	StopReason string             `json:"stop_reason"`
	Extracted  Extracted          `json:"extracted"`
	Report     Report             `json:"report"`
	Tones      map[string]float64 `json:"tones"`
}

// RenderOne 은 백엔드 프롬프트로 렌더하고, 백엔드를 모르는 재추출 프롬프트(BuildExtractPrompt)로 다시 뽑아 diff 한다.
// Completer 오류는 그대로 돌려준다(아무것도 남기지 않으니 다시 돌리면 재시도한다).
func RenderOne(ctx context.Context, c Completer, work string, s Scene, b Backend) (Render, error) {
	r := Render{Work: work, Scene: s.ID, Backend: b.ID, Lang: b.Lang, Tone: b.Tone}
	sys, user, err := b.Prompt(s)
	if err != nil {
		return r, err
	}
	res, err := c.Complete(ctx, sys, user)
	if err != nil {
		return r, fmt.Errorf("%s/%s render: %w", s.ID, b.ID, err)
	}
	r.Prose, r.StopReason = res.Text, res.StopReason
	if b.Lang == "ko" {
		r.Tones = ClassifyTone(r.Prose)
	}
	if res.StopReason != "end_turn" {
		r.Report.ExtractError = stopped("render", res.StopReason)
		return r, nil
	}
	xs, xu := BuildExtractPrompt(s, b.Lang, r.Prose)
	xr, err := c.Complete(ctx, xs, xu)
	if err != nil {
		return r, fmt.Errorf("%s/%s extract: %w", s.ID, b.ID, err)
	}
	if xr.StopReason != "end_turn" {
		r.Report.ExtractError = stopped("extract", xr.StopReason)
	} else if e, err := ParseExtracted(xr.Text); err != nil {
		r.Report.ExtractError = err.Error()
	} else {
		r.Extracted, r.Report = e, Diff(s, e)
	}
	return r, nil
}

func stopped(what, reason string) string {
	if reason == "" {
		reason = "<empty>"
	}
	return what + " stopped: " + reason
}

// ProsePath 는 산문 파일의 dir 기준 상대 경로.
func ProsePath(work, scene, backend string) string {
	return filepath.ToSlash(filepath.Join(work, scene+"."+backend+".md"))
}

// RenderCached 는 dir/<work>/<scene>.<backend>.{md,json} 이 둘 다 있으면 다시 부르지 않는다(구독 한도). force 면 다시 한다.
// 기록(.json)을 마지막에 쓴다.
func RenderCached(ctx context.Context, c Completer, dir, work string, s Scene, b Backend, force bool) (Render, error) {
	md := filepath.Join(dir, filepath.FromSlash(ProsePath(work, s.ID, b.ID)))
	rec := strings.TrimSuffix(md, ".md") + ".json"
	if !force {
		if _, err := os.Stat(md); err == nil {
			if raw, err := os.ReadFile(rec); err == nil {
				var r Render
				if err := json.Unmarshal(raw, &r); err != nil {
					return r, fmt.Errorf("%s: %w", rec, err)
				}
				return r, nil
			}
		}
	}
	r, err := RenderOne(ctx, c, work, s, b)
	if err != nil {
		return r, err
	}
	if err := os.MkdirAll(filepath.Dir(md), 0o755); err != nil {
		return r, err
	}
	head := fmt.Sprintf("---\nwork: %s\nscene: %s\nbackend: %s\nlang: %s\nstop_reason: %s\n---\n\n", work, s.ID, b.ID, b.Lang, r.StopReason)
	if err := os.WriteFile(md, []byte(head+r.Prose+"\n"), 0o644); err != nil {
		return r, err
	}
	raw, err := json.MarshalIndent(r, "", "  ")
	if err != nil {
		return r, err
	}
	return r, os.WriteFile(rec, raw, 0o644)
}

// Row 는 렌더 하나의 판정.
type Row struct {
	Render
	Top  string   `json:"top"` // 비율 1위 어투
	Pass bool     `json:"pass"`
	Why  []string `json:"why"`
}

// SceneVerdict 는 한 장면을 모든 백엔드로 렌더한 판정. Jaccard 키는 "a~b"(백엔드 ID 이름순).
type SceneVerdict struct {
	Work, Scene string
	Rows        []Row
	Jaccard     map[string]float64
	Pass        bool
	Why         []string // 장면 단위 실패(Jaccard, 1위 어투 겹침)
}

// ContentKeys 는 재추출한 액트 중 사실을 전하는 것(입·퇴장 제외)의 키 멀티셋. Diff 가 원본과 비교하는 것과 같은 키다.
func ContentKeys(e Extracted) map[string]int {
	m := map[string]int{}
	for _, a := range e.Acts {
		if a.Fact != "" && VerbClass(a.Verb) != "presence" {
			m[actKey(a.Actor, a.Verb, a.Fact)]++
		}
	}
	return m
}

// Jaccard 는 멀티셋 Jaccard(Σmin / Σmax). 둘 다 비면 1.
func Jaccard(a, b map[string]int) float64 {
	lo, hi := 0, 0
	for k := range union(a, b) {
		lo += min(a[k], b[k])
		hi += max(a[k], b[k])
	}
	if hi == 0 {
		return 1
	}
	return float64(lo) / float64(hi)
}

func union(a, b map[string]int) map[string]bool {
	u := map[string]bool{}
	for k := range a {
		u[k] = true
	}
	for k := range b {
		u[k] = true
	}
	return u
}

func topTone(t map[string]float64) string {
	best := ""
	for k, v := range t {
		if best == "" || v > t[best] || (v == t[best] && k < best) {
			best = k
		}
	}
	return best
}

// Judge: 내용 동일(각 렌더 missing ≤ 1, hallucinated ≤ 1, leaked = 0, 백엔드끼리 Jaccard ≥ 0.9)과
// 어투 상이(어투가 있는 백엔드는 목표 비율 ≥ 0.8, 그들끼리 1위 어투가 모두 다르다).
func Judge(work, scene string, rs []Render) SceneVerdict {
	v := SceneVerdict{Work: work, Scene: scene, Jaccard: map[string]float64{}, Pass: true}
	sort.Slice(rs, func(i, j int) bool { return rs[i].Backend < rs[j].Backend })
	tops := map[string]string{}
	for _, r := range rs {
		row := Row{Render: r, Top: topTone(r.Tones), Pass: true}
		bad := func(f string, a ...any) { row.Pass = false; row.Why = append(row.Why, fmt.Sprintf(f, a...)) }
		rep := r.Report
		if rep.ExtractError != "" {
			bad("extract error: %s", rep.ExtractError)
		}
		if len(rep.Missing) > MaxMissing {
			bad("missing %d > %d: %s", len(rep.Missing), MaxMissing, strings.Join(rep.Missing, "; "))
		}
		if len(rep.Hallucinated) > MaxHallucinated {
			bad("hallucinated %d > %d: %s", len(rep.Hallucinated), MaxHallucinated, strings.Join(rep.Hallucinated, "; "))
		}
		if len(rep.Leaked) > 0 {
			bad("leaked: %s", strings.Join(rep.Leaked, "; "))
		}
		if r.Tone != "" {
			if r.Tones[r.Tone] < MinToneShare {
				bad("%s share %.2f < %.2f", r.Tone, r.Tones[r.Tone], MinToneShare)
			}
			if other, dup := tops[row.Top]; dup {
				v.Why = append(v.Why, fmt.Sprintf("%s and %s have the same top tone %q", other, r.Backend, row.Top))
			}
			tops[row.Top] = r.Backend
		}
		v.Pass = v.Pass && row.Pass
		v.Rows = append(v.Rows, row)
	}
	for i := range rs {
		for j := i + 1; j < len(rs); j++ {
			k := rs[i].Backend + "~" + rs[j].Backend
			v.Jaccard[k] = Jaccard(ContentKeys(rs[i].Extracted), ContentKeys(rs[j].Extracted))
			if v.Jaccard[k] < MinJaccard {
				v.Why = append(v.Why, fmt.Sprintf("jaccard %s = %.2f < %.2f", k, v.Jaccard[k], MinJaccard))
			}
		}
	}
	v.Pass = v.Pass && len(v.Why) == 0
	return v
}
