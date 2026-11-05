package syntax

import (
	"fmt"
	"strings"
	"testing"
)

// Every production in spec/grammar.ebnf, including the language additions.
const coverage = `module macbeth.cast
import macbeth.common

work macbeth "The Tragedy of Macbeth"
  unit day
  start 1040-08
  name ko "맥베스의 비극"

character macbeth "Macbeth" major
  name ko "맥베스"
character cawdor "the elder Thane of Cawdor" minor   # 무대에 한 번도 안 나온다
group witches "the Weird Sisters" major
  members witch1, witch2, witch3
  knows oracle
character athena "Athena"
  knows omniscient
place heath "a heath"
distance camp heath 0d..1d
role thane_of_cawdor "Thane of Cawdor", "Cawdor"
  held cawdor until @1.2.decree
  held macbeth from @1.2.decree
role chancellor "승상"
  shared
  held cao_cao from start until @chibi.end
fact cawdor_traitor "Cawdor aided Norway against Duncan" true
fact king_hereafter = holds(king, macbeth)
artifact macbeth_letter "Macbeth's letter"
  contains holds(thane_of_cawdor, macbeth), king_hereafter
  "They met me in the day of success"
thread scotland "Scotland"
parent adam of seth
parent noah of shem, ham, japheth
born seth when after @adam.born by 130y..130y
dies methuselah when after @methuselah.born by 969y..969y
#! allow time-cycle: "5:32는 첫아들 기준으로 읽는 것이 일반적"
born shem when after @noah.born by 500y..500y
setup kings promise: "Banquo's line"

event macbeth_writes "Macbeth writes to his wife"
  when after 1.3.ross_news
  when before 1.5
  macbeth writes macbeth_letter

scene 45.1 at camp in wei
  when at 208-11 dawn
  when during 44.9
  when before 46.1 by 1h..12h
  discourse after 44.9
  enter 曹操, 蔣幹
  mention zhou_yu
  role:chancellor tells 蔣幹: commanders_traitors src rtk.45.12
  曹操 does execute cai_mao, zhang_yun because commanders_traitors
  蔣幹 lies 曹操: not loyal(cai_mao) "they plot against you" src "45:3"
  odysseus whispers eurycleia: beggar_is_odysseus
  eurycleia sees odysseus: beggar_is_odysseus
  曹操 says: holds(chancellor, 曹操) "I am the chancellor"
  witch1 hails macbeth: holds(thane_of_glamis, macbeth)
  macbeth aside: "Two truths are told"
  macbeth thinks: "why do I yield"
  witches shows macbeth: banquo_line
  lady_macbeth reads macbeth_letter
  macbeth writes macbeth_letter
  duncan sends ross, angus to macbeth with holds(thane_of_cawdor, macbeth), king_hereafter
  duncan orders ross: not alive(cawdor) "go pronounce his present death"
  duncan decrees: holds(thane_of_cawdor, macbeth)
  曹操 believes commanders_traitors
  曹操 doubts letter_genuine because @45.1.theft
  setup forged_hint hint: letter_forged
  reveal letter_forged at @46.2
  withhold letter_forged until 46.2
  payoff macbeth.act1/banquo_kings
  exit 蔣幹
  exit all
  render
    pov 曹操 close
    show "the burning ships"
    omit letter.full_text

account mark of peter_denies by mark
  when during 14.66
  maid asks peter: knows(peter, jesus)
  peter says: not knows(peter, jesus) "I know not this man"
`

func propStr(p *Prop) string {
	if p == nil {
		return ""
	}
	s := p.Name
	if p.Call {
		var args []string
		for _, a := range p.Args {
			args = append(args, propStr(a))
		}
		s += "(" + strings.Join(args, ", ") + ")"
	}
	if p.Not {
		s = "not " + s
	}
	return s
}

func props(ps []*Prop) string {
	var out []string
	for _, p := range ps {
		out = append(out, propStr(p))
	}
	return strings.Join(out, "; ")
}

func names(ids []Ident) string {
	var out []string
	for _, id := range ids {
		out = append(out, id.Name)
	}
	return strings.Join(out, ",")
}

