> Public snapshot (2026-11-09). The design notes are the reference and are not published.

# Language — concept model and grammar

> This changes during 0.x. The single source of truth for the grammar is [`spec/grammar.ebnf`](../spec/grammar.ebnf); this document explains the concepts and how to write them.

## Grammar principles

- **One statement per line, blocks by indentation (2 spaces).** Line-based diffs stay clean, and a wrong position in LLM output can be pointed at by line number.
- **Meaning by ID, sentences in quotes.** Strings inside quotes are not checked. They are only a reference for the renderer and for people.
- **Source quotations are referenced by line number, not copied** (`src 1.3.48`). Corpus files stay small, and round-trip comparison can line up with the original text.
- Keywords are lowercase English. Identifiers may use Unicode letters (`曹操`, `조조`).
- Comments start with `#`; check-waiver directives start with `#!`.
- The grammar in this document is the language written inside the ```` ```sl ```` blocks of a `.dieg.md` file. Front matter is sugar for this language → [file-format](file-format.md)

## Concept model

| Concept | Declaration | Notes |
|---|---|---|
| Work | `work` | Has a time unit (`unit`) and an origin (`start`) |
| Module | `module a.b`, `import a.b` | One file is one module. Split by chapter or volume |
| Entity | `character` `group` `place` `thing` `faction` | Has a unique ID and an importance `major` / `minor` / `extra` |
| Role | `role` | **An alias with time spans.** A title whose holder changes, such as "Thane of Cawdor" or "Chancellor" |
| Proposition | `fact` | A statement about the world. Its truth is fixed, or defined by a state predicate |
| Attribute | `attr entity.key = value` | A number, string or ID. Tied to a point in time, it changes over time |
| Event / scene | `scene`, `event` | A scene has a place and an attendance list. An `event` happens offstage |
| Speech / perception | `tells` `says` `lies` `aside` `sees` `reads` … | Recipients are computed from the attendance list |
| Document | `artifact` | A letter, a forged letter. Carries propositions to whoever reads it |
| Narrative thread | `thread` | A strand that runs in parallel |
| Foreshadowing | `setup`, `payoff` | Kinds: `promise` (must be paid off) / `hint` / `symbol` |
| Pattern declaration | `pattern` | The author declares "this narrative pattern is present" |
| Reader | `reveal`, `withhold`, `discourse` | Telling order and the schedule of information disclosure |
| Render directive | `render` block | Viewpoint, pace, tension, what to describe, omissions |
| Record / perspective | `account` | Records that write the same event differently (official history vs. the romance, the Gospels) |

### Declarations and the work

In `sl` blocks, Korean strings such as `"작은 이야기"` and `"안나"` are story data and are left as written (the work title "A small story", the name "Anna").

<!-- check -->
```sl
module demo.cast

work demo "작은 이야기"
  unit day
  name ko "작은 이야기"

character anna "Anna" major
  name ko "안나"
character ben "Ben" minor
  name ko "벤"
place hall "the hall"
fact door_locked "The cellar door is locked" true
fact key_hidden "The key is hidden in the hall"
```

### Time expressions

A point in time is referred to with `@scene`, `@scene.beat`, `@scene.end`, `start`. Scenes and events are placed with `when` and given a length with `lasts`.

<!-- check -->
```sl
module demo.time

work demo
  unit day
place hall

scene 1.1 at hall
  when after start
  lasts 1h..2h
scene 1.2 at hall
  when after 1.1 by 0d..1d      # between 0 and 1 day after 1.1 ends
scene 1.3 at hall
  when before 1.4               # only the order is known
scene 1.4 at hall
  when at 208-11                # absolute time (precision is a month)
scene 1.5 at hall
  when during 1.4               # inside 1.4
```

The compiler turns these expressions into STN constraints → [compiler](compiler.md#time-model)

### Proposition expressions

```
fact cawdor_traitor "Cawdor sided with Norway" true      # fixed truth
holds(thane_of_cawdor, macbeth)                           # state predicate: role holding
alive(duncan)  at(macbeth, inverness)                     # built-in predicates
not P
fact king_hereafter = holds(king, macbeth)                # prophecy: false now, true later
```

The truth of a proposition defined by a state predicate is computed **at each point in time**. A prophecy is not a separate concept; it is "a speech act that asserts a proposition which becomes true at a future point".

### Scenes, beats, acts

<!-- check -->
```sl
module demo.scene

