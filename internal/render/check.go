package render

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
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
// Report·Tones 는 기록 당시 값(참고용)이다. Judge 는 Score 로 산문·재추출에서 다시 센다.
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
	PromptHash string             `json:"prompt_hash"` // PromptHash(장면, 백엔드). 다르면 캐시를 버린다
}

// PromptHash 는 렌더 시스템·사용자 프롬프트와 (산문을 뺀) 재추출 프롬프트의 sha256. 장면·백엔드·프롬프트 코드 중
// 하나라도 바뀌면 달라진다.
func PromptHash(s Scene, b Backend) (string, error) {
	sys, user, err := b.Prompt(s)
	if err != nil {
		return "", err
	}
	xs, xu := BuildExtractPrompt(s, b.Lang, "")
	h := sha256.Sum256([]byte(sys + "\x00" + user + "\x00" + xs + "\x00" + xu))
	return hex.EncodeToString(h[:]), nil
}

// Score 는 결정적인 판정 입력(어투 비율, diff)을 산문·재추출에서 다시 센다. 렌더·재추출 오류(ExtractError)는 남긴다.
func Score(s Scene, r Render) Render {
	r.Tones = nil
	if r.Lang == "ko" {
		r.Tones = ClassifyTone(r.Prose)
	}
	if r.Report.ExtractError == "" {
		r.Report = Diff(s, r.Extracted)
	}
	return r
}

// RenderOne 은 백엔드 프롬프트로 렌더하고, 백엔드를 모르는 재추출 프롬프트(BuildExtractPrompt)로 다시 뽑아 diff 한다.
// Completer 오류는 그대로 돌려준다(아무것도 남기지 않으니 다시 돌리면 재시도한다).
func RenderOne(ctx context.Context, c Completer, work string, s Scene, b Backend) (Render, error) {
	r := Render{Work: work, Scene: s.ID, Backend: b.ID, Lang: b.Lang, Tone: b.Tone}
	sys, user, err := b.Prompt(s)
	if err != nil {
		return r, err
	}
	if r.PromptHash, err = PromptHash(s, b); err != nil {
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

// RenderCached 는 dir/<work>/<scene>.<backend>.{md,json} 이 둘 다 있고 기록의 prompt_hash 가 지금 프롬프트와 같으면
// 다시 부르지 않는다(구독 한도). force 면 다시 한다. 기록(.json)을 마지막에 쓴다.
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
				h, err := PromptHash(s, b)
				if err != nil {
					return r, err
				}
				if r.PromptHash == h {
					return r, nil
				}
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
// Jaccard = 사실을 전하는 비입퇴장 액트(판정), JaccardAll = 모든 액트(참고, 판정에 쓰지 않는다).
type SceneVerdict struct {
	Work, Scene string
	Rows        []Row
	Jaccard     map[string]float64
	JaccardAll  map[string]float64
	Pass        bool
	Why         []string // 장면 단위 실패(Jaccard, 1위 어투 겹침)
}

// ContentKeys 는 재추출한 액트 중 사실을 전하는 것(입·퇴장 제외)의 키 멀티셋. Diff 가 원본과 비교하는 것과 같은 키
// (사실은 정규 명제)다. all 이면 모든 액트를 센다(참고용).
func ContentKeys(s Scene, e Extracted, all bool) map[string]int {
	m := map[string]int{}
	for _, a := range e.Acts {
		if all || a.Fact != "" && VerbClass(a.Verb) != "presence" {
			m[actKey(a.Actor, a.Verb, s.Canon(a.Fact))]++
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
// 기록된 Report·Tones 는 쓰지 않고 Score 로 다시 센다.
func Judge(work string, s Scene, rs []Render) SceneVerdict {
	v := SceneVerdict{Work: work, Scene: s.ID, Jaccard: map[string]float64{}, JaccardAll: map[string]float64{}, Pass: true}
	rs = append([]Render{}, rs...)
	for i := range rs {
		rs[i] = Score(s, rs[i])
	}
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
			v.Jaccard[k] = Jaccard(ContentKeys(s, rs[i].Extracted, false), ContentKeys(s, rs[j].Extracted, false))
			v.JaccardAll[k] = Jaccard(ContentKeys(s, rs[i].Extracted, true), ContentKeys(s, rs[j].Extracted, true))
			if v.Jaccard[k] < MinJaccard {
				v.Why = append(v.Why, fmt.Sprintf("jaccard %s = %.2f < %.2f", k, v.Jaccard[k], MinJaccard))
			}
		}
	}
	v.Pass = v.Pass && len(v.Why) == 0
	return v
}
