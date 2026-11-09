package main

// One-off spike: verifier for the multi-axis scorecard (design note rendering.md "multi-axis scorecard"). Does not touch production prompts.
//   -spike sc-gen|sc-run|sc-old|sc-report  (-out examples/corpus-ab/scorecard, -budget N, -pairs variant-name,...)

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/starthalf51/diegesis-lang/internal/render"
)

type itemAns struct {
	Present, AsStated bool
	Quote             string
	T2                bool `json:",omitempty"`
}

type cand struct {
	Ev        render.NewEvent
	Quote     string
	Confirmed bool // stage 2 confirmed it as "extra" (or stage 2 broke so unconfirmed → conservatively true)
	Verdict   string
	Covered   bool `json:",omitempty"` // the result of filtering again with coveredByIR at report time
}

type scRun struct {
	Variant               string
	Run                   int
	Items                 map[string]itemAns
	Leak                  map[string]bool
	LeakQuote             map[string]string
	Cands                 []cand
	G1Fail, G2Fail        bool
	S1, S2, S3, S4, Score float64
	Calls                 int
	Tier2, Retries        int
	In, Out, CR, CW       int64
	Weighted, Eff         float64
	Err                   string `json:",omitempty"`
}

var scVerbs = "enter, exit, tells, says, hails, lies, whispers, sees, reads, does, aside, asks, orders, shows, decrees, sends, writes, thinks, enacts"

func weighted(u render.Usage) float64 {
	return 3*float64(u.Input+u.CacheRead+u.CacheWrite) + 15*float64(u.Output)
}

