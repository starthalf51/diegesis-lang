package write

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"sort"
	"strconv"
	"strings"

	"diegesis/internal/render"
)

// SceneHashes maps each scene id to its render.RenderHash: the render request as the backend would send it.
func SceneHashes(scenes []render.Scene, b render.Backend) (map[string]string, error) {
	m := map[string]string{}
	for _, s := range scenes {
		h, err := render.RenderHash(s, b)
		if err != nil {
			return nil, fmt.Errorf("scene %s: %w", s.ID, err)
		}
		m[s.ID] = h
	}
	return m, nil
}

// ChangedScenes compares two SceneHashes maps: ids in both with a different hash, ids only in after, ids only in
// before. Each list is sorted.
func ChangedScenes(before, after map[string]string) (changed, added, removed []string) {
	for id, h := range after {
		switch old, ok := before[id]; {
		case !ok:
			added = append(added, id)
		case old != h:
			changed = append(changed, id)
		}
	}
	for id := range before {
		if _, ok := after[id]; !ok {
			removed = append(removed, id)
		}
	}
	sort.Strings(changed)
	sort.Strings(added)
	sort.Strings(removed)
	return
}

// readTree reads every regular file under dir into a path -> content map.
func readTree(dir string) (map[string]string, error) {
	m := map[string]string{}
	err := filepath.WalkDir(dir, func(p string, d os.DirEntry, err error) error {
		if err != nil || !d.Type().IsRegular() {
			return err
		}
		b, err := os.ReadFile(p)
		rel, _ := filepath.Rel(dir, p)
		m[filepath.ToSlash(rel)] = string(b)
		return err
	})
	return m, err
}

// ReviseLoop edits the code in storyDir to satisfy request. The prompt carries the whole current code (an edit has
// to stay consistent with the past episodes) and the model answers with only the files it changes or adds, plus
// "=== DELETE: <path> ===" lines. Each well-formed round is applied to a candidate copy, <out>/rounds/rev<k>-r<r>/,
// which the checker checks; the next round starts from that candidate. Only when the candidate is clean is it
// swapped into storyDir; otherwise storyDir is untouched. A DELETE of a missing path is a "bundle" diagnostic.
// <out>/diags-revise.txt holds the last well-formed round's diagnostics. ReviseLoop never renders.
func ReviseLoop(ctx context.Context, c render.Completer, check Checker, storyDir, request string, refs Refs, o Opts) (_ Result, rerr error) {
	if o.Rounds <= 0 {
		o.Rounds = DefaultOpts().Rounds
	}
	if o.Out == "" {
		return Result{}, fmt.Errorf("revise: no output dir")
	}
	if strings.TrimSpace(request) == "" {
		return Result{}, fmt.Errorf("revise: empty request")
	}
	cur, err := readTree(storyDir)
	if err != nil {
		return Result{}, err
	}
	res := Result{Dir: storyDir}
	var lastDiags, diags []Diag
	wellFormed, checked := false, false
	defer func() { // an error after a check ran leaves PG on a rejected candidate: put it back on story/
		if rerr != nil && checked {
			check(ctx, storyDir)
		}
	}()
	rev := "rev" + strconv.Itoa(1+countRevs(o.Out)) // revision k never overwrites revision k-1's rounds
	for r := 1; r <= o.Rounds; r++ {
		l := rev + "-code-r1"
		if r > 1 {
			l = rev + "-fix-r" + strconv.Itoa(r)
		}
		out, err := label(c, l).Complete(ctx, System, RevisePrompt(request, packMap(cur), diags, refs, o.Reserved, o.Skeleton))
		if err != nil {
			return res, fmt.Errorf("round %d: %w", r, err)
		}
		res.Rounds = r
		rdir := filepath.Join(o.Out, "rounds", rev+"-r"+strconv.Itoa(r))
		if err := os.MkdirAll(filepath.Dir(rdir), 0o755); err != nil {
			return res, err
		}
		if err := os.WriteFile(rdir+".raw.txt", []byte(out.Text), 0o644); err != nil {
			return res, err
		}
		files, dels, perr := ParseBundle(out.Text)
		if perr == nil {
			perr = checkAgainst(cur, files)
		}
		for _, d := range dels {
			if _, ok := cur[d]; perr == nil && (!ok || !strings.HasSuffix(d, ".dieg.md")) {
				perr = fmt.Errorf("DELETE %q: no such file in the current code", d)
			}
		}
		switch {
		case out.StopReason != "end_turn":
			res.Diags = bundleDiag(fmt.Sprintf("model output stopped early (%s); write every changed file and end with %s", out.StopReason, End))
		case perr != nil:
			res.Diags = bundleDiag(perr.Error())
		default:
			next := make(map[string]string, len(cur)+len(files))
			for p, c := range cur {
				next[p] = c
			}
			for _, d := range dels {
				delete(next, d)
			}
			for p, c := range files {
				next[p] = c
			}
			if err := writeTree(rdir, next); err != nil {
				return res, err
			}
			checked = true
			if res.Diags, err = check(ctx, rdir); err != nil {
				return res, fmt.Errorf("round %d check: %w", r, err)
			}
			cur, lastDiags, wellFormed = next, res.Diags, true
			res.Clean = true
			for _, x := range res.Diags {
				res.Clean = res.Clean && !x.Blocking()
			}
		}
		if res.Clean {
			break
		}
		diags = blocking(res.Diags)
	}
	if res.Clean {
		if err := swapDir(storyDir, cur); err != nil {
			return res, err
		}
	}
	if !wellFormed {
		lastDiags = res.Diags
	}
	var sb strings.Builder
	for _, x := range lastDiags {
		sb.WriteString(x.String() + "\n")
	}
	return res, os.WriteFile(filepath.Join(o.Out, "diags-revise.txt"), []byte(sb.String()), 0o644)
}

