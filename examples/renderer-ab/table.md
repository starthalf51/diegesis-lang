실제 호출 117회

- macbeth/1.3 (acts 26, beats 11)
- chibi/45.88 (acts 24, beats 10)
- haejin/s3 (acts 12, beats 4)
- haejin/s19 (acts 12, beats 4)

| 팔 | n | 통과 | 평균 호출 | sonnet 토큰 in/out | haiku 토큰 in/out | 가중 토큰/장면 | 어투 | 글자(목표 2500) | 비트 재시도 | 환각 new_event/사실형 | 오류 |
|---|---|---|---|---|---|---|---|---|---|---|---|
| S-son | 2 | 1/2 (50%) | 6.0 | 36252/12725 | 0/0 | 149816 | 1.00 | 690 | 0 | 2/1 | 0 |
| L-son | 2 | 0/2 (0%) | 18.0 | 74205/27413 | 0/0 | 316905 | 0.99 | 1444 | 6 | 3/1 | 0 |
| L-hai | 2 | 1/2 (50%) | 17.0 | 41697/4074 | 30409/50226 | 233870 | 1.00 | 1469 | 5 | 1/0 | 0 |
| S-hai | 2 | 1/2 (50%) | 4.0 | 12428/1842 | 13069/16526 | 80306 | 1.00 | 1048 | 0 | 2/0 | 0 |

| 추가 | 장면 | 백엔드 | 통과 | 호출 | 렌더 모델 in/out | sonnet in/out | 어투 |
|---|---|---|---|---|---|---|---|
| L-hai-swap | haejin/s3 | ko-hapsho | true | 3 | haiku 7044/9211 | 3997/436 | 1.00 |
| S-son-hapsho | haejin/s3 | ko-hapsho | true | 6 | sonnet 10894/5565 | 8437/1607 | 1.00 |
| S-son-extra | chibi/45.88 | ko-haera | false | 6 | sonnet 9030/6049 | 7959/2899 | 1.00 |
| S-hai-extra | chibi/45.88 | ko-haera | false | 6 | haiku 9142/15773 | 9091/2357 | 1.00 |
| S-son-extra | macbeth/1.3 | ko-haera | false | 6 | sonnet 10286/10179 | 11108/3696 | 1.00 |
| S-hai-extra | macbeth/1.3 | ko-haera | false | 0 | haiku 0/0 | 0/0 | 0.00 |budget: used 117 + est 6 > 119

판정: 렌더러 single, 렌더 모델 claude-haiku-5-5 — 최고 통과율 S-hai 50% (가중 토큰 80306); S-hai 50% vs S-son 50% → claude-haiku-5-5
