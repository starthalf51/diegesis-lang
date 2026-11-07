package render

import (
	"sort"
	"strings"
)

// Report is the diff verdict. ExtractError is set by the runner, never by Diff.
type Report struct {
	Missing      []string `json:"missing"`
	Hallucinated []string `json:"hallucinated"`
	Leaked       []string `json:"leaked"`
	ExtractError string   `json:"extract_error"`
}

// VerbClass collapses synonyms; an unknown verb is its own class so it never silently matches.
func VerbClass(v string) string {
	switch v {
	case "tells", "says", "hails", "lies", "whispers", "aside", "asks", "orders", "shows", "decrees", "sends", "thinks", "enacts":
		return "assert"
	case "sees":
		return "perceive"
	case "reads", "writes":
		return "read"
	case "does":
		return "act"
	case "enter", "exit":
		return "presence"
	}
	return v
}

func actKey(actor, verb, fact string) string { return actor + "|" + VerbClass(verb) + "|" + fact }

// Diff compares original scene to re-extracted IR by actor|class|canon(fact) keys only: 별칭끼리는 같은 사실이다.
func Diff(s Scene, e Extracted) Report {
	got := map[string]int{}
	hall, leak := set{}, set{}
	withheld := map[string]bool{}
	for _, id := range s.Withheld {
		withheld[s.Canon(id)] = true
	}
	check := func(id string) {
		if id == "" {
			return
		}
		if withheld[s.Canon(id)] {
			leak[id] = true
		}
		if _, ok := s.FactByID(id); !ok {
			hall["unknown-fact:"+id] = true
		}
	}
	for _, a := range e.Acts {
		got[actKey(a.Actor, a.Verb, s.Canon(a.Fact))]++
		check(a.Fact)
	}
	for _, id := range e.FactsStated {
		check(id)
	}
	for _, n := range e.NewEvents {
		if !s.authored(n) {
			hall[n.Label()] = true
		}
	}
	want := map[string]int{}
	for _, b := range s.Beats {
		for _, a := range b.Acts {
			if a.Fact == "" || VerbClass(a.Verb) == "presence" {
				continue
			}
			want[actKey(a.Actor, a.Verb, s.Canon(a.Fact))]++
		}
	}
	// multiset: one Missing entry per shortfall, so same-key acts cannot mask each other
	miss := []string{}
	for k, n := range want {
		for i := got[k]; i < n; i++ {
			miss = append(miss, k)
		}
	}
	sort.Strings(miss)
	// R4: 원본보다 많이 뽑힌 사실 액트(원본에 없는 것 포함)는 초과분만큼 환각이다. 모르는 사실은 위에서 이미 셌다.
	halluc := hall.sorted()
	for _, a := range e.Acts {
		if _, ok := s.FactByID(a.Fact); !ok || VerbClass(a.Verb) == "presence" {
			continue
		}
		if k := actKey(a.Actor, a.Verb, s.Canon(a.Fact)); want[k] > 0 {
			want[k]--
		} else {
			halluc = append(halluc, "extra-act:"+k)
		}
	}
	sort.Strings(halluc)
	return Report{Missing: miss, Hallucinated: halluc, Leaked: leak.sorted()}
}

type set map[string]bool

func (m set) sorted() []string {
	out := make([]string, 0, len(m))
	for k := range m {
		out = append(out, k)
	}
	sort.Strings(out)
	return out
}

// authored reports whether a new_event merely restates an act the IR itself authors without a fact
// (e.g. `does hide ferry_ticket`, `exit all`). Such an event is not invented content, so ADR-019
// (no unauthored plot) is not loosened: anything not authored in the IR stays a hallucination.
func (s Scene) authored(n NewEvent) bool {
	ev := s.resolve(n.Actor)
	vc := VerbClass(n.Verb)
	for _, b := range s.Beats {
		for _, a := range b.Acts {
			ac := VerbClass(a.Verb)
			if a.Fact != "" && ac != "presence" || ac != vc {
				continue
			}
			tg := map[string]bool{}
			for _, t := range a.Targets {
				tg[s.resolve(t)] = true
			}
			if a.Actor == "" && a.Verb == "exit" && n.Verb == "exit" && tg[ev] {
				return true // exit all: empty actor, targets = the cast
			}
			if s.resolve(a.Actor) != ev {
				continue
			}
			switch {
			case vc == "act" && n.Action != "":
				if strings.EqualFold(n.Action, a.Action) {
					return true
				}
			case (vc == "act" || vc == "presence") && len(n.Targets) > 0:
				for _, t := range n.Targets {
					if tg[s.resolve(t)] {
						return true
					}
				}
			default:
				return true
			}
		}
	}
	return false
}

// resolve maps an id or any cast name to a lowercase canonical id; unknown strings stay as lowercase text.
func (s Scene) resolve(x string) string {
	for _, e := range s.Cast {
		if strings.EqualFold(e.ID, x) {
			return strings.ToLower(e.ID)
		}
		for _, ns := range e.Names {
			for _, n := range ns {
				if strings.EqualFold(n, x) {
					return strings.ToLower(e.ID)
				}
			}
		}
	}
	return strings.ToLower(x)
}
