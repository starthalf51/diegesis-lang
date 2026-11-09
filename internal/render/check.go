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

// Acceptance criteria (ADR-012). Not lowered.
const (
	MaxMissing      = 1
	MaxHallucinated = 1
	MinJaccard      = 0.9
	MinToneShare    = 0.8
)

// Render is the result of render → k re-extractions → majority vote → diff for one (scene, backend). Stored in out/render/<work>/<scene>.<backend>.json.
// Extracted, Report and Tones are the values at recording time (for reference). Judge recounts from Score using the prose and the original re-extractions (Extractions).
type Render struct {
	Work        string             `json:"work"`
	Scene       string             `json:"scene"`
	Backend     string             `json:"backend"`
	Lang        string             `json:"lang"`
	Tone        string             `json:"tone"`
	Prose       string             `json:"prose"`
	StopReason  string             `json:"stop_reason"`
	Extracted   Extracted          `json:"extracted"`   // Vote(Extractions)
	Extractions []Extracted        `json:"extractions"` // DefaultK original re-extractions
	Report      Report             `json:"report"`
	Tones       map[string]float64 `json:"tones"`
	RenderHash  string             `json:"render_hash"`  // RenderHash(scene, backend). If different, render again
	ExtractHash string             `json:"extract_hash"` // ExtractHash(scene, language). If different, keep the prose and extract again
}

func sum(parts ...string) string {
	h := sha256.Sum256([]byte(strings.Join(parts, "\x00")))
	return hex.EncodeToString(h[:])
}

// RenderHash is the sha256 of the render system and user prompts. It changes if any of the scene, backend or render prompt code changes.
func RenderHash(s Scene, b Backend) (string, error) {
	sys, user, err := b.Prompt(s)
	if err != nil {
		return "", err
	}
	return sum(sys, user), nil
}

// ExtractHash is the sha256 of the re-extraction prompt (without prose) and the count.
func ExtractHash(s Scene, lang string) string {
	xs, xu := BuildExtractPrompt(s, lang, "")
	return sum(xs, xu, fmt.Sprint(DefaultK))
}

// Score recounts the deterministic judgment inputs (tone ratio, majority vote, diff) from the prose and the original re-extractions. Render and re-extraction errors (ExtractError) are kept.
func Score(s Scene, r Render) Render {
	r.Tones = nil
	if r.Lang == "ko" {
		r.Tones = ClassifyTone(r.Prose)
	}
	if r.Report.ExtractError == "" {
		if len(r.Extractions) > 0 {
			r.Extracted = Vote(s, r.Extractions)
		}
		r.Report = Diff(s, r.Extracted)
	}
	return r
}

// RenderOne renders with the backend prompt, extracts again DefaultK times with a backend-agnostic re-extraction prompt (BuildExtractPrompt),
// and diffs by majority vote. A Completer error is returned as is (nothing is recorded, so rerunning retries).
func RenderOne(ctx context.Context, c Completer, work string, s Scene, b Backend) (Render, error) {
	return renderWith(ctx, c, c, work, s, b, DefaultK, "", "", nil)
}

// renderWith is the body of RenderOne. It re-extracts k times, and if extra is given, appends it to the end of the user prompt and renders
// (RenderHash is based on the prompt without extra, so the cache key does not change).
func renderWith(ctx context.Context, c, x Completer, work string, s Scene, b Backend, k int, suffix, extra string, card *CardOpts) (Render, error) {
	r := Render{Work: work, Scene: s.ID, Backend: b.ID, Lang: b.Lang, Tone: b.Tone}
	sys, user, err := b.Prompt(s)
	if err != nil {
		return r, err
	}
	r.RenderHash = sum(sys, user)
	user += suffix + extra
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
	if card != nil { // card mode: leave the re-extraction fields (Extracted, Extractions, ExtractHash) empty
		rep, err := ScoreCard(ctx, x, s, b, r.Prose, *card)
		r.Report = rep
		if err != nil {
			return r, fmt.Errorf("%s/%s card: %w", s.ID, b.ID, err)
		}
		return r, nil
	}
	return r, reextract(ctx, x, s, &r, k)
}

// reextract re-extracts r.Prose and refills r's re-extraction and judgment fields. If the answer is broken it is left as ExtractError.
func reextract(ctx context.Context, c Completer, s Scene, r *Render, k int) error {
	r.ExtractHash = ExtractHash(s, r.Lang)
	r.Extracted, r.Extractions, r.Report = Extracted{}, nil, Report{}
	e, xs, err := ExtractK(ctx, c, s, r.Lang, r.Prose, k)
	r.Extractions = xs
	switch {
	case IsBadExtract(err):
		r.Report.ExtractError = err.Error()
	case err != nil:
		return fmt.Errorf("%s/%s %w", s.ID, r.Backend, err)
	default:
		r.Extracted, r.Report = e, Diff(s, e)
	}
	return nil
}

