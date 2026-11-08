package write

import (
	"context"
	"os"
	"path/filepath"
	"strings"

	"diegesis/internal/render"
)

// patchScene brings one stale scene's prose up to date by sentence-level patch (render.PatchVerified) from its old
// prose in out/novel and its pre-revision scene old. ok is false when there is no old prose to patch (the caller
// renders the scene in full). Like Novel, a failed call leaves no hash so the scene is retried.
func patchScene(ctx context.Context, cs render.Completers, work string, old, s render.Scene, b render.Backend, out string, o Opts) (sr SceneResult, ok bool, err error) {
	dir := filepath.Join(out, "novel")
	prose, rerr := os.ReadFile(filepath.Join(dir, fileID(s.ID)+"."+b.ID+".md"))
	if rerr != nil || strings.TrimSpace(string(prose)) == "" {
		return sr, false, nil
	}
	s.FreeDialogue = o.Skeleton
	digest, dh, derr := sceneDigest(ctx, o, s.ID)
	if derr != nil {
		return SceneResult{ID: s.ID, Err: derr}, true, nil
	}
	n0 := 0
	lc := render.Completers{Render: &sceneLabeler{c: cs.Render, scene: s.ID, kind: "render-", attempt: &n0}, Extract: &sceneLabeler{c: cs.Extract, scene: s.ID, kind: "extract-", attempt: &n0}}
	r, n, cerr := render.PatchVerified(ctx, lc, work, old, s, b, strings.TrimRight(string(prose), "\n"), digest,
		render.VerifyOpts{K: 1, Retries: 2, Verifier: o.Verifier, UserSuffix: digest})
	sr = SceneResult{ID: s.ID, Attempts: n, Err: cerr}
	if cerr != nil {
		return sr, true, nil
	}
	sr.Pass, sr.Report = render.Passed(r), r.Report
	return sr, true, saveScene(dir, s, b, o, dh, r)
}
