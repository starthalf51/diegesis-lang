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

// ParseFiles splits a bundle into files. Text before the first header is ignored (as unpack.sh does).
// Each content is trimmed of surrounding blank lines and ends with exactly one newline.
func ParseFiles(text string) (map[string]string, error) {
	files := map[string]string{}
	var cur string
	var body []string
	flush := func() error {
		if cur == "" {
			return nil
		}
		c := strings.Trim(strings.Join(body, "\n"), "\r\n")
		if strings.TrimSpace(c) == "" {
			return fmt.Errorf("file %q is empty", cur)
		}
		files[cur] = c + "\n"
		return nil
	}
	for _, l := range strings.Split(text, "\n") {
		l = strings.TrimRight(l, "\r")
		if strings.HasPrefix(l, hdrPre) && strings.HasSuffix(l, hdrPost) && len(l) >= len(hdrPre)+len(hdrPost) {
			if err := flush(); err != nil {
				return nil, err
			}
			cur = strings.TrimSuffix(strings.TrimPrefix(l, hdrPre), hdrPost)
			if err := validPath(cur); err != nil {
				return nil, err
			}
			if _, dup := files[cur]; dup {
				return nil, fmt.Errorf("duplicate file %q", cur)
			}
			files[cur] = "" // reserve so a repeat is caught before flush
			body = nil
			continue
		}
		body = append(body, l)
	}
	if err := flush(); err != nil {
		return nil, err
	}
	if len(files) == 0 {
		return nil, fmt.Errorf("no %q header found", hdrPre+"<path>"+hdrPost)
	}
	return files, nil
}

// WriteFiles empties dir, then writes the files. Every path is re-validated, so nothing lands outside dir.
func WriteFiles(dir string, files map[string]string) error {
	if dir == "" {
		return fmt.Errorf("empty dir")
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
func PackFiles(dir string) (string, error) {
	var paths []string
	err := filepath.WalkDir(dir, func(p string, d os.DirEntry, err error) error {
		if err == nil && !d.IsDir() {
			rel, _ := filepath.Rel(dir, p)
			paths = append(paths, filepath.ToSlash(rel))
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
