// Package mutate 는 IR 변이 연산자다. 작가가 쓴 작품 하나에 결함 하나를 넣은 변이본을 만들고,
// 린터가 그 결함을 잡는지(Expect 규칙 중 하나가 새 warn 을 내는지)를 accept 가 판정한다.
//
// 대상 고르기는 IR 의 구문만 본다(누가 어떤 명제를 어떤 행위에서 쓰는가). 시간 풀이와 믿음 파생은
// 하지 않는다(ADR-010). "나중에 쓴다" 와 move-before 의 출처는 Mutation.Pairs/Uses 로 넘기고
// 하네스(accept.RunMutants)가 PG 로 판정·보충한다.
package mutate

import (
	"math/rand"
	"slices"
	"strings"

	"story-lang/internal/ir"
)

// Mutation: Op 연산자, Target 대상(행위 ID, `entity@scene`, `act prop`, setup ID, died 시점 ID),
// Expect 이 중 하나라도 새 warn 으로 나오면 탐지.
// Pairs = (정보를 받은 행위 시점, 그 정보를 쓰는 행위 시점) 후보. 하네스가 PG precedes 로 "쓰는 쪽이 뒤"인
// 쌍이 하나라도 있는 변이만 돌린다(Go 는 시간을 판정하지 않는다). 비어 있으면 거르지 않는다.
// Before = move-before 가 앞서야 할 출처 시점들. 하네스가 PG 의 believes 에서 채운다.
// Uses = move-before 대상 행위가 쓰는 명제(정규형). 하네스가 그 출처를 찾을 때 쓴다.
type Mutation struct {
	Op, Target string
	Expect     []string
	Pairs      [][2]string
	Uses       []string
	Before     []string
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
	add := func(op, target string, pairs [][2]string, uses []string) {
		out = append(out, Mutation{Op: op, Target: target, Expect: opExpect[op], Pairs: pairs, Uses: uses})
	}
	for _, op := range ops {
		switch op {
		case "drop-tells":
			for _, a := range w.Acts {
				if pairs := x.heardAndUsed(a, false); telling[a.Verb] && len(pairs) > 0 {
					add(op, a.ID, pairs, nil)
				}
			}
		case "drop-enter":
			for _, k := range x.presence {
				if x.exempt(k.entity) {
					continue
				}
				var pairs [][2]string
				for _, a := range w.Acts {
					if a.SceneID == k.scene && slices.Contains(x.aud[a.ID], k.entity) && assertive[a.Verb] {
						pairs = append(pairs, x.usedElsewhere(k.entity, a)...)
					}
				}
				if len(pairs) > 0 {
					add(op, k.entity+"@"+k.scene, pairs, nil)
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
					add(op, a.ID, nil, uses)
				}
			}
		case "drop-setup", "drop-payoff":
			for _, s := range w.Setups {
				if (op == "drop-setup" || s.Kind == "promise") && slices.ContainsFunc(w.Payoffs, func(p ir.Payoff) bool { return p.SetupID == s.ID }) {
					add(op, s.ID, nil, nil)
				}
			}
		case "kill-early":
			for _, p := range w.Points {
				if p.Kind == "died" && x.lastAct(strings.TrimSuffix(p.ID, ".died")) != "" {
					add(op, p.ID, nil, nil)
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
						add(op, a.ID+" "+p, pairs, nil)
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

// usedElsewhere: e 가 a 의 내용 중 배경 지식이 아닌 명제를 쓰는 다른 행위마다 (a 시점, 그 행위 시점) 쌍.
// "뒤에 쓰는가" 는 여기서 판정하지 않는다(하네스가 PG precedes 로 거른다).
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

// heardAndUsed: a 의 내용을 받은 사람(self = 화자 자신이 지각한 경우, 아니면 청중) 중 면제가 아닌 사람들의
// usedElsewhere 쌍.
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
