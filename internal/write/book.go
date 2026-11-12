package write

import (
	"context"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/starthalf51/diegesis-lang/internal/render"
	"github.com/starthalf51/diegesis-lang/internal/syntax"
)

// Part is one conversion pass of a book: chapter Chapter (0 = the epilogue) titled Title, piece Seq (0 when the
// chapter fits in one pass, else 1, 2, …), and its paragraphs joined by blank lines.
type Part struct {
	Chapter int
	Title   string
	Seq     int
	Text    string
}

var (
	epilogueRE = regexp.MustCompile(`(?i)^\W*epilogue`)
	headNum    = regexp.MustCompile(`\d+`)
)

// SplitBook cuts text at the lines chapterRE matches. Text before the first heading is dropped, and so is a
// heading with no text under it (a contents list entry). A chapter takes the first number in its heading
// ("CHAPTER 32. The Cetology" → 32), else its position among the kept chapters; an "Epilogue" heading is chapter 0.
// A chapter met twice is an error, not a silent drop: a contents list whose last entry is followed by front matter
// (Gutenberg's "Epilogue" then ETYMOLOGY) collides with the real chapter, so the caller strips the front matter.
// A chapter over max runes is split at paragraph boundaries into nearly even parts of at most max runes.
func SplitBook(text string, chapterRE *regexp.Regexp, max int) ([]Part, error) {
	type chapter struct {
		title string
		body  []string
	}
	var chs []chapter
	for _, l := range strings.Split(strings.ReplaceAll(text, "\r\n", "\n"), "\n") {
		if chapterRE.MatchString(l) {
			chs = append(chs, chapter{title: strings.TrimSpace(l)})
		} else if len(chs) > 0 {
			chs[len(chs)-1].body = append(chs[len(chs)-1].body, l)
		}
	}
	if len(chs) == 0 {
		return nil, fmt.Errorf("convert-book: no chapter headings match %q", chapterRE)
	}
	var parts []Part
	n := 0
	seen := map[string]string{} // part stem → heading, to refuse a chapter met twice
	for _, c := range chs {
		var ps []string
		for _, p := range blankLine.Split(strings.Join(c.body, "\n"), -1) {
			if p = strings.TrimSpace(p); p != "" {
				ps = append(ps, p)
			}
		}
		if len(ps) == 0 {
			continue
		}
		num := 0
		if !epilogueRE.MatchString(c.title) {
			n++
			num = n
			if d := headNum.FindString(c.title); d != "" {
				num, _ = strconv.Atoi(d)
			}
		}
		key := PartStem(Part{Chapter: num})
		if prev, dup := seen[key]; dup {
			return nil, fmt.Errorf("convert-book: chapter %q twice (headings %q and %q): strip the front matter before the first chapter (a contents list) or tighten --chapter-re", key, prev, c.title)
		}
		seen[key] = c.title
		all := strings.Join(ps, "\n\n")
		total := len([]rune(all))
		if total <= max {
			parts = append(parts, Part{Chapter: num, Title: c.title, Text: all})
			continue
		}
		k := (total + max - 1) / max
		target := (total + k - 1) / k
		var cur []string
		curN, seq := 0, 0
		flush := func() {
			seq++
			parts = append(parts, Part{Chapter: num, Title: c.title, Seq: seq, Text: strings.Join(cur, "\n\n")})
			cur, curN = nil, 0
		}
		for _, p := range ps {
			pn := len([]rune(p))
			if pn > max {
				return nil, fmt.Errorf("convert-book: %s: a paragraph of %d runes is over %d", c.title, pn, max)
			}
			if len(cur) > 0 && (curN+2+pn > max || curN >= target) {
				flush()
			}
			if len(cur) > 0 {
				curN += 2
			}
			cur, curN = append(cur, p), curN+pn
		}
		flush()
	}
	return parts, nil
}

// PartStem is the part's file stem and module suffix: ch001, ch032a, ch032b, epilogue.
func PartStem(p Part) string {
	s := fmt.Sprintf("ch%03d", p.Chapter)
	if p.Chapter == 0 {
		s = "epilogue"
	}
	if p.Seq > 0 {
		s += string(rune('a' + p.Seq - 1))
	}
	return s
}

// ParseChapters reads a --chapters list such as "1-36,133-135,epilogue" (epilogue = chapter 0); "" takes all.
func ParseChapters(spec string) (func(chapter int) bool, error) {
	if spec == "" {
		return func(int) bool { return true }, nil
	}
	type span struct{ lo, hi int }
	var spans []span
	for _, f := range strings.Split(spec, ",") {
		if f == "epilogue" {
			spans = append(spans, span{0, 0})
			continue
		}
		a, b, isRange := strings.Cut(f, "-")
		lo, err1 := strconv.Atoi(a)
		hi, err2 := lo, error(nil)
		if isRange {
			hi, err2 = strconv.Atoi(b)
		}
		if err1 != nil || err2 != nil || lo < 1 || hi < lo {
			return nil, fmt.Errorf("convert-book: --chapters %q: bad item %q (want N, N-M or epilogue)", spec, f)
		}
		spans = append(spans, span{lo, hi})
	}
	return func(c int) bool {
		for _, s := range spans {
			if s.lo <= c && c <= s.hi {
				return true
			}
		}
		return false
	}, nil
}

