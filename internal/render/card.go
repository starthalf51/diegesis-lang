package render

import (
	"context"
	"encoding/json"
	"fmt"
	"sort"
	"strings"
)

// Scores 는 다축 채점표: Coverage(사실 있음), Kind(사실형 맞음), Tone(어투), Length(분량), Score(가중합).
type Scores struct{ Coverage, Kind, Tone, Length, Score float64 }

// CardOpts: Tier2 "off" 또는 "absent-only"(기본, ""). TargetChars 0 이면 분량 축을 뺀다.
type CardOpts struct {
	Tier2       string
	TargetChars int
}

type cardItem struct{ id, key, text string }

// cardItems 는 Diff 가 기대하는 키(멀티셋)를 정렬해 E1.. 번호를 붙인다.
func cardItems(s Scene) []cardItem {
	text := map[string]string{}
	var keys []string
	for _, b := range s.Beats {
		for _, a := range b.Acts {
			if a.Fact == "" || VerbClass(a.Verb) == "presence" {
				continue
			}
			k := actKey(a.Actor, a.Verb, s.Canon(a.Fact))
			if _, ok := text[k]; !ok {
				f, _ := s.FactByID(a.Fact)
				text[k] = fmt.Sprintf("%s %s %s: %s", a.Actor, a.Verb, strings.Join(a.Targets, ", "), f.Text)
			}
			keys = append(keys, k)
		}
	}
	sort.Strings(keys)
	out := make([]cardItem, len(keys))
	for i, k := range keys {
		out[i] = cardItem{fmt.Sprintf("E%d", i+1), k, text[k]}
	}
	return out
}

// CardItems 는 Diff(s, Extracted{}).Missing 과 같은 목록이다.
func CardItems(s Scene) []string {
	its := cardItems(s)
	out := make([]string, len(its))
	for i, it := range its {
		out[i] = it.key
	}
	return out
}

const cardVerbs = "enter, exit, tells, says, hails, lies, whispers, sees, reads, does, aside, asks, orders, shows, decrees, sends, writes, thinks, enacts"

func cardCatalog(s Scene, lang string) string {
	var b strings.Builder
	b.WriteString("Prose language: " + lang + "\n\n## Entities (ID: names)\n")
	for _, e := range s.Cast {
		var ps []string
		for _, ns := range e.Names {
			ps = append(ps, strings.Join(ns, ", "))
		}
		sort.Strings(ps)
		fmt.Fprintf(&b, "- %s: %s\n", e.ID, strings.Join(ps, " / "))
	}
	var av strings.Builder
	for _, bt := range s.Beats {
		for _, a := range bt.Acts {
			if a.Verb == "does" && a.Action != "" {
				fmt.Fprintf(&av, "- %s: %s -> %s\n", a.Action, a.Actor, strings.Join(a.Targets, ", "))
			}
		}
	}
	if av.Len() > 0 {
		b.WriteString("\n## Authored actions (action word: actor -> targets)\n" + av.String())
	}
	return b.String()
}

func cardChecklist(s Scene) string {
	var b strings.Builder
	b.WriteString("\n## Checklist (expected facts)\n")
	for _, i := range cardItems(s) {
		b.WriteString(i.id + ". " + i.text + "\n")
	}
	if len(s.Withheld) > 0 {
		b.WriteString("\n## Withheld facts (must NOT be revealed)\n")
		for i, id := range s.Withheld {
			f, _ := s.FactByID(id)
			fmt.Fprintf(&b, "W%d. %s\n", i+1, f.Text)
		}
	}
	return b.String()
}

const cardSys1 = `You audit a piece of prose against a checklist. Judge only from what the prose says or implies to a reader who does not know the source story; ignore your knowledge of any original work.
1. items: for EVERY checklist id give "fact_present" ("yes" if the prose states or clearly implies the fact's content, by anyone and in any way, including through how a character speaks, addresses someone, admits something, or thinks; else "no"), and "as_stated" ("yes" only if fact_present is yes AND the prose has that very actor conveying it with that kind of verb: says/conveys/thinks vs sees/hears/notices vs reads/writes; if another character is the one who says, hears or does it, answer "no"). Add the shortest supporting "quote" copied from the prose (empty if fact_present is no).
2. withheld: for every withheld id, "revealed" "yes" if the prose states or clearly implies it, else "no", with a "quote" if yes. Use [] if there are none.
3. other: plot events or facts stated in the prose that are NOT covered by any checklist or withheld item: an action or revelation that changes the situation. Ignore description, gesture, setting and manner, and anything that merely restates a listed item. Each as {"actor":"<entity ID or name>","verb":"<one of: ` + cardVerbs + `>","targets":["<entity IDs or names>"],"action":"<the thing done for does, else empty>","text":"<one short sentence>","quote":"<verbatim from the prose>"}. Use [] if none. When an event re-tells one of the Authored actions in the catalog, even in other words, report it with that exact action word as "action" and that actor and those targets.
Reply with a single ` + "```json" + ` fenced block and nothing else, shaped as {"items":[{"id":"E1","fact_present":"yes","as_stated":"yes","quote":""}],"withheld":[{"id":"W1","revealed":"no","quote":""}],"other":[]}.`

