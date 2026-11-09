// Package mutate holds the IR mutation operators. It makes mutants by injecting one defect into a work the author wrote,
// and accept decides whether the linter catches that defect (whether one of the Expect rules emits a new warn).
//
// Target selection looks only at IR syntax (who uses which proposition in which act). Time solving and belief derivation are
// not done (ADR-010). "Used later" and move-before's sources are passed through Mutation.Pairs/Uses and
// the harness (accept.RunMutants) decides and fills them in with PG.
package mutate

import (
	"math/rand"
	"slices"
	"strings"

	"github.com/starthalf51/diegesis-lang/internal/ir"
)

// Mutation: Op operator, Target (act ID, `entity@scene`, `act prop`, setup ID, died point ID, edge `from to`),
// Expect: if any one of these appears as a new warn inside Site, it is detected.
// Site = mutation site (Review Focus 5): the mutated act/statement/setup/point, that act's scene, acts that use the mutated proposition,
// the other end of an edge whose endpoint is the mutated point (for a scene boundary that scene and its acts, for born/died that person's acts).
// The harness counts a detection only when the new diagnostic's target (for time-cycle, one of the points the cycle passes through) is in Site.
// Pairs = candidates of (point of the act that received the information, point of the act that uses it). The harness runs only mutants that have at least one
// pair where "the user is later" by PG precedes (Go does not judge time). If empty, nothing is filtered.
// Before = the source points that move-before must come ahead of. The harness fills them from PG's believes.
// Uses = the propositions used by the move-before target act (canonical form). The harness uses them when looking up the source.
type Mutation struct {
	Op, Target string
	Expect     []string
	Site       []string
	Pairs      [][2]string
	Uses       []string
	Before     []string
}

// info-flow and its kin. The three are the same check split by target act kind (one diagnostic per defect, Task 6).
var infoRules = []string{"info-flow", "letter-unknown-info", "because-unbelieved"}

var opExpect = map[string][]string{
	"drop-tells":     append([]string{"undelivered-message"}, infoRules...),
	"drop-enter":     infoRules,
	"move-before":    infoRules,
	"drop-setup":     {"payoff-without-setup"},
	"drop-payoff":    {"unpaid-promise"},
	"kill-early":     {"dead-actor"},
	"flip-perceived": append([]string{"belief-contradiction"}, infoRules...),
	// Where it shows up when time is moved to contradict the source: a cycle with other constraints, an act after death, knowing something that has not happened yet.
	// shift-duration is also detected otherwise by work query mismatch (the harness reruns the queries).
	"shift-duration": append([]string{"time-cycle", "dead-actor"}, infoRules...),
	"swap-order":     append([]string{"time-cycle", "dead-actor"}, infoRules...),
}

var ops = []string{"drop-tells", "drop-enter", "move-before", "drop-setup", "drop-payoff", "kill-early", "flip-perceived",
	"shift-duration", "swap-order"}

// Shift is the value shift-duration adds to an interval (50 years, in seconds). It is bound to break a genealogy where ages and years are exact.
const Shift = 50 * 365 * 24 * 3600