// ConvertBook converts parts into one work under <dir>/book/: book.dieg.md (module <work>.book) holds the only
// `work` declaration, and each part becomes <stem>.dieg.md (module <work>.<stem>, see PartStem) through
// ConvertIdea and Loop, its scratch in <dir>/runs/<stem>/. A part's prompt lists the IDs the book already
// declares and the modules to import; its round is checked together with the book, and redeclaring a book ID is a
// dup-decl error. Only clean parts are written to book/; a part whose file exists is skipped, so a rerun resumes.
// Each converted part appends a row to <dir>/summary.tsv. It returns how many parts ended unclean.
func ConvertBook(ctx context.Context, c render.Completer, check Checker, work, title, dir string, parts []Part, refs Refs, o Opts) (int, error) {
	book := filepath.Join(dir, "book")
	if err := os.MkdirAll(book, 0o755); err != nil {
		return 0, err
	}
	head := filepath.Join(book, "book.dieg.md")
	if _, err := os.Stat(head); err != nil {
		src := fmt.Sprintf("---\nmodule: %s.book\n---\n\n```sl\nwork %s \"%s\"\n```\n", work, work, strings.ReplaceAll(title, `"`, ""))
		if err := os.WriteFile(head, []byte(src), 0o644); err != nil {
			return 0, err
		}
	}
	sumPath := filepath.Join(dir, "summary.tsv")
	if _, err := os.Stat(sumPath); err != nil {
		if err := os.WriteFile(sumPath, []byte("chapter\tseq\tclean\trounds\tseconds\ttokens\n"), 0o644); err != nil {
			return 0, err
		}
	}
	unclean := 0
	for _, p := range parts {
		stem := PartStem(p)
		file := stem + ".dieg.md"
		if _, err := os.Stat(filepath.Join(book, file)); err == nil {
			continue
		}
		known, list, mods, err := bookDecls(book, file)
		if err != nil {
			return unclean, err
		}
		idea, n, err := ConvertIdea(title+" — "+p.Title, p.Text)
		if err != nil {
			return unclean, fmt.Errorf("%s: %w", stem, err)
		}
		idea += bookRules(work, title, p, file, mods, list)
		po := o
		po.Out = filepath.Join(dir, "runs", stem)
		po.Coverage = SrcCoverage
		if err := os.RemoveAll(po.Out); err != nil {
			return unclean, err
		}
		before, start := tokens(c), time.Now()
		res, err := Loop(ctx, c, CoverageChecker(partChecker(check, book, file, work+"."+stem, known), n), idea, refs, po)
		if err != nil {
			return unclean, fmt.Errorf("%s: %w", stem, err)
		}
		if res.Clean {
			b, err := os.ReadFile(filepath.Join(res.Dir, file))
			if err == nil {
				err = os.WriteFile(filepath.Join(book, file), b, 0o644)
			}
			if err != nil {
				return unclean, err
			}
		} else {
			unclean++
		}
		ch := strconv.Itoa(p.Chapter)
		if p.Chapter == 0 {
			ch = "epilogue"
		}
		row := fmt.Sprintf("%s\t%d\t%t\t%d\t%.0f\t%d\n", ch, p.Seq, res.Clean, res.Rounds, time.Since(start).Seconds(), tokens(c)-before)
		f, err := os.OpenFile(sumPath, os.O_APPEND|os.O_WRONLY, 0o644)
		if err == nil {
			_, err = f.WriteString(row)
			f.Close()
		}
		if err != nil {
			return unclean, err
		}
	}
	return unclean, nil
}

// tokens is c's input+output total so far when c is a Meter, else 0.
func tokens(c render.Completer) int64 {
	if m, ok := c.(*render.Meter); ok {
		t := m.Total()
		return int64(t.Input + t.Output)
	}
	return 0
}

// bookDecls reads the book's files other than skip: known maps each declared id to its module, list is one
// "kind id (module)" line per declaration and setup, mods the module names in file order.
func bookDecls(book, skip string) (known map[string]string, list, mods []string, err error) {
	es, err := os.ReadDir(book)
	if err != nil {
		return nil, nil, nil, err
	}
	known = map[string]string{}
	for _, e := range es {
		if e.IsDir() || e.Name() == skip || !syntax.IsSource(e.Name()) {
			continue
		}
		b, err := os.ReadFile(filepath.Join(book, e.Name()))
		if err != nil {
			return nil, nil, nil, err
		}
		f, _ := syntax.ParseFile(e.Name(), b)
		if f.Module == nil {
			continue
		}
		mod := f.Module.Name
		mods = append(mods, mod)
		for _, it := range f.Items {
			switch d := it.(type) {
			case *syntax.Decl:
				known[d.ID.Name] = mod
				list = append(list, fmt.Sprintf("%s %s (%s)", d.Kind, d.ID.Name, mod))
			case *syntax.Setup:
				list = append(list, fmt.Sprintf("setup %s (%s)", d.ID.Name, mod))
			}
		}
	}
	return known, list, mods, nil
}

