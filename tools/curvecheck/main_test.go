package main

import (
	"strings"
	"testing"
)

// 20 chapters; mean = index, so ch01 is rank 1 (lowest) and ch20 rank 20 (highest).
func fakeCurve(means map[string]float64) []row {
	var rows []row
	for i := 1; i <= 20; i++ {
		name := "w.ch" + string(rune('a'+i-1))
		rows = append(rows, row{Module: name, Mean: float64(i)})
	}
	for k, v := range means {
		for i := range rows {
			if rows[i].Module == "w."+k {
				rows[i].Mean = v
			}
		}
	}
	return rows
}

func TestCurveCheck(t *testing.T) {
	exp := Expect{Valley: []string{"cha"}, High: []string{"cht", "chs", "chr", "chq"}}
	out, pass := Check(fakeCurve(nil), exp)
	if !pass || !strings.HasSuffix(strings.TrimSpace(out), "PASS") {
		t.Fatalf("want PASS, got:\n%s", out)
	}
	if !strings.Contains(out, "valley cha: rank 1/20 (bottom 10%? yes)") || !strings.Contains(out, "high: 4/4 in top quartile") {
		t.Fatalf("verdict lines wrong:\n%s", out)
	}
	// valley chapter sits mid-curve -> FAIL
	out, pass = Check(fakeCurve(map[string]float64{"cha": 15.5}), exp)
	if pass || !strings.HasSuffix(strings.TrimSpace(out), "FAIL") || !strings.Contains(out, "bottom 10%? no") {
		t.Fatalf("want FAIL, got:\n%s", out)
	}
}
