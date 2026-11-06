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
// A call is a render when its system prompt is the backend's render system prompt.
type sceneLabeler struct {
	c          render.Completer
	scene, sys string
	attempt    int
}

func (l *sceneLabeler) Complete(ctx context.Context, system, user string) (render.Result, error) {
	kind := "extract-"
	if system == l.sys {
		l.attempt++
		kind = "render-"
	}
	return label(l.c, kind+l.scene+"-a"+strconv.Itoa(l.attempt)).Complete(ctx, system, user)
}

// Novel renders scenes (already in discourse order) with RenderVerified and writes <out>/novel/<scene>.<backend>.md
// and the concatenation <out>/novel.md. A scene that fails verification keeps its last render; a scene whose
// call fails gets no file. Neither stops the rest.
func Novel(ctx context.Context, c render.Completer, work string, scenes []render.Scene, b render.Backend, out string) ([]SceneResult, error) {
	dir := filepath.Join(out, "novel")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return nil, err
	}
	var rs []SceneResult
	var novel strings.Builder
	for _, s := range scenes {
		sys, _, err := b.Prompt(s)
		if err != nil {
			return rs, err
		}
		r, n, err := render.RenderVerified(ctx, &sceneLabeler{c: c, scene: s.ID, sys: sys}, work, s, b, render.VerifyOpts{K: 1, Retries: 2})
		sr := SceneResult{ID: s.ID, Attempts: n, Err: err}
		if err == nil {
			sr.Pass = render.Passed(r)
			sr.Report = r.Report
			rep, _ := json.MarshalIndent(r.Report, "", "  ")
			if err := os.WriteFile(filepath.Join(dir, s.ID+"."+b.ID+".report.json"), append(rep, '\n'), 0o644); err != nil {
				return rs, err
			}
			if err := os.WriteFile(filepath.Join(dir, s.ID+"."+b.ID+".md"), []byte(r.Prose+"\n"), 0o644); err != nil {
				return rs, err
			}
			title := s.Title
			if title == "" {
				title = s.ID
			}
			novel.WriteString("## " + title + "\n\n" + strings.TrimSpace(r.Prose) + "\n\n")
		}
		rs = append(rs, sr)
	}
	return rs, os.WriteFile(filepath.Join(out, "novel.md"), []byte(novel.String()), 0o644)
}
