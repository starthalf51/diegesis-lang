# Judge item verification (reset judgments, sonnet once each / checked by me against the source)

The judge prompt and method are the same as the retcon spike (`rcJudgePrompt`, sonnet). The input is the full text built from M (fixed scenes + untouched scenes + B's newly created scenes as they are). All quotations were found in the source text in the automatic cross-check (R1 4/4, R3 2/2). The classification is the same three as the retcon spike: confirmed (counted) / borderline (not counted) / rejected.

## R1 (the one who broke the lens) — confirmed residual 0, judge items 2

residual 0. introduced 2:

- **i1 (episode 21) — borderline.** The quotation "등대 아래 흩어져 있던 유리 조각과 깨진 렌즈의 귀퉁이는 그 밤 혼자 불을 다시 켜려던 백영호가 램프에 부딪혀 생긴 것이라고 짐작되었고…" ("the glass fragments scattered below the lighthouse and the corner of the broken lens were presumed to have come from Baek Yeong-ho, who had tried to relight it alone that night, bumping into the lamp…") is in M's s21. Seo-yun guesses that sequence of events before Baek Yeong-ho's testimony in episode 25 ("도경태가 내려간 뒤 나는 혼자 불을 다시 켜려고 올라갔소." — "After Do Gyeong-tae went down, I went up alone to relight it."). This sentence is what the reused revised IR's s21 act (`한서윤 thinks: … 백영호가 램프에 부딪혀 생긴 것이고 …`, "Han Seo-yun thinks: … it was caused by Baek Yeong-ho bumping into the lamp …") requires, so it is not a contradiction made by M's edit. B's s21 writes the same content ("그 밤 혼자 불을 다시 켜려던 백영호가 램프에 부딪혀 그렇게 되었을 것이다." — "It must have happened because Baek Yeong-ho, trying to relight it alone that night, bumped into the lamp."). It is awkward for Seo-yun to guess this specifically without grounds, but it is marked as a "guess" so we do not see it as a contradiction. Not counted.
- **i2 (episode 24) — rejected.** The quotation "서윤은 유리 조각과 렌즈 귀퉁이는 그 밤 혼자 불을 다시 켜려던 백영호가 램프에 부딪혀 생긴 것이고, … 짐작했다." ("Seo-yun guessed that the glass fragment and the lens corner came from Baek Yeong-ho, who tried to relight it alone that night, bumping into the lamp, …") and, in the same scene, "백영호가 램프에 부딪혀 깨뜨린 것이라면 저런 자국이 남을까. 서윤은 그 생각을 입 밖에 내지 않았다." ("If Baek Yeong-ho had broken it by bumping into the lamp, would a mark like that remain? Seo-yun did not say the thought aloud."). It is Seo-yun's guess and she even doubts it in the same scene, so the knowledge state does not clash. It is a sentence the revised IR's s24 act required.

Own audit (separate from the judge): searching M's s1–s31 for `몸싸움|렌즈를 깨|렌즈가 깨` ("scuffle | broke the lens | the lens broke") gives 0, and places where "깨/깨뜨" ("break/smash") and "도경태" are in one sentence are also 0. There is no trace of the old setting (Do Gyeong-tae broke the lens).

## R3 (episode 4 knows the alibi is false) — confirmed residual 0, judge items 1

residual 0. introduced 1:

- **i1 (episode 18) — borderline.** The quotation "지금은 이 종이 한 장뿐이야. 확실한 증거를 손에 쥐기 전까지 이 일을 아무에게도 말하지 않기로 하자." ("For now this is only a single sheet of paper. Until we hold solid evidence, let's not tell anyone about this.") (s32, B's newly created episode 4 scene as it is) and "육지 병원에는 그 무렵 도경태 씨가 입원했던 기록이 전혀 없어요." 서윤이 낮게 말했다. ("There is no record at all that Mr. Do Gyeong-tae was hospitalized at the mainland hospital around that time." Seo-yun said in a low voice.) (M's s18). Having agreed with Tae-o "not to tell anyone", she tells Hae-sol in the storeroom. But Hae-sol is someone who investigates this together with her, and the revised IR's s18 act requires Seo-yun to tell Hae-sol this fact. B's s18 says the same ("육지 병원에는 해진호가 가라앉던 무렵 도경태 씨가 입원했던 기록이 전혀 없어요." — "There is no record at all that Mr. Do Gyeong-tae was hospitalized at the mainland hospital around the time the Haejin-ho sank."). It can be read as a narrower scope rather than breaking the promise, so not counted.

Own audit: searching after episode 4 for sentences in which Seo-yun still believes the alibi turns up s20 ("…도경태 씨는 육지 병원에 입원해 있었던 거죠?" 서윤이 물었다 — "…Mr. Do Gyeong-tae was hospitalized at the mainland hospital, right?" Seo-yun asked) and s22 ("그날 밤 아저씨는 육지 병원에 입원해 계셨다면서요?" — "You were hospitalized at the mainland hospital that night, you said?"). Both are questions in which Seo-yun, who already knows the alibi is false, probes the other person (s22 has the same hash, so it is an untouched scene, and after this question Gyeong-tae answers with a lie), and the same sentences are in B's same scenes. They do not read as belief, so they were not counted as residual.

## R2 — not judged

Because of the call cap of 60, R2 tested only the first 6 of the 17 affected scenes (pilot). The full text was only partly fixed, so no judgment call was used.

## Totals

| | M confirmed residual | M confirmed new contradictions | Borderline (not counted) |
|---|---|---|---|
| R1 | 0 | 0 | 1 (rejected 1 separate) |
| R3 | 0 | 0 | 1 |
| Total | 0 | 0 | 2 |

If we strictly count all 3 judge-introduced items (borderline 2, rejected 1), M is 3 and B (confirmed new contradictions in the same two resets) is 0. All three are sentences the revised IR requires and the same content is in B's prose, so I judged that they were not created by M's edit (B's judge did not flag these three — the variation of a single judgment).
