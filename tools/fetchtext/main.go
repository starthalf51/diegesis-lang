// fetchtext downloads public-domain texts as plain text with a source header, for corpus work.
//
//	go run ./tools/fetchtext [-o data/raw] gutenberg:1533 wikisource-ko:운수_좋은_날
//
// Only sources whose texts are public domain: Project Gutenberg (header/footer stripped) and Korean
// Wikisource (wikitext reduced to prose). One request at a time with a pause, identified by User-Agent.
package main

import (
	"flag"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"time"
)

const agent = "diegesis-fetchtext/0.1 (+https://github.com/starthalf51/diegesis-lang)"

func main() {
	out := flag.String("o", "data/raw", "output directory")
	pause := flag.Duration("pause", 2*time.Second, "pause between requests")
	flag.Parse()
	if flag.NArg() == 0 {
		fmt.Fprintln(os.Stderr, "usage: fetchtext [-o dir] gutenberg:<id> | wikisource-ko:<title> ...")
		os.Exit(2)
	}
	if err := os.MkdirAll(*out, 0o755); err != nil {
		fail(err)
	}
	for i, ref := range flag.Args() {
		if i > 0 {
			time.Sleep(*pause)
		}
		name, src, text, err := fetch(ref)
		if err != nil {
			fail(fmt.Errorf("%s: %w", ref, err))
		}
		head := fmt.Sprintf("# %s (public domain)\n# Source: %s\n# Retrieved %s by tools/fetchtext.\n\n", name, src, time.Now().Format("2006-01-02"))
		p := filepath.Join(*out, safe(name)+".txt")
		if err := os.WriteFile(p, []byte(head+text), 0o644); err != nil {
			fail(err)
		}
		fmt.Printf("%s  %d chars\n", p, len([]rune(text)))
	}
}

func fetch(ref string) (name, src, text string, err error) {
	kind, arg, ok := strings.Cut(ref, ":")
	if !ok || arg == "" {
		return "", "", "", fmt.Errorf("want gutenberg:<id> or wikisource-ko:<title>")
	}
	switch kind {
	case "gutenberg":
		if !regexp.MustCompile(`^\d+$`).MatchString(arg) {
			return "", "", "", fmt.Errorf("gutenberg id must be a number")
		}
		src = fmt.Sprintf("https://www.gutenberg.org/cache/epub/%s/pg%s.txt", arg, arg)
		raw, err := get(src)
		if err != nil {
			return "", "", "", err
		}
		name, text = gutenberg(raw)
		if name == "" {
			name = "gutenberg-" + arg
		}
		return name, src, text, nil
	case "wikisource-ko":
		src = "https://ko.wikisource.org/w/index.php?action=raw&title=" + url.QueryEscape(arg)
		raw, err := get(src)
		if err != nil {
			return "", "", "", err
		}
		return strings.ReplaceAll(arg, "_", " "), src, wikitext(raw), nil
	}
	return "", "", "", fmt.Errorf("unknown source %q", kind)
}

func get(u string) (string, error) {
	req, err := http.NewRequest(http.MethodGet, u, nil)
	if err != nil {
		return "", err
	}
	req.Header.Set("User-Agent", agent)
	c := &http.Client{Timeout: time.Minute}
	resp, err := c.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("GET %s: %s", u, resp.Status)
	}
	b, err := io.ReadAll(io.LimitReader(resp.Body, 50<<20))
	return strings.ReplaceAll(string(b), "\r\n", "\n"), err
}

var gutTitle = regexp.MustCompile(`(?m)^Title: (.+)$`)
var gutStart = regexp.MustCompile(`(?m)^\*\*\* ?START OF (THE|THIS) PROJECT GUTENBERG.*$`)
var gutEnd = regexp.MustCompile(`(?m)^\*\*\* ?END OF (THE|THIS) PROJECT GUTENBERG.*$`)

// gutenberg keeps the text between the START/END markers and returns the Title: line.
func gutenberg(s string) (title, body string) {
	if m := gutTitle.FindStringSubmatch(s); m != nil {
		title = strings.TrimSpace(m[1])
	}
	if loc := gutStart.FindStringIndex(s); loc != nil {
		s = s[loc[1]:]
	}
	if loc := gutEnd.FindStringIndex(s); loc != nil {
		s = s[:loc[0]]
	}
	return title, strings.TrimSpace(s) + "\n"
}

var (
	wTemplate = regexp.MustCompile(`\{\{[^{}]*\}\}`)
	wLink     = regexp.MustCompile(`\[\[(?:[^\]|]*\|)?([^\]]*)\]\]`)
	wRef      = regexp.MustCompile(`(?s)<ref[^>]*?(/>|>.*?</ref>)`)
	wTag      = regexp.MustCompile(`<[^>]+>`)
	wEmph     = regexp.MustCompile(`'{2,}`)
	wHead     = regexp.MustCompile(`(?m)^=+\s*(.*?)\s*=+\s*$`)
	wBlank    = regexp.MustCompile(`\n{3,}`)
)

// wikitext reduces MediaWiki markup to prose: templates, refs, tags and emphasis dropped, links to their label.
func wikitext(s string) string {
	for prev := ""; prev != s; { // nested templates
		prev, s = s, wTemplate.ReplaceAllString(s, "")
	}
	s = wRef.ReplaceAllString(s, "")
	s = wLink.ReplaceAllString(s, "$1")
	s = wTag.ReplaceAllString(s, "")
	s = wEmph.ReplaceAllString(s, "")
	s = wHead.ReplaceAllString(s, "$1")
	s = regexp.MustCompile(`(?m)^(Category|분류):.*$`).ReplaceAllString(s, "")
	return strings.TrimSpace(wBlank.ReplaceAllString(s, "\n\n")) + "\n"
}

// winReserved matches a Windows device name (CON, NUL, COM1...) that cannot be a file name.
var winReserved = regexp.MustCompile(`(?i)^(CON|PRN|AUX|NUL|COM[1-9]|LPT[1-9])(\.|$)`)

func safe(s string) string {
	s = regexp.MustCompile(`[\\/:*?"<>|\s]+`).ReplaceAllString(strings.TrimSpace(s), "-")
	if r := []rune(s); len(r) > 80 {
		s = string(r[:80])
	}
	if s == "" {
		return "text"
	}
	if winReserved.MatchString(s) {
		s = "_" + s
	}
	return s
}

func fail(err error) {
	fmt.Fprintln(os.Stderr, "fetchtext:", err)
	os.Exit(1)
}
