# Idea → novel: writing with Diegesis vs the AI writing directly

We turned a one-line idea into a novel of 9 scenes by two methods. Both use the same model, `claude -p` (subscription CLI, claude-opus-5-5).

- **A (direct)**: the AI writes an outline, then writes the scenes one at a time. For each scene it sees only the outline and the previous scene.
- **B (Diegesis)**: the AI writes the story as `.dieg.md` code → the `dieg check` and `dieg lint` diagnostics are returned to the AI → the AI fixes it
  → `dieg render` writes the prose for each scene.

Both methods got the same idea sentence and the same requirements (`prompts/common-requirements.txt`). All B has more is the language and tools.

## Results

| Axis | A direct | B Diegesis |
|---|---|---|
| Continuity errors (blind judgment, 3-vote majority → verified by me) | **7** (3.8 per 10,000 chars) | **2** (1.7 per 10,000 chars). Both are sentences invented by the renderer; 0 came from the code |
| Planned structure (6 items) | 6/6 present, not guaranteed | 6/6 present. Foreshadowing, payoff, lies, and reveals exist as code lines, and lint checks payoffs and ordering |
| Change request "change the culprit": cost | 1 outline + all 9 scenes rewritten (10 calls, 681 s). 476 lines differ from the original | 1 code edit + 1 diagnostic fix + 2 scenes re-rendered (4 calls, plus 1 call whose output was lost). Code +70/−28 lines |
| Change request: downstream conflict caught by the compiler | Not applicable (no means of checking) | **1** (`warn info-flow`: with the changed truth, the homeroom teacher cannot say "I pushed her") |
| Errors remaining after the change (1 judgment → verified) | **2** (both were there before the change) | **3** (1 the renderer produced even before the change, 1 newly produced on re-render, 1 location error in the code) |
| Style swap | Must be rewritten | Render the same scene code with only the backend changed (ko-haera → ko-hapsho, 1 call) |
| Cost (original) | 10 calls, 706 s | 12 calls, 1085 s (code writing 530 s + 2 fixes 213 s + 9 renders 342 s) |
| Judging | 4 calls, 619 s (shared by both arms) | |

There were 42 calls in total. Every call was logged as one line in `calls.tsv` (arm, stage, prompt, output, seconds).

**Honest summary.** On the first manuscript, B was clearly ahead. A's 7 errors are contradictions within the story itself (hairpin shape, time arithmetic, who saw what).
B's 2 are sentences the renderer added that are not in the code. On the change request, however, B did not win. A rewrote all the scenes
and built a mostly consistent new truth (the caretaker Oh Man-seok is the culprit). The judge found 2 remaining errors for A, fewer than B's 3.
B needed far fewer calls and changed far fewer lines, but the compiler caught only 1 conflict. Of B's remaining errors, two came from the renderer and one from a place the current rules do not look.

## The idea

`idea.txt`:

