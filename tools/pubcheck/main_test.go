package main

import "testing"

func TestPubcheckRules(t *testing.T) {
	hit := map[string]string{
		"anthropic": "k=sk-ant-abcdefghijklmnopqrstuvwxyz",
		"ghp":       "ghp_" + "abcdefghijklmnopqrstuvwxyz0123456789",
		"ghpat":     "github_pat_11AAAA",
		"aws":       "AKIA" + "ABCDEFGHIJKLMNOP",
		"pem":       "-----BEGIN RSA PRIVATE KEY-----",
		"generic":   `api_key = "abcdefghijklmnop"`,
		"path":      `C:\Users\sg671\Documents`,
		"path2":     "/c/users/sg671/x",
		"user":      "home sg671 here",
		"vault":     "see Obsidian Vault notes",
		"vaultword": "the Vault holds it",
	}
	for n, s := range hit {
		if rule(s) == "" {
			t.Errorf("%s: want hit for %q", n, s)
		}
	}
	miss := []string{
		"postgres://sl:sl@localhost:55432/sl?sslmode=disable",
		"github.com/starthalf51/diegesis-lang",
		`token := "short"`,
		"plain text",
		"a vaulted ceiling",
	}
	for _, s := range miss {
		if r := rule(s); r != "" {
			t.Errorf("want no hit for %q, got %s", s, r)
		}
	}
}
