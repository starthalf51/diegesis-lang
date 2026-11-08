package write

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
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
	Patched  bool          // the prose came from a sentence patch (see Report.Patched), not a full render
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

func renderScene(ctx context.Context, cs render.Completers, work string, s render.Scene, b render.Backend, o Opts, digest string) (render.Render, int, error) {
	n0 := 0
	return render.RenderVerified2(ctx, render.Completers{Render: &sceneLabeler{c: cs.Render, scene: s.ID, kind: "render-", attempt: &n0}, Extract: &sceneLabeler{c: cs.Extract, scene: s.ID, kind: "extract-", attempt: &n0}}, work, s, b, render.VerifyOpts{K: 1, Retries: 2, Verifier: o.Verifier, UserSuffix: digest})
}

// Novel renders scenes (already in discourse order) with RenderVerified2
// and writes <out>/novel/<scene>.<backend>.md and the concatenation <out>/novel.md. A scene that fails
// verification keeps its last render; a scene whose call fails gets no file. Neither stops the rest.
// o.Skeleton sets Scene.FreeDialogue.
func Novel(ctx context.Context, cs render.Completers, work string, scenes []render.Scene, b render.Backend, out string, o Opts) ([]SceneResult, error) {
	dir := filepath.Join(out, "novel")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return nil, err
	}
	var rs []SceneResult
	for _, s := range scenes {
		s.FreeDialogue = o.Skeleton
		digest, state, err := sceneDigest(ctx, o, s.ID)
		var r render.Render
		n := 0
		if err == nil {
			r, n, err = renderScene(ctx, cs, work, s, b, o, digest)
		}
		sr := SceneResult{ID: s.ID, Attempts: n, Err: err}
		if err == nil {
			sr.Pass, sr.Report = render.Passed(r), r.Report
			if err := saveScene(dir, s, b, o, state, r); err != nil {
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

// sceneDigest is o.Digest for scene id, blank-line-prefixed so it can be appended to a prompt; "" when there is none.
// The second result is the digest's state-only part (what staleness hashes and what a state patch diffs against).
func sceneDigest(ctx context.Context, o Opts, id string) (suffix, state string, err error) {
	if o.Digest == nil {
		return "", "", nil
	}
	d, st, err := o.Digest(ctx, id)
	if err != nil {
		return "", "", err
	}
	if strings.TrimSpace(d) != "" {
		suffix = "\n\n" + d
	}
	return suffix, st, nil
}

func digestHash(d string) string { h := sha256.Sum256([]byte(d)); return hex.EncodeToString(h[:8]) }

// stamp is the .hash content: "<irhash>" without a digest, else "<irhash> <digesthash>".
func stamp(ir string, o Opts, stateHash string) string {
	if o.Digest == nil {
		return ir
	}
	return ir + " " + stateHash
}

// saveScene writes a scene's prose, report, state text and staleness stamp into dir.
func saveScene(dir string, s render.Scene, b render.Backend, o Opts, state string, r render.Render) error {
	rep, _ := json.MarshalIndent(r.Report, "", "  ")
	base := filepath.Join(dir, fileID(s.ID)+"."+b.ID)
	if err := os.WriteFile(base+".report.json", append(rep, '\n'), 0o644); err != nil {
		return err
	}
	if err := os.WriteFile(base+".md", []byte(r.Prose+"\n"), 0o644); err != nil {
		return err
	}
	if h, err := genHash(s, b); err == nil {
		if err := writeState(base, o, state); err != nil {
			return err
		}
		return os.WriteFile(base+".hash", []byte(stamp(h, o, digestHash(state))+"\n"), 0o644)
	}
	return nil
}

// writeState stores the state digest text a scene was rendered under (<scene>.<backend>.state.txt, next to .hash), so a
// later state-stale patch can show the model which state lines changed. Without a digest there is nothing to store.
func writeState(base string, o Opts, state string) error {
	if o.Digest == nil {
		return nil
	}
	return os.WriteFile(base+".state.txt", []byte(state), 0o644)
}
