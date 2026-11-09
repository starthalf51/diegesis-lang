> Public snapshot (2026-11-09). The design notes are the reference and are not published.

# Renderer

> Takes the IR and render directives and produces prose. The renderer's job is to write sentences **without inventing new facts and without leaking what must still be hidden from the reader**. Both can be verified by machine. Sentence quality is judged by a person (automatic evaluation is not a goal). → [compiler](compiler.md)

## Flow

1. `render_scene(work, scene)` (PG) builds the scene's **render request** as JSON → [storage](storage.md). The request holds the scene ID, place, the summary of what came before (`previously`), characters, attendance, facts, acts per beat, render directives (`pov` `pace` `tension` `focus` `omit`), the viewpoint character's beliefs, what the reader knows, and what must not be leaked in this scene (`withheld`).
2. The **world-state digest** (`internal/pg/digest.go`) summarizes the world state at that scene's point in time (who believes what, where, etc.) in Korean and puts it in the render prompt. It is on by default and turned off with `--no-digest`. It is a SELECT that only reads existing tables and functions.
3. A style backend (`styles/<id>.json`) supplies the language, the target register and public-domain style samples.
4. An LLM writes the prose.
5. With `--verify`, it goes through verification (below).

The layered renderer that once existed has been removed. Now there is only the single path above.

## Verification (`dieg render --verify`)

The default verifier is the **checklist card** (`--verifier card`).

- It builds a list of expected facts from the request, and in one LLM call receives JSON saying whether the prose covers each item (yes/no), whether subject and verb are right, and whether a fact to be hidden was revealed.
- The deterministic axes (register ratio, length) are counted without an LLM.
- Pass criteria: 0 leaks, and omissions + wrong kinds ≤ 1. There is a second stage that re-checks only the items reported as "absent" (the structure can only rescue).
- Whether it invented an event that is not in the IR (G2) cannot be trusted with either method, so it was taken off the gate and is reported **as a warning only**.
- On failure, feedback (missing facts, wrong subjects, leaks) is appended to the end of the prompt and it renders again, at most `1 + --retries` times. If it still does not pass at the limit, the prose is written anyway but the exit code is 3.

`--verifier open` is the earlier method. A separate LLM call re-extracts the IR from the prose and diffs it against the original scene (a missing speech act = plot loss, a new fact = hallucination, a `withheld` proposition appearing = leak).

## Revising (`dieg revise`)

When the author edits the code (IR), only scenes whose render request changed and new scenes are re-rendered.

- A changed scene is **patched sentence by sentence**. Given the old prose + the new scene + the fact differences, it receives only an edit list (JSON) of "old sentence → new sentence" and applies it deterministically. If it cannot be applied or does not get past the card verification, it tries once more, and if that still fails it falls back to a full render. Patching is used only when the card verifier is on.
- Staleness is decided by comparing the render-request hash and the world-state digest hash recorded in the scene's `.hash` file with the current values. A scene whose request is the same but whose state digest changed is reported as `state_stale`. `dieg stamp` fills in the state hash in older hash files (which had only the request hash). Neither of these makes a model call.
- The prose of a deleted scene is deleted.

## Writing pipeline

`dieg write` starts from an idea, has an AI write Diegesis code → verifies it with `dieg check` / lint → feeds errors back to fix it (`--rounds`) → renders. `dieg continue` handles the next episode, and `dieg revise` handles a change request. The model is chosen separately per stage (`--code-model` `--render-model` `--extract-model`).

## Style-backend gate

- A backend is `styles/<id>.json`: `id`, `lang`, `tone` (`haera` | `hapsho` | `gojeon` | none), and style samples (`author`, `work`, `source`, `text`, `died_year`, `published_year`). The Korean register ids name the narration styles: `haera` is plain-form, `hapsho` is polite-formal, `gojeon` is classical; the backend ids (e.g. `ko-haera`) keep these names.
- The loader checks the copyright expiry of each sample. Only samples whose author died more than 70 years ago (Korea) and which were published more than 95 years ago (US) pass. The stricter of the two applies. If even one file fails the check, loading is an error.
- Naming a living author in the prompt to direct the style is also forbidden. Only the sample text is the basis of the style.
- Korean translations are not used as style samples. Korean output is **generated** from the IR, not translated, so Korean style samples are taken only from the original texts of Korean authors.
- The Korean register rules apply only to narration outside quotation marks. Dialogue (`quote`) is given in the original language and carried over without changing its meaning. The register judgment (`ClassifyTone`) is deterministic.

## Cost

LLM calls go through the `claude` CLI (`internal/render/claudecli.go`). `dieg write` / `dieg revise` report the call count and tokens in one line at the end.