// Generate makes every target each operator can apply to (an operator with no target emits nothing).
// If n > 0, shuffle with seed and keep only n (original order preserved).
func Generate(w *ir.Work, seed int64, n int) []Mutation {
	x := index(w)
	var out []Mutation
	add := func(op, target string, site []string, pairs [][2]string, uses []string) {
		out = append(out, Mutation{Op: op, Target: target, Expect: opExpect[op], Site: uniq(site), Pairs: pairs, Uses: uses})
	}
	for _, op := range ops {
		switch op {
		case "drop-tells":
			for _, a := range w.Acts {
				if pairs := x.heardAndUsed(a, false); telling[a.Verb] && len(pairs) > 0 {
					add(op, a.ID, append([]string{a.ID, a.SceneID}, x.usersOf(content(a))...), pairs, nil)
				}
			}
		case "drop-enter":
			for _, k := range x.presence {
				if x.exempt(k.entity) {
					continue
				}
				var pairs [][2]string
				site := []string{k.scene}
				for _, a := range w.Acts {
					if a.SceneID == k.scene && slices.Contains(x.aud[a.ID], k.entity) && assertive[a.Verb] {
						pairs = append(pairs, x.usedElsewhere(k.entity, a)...)
						site = append(append(site, a.ID), x.usersOf(content(a))...)
					}
				}
				if len(pairs) > 0 {
					add(op, k.entity+"@"+k.scene, site, pairs, nil)
				}
			}
		case "move-before":
			for _, a := range w.Acts {
				if a.ActorIsRole || x.exempt(a.ActorID) {
					continue
				}
				var uses []string
				for _, p := range x.used(a) {
					if !x.background(a.ActorID, p) && !slices.Contains(uses, p) {
						uses = append(uses, p)
					}
				}
				if len(uses) > 0 {
					add(op, a.ID, append([]string{a.ID, a.SceneID}, x.usersOf(uses)...), nil, uses)
				}
			}
		case "drop-setup", "drop-payoff":
			for _, s := range w.Setups {
				if (op == "drop-setup" || s.Kind == "promise") && slices.ContainsFunc(w.Payoffs, func(p ir.Payoff) bool { return p.SetupID == s.ID }) {
					add(op, s.ID, []string{s.ID, s.SceneID}, nil, nil)
				}
			}
		case "kill-early":
			for _, p := range w.Points {
				if p.Kind == "died" && x.lastAct(strings.TrimSuffix(p.ID, ".died")) != "" {
					add(op, p.ID, x.around(p.ID), nil, nil)
				}
			}
		case "flip-perceived":
			for _, a := range w.Acts {
				if a.Verb != "sees" && a.Verb != "tells" && a.Verb != "says" {
					continue
				}
				for _, p := range content(a) {
					one := a
					one.PropID, one.With = p, nil
					if pairs := x.heardAndUsed(one, a.Verb == "sees"); len(pairs) > 0 {
						add(op, a.ID+" "+p, append([]string{a.ID, a.SceneID}, x.usersOf([]string{p})...), pairs, nil)
					}
				}
			}
		case "shift-duration", "swap-order":
			for _, e := range w.TimeEdges {
				if e.Waived || e.TargetID == "" {
					continue
				}
				if op == "shift-duration" && (e.Min == nil || e.Max == nil) {
					continue
				}
				// swap-order: born→died of the same person, or `before` (where the statement's own point is the edge's start)
				// lasts (the scene's start→end) is the same edge when flipped, so it is left out
				if op == "swap-order" && (!(lifespan(e) || e.From == e.TargetID || strings.HasPrefix(e.From, e.TargetID+".")) ||
					e.From == e.TargetID+".start" && e.To == e.TargetID+".end") {
					continue
				}
				add(op, e.From+" "+e.To, append(x.around(e.From, e.To), e.TargetID), nil, nil)
			}
		}
	}
	if n > 0 && n < len(out) {
		pick := rand.New(rand.NewSource(seed)).Perm(len(out))[:n]
		slices.Sort(pick)
		keep := make([]Mutation, n)
		for i, j := range pick {
			keep[i] = out[j]
		}
		out = keep
	}
	return out
}

func lifespan(e ir.TimeEdge) bool {
	p, ok := strings.CutSuffix(e.From, ".born")
	return ok && e.To == p+".died"
}

func uniq(s []string) []string {
	var out []string
	for _, x := range s {
		if x != "" && !slices.Contains(out, x) {
			out = append(out, x)
		}
	}
	return out
}

// usersOf: acts that use (used) one of the propositions (canonical form).
func (x *idx) usersOf(props []string) []string {
	var out []string
	for _, b := range x.w.Acts {
		if slices.ContainsFunc(x.used(b), func(c string) bool {
			return slices.ContainsFunc(props, func(p string) bool { return x.canon(p) == c })
		}) {
			out = append(out, b.ID)
		}
	}
	return out
}

// around: points and their neighbors — for a scene/event boundary that container and the acts inside, for an act point that act,
// for `x.born`/`x.died` x's acts, and the statements of edges with this point as an endpoint (waiver targets).
func (x *idx) around(pts ...string) []string {
	out := slices.Clone(pts)
	for _, p := range pts {
		for _, s := range x.w.Scenes {
			if s.StartPt == p || s.EndPt == p {
				out = append(out, s.ID)
				for _, a := range x.w.Acts {
					if a.SceneID == s.ID {
						out = append(out, a.ID)
					}
				}
			}
		}
		e, born := strings.CutSuffix(p, ".born")
		if d, died := strings.CutSuffix(p, ".died"); died {
			e = d
		} else if !born {
			e = ""
		}
		for _, a := range x.w.Acts {
			if a.Pt == p || (e != "" && a.ActorID == e) {
				out = append(out, a.ID)
			}
		}
		for _, t := range x.w.TimeEdges {
			if t.From == p || t.To == p {
				out = append(out, t.TargetID)
			}
		}
	}
	return out
}

// telling is the target verb of drop-tells (an utterance that conveys information). sends is a promise so it is excluded (delivery is on the tells side).
var telling = map[string]bool{"tells": true, "says": true, "whispers": true, "hails": true, "shows": true}

