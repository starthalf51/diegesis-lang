package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestLicenseFiles(t *testing.T) {
	lic, err := os.ReadFile("../../LICENSE")
	if err != nil {
		t.Fatal(err)
	}
	if first := strings.TrimSpace(string(lic)); !strings.HasPrefix(first, "Apache License") {
		t.Fatalf("LICENSE must start with Apache License, got %.40q", first)
	}
	notice, err := os.ReadFile("../../NOTICE")
	if err != nil {
		t.Fatal(err)
	}
	texts, _ := filepath.Glob("../../corpus/*/texts/*")
	n := 0
	for _, l := range strings.Split(string(notice), "\n") {
		if strings.HasPrefix(l, "corpus/") {
			n++
		}
	}
	if n != len(texts) {
		t.Fatalf("NOTICE has %d corpus lines, want %d", n, len(texts))
	}
}
