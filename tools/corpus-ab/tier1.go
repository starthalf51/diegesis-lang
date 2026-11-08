package main

// 계획6 T1: 2단 필요 여부 오프라인 판정. 저장된 1단 원문(calls/<변형>_r<n>_t1_a.json)만 다시 채점한다. 호출 0.
// 정의는 sc_report.go 와 같다(S1=fact_present, S2=fact_present&&as_stated, G1=revealed, Unsure 제외).
// G2 는 PG 의 장면(IR)이 있어야 걸러지므로 여기서는 재계산하지 않는다(판정 규칙도 S1 만 쓴다).

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
	same := &ctr{} // 1단 답 == 저장된 최종 답(2단 덮어쓰기 안 된 항목)
	over := 0      // 2단이 덮어쓴 항목 수
	flipped := 0   // 덮어쓰기로 Present 가 바뀐 항목
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
	fmt.Printf("규칙: 1단만 S1 검출 >= 90%% 그리고 오경보 <= 5%% 이면 off, 아니면 absent-only. S1 검출 %s, 오경보 %s.\n\n", s1d, s1f)
	fmt.Printf("1단 원문 %d실행 재채점 (2단 덮어쓰기 없음). 시행 = 변형 x 실행\n\n", nr)
	fmt.Println("| 축 | 검출(변형 항목) | 오경보 | 두 실행 일치 | 뒤집힘 |\n|---|---|---|---|---|")
	for _, k := range []string{"S1", "S2", "G1"} {
		fl := "n/a"
		if ag[k].n > 0 {
			fl = fmt.Sprintf("%.1f%%", 100-100*float64(ag[k].hit)/float64(ag[k].n))
		}
		fmt.Printf("| %s | %s | %s | %s | %s |\n", k, det[k], fa[k], ag[k], fl)
	}
	fmt.Printf("\nS2 검출(삭제 항목 포함): %s\n", det["S2(D)"])
	fmt.Printf("점검: 2단이 덮어쓴 항목 %d개(그중 Present 가 바뀐 것 %d), 덮어쓰지 않은 항목의 1단 답 == 최종 답: %s\n", over, flipped, same)
}
