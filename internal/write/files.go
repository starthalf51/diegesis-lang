// Package write handles the AI-output file bundle: "=== FILE: <path> ===" header lines, same as
// examples/idea-to-novel/prompts/unpack.sh.
package write

import (
	"fmt"
	"os"
	"path"
	"path/filepath"
	"sort"
	"strings"
)

const (
	hdrPre  = "=== FILE: "
	hdrPost = " ==="
	delPre  = "=== DELETE: "
)

func validPath(p string) error {
	if p == "" || strings.ContainsAny(p, `\:`) || strings.HasPrefix(p, "/") || path.Clean(p) != p {
		return fmt.Errorf("bad file path %q", p)
	}
	for _, seg := range strings.Split(p, "/") {
		if seg == ".." {
			return fmt.Errorf("bad file path %q", p)
		}
	}
	for _, ext := range []string{".dieg.md", ".md", ".yaml"} {
		if strings.HasSuffix(p, ext) {
			return nil
		}
	}
	return fmt.Errorf("file %q: extension must be .dieg.md, .md or .yaml", p)
}

// End is the sentinel line that closes a bundle; everything after it (model chatter, a closing fence) is dropped.
const End = "=== END ==="

func isFence(l string) bool { return strings.HasPrefix(strings.TrimSpace(l), "```") }

// dropWrapperFence removes a trailing fence line left by a model that wrapped the whole bundle in ``` … ```.
// A file's own fences (```sl … ```) are balanced, so an odd count means the last one is the wrapper's.
func dropWrapperFence(body []string) []string {
	n := 0
	for _, l := range body {
		if isFence(l) {
			n++
		}
	}
	i := len(body) - 1
	for i >= 0 && strings.TrimSpace(body[i]) == "" {
		i--
	}
	if n%2 == 1 && i >= 0 && isFence(body[i]) {
		return body[:i]
	}
	return body
}

// ParseFiles splits a bundle into files. Text before the first header (chatter, an opening ``` fence) is
// ignored, as unpack.sh does; text after the End line is dropped. Without End, an unbalanced trailing fence
// line is stripped from the last file. Each content is trimmed of surrounding blank lines and ends with exactly
// one newline. A bundle with a DELETE header is refused: write and continue never delete (Review Focus 1).
func ParseFiles(text string) (map[string]string, error) {
	files, deletes, err := ParseBundle(text)
	if err == nil && len(deletes) > 0 {
		return nil, fmt.Errorf("%q headers are not allowed here: this command never deletes files (%s)", delPre+"<path>"+hdrPost, strings.Join(deletes, ", "))
	}
	return files, err
}

// ParseBundle is ParseFiles plus "=== DELETE: <path> ===" header lines (no body), whose paths get the same
// validation. A path may not be both written and deleted.
func ParseBundle(text string) (files map[string]string, deletes []string, err error) {
	files = map[string]string{}
	lower := map[string]string{}
	var cur string
	var body []string
	flush := func() error {
		if cur == "" {
			return nil
		}
		body = dropWrapperFence(body)
		c := strings.Trim(strings.Join(body, "\n"), "\r\n")
		if strings.TrimSpace(c) == "" {
			return fmt.Errorf("file %q is empty", cur)
		}
		files[cur] = c + "\n"
		return nil
	}
	hdr := func(l, pre string) (string, bool) {
		if strings.HasPrefix(l, pre) && strings.HasSuffix(l, hdrPost) && len(l) >= len(pre)+len(hdrPost) {
			return strings.TrimSuffix(strings.TrimPrefix(l, pre), hdrPost), true
		}
		return "", false
	}
	for _, l := range strings.Split(text, "\n") {
		l = strings.TrimRight(l, "\r")
		if strings.TrimSpace(l) == End {
			break
		}
		p, isFile := hdr(l, hdrPre)
		d, isDel := hdr(l, delPre)
		if !isFile && !isDel {
			body = append(body, l)
			continue
		}
		if err := flush(); err != nil {
			return nil, nil, err
		}
		cur, body = "", nil
		p += d
		if err := validPath(p); err != nil {
			return nil, nil, err
		}
		low := strings.ToLower(p)
		if prev, ok := lower[low]; ok {
			if prev == p {
				return nil, nil, fmt.Errorf("duplicate file %q", p)
			}
			return nil, nil, fmt.Errorf("files %q and %q differ only in case", prev, p)
		}
		lower[low] = p
		if isDel {
			deletes = append(deletes, p)
			continue
		}
		cur = p
		files[cur] = "" // reserve so a repeat is caught before flush
	}
	if err := flush(); err != nil {
		return nil, nil, err
	}
	if len(files) == 0 && len(deletes) == 0 {
		return nil, nil, fmt.Errorf("no %q header found", hdrPre+"<path>"+hdrPost)
	}
	sort.Strings(deletes)
	return files, deletes, nil
}

