package main

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
)

type Job struct {
	Scene   Scene
	Variant Variant
	Lang    string
	Style   *StyleSample
}

// Key is "<scene>.<variant>.<lang>".
func (j Job) Key() string { return fmt.Sprintf("%s.%s.%s", j.Scene.ID, j.Variant, j.Lang) }

// Jobs expands scenes x variants x langs; b and c use the first style sample in the job's lang.
func Jobs(scenes []Scene, styles []StyleSample, variants []Variant, langs []string) ([]Job, error) {
	var jobs []Job
	for _, s := range scenes {
		for _, v := range variants {
			for _, l := range langs {
				j := Job{Scene: s, Variant: v, Lang: l}
				if v != VariantIROnly {
					for i := range styles {
						if styles[i].Lang == l {
							j.Style = &styles[i]
							break
						}
					}
					if j.Style == nil {
						return nil, fmt.Errorf("%s: no style sample for lang %q", j.Key(), l)
					}
				}
				jobs = append(jobs, j)
			}
		}
	}
	return jobs, nil
}

type Record struct {
	Key          string  `json:"key"`
	Prose        string  `json:"prose"`
	StopReason   string  `json:"stop_reason"`
	Report       Report  `json:"report"`
	RenderCost   float64 `json:"render_cost"`
	ExtractCost  float64 `json:"extract_cost"`
	RenderUsage  Usage   `json:"render_usage"`
	ExtractUsage Usage   `json:"extract_usage"`
	Skipped      bool    `json:"skipped"`
}

func stopped(what, reason string) string {
	if reason == "" {
		reason = "<empty>"
	}
	return what + " stopped: " + reason
}

// RunJob renders, re-extracts and diffs. A Completer error returns before anything is written,
// so a re-run retries the job. The .json is written last: it is the skip marker.
func RunJob(ctx context.Context, c Completer, j Job, outDir string, force bool) (Record, error) {
	key := j.Key()
	jsonPath := filepath.Join(outDir, key+".json")
	if !force {
		if b, err := os.ReadFile(jsonPath); err == nil {
			var r Record
			if err := json.Unmarshal(b, &r); err != nil {
				return r, fmt.Errorf("%s: %w", jsonPath, err)
			}
			r.Skipped = true
			return r, nil
		}
	}
	sys, user, err := BuildPrompt(j.Scene, j.Variant, j.Lang, j.Style)
	if err != nil {
		return Record{}, err
	}
	res, err := c.Complete(ctx, sys, user)
	if err != nil {
		return Record{}, fmt.Errorf("%s render: %w", key, err)
	}
	rec := Record{Key: key, Prose: res.Text, StopReason: res.StopReason, RenderCost: Cost(res.Usage), RenderUsage: res.Usage}
	if res.StopReason != "end_turn" {
		rec.Report.ExtractError = stopped("render", res.StopReason)
	} else {
		xs, xu := BuildExtractPrompt(j.Scene, j.Lang, res.Text)
		xr, err := c.Complete(ctx, xs, xu)
		if err != nil {
			return Record{}, fmt.Errorf("%s extract: %w", key, err)
		}
		rec.ExtractCost = Cost(xr.Usage)
		rec.ExtractUsage = xr.Usage
		if xr.StopReason != "end_turn" {
			rec.Report.ExtractError = stopped("extract", xr.StopReason)
		} else if e, err := ParseExtracted(xr.Text); err != nil {
			rec.Report.ExtractError = err.Error()
		} else {
			rec = withReport(rec, Diff(j.Scene, e))
		}
	}
	if err := writeOutputs(c, j, rec, outDir); err != nil {
		return rec, err
	}
	return rec, nil
}

func withReport(r Record, rep Report) Record { r.Report = rep; return r }

func writeOutputs(c Completer, j Job, rec Record, outDir string) error {
	model := "unknown"
	if cl, ok := c.(*Claude); ok {
		model = cl.Model
	}
	md := fmt.Sprintf("---\nscene: %s\nvariant: %s\nlang: %s\nmodel: %s\nstop_reason: %s\nmode: rendered\n---\n\n%s\n",
		j.Scene.ID, j.Variant, j.Lang, model, rec.StopReason, rec.Prose)
	if err := os.WriteFile(filepath.Join(outDir, rec.Key+".md"), []byte(md), 0o644); err != nil {
		return err
	}
	b, err := json.MarshalIndent(rec, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(filepath.Join(outDir, rec.Key+".json"), b, 0o644)
}
