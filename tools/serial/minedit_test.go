package main

import (
	"strings"
	"testing"

	"diegesis/internal/render"
)

func TestSceneDiff(t *testing.T) {
	mk := func(place string) render.Scene {
		return render.Scene{ID: "s1", Title: "t", Place: place,
			Cast:  []render.Entity{{ID: "a", Names: map[string][]string{"ko": {"가"}}}},
			Facts: []render.Fact{{ID: "f", Text: "사실", True: true}}}
	}
	rm, ad := sceneDiff(mk("옛 방"), mk("새 방"))
	if len(rm) != 1 || len(ad) != 1 || !strings.Contains(rm[0], "옛 방") || !strings.Contains(ad[0], "새 방") {
		t.Fatalf("rm=%v ad=%v", rm, ad)
	}
	if rm, ad = sceneDiff(mk("같은"), mk("같은")); len(rm)+len(ad) != 0 {
		t.Fatalf("same scene differs: %v %v", rm, ad)
	}
}