// safeToEmpty refuses dirs whose RemoveAll would destroy more than a generated bundle: "", ".", a filesystem
// root, a dir holding .git (a repo root), and the cwd or any of its ancestors.
func safeToEmpty(dir string) error {
	if dir == "" || filepath.Clean(dir) == "." {
		return fmt.Errorf("refusing to empty %q", dir)
	}
	abs, err := filepath.Abs(dir)
	if err != nil {
		return err
	}
	if filepath.Dir(abs) == abs {
		return fmt.Errorf("refusing to empty filesystem root %q", abs)
	}
	if _, err := os.Stat(filepath.Join(abs, ".git")); err == nil {
		return fmt.Errorf("refusing to empty %q: it holds .git", abs)
	}
	cwd, err := os.Getwd()
	if err != nil {
		return err
	}
	if rel, err := filepath.Rel(abs, cwd); err == nil && rel != ".." && !strings.HasPrefix(rel, ".."+string(filepath.Separator)) {
		return fmt.Errorf("refusing to empty %q: it holds the working directory", abs)
	}
	return nil
}

// WriteFiles empties dir, then writes the files. Every path is re-validated, so nothing lands outside dir.
func WriteFiles(dir string, files map[string]string) error {
	if err := safeToEmpty(dir); err != nil {
		return err
	}
	for p := range files {
		if err := validPath(p); err != nil {
			return err
		}
	}
	if err := os.RemoveAll(dir); err != nil {
		return err
	}
	for p, c := range files {
		full := filepath.Join(dir, filepath.FromSlash(p))
		if err := os.MkdirAll(filepath.Dir(full), 0o755); err != nil {
			return err
		}
		if err := os.WriteFile(full, []byte(c), 0o644); err != nil {
			return err
		}
	}
	return os.MkdirAll(dir, 0o755)
}

// PackFiles bundles every file under dir in sorted path order, for feeding back to the model.
func PackFiles(dir string) (string, error) { return packFiles(dir, nil) }

// packFiles is PackFiles over the paths keep accepts (nil keeps all).
func packFiles(dir string, keep func(rel string) bool) (string, error) {
	var paths []string
	err := filepath.WalkDir(dir, func(p string, d os.DirEntry, err error) error {
		if err == nil && !d.IsDir() {
			rel, _ := filepath.Rel(dir, p)
			if rel = filepath.ToSlash(rel); keep == nil || keep(rel) {
				paths = append(paths, rel)
			}
		}
		return err
	})
	if err != nil {
		return "", err
	}
	sort.Strings(paths)
	var sb strings.Builder
	for _, p := range paths {
		b, err := os.ReadFile(filepath.Join(dir, filepath.FromSlash(p)))
		if err != nil {
			return "", err
		}
		c := string(b)
		if !strings.HasSuffix(c, "\n") {
			c += "\n"
		}
		sb.WriteString(hdrPre + p + hdrPost + "\n" + c)
	}
	return sb.String(), nil
}

// ResetRun removes out/novel and out/rounds left by an earlier run, so stale scene files and higher round dirs
// cannot survive into this one. Each is checked with safeToEmpty first.
func ResetRun(out string) error {
	if err := safeToEmpty(out); err != nil {
		return err
	}
	for _, n := range []string{"novel", "rounds"} {
		d := filepath.Join(out, n)
		if err := safeToEmpty(d); err != nil {
			return err
		}
		if err := os.RemoveAll(d); err != nil {
			return err
		}
	}
	return nil
}
