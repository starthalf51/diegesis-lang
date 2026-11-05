package main

import "testing"

func mustLoad(t *testing.T, path string) Scene {
	t.Helper()
	s, err := LoadScene(path)
	if err != nil {
		t.Fatal(err)
	}
	return s
}

func mustStyle(t *testing.T, path string) StyleSample {
	t.Helper()
	s, err := loadStyle(path)
	if err != nil {
		t.Fatal(err)
	}
	return s
}

func factText(s Scene, id string) string {
	f, _ := s.FactByID(id)
	return f.Text
}

// extractedFromScene is the "perfect extractor": every act with a Fact, facts deduped, no new events.
func extractedFromScene(s Scene) Extracted {
	e := Extracted{Acts: []ExtractedAct{}, FactsStated: []string{}, NewEvents: []string{}}
	seen := map[string]bool{}
	for _, b := range s.Beats {
		for _, a := range b.Acts {
			if a.Fact == "" {
				continue
			}
			e.Acts = append(e.Acts, ExtractedAct{Actor: a.Actor, Verb: a.Verb, Targets: a.Targets, Fact: a.Fact})
			if !seen[a.Fact] {
				seen[a.Fact] = true
				e.FactsStated = append(e.FactsStated, a.Fact)
			}
		}
	}
	return e
}
