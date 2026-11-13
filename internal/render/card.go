package render

import (
	"context"
	"encoding/json"
	"fmt"
	"sort"
	"strings"
)

// Scores is the multi-axis scorecard: Coverage (fact present), Kind (fact type correct), Tone, Length, Score (weighted sum).
type Scores struct{ Coverage, Kind, Tone, Length, Score float64 }

// CardOpts: Tier2 "off" or "absent-only" (default, ""). TargetChars 0 drops the length axis.
type CardOpts struct {
	Tier2       string
	TargetChars int
}

type cardItem struct{ id, key, text string }

// cardItems sorts the keys Diff expects (a multiset) and numbers them E1...
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

// CardItems is the same list as Diff(s, Extracted{}).Missing.
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

// BuildCardPrompt is the stage-1 prompt. The hidden section is attached only when there are hidden facts.
// Under flesh "novel" the "other" rule asks for plot-bearing additions only, each with a kind (same schema as the re-extraction).
func BuildCardPrompt(s Scene, lang, prose string) (system, user string) {
	system = cardSys1
	if s.Flesh == "novel" {
		system = strings.Replace(system, legacyCardOther, fleshCardOther, 1)
	}
	return system, cardCatalog(s, lang) + cardChecklist(s) + "\n## Prose\n" + prose + "\n"
}

const legacyCardOther = `3. other: plot events or facts stated in the prose that are NOT covered by any checklist or withheld item: an action or revelation that changes the situation. Ignore description, gesture, setting and manner, and anything that merely restates a listed item. Each as {`

const fleshCardOther = `3. other: plot-bearing additions in the prose that are NOT covered by any checklist or withheld item: a new event; any change of possession, location (a character leaving the place), life or death, or relationship; a new named character or place; a revelation of a new fact; another character's unspoken thoughts in first-person narration; any line of speech or reported speech that is not one of the acts; anything that contradicts a listed item. Do not report sensory detail, gestures, small movements within the same place, the point-of-view character's reactions or inner thoughts that state no new fact (inner thoughts are not speech), or atmosphere, nor anything that merely restates a listed item. Each as {"kind":"<one of: event, possession, location, life, relationship, name, contradiction, fact, mind, speech>",`

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
		Actor, Verb, Action, Text, Kind string
		Targets                         []string
	} `json:"other"`
}

type itemAns struct{ present, asStated bool }

type card struct {
	items    map[string]itemAns
	revealed map[string]bool // W number
	other    []NewEvent
}

// yn accepts only "yes"/"no" (whitespace and case ignored). Any other value ("partially", "", "yes (implied)") is badExtract — so the leak check does not pass while left open.
func yn(x, field string) (bool, error) {
	switch strings.ToLower(strings.TrimSpace(x)) {
	case "yes":
		return true, nil
	case "no":
		return false, nil
	}
	return false, badExtract(fmt.Sprintf("card: %s must be yes or no, got %q", field, x))
}

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
	ok := map[string]bool{}
	for _, id := range want {
		ok[id] = true
	}
	for _, it := range list {
		if !ok[it.ID] {
			return nil, badExtract("card: unknown id " + it.ID)
		}
		if _, dup := m[it.ID]; dup {
			return nil, badExtract("card: duplicate id " + it.ID)
		}
		fp, err := yn(it.FactPresent, it.ID+" fact_present")
		if err != nil {
			return nil, err
		}
		as, err := yn(it.AsStated, it.ID+" as_stated")
		if err != nil {
			return nil, err
		}
		m[it.ID] = itemAns{fp, fp && as}
	}
	for _, id := range want {
		if _, ok := m[id]; !ok {
			return nil, badExtract("card: missing id " + id)
		}
	}
	return m, nil
}

// ParseCard reads the stage-1 answer. Broken JSON, missing items or hidden ids are badExtract.
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
		var n int
		if _, e := fmt.Sscanf(w.ID, "W%d", &n); e != nil || n < 1 || n > len(s.Withheld) || w.ID != fmt.Sprintf("W%d", n) {
			return card{}, badExtract("card: unknown id " + w.ID)
		}
		if _, dup := c.revealed[w.ID]; dup {
			return card{}, badExtract("card: duplicate id " + w.ID)
		}
		rv, err := yn(w.Revealed, w.ID+" revealed")
		if err != nil {
			return card{}, err
		}
		c.revealed[w.ID] = rv
	}
	for i := range s.Withheld {
		if _, ok := c.revealed[fmt.Sprintf("W%d", i+1)]; !ok {
			return card{}, badExtract(fmt.Sprintf("card: missing id W%d", i+1))
		}
	}
	for _, x := range o.Other {
		c.other = append(c.other, NewEvent{Actor: x.Actor, Verb: x.Verb, Targets: x.Targets, Action: x.Action, Text: x.Text, Kind: x.Kind})
	}
	return c, nil
}

// ask calls once, and if the answer is unusable calls again just once.
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

// ScoreCard scores one render against the checklist (stage 1 + stage 2 once if needed). An unusable answer is left in Report.ExtractError and the error is nil.
// G2 candidates (other) are not re-asked. Legacy (ADR-028): left only as Warnings, Hallucinated is always empty.
// Flesh "novel" (T3-R2): the prompt asks for plot-bearing additions only, so every non-texture one goes to Hallucinated
// and fails the render; a speech addition is covered by a fact-less speech act only in FreeDialogue mode.
func ScoreCard(ctx context.Context, c Completer, s Scene, b Backend, prose string, o CardOpts) (Report, error) {
	s.Flesh = b.Sample.Flesh
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
			if err != nil { // if stage 2 breaks, use the stage-1 answer as is
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
		novel := s.Flesh == "novel"
		covered := coveredByIR(s, ev, false)
		if novel { // I1: plot kinds are never covered; an event needs the act's action word, speech needs FreeDialogue
			covered = ev.Kind == "event" && coveredByIR(s, ev, true) || ev.Kind == "speech" && s.FreeDialogue && covered
		}
		switch {
		case s.authored(ev) || covered || s.texture(ev):
		case novel:
			rep.Hallucinated = append(rep.Hallucinated, ev.Label())
		default:
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

// coveredByIR: an event that re-tells an author act with no fact (orders, asks, whispers, sees, etc.) is not invented.
// authored() only looks at does and presence, so match the other verb classes by the same subject, same verb class and overlapping objects.
// With action set (flesh "novel"), the act's action word must also match, as authored() requires.
func coveredByIR(s Scene, ev NewEvent, action bool) bool {
	actor := resolveCastID(s, ev.Actor)
	if actor == "" {
		return false
	}
	for _, bt := range s.Beats {
		for _, a := range bt.Acts {
			if a.Fact != "" || a.Actor != actor || VerbClass(a.Verb) != VerbClass(ev.Verb) || action && !strings.EqualFold(a.Action, ev.Action) {
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