func catalog(s render.Scene, lang string) string {
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

func checklist(v vSet) string {
	var b strings.Builder
	b.WriteString("\n## Checklist (expected facts)\n")
	for _, i := range v.Expect {
		b.WriteString(i.ID + ". " + i.Text + "\n")
	}
	if len(v.Withheld) > 0 {
		b.WriteString("\n## Withheld facts (must NOT be revealed)\n")
		for _, i := range v.Withheld {
			b.WriteString(i.ID + ". " + i.Text + "\n")
		}
	}
	return b.String()
}

func scSys1() string {
	return `You audit a piece of prose against a checklist. Judge only from what the prose says or implies to a reader who does not know the source story; ignore your knowledge of any original work.
1. items: for EVERY checklist id give "fact_present" ("yes" if the prose states or clearly implies the fact's content, by anyone and in any way, including through how a character speaks, addresses someone, admits something, or thinks; else "no"), and "as_stated" ("yes" only if fact_present is yes AND the prose has that very actor conveying it with that kind of verb: says/conveys/thinks vs sees/hears/notices vs reads/writes; if another character is the one who says, hears or does it, answer "no"). Add the shortest supporting "quote" copied from the prose (empty if fact_present is no).
2. withheld: for every withheld id, "revealed" "yes" if the prose states or clearly implies it, else "no", with a "quote" if yes. Use [] if there are none.
3. other: plot events or facts stated in the prose that are NOT covered by any checklist or withheld item: an action or revelation that changes the situation. Ignore description, gesture, setting and manner, and anything that merely restates a listed item. Each as {"actor":"<entity ID or name>","verb":"<one of: ` + scVerbs + `>","targets":["<entity IDs or names>"],"action":"<the thing done for does, else empty>","text":"<one short sentence>","quote":"<verbatim from the prose>"}. Use [] if none. When an event re-tells one of the Authored actions in the catalog, even in other words, report it with that exact action word as "action" and that actor and those targets.
Reply with a single ` + "```json" + ` fenced block and nothing else, shaped as {"items":[{"id":"E1","fact_present":"yes","as_stated":"yes","quote":""}],"withheld":[{"id":"W1","revealed":"no","quote":""}],"other":[]}.`
}

func scPrompt1(s render.Scene, lang, prose string, v vSet) (string, string) {
	return scSys1(), catalog(s, lang) + checklist(v) + "\n## Prose\n" + prose + "\n"
}

func scSys2() string {
	return `You re-check a few disputed points of a prose against its checklist. Judge only from what the prose says or implies to a reader who does not know the source story. Read the whole prose carefully before answering.
- recheck: for each id answer again "fact_present" ("yes" if the prose states or clearly implies the content, by anyone; else "no") and "as_stated" ("yes" only if that very actor conveys it with that kind of verb), with the shortest verbatim "quote".
- candidates: each is a possible extra event or fact in the prose. verdict "extra" if it is a real plot event or fact that no checklist item covers; "covered" if it only restates a checklist item (say which id in "item"); "trivial" if it is description, gesture, setting or manner.
Reply with a single ` + "```json" + ` fenced block and nothing else, shaped as {"recheck":[{"id":"E1","fact_present":"yes","as_stated":"yes","quote":""}],"candidates":[{"id":"C1","verdict":"extra","item":"","quote":""}]}.`
}

type rawItem struct {
	ID          string `json:"id"`
	FactPresent string `json:"fact_present"`
	AsStated    string `json:"as_stated"`
	Quote       string `json:"quote"`
}

type rawOut struct {
	Items    []rawItem `json:"items"`
	Recheck  []rawItem `json:"recheck"`
	Withheld []struct {
		ID       string `json:"id"`
		Revealed string `json:"revealed"`
		Quote    string `json:"quote"`
	} `json:"withheld"`
	Other []struct {
		Actor, Verb, Action, Text, Quote string
		Targets                          []string
	} `json:"other"`
	Candidates []struct {
		ID      string `json:"id"`
		Verdict string `json:"verdict"`
		Item    string `json:"item"`
		Quote   string `json:"quote"`
	} `json:"candidates"`
}

func fence(text string) string {
	body := text
	if i := strings.Index(text, "```json"); i >= 0 {
		rest := text[i+7:]
		if j := strings.Index(rest, "```"); j >= 0 {
			body = rest[:j]
		}
	}
	return strings.TrimSpace(body)
}

// parseRaw: non-strict JSON, but every requested id must be present.
func parseRaw(text string, wantItems []string, tier2 bool, wantCands int) (rawOut, error) {
	var o rawOut
	if err := json.Unmarshal([]byte(fence(text)), &o); err != nil {
		return o, err
	}
	list := o.Items
	if tier2 {
		list = o.Recheck
	}
	have := map[string]bool{}
	for _, it := range list {
		have[it.ID] = true
	}
	for _, id := range wantItems {
		if !have[id] {
			return o, fmt.Errorf("missing id %s", id)
		}
	}
	if tier2 && len(o.Candidates) < wantCands {
		return o, errors.New("missing candidates")
	}
	return o, nil
}

func yes(s string) bool { return strings.EqualFold(strings.TrimSpace(s), "yes") }

type scEnv struct {
	ctx    context.Context
	c      render.Completer // completer wrapped with counter+meter
	m      *render.Meter
	out    string // directory where per-call JSON is saved
	budget int
}

// ask calls once and parses; if broken it calls again just once. Each call is saved as a file.
func (e *scEnv) ask(tag, sys, user string, wantItems []string, tier2 bool, wantCands int, r *scRun) (rawOut, error) {
	var last error
	for try := 0; try < 2; try++ {
		u := user
		if try == 1 {
			u += "\nYour previous reply was not valid or was incomplete JSON. Reply again with the single ```json block only, every id included, quotes escaped.\n"
			r.Retries++
		}
		res, err := e.c.Complete(e.ctx, sys, u)
		r.Calls++
		name := fmt.Sprintf("%s_%s.json", tag, map[bool]string{false: "a", true: "b"}[try == 1])
		rec := map[string]any{"tag": tag, "try": try, "raw": res.Text, "stop": res.StopReason}
		if err != nil {
			rec["err"] = err.Error()
		}
		j, _ := json.MarshalIndent(rec, "", "  ")
		_ = os.WriteFile(filepath.Join(e.out, name), j, 0o644)
		if err != nil {
			return rawOut{}, err
		}
		o, perr := parseRaw(res.Text, wantItems, tier2, wantCands)
		if perr == nil {
			return o, nil
		}
		last = perr
	}
	return rawOut{}, fmt.Errorf("malformed after retry: %w", last)
}

func ids(items []vItem) []string {
	var o []string
	for _, i := range items {
		o = append(o, i.ID)
	}
	return o
}

// scoreRun is the deterministic axes and the total. Gate failure is separate from the score.
func scoreRun(r *scRun, v vSet, b render.Backend, prose string, target int) {
	n, p, a := float64(len(v.Expect)), 0.0, 0.0
	for _, it := range v.Expect {
		x := r.Items[it.ID]
		if x.Present {
			p++
			if x.AsStated {
				a++
			}
		}
	}
	if n > 0 {
		r.S1, r.S2 = p/n, a/n
	}
	r.S3 = 1
	if b.Tone != "" {
		r.S3 = render.ClassifyTone(prose)[b.Tone]
	}
	c := float64(nws(prose))
	r.S4 = min(c, float64(target)) / max(c, float64(target))
	r.Score = 0.6*r.S1 + 0.2*r.S2 + 0.1*r.S3 + 0.1*r.S4
	if r.Weighted > 0 {
		r.Eff = r.Score / (r.Weighted / 10000)
	}
}

// runScorecard: scores one render with the new verifier (stage 1 once + stage 2 once if needed).
func runScorecard(e *scEnv, tag string, s render.Scene, b render.Backend, prose string, v vSet, target int) scRun {
	r := scRun{Items: map[string]itemAns{}, Leak: map[string]bool{}, LeakQuote: map[string]string{}}
	sys, user := scPrompt1(s, b.Lang, prose, v)
	o, err := e.ask(tag+"_t1", sys, user, ids(v.Expect), false, 0, &r)
	if err != nil {
		r.Err = err.Error()
		r.G1Fail, r.G2Fail = true, true
		return r
	}
	for _, it := range o.Items {
		fp := yes(it.FactPresent)
		r.Items[it.ID] = itemAns{Present: fp, AsStated: fp && yes(it.AsStated), Quote: it.Quote}
	}
	for _, w := range o.Withheld {
		r.Leak[w.ID], r.LeakQuote[w.ID] = yes(w.Revealed), w.Quote
	}
	for _, x := range o.Other { // G2 candidates: excluded if explained by an author act in the IR (the Diff/authored rule as is)
		ev := render.NewEvent{Actor: x.Actor, Verb: x.Verb, Targets: x.Targets, Action: x.Action, Text: x.Text}
		if rep := render.Diff(s, render.Extracted{Acts: []render.ExtractedAct{}, FactsStated: []string{}, NewEvents: []render.NewEvent{ev}}); len(rep.Hallucinated) > 0 && !coveredByIR(s, ev) {
			r.Cands = append(r.Cands, cand{Ev: ev, Quote: x.Quote})
		}
	}
	// Stage 2: items that came out as absent/subject mismatch + G2 candidates only, once if there is any.
	var re []string
	for _, it := range v.Expect {
		if x := r.Items[it.ID]; !x.Present || !x.AsStated {
			re = append(re, it.ID)
		}
	}
	if len(re)+len(r.Cands) > 0 {
		r.Tier2++
		var u strings.Builder
		u.WriteString(catalog(s, b.Lang) + checklist(v) + "\n## Prose\n" + prose + "\n\n## Recheck these items\n")
		byID := map[string]vItem{}
		for _, it := range v.Expect {
			byID[it.ID] = it
		}
		for _, id := range re {
			u.WriteString(id + ". " + byID[id].Text + "\n")
		}
		u.WriteString("\n## Candidate extra events (found by a first pass)\n")
		for i, c := range r.Cands {
			fmt.Fprintf(&u, "C%d. %s (quote: %q)\n", i+1, c.Ev.Label(), c.Quote)
		}
		o2, err := e.ask(tag+"_t2", scSys2(), u.String(), re, true, len(r.Cands), &r)
		for i := range r.Cands {
			r.Cands[i].Confirmed, r.Cands[i].Verdict = true, "unchecked" // if stage 2 breaks, keep conservatively
		}
		if err != nil {
			r.Err = "t2: " + err.Error()
		} else {
			for _, it := range o2.Recheck {
				if _, ok := r.Items[it.ID]; ok {
					fp := yes(it.FactPresent)
					r.Items[it.ID] = itemAns{Present: fp, AsStated: fp && yes(it.AsStated), Quote: it.Quote, T2: true}
				}
			}
			for _, c := range o2.Candidates {
				var i int
				if _, err := fmt.Sscanf(c.ID, "C%d", &i); err == nil && i >= 1 && i <= len(r.Cands) {
					r.Cands[i-1].Verdict, r.Cands[i-1].Confirmed = c.Verdict, c.Verdict == "extra"
				}
			}
		}
	}
	for _, c := range r.Cands {
		r.G2Fail = r.G2Fail || c.Confirmed
	}
	for _, y := range r.Leak {
		r.G1Fail = r.G1Fail || y
	}
	u := e.m.Total()
	r.In, r.Out, r.CR, r.CW = u.Input, u.Output, u.CacheRead, u.CacheWrite
	r.Weighted = weighted(u)
	scoreRun(&r, v, b, prose, target)
	return r
}

func resolveID(s render.Scene, x string) string {
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
// render's authored() only looks at does and presence, so match the other verb classes by the same subject, same verb class and overlapping objects.
// An act with a fact is covered by the checklist, so it is not looked at here (a re-statement is filtered by stage 2's "covered").
func coveredByIR(s render.Scene, ev render.NewEvent) bool {
	actor := resolveID(s, ev.Actor)
	if actor == "" {
		return false
	}
	for _, bt := range s.Beats {
		for _, a := range bt.Acts {
			if a.Fact != "" || a.Actor != actor || render.VerbClass(a.Verb) != render.VerbClass(ev.Verb) {
				continue
			}
			if len(ev.Targets) == 0 || len(a.Targets) == 0 {
				return true
			}
			for _, t := range ev.Targets {
				for _, u := range a.Targets {
					if resolveID(s, t) == u {
						return true
					}
				}
			}
		}
	}
	return false
}
