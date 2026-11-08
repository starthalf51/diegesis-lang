package render

import (
	"context"
	"encoding/json"
	"fmt"
	"sort"
	"strings"
)

// 문장 단위 최소 수정(dieg revise): 옛 산문 + 새 장면 + 사실 차이를 주고 "옛 문장 → 새 문장" 목록(JSON)만 받아
// 결정적으로 적용한다. 적용이 안 되거나 카드 검수를 못 넘으면 한 번 더, 그래도 안 되면 전체 렌더로 대체한다.
// 프롬프트는 tools/serial/minedit.go(examples/value/minedit 에서 측정)를 JSON 편집 목록 출력으로 바꾼 것이다.

// Edit 는 옛 산문의 한 구간(Old, 정확히 한 번 나오는 그대로)을 New 로 바꾼다.
type Edit struct{ Old, New string }

// ParsePatch 는 {"edits":[{"old":"…","new":"…"}]} 를 읽는다(```json 펜스 허용). 깨졌거나 edits 키가 없으면 badExtract.
func ParsePatch(text string) ([]Edit, error) {
	body := text
	if i := strings.Index(text, "```json"); i >= 0 {
		rest := text[i+7:]
		if j := strings.Index(rest, "```"); j >= 0 {
			body = rest[:j]
		}
	}
	var o struct {
		Edits *[]struct {
			Old *string `json:"old"`
			New *string `json:"new"`
		} `json:"edits"`
	}
	if err := json.Unmarshal([]byte(strings.TrimSpace(body)), &o); err != nil {
		return nil, badExtract("patch: " + err.Error())
	}
	if o.Edits == nil {
		return nil, badExtract(`patch: no "edits" list`)
	}
	es := make([]Edit, 0, len(*o.Edits))
	for i, e := range *o.Edits {
		if e.Old == nil || e.New == nil {
			return nil, badExtract(fmt.Sprintf("patch: edit %d needs old and new", i+1))
		}
		es = append(es, Edit{*e.Old, *e.New})
	}
	return es, nil
}

// ApplyPatch 는 각 Old 가 prose 에 정확히 한 번 나오고 구간끼리 겹치지 않을 때만 적용한다. 아니면 badExtract.
func ApplyPatch(prose string, es []Edit) (string, error) {
	type span struct {
		at, end int
		nw      string
	}
	var sp []span
	for i, e := range es {
		if e.Old == "" {
			return "", badExtract(fmt.Sprintf("patch: edit %d has an empty old", i+1))
		}
		n := 0
		for j := strings.Index(prose, e.Old); j >= 0; { // overlapping repeats ("aa" in "aaa") are ambiguous too
			n++
			k := strings.Index(prose[j+1:], e.Old)
			if k < 0 {
				break
			}
			j += 1 + k
		}
		switch {
		case n == 0:
			return "", badExtract(fmt.Sprintf("patch: edit %d old not found in the prose: %q", i+1, e.Old))
		case n > 1:
			return "", badExtract(fmt.Sprintf("patch: edit %d old appears %d times (make it longer so it is unique): %q", i+1, n, e.Old))
		}
		at := strings.Index(prose, e.Old)
		sp = append(sp, span{at, at + len(e.Old), e.New})
	}
	sort.Slice(sp, func(i, j int) bool { return sp[i].at < sp[j].at })
	var b strings.Builder
	pos := 0
	for _, s := range sp {
		if s.at < pos {
			return "", badExtract("patch: edits overlap")
		}
		b.WriteString(prose[pos:s.at] + s.nw)
		pos = s.end
	}
	b.WriteString(prose[pos:])
	return b.String(), nil
}

