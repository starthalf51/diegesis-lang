package main

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"gopkg.in/yaml.v3"
)

// TestWorkflowsParse: every workflow parses as YAML and has the required keys and strings.
func TestWorkflowsParse(t *testing.T) {
	need := map[string][]string{
		"release.yml": {"CGO_ENABLED=0", "dieg_${TAG#v}_${GOOS}_${GOARCH}", "checksums.txt", "softprops/action-gh-release@v2", "contents: write", "tags:", "goos: [linux, darwin, windows]", "goarch: [amd64, arm64]"},
		"pg-image.yml": {"ghcr.io/starthalf51/diegesis-pg:age1.8.0-pgvector0.8.7", "diegesis-pg:latest", "packages: write", "branches: [main]", "cache-from: type=gha", "cache-to: type=gha,mode=max", "linux/amd64,linux/arm64"},
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

func TestVersionOverridable(t *testing.T) {
	exe := filepath.Join(t.TempDir(), "dieg.exe")
	if out, err := exec.Command("go", "build", "-ldflags", "-X main.version=9.9.9", "-o", exe, ".").CombinedOutput(); err != nil {
		t.Fatalf("build: %v\n%s", err, out)
	}
	out, err := exec.Command(exe, "version").Output()
	if err != nil {
		t.Fatal(err)
	}
	if got := strings.TrimSpace(string(out)); got != "dieg 9.9.9" {
		t.Fatalf("got %q", got)
	}
}