const cardSys2 = `You re-check a few disputed points of a prose against its checklist. Judge only from what the prose says or implies to a reader who does not know the source story. Read the whole prose carefully before answering.
- recheck: for each id answer again "fact_present" ("yes" if the prose states or clearly implies the content, by anyone; else "no") and "as_stated" ("yes" only if that very actor conveys it with that kind of verb), with the shortest verbatim "quote".
Reply with a single ` + "```json" + ` fenced block and nothing else, shaped as {"recheck":[{"id":"E1","fact_present":"yes","as_stated":"yes","quote":""}]}.`

// BuildCardPrompt 는 1단 프롬프트다. 숨김 절은 숨긴 사실이 있을 때만 붙는다.
func BuildCardPrompt(s Scene, lang, prose string) (system, user string) {
	return cardSys1, cardCatalog(s, lang) + cardChecklist(s) + "\n## Prose\n" + prose + "\n"
}

type rawItem struct {
	ID          string `json:"id"`
	FactPresent string `json:"fact_present"`
	AsStated    string `json:"as_stated"`
}

type rawCard struct {
	Items    []rawItem `json:"items"`
	Recheck  []rawItem `json:"recheck"`
	Withheld []struct {
		ID       string `json:"id"`
		Revealed string `json:"revealed"`
	} `json:"withheld"`
	Other []struct {
		Actor, Verb, Action, Text string
		Targets                   []string
	} `json:"other"`
}

type itemAns struct{ present, asStated bool }

type card struct {
	items    map[string]itemAns
	revealed map[string]bool // W번호
	other    []NewEvent
}

func yes(x string) bool { return strings.EqualFold(strings.TrimSpace(x), "yes") }

func cardJSON(text string) (rawCard, error) {
	body := text
	if i := strings.Index(text, "```json"); i >= 0 {
		rest := text[i+7:]
		if j := strings.Index(rest, "```"); j >= 0 {
			body = rest[:j]
		}
	}
	var o rawCard
	err := json.Unmarshal([]byte(strings.TrimSpace(body)), &o)
	return o, err
}

func ansOf(list []rawItem, want []string) (map[string]itemAns, error) {
	m := map[string]itemAns{}
	for _, it := range list {
		fp := yes(it.FactPresent)
		m[it.ID] = itemAns{fp, fp && yes(it.AsStated)}
	}
	for _, id := range want {
		if _, ok := m[id]; !ok {
			return nil, badExtract("card: missing id " + id)
		}
	}
	return m, nil
}

// ParseCard 는 1단 답을 읽는다. 깨진 JSON, 빠진 항목·숨김 id 는 badExtract.
func ParseCard(s Scene, text string) (card, error) {
	o, err := cardJSON(text)
	if err != nil {
		return card{}, badExtract("card: " + err.Error())
	}
	var want []string
	for _, it := range cardItems(s) {
		want = append(want, it.id)
	}
	items, err := ansOf(o.Items, want)
	if err != nil {
		return card{}, err
	}
	c := card{items: items, revealed: map[string]bool{}}
	for _, w := range o.Withheld {
		c.revealed[w.ID] = yes(w.Revealed)
	}
	for i := range s.Withheld {
		if _, ok := c.revealed[fmt.Sprintf("W%d", i+1)]; !ok {
			return card{}, badExtract(fmt.Sprintf("card: missing id W%d", i+1))
		}
	}
	for _, x := range o.Other {
		c.other = append(c.other, NewEvent{Actor: x.Actor, Verb: x.Verb, Targets: x.Targets, Action: x.Action, Text: x.Text})
	}
	return c, nil
}

// ask 는 한 번 부르고, 쓸 수 없는 답이면 한 번만 다시 부른다.
func ask(ctx context.Context, c Completer, sys, user string, parse func(string) error) error {
	var last error
	for try := 0; try < 2; try++ {
		u := user
		if try == 1 {
			u += "\nYour previous reply was not valid or was incomplete JSON. Reply again with the single ```json block only, every id included, quotes escaped.\n"
		}
		res, err := c.Complete(ctx, sys, u)
		if err != nil {
			return err
		}
		if res.StopReason != "end_turn" {
			last = badExtract(stopped("card", res.StopReason))
			continue
		}
		if last = parse(res.Text); last == nil {
			return nil
		}
	}
	return last
}