func tref(t *Tref) string {
	if t == nil {
		return ""
	}
	if t.At {
		return "@" + t.Path
	}
	return t.Path
}

func find[T Node](items []Node, ok func(T) bool) T {
	for _, it := range items {
		if x, is := it.(T); is && ok(x) {
			return x
		}
	}
	var zero T
	panic(fmt.Sprintf("no %T found", zero))
}

func decl(items []Node, id string) *Decl {
	return find(items, func(d *Decl) bool { return d.ID.Name == id })
}

func TestParseCoverage(t *testing.T) {
	f, diags := ParseFile("cast.sl", []byte(coverage))
	for _, d := range diags {
		t.Errorf("%s", d)
	}
	if f.Module == nil || f.Module.Name != "macbeth.cast" || len(f.Imports) != 1 || f.Imports[0].Name != "macbeth.common" {
		t.Fatalf("header = %+v %+v", f.Module, f.Imports)
	}
	kinds := map[string]int{}
	for _, it := range f.Items {
		kinds[strings.TrimPrefix(fmt.Sprintf("%T", it), "*syntax.")]++
	}
	for k, n := range map[string]int{"Work": 1, "Decl": 11, "Distance": 1, "Parent": 2, "Life": 3,
		"Pragma": 1, "Setup": 1, "Event": 1, "Scene": 1, "Account": 1} {
		if kinds[k] != n {
			t.Errorf("%s count = %d, want %d (all %v)", k, kinds[k], n, kinds)
		}
	}
	check := func(what, got, want string) {
		t.Helper()
		if got != want {
			t.Errorf("%s = %q, want %q", what, got, want)
		}
	}
	const day, year = 86400, 365 * 86400

	w := find(f.Items, func(*Work) bool { return true })
	check("work", fmt.Sprint(w.ID.Name, w.Title, w.Attrs[0].(*Unit).Unit, w.Attrs[1].(*Start).Date, w.Attrs[2].(*Name).Text),
		"macbethThe Tragedy of Macbethday1040-08맥베스의 비극")

	// declarations and attributes
	m := decl(f.Items, "macbeth")
	check("macbeth", fmt.Sprint(m.Kind, m.Names, m.Tier, m.Attrs[0].(*Name).Lang), "character[Macbeth]majorko")
	wi := decl(f.Items, "witches")
	check("witches", fmt.Sprint(wi.Kind, names(wi.Attrs[0].(*Members).IDs), wi.Attrs[1].(*Knows).Source), "groupwitch1,witch2,witch3oracle")
	check("athena knows", decl(f.Items, "athena").Attrs[0].(*Knows).Source, "omniscient")
	tc := decl(f.Items, "thane_of_cawdor")
	h0, h1 := tc.Attrs[0].(*Held), tc.Attrs[1].(*Held)
	check("thane names", fmt.Sprint(tc.Kind, tc.Names), "role[Thane of Cawdor Cawdor]")
	check("held 0", fmt.Sprint(h0.Holder.Name, h0.From == nil, tref(h0.Until)), "cawdortrue@1.2.decree")
	check("held 1", fmt.Sprint(h1.Holder.Name, tref(h1.From), h1.Until == nil), "macbeth@1.2.decreetrue")
	ch := decl(f.Items, "chancellor")
	_ = ch.Attrs[0].(*Shared)
	hc := ch.Attrs[1].(*Held)
	check("chancellor held", fmt.Sprint(hc.Holder.Name, tref(hc.From), tref(hc.Until)), "cao_caostart@chibi.end")
	ct := decl(f.Items, "cawdor_traitor")
	check("fact value", fmt.Sprint(ct.Names, ct.Value, ct.Def == nil), "[Cawdor aided Norway against Duncan]truetrue")
	check("fact def", propStr(decl(f.Items, "king_hereafter").Def), "holds(king, macbeth)")
	art := decl(f.Items, "macbeth_letter")
	check("artifact", fmt.Sprint(props(art.Attrs[0].(*Contains).Props), "|", art.Attrs[1].(*Text).Text),
		"holds(thane_of_cawdor, macbeth); king_hereafter|They met me in the day of success")
	d := find(f.Items, func(*Distance) bool { return true })
	check("distance", fmt.Sprint(d.A.Name, d.B.Name, d.Min.Sec, d.Max.Sec), fmt.Sprint("camp", "heath", 0, day))

	// genealogy and life events, year units in seconds (1y = 365d)
	p := find(f.Items, func(p *Parent) bool { return p.Parent.Name == "noah" })
	check("parent", names(p.Children), "shem,ham,japheth")
	check("parent adam", names(find(f.Items, func(p *Parent) bool { return p.Parent.Name == "adam" }).Children), "seth")
	born := find(f.Items, func(l *Life) bool { return l.Who.Name == "seth" })
	check("born seth", fmt.Sprint(born.Kind, born.When.Op, tref(born.When.Ref), born.When.Min.N, born.When.Min.Unit, born.When.Min.Sec, born.When.Max.Sec),
		fmt.Sprint("born", "after", "@adam.born", 130, "y", 130*year, 130*year))
	dies := find(f.Items, func(l *Life) bool { return l.Who.Name == "methuselah" })
	check("dies", fmt.Sprint(dies.Kind, tref(dies.When.Ref), dies.When.Max.Sec), fmt.Sprint("dies", "@methuselah.born", 969*year))
	pr := find(f.Items, func(*Pragma) bool { return true })
	check("pragma", pr.Rule, "time-cycle")
	top := find(f.Items, func(*Setup) bool { return true })
	check("setup top", fmt.Sprint(top.ID.Name, top.Kind, top.Text), "kingspromiseBanquo's line")

	ev := find(f.Items, func(*Event) bool { return true })
	check("event", fmt.Sprint(ev.ID.Name, ev.Title, len(ev.Body), ev.Body[0].(*When).Op, tref(ev.Body[0].(*When).Ref)),
		"macbeth_writesMacbeth writes to his wife3after1.3.ross_news")

	// scene statements
	sc := find(f.Items, func(*Scene) bool { return true })
	check("scene", fmt.Sprint(sc.ID.Name, sc.At.Name, sc.In.Name), "45.1campwei")
	w0, w1, w2 := sc.Body[0].(*When), sc.Body[1].(*When), sc.Body[2].(*When)
	check("when at", fmt.Sprint(w0.Op, w0.Date, w0.Time), "at208-11dawn")
	check("when during", fmt.Sprint(w1.Op, tref(w1.Ref)), "during44.9")
	check("when before", fmt.Sprint(w2.Op, tref(w2.Ref), w2.Min.Sec, w2.Max.Sec), fmt.Sprint("before", "46.1", 3600, 12*3600))
	check("discourse", tref(&sc.Body[3].(*Discourse).After), "44.9")
	check("enter", names(sc.Body[4].(*Presence).IDs), "曹操,蔣幹")
	check("mention", sc.Body[5].(*Presence).Op+names(sc.Body[5].(*Presence).IDs), "mentionzhou_yu")

	type actWant struct{ actor, objects, to, with, prop, quote, because, src, action string }
	acts := map[string]actWant{}
	var role *Act
	for _, n := range sc.Body {
		if a, ok := n.(*Act); ok {
			if a.Role {
				role = a
			}
			if _, dup := acts[a.Verb]; !dup {
				acts[a.Verb] = actWant{a.Actor.Name, names(a.Objects), names(a.To), props(a.With), propStr(a.Prop), a.Quote, propStr(a.Because), a.Src, a.Action}
			}
		}
	}
	acc := find(f.Items, func(*Account) bool { return true })
	check("account", fmt.Sprint(acc.ID.Name, acc.Of.Name, acc.By.Name, len(acc.Body), acc.Body[0].(*When).Op), "markpeter_deniesmark3during")
	for _, n := range acc.Body[1:] {
		a := n.(*Act)
		acts["account "+a.Verb] = actWant{a.Actor.Name, names(a.Objects), "", "", propStr(a.Prop), a.Quote, "", "", ""}
	}
	if role == nil || role.Actor.Name != "chancellor" || role.Verb != "tells" {
		t.Errorf("role: actor = %+v", role)
	}
	for verb, want := range map[string]actWant{
		"tells":        {actor: "chancellor", objects: "蔣幹", prop: "commanders_traitors", src: "rtk.45.12"},
		"does":         {actor: "曹操", action: "execute", objects: "cai_mao,zhang_yun", because: "commanders_traitors"},
		"lies":         {actor: "蔣幹", objects: "曹操", prop: "not loyal(cai_mao)", quote: "they plot against you", src: "45:3"},
		"whispers":     {actor: "odysseus", objects: "eurycleia", prop: "beggar_is_odysseus"},
		"sees":         {actor: "eurycleia", objects: "odysseus", prop: "beggar_is_odysseus"},
		"says":         {actor: "曹操", prop: "holds(chancellor, 曹操)", quote: "I am the chancellor"},
		"hails":        {actor: "witch1", objects: "macbeth", prop: "holds(thane_of_glamis, macbeth)"},
		"aside":        {actor: "macbeth", quote: "Two truths are told"},
		"thinks":       {actor: "macbeth", quote: "why do I yield"},
		"shows":        {actor: "witches", objects: "macbeth", prop: "banquo_line"},
		"reads":        {actor: "lady_macbeth", objects: "macbeth_letter"},
		"writes":       {actor: "macbeth", objects: "macbeth_letter"},
		"sends":        {actor: "duncan", objects: "ross,angus", to: "macbeth", with: "holds(thane_of_cawdor, macbeth); king_hereafter"},
		"orders":       {actor: "duncan", objects: "ross", prop: "not alive(cawdor)", quote: "go pronounce his present death"},
		"decrees":      {actor: "duncan", prop: "holds(thane_of_cawdor, macbeth)"},
		"account asks": {actor: "maid", objects: "peter", prop: "knows(peter, jesus)"},
		"account says": {actor: "peter", prop: "not knows(peter, jesus)", quote: "I know not this man"},
	} {
		if got := acts[verb]; got != want {
			t.Errorf("act %s = %+v, want %+v", verb, got, want)
		}
	}
	if len(verbs) != 16 {
		t.Errorf("verb table has %d verbs, want 16", len(verbs))
	}
	for v := range verbs {
		if _, ok := acts[v]; !ok && v != "asks" {
			t.Errorf("verb %s not covered in scene", v)
		}
	}

	b1 := find(sc.Body, func(b *Belief) bool { return b.Verb == "believes" })
	b2 := find(sc.Body, func(b *Belief) bool { return b.Verb == "doubts" })
	check("believes", fmt.Sprint(b1.Who.Name, propStr(b1.Prop), b1.Because == nil), "曹操commanders_traitorstrue")
	check("doubts", fmt.Sprint(b2.Who.Name, propStr(b2.Prop), tref(b2.Because)), "曹操letter_genuine@45.1.theft")
	su := find(sc.Body, func(*Setup) bool { return true })
	check("setup hint", fmt.Sprint(su.ID.Name, su.Kind, propStr(su.Prop)), "forged_hinthintletter_forged")
	rv := find(sc.Body, func(*Reveal) bool { return true })
	check("reveal", propStr(rv.Prop)+" "+tref(rv.At), "letter_forged @46.2")
	wh := find(sc.Body, func(*Withhold) bool { return true })
	check("withhold", propStr(wh.Prop)+" "+tref(&wh.Until), "letter_forged 46.2")
	check("payoff", find(sc.Body, func(*Payoff) bool { return true }).Ref.Name, "macbeth.act1/banquo_kings")
	ex := find(sc.Body, func(p *Presence) bool { return p.Op == "exit" && !p.All })
	check("exit", names(ex.IDs), "蔣幹")
	find(sc.Body, func(p *Presence) bool { return p.Op == "exit" && p.All })
	r := find(sc.Body, func(*Render) bool { return true })
	var es []string
	for _, e := range r.Entries {
		es = append(es, e.Key+"="+strings.Join(e.Values, "+"))
	}
	check("render", strings.Join(es, " "), "pov=曹操+close show=the burning ships omit=letter.full_text")
}

