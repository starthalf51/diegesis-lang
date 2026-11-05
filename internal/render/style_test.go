package render

import (
	"path/filepath"
	"testing"
)

func TestCheckPublicDomain(t *testing.T) {
	ok := []StyleSample{{Author: "Arthur Conan Doyle", DiedYear: 1930, PublishedYear: 1891}, {Author: "현진건", DiedYear: 1943, PublishedYear: 1924}}
	for _, s := range ok {
		if err := CheckPublicDomain(s, 2026); err != nil {
			t.Error(err)
		}
	}
	bad := []StyleSample{{DiedYear: 1956, PublishedYear: 1900}, {DiedYear: 1930, PublishedYear: 1931}, {DiedYear: 0, PublishedYear: 1900}}
	for _, s := range bad {
		if CheckPublicDomain(s, 2026) == nil {
			t.Errorf("want reject %+v", s)
		}
	}
}

func TestCorpusFixturesLoad(t *testing.T) {
	scenes, _ := filepath.Glob("testdata/scenes/*.json")
	want := map[string]bool{"macbeth-1.3.json": false, "chibi-jianggan.json": false, "holmes-scandal-opening.json": false}
	for _, p := range scenes {
		if _, ok := want[filepath.Base(p)]; !ok {
			continue
		}
		want[filepath.Base(p)] = true
		if _, err := LoadScene(p); err != nil {
			t.Error(err)
		}
	}
	for n, seen := range want {
		if !seen {
			t.Errorf("missing scene %s", n)
		}
	}
	st, err := LoadStyles("testdata/styles")
	if err != nil {
		t.Fatal(err)
	}
	if len(st) != 2 {
		t.Errorf("want 2 styles, got %d", len(st))
	}
	for _, s := range st {
		if s.Text == "" || s.Source == "" {
			t.Errorf("incomplete sample %s", s.ID)
		}
	}
}
