package main

import (
	"strings"
	"testing"
)

func TestGutenbergStrip(t *testing.T) {
	title, body := gutenberg("Title: Macbeth\nlicense\n*** START OF THE PROJECT GUTENBERG EBOOK MACBETH ***\nWhen shall we three meet again\n*** END OF THE PROJECT GUTENBERG EBOOK MACBETH ***\nfooter")
	if title != "Macbeth" || body != "When shall we three meet again\n" {
		t.Fatalf("%q %q", title, body)
	}
}

func TestWikitext(t *testing.T) {
	got := wikitext("{{머리말|{{작품|운수 좋은 날}}}}\n== 1 ==\n'''김첨지'''는 [[동소문|동소문 안]]에서<ref>주석</ref> 인력거<br/>꾼\n\n\n\n[[분류:소설]]\n분류:소설")
	want := "1\n김첨지는 동소문 안에서 인력거꾼\n"
	if got != want {
		t.Fatalf("got %q", got)
	}
}

func TestSafe(t *testing.T) {
	for in, want := range map[string]string{
		"a/b: c":                 "a-b-c",
		"":                       "text",
		" ? ":                    "-",
		"end. ":                  "end",
		"con":                    "_con",
		"NUL":                    "_NUL",
		"Com1":                   "_Com1",
		"lpt9":                   "_lpt9",
		"console":                "console",
		strings.Repeat("가", 100): strings.Repeat("가", 80),
	} {
		if got := safe(in); got != want {
			t.Errorf("safe(%q) = %q, want %q", in, got, want)
		}
	}
}
