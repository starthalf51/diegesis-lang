// Package mutate 는 IR 변이 연산자다. 작가가 쓴 작품 하나에 결함 하나를 넣은 변이본을 만들고,
// 린터가 그 결함을 잡는지(Expect 규칙 중 하나가 새 warn 을 내는지)를 accept 가 판정한다.
//
// 대상 고르기는 IR 의 구문만 본다(누가 어떤 명제를 어떤 행위에서 쓰는가). 시간 풀이와 믿음 파생은
// 하지 않는다(ADR-010). 그래서 "나중에 쓴다" 는 "다른 행위에서 쓴다" 로 넓게 잡는다 — 출처가 따로 있어
// 탐지되지 않는 변이는 생존으로 남고 보고서에 그대로 나온다.
package mutate

import (
	"math/rand"
	"slices"
	"strings"

	"story-lang/internal/ir"
)

// Mutation: Op 연산자, Target 대상(행위 ID, `entity@scene`, `act prop`, setup ID, died 시점 ID),
// Expect 이 중 하나라도 새 warn 으로 나오면 탐지.
type Mutation struct {
	Op, Target string
	Expect     []string
}

// info-flow 와 그 동족. 셋은 같은 검사를 대상 행위 종류로 나눈 것이다(한 결함에 진단 하나, Task 6).
var infoRules = []string{"info-flow", "letter-unknown-info", "because-unbelieved"}

var opExpect = map[string][]string{
	"drop-tells":     append([]string{"undelivered-message"}, infoRules...),
	"drop-enter":     infoRules,
	"move-before":    infoRules,
	"drop-setup":     {"payoff-without-setup"},
	"drop-payoff":    {"unpaid-promise"},
	"kill-early":     {"dead-actor"},
	"flip-perceived": append([]string{"belief-contradiction"}, infoRules...),
}

var ops = []string{"drop-tells", "drop-enter", "move-before", "drop-setup", "drop-payoff", "kill-early", "flip-perceived"}

// Generate 는 연산자마다 적용할 수 있는 대상을 모두 만든다(대상이 없는 연산자는 하나도 내지 않는다).
// n > 0 이면 seed 로 섞어 n 개만 남긴다(원래 순서는 유지).
func Generate(w *ir.Work, seed int64, n int) []Mutation {
	x := index(w)
	var out []Mutation
	add := func(op, target string) { out = append(out, Mutation{Op: op, Target: target, Expect: opExpect[op]}) }
	for _, op := range ops {
		switch op {
		case "drop-tells":
			for _, a := range w.Acts {
				if telling[a.Verb] && x.heardAndUsed(a, false) {
					add(op, a.ID)
				}
			}
		case "drop-enter":
			for _, k := range x.presence {
				if x.exempt(k.entity) {
					continue
				}
				for _, a := range w.Acts {
					if a.SceneID == k.scene && slices.Contains(x.aud[a.ID], k.entity) && assertive[a.Verb] && x.usedElsewhere(k.entity, a) {
						add(op, k.entity+"@"+k.scene)
						break
					}
				}
			}
		case "move-before":
			for _, a := range w.Acts {
				if !a.ActorIsRole && !x.exempt(a.ActorID) && slices.ContainsFunc(x.used(a), func(p string) bool { return !x.background(a.ActorID, p) }) {
					add(op, a.ID)
				}
			}
		case "drop-setup", "drop-payoff":
			for _, s := range w.Setups {
				if (op == "drop-setup" || s.Kind == "promise") && slices.ContainsFunc(w.Payoffs, func(p ir.Payoff) bool { return p.SetupID == s.ID }) {
					add(op, s.ID)
				}
			}
		case "kill-early":
			for _, p := range w.Points {
				if p.Kind == "died" && x.lastAct(strings.TrimSuffix(p.ID, ".died")) != "" {
					add(op, p.ID)
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
					if x.heardAndUsed(one, a.Verb == "sees") {
						add(op, a.ID+" "+p)
					}
				}
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

// telling 은 drop-tells 의 대상 동사(정보를 전하는 발화). sends 는 약속이라 뺀다(전달은 tells 쪽이다).
var telling = map[string]bool{"tells": true, "says": true, "whispers": true, "hails": true, "shows": true}

// assertive 는 003 의 assertive() 와 같은 동사 목록이다(청중이 내용을 믿는다).
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
	alias    map[string]string   // fact → alias target (한 단계, lint_canon 과 같다)
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

// up 은 e 와 e 가 (간접) 속한 그룹들.
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

// background: e(또는 속한 그룹)가 처음부터 P 를 믿는다(시점 없는 believes).
func (x *idx) background(e, p string) bool {
	ups := x.up(e)
	return slices.ContainsFunc(x.w.Beliefs, func(b ir.Belief) bool {
		return b.AtPt == "" && b.Verb == "believes" && slices.Contains(ups, b.EntityID) && x.canon(b.PropID) == x.canon(p)
	})
}

// used 는 행위 a 의 화자가 그 행위에서 쓰는 명제(정규형): 진실한 단언의 내용, writes 의 문서 내용, because.
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

// usedElsewhere: e 가 a 의 내용 중 하나를 다른 행위에서 쓰고, 그 명제가 e 의 배경 지식이 아니다.
func (x *idx) usedElsewhere(e string, a ir.Act) bool {
	for _, p := range content(a) {
		c := x.canon(p)
		if x.background(e, c) {
			continue
		}
		for _, b := range x.w.Acts {
			if b.ID != a.ID && b.ActorID == e && slices.Contains(x.used(b), c) {
				return true
			}
		}
	}
	return false
}

// heardAndUsed: a 의 내용을 받은 사람(self = 화자 자신이 지각한 경우, 아니면 청중) 중 면제가 아닌 누군가가
// 그 내용을 다른 행위에서 쓴다.
func (x *idx) heardAndUsed(a ir.Act, self bool) bool {
	who := x.aud[a.ID]
	if self {
		who = []string{a.ActorID}
	}
	return slices.ContainsFunc(who, func(e string) bool {
		return (self || e != a.ActorID) && !x.exempt(e) && x.usedElsewhere(e, a)
	})
}

// lastAct 는 e 가 화자인 마지막 행위(IR 순서 = 원문 순서)의 시점. 없으면 "".
func (x *idx) lastAct(e string) string {
	last := ""
	for _, a := range x.w.Acts {
		if a.ActorID == e {
			last = a.Pt
		}
	}
	return last
}
