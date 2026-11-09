# Showcase: one retcon, two ways

Built from the saved artifacts of [`examples/value/retcon3`](../value/retcon3/README.md) (retcon R4, v0.7.0). No model calls were made for this page. Excerpts are verbatim (Korean); translations are mine.

## The request (same text for both arms)

> 두꺼운 유리 조각의 주인은 정수아가 아니라 분교 교사 도하은이며, 하은이 오래전 바닷가에서 주워 간직해 온 것을 수아가 빌려 갖고 있다. 이 조각을 누가 주웠고 누가 갖고 있는지를 말하거나 짐작하는 모든 장면을 이에 맞게 고쳐라.

("The thick glass shard belongs to the branch-school teacher Do Ha-eun, not to Jeong Su-a; Ha-eun found it long ago and kept it, and Su-a borrowed it. Fix every scene that says or guesses who found or holds it.")

- **A**: a direct AI edit of the episode text (6 calls). **B**: `dieg revise` (39 calls).

## A: the old fact survives in episode 9

Left unedited, in the scene where Seo-yun borrows the shard (`r4/judge/a.prompt.txt`, episode e09):

> "그거 어데서 주웠는데?"
>
> "동쪽 벼랑 밑예. 물 제일 많이 빠진 날. 돌 틈에 끼어 있었어예."

("Where did you find it?" / "Under the east cliff. The lowest-tide day. It was wedged between rocks.") **<- contradiction: Su-a says she found it; the retcon says Ha-eun did.**

Later in the same episode:

> "동쪽 벼랑 밑에서 아이가 주웠어요. 오늘 낮에 등대 렌즈에 대 봤습니다. 맞았어요."

("A child found it under the east cliff. I held it to the lens today. It fit.") **<- contradiction again; the verifier notes Seo-yun learned in episodes 3-4 that Ha-eun found it, and nothing shows she is lying.**

Both were confirmed against the source text (`r4/judge/a.out.txt`, `verification.md`). A also changed 225 unrelated characters.

## B: the same facts, rewritten consistently

B's scene s24 (`r4/b/novel/s24.ko-haera.md`):

> “이 유리 조각은 하은 선생님에게서 빌려 갖고 있어요.” 수아가 고개를 들며 말했다.

("I'm borrowing this shard from Ha-eun.") And where the shard is shown to the old keeper, s27 (`r4/b/novel/s27.ko-haera.md`):

> “도하은 선생님이 등대 아래 바위틈에서 주워 간직해 온 유리 조각이에요. 이건 깨진 등대 렌즈 귀퉁이에서 떨어져 나온 거예요.”

("It's the shard Teacher Do Ha-eun found among the rocks below the lighthouse and kept. It came off the broken lens.") No contradiction was found for B in R4, and B changed 0 unrelated characters.

## Caveats

The two stories were written independently, so the scenes are parallel, not line-for-line. This is one of six retcons (one run per arm, one verifier): A left 6 confirmed contradictions and B 3 across all six, and B costs about as much only under one reading of the data and makes 6.7x the calls. See the retcon3 README for the full verdict.