var rename = os.Rename // a variable so a test can fail a step

// swapDir replaces dir by files without a window in which dir is half written: write dir.new, move dir to
// dir.old, move dir.new in, drop dir.old. If a step fails the old dir is put back.
func swapDir(dir string, files map[string]string) error {
	nw, old := dir+".new", dir+".old"
	if err := safeToEmpty(old); err != nil {
		return err
	}
	if err := os.RemoveAll(old); err != nil {
		return err
	}
	if err := writeTree(nw, files); err != nil {
		return err
	}
	if err := rename(dir, old); err != nil {
		os.RemoveAll(nw)
		return err
	}
	if err := rename(nw, dir); err != nil {
		if rerr := rename(old, dir); rerr != nil {
			return fmt.Errorf("%w (restoring failed: %v; previous story kept at %s)", err, rerr, old)
		}
		os.RemoveAll(nw)
		return err
	}
	return os.RemoveAll(old)
}

// countRevs is the number of revision dirs (rounds/rev<k>-r*) an earlier revise left in out.
func countRevs(out string) int {
	ents, _ := os.ReadDir(filepath.Join(out, "rounds"))
	n := 0
	for _, e := range ents {
		if e.IsDir() && strings.HasPrefix(e.Name(), "rev") {
			n++
		}
	}
	return n
}

// checkAgainst rejects a model file that overwrites a file the model does not own (anything but .dieg.md) or that
// equals an existing path up to case.
func checkAgainst(cur, files map[string]string) error {
	lower := map[string]string{}
	for p := range cur {
		lower[strings.ToLower(p)] = p
	}
	for p := range files {
		if prev, ok := lower[strings.ToLower(p)]; ok {
			if prev != p {
				return fmt.Errorf("file %q differs only in case from existing %q", p, prev)
			}
			if !strings.HasSuffix(p, ".dieg.md") {
				return fmt.Errorf("file %q is not Diegesis code and cannot be changed", p)
			}
		}
	}
	return nil
}

func packMap(m map[string]string) string {
	var ps []string
	for p := range m {
		if !strings.HasSuffix(p, ".dieg.md") {
			continue // the user's other files pass through, unseen by the model
		}
		ps = append(ps, p)
	}
	sort.Strings(ps)
	var sb strings.Builder
	for _, p := range ps {
		c := m[p]
		if !strings.HasSuffix(c, "\n") {
			c += "\n"
		}
		sb.WriteString(hdrPre + p + hdrPost + "\n" + c)
	}
	return sb.String()
}

// RevisePrompt: task's intro and code rules, the change request, the whole current code, the diagnostics of the
// previous candidate (fix rounds), the output format (changed files only) and the references.
func RevisePrompt(request, bundle string, diags []Diag, r Refs, reserved []string, skeleton bool) string {
	t := task
	if skeleton {
		t = strings.Replace(t, quoteRule, skeletonRule, 1)
	}
	var b strings.Builder
	b.WriteString("# 과제: 이미 쓴 Diegesis 코드를 변경 요청에 맞게 고친다\n\n")
	b.WriteString(between(t, "Diegesis 는 소설을", "## 이야기 요구 사항"))
	fmt.Fprintf(&b, `## 변경 요청
%s

## 수정 규칙
- 요청을 이루는 데 필요한 파일만 고친다. 요청과 상관없는 인물·사실·장면은 그대로 둔다.
- 요청이 바꾸는 사실은 그 사실을 쓰는 모든 장면에서 일관되게 바꾼다(누가 무엇을 아는가, 시각, 복선과 회수 포함).
- `+"`work`"+` 의 ID 는 바꾸지 않는다. 다른 작품 ID(%s)는 쓰지 않는다. 새 장면 ID 는 기존 것과 겹치지 않게 정한다.
- 장면의 렌더 요청이 바뀌지 않은 파일은 내지 않는다. 바뀐 장면과 새 장면만 다시 렌더된다.

## 코드 작성 규칙
`, strings.TrimSpace(request), strings.Join(reserved, ", "))
	b.WriteString(between(t, "- 모든 인물·장소·물건에", "## 출력 형식"))
	b.WriteString("\n# 현재 코드 (전체)\n\n" + bundle)
	if len(diags) > 0 {
		b.WriteString("\n# 도구 진단 (dieg check, 그다음 dieg lint — 원문 그대로)\n\n")
		for _, d := range diags {
			b.WriteString(d.String() + "\n")
		}
		b.WriteString("\n위 진단을 모두 해결하도록 현재 코드를 고친다. `bundle` 진단은 출력 형식 위반이다.\n")
	}
	b.WriteString(`
## 출력 형식 (반드시 지킨다)
**바꾸거나 새로 만드는 파일만** 아래 머리줄과 함께 파일 전체 내용으로 낸다. 내지 않은 파일은 그대로 유지된다.
파일을 지우려면 내용 없이 ` + "`=== DELETE: <경로> ===`" + ` 한 줄만 쓴다. 마지막에 ` + "`" + End + "`" + ` 한 줄로 끝낸다.
다른 설명은 쓰지 않는다. 코드 펜스로 전체를 감싸지 않는다. 경로는 위 현재 코드의 경로와 같은 상대 경로이고 확장자는 .dieg.md 다.

=== FILE: ep01/s2.dieg.md ===
(내용)
=== DELETE: ep01/s3.dieg.md ===
` + End + `
`)
	b.WriteString(refsBlock(r))
	return b.String()
}