func TestFixRoundErrors(t *testing.T) {
	sceneFM := "---\nmodule: m\nscene: \"2\"\n"
	runCases(t, []errCase{
		{name: "dup fm key location", src: sceneFM + "location: x\nlocation: y\n---\n",
			want: []want{{5, 1, "dup-decl"}}},
		{name: "dup fm key characters", src: sceneFM + "characters: [a]\ncharacters: [b]\n---\n",
			want: []want{{5, 1, "dup-decl"}}},
		{name: "dup render map key", src: sceneFM + "render: {pace: slow, pace: fast}\n---\n",
			want: []want{{4, 22, "dup-decl"}}},
		{name: "dup decl map field", src: "---\nmodule: m\nplaces: {heath: {name: A, name: B}}\n---\n",
			want: []want{{3, 27, "dup-decl"}}},
		{name: "fm when vs block when", src: sceneFM + "when: after 1\n---\n```sl\nwhen before 3\n```\n",
			want: []want{{7, 1, "dup-decl"}}},
		{name: "fm date vs block when at", src: sceneFM + "date: 1805-07\n---\n```sl\nwhen at 1805-07\n```\n",
			want: []want{{7, 1, "dup-decl"}}},
		{name: "fm discourse vs block", src: sceneFM + "discourse: after 1\n---\n```sl\ndiscourse after 1\n```\n",
			want: []want{{7, 1, "dup-decl"}}},
		{name: "block when alone is fine", src: sceneFM + "---\n```sl\nwhen after 1\n```\n"},
		{name: "reserved word as id", src: "```sl\nmodule m\ncharacter when \"x\"\n```\n",
			want: []want{{3, 11, "syntax"}}},
		{name: "reserved word as fm id", src: "---\nmodule: m\ncharacters: {all: X}\n---\n",
			want: []want{{3, 14, "syntax"}}},
		{name: "bom before fence", src: bom + "```sl\nmodule m\n```\n"},
		{name: "bom before frontmatter", src: bom + "---\nmodule: m\n---\n"},
	})
}