> 폭설로 길이 끊긴 산골 분교에서 교장 선생님이 숨진 채 발견된다. 열한 살 서진은 "그 시각 나와 함께 있었다"는 담임 선생님의 말을 믿었지만, 그날 울리지 않은 종이 그 거짓말을 드러낸다.
>
> (In a mountain branch school cut off by heavy snow, the principal is found dead. Eleven-year-old Seo-jin believes the homeroom teacher's words, "She was with me at that time," but the bell that did not ring that day exposes the lie.)

This idea contains a false alibi, a character who believes the lie, times, a bell that rang every day (foreshadowing), and a culprit reveal.
The two arms made different stories from the same idea.

- A: The homeroom teacher Yun Jae-hui sneaked into the staff house to look for forged documents. The culprit is the principal's son Kang Tae-o, and the weapon is the bell's clapper.
- B: The culprit is the homeroom teacher Jeong Hye-won. The evidence is a red mitten buried under the bell tower, and the pocket watch stopped at 2:52.

## The same scene: the false alibi

A s4 (`a-direct/scenes/s4.md`):

> "저는 서진이랑 교실에 있었어요. 세 시부터 다섯 시까지요."
>
> "서진이가 감기 기운이 있어서 약 먹이고 난로 옆에 재웠어요. 네 시 종이 울려서 깨웠고, 받아쓰기 세 개 하고 다시 재웠어요. 화장실에 한 번 잠깐 다녀온 것 말고는 계속 같이 있었어요."
>
> ("I was in the classroom with Seo-jin. From three to five." / "Seo-jin had a bit of a cold, so I gave her medicine and put her to sleep by the stove. The four o'clock bell rang and I woke her, did three dictations, and put her back to sleep. Apart from one quick trip to the bathroom, I was with her the whole time.")

B s4 (`b-diegesis/novel/s4.ko-haera.md`):

> “선생님, 세 시 종 쳤어요? 깨워 준다고 했잖아요.”
>
> “벌써 쳤지. 종 울릴 때 네가 뒤척이길래, 너무 곤히 자서 안 깨웠어.”
>
> 선생님은 웃는 낯으로 그렇게 말하였다. 그날 오후 세 시에 분교의 종은 울렸고, 서진은 그것을 자면서 지나 보낸 셈이었다. (…)
>
> “선생님은 내내 여기 네 옆에 있었어.”
>
> ("Teacher, did the three o'clock bell ring? You said you'd wake me." / "It rang already. You stirred when it rang, but you were sleeping so soundly that I didn't wake you." The teacher said so with a smiling face. At three that afternoon the branch school's bell did ring, and Seo-jin had slept through it. (…) "I was right here beside you the whole time.")

B's code for this scene (`b-diegesis/story/scenes/s4.dieg.md`):

```
beat alibi:
  teacher says seojin: bell_rang "벌써 쳤지. 종 울릴 때 네가 뒤척이길래, 너무 곤히 자서 안 깨웠어."
  teacher lies seojin: teacher_alibi "선생님은 내내 여기 네 옆에 있었어."
seojin believes teacher_alibi because @s4.alibi
seojin believes bell_rang because @s4.alibi
```

`lies` (a lie) and `says` (a statement made in belief) are distinguished. The teacher believes the caretaker rang the bell in her place (`teacher believes bell_rang`, offstage event three_oclock).
So the bell talk is a mistake, not a lie. Seo-jin comes to believe both, and in s7, hearing Ha-ram's words (`bell_talk`), turns to doubt.
The engine tracks this per viewpoint.

## A's errors, and how B blocked the same kinds

The full list and quotations are in `judge/verification-1.md`.

1. **Who saw what (A6, knowledge).** In A s5, Jae-hui sees from inside the storeroom only up to "태오는 손을 오래 씻었다. (…) 그러고는 다시 보따리를 싸서 뒷문으로 나갔다." ("Tae-o washed his hands for a long time. (…) Then he packed the bundle again and went out the back door.").
   Yet in s9 she testifies, "태오 씨가 창고 옆 눈사람 머리를 파고 그 안에 뭘 넣었습니다." ("Mr. Tae-o dug into the snowman's head beside the storeroom and put something in it.").
   In B, such testimony is written as an act (`tells`). If the speaker has not seen or heard that fact earlier, the `info-flow` rule warns.
   In fact, this rule caught the same kind during the change request (below).
2. **Attribute (A2, attribute).** The decisive evidence, the hairpin, is in s2 "나비 날개 한쪽에 파란 큐빅이 박혀 있었는데, 다른 쪽 큐빅은 빠지고 없었다." ("a blue cubic stone was set in one butterfly wing, and the stone on the other side had fallen out").
   In s5 it changes to "날개 한쪽 칠이 벗겨진 나비였다." ("a butterfly with the paint worn off one wing"). In B, an object's attribute is declared once, like `attr mitten.maker = "서진과 할머니"` ("Seo-jin and her grandmother").
   If two places assert differently, `attribute-conflict` catches it. But as B1 shows, it cannot stop the renderer from inventing an undeclared attribute in the prose.
3. **Time (A3, A4, timeline).** A s5 uses both "아홉 시가 되자" ("when it became nine o'clock") and "다섯 시간 전에도" ("five hours earlier too") (actually six hours from 15:00).
   B writes the times of scenes and events as constraints, like `when after s3.end by 25m..25m` and `lasts 5m..10m`. PG's `solve_time()` finds cycles and contradictions.
   In B's first fix round, info-flow gave "order not determined" (5, info). It means the event durations (`lasts`) were missing, so the order of knowledge was undetermined.
   When the AI wrote in the durations, the diagnostics disappeared (`b-diegesis/rounds/r1.lint.txt` → `r2`).

B's code-writing log: the first code (r0) had 16 compile errors (enter/exit inside an offstage event, the target of `shows` being an object).
After the first fix there were 0 errors and warnings and 5 info remained. After the second round, 0. Raw outputs are in `b-diegesis/rounds/r*.check.json` and `r*.lint.txt`.

## Planned structure

| What the idea requires | A | B | Evidence in B's code |
|---|---|---|---|
| A bell that rings every day (foreshadowing) | Present (s1 the principal boasting of the "4 o'clock bell") | Present | s1 `principal tells seojin, haram: bell_daily …`, `setup promise_bell promise:` → s9 `payoff bungyo.scenes/promise_bell` |
| False alibi | Present (s4) | Present | s4, s5, s6 `teacher lies …: teacher_alibi` |
| Seo-jin believes it | Present (s4 nodding) | Present | s4 `seojin believes teacher_alibi because @s4.alibi`, s6 `seojin says officer: teacher_alibi` |
| The bell that did not ring exposes the lie | Present (s6 "no memory of the dictation") | Present | s7 `seojin doubts teacher_alibi because @s7.bell_talk`, `reveal teacher_left` |
| Object foreshadowing → payoff | Present (snowman → clapper) | Present | s2 `setup promise_mitten` → s8 `payoff …/promise_mitten`; s1 `setup promise_watch` → s6 `payoff …/promise_watch` |
| Culprit reveal | Present (s9 arrest) | Present | s1 `withhold teacher_pushed until s9` → s9 `reveal teacher_pushed` |

A also has all six. The difference is the guarantee. In B, an unpaid `setup … promise` triggers an `unpaid-promise` warning,
and a payoff before its setup triggers `payoff-without-setup`. In A, you can only know by having a person read the finished manuscript.

## The change-request process

The request (`prompts/change.txt`, shared by both arms): "Change the culprit … Keep the teacher's lie and the bell that did not ring."

**A**: revised the outline (the new culprit is the caretaker Oh Man-seok). Then rewrote the 9 scenes one by one. Each time it was given the revised outline, the original scene, and the scene it had just revised.
The prose changed by 476 lines. The errors left in the judgment are 2 that were there before the change (coat color, "five hours earlier").

**B**: told the AI "minimally edit only the lines that decide the truth". The AI edited two files, `cast.dieg.md` and `scenes/offstage.dieg.md` (+28/−10 lines).
The culprit became the kitchen aide Choe Mal-sun, and the homeroom teacher became someone who merely found the body. The compile and lint results, verbatim (`b-diegesis/change/c1.check.json`, `c1.lint.txt`):

```
[]
```
```
up to date
loaded bungyo
b-diegesis/change/c1/scenes/s9.dieg.md:33:3: warn info-flow: teacher says teacher_pushed without having learned it before
```

The confession in s9, `teacher tells officer: teacher_pushed "서류를 달라고 매달렸어요. 계단에서… 제가 밀었어요."` ("I begged her for the papers. On the stairs… I pushed her."), does not hold under the changed truth.
This is because the event of the teacher pushing directly no longer exists. The engine pointed out a contradiction inside a scene file that was never touched.
The AI fixed it in one go after receiving this diagnostic (`c2.diff`: the teacher's truthful confession, a burned consent form, the aide's confession, a new foreshadowing `promise_form`). The re-run check and lint had 0.

The scenes to re-render were chosen by comparing render requests (`tools/scenereq`, `change/req-before|after`). The only scenes whose acts changed were s8 and s9.
In s1–s7 only the ID in the "facts that must not be stated" list changed from `teacher_pushed` to `cook_pushed`, with the same acts. So the original prose was used as is.
**Only two scenes were re-rendered.**

But B had more remaining errors (3 vs 2, `judge/verification-2.md`). The re-rendered s9 invented a recollection of "an empty chair when she opened her eyes".
That recollection clashes with s4. The compiler caught only 1. The aide's actions in earlier scenes (a glance suspecting the teacher, words steering toward the caretaker)
came to read as a 'cover-up' under the revised truth, regardless of intent. That is not an error, but the engine has not confirmed that it reads that way either.

## Style swap

The same B s4 code, only the render backend differs. `dieg render b-diegesis/story s4 --backend ko-hapsho`.

| ko-haera (`b-diegesis/novel/s4.ko-haera.md`) | ko-hapsho (`b-diegesis/styles/s4.ko-hapsho.md`) |
|---|---|
| 침대 곁 걸상에 앉아 있던 정혜원 선생님이 손을 뻗어 서진의 이마를 짚었다. “깼어? 열은 좀 내렸네.” (Teacher Jeong Hye-won, who had been sitting on the stool by the bed, reached out and touched Seo-jin's forehead. "You're awake? The fever's gone down a bit.") | 그때 침대 곁에서 정혜원 선생님이 서진의 이마에 손을 얹으며, “깼어? 열은 좀 내렸네.” 하였습니다. (Then, beside the bed, Teacher Jeong Hye-won laid her hand on Seo-jin's forehead and said, "You're awake? The fever's gone down a bit.") |
| 선생님은 웃는 낯으로 그렇게 말하였다. 그날 오후 세 시에 분교의 종은 울렸고, 서진은 그것을 자면서 지나 보낸 셈이었다. (The teacher said so with a smiling face. At three that afternoon the branch school's bell rang, and Seo-jin had slept through it.) | 하였습니다. 그날 오후 세 시에도 분교의 종은 울렸던 것입니다. (She said it. At three that afternoon, too, the branch school's bell had rung.) |

The dialogue is a quoted string in the code, so the two renders do not differ by a single character. Only the narration register changes.
We did not run `dieg render-check`. For one scene it needs 4 renders and 12 re-extractions, 16 calls in all, which does not fit within the call cap (55), so we left it out.

## Limitations

- **One idea, one run.** These are cases, not statistics. The two arms made different stories from the same idea. The error counts also mix in the difference in story complexity.
- **The lengths differ.** A was asked for 2500 characters per scene, because we could not know in advance how many characters B would produce. Excluding whitespace, A is 18,400 characters and B is 12,062.
  So per-10,000-character figures are given alongside.
- **The judge's recall.** Judgment 1 is a 3-vote majority and judgment 2 is a single run (call cap). The 5 A errors from judgment 1 did not reappear in judgment 2.
  We did not separate whether they were fixed or missed. B's code error I found separately (the s9 location) is not in judgment 1's count.
- **Renderer variation.** All of B's errors are either facts the renderer invented outside the code (B1, B2, B3) or something the current rules do not look at (B4).
  Even if the code is right, the prose can be wrong. `render-check` (re-extraction comparison) is the tool that measures this, but we did not run it this time.
- **Prose quality.** A is longer, with freer description between scenes. B transcribes acts in order, so some places are less natural.
  For example, a thought (`thinks`) in the first line of s4 was rendered as quoted dialogue. Quality was not judged.
- **Code-writing cost.** B's first code-writing call alone takes 530 s, close to A's entire manuscript (706 s). The prompt contains grammar, README, and examples, so every fix call is over 1,100 lines.
- **Same model, same system prompt** (`prompts/call.sh`). Both arms are `claude -p` with tools off. B's render calls use the renderer's own prompt (`internal/render/prompt.go`).
- One call lost its output (B change-edit, no output directory). It is noted in `calls.tsv`.

## Plan 3 candidates (gaps in the language and tools)

1. **There is no check of object location ↔ scene location.** With `attr bell.hung_at = tower`, `seojin does ring bell` passes in a scene with `location: staffroom`.
2. **`shows` cannot take an object as its target.** `principal shows watch to seojin` is an `arg-kind` error ("shows object "watch" is a thing, want character or group").
   To avoid this, the AI put narration like "the principal took the pocket watch out of his pocket and showed it" inside a dialogue quotation string. The grammar (EBNF) does not state this constraint either.
3. **`reveal` and `withhold` of a false fact are not checked.** The step-1 edit code changed `teacher_pushed` to `false` but left `reveal teacher_pushed`.
   Lint let it pass. As a result info-flow caught the confession in the same scene, but there is no rule about the line that reveals a falsehood to the reader.
4. **The means to declare story-level requirements such as "is the culprit revealed" is weak.** There is a `pattern` declaration, but no pattern fits this story (for example, the collapse of a false alibi).
5. **The renderer's addition of facts cannot be blocked at the render stage.** The render rules say "do not add facts not in the spec", yet B1, B2, and B3 occurred.
   A default path is needed that re-extracts and compares each render with `render-check` (today it only targets one scene of the corpus's `expect.yaml`).
6. **Knowledge order being undetermined (info "order not determined")** is info, not a warning. The AI fixed it this time, but it may be worth raising to a warning.

## Files

- `idea.txt`, `calls.tsv` (a record of all calls), `prompts/` (prompts, assembly scripts, `call.sh`)
  - The assembled prompts over 500 lines (grammar, examples, code, and both novels stuffed in) and the model's raw outputs (`*.raw.txt`) were not committed.
    Instead they can be rebuilt with the scripts, and their sha256 is kept in `prompts/SHA256SUMS-uncommitted.txt`. The raw outputs are included as files unpacked with `unpack.sh`.
- `a-direct/`: `outline.md`, `scenes/s1–s9.md`, `change/` (revised outline and scenes)
- `b-diegesis/`: `story/` (final code), `rounds/` (r0–r2 code and per-round diagnostics), `novel/` (ko-haera render), `styles/` (ko-hapsho),
  `change/` (c1 = step-1 edit, c2 = diagnostic fix, `*.diff`, diagnostics, render request comparison, `novel/`)
- `judge/`: the judge's raw output (`judge*-run*.json`), the X/Y assignment (`mapping.txt`), my verification (`verification-1.md`, `verification-2.md`)
- `tools/scenereq/`: a small tool that extracts and compares scene render requests (the compiler and renderer were not changed)

## Re-running

```sh
export PATH=<go-install>/bin:$PATH SL_PG_DSN='postgres://sl:sl@localhost:55432/sl?sslmode=disable'
go build -o dieg ./cmd/dieg && export DIEG=$PWD/dieg
cd examples/idea-to-novel
prompts/b-build-prompt.sh && prompts/call.sh B code-r0 prompts/b1-code.txt b-diegesis/rounds/r0.raw.txt
prompts/unpack.sh b-diegesis/rounds/r0.raw.txt b-diegesis/rounds/r0 && prompts/b-check.sh b-diegesis/rounds/r0 b-diegesis/rounds/r0
prompts/b-fix-prompt.sh b-diegesis/rounds/r0 b-diegesis/rounds/r0 prompts/b2-fix-r1.txt   # … up to r2
prompts/b-render.sh b-diegesis/story b-diegesis/novel ko-haera s1 s2 s3 s4 s5 s6 s7 s8 s9
prompts/call.sh A outline prompts/a1-outline.txt a-direct/outline.md && prompts/a-scenes.sh a-direct/outline.md a-direct/scenes a2
prompts/judge-build.sh a-direct/scenes .md b-diegesis/novel .ko-haera.md B prompts/judge1.txt
```
