package write

import (
	"context"
	"fmt"
	"os"
	"path"
	"path/filepath"
	"sort"
	"strconv"
	"strings"

	"diegesis/internal/render"
	"diegesis/internal/syntax"
)

// ContinueLoop writes episode ep into storyDir (the accumulated story/ code): it asks c for new files under
// story/epNN/ only, writes them there, checks the whole storyDir (past + new) and feeds diagnostics back until clean
// or o.Rounds is spent. The prompt carries the cast files and o.Digest, not the past scene code. A bundle with a
// path outside epNN/ or one that already exists is a "bundle" diagnostic and consumes its round (Review Focus 1);
// past files are never written. A rejected round's epNN/ is removed again, so after an unclean run storyDir is
// as it was. Round r's raw output is <out>/rounds/epNN-r<r>.raw.txt and its files <out>/rounds/epNN-r<r>/;
// <out>/diags-epNN.txt holds the last well-formed round's diagnostics. ContinueLoop never renders.
func ContinueLoop(ctx context.Context, c render.Completer, check Checker, storyDir string, ep int, brief string, refs Refs, o Opts) (Result, error) {
	if o.Rounds <= 0 {
		o.Rounds = DefaultOpts().Rounds
	}
	if o.Out == "" {
		return Result{}, fmt.Errorf("continue: no output dir")
	}
	if ep < 2 || ep > 99 {
		return Result{}, fmt.Errorf("continue: episode %d out of range 2..99", ep)
	}
	d := EpDir(ep)
	epPath := filepath.Join(storyDir, d)
	if _, err := os.Stat(epPath); err == nil {
		return Result{}, fmt.Errorf("continue: %s already exists; episodes are written once", epPath)
	}
	cast, err := packFiles(storyDir, func(rel string) bool { return path.Base(rel) == "cast.dieg.md" })
	if err != nil {
		return Result{}, err
	}
	res := Result{Dir: storyDir}
	var last map[string]string
	var lastDiags []Diag
	var bundle string
	first := ContinuePrompt(o.Digest, cast, brief, ep, refs, o.Reserved, o.Skeleton)
	user := first
	for r := 1; r <= o.Rounds; r++ {
		l := d + "-code-r" + strconv.Itoa(r)
		if r > 1 {
			l = d + "-fix-r" + strconv.Itoa(r)
		}
		out, err := label(c, l).Complete(ctx, System, user)
		if err != nil {
			return res, fmt.Errorf("round %d: %w", r, err)
		}
		res.Rounds = r
		rdir := filepath.Join(o.Out, "rounds", d+"-r"+strconv.Itoa(r))
		if err := os.MkdirAll(filepath.Dir(rdir), 0o755); err != nil {
			return res, err
		}
		if err := os.WriteFile(rdir+".raw.txt", []byte(out.Text), 0o644); err != nil {
			return res, err
		}
		files, perr := ParseFiles(out.Text)
		if perr == nil {
			perr = newOnly(storyDir, d, files)
		}
		switch {
		case out.StopReason != "end_turn":
			res.Diags = bundleDiag(fmt.Sprintf("model output stopped early (%s); write every file and end with %s", out.StopReason, End))
		case perr != nil:
			res.Diags = bundleDiag(perr.Error())
		default:
			if err := WriteFiles(rdir, files); err != nil {
				return res, err
			}
			if bundle, err = PackFiles(rdir); err != nil {
				return res, err
			}
			own := map[string]string{}
			for p, c := range files {
				own[strings.TrimPrefix(p, d+"/")] = c
			}
			if err := WriteFiles(epPath, own); err != nil {
				os.RemoveAll(epPath)
				return res, err
			}
			res.Diags, err = check(ctx, storyDir)
			if err != nil {
				os.RemoveAll(epPath)
				return res, fmt.Errorf("round %d check: %w", r, err)
			}
			last, lastDiags = files, res.Diags
			res.Clean = true
			for _, x := range res.Diags {
				res.Clean = res.Clean && !x.Blocking()
			}
			if !res.Clean {
				if err := os.RemoveAll(epPath); err != nil {
					return res, err
				}
			}
		}
		if res.Clean {
			break
		}
		user = first + fixTail(bundle, blocking(res.Diags))
	}
	if res.Clean {
		res.Scenes = sceneIDs(last)
	}
	if last == nil {
		lastDiags = res.Diags
	}
	var sb strings.Builder
	for _, x := range lastDiags {
		sb.WriteString(x.String() + "\n")
	}
	return res, os.WriteFile(filepath.Join(o.Out, "diags-"+d+".txt"), []byte(sb.String()), 0o644)
}

// newOnly refuses any path outside d/ or already present under storyDir.
func newOnly(storyDir, d string, files map[string]string) error {
	var bad []string
	for p := range files {
		if !strings.HasPrefix(p, d+"/") {
			bad = append(bad, p+" (outside "+d+"/)")
		} else if _, err := os.Stat(filepath.Join(storyDir, filepath.FromSlash(p))); err == nil {
			bad = append(bad, p+" (exists)")
		}
	}
	if len(bad) == 0 {
		return nil
	}
	sort.Strings(bad)
	return fmt.Errorf("continue writes only new files under %s/; past files and other paths are refused: %s", d, strings.Join(bad, ", "))
}

// sceneIDs lists the scene ids the files declare.
func sceneIDs(files map[string]string) []string {
	var ps, ids []string
	for p := range files {
		ps = append(ps, p)
	}
	sort.Strings(ps)
	for _, p := range ps {
		f, _ := syntax.ParseFile(p, []byte(files[p]))
		for _, it := range f.Items {
			if s, ok := it.(*syntax.Scene); ok {
				ids = append(ids, s.ID.Name)
			}
		}
	}
	return ids
}

// Only keeps the scenes of all whose id is in ids, in all's order.
func Only(all []render.Scene, ids []string) []render.Scene {
	var out []render.Scene
	for _, s := range all {
		for _, id := range ids {
			if s.ID == id {
				out = append(out, s)
				break
			}
		}
	}
	return out
}
