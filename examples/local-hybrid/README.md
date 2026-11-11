# Local draft, Claude finish: one episode end to end

A local model (Ollama `gemma4:26b` on an M1 Pro MacBook) writes the Diegesis code; `dieg check`/`lint` feed its mistakes back; when it runs out of rounds, Claude haiku finishes from the best round; haiku renders the prose and each scene is verified. Run on 2026-11-11 (v0.10.0 development build).

```sh
OLLAMA_HOST=<mac>:11434 DIEG_CALL_CAP=40 dieg write \
  "회귀한 등대지기가 십 년 전 자신에게 온 편지 한 통을 숨긴다. 그 편지를 읽으면 마을의 비극이 시작된다" \
  -o out/m12-demo --code-model ollama:gemma4:26b --finish-model claude-haiku-5-5 --allow-warn
```

("A lighthouse keeper who has gone back in time hides a letter sent to him ten years ago. Reading it starts the village's tragedy.")

Files: [`story/`](story) (the final code), [`novel.md`](novel.md) (the rendered episode, Korean), [`calls.jsonl`](calls.jsonl) (every model call).

## Rounds

| Round | Model | Errors after the round |
|---|---|---|
| 1 | gemma4 (local) | 28 |
| 2 | gemma4 | 14 |
| 3 | gemma4 | 4 |
| 4 | gemma4 | 4 |
| 5 | haiku (finish, from the best round) | 3 |
| 6 | haiku | **0** |

Then 9 scenes rendered by haiku, all 9 passed the scene checklist on the first attempt.

## Calls

| Role | Model | Calls | Output tokens | Time |
|---|---|---|---|---|
| Code, fix | gemma4 (local) | 4 | 30,388 | 21 min |
| Finish | haiku | 2 | 58,480 | 4 min |
| Render | haiku | 9 | 20,356 | 3 min |
| Verify (extract) | sonnet | 9 | 4,476 | 1.5 min |

Wall time 30 minutes.

## What this does and does not show

- **Shown:** with only compiler feedback a weak local model cut the errors from 28 to 4, and the pipeline finishes a verified episode with no hand edits.
- **Not shown: a token saving on coding.** On a simpler idea, haiku alone was clean in 2 rounds (54,518 output tokens, 4 minutes). Here the haiku finish used 58,480 output tokens, because each round rewrites every file. The local draft did not shrink haiku's work. A patch-style finish (only the files with errors) is the obvious next step; until then, the untested hypothesis is that the local model pays off in bulk work run overnight (e.g. `dieg convert` batches), where it carries most rounds and Claude only finishes.
- One run, one idea. Prose quality was not judged.