// sceneLines flattens a scene request into "section: text" lines (what the renderer is told).
func sceneLines(s Scene) []string {
	name := func(id string) string {
		if e, ok := s.Entity(id); ok && len(e.Names["ko"]) > 0 {
			return e.Names["ko"][0]
		}
		return id
	}
	fact := func(id string) string { f, _ := s.FactByID(id); return f.Text }
	var l []string
	add := func(sec, t string) {
		if strings.TrimSpace(t) != "" {
			l = append(l, sec+": "+t)
		}
	}
	add("제목", s.Title)
	add("장소", s.Place)
	add("이전 줄거리", s.Previously)
	for _, e := range s.Cast {
		add("등장인물", name(e.ID))
	}
	for _, b := range s.Beats {
		for _, a := range b.Acts {
			p := []string{}
			if a.Actor != "" {
				p = append(p, name(a.Actor))
			}
			p = append(p, a.Verb)
			if a.Action != "" {
				p = append(p, "("+a.Action+")")
			}
			for _, t := range a.Targets {
				p = append(p, name(t))
			}
			t := strings.Join(p, " ")
			if a.Fact != "" {
				f, _ := s.FactByID(a.Fact)
				t += ": " + f.Text
				if !f.True {
					t += " (거짓 — 말하는 이가 믿거나 주장할 뿐, 서술자가 단정하면 안 된다)"
				}
			}
			if a.Quote != "" {
				t += fmt.Sprintf(" 대사: %q", a.Quote)
			}
			add("행위", t)
		}
	}
	add("시점 인물", name(s.Directives.POV))
	for _, x := range []struct {
		sec string
		ids []string
	}{{"시점 인물이 믿는 것", s.POVBeliefs}, {"독자가 이미 아는 것", s.ReaderKnows}, {"말하면 안 되는 것", s.Withheld}} {
		for _, id := range x.ids {
			add(x.sec, fact(id))
		}
	}
	add("지시어", fmt.Sprintf("pace=%s tension=%g focus=%s omit=%s", s.Directives.Pace, s.Directives.Tension,
		strings.Join(s.Directives.Focus, ";"), strings.Join(s.Directives.Omit, ";")))
	return l
}

// sceneDiff is the multiset difference of two scene requests: lines only in old (removed), only in new (added).
func SceneDiff(old, nw Scene) (removed, added []string) {
	cnt := map[string]int{}
	for _, x := range sceneLines(old) {
		cnt[x]++
	}
	for _, x := range sceneLines(nw) {
		if cnt[x] > 0 {
			cnt[x]--
		} else {
			added = append(added, x)
		}
	}
	nc := map[string]int{}
	for _, x := range sceneLines(nw) {
		nc[x]++
	}
	for _, x := range sceneLines(old) {
		if nc[x] > 0 {
			nc[x]--
		} else {
			removed = append(removed, x)
		}
	}
	return
}

func diffText(rm, ad []string) string {
	var b strings.Builder
	b.WriteString("### 삭제되거나 바뀌기 전의 항목 (옛 명세에만 있음)\n")
	for _, x := range rm {
		b.WriteString("- " + x + "\n")
	}
	if len(rm) == 0 {
		b.WriteString("(없음)\n")
	}
	b.WriteString("\n### 추가되거나 바뀐 뒤의 항목 (새 명세에만 있음)\n")
	for _, x := range ad {
		b.WriteString("- " + x + "\n")
	}
	if len(ad) == 0 {
		b.WriteString("(없음)\n")
	}
	return b.String()
}

// ---- the minimal-edit prompt (spike only; production prompts are untouched) ----

func patchSystem(b Backend, s Scene) string {
	sy, _, _ := b.Prompt(s)
	_, tone, _ := strings.Cut(sy, "\n목표 어투(반드시 지킨다): ")
	return `너는 한국어 소설 편집자다. 장면 하나의 옛 산문을 새 장면 명세에 맞게 '최소한으로' 고친다. 고친 산문 전체가 아니라, 바꿀 문장만 "옛 문장 → 새 문장" 목록으로 답한다.

규칙:
- 아래 '변경 내역'에 나온 사실·행위와 직접 닿는 문장만 고친다. 닿지 않는 문장은 목록에 넣지 않는다. 문장을 다듬거나 순서를 바꾸지 않는다.
- '옛 명세에만 있음'에 있는 내용은 산문에서 말하거나 암시하지 않는다(그 내용을 전하던 문장은 새 명세에 맞게 고치거나, 자리가 없으면 new 를 빈 문자열로 해 그 문장만 지운다).
- '새 명세에만 있음'에 있는 내용은 그것을 전하는 인물·동사(누가 말하는가, 보는가)대로 해당 문장에 반영한다. 자리가 없으면 가까운 문장에 이어 붙이는 식으로 old 를 그 문장으로 하고 new 에 덧붙인다.
- 새 명세(Acts, Must not state or imply 등)에 없는 사건·사실·대사를 새로 지어내지 않는다. 말하면 안 되는 사실은 암시도 하지 않는다. 거짓으로 표시된 사실은 서술자가 사실로 단정하지 않는다. 시점은 그대로 유지한다.
- 고친 문장도 목표 어투를 지킨다.
- old 는 옛 산문에 나온 그대로(띄어쓰기·문장부호 포함) 한 문장 또는 짧은 구간이고, 옛 산문에서 정확히 한 번만 나와야 한다. 겹치는 구간을 고르지 않는다.

목표 어투(반드시 지킨다): ` + strings.TrimSpace(tone) + `

출력: 아래 JSON 하나만 쓴다. 다른 설명은 쓰지 않는다.
{"edits":[{"old":"옛 산문의 문장","new":"고친 문장"}]}
고칠 곳이 없으면 {"edits":[]}`
}

