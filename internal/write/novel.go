package write

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"diegesis/internal/render"
)

// SceneResult is one scene's verified render: Attempts renders, Pass per render.Passed, Err if a call failed.
type SceneResult struct {
	ID       string
	Attempts int
	Pass     bool
	Err      error
	Report   render.Report // last attempt's verification report
}

// sceneLabeler tags render calls render-<scene>-a<n> and the extraction calls that follow extract-<scene>-a<n>.
type sceneLabeler struct {
	c           render.Completer
	scene, kind string // kind: "render-" bumps the shared attempt counter, "extract-" reuses it
	attempt     *int
}

func (l *sceneLabeler) Complete(ctx context.Context, system, user string) (render.Result, error) {
	if l.kind == "render-" {
		*l.attempt++
	}
	return label(l.c, l.kind+l.scene+"-a"+strconv.Itoa(*l.attempt)).Complete(ctx, system, user)
}

// Novel renders scenes (already in discourse order) with RenderVerified2 and writes <out>/novel/<scene>.<backend>.md
// and the concatenation <out>/novel.md. A scene that fails verification keeps its last render; a scene whose
// call fails gets no file. Neither stops the rest.
func Novel(ctx context.Context, cs render.Completers, work string, scenes []render.Scene, b render.Backend, out string, skeleton bool) ([]SceneResult, error) {
	dir := filepath.Join(out, "novel")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return nil, err
	}
	var rs []SceneResult
	for _, s := range scenes {
		s.FreeDialogue = skeleton
		n0 := 0
		r, n, err := render.RenderVerified2(ctx, render.Completers{Render: &sceneLabeler{c: cs.Render, scene: s.ID, kind: "render-", attempt: &n0}, Extract: &sceneLabeler{c: cs.Extract, scene: s.ID, kind: "extract-", attempt: &n0}}, work, s, b, render.VerifyOpts{K: 1, Retries: 2})
		sr := SceneResult{ID: s.ID, Attempts: n, Err: err}
		if err == nil {
			sr.Pass = render.Passed(r)
			sr.Report = r.Report
			rep, _ := json.MarshalIndent(r.Report, "", "  ")
			if err := os.WriteFile(filepath.Join(dir, fileID(s.ID)+"."+b.ID+".report.json"), append(rep, '\n'), 0o644); err != nil {
				return rs, err
			}
			if err := os.WriteFile(filepath.Join(dir, fileID(s.ID)+"."+b.ID+".md"), []byte(r.Prose+"\n"), 0o644); err != nil {
				return rs, err
			}
		}
		rs = append(rs, sr)
	}
	return rs, Concat(out, scenes, b.ID)
}

// Concat writes <out>/novel.md from the scene files <out>/novel/<scene>.<backend>.md in scenes' order (discourse
// order); a scene without a file is left out.
func Concat(out string, scenes []render.Scene, backend string) error {
	var novel strings.Builder
	for _, s := range scenes {
		p, err := os.ReadFile(filepath.Join(out, "novel", fileID(s.ID)+"."+backend+".md"))
		if os.IsNotExist(err) {
			continue
		}
		if err != nil {
			return err
		}
		title := s.Title
		if title == "" {
			title = s.ID
		}
		novel.WriteString("## " + title + "\n\n" + strings.TrimSpace(string(p)) + "\n\n")
	}
	return os.WriteFile(filepath.Join(out, "novel.md"), []byte(novel.String()), 0o644)
}

// fileID makes a scene id safe as a file name part.
func fileID(id string) string {
	return strings.Map(func(r rune) rune {
		if strings.ContainsRune(`/\?:*"<>|`, r) {
			return '_'
		}
		return r
	}, id)
}
