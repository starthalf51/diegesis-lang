package main

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
)

type StyleSample struct {
	ID            string `json:"id"`
	Author        string `json:"author"`
	Lang          string `json:"lang"`
	Work          string `json:"work"`
	Source        string `json:"source"`
	Text          string `json:"text"`
	DiedYear      int    `json:"died_year"`
	PublishedYear int    `json:"published_year"`
}

// CheckPublicDomain: author dead 70+ years (KR) and published 95+ years ago (US).
func CheckPublicDomain(s StyleSample, year int) error {
	if s.DiedYear <= 0 || s.PublishedYear <= 0 {
		return fmt.Errorf("%q: died_year/published_year required", s.Author)
	}
	if s.DiedYear > year-71 {
		return fmt.Errorf("%q: died %d, not public domain in %d", s.Author, s.DiedYear, year)
	}
	if s.PublishedYear > year-96 {
		return fmt.Errorf("%q: published %d, not public domain in %d", s.Author, s.PublishedYear, year)
	}
	return nil
}

// LoadStyles reads every *.json in dir and rejects non-public-domain samples (checked against 2026).
func LoadStyles(dir string) ([]StyleSample, error) {
	paths, err := filepath.Glob(filepath.Join(dir, "*.json"))
	if err != nil {
		return nil, err
	}
	sort.Strings(paths)
	var out []StyleSample
	for _, p := range paths {
		b, err := os.ReadFile(p)
		if err != nil {
			return nil, err
		}
		var s StyleSample
		if err := json.Unmarshal(b, &s); err != nil {
			return nil, fmt.Errorf("%s: %w", p, err)
		}
		if err := CheckPublicDomain(s, 2026); err != nil {
			return nil, fmt.Errorf("%s: %w", p, err)
		}
		out = append(out, s)
	}
	return out, nil
}
