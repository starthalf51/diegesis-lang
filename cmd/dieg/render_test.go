package main

import (
	"strings"
	"testing"

	"diegesis/internal/render"
)

// 판정 Jaccard 는 사실을 전하는 비입퇴장 액트만 센다고 밝히고, 모든 액트 Jaccard 는 참고로 함께 싣는다.
func TestCheckReportLabelsJaccard(t *testing.T) {
	v := render.SceneVerdict{Work: "w", Scene: "s", Pass: true,
		Jaccard: map[string]float64{"a~b": 0.95}, JaccardAll: map[string]float64{"a~b": 0.61}}
	got := checkReport([]render.SceneVerdict{v}, nil, true)
	for _, want := range []string{"사실을 전하는 비입퇴장 액트 — 판정): a~b 0.95", "모든 액트 — 참고, 판정에 쓰지 않음): a~b 0.61"} {
		if !strings.Contains(got, want) {
			t.Errorf("report lacks %q:\n%s", want, got)
		}
	}
}

// --only 는 --force·--force-extract 를 적힌 쌍에만 쓴다.
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
