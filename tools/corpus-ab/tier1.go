package main

// Plan 6 T1: offline verdict on whether stage 2 is needed. Re-scores only the stored stage-1 raw answers (calls/<variant>_r<n>_t1_a.json). 0 calls.
// Definitions are the same as sc_report.go (S1=fact_present, S2=fact_present&&as_stated, G1=revealed, Unsure excluded).
// G2 is filtered only with the PG scene (IR), so it is not recomputed here (the verdict rule uses only S1 too).

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
)

func tier1(out string) {
	det := map[string]*ctr{"S1": {}, "S2": {}, "S2(D)": {}, "G1": {}}
	fa := map[string]*ctr{"S1": {}, "S2": {}, "G1": {}}
	ag := map[string]*ctr{"S1": {}, "S2": {}, "G1": {}}
	same := &ctr{} // stage-1 answer == stored final answer (items not overwritten by stage 2)
	over := 0      // number of items stage 2 overwrote
	flipped := 0   // items whose Present changed by the overwrite
	nr := 0
	for _, vd := range scVariants {
		_, t := readVariant(out, vd.Name)
		var rs [2]map[string]itemAns
		var lk [2]map[string]bool
		ok := [2]bool{}
		for i := range rs {
			b, err := os.ReadFile(filepath.Join(out, "calls", fmt.Sprintf("%s_r%d_t1_a.json", vd.Name, i+1)))
			if err != nil {
				continue
			}
			var rec struct{ Raw string }
			if json.Unmarshal(b, &rec) != nil {
				continue
			}
			var want []string
			for id := range t.Expect {
				want = append(want, id)
			}
			o, err := parseRaw(rec.Raw, want, false, 0)
			if err != nil {
				fmt.Fprintln(os.Stderr, "skip", vd.Name, i+1, err)
				continue
			}
			rs[i], lk[i], ok[i] = map[string]itemAns{}, map[string]bool{}, true
			for _, it := range o.Items {
				fp := yes(it.FactPresent)
				rs[i][it.ID] = itemAns{Present: fp, AsStated: fp && yes(it.AsStated)}
			}
			for _, w := range o.Withheld {
				lk[i][w.ID] = yes(w.Revealed)
			}
			nr++
			if full, k := loadRun(out, vd.Name, i+1); k {
				for id, f := range full.Items {
					if f.T2 {
						over++
						if f.Present != rs[i][id].Present {
							flipped++
						}
					} else {
						same.add(f.Present == rs[i][id].Present && f.AsStated == rs[i][id].AsStated)
					}
				}
			}
			mut := map[string]bool{}
			for _, m := range t.Mutated {
				mut[m] = true
			}
			for id, tr := range t.Expect {
				if tr.Unsure {
					continue
				}
				a := rs[i][id]
				switch {
				case mut[id] && !tr.Present:
					det["S1"].add(!a.Present)
					if a.Present {
						fmt.Fprintf(os.Stderr, "S1 miss: %s r%d %s\n", vd.Name, i+1, id)
					}
					det["S2(D)"].add(!a.AsStated)
				case mut[id] && tr.Present && !tr.AsStated:
					det["S2"].add(!a.AsStated)
				case !mut[id] && tr.Present:
					fa["S1"].add(!a.Present)
					if a.Present {
						fa["S2"].add(!a.AsStated)
					}
				}
			}
			for id, rv := range t.Withheld {
				if rv {
					det["G1"].add(lk[i][id])
				} else {
					fa["G1"].add(lk[i][id])
				}
			}
		}
		if ok[0] && ok[1] {
			for id, tr := range t.Expect {
				if !tr.Unsure {
					ag["S1"].add(rs[0][id].Present == rs[1][id].Present)
					ag["S2"].add(rs[0][id].AsStated == rs[1][id].AsStated)
				}
			}
			for id := range t.Withheld {
				ag["G1"].add(lk[0][id] == lk[1][id])
			}
		}
	}
	verdict := "absent-only"
	s1d, s1f := det["S1"], fa["S1"]
	if s1d.n > 0 && s1f.n > 0 && 100*s1d.hit >= 90*s1d.n && 100*s1f.hit <= 5*s1f.n {
		verdict = "off"
	}
	fmt.Printf("Tier2: %s\n\n", verdict)
	fmt.Printf("rule: off if stage 1 alone has S1 detection >= 90%% and false alarms <= 5%%, otherwise absent-only. S1 detection %s, false alarms %s.\n\n", s1d, s1f)
	fmt.Printf("stage-1 raw answers re-scored over %d runs (no stage-2 overwrite). trial = variant x run\n\n", nr)
	fmt.Println("| axis | detected (variant items) | false alarms | agreement across two runs | flipped |\n|---|---|---|---|---|")
	for _, k := range []string{"S1", "S2", "G1"} {
		fl := "n/a"
		if ag[k].n > 0 {
			fl = fmt.Sprintf("%.1f%%", 100-100*float64(ag[k].hit)/float64(ag[k].n))
		}
		fmt.Printf("| %s | %s | %s | %s | %s |\n", k, det[k], fa[k], ag[k], fl)
	}
	fmt.Printf("\nS2 detection (deleted items included): %s\n", det["S2(D)"])
	fmt.Printf("check: %d items overwritten by stage 2 (of which %d changed Present), stage-1 answer == final answer for items not overwritten: %s\n", over, flipped, same)
}
