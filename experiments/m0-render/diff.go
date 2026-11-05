package main

import "sort"

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
	case "tells", "says", "hails", "lies", "whispers", "aside":
		return "assert"
	case "sees":
		return "perceive"
	case "reads":
		return "read"
	case "does":
		return "act"
	case "enter", "exit":
		return "presence"
	}
	return v
}

func actKey(actor, verb, fact string) string { return actor + "|" + VerbClass(verb) + "|" + fact }

// Diff compares original scene to re-extracted IR by actor|class|fact keys only.
func Diff(s Scene, e Extracted) Report {
	got := map[string]int{}
	hall, leak := set{}, set{}
	withheld := map[string]bool{}
	for _, id := range s.Withheld {
		withheld[id] = true
	}
	check := func(id string) {
		if id == "" {
			return
		}
		if withheld[id] {
			leak[id] = true
		}
		if _, ok := s.FactByID(id); !ok {
			hall["unknown-fact:"+id] = true
		}
	}
	for _, a := range e.Acts {
		got[actKey(a.Actor, a.Verb, a.Fact)]++
		check(a.Fact)
	}
	for _, id := range e.FactsStated {
		check(id)
	}
	for _, n := range e.NewEvents {
		hall[n] = true
	}
	want := map[string]int{}
	for _, b := range s.Beats {
		for _, a := range b.Acts {
			if a.Fact == "" || VerbClass(a.Verb) == "presence" {
				continue
			}
			want[actKey(a.Actor, a.Verb, a.Fact)]++
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
	return Report{Missing: miss, Hallucinated: hall.sorted(), Leaked: leak.sorted()}
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