func stopped(what, reason string) string {
	if reason == "" {
		reason = "<empty>"
	}
	return what + " stopped: " + reason
}

// ProsePath is the prose file's path relative to dir.
func ProsePath(work, scene, backend string) string {
	return filepath.ToSlash(filepath.Join(work, scene+"."+backend+".md"))
}

// Force is the scope in which to ignore the cache. Render means render again (and re-extract), Extract means keep the prose cache and only re-extract.
type Force struct{ Render, Extract bool }

// RenderCached skips calling for prose again (subscription limit) if both dir/<work>/<scene>.<backend>.{md,json} exist and the record's render_hash equals the current render prompt.
// Re-extraction happens again if extract_hash differs, there are not DefaultK of them, it is broken, or f.Extract is set.
// A record where the render stopped is left alone until f.Render. The record (.json) is written last.
func RenderCached(ctx context.Context, c Completer, dir, work string, s Scene, b Backend, f Force) (Render, error) {
	md := filepath.Join(dir, filepath.FromSlash(ProsePath(work, s.ID, b.ID)))
	rec := strings.TrimSuffix(md, ".md") + ".json"
	rh, err := RenderHash(s, b)
	if err != nil {
		return Render{}, err
	}
	var r Render
	cached := false
	if _, err := os.Stat(md); err == nil && !f.Render {
		if raw, err := os.ReadFile(rec); err == nil {
			if err := json.Unmarshal(raw, &r); err != nil {
				return r, fmt.Errorf("%s: %w", rec, err)
			}
			cached = r.RenderHash == rh
		}
	}
	if cached {
		if r.StopReason != "end_turn" || !f.Extract && r.ExtractHash == ExtractHash(s, b.Lang) &&
			len(r.Extractions) == DefaultK && r.Report.ExtractError == "" {
			return r, nil
		}
		if err := reextract(ctx, c, s, &r, DefaultK); err != nil {
			return r, err
		}
	} else {
		if r, err = RenderOne(ctx, c, work, s, b); err != nil {
			return r, err
		}
		if err := os.MkdirAll(filepath.Dir(md), 0o755); err != nil {
			return r, err
		}
		head := fmt.Sprintf("---\nwork: %s\nscene: %s\nbackend: %s\nlang: %s\nstop_reason: %s\n---\n\n", work, s.ID, b.ID, b.Lang, r.StopReason)
		if err := os.WriteFile(md, []byte(head+r.Prose+"\n"), 0o644); err != nil {
			return r, err
		}
	}
	raw, err := json.MarshalIndent(r, "", "  ")
	if err != nil {
		return r, err
	}
	return r, os.WriteFile(rec, raw, 0o644)
}

// Row is the verdict for one render.
type Row struct {
	Render
	Top  string   `json:"top"` // the top-ratio tone
	Pass bool     `json:"pass"`
	Why  []string `json:"why"`
}

// SceneVerdict is the verdict for one scene rendered with every backend. The Jaccard key is "a~b" (backend IDs in name order).
// Jaccard = non-entry/exit acts that convey facts (judged), JaccardAll = all acts (reference, not used in the verdict).
type SceneVerdict struct {
	Work, Scene string
	Rows        []Row
	Jaccard     map[string]float64
	JaccardAll  map[string]float64
	Pass        bool
	Why         []string // scene-level failures (Jaccard, top-tone overlap)
}

// ContentKeys is the key multiset of the re-extracted acts that convey facts (excluding entries/exits). The same keys Diff compares with the original
// (facts are canonical propositions). If all, count every act (for reference).
func ContentKeys(s Scene, e Extracted, all bool) map[string]int {
	m := map[string]int{}
	for _, a := range e.Acts {
		if all || a.Fact != "" && VerbClass(a.Verb) != "presence" {
			m[actKey(a.Actor, a.Verb, s.Canon(a.Fact))]++
		}
	}
	return m
}

// Jaccard is the multiset Jaccard (Σmin / Σmax). 1 if both are empty.
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

// Judge: same content (each render missing ≤ 1, hallucinated ≤ 1, leaked = 0, Jaccard between backends ≥ 0.9) and
// different tones (backends with a tone have target ratio ≥ 0.8, and their top tones are all different).
// Does not use the recorded Report and Tones; recounts with Score.
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
