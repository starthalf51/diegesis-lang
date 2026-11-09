# Judge item check (single verifier)

I checked the items from the judge (sonnet, once per arm and reset) one by one against the source text (`rN/a/eps/`, or `examples/serial/run3/a/eps/` if absent; `rN/b/novel/`).
Criteria: **confirmed** = the two clashing places can be pointed to in the source and no interpretation explains it / **borderline** = explainable (a guess, a recollection, a character's lie, identity fixed by job title), so not counted / **rejected** = an old problem unrelated to the reset (already in the original run3) or not a contradiction.
Then, to see whether the judge missed anything, I did a keyword audit (grepping the reset's old-setting expressions in the final text, episodes 1–11 and s1–s31).

| Reset | Arm | Judge item | My classification | Basis |
|---|---|---|---|---|
| R1 | A | residual 1 (episode 11 ledger summary "몸싸움 중 렌즈 하단 파손", "lower part of the lens broken during the scuffle") | **confirmed residual 1** | The episode 11 ledger line is the old setting as is ("도경태 … 근무자가 저지함. 몸싸움 중 렌즈 하단 파손", "Do Gyeong-tae … the attendant stops him. Lower part of the lens broken during the scuffle"); episode 10 was fixed to say Yeong-ho broke it alone. A's pick call chose only episode 10 and missed episode 11 |
| R1 | A | introduced 1 (episode 4 door window intact vs episode 11 "board over the spot of the broken door glass") | rejected | The same sentence is already in the original run3 episode 11, and the "small window" in episode 4 is in the original too. An old continuity problem unrelated to the reset |
| R1 | B | residual 0 | audit: residual 0 | In s1–s31, near the lens and scuffle, there is no expression that Gyeong-tae broke it. s28 is "경태가 그러더군. 렌즈를 깨뜨린 게 등대지기 나니" ("Gyeong-tae said so. That it was me, the lighthouse keeper, who broke the lens") (Gyeong-tae's claim; consistent with the new setting) |
| R1 | B | introduced 3 (s21, s24, s27 "장부에 적힌 대로 백영호가 깨뜨린", "Baek Yeong-ho, who broke it as written in the ledger") | **borderline 3 (not counted)** | One problem: the old prose's "도경태가 … 짐작되었다" ("it was presumed that Do Gyeong-tae …") was patched to "장부에 적힌 대로 백영호가 깨뜨린". The s21 ledger page writes that the attendant's name is smeared and hard to read, and no record of the lens damage is visible. It is a guess sentence and attendant = Yeong-ho can be known from the job title, so it cannot be asserted as a contradiction |
| R2 | A | residual 2 (episode 9 Su-a "동쪽 벼랑 밑예 … 돌 틈", "under the east cliff … a gap between rocks"; Seo-yun "동쪽 벼랑 밑에서 아이가 주웠어요", "A child found it under the east cliff") | **confirmed residual 2** | Episodes 3 and 4 were fixed to the end of the breakwater and episode 9 was not (A's pick call chose only episode 4, wrote episodes 3 and 4, and missed episode 9). The "렌즈 한쪽이 … 벼랑 밑까지 갔다" ("one side of the lens … went all the way under the cliff") in the same episode 9 is of the same family, so it was not counted |
| R2 | B | residual 1 (s11 "정수아가 바위틈에서 주운 유리 조각은 바로 이 귀퉁이에서", "the glass shard Jeong Su-a picked up from the rock crevice came from exactly this corner") | **confirmed residual 1** | s6, s8, s9, s21, s24, s27 all say the end of the breakwater, and only s11 has the old "rock crevice". The patch missed this sentence in s11 |
| R2 | B | (audit) s24 scene opening "등대 아래 바위틈 … 수아가 손수건을 펼쳐 들고" ("the rock crevice below the lighthouse … Su-a spreading out a handkerchief") | borderline (not counted) | It is the scene's place setting (the survey location), and the dialogue was fixed to the end of the breakwater. The judge did not point it out either |
| R3 | A | i1 episode 7 "그 밤에 경태 저거 섬에 있었다" ("that Gyeong-tae was on the island that night") (the grandmother in episode 4 mentions only the pier the next morning) | borderline | An expansion in recollection. The grandmother may have said more |
| R3 | A | i2 episode 4 words heard in the daytime at the end of the veranda vs episode 7 "토요일 저녁 … 부엌으로 끌고 가" ("Saturday evening … dragged her into the kitchen") | **confirmed new contradiction 1 (minor)** | Episode 4 is the end of the veranda, episode 7 the kitchen. The place clashes (a trace of episode 7 having been rewritten from episode 4) |
| R3 | A | i3 episode 7 'no confirmation needed' location | rejected | The same sentence is already in the original episode 7 |
| R3 | A | residual 0 | audit: residual 0 | In episodes 5–11 there is no narration in which Seo-yun believes the alibi (episodes 8 and 9 are probing questions) |
| R3 | B | residual 0, introduced 0 | audit: residual 0 | s16 and s19 are Gyeong-tae's and Ha-eun's words, and Seo-yun's questions in s20 and s22 read as probing by someone who knows the lie (borderline, not counted) |

## Confirmed totals

| | R1 | R2 | R3 | Total |
|---|---|---|---|---|
| A residual / new contradictions | 1 / 0 | 2 / 0 | 0 / 1 | **3 / 1** |
| B residual / new contradictions | 0 / 0 | 1 / 0 | 0 / 0 | **1 / 0** |
| Borderline A / B (not counted) | 1 / 3 | 0 / 1 | 1 / 2 | 2 / 6 |

In the automatic cross-check of judge quotations (`rN/judge/*.quotes.md`), every quotation was present verbatim in the source (22/22 in total; B R3 had 0 items).

# Corrections and re-run check

- **Correction:** B's R1 s21 in the first run is raised to a confirmed new contradiction (reviewer cross-check; the sentence saying the attendant's name is smeared and unreadable and "장부에 적힌 대로 백영호가" ("Baek Yeong-ho, as written in the ledger") clash in the same scene). First-run total: B 2 vs A 4. s24 and s27 are repetitions of the same problem (counted per location, B would be 4).
- **Re-run (`rerun/`) B check:**

| Reset | Judge | My check |
|---|---|---|
| R1 | residual 0 / introduced 0 | In the audit (Gyeong-tae/ledger expressions near the lens and scuffle), only the Yeong-ho confession in s25 and the ledger fragment in s30 turn up, residual 0. The first run's "as written in the ledger" sentence is absent |
| R2 | 0 / 0 | Audit: "rock crevice" is only in the two scene-opening place lines of s24 (the same borderline as in the first run, not counted). Residual 0 (s11 fixed) |
| R3 | residual 0 / introduced 1 (s11b "일층 계단 입구" ("first-floor stair entrance") vs "계단을 내려갔다" ("went down the stairs"), location relative to s9) | Borderline. s11b comes after s11 in novel order, so the judge's premise of "the following s9" is wrong. Audit: after episode 4 there is no narration in which Seo-yun believes the alibi (s11b is the reset's scene, and s20 and s22 are probing questions). New contradictions 0, borderline 1 |
