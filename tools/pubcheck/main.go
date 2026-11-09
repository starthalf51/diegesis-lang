// pubcheck scans tracked files (or, with -history, every commit diff) for
// secrets and personal paths. Exit 1 on findings. Makes no network/model calls.
package main

import (
	"bufio"
	"flag"
	"fmt"
	"os"
	"os/exec"
	"regexp"
	"strings"
)

type pat struct {
	name string
	re   *regexp.Regexp
}

var rules = []pat{
	{"secret:anthropic-key", regexp.MustCompile(`sk-ant-[A-Za-z0-9_-]{20,}`)},
	{"secret:github-token", regexp.MustCompile(`ghp_[A-Za-z0-9]{36}`)},
	{"secret:github-pat", regexp.MustCompile(`github_pat_`)},
	{"secret:aws-key", regexp.MustCompile(`AKIA[0-9A-Z]{16}`)},
	{"secret:private-key", regexp.MustCompile(`-----BEGIN [A-Z ]*PRIVATE KEY-----`)},
	{"secret:generic", regexp.MustCompile(`(?i)(api[_-]?key|secret|token)\s*[:=]\s*['"][^'"]{12,}`)},
	{"path:personal", regexp.MustCompile(`(?i)(c:.users.sg671|/c/users/sg671|sg671)`)},
	{"path:vault", regexp.MustCompile(`Obsidian Vault`)},
	{"word:vault", regexp.MustCompile(`(?i)\bvault\b`)},
}

// allow lists intentional local-dev defaults.
var allow = []string{"postgres://sl:sl@localhost"}

// ponytail: pubcheck's own sources hold the patterns, so they are skipped by path.
var skipFile = []string{"tools/pubcheck/", "corpus/macbeth/texts/"} // Shakespeare says "vault"

func rule(line string) string {
	for _, a := range allow {
		line = strings.ReplaceAll(line, a, "")
	}
	for _, r := range rules {
		if r.re.MatchString(line) {
			return r.name
		}
	}
	return ""
}

func skipped(f string) bool {
	for _, s := range skipFile {
		if strings.HasPrefix(f, s) {
			return true
		}
	}
	return false
}

func main() {
	hist := flag.Bool("history", false, "scan all commit diffs")
	flag.Parse()
	n := 0
	if *hist {
		n = scanHistory()
	} else {
		n = scanTree()
	}
	if n > 0 {
		fmt.Fprintf(os.Stderr, "%d finding(s)\n", n)
		os.Exit(1)
	}
	fmt.Println("0 findings")
}

func lines(f func(*bufio.Scanner), name string, args ...string) {
	cmd := exec.Command(name, args...)
	out, err := cmd.StdoutPipe()
	if err != nil || cmd.Start() != nil {
		fmt.Fprintln(os.Stderr, "cannot run", name)
		os.Exit(2)
	}
	sc := bufio.NewScanner(out)
	sc.Buffer(make([]byte, 1<<20), 64<<20)
	f(sc)
	cmd.Wait()
}

func scanTree() int {
	var files []string
	lines(func(sc *bufio.Scanner) {
		for sc.Scan() {
			files = append(files, sc.Text())
		}
	}, "git", "ls-files")
	n := 0
	for _, f := range files {
		if skipped(f) {
			continue
		}
		fh, err := os.Open(f)
		if err != nil {
			continue
		}
		sc := bufio.NewScanner(fh)
		sc.Buffer(make([]byte, 1<<20), 64<<20)
		for i := 1; sc.Scan(); i++ {
			if r := rule(sc.Text()); r != "" {
				fmt.Printf("%s:%d: %s\n", f, i, r)
				n++
			}
		}
		fh.Close()
	}
	return n
}

// scanHistory reports commit:file:rule for added lines only; values are never printed.
func scanHistory() int {
	n := 0
	lines(func(sc *bufio.Scanner) {
		commit, file := "", ""
		for sc.Scan() {
			l := sc.Text()
			switch {
			case strings.HasPrefix(l, "commit "):
				commit = l[7:min(len(l), 19)]
			case strings.HasPrefix(l, "+++ b/"):
				file = l[6:]
			case strings.HasPrefix(l, "+") && !strings.HasPrefix(l, "+++"):
				if skipped(file) {
					continue
				}
				if r := rule(l[1:]); r != "" {
					fmt.Printf("%s %s: %s\n", commit, file, r)
					n++
				}
			}
		}
	}, "git", "log", "-p", "--all", "--no-color", "--no-ext-diff", "--text")
	return n
}