// bookRules is what a part's idea adds to ConvertIdea's: one file, its module and imports, no `work`, and the IDs
// earlier parts declared.
func bookRules(work, title string, p Part, file string, mods, list []string) string {
	piece := ""
	if p.Seq > 0 {
		piece = fmt.Sprintf(" %d번째 조각", p.Seq)
	}
	ids := "(아직 없다)"
	if len(list) > 0 {
		ids = strings.Join(list, "\n")
	}
	return fmt.Sprintf(`
## 장편 변환 규칙 (위의 파일·work 규칙보다 우선한다)
이 원문은 장편 「%s」의 %s%s 이다. 책 전체가 작품 %s 하나로 컴파일된다.
- 파일은 `+"`%s`"+` 하나만 쓴다. cast.dieg.md 나 ep01/ 파일을 만들지 않는다. 선언과 장면을 모두 이 파일에 둔다.
- 프론트매터는 `+"`module: %s.%s`"+` 와 `+"`import: [%s]`"+` 이다.
- `+"`work`"+` 를 선언하지 않는다. 작품 %s 는 book.dieg.md 에 이미 선언되어 있다.
- 장면은 `+"```sl"+` 블록 안에 `+"`scene s1 at <장소>`"+` 머리줄과 들여 쓴 몸체로 쓴다. 장면 ID 는 이 파일 안에서 s1, s2 … 이다.

## 이미 선언된 ID (reuse these IDs, do not redeclare)
앞 부분들이 선언한 인물·장소·물건·사실이다. 같은 대상은 이 ID 를 그대로 쓰고 다시 선언하지 않는다. 다시 선언하면 dup-decl 오류다.
이 부분에 처음 나오는 대상만 이 파일에 선언한다.

%s
`, title, p.Title, piece, work, file, work, strings.TrimSuffix(file, ".dieg.md"), strings.Join(mods, ", "), work, ids)
}

// partChecker checks a round that must be exactly the one file `file`, together with the book's other files (in a
// temp dir), and adds a dup-decl error for each id the book already declares. A file whose module is not `module`
// is refused before checking: metrics group a work's parts by module name.
func partChecker(check Checker, book, file, module string, known map[string]string) Checker {
	return func(ctx context.Context, dir string) ([]Diag, error) {
		var extra []string
		filepath.WalkDir(dir, func(p string, d fs.DirEntry, err error) error {
			if rel, _ := filepath.Rel(dir, p); err == nil && !d.IsDir() && syntax.IsSource(p) && filepath.ToSlash(rel) != file {
				extra = append(extra, filepath.ToSlash(rel))
			}
			return nil
		})
		src, err := os.ReadFile(filepath.Join(dir, file))
		if err != nil || len(extra) > 0 {
			return bundleDiag(fmt.Sprintf("write exactly one file, %s; got %v", file, extra)), nil
		}
		f, _ := syntax.ParseFile(file, src)
		if f.Module == nil || f.Module.Name != module {
			got := "none"
			if f.Module != nil {
				got = f.Module.Name
			}
			return []Diag{{File: file, Line: 1, Col: 1, Severity: "error", Rule: "bundle",
				Msg: fmt.Sprintf("the frontmatter must say `module: %s` (got %s)", module, got)}}, nil
		}
		tmp, err := os.MkdirTemp("", "dieg-book-")
		if err != nil {
			return nil, err
		}
		defer os.RemoveAll(tmp)
		es, err := os.ReadDir(book)
		if err != nil {
			return nil, err
		}
		for _, e := range es {
			if e.IsDir() || e.Name() == file || !syntax.IsSource(e.Name()) {
				continue
			}
			b, err := os.ReadFile(filepath.Join(book, e.Name()))
			if err == nil {
				err = os.WriteFile(filepath.Join(tmp, e.Name()), b, 0o644)
			}
			if err != nil {
				return nil, err
			}
		}
		if err := os.WriteFile(filepath.Join(tmp, file), src, 0o644); err != nil {
			return nil, err
		}
		ds, err := check(ctx, tmp)
		if err != nil {
			return ds, err
		}
		for _, it := range f.Items {
			if d, ok := it.(*syntax.Decl); ok {
				if mod, dup := known[d.ID.Name]; dup {
					ds = append(ds, Diag{File: file, Line: d.ID.Line, Col: d.ID.Col, Severity: "error", Rule: "dup-decl",
						Msg: fmt.Sprintf("%q is already declared in module %s; reuse it, do not redeclare", d.ID.Name, mod)})
				}
			}
		}
		return ds, nil
	}
}