// ScoreCard 는 렌더 하나를 체크리스트로 채점한다(1단 + 필요하면 2단 1회). 쓸 수 없는 답은 Report.ExtractError 에 남고 오류는 nil 이다.
// G2 후보(other)는 되묻지 않고 Warnings 로만 남는다. Hallucinated 는 늘 비어 있다.
func ScoreCard(ctx context.Context, c Completer, s Scene, b Backend, prose string, o CardOpts) (Report, error) {
	rep := Report{Missing: []string{}, Hallucinated: []string{}, Leaked: []string{}}
	sys, user := BuildCardPrompt(s, b.Lang, prose)
	var cd card
	err := ask(ctx, c, sys, user, func(t string) (e error) { cd, e = ParseCard(s, t); return })
	if err != nil {
		if IsBadExtract(err) {
			rep.ExtractError = err.Error()
			return rep, nil
		}
		return rep, err
	}
	items := cardItems(s)
	if o.Tier2 != "off" {
		var re []string
		var u strings.Builder
		for _, it := range items {
			if x := cd.items[it.id]; !x.present || !x.asStated {
				re = append(re, it.id)
				u.WriteString(it.id + ". " + it.text + "\n")
			}
		}
		if len(re) > 0 {
			var m map[string]itemAns
			user2 := cardCatalog(s, b.Lang) + cardChecklist(s) + "\n## Prose\n" + prose + "\n\n## Recheck these items\n" + u.String()
			err := ask(ctx, c, cardSys2, user2, func(t string) error {
				r, e := cardJSON(t)
				if e != nil {
					return badExtract("card: " + e.Error())
				}
				m, e = ansOf(r.Recheck, re)
				return e
			})
			if err != nil && !IsBadExtract(err) {
				return rep, err
			}
			if err != nil { // 2단이 깨지면 1단 답을 그대로 쓴다
				rep.Warnings = append(rep.Warnings, "tier2 failed: "+err.Error())
			}
			for _, id := range re {
				if x, ok := m[id]; ok {
					cd.items[id] = x
				}
			}
		}
	}
	var p, a float64
	for _, it := range items {
		x := cd.items[it.id]
		if x.present {
			p++
		}
		if x.asStated {
			a++
		}
		if !x.present || !x.asStated {
			rep.Missing = append(rep.Missing, it.key)
		}
	}
	sort.Strings(rep.Missing)
	for i, id := range s.Withheld {
		if cd.revealed[fmt.Sprintf("W%d", i+1)] {
			rep.Leaked = append(rep.Leaked, id)
		}
	}
	for _, ev := range cd.other {
		if !s.authored(ev) && !coveredByIR(s, ev) {
			rep.Warnings = append(rep.Warnings, ev.Label())
		}
	}
	sc := Scores{Coverage: 1, Kind: 1, Tone: 1}
	if n := float64(len(items)); n > 0 {
		sc.Coverage, sc.Kind = p/n, a/n
	}
	if b.Lang == "ko" && b.Tone != "" {
		sc.Tone = ClassifyTone(prose)[b.Tone]
	}
	if t := float64(o.TargetChars); t > 0 {
		n := float64(len([]rune(strings.Join(strings.Fields(prose), ""))))
		sc.Length = max(0, 1-abs(n-t)/t)
		sc.Score = 0.6*sc.Coverage + 0.2*sc.Kind + 0.1*sc.Tone + 0.1*sc.Length
	} else {
		sc.Score = (0.6*sc.Coverage + 0.2*sc.Kind + 0.1*sc.Tone) / 0.9
	}
	rep.Scores = &sc
	return rep, nil
}

func abs(x float64) float64 {
	if x < 0 {
		return -x
	}
	return x
}

func resolveCastID(s Scene, x string) string {
	for _, e := range s.Cast {
		if strings.EqualFold(e.ID, x) {
			return e.ID
		}
		for _, ns := range e.Names {
			for _, n := range ns {
				if strings.EqualFold(n, x) {
					return e.ID
				}
			}
		}
	}
	return ""
}

// coveredByIR: 사실 없는 지은이 행위(orders·asks·whispers·sees 등)를 다시 말한 사건은 지어낸 것이 아니다.
// authored() 는 does·presence 만 봐 주므로 그 밖의 동사류를 같은 주체·같은 동사류·겹치는 대상으로 맞댄다.
func coveredByIR(s Scene, ev NewEvent) bool {
	actor := resolveCastID(s, ev.Actor)
	if actor == "" {
		return false
	}
	for _, bt := range s.Beats {
		for _, a := range bt.Acts {
			if a.Fact != "" || a.Actor != actor || VerbClass(a.Verb) != VerbClass(ev.Verb) {
				continue
			}
			if len(ev.Targets) == 0 || len(a.Targets) == 0 {
				return true
			}
			for _, t := range ev.Targets {
				for _, u := range a.Targets {
					if resolveCastID(s, t) == u {
						return true
					}
				}
			}
		}
	}
	return false
}
