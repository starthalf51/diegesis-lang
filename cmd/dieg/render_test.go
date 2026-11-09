package main

import (
	"strings"
	"testing"

	"github.com/starthalf51/diegesis-lang/internal/render"
)

// states that the judged Jaccard counts only non-entry/exit acts that convey facts, and lists the all-acts Jaccard alongside for reference.
func TestCheckReportLabelsJaccard(t *testing.T) {
	v := render.SceneVerdict{Work: "w", Scene: "s", Pass: true,
		Jaccard: map[string]float64{"a~b": 0.95}, JaccardAll: map[string]float64{"a~b": 0.61}}
	got := checkReport([]render.SceneVerdict{v}, nil, true)
	for _, want := range []string{"non-entry/exit acts that convey facts — judged): a~b 0.95", "all acts — reference only, not judged): a~b 0.61"} {
		if !strings.Contains(got, want) {
			t.Errorf("report lacks %q:\n%s", want, got)
		}
	}
}

// --only applies --force and --force-extract only to the listed pairs.
func TestScopedForce(t *testing.T) {
	f := render.Force{Render: true}
	if got := scoped(f, map[string]bool{}, "w/s.b"); got != f {
		t.Errorf("no --only: %+v", got)
	}
	only := map[string]bool{"w/s.b": false}
	if got := scoped(f, only, "w/s.c"); got != (render.Force{}) {
		t.Errorf("unlisted pair forced: %+v", got)
	}
	if got := scoped(f, only, "w/s.b"); got != f || !only["w/s.b"] {
		t.Errorf("listed pair: %+v %v", got, only)
	}
}