// assertive is the same verb list as 003's assertive() (the audience believes the content).
var assertive = map[string]bool{"tells": true, "says": true, "whispers": true, "hails": true, "lies": true,
	"shows": true, "decrees": true, "sends": true}

func content(a ir.Act) []string {
	var out []string
	if a.PropID != "" {
		out = append(out, a.PropID)
	}
	return append(out, a.With...)
}

type key struct{ entity, scene string }

type idx struct {
	w        *ir.Work
	aud      map[string][]string // act → audience
	alias    map[string]string   // fact → alias target (one level, same as lint_canon)
	groups   map[string][]string // entity → groups it is a direct member of
	presence []key
}

func index(w *ir.Work) *idx {
	x := &idx{w: w, aud: map[string][]string{}, alias: map[string]string{}, groups: map[string][]string{}}
	for _, a := range w.Audience {
		x.aud[a.ActID] = append(x.aud[a.ActID], a.EntityID)
	}
	for _, f := range w.Facts {
		if f.Def != nil && f.Def.Alias != "" {
			x.alias[f.ID] = f.Def.Alias
		}
	}
	for _, g := range w.GroupMembers {
		x.groups[g.MemberID] = append(x.groups[g.MemberID], g.GroupID)
	}
	for _, p := range w.Presence {
		if k := (key{p.EntityID, p.SceneID}); !slices.Contains(x.presence, k) {
			x.presence = append(x.presence, k)
		}
	}
	return x
}

func (x *idx) canon(p string) string {
	if q, ok := x.alias[p]; ok {
		return q
	}
	return p
}

// up is e and the groups e (indirectly) belongs to.
func (x *idx) up(e string) []string {
	out := []string{e}
	for i := 0; i < len(out); i++ {
		for _, g := range x.groups[out[i]] {
			if !slices.Contains(out, g) {
				out = append(out, g)
			}
		}
	}
	return out
}

func (x *idx) exempt(e string) bool {
	ups := x.up(e)
	return slices.ContainsFunc(x.w.Entities, func(en ir.Entity) bool { return en.Knows != "" && slices.Contains(ups, en.ID) })
}

// background: e (or a group it belongs to) believes P from the start (a believes with no point).
func (x *idx) background(e, p string) bool {
	ups := x.up(e)
	return slices.ContainsFunc(x.w.Beliefs, func(b ir.Belief) bool {
		return b.AtPt == "" && b.Verb == "believes" && slices.Contains(ups, b.EntityID) && x.canon(b.PropID) == x.canon(p)
	})
}

// used is the proposition the speaker of act a uses in that act (canonical form): the content of a truthful assertion, the document content of writes, because.
func (x *idx) used(a ir.Act) []string {
	var out []string
	switch {
	case assertive[a.Verb] && a.Verb != "lies" && a.Verb != "decrees":
		out = content(a)
	case a.Verb == "writes":
		for _, art := range x.w.Artifacts {
			if slices.Contains(a.Objects, art.ID) {
				out = append(out, art.Contains...)
			}
		}
	}
	if a.BecauseID != "" {
		out = append(out, a.BecauseID)
	}
	for i := range out {
		out[i] = x.canon(out[i])
	}
	return out
}

// usedElsewhere: for each other act where e uses a non-background proposition from a's content, a pair (a's point, that act's point).
// "Is it used later" is not judged here (the harness filters with PG precedes).
func (x *idx) usedElsewhere(e string, a ir.Act) [][2]string {
	var out [][2]string
	for _, p := range content(a) {
		c := x.canon(p)
		if x.background(e, c) {
			continue
		}
		for _, b := range x.w.Acts {
			if b.ID != a.ID && b.ActorID == e && slices.Contains(x.used(b), c) && !slices.Contains(out, [2]string{a.Pt, b.Pt}) {
				out = append(out, [2]string{a.Pt, b.Pt})
			}
		}
	}
	return out
}

// heardAndUsed: the usedElsewhere pairs of the non-exempt people among those who received a's content (self = when the speaker perceived it, otherwise the audience).
// usedElsewhere pairs.
func (x *idx) heardAndUsed(a ir.Act, self bool) [][2]string {
	who := x.aud[a.ID]
	if self {
		who = []string{a.ActorID}
	}
	var out [][2]string
	for _, e := range who {
		if (self || e != a.ActorID) && !x.exempt(e) {
			out = append(out, x.usedElsewhere(e, a)...)
		}
	}
	return out
}

// lastAct is the point of the last act where e is the speaker (IR order = source order). "" if none.
func (x *idx) lastAct(e string) string {
	last := ""
	for _, a := range x.w.Acts {
		if a.ActorID == e {
			last = a.Pt
		}
	}
	return last
}
