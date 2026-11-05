package main

import (
	"strings"
	"testing"
)

func TestParseExtracted(t *testing.T) {
	e, err := ParseExtracted("Here you go:\n```json\n{\"acts\":[{\"actor\":\"ross\",\"verb\":\"tells\",\"targets\":[\"macbeth\"],\"fact\":\"cawdor_title\"}],\"facts_stated\":[\"cawdor_title\"],\"new_events\":[]}\n```")
	if err != nil || e.Acts[0].Actor != "ross" {
		t.Fatal(err, e)
	}
	if e, err := ParseExtracted(" {\"acts\":[],\"facts_stated\":[\"a\"],\"new_events\":[]} "); err != nil || e.FactsStated[0] != "a" {
		t.Fatal("bare json", err, e)
	}
	for _, bad := range []string{"", "  \n", "not json", "```json\n{\"acts\": [\n```", `{"acts":[],"extra":1}`,
		"```json\n{\"acts\":[]}", `{"acts":[]} {"acts":[]}`, "```json\n{}\n``` trailing"[:0] + "```json\n{\"acts\":[]} x\n```"} {
		if _, err := ParseExtracted(bad); err == nil {
			t.Errorf("want error for %q", bad)
		}
	}
}

func TestExtractPromptHidesAnswers(t *testing.T) {
	s := mustLoad(t, "testdata/scenes/chibi-jianggan.json")
	sys, user := BuildExtractPrompt(s, "ko", "장간은 …")
	all := strings.ToLower(sys + user)
	if strings.Contains(all, "must not state") || strings.Contains(all, "beat") || strings.Contains(all, "withheld") {
		t.Error("answer key leaked into extractor")
	}
	if !strings.Contains(user, "letter_forged") || !strings.Contains(user, "조조") || !strings.Contains(user, "曹操") || !strings.Contains(user, "장간은 …") {
		t.Error("catalog must list all fact ids, all names, and the prose")
	}
	for _, f := range s.Withheld {
		if !strings.Contains(user, f) || !strings.Contains(user, factText(s, f)) {
			t.Errorf("withheld fact %s must appear unlabeled in catalog", f)
		}
	}
}
