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
