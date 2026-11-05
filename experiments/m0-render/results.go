package main

import (
	. "story-lang/internal/render"

	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
)

// LoadRecords reads every <key>.json record in outDir (results.md is not .json).
func LoadRecords(outDir string) ([]Record, error) {
	paths, err := filepath.Glob(filepath.Join(outDir, "*.json"))
	if err != nil {
		return nil, err
	}
	var rs []Record
	for _, p := range paths {
		b, err := os.ReadFile(p)
		if err != nil {
			return nil, err
		}
		var r Record
		if err := json.Unmarshal(b, &r); err != nil {
			return nil, fmt.Errorf("%s: %w", p, err)
		}
		rs = append(rs, r)
	}
	return rs, nil
}

// oldScores parses an existing results.md table: key -> non-empty last (판정) cell.
func oldScores(path string) map[string]string {
	out := map[string]string{}
	b, err := os.ReadFile(path)
	if err != nil {
		return out
	}
	for _, line := range strings.Split(string(b), "\n") {
		line = strings.TrimSpace(line)
		if !strings.HasPrefix(line, "|") || !strings.HasSuffix(line, "|") {
			continue
		}
		cells := strings.Split(line, "|")
		if len(cells) < 4 {
			continue
		}
		key, score := strings.TrimSpace(cells[1]), strings.TrimSpace(cells[len(cells)-2])
		if key != "key" && !strings.HasPrefix(key, "---") && score != "" {
			out[key] = score
		}
	}
	return out
}

// WriteResults writes the table for all records plus failed jobs (key -> message, rows
// marked ERROR), preserving human scores already present in an existing results.md.
func WriteResults(records []Record, failed map[string]string, path string) error {
	scores := oldScores(path)
	byKey := map[string]Record{}
	for _, r := range records {
		byKey[r.Key] = r
	}
	for k, msg := range failed {
		byKey[k] = Record{Key: k, Report: Report{ExtractError: "ERROR: " + msg}}
	}
	keys := make([]string, 0, len(byKey))
	for k := range byKey {
		keys = append(keys, k)
	}
	sort.Strings(keys)

	var sb, detail strings.Builder
	sb.WriteString("| key | 손실 | 환각 | 누출 | 추출오류 | 비용($) | 판정(1-5) |\n|---|---|---|---|---|---|---|\n")
	var total float64
	for _, k := range keys {
		r := byKey[k]
		m, h, l := "?", "?", "?"
		if r.Report.ExtractError == "" {
			m, h, l = fmt.Sprint(len(r.Report.Missing)), fmt.Sprint(len(r.Report.Hallucinated)), fmt.Sprint(len(r.Report.Leaked))
		}
		cost := r.RenderCost + r.ExtractCost
		total += cost
		fmt.Fprintf(&sb, "| %s | %s | %s | %s | %s | %.4f | %s |\n", k, m, h, l, strings.ReplaceAll(r.Report.ExtractError, "|", "/"), cost, scores[k])
		rep := r.Report
		if len(rep.Missing)+len(rep.Hallucinated)+len(rep.Leaked) == 0 && rep.ExtractError == "" {
			continue
		}
		fmt.Fprintf(&detail, "\n### %s\n", k)
		for _, it := range []struct {
			name  string
			items []string
		}{{"Missing", rep.Missing}, {"Hallucinated", rep.Hallucinated}, {"Leaked", rep.Leaked}} {
			if len(it.items) > 0 {
				fmt.Fprintf(&detail, "- %s: %s\n", it.name, strings.Join(it.items, "; "))
			}
		}
		if rep.ExtractError != "" {
			fmt.Fprintf(&detail, "- ExtractError: %s\n", rep.ExtractError)
		}
	}
	fmt.Fprintf(&sb, "\n총비용: $%.4f\n", total)
	if detail.Len() > 0 {
		sb.WriteString("\n## 상세\n" + detail.String())
	}
	return os.WriteFile(path, []byte(sb.String()), 0o644)
}