func patchUser(oldProse string, nw Scene, b Backend, digest, diff, feedback string) string {
	_, user, _ := b.Prompt(nw)
	u := "## 옛 산문\n\n" + oldProse + "\n\n## 새 장면 명세\n\n" + user + digest + "\n\n## 변경 내역 (옛 명세 → 새 명세)\n\n" + diff
	if feedback != "" {
		u += "\n" + feedback
	}
	return u
}

// patchFeedback 은 직전 수정본의 카드 결과를 한국어로 알린다. 다시 옛 산문에서 고친다.
func patchFeedback(s Scene, lang string, rep Report) string {
	var b strings.Builder
	b.WriteString("## 직전 수정본의 검수 결과\n직전 수정본은 아래 문제가 있었다. 같은 최소 수정 규칙으로 옛 산문에서 다시 고친다.\n")
	for _, m := range rep.Missing {
		b.WriteString("- 빠졌거나 전하는 이가 다르다. 반영할 것: " + describe(s, lang, m) + "\n")
	}
	for _, l := range rep.Leaked {
		b.WriteString("- 드러나면 안 되는 내용이 드러났다. 지울 것: " + describe(s, lang, l) + "\n")
	}
	if rep.ExtractError != "" {
		b.WriteString("- 검수 오류: " + rep.ExtractError + "\n")
	}
	return b.String()
}

// PatchVerified 는 oldProse 를 새 장면 nw 에 맞게 문장 단위로 고친다: 패치 1회 → 카드. 적용이 안 되거나 카드가 실패면
// 그 피드백으로 (옛 산문에서) 패치 1회 더. 그래도 안 되면 RenderVerified2 로 전체 렌더한다. digest 는 빈 줄로 시작하는
// 요약 절(없으면 "")로 패치 프롬프트와 대체 렌더 모두에 붙는다. 검수는 카드 고정이다. 돌려주는 int 는 모델 렌더 시도 수(패치 + 대체 렌더).
func PatchVerified(ctx context.Context, cs Completers, work string, old, nw Scene, b Backend, oldProse, digest string, o VerifyOpts) (Render, int, error) {
	rm, ad := SceneDiff(old, nw)
	diff, sys := diffText(rm, ad), patchSystem(b, nw)
	r := Render{Work: work, Scene: nw.ID, Backend: b.ID, Lang: b.Lang, Tone: b.Tone}
	ps, pu, err := b.Prompt(nw)
	if err != nil {
		return r, 0, err
	}
	r.RenderHash = sum(ps, pu)
	fb := ""
	for n := 1; n <= 2; n++ {
		res, err := cs.Render.Complete(ctx, sys, patchUser(oldProse, nw, b, digest, diff, fb))
		if err != nil {
			return r, n, fmt.Errorf("%s/%s patch: %w", nw.ID, b.ID, err)
		}
		var prose string
		es, perr := []Edit(nil), error(nil)
		if res.StopReason != "end_turn" {
			perr = badExtract(stopped("patch", res.StopReason))
		} else if es, perr = ParsePatch(res.Text); perr == nil {
			prose, perr = ApplyPatch(oldProse, es)
		}
		if perr != nil {
			fb = "## 직전 답의 문제\n직전 답은 쓸 수 없었다: " + perr.Error() + "\nold 는 옛 산문에 있는 그대로, 정확히 한 번 나오는 구간이어야 하고 구간끼리 겹치면 안 된다. JSON 하나만 다시 쓴다.\n"
			continue
		}
		r.Prose, r.StopReason = prose, res.StopReason
		if b.Lang == "ko" {
			r.Tones = ClassifyTone(prose)
		}
		if r.Report, err = ScoreCard(ctx, cs.Extract, nw, b, prose, o.Card); err != nil {
			return r, n, fmt.Errorf("%s/%s card: %w", nw.ID, b.ID, err)
		}
		r.Report.Patched = true
		if Passed(r) {
			return r, n, nil
		}
		fb = patchFeedback(nw, b.Lang, r.Report)
	}
	o.UserSuffix = digest
	full, m, err := RenderVerified2(ctx, cs, work, nw, b, o)
	return full, 2 + m, err
}
