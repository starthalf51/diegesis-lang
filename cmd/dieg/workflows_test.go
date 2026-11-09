package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"gopkg.in/yaml.v3"
)

// TestWorkflowsParse: 모든 워크플로가 YAML 로 파싱되고 필수 키·문자열을 가진다.
func TestWorkflowsParse(t *testing.T) {
	need := map[string][]string{
		"pg-image.yml": {"ghcr.io/starthalf51/diegesis-pg:age1.8.0-pgvector0.8.7", "diegesis-pg:latest", "packages: write", "linux/amd64,linux/arm64"},
	}
	files, _ := filepath.Glob("../../.github/workflows/*.yml")
	if len(files) != len(need) {
		t.Fatalf("workflows = %v, want %d", files, len(need))
	}
	for _, f := range files {
		b, err := os.ReadFile(f)
		if err != nil {
			t.Fatal(err)
		}
		var doc map[string]any
		if err := yaml.Unmarshal(b, &doc); err != nil {
			t.Fatalf("%s: %v", f, err)
		}
		for _, k := range []string{"on", "permissions", "jobs"} {
			if _, ok := doc[k]; !ok {
				t.Errorf("%s: missing key %q", f, k)
			}
		}
		for _, s := range need[filepath.Base(f)] {
			if !strings.Contains(string(b), s) {
				t.Errorf("%s: missing %q", f, s)
			}
		}
	}
}