work demo
  unit day
character anna "Anna" major
character ben "Ben" minor
place hall
fact door_locked "The cellar door is locked" true
fact key_hidden "The key is hidden in the hall"

scene 2.1 at hall
  when after start
  enter anna, ben
  anna tells ben: key_hidden "The key is under the lamp." src 2.1
  beat quarrel:
    ben asks anna: door_locked "Why is the door locked?"
    anna lies ben: not door_locked "It is not." src 2.2
  ben doubts key_hidden
  exit all
```

The shape of an act is `actor verb [target] [: proposition] ["quotation"] [because proposition] [src line-number]`. A role may stand in the actor position: `role:chancellor tells …`. The compiler resolves it to the holder at that point in time, and warns that it is ambiguous if the holder might change at that point.

The verbs are `tells` `says` `hails` `asks` `lies` `whispers` `aside` `thinks` `sees` `shows` `reads` `writes` `sends` `orders` `decrees` `does` `enacts`. `enacts` is a performance or re-enactment (a play within the play): everyone on stage sees it, but it is not an assertion, so it creates no beliefs.

### Roles and documents

<!-- check -->
```sl
module demo.role

work demo
  unit day
character king "King" major
character heir "Heir" minor
character spy "Spy" minor
place court
role crown "the Crown"
  held king until 3.1
  held heir from 3.1
artifact letter "a forged letter"
  "Your son is dead."
fact son_dead "The son is dead" false

scene 3.1 at court
  when after start
  enter spy, king
  spy writes letter
  king reads letter
```

## Knowledge and belief

- The only thing stored is `believes(who, proposition, since when, causing event)`.
- **Knows** = believes ∧ true in the world at that point. It is derived and computed.
- **Deception** = a `lies` speech act (the speaker asserts P while believing ¬P), or creating an `artifact` that carries a false proposition.
- A recipient's belief changes as told, by default. `doubts` marks non-acceptance.
- Causation where a belief changes behavior is written `does X because P`. The linter checks that the actor really believed P at that point.
- An entity with a separate source of information is declared: `knows oracle` (supernatural), `knows omniscient` (narrator). With this declaration the information-flow check is waived, but the source is kept on record.
- A character's background knowledge (what they know from before the play) is written as `believes P` inside the declaration.

## Reader

- By default, scenes are told in the order they are written in the file. To change that order, override it with `discourse after <scene>`.
- Speech and perception that happen in a shown scene are disclosed to the reader at that position in the telling.
- `aside` and `thinks` are received by **the reader only**.
- To depart from the default disclosure rule, use `withhold P until <scene>` and `reveal P at <beat>`.
- Dramatic irony is a span where the reader knows and the character does not. A mystery is a span where something is true but the reader does not yet know. Pattern queries output these spans as a **report** (not as a warning).

### Terms

A work-specific word is declared once with its surface forms and a gloss: `term <id> "<surface>" ["<surface2>"…] = "<gloss>"`. A shown scene explains it to the reader with `explain <id>[, <id>…]` in its `render` block. If a surface reaches the reader (fact text carried by an act, a quote, a `does` action, or `render focus`) as a whole word — Korean particles after it are allowed, so `"칩"` does not match `칩거` — before any scene at or before that point in the telling explains it, lint reports the error `reader-unknown-term`. The reader stack accumulates over the telling: a term is new only where it is first explained. A later `explain` of the same term draws the warning `term-re-explained`; to remind the reader briefly, use `recap <id>[, <id>…]` in `render` (the renderer writes one short phrase, never a definition). `explain <id>[, <id>…] via action|dialogue|consequence|sense` says how to weave the listed terms into the scene (any other value is `arg-kind`); without `via` the renderer chooses. Terms are never to be rendered as dictionary definitions: see the prose checks in [rendering.md](rendering.md).

A scene may carry a medium-neutral `design` block that shapes how it is rendered: `goal <character>: "<text>"` (what the character wants), `turn <A> -> <B>` (the value change across the scene; each side is a quoted string or a single word), `tempo scene|summary|ellipsis` (how much telling time the acts get), `clock "<text>"` (what must happen before what inside the scene — its own deadline), `risk "<text>"` (what the character stands to lose in it). Example:

```
design
  goal hamlet: "learn whether the ghost spoke truly"
  turn "doubt" -> resolve
  tempo scene
