package main

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"time"
)

// runBEpisode runs `dieg write` (ep 1) or `dieg continue` (ep >= 2). The idea file carries ep1's brief, later
// briefs go through --brief; both carry the same length line as A's prompts.
func runBEpisode(ctx context.Context, t *tracker, briefs []string, e int) error {
	c := t.c
	b := filepath.Join(c.out, "b")
	sc := strconv.Itoa(perEp)
	args := []string{"continue", b, "--ep", strconv.Itoa(e), "--brief", briefs[e] + " (" + lengthLine + ")", "--scenes", sc}
	if e == 1 {
		idea := filepath.Join(c.out, "b-idea.txt")
		if err := write(idea, strings.TrimSpace(c.idea)+fmt.Sprintf("\n\n## 연재 설정\n총 %d화, 화당 장면 %d개(%s). 지금은 1화만 쓴다.\n1화 방향: %s", c.eps, perEp, lengthLine, briefs[1])); err != nil {
			return err
		}
		args = []string{"write", idea, "-o", b, "--scenes", sc}
	}
	if err := t.check(judgeReserve); err != nil {
		return err
	}
	if err := runDieg(ctx, c.out, args); err != nil {
		return fmt.Errorf("dieg %s (ep %d): %w (see %s)", args[0], e, err, filepath.Join(c.out, "b.log"))
	}
	t.syncScenes(e)
	t.record("B", fmt.Sprintf("ep%d", e))
	return nil
}

// runBRevise runs `dieg revise` for revision k (no --scenes: every scene whose render request changed is re-rendered).
func runBRevise(ctx context.Context, t *tracker, k, e int) error {
	return t.revStep("B", k+1, func() error {
		if err := t.check(judgeReserve); err != nil {
			return err
		}
		if err := runDieg(ctx, t.c.out, []string{"revise", filepath.Join(t.c.out, "b"), revReqs[k]}); err != nil {
			return fmt.Errorf("dieg revise (rev %d): %w (see %s)", k+1, err, filepath.Join(t.c.out, "b.log"))
		}
		t.syncScenes(e)
		return nil
	})
}

// syncScenes assigns scene files in b/novel/ that appeared during episode ep's step to ep (this is how B's scenes
// are grouped into episodes, rather than by position, so one missing scene cannot shift the later episodes) and
// forgets files that were removed.
func (t *tracker) syncScenes(ep int) {
	ents, _ := os.ReadDir(filepath.Join(t.c.out, "b", "novel"))
	cur := map[string]time.Time{}
	for _, e := range ents {
		if i, err := e.Info(); err == nil && !e.IsDir() && strings.HasSuffix(e.Name(), ".md") {
			cur[e.Name()] = i.ModTime()
		}
	}
	known := map[string]bool{}
	for e, fs := range t.bScenes {
		var keep []string
		for _, f := range fs {
			if _, ok := cur[f]; ok {
				keep = append(keep, f)
				known[f] = true
			}
		}
		t.bScenes[e] = keep
	}
	var added []string
	for f := range cur {
		if !known[f] {
			added = append(added, f)
		}
	}
	sort.Slice(added, func(i, j int) bool {
		if ti, tj := cur[added[i]], cur[added[j]]; !ti.Equal(tj) {
			return ti.Before(tj)
		}
		return added[i] < added[j]
	})
	t.bScenes[ep] = append(t.bScenes[ep], added...)
}

// bEpText is episode e's rendered scenes joined like A's.
func (t *tracker) bEpText(e int) string {
	var parts []string
	for _, f := range t.bScenes[e] {
		if b, err := os.ReadFile(filepath.Join(t.c.out, "b", "novel", f)); err == nil {
			parts = append(parts, strings.TrimSpace(string(b)))
		}
	}
	return strings.Join(parts, sceneSep)
}

// bMissing lists episodes (up to the cut) that have fewer than perEp rendered scenes: episode -> how many are missing.
func (t *tracker) bMissing() map[string]int {
	m := map[string]int{}
	for e := 1; e <= t.cut; e++ {
		if n := len(t.bScenes[e]); n < perEp {
			m[strconv.Itoa(e)] = perEp - n
		}
	}
	return m
}
