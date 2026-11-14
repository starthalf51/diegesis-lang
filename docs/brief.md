# Episode briefs (`dieg brief`)

`dieg brief` compiles a range of scenes into one writing brief for a whole episode: what the reader already holds,
what this episode must establish, what stays hidden, the scene beats, and the code itself with unmet names masked.
A model (or a person) writes the episode from the brief in one pass. The brief is deterministic: it calls no model.

The brief text is Korean, like the generation prompts. Everything it says comes from the work's code and the
attribute conventions below; a work that uses none of them still gets a brief (facts, beats, spoken lines, code).

## Commands

```
dieg brief <work|dir> --from <scene> [--to <scene>] [--since <scene>] [--code <file>] [--style <file>] [--rules <dir>] [--settled <dir>]
dieg settle <work|dir> --from <scene> --text <manuscript> -o <dir> [--model M]
```

- `--from`/`--to`: the episode is the scenes told from `--from` up to, not including, `--to` (to the end without it).
- `--since`: quote the lines spoken since that scene, word for word, with who heard them.
- `--code`: the episode's source file, appended with author comments stripped and unmet names masked.
- `--style`: a style sheet appended as is.
- `--rules`: the work's project rules; their numeric `data` values become counters (see `frame_open` below).
- `--settled`: a directory of `dieg settle` output. Files named `<scene id>.txt` of scenes told before `--from`
  are listed, in telling order, as details the episode must not change. Other file names are skipped with a warning.

`dieg settle` reads one written episode and asks a model (default `sonnet`, through the `claude` CLI) for the
concrete details it fixed that later episodes must not contradict: object states, colors, positions, who is where,
what is locked or broken, numbers and times, names and nicknames used, wounds, promises and threats, and names the
manuscript invented. It writes at most 25 lines, one per line, to `<dir>/<from>.txt`. A reply with no list lines
writes an empty file. This closes the loop between written prose and the next brief: invented details stop drifting.

## Sections

| Section | From |
|---|---|
| ⓪ now | story day of the first scene (`when`), a warning when the previous scene may be the same day |
| ⓪ world rules | `attr <term>.rule` |
| ⓪ clocks and counters | `attr <term>.clock`, project-rule counters, `frame_open`/`frame_close` lines |
| ① what the reader holds | reader state before `--from`; forged records are marked |
| ① cards, places | character and place attributes below; each off-POV character's knowledge and how they got it |
| ① settled details | `--settled` |
| ① heard lines | `--since` |
| ② what this episode establishes | terms first explained here, each with the event that shows it |
| ② handed over | facts the reader learns in the range |
| ③ not revealed | facts still hidden after the range (beliefs, rules and creeds excluded) |
| ④ writing rules | fixed rules, plus one closing rule (see `frame_policy`) |
| ⑤ beats, lines, code | `design` entries, spoken lines with their audience, the code |

## Attribute conventions

Each is an ordinary `attr`. None is required; each one the work sets makes the brief say more. The principle is the
same for every genre: the brief carries what the code states and never invents it.

Terms (`term`):

- `rule` — a world rule the writer must not break, acted out rather than explained. `attr permit.rule = "no one passes without one"`
- `clock` — a deadline event; the brief counts story days to it. `attr permit.clock = expiry`
- `frame_open` / `frame_close` — a series device line written at the start/end of every episode, with `{key}` slots
  filled from clocks and counters. `attr log.frame_close = "[log] {days} days left"`
- `frame_policy` — `every` (default; also any unknown value): close every episode with a frame line, the same values
  if nothing moved; `on_change`: close with a frame line only when a value changed.
- `shown_by` — a fact whose event shows what the term means. `attr permit.shown_by = turned_away`
- `proper` — a proper name that may be used once. `attr city.proper = "Harrow"`

Characters:

- `role`, `want`, `target` (whom the want aims at), `look` (how others see them), `voice`, `unaware` (what they do not
  know about themselves), `creed` (a belief they act on but never recite), `known_as` (the name the narrator uses
  before learning the real one). `attr ann.role = "gatekeeper"`

Places:

- `look` — sense words, two per scene are dealt out; a revisited place gets only what changed. `talk` — what bystanders say there.

## Design keys

A scene's `design` block (see [language](language.md)) becomes its beat: `goal`, `turn`, `tempo`, `clock` (the
scene's own deadline), `risk` (what the character stands to lose).

<!-- check -->
```sl
work demo "Demo"
character ann "Ann"
place gate "Gate"
term permit "permit" = "a paper that lets one pass the gate"
fact shut "the gate is shut" true
attr permit.shown_by = shut
attr ann.role = "gatekeeper"
attr gate.look = "rusted bars · wet stone"
scene 1 at gate
  enter ann
  render
    explain permit
  design
    goal ann: "keep the gate shut"
    clock "before the bell"
    risk "her post"
```

## Data-gap lint

Three `info` rules report what a brief would be missing. Each fires only in a work that already uses its convention,
so works without cards are not affected:

- `brief-term-unshown` — an explained term has no `shown_by`.
- `brief-card-missing` — a character on stage or speaking into the scene has none of role, want, look or voice.
- `brief-place-unseen` — a scene's place has no `look`.