```

`dieg brief` turns these into scene beats; see [brief](brief.md). The renderer adds a `## Scene design` section only when a scene has a design entry. Lint `scene-no-turn` (warn) reports shown scenes without a `turn`, but only in a module (file) where some scene has a design block.

<!-- check -->
```sl
module demo.terms

work demo_terms
character anna "Anna"
term jokbo "족보" "족보 창" = "모든 시민의 신원과 배급을 묶은 국가 기록"
fact burned "족보가 불탔다" true

scene 1
  enter anna
  render
    explain jokbo
  anna says: burned
```

<!-- check -->
```sl
module demo.reader

work demo
  unit day
character anna "Anna" major
character ben "Ben" minor
place hall
fact door_locked "The cellar door is locked" true
fact key_hidden "The key is hidden in the hall"

scene 4.1 at hall
  when after start
  enter anna, ben
  withhold door_locked until 4.2
  anna thinks: key_hidden "He must never learn where it is."
  setup lamp promise: key_hidden
  exit all
scene 4.2 at hall
  when after 4.1 by 0d..1d
  enter ben
  reveal door_locked
  payoff demo.reader/lamp
  pattern mystery
  render
    pov ben
    pace slow
    tension 0.8
  exit all
```

## Grammar summary

The full grammar is in [`spec/grammar.ebnf`](../spec/grammar.ebnf). Only the skeleton is copied here.

```ebnf
file        = header { top } ;
header      = "module" qname NL { "import" qname NL } ;
top         = work | decl | term | scene | event | account | setup | distance | parent | life
            | belief | attribute | pattern | pragma ;
decl        = kind ident [string {"," string}] [tier] ["=" prop] [bool] ["tag" ident] [BLOCK(attr)] ;
term        = "term" ident string {[","] string} "=" string [BLOCK(attr)] ;
kind        = "character"|"group"|"place"|"thing"|"faction"|"role"|"fact"|"artifact"|"thread" ;
scene       = "scene" sid ["at" ident] ["in" ident] [BLOCK(stmt)] ;
event       = "event" ident [string] [BLOCK(stmt)] ;              (* offstage event *)
account     = "account" ident "of" qname "by" ident [BLOCK(stmt)] ;
stmt        = when | lasts | presence | beat | act | belief | reader | render | setup | payoff
            | discourse | parent | life | attribute | pattern | pragma ;
when        = "when" ( ("after"|"before") tref ["by" dur ".." dur] | "at" date [ident] | "during" tref ) ;
presence    = "enter" idlist | "exit" "all" | "exit" idlist | "mention" idlist ;
beat        = "beat" ident ":" [BLOCK(stmt)] ;
act         = actor verb [ident] [idlist] ["to" idlist] ["with" proplist]
              [":" [prop]] [string] ["because" prop] ["src" (word|string)] ;
reader      = "reveal" prop ["at" tref] | "withhold" prop "until" tref ;
setup       = "setup" ident ("promise"|"hint"|"symbol") ":" (prop|string) ;
payoff      = "payoff" qname ;
pragma      = "#!" "allow" ruleid ":" string ;                    (* attaches to the next statement *)
render      = "render" BLOCK(rkey rvalue) ;
rkey        = "pov"|"pace"|"tension"|"focus"|"omit"|"show"|"explain"|"recap"|"narration" ;   (* explain may end with: "via" ("action"|"dialogue"|"consequence"|"sense") *)
design      = "design" BLOCK(("goal" ident ":" string) | ("turn" dvalue "->" dvalue) | ("tempo" ("scene"|"summary"|"ellipsis")) | ("clock" string) | ("risk" string)) ;
```

## Deliberately left out

- Conditional branches and variables — this language is not interactive fiction (Ink). It **describes** rather than executes.
- Sentence-level prose — the renderer produces it.
- Probabilistic belief strength — if a case needs it, `believes P 0.7` will be considered.
