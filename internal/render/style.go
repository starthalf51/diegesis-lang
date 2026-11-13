package render

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
)

type StyleSample struct {
	ID            string     `json:"id"`
	Author        string     `json:"author"`
	Lang          string     `json:"lang"`
	Work          string     `json:"work"`
	Source        string     `json:"source"`
	Text          string     `json:"text"`
	DiedYear      int        `json:"died_year"`
	PublishedYear int        `json:"published_year"`
	Rules         []string   `json:"rules"`           // style rules, listed in the system prompt
	Lexicon       []LexEntry `json:"lexicon"`         // word choices
	Opening       string     `json:"opening"`         // opening-sentence instruction used with --opening
	Flesh         string     `json:"flesh,omitempty"` // "" (bone only: add nothing) or "novel" (flesh contract: reactions required, texture allowed)
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
		s, err := LoadStyle(p)
		if err != nil {
			return nil, err
		}
		out = append(out, s)
	}
	return out, nil
}

// LoadStyle reads one style file and checks it is public domain (2026).
func LoadStyle(p string) (StyleSample, error) {
	var s StyleSample
	b, err := os.ReadFile(p)
	if err != nil {
		return s, err
	}
	if err := json.Unmarshal(b, &s); err != nil {
		return s, fmt.Errorf("%s: %w", p, err)
	}
	if err := CheckStyle(s); err != nil {
		return s, fmt.Errorf("%s: %w", p, err)
	}
	return s, nil
}

// LexEntry is one word choice: use Use for the concept For; never any of Avoid.
type LexEntry struct {
	Use   string   `json:"use"`
	For   string   `json:"for"`
	Avoid []string `json:"avoid"`
}

// CheckStyle validates a style: flesh is "" or "novel"; a sample text must be public domain (2026); a rules-only style (no text) skips that check; a style with neither is an error.
func CheckStyle(s StyleSample) error {
	if s.Flesh != "" && s.Flesh != "novel" {
		return fmt.Errorf("style %q: flesh %q, want \"\" or \"novel\"", s.ID, s.Flesh)
	}
	if s.Text == "" {
		if len(s.Rules) == 0 && len(s.Lexicon) == 0 {
			return fmt.Errorf("style %q has neither sample text nor rules", s.ID)
		}
		return nil
	}
	return CheckPublicDomain(s, 2026)
}
