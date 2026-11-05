package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestRunJobSkipsExisting(t *testing.T) {
	dir := t.TempDir()
	fake := &fakeCompleter{replies: []Result{{Text: "prose", StopReason: "end_turn"}, {Text: validJSON, StopReason: "end_turn"}}}
	j := testJob(t)
	if _, err := RunJob(ctx, fake, j, dir, false); err != nil {
		t.Fatal(err)
	}
	r, err := RunJob(ctx, fake, j, dir, false)
	if err != nil || !r.Skipped || fake.calls != 2 {
		t.Fatalf("err=%v skipped=%v calls=%d", err, r.Skipped, fake.calls)
	}
	b, _ := os.ReadFile(filepath.Join(dir, j.Key()+".json"))
	if strings.Contains(string(b), `"skipped": true`) || strings.Contains(string(b), `"skipped":true`) {
		t.Error("skipped must not be persisted true")
	}
	md, _ := os.ReadFile(filepath.Join(dir, j.Key()+".md"))
	if !strings.HasPrefix(string(md), "---\nscene: ") || !strings.Contains(string(md), "mode: rendered\n---\n\nprose") {
		t.Errorf("md: %q", md)
	}
}

func TestRunJobRefusalNoExtract(t *testing.T) {
	fake := &fakeCompleter{replies: []Result{{StopReason: "refusal"}}}
	r, _ := RunJob(ctx, fake, testJob(t), t.TempDir(), false)
	if fake.calls != 1 || !strings.Contains(r.Report.ExtractError, "refusal") {
		t.Fatalf("%+v", r)
	}
	fake = &fakeCompleter{replies: []Result{{Text: "x"}}}
	r, _ = RunJob(ctx, fake, testJob(t), t.TempDir(), false)
	if r.Report.ExtractError != "render stopped: <empty>" {
		t.Fatalf("%+v", r)
	}
}

func TestRunJobBadExtractFailsClosed(t *testing.T) {
	fake := &fakeCompleter{replies: []Result{{Text: "prose", StopReason: "end_turn"}, {Text: "not json", StopReason: "end_turn"}}}
	r, err := RunJob(ctx, fake, testJob(t), t.TempDir(), false)
	if err != nil || r.Report.ExtractError == "" {
		t.Fatalf("%v %+v", err, r)
	}
	fake = &fakeCompleter{replies: []Result{{Text: "prose", StopReason: "end_turn"}, {Text: validJSON, StopReason: "max_tokens"}}}
	r, _ = RunJob(ctx, fake, testJob(t), t.TempDir(), false)
	if !strings.Contains(r.Report.ExtractError, "extract stopped: max_tokens") {
		t.Fatalf("%+v", r)
	}
}

func TestRunJobCompleterErrorWritesNothing(t *testing.T) {
	dir := t.TempDir()
	for _, f := range []*fakeCompleter{
		{err: os.ErrDeadlineExceeded},
		{replies: []Result{{Text: "prose", StopReason: "end_turn"}}}, // extract call errors (out of replies)
	} {
		if _, err := RunJob(ctx, f, testJob(t), dir, false); err == nil {
			t.Fatal("want error")
		}
	}
	if es, _ := os.ReadDir(dir); len(es) != 0 {
		t.Fatalf("wrote %d files", len(es))
	}
}

func TestWriteResultsMarksUnknown(t *testing.T) {
	p := filepath.Join(t.TempDir(), "results.md")
	recs := []Record{
		{Key: "z.a.en", Report: Report{Missing: []string{"f"}}, RenderCost: 0.1, ExtractCost: 0.05},
		{Key: "b.a.en", Report: Report{ExtractError: "render stopped: refusal", Missing: []string{"x"}}},
	}
	if err := WriteResults(recs, p); err != nil {
		t.Fatal(err)
	}
	b, _ := os.ReadFile(p)
	s := string(b)
	if !strings.Contains(s, "| key | 손실 | 환각 | 누출 | 추출오류 | 비용($) | 판정(1-5) |") ||
		!strings.Contains(s, "| b.a.en | ? | ? | ? | render stopped: refusal | 0.0000 |  |") ||
		!strings.Contains(s, "| z.a.en | 1 | 0 | 0 |  | 0.1500 |  |") ||
		!strings.Contains(s, "총비용: $0.1500") || strings.Index(s, "b.a.en") > strings.Index(s, "z.a.en") {
		t.Errorf("table:\n%s", s)
	}
}

func TestJobsCount(t *testing.T) {
	var scenes []Scene
	for _, n := range []string{"chibi-jianggan", "holmes-scandal-opening", "macbeth-1.3"} {
		scenes = append(scenes, mustLoad(t, "testdata/scenes/"+n+".json"))
	}
	styles := []StyleSample{mustStyle(t, "testdata/styles/doyle-redheaded.json"), mustStyle(t, "testdata/styles/hyunjingeon-unsu.json")}
	vs := []Variant{VariantIROnly, VariantStyle, VariantStyleDirectives}
	jobs, err := Jobs(scenes, styles, vs, []string{"ko", "en"})
	if err != nil || len(jobs) != 18 {
		t.Fatal(err, len(jobs))
	}
	seen := map[string]bool{}
	for _, j := range jobs {
		seen[j.Key()] = true
	}
	if len(seen) != 18 || !seen["macbeth-1.3.c.ko"] {
		t.Errorf("keys: %v", seen)
	}
	if _, err := Jobs(scenes, styles[:1], vs, []string{"ko"}); err == nil {
		t.Error("want error when no style for lang")
	}
}