// RerenderResult: the scene ids whose render request changed / is new / is gone, and the re-render outcomes.
type RerenderResult struct {
	Changed, Added, Removed []string
	Scenes                  []SceneResult
}

// Rerender brings the prose in out/novel up to date with all (every shown scene of the story, discourse order):
// a scene is stale when its stored <scene>.<backend>.hash is missing (Added) or differs from its current RenderHash
// (Changed). Stale scenes are rendered (limit > 0 caps this run to the first limit; the rest stay stale and the next
// run picks them up), prose files of scenes no longer in all are deleted (Removed), and novel.md is rebuilt.
// A scene whose render call failed gets no hash, so it is retried; one that rendered but failed verification keeps
// its hash (its report.json says so).
func Rerender(ctx context.Context, cs render.Completers, work string, all []render.Scene, b render.Backend, out string, o Opts, limit int) (RerenderResult, error) {
	cur := make([]render.Scene, len(all))
	copy(cur, all)
	live := map[string]bool{}
	for i := range cur {
		cur[i].FreeDialogue = o.Skeleton // Novel renders (and hashes) with this
		live[fileID(cur[i].ID)] = true
	}
	after := map[string]string{} // what Novel stores
	for _, s := range cur {
		h, err := render.RenderHash(s, b)
		if err != nil {
			return RerenderResult{}, fmt.Errorf("scene %s: %w", s.ID, err)
		}
		after[s.ID] = h
	}
	dir := filepath.Join(out, "novel")
	before, beforeDig := map[string]string{}, map[string]string{}
	for _, s := range cur {
		if h, err := os.ReadFile(filepath.Join(dir, fileID(s.ID)+"."+b.ID+".hash")); err == nil {
			f := strings.Fields(string(h))
			before[s.ID] = strings.Join(f[:min(1, len(f))], "")
			if len(f) > 1 { // a legacy single-hash file means "digest unknown": never stale because of it
				beforeDig[s.ID] = f[1]
			}
		}
	}
	rr := RerenderResult{}
	rr.Changed, rr.Added, _ = ChangedScenes(before, after)
	if o.Digest != nil {
		for _, s := range cur {
			if old, ok := beforeDig[s.ID]; ok && !slices.Contains(rr.Changed, s.ID) {
				_, dh, err := sceneDigest(ctx, o, s.ID)
				if err != nil {
					return RerenderResult{}, fmt.Errorf("scene %s digest: %w", s.ID, err)
				}
				if dh != old {
					rr.Changed = append(rr.Changed, s.ID)
				}
			}
		}
		sort.Strings(rr.Changed)
	}
	if ents, err := os.ReadDir(dir); err == nil {
		for _, e := range ents {
			id, ok := strings.CutSuffix(e.Name(), "."+b.ID+".md")
			if !ok || live[id] {
				continue
			}
			rr.Removed = append(rr.Removed, id)
			for _, ext := range []string{".md", ".report.json", ".hash"} {
				if err := os.Remove(filepath.Join(dir, id+"."+b.ID+ext)); err != nil && !os.IsNotExist(err) {
					return rr, err
				}
			}
		}
	}
	ss := Only(cur, append(append([]string(nil), rr.Changed...), rr.Added...))
	if limit > 0 && limit < len(ss) {
		ss = ss[:limit]
	}
	var err error
	rr.Scenes, err = Novel(ctx, cs, work, ss, b, out, o)
	if err == nil {
		err = Concat(out, cur, b.ID)
	}
	return rr, err
}
