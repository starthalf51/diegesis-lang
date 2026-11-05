// Package syntax splits .sl.md files, desugars frontmatter and parses the
// story-lang language (spec/grammar.ebnf) into an AST.
package syntax

import (
	"encoding/json"
	"fmt"
	"reflect"
)

// Diag is one diagnostic. Rule ids: syntax, indent, dup-decl, yaml-anchor,
// unknown-key, unterminated-fence. sema adds undefined-ref, import-cycle,
// arg-kind, waiver-reason, waiver-target, role-unknown (errors) and
// offstage-speaker, offstage-exit (warn, waivable: Waived keeps the diag).
type Diag struct {
	File     string `json:"file"`
	Line     int    `json:"line"`
	Col      int    `json:"col"`
	Severity string `json:"severity"`
	Rule     string `json:"rule"`
	Msg      string `json:"msg"`
	Waived   bool   `json:"waived"`
}

// String is the CLI form: file:line:col: severity rule: msg.
func (d Diag) String() string {
	s := fmt.Sprintf("%s:%d:%d: %s %s: %s", d.File, d.Line, d.Col, d.Severity, d.Rule, d.Msg)
	if d.Waived {
		s += " (waived)"
	}
	return s
}

// Pos is a 1-based source position (column counted in runes).
type Pos struct{ Line, Col int }

// Node is any AST node. Items, bodies and attribute lists hold Nodes.
type Node interface{ pos() Pos }

func (p Pos) pos() Pos { return p }

// Ident is a name with its position (references and declarations).
type Ident struct {
	Pos
	Name string
}

type File struct {
	Pos
	Path     string
	Module   *Module
	Imports  []*Import
	Items    []Node // Work, Decl, Scene, Event, Account, Setup, Distance, Parent, Life, Pragma
	Prose    []Node // Prose, BeatAnchor in source order
	Status   string // frontmatter metadata only
	Mode     string
	Rendered map[string]string
}

type Module struct {
	Pos
	Name string
}

type Import struct {
	Pos
	Name string
}

type Work struct {
	Pos
	ID    Ident
	Title string
	Attrs []Node // Unit, Start, Name, Pragma
}

type Decl struct {
	Pos
	Kind  string // character group place thing faction role fact artifact thread
	ID    Ident
	Names []string
	Tier  string
	Def   *Prop  // fact x = prop
	Value string // "true" | "false" | ""
	Attrs []Node // Name Members Knows Held Shared Contains Text Pragma
}

type Unit struct {
	Pos
	Unit string
}

type Start struct {
	Pos
	Date string
}

type Name struct {
	Pos
	Lang, Text string
}

type Members struct {
	Pos
	IDs []Ident
}

type Knows struct {
	Pos
	Source string // oracle | omniscient
}

type Held struct {
	Pos
	Holder      Ident
	Role        bool // holder written as role:<id>
	From, Until *Tref
}

type Shared struct{ Pos }

type Contains struct {
	Pos
	Props []*Prop
}

type Text struct {
	Pos
	Text string
}

type Distance struct {
	Pos
	A, B     Ident
	Min, Max Dur
}

type Parent struct {
	Pos
	Parent   Ident
	Children []Ident
}

// Life is `born x when …` / `dies x when …`.
type Life struct {
	Pos
	Kind string // born | dies
	Who  Ident
	When *When
}

type Scene struct {
	Pos
	ID   Ident
	At   *Ident
	In   *Ident
	Body []Node
}

type Event struct {
	Pos
	ID    Ident
	Title string
	Body  []Node
}

type Account struct {
	Pos
	ID, Of, By Ident
	Body       []Node
}

type When struct {
	Pos
	Op       string // after before at during
	Ref      *Tref
	Min, Max *Dur
	Date     string
	Time     string
}

type Presence struct {
	Pos
	Op  string // enter exit mention
	All bool   // exit all
	IDs []Ident
}

type Beat struct {
	Pos
	ID   Ident
	Body []Node
}

type Act struct {
	Pos
	Actor   Ident
	Role    bool // actor written as role:<id>
	Verb    string
	Action  string // does <action>
	Objects []Ident
	To      []Ident
	With    []*Prop
	Prop    *Prop
	Quote   string
	Because *Prop
	Src     string
}

type Belief struct {
	Pos
	Who     Ident
	Verb    string // believes | doubts
	Prop    *Prop
	Because *Tref
}

type Reveal struct {
	Pos
	Prop *Prop
	At   *Tref
}

type Withhold struct {
	Pos
	Prop  *Prop
	Until Tref
}

type Setup struct {
	Pos
	ID   Ident
	Kind string // promise hint symbol
	Prop *Prop
	Text string
}

type Payoff struct {
	Pos
	Ref Ident
}

type Discourse struct {
	Pos
	After Tref
}

type Pragma struct {
	Pos
	Rule, Reason string
}

type Render struct {
	Pos
	Entries []*RenderEntry
}

type RenderEntry struct {
	Pos
	Key    string
	Values []string
}

// Prop is a proposition: a name (fact ref / ident argument), a predicate
// call name(args…), optionally negated (Not).
type Prop struct {
	Pos
	Not  bool
	Name string
	Call bool
	Args []*Prop
}

// Tref is a time reference: [@]path, e.g. @1.3.ross_news, 1.2, start.
type Tref struct {
	Pos
	At   bool
	Path string
}

type Dur struct {
	Pos
	N    int64
	Unit string // y d h m
	Sec  int64  // N in seconds, 1y = 365d
}

type Prose struct {
	Pos
	Text string
}

type BeatAnchor struct {
	Pos
	Beat string
}

// Dump renders an AST as stable, indented JSON with a "_" type tag on every
// node. Empty strings, false, nil and empty slices are omitted. With
// withPos=false all positions are dropped (for sugar equivalence).
func Dump(v any, withPos bool) []byte {
	b, err := json.MarshalIndent(tree(reflect.ValueOf(v), withPos), "", "  ")
	if err != nil {
		panic(err)
	}
	return append(b, '\n')
}

var posType = reflect.TypeOf(Pos{})
var identType = reflect.TypeOf(Ident{})

func tree(v reflect.Value, withPos bool) any {
	switch v.Kind() {
	case reflect.Interface, reflect.Pointer:
		if v.IsNil() {
			return nil
		}
		return tree(v.Elem(), withPos)
	case reflect.Slice:
		out := make([]any, v.Len())
		for i := range out {
			out[i] = tree(v.Index(i), withPos)
		}
		return out
	case reflect.Map:
		out := map[string]any{}
		for _, k := range v.MapKeys() {
			out[k.String()] = tree(v.MapIndex(k), withPos)
		}
		return out
	case reflect.Struct:
		if v.Type() == identType {
			id := v.Interface().(Ident)
			if withPos {
				return fmt.Sprintf("%s@%d:%d", id.Name, id.Line, id.Col)
			}
			return id.Name
		}
		out := map[string]any{"_": v.Type().Name()}
		for i := 0; i < v.NumField(); i++ {
			f, fv := v.Type().Field(i), v.Field(i)
			if f.Type == posType {
				if withPos {
					p := fv.Interface().(Pos)
					out["pos"] = fmt.Sprintf("%d:%d", p.Line, p.Col)
				}
				continue
			}
			if fv.IsZero() || (fv.Kind() == reflect.Slice || fv.Kind() == reflect.Map) && fv.Len() == 0 {
				if fv.Kind() != reflect.Int64 && fv.Kind() != reflect.Int {
					continue
				}
			}
			out[f.Name] = tree(fv, withPos)
		}
		return out
	default:
		return v.Interface()
	}
}
