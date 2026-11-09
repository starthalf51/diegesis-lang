# Judge item verification (human cross-check)

I checked the items from the judge (sonnet, once per arm and reset) one by one against the post-revision source. Whether a quotation exists verbatim in the source was filtered first by the automatic cross-check (`rN/judge/{a,b}.quotes.md`, all "present"), and what follows looks at **whether it is really a contradiction**.
Rule: confirmed only when the two sentences cannot both be true. If an interpretation explains it, borderline (not counted). For `introduced` I compared against whether the inconsistency was already in the pre-revision source (run3). Borderline items were dropped with the same rule for both arms.
"Audit" is what I found by sweeping keywords for things the judge did not raise (not included in the table's base values, written separately). The sweep method is at the end of this document.

## R1 — who broke the lens: Do Gyeong-tae → Baek Yeong-ho

| Arm | Item | Result | Basis |
|---|---|---|---|
| A | Episode 11 ledger entry | **confirmed** (residual) | Episode 11 (not revised): "00:10경 도경태 이장 계단으로 올라옴, 문 유리 깨고 등실 진입, 점등 요구. 근무자가 저지함. **몸싸움 중 렌즈 하단 파손**, 파편 창밖으로 떨어짐." ("Around 00:10 village head Do Gyeong-tae comes up the stairs, breaks the door glass, enters the lantern room, demands lighting. The attendant stops him. **Lower part of the lens broken during the scuffle**, fragments fall out the window.") The reset says Yeong-ho bumped into the lamp alone after the village head went down. A rewrote only episodes 10 and 12. In episode 12 the same ledger phrase became "근무자가 저지함. 이장 내려감. 영시 이십 분경 근무자 혼자 점등하려다 램프에 부딪힘." ("The attendant stops him. The village head goes down. Around 00:20 the attendant, trying to light it alone, bumps into the lamp."), so **episodes 11 and 12 read the same ledger differently.** |
| B | s21 | borderline (not counted) | "짐작이 맞다면 둘은 한 자리에서 다쳤고, 그 자리에 도경태가 있었다." ("If the guess is right, the two were hurt in one place, and Do Gyeong-tae was there.") In the same paragraph Seo-yun guesses that Yeong-ho bumped into the lamp and broke it alone. "One place" points to the lantern room, and Do Gyeong-tae was in the lantern room first that night (s25, s27: "도경태가 내려간 뒤에 나는 혼자…" — "After Do Gyeong-tae went down, I alone…"). It does not assert that the two were there at the same time, so both can be true. |
| Audit | B overall | none | I read every place (s21, s24, s25, s26, s27) where `경태` and one of `렌즈·깨뜨·깨져·깼·부딪·뿌리` ("lens, smash, broken, broke, bump, shake off") are on the same line. The old setting ("뿌리친 오른손이 렌즈를 깨뜨렸다", "the hand he shook off smashed the lens", s27 / "렌즈를 깨다가 손을 벤 경태", "Gyeong-tae, who cut his hand breaking the lens", s26) does not remain. |
| Audit | A overall | no addition | Episode 9 line 117, "바람에 깨진 렌즈 조각이 손등에 박힐 수는 없었다" ("a lens fragment broken by the wind could not have lodged in the back of a hand"), is Seo-yun rebutting Yeong-ho's false explanation, so not a contradiction. |

introduced: A 0, B 0.

## R2 — where the glass shard was picked up: among the rocks below the lighthouse → the end of the pier breakwater

| Arm | Item | Result | Basis |
|---|---|---|---|
| A | Episode 3 | **confirmed** (residual) | Episode 3 (not revised): "등대 밑에. 벼랑 바로 밑이라서 엄마가 가지 말라 카는 데다. … 돌 틈에 박혀 있었다." ("Under the lighthouse. It's right below the cliff, a place Mom says not to go. … It was wedged in the rocks.") Episode 4 (revised) became "방파제 끝에서 … 수아가 태풍 다음 날 …" ("at the end of the breakwater … Su-a, the day after the typhoon …"). |
| A | Episode 9 (Su-a's line) | **confirmed** (residual) | Episode 9 (not revised): "동쪽 벼랑 밑예. 물 제일 많이 빠진 날. 돌 틈에 끼어 있었어예." ("Under the east cliff. The lowest-tide day. It was wedged between rocks.") — not the end of the breakwater. |
| A | Episode 9 (Seo-yun's line) | **confirmed** (residual) | Episode 9: "동쪽 벼랑 밑에서 아이가 주웠어요. 오늘 낮에 등대 렌즈에 대 봤습니다." ("A child found it under the east cliff. I held it to the lighthouse lens today.") A different line in the same episode, so counted separately (merged, A would be 2). |
| B | introduced episode 6 | **confirmed** (new contradiction) | Episode 6: "그것이 저 **방파제 끝의 등대 불**을 가리키는 것인지" ("whether it pointed to that **lighthouse light at the end of the breakwater**") — before the revision s6 was a scene where Seo-yun "looked toward the lighthouse" (line 9) and did not put the lighthouse at the end of the breakwater. Episode 12 (s12, revised): "섬 가운데 높이 솟은 등대의 흰 몸통" ("the white body of the lighthouse rising high in the middle of the island"). While applying the reset it moved the lighthouse to the end of the breakwater. |
| Audit | A | +1 | Episode 9 line 83, "바람에 렌즈 한쪽이 떨어져 나가서 **벼랑 밑까지** 갔다고요." ("You said that one side of the lens came off in the wind and went **all the way under the cliff**.") A premise of the old location, that the shard fell below the cliff. The judge missed it (A rewrote only episode 4). |
| Audit | B | none | I swept `바위틈·방파제·벼랑 아래·등대 아래` ("rock crevice, breakwater, below the cliff, below the lighthouse") across all scenes. Every place that gives the shard's location (s6, s8, s9, s21, s24, s25, s27) says the end of the breakwater. "등대 아래 계단참" ("the landing below the lighthouse") in s9 is a conversation location. |

## R3 — when Seo-yun learned Do Gyeong-tae's alibi is false: the medical record → episode 4

| Arm | Item | Result | Basis |
|---|---|---|---|
| A | introduced episode 8 | **confirmed** (new contradiction, minor) | Episode 4 (revised): coming down from the lighthouse, "말순은 방에서 낮잠이 들어 있었다" ("Mal-sun had fallen asleep in the room for an afternoon nap"), and then the grandmother from the lower house says the alibi is false. Episode 8 (revised): "토요일 **저녁** 마늘 망 앞에서 들은 말" ("words heard in front of the garlic net on Saturday **evening**"). It calls something from nap time evening (episode 7 says "토요일 저녁에 적은 글씨" — "writing written on Saturday evening", so the time of writing and the time of hearing may just have been mixed; minor). The pre-revision source did not have these two places. |
| B | introduced s32↔s27 | borderline (not counted) | New scene s32 (episode 4): "그 사람은 그날 밤 섬을 떠나지 않았어." ("That man did not leave the island that night.") s27: "몰래 배를 타고 섬에 들어와 있었소." ("I had secretly taken a boat and come onto the island.") Both can be true in that "Do Gyeong-tae was on the island that night". However s32 asserts more than its basis (a single page with no hospital record). |
| Audit | A | none | I swept `알리바이·병원·맹장·믿었·의심` ("alibi, hospital, appendix, believed, suspected") in episodes 4–9. There is no sentence after episode 4 in which Seo-yun believes the alibi (episode 5 "이제 그 말들을 전처럼 받아…" — "now she no longer took those words as before…", episode 6 "이제는 거짓인 줄 아는 병원" — "the hospital she now knew was false"). |
| Audit | B | none | I swept `병원·알리바이·맹장` after s9. s18 was rewritten on the premise that Seo-yun already knows ("기록이 전혀 없어요" — "there is no record at all"). The "was in the hospital" in s16, s19, s20, s22 are words of Do Gyeong-tae, Ha-eun, and Mal-sun. |

## Totals

| | Confirmed residual (judge items) | Confirmed new contradictions | Sum | More found by audit |
|---|---:|---:|---:|---:|
| A | R1 1 · R2 3 · R3 0 = **4** | 0 · 0 · 1 = **1** | **5** | R2 +1 |
| B | R1 0 · R2 0 · R3 0 = **0** | 0 · 1 · 0 = **1** | **1** | 0 |

Borderline (not counted): B 2 (R1 s21, R3 s32), A 0.
Including the audit, A's residual is 5 and the sum 6. The judge was called only once per novel, so B may also have missed items. The keyword scope I swept was the same for A and B (the "Audit" rows in the table above).

### How the audit swept

In the revised arm's text, I grepped keywords that point to the reset's old setting (R1 `렌즈|깨뜨|깨져|깼|부딪|뿌리|몸싸움` — "lens, smash, broken, broke, bump, shake off, scuffle"; R2 `벼랑 밑|벼랑 바로|돌 틈|바위틈|방파제` — "under the cliff, right below the cliff, rock gap, rock crevice, breakwater"; R3 `알리바이|병원|맹장|믿|의심` — "alibi, hospital, appendix, believe, suspect") and read the resulting lines with context. It was done by a person, not used by the judge, and cannot see contradictions that miss the keywords.