func TestFrontmatterErrorPositions(t *testing.T) {
	sceneFM := "---\nmodule: m\nscene: \"1\"\n"
	runCases(t, []errCase{
		{name: "when missing ref", src: sceneFM + "when: after\n---\n",
			want: []want{{4, 11, "syntax"}}},
		{name: "empty location", src: sceneFM + "location:\n---\n",
			want: []want{{4, 1, "syntax"}}},
		{name: "empty characters is fine", src: sceneFM + "characters: []\n---\n",
			check: func(t *testing.T, f *File) {
				if b := f.Items[0].(*Scene).Body; len(b) != 0 {
					t.Errorf("body = %v, want empty", b)
				}
			}},
		{name: "scene id with space", src: "---\nmodule: m\nscene: \"a b\"\n---\n",
			want: []want{{3, 11, "syntax"}}},
		{name: "literal block scalar", src: sceneFM + "when: |\n  after 1.2\n---\n",
			want: []want{{4, 7, "syntax"}}},
		{name: "folded block scalar", src: sceneFM + "when: >\n  after 1.2\n---\n",
			want: []want{{4, 7, "syntax"}}},
		{name: "multi-line plain scalar", src: sceneFM + "when: after\n  1.2\n---\n",
			want: []want{{4, 7, "syntax"}}},
		{name: "multi-line quoted scalar", src: sceneFM + "location: \"camp\n  near\"\n---\n",
			want: []want{{4, 11, "syntax"}}},
	})
}

// An end-of-line error after a string points at its closing quote.
func TestHereAfterString(t *testing.T) {
	ln, _ := lexLine(srcLine{1, `x "ab\"c"`}, func(d Diag) { t.Errorf("%v", d) })
	c := &cur{l: ln, k: len(ln.Toks)}
	if got := c.here(); got != (Pos{1, 9}) {
		t.Errorf("here() = %v, want 1:9", got)
	}
}

const bom = "\xef\xbb\xbf"
