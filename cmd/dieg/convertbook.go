package main

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"

	"github.com/starthalf51/diegesis-lang/internal/write"
)

// cmdConvertBook runs `dieg convert-book <text> -o <dir> --chapter-re <re> [--chapters 1-36,133-135,epilogue]
// [--rounds N] [--code-model M] [--code-effort E] [--finish-model M] [--finish-rounds N] [--allow-warn]`: a long
// public-domain text becomes one work, <dir>/book/, converted chapter by chapter (write.ConvertBook). Parts already
// in <dir>/book/ are skipped. Exit 1 when a part ends unclean.
func cmdConvertBook(args []string) int {
	o := write.DefaultOpts()
	o.Render = false
	var file, re, chapters, styles string
	var limit int
	for i := 0; i < len(args); i++ {
		j, ok, code := commonFlag(args, i, &o, &styles, &limit)
		switch a := args[i]; {
		case code != 0:
			return code
		case ok && renderOnly[a]:
			fmt.Fprintf(os.Stderr, "dieg: convert-book does not render: %s is not accepted\n", a)
			return 2
		case ok:
			i = j
		case a == "-o" && i+1 < len(args):
			i++
			o.Out = args[i]
		case a == "--chapter-re" && i+1 < len(args):
			i++
			re = args[i]
		case a == "--chapters" && i+1 < len(args):
			i++
			chapters = args[i]
		case file == "" && !strings.HasPrefix(a, "-"):
			file = a
		default:
			return usage()
		}
	}
	if file == "" || o.Out == "" || re == "" || finishRoundsAlone(o) {
		return usage()
	}
	code, err := runConvertBook(file, re, chapters, o)
	if err != nil {
		fmt.Fprintln(os.Stderr, "dieg:", err)
		return 1
	}
	return code
}

var nonIdent = regexp.MustCompile(`[^a-z0-9_]`)

func runConvertBook(file, reSrc, chapters string, o write.Opts) (int, error) {
	re, err := regexp.Compile(reSrc)
	if err != nil {
		return 1, fmt.Errorf("--chapter-re: %w", err)
	}
	in, err := write.ParseChapters(chapters)
	if err != nil {
		return 1, err
	}
	b, err := os.ReadFile(file)
	if err != nil {
		return 1, err
	}
	all, err := write.SplitBook(string(b), re, 12000)
	if err != nil {
		return 1, err
	}
	var parts []write.Part
	for _, p := range all {
		if in(p.Chapter) {
			parts = append(parts, p)
		}
	}
	title := strings.TrimSuffix(filepath.Base(file), filepath.Ext(file))
	work := nonIdent.ReplaceAllString(strings.ToLower(title), "")
	if work == "" || work[0] < 'a' || work[0] > 'z' {
		work = "book" + work
	}
	refs, err := write.LoadRefs()
	if err != nil {
		return 1, err
	}
	if err := notUnderCorpus(o.Out); err != nil {
		return 1, err
	}
	if err := os.MkdirAll(o.Out, 0o755); err != nil {
		return 1, err
	}
	f, err := os.OpenFile(filepath.Join(o.Out, "calls.jsonl"), os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0o644)
	if err != nil {
		return 1, err
	}
	defer f.Close()
	m, _, k := meters(o, f)
	o = withFinish(o, m, k)
	o.Reserved = corpusWorks()
	for _, r := range o.Reserved {
		if work == r {
			return 1, fmt.Errorf("work id %q (from the file name) belongs to a corpus work; rename the file", work)
		}
	}
	unclean, err := write.ConvertBook(context.Background(), m, writeChecker(o.Reserved), work, title, o.Out, parts, refs, o)
	t := m.Total()
	fmt.Printf("convert-book: work=%s parts=%d of %d unclean=%d calls=%d tokens in=%d out=%d → %s\n",
		work, len(parts), len(all), unclean, k.n.Load(), t.Input, t.Output, filepath.Join(o.Out, "book"))
	if err != nil {
		return 1, err
	}
	if unclean > 0 {
		return 1, fmt.Errorf("%d part(s) not clean; see %s", unclean, filepath.Join(o.Out, "summary.tsv"))
	}
	return 0, nil
}
