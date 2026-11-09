package write

import (
	"context"
	"os"
	"path/filepath"
	"strings"

	"github.com/starthalf51/diegesis-lang/internal/render"
)

// patchScene brings one stale scene's prose up to date by sentence-level patch (render.PatchVerified) from its old
// prose in out/novel and its pre-revision scene old. ok is false when there is no old prose to patch (the caller
// renders the scene in full). Like Novel, a failed call leaves no hash so the scene is retried.
// oldDig != "" marks a state-stale scene (same request, only the PG state digest changed): its diff is the state lines
// that left/entered the digest, read from the stored <scene>.<backend>.state.txt. Without that file (legacy), or when it
// does not match oldDig, or when no line differs, there is nothing to tell the model: ok is false.
func patchScene(ctx context.Context, cs render.Completers, work string, old, s render.Scene, b render.Backend, out string, o Opts, oldDig string) (sr SceneResult, ok bool, err error) {
	dir := filepath.Join(out, "novel")
	base := filepath.Join(dir, fileID(s.ID)+"."+b.ID)
	prose, rerr := os.ReadFile(base + ".md")
	if rerr != nil || strings.TrimSpace(string(prose)) == "" {
		return sr, false, nil
	}
	s.FreeDialogue = o.Skeleton
	digest, state, derr := sceneDigest(ctx, o, s.ID)
	if derr != nil {
		return SceneResult{ID: s.ID, Err: derr}, true, nil
	}
	vo := render.VerifyOpts{K: 1, Retries: 2, Verifier: o.Verifier, UserSuffix: digest}
	if oldDig != "" {
		oldState, serr := os.ReadFile(base + ".state.txt")
		if serr != nil || digestHash(string(oldState)) != oldDig {
			return sr, false, nil
		}
		if vo.StateRm, vo.StateAd = render.StateDiff(string(oldState), state); len(vo.StateRm)+len(vo.StateAd) == 0 {
			return sr, false, nil
		}
	}
	n0 := 0
	lc := render.Completers{Render: &sceneLabeler{c: cs.Render, scene: s.ID, kind: "render-", attempt: &n0}, Extract: &sceneLabeler{c: cs.Extract, scene: s.ID, kind: "extract-", attempt: &n0}}
	r, n, cerr := render.PatchVerified(ctx, lc, work, old, s, b, strings.TrimRight(string(prose), "\n"), digest, vo)
	sr = SceneResult{ID: s.ID, Attempts: n, Err: cerr}
	if cerr != nil {
		return sr, true, nil
	}
	sr.Pass, sr.Report, sr.Patched = render.Passed(r), r.Report, r.Report.Patched
	return sr, true, saveScene(dir, s, b, o, state, r)
}

// proseMatches: the stored prose was rendered from old iff its stored IR hash is old's genHash (or legacy RenderHash). After a --scenes cap
// or --no-render the disk prose can be older than old, and a patch would then ship stale prose.
func proseMatches(old render.Scene, o Opts, b render.Backend, stored string) bool {
	old.FreeDialogue = o.Skeleton
	h, err := genHash(old, b)
	full, _ := render.RenderHash(old, b) // legacy stamp
	return err == nil && stored != "" && (h == stored || full == stored)
}
