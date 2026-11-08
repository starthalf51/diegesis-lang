# 렌더 effort A/B (플랜 6 태스크 5)

haiku(`claude-haiku-5-5`) 렌더를 effort `medium` 과 `low` 로 돌려 card 검증기 점수로 비교했다. 결과가 기본 `--render-effort` 를 정한다(반영은 태스크 7).

- 하네스: `go run ./tools/corpus-ab -spike effort -budget 60 -out examples/effort-ab` (드라이런 `-spike effort-dry`, 표만 다시 `-spike effort-report`).
- 장면마다 `render.RenderVerified2`(`K:1, Retries:2, Verifier:"card", Card:{}`) 1회. 채점·재채점은 sonnet/low. 백엔드는 모두 `ko-haera`.
- 장면 순서대로 medium → low 를 끼워 돌렸다. 호출마다 예산 가드(60, 하드 캡)가 앞선다.
- TargetChars 가 0 이라 S4(분량)는 빠졌고 Score 는 `(0.6·S1+0.2·S2+0.1·S3)/0.9` 다. S1 사실 있음, S2 사실형 맞음, S3 어투.
- 가중 토큰은 `3·(입력+캐시)+15·출력`(tools/corpus-ab 가중치)을 렌더와 채점 호출에 합산했다.
- 산문·장면별 JSON 은 `runs/`, 호출 기록은 `calls.jsonl`, 표는 `table.md`.

## 장면 (6)

| 장면 | 출처 |
|---|---|
| bible/mark_denial | 코퍼스 render-check |
| chibi/45.88 | 코퍼스 render-check |
| macbeth/1.3 | 코퍼스 render-check |
| odyssey/19.1 | 코퍼스 render-check |
| haejin/s3 | 연재 haejin (행위 수 상위 2, FreeDialogue) |
| haejin/s19 | 연재 haejin (같음) |

## 장면·팔별 결과

| 장면 | effort | 통과 | 시도 | Score | S1 | S2 | S3 | 렌더 출력토큰/호출 | 렌더 호출 | 채점 호출 | 가중 토큰 |
|---|---|---|---|---|---|---|---|---|---|---|---|
| bible/mark_denial | medium | ✓ | 1 | 1.000 | 1.00 | 1.00 | 1.00 | 3043 | 1 | 2 | 86889 |
| bible/mark_denial | low | ✓ | 1 | 1.000 | 1.00 | 1.00 | 1.00 | 3830 | 1 | 1 | 81618 |
| chibi/45.88 | medium | ✓ | 1 | 1.000 | 1.00 | 1.00 | 1.00 | 5599 | 1 | 1 | 115512 |
| chibi/45.88 | low | ✓ | 1 | 1.000 | 1.00 | 1.00 | 1.00 | 6118 | 1 | 1 | 124650 |
| macbeth/1.3 | medium | ✓ | 1 | 1.000 | 1.00 | 1.00 | 1.00 | 8646 | 1 | 1 | 171756 |
| macbeth/1.3 | low | ✓ | 1 | 0.998 | 1.00 | 1.00 | 0.98 | 5122 | 1 | 1 | 117714 |
| odyssey/19.1 | medium | ✓ | 2 | 0.968 | 1.00 | 0.86 | 1.00 | 5288 | 2 | 4 | 256386 |
| odyssey/19.1 | low | ✓ | 1 | 1.000 | 1.00 | 1.00 | 1.00 | 5412 | 1 | 2 | 134853 |
| haejin/s3 | medium | ✓ | 1 | 1.000 | 1.00 | 1.00 | 1.00 | 4480 | 1 | 1 | 101706 |
| haejin/s3 | low | ✓ | 1 | 1.000 | 1.00 | 1.00 | 1.00 | 4104 | 1 | 1 | 92244 |
| haejin/s19 | medium | ✓ | 1 | 1.000 | 1.00 | 1.00 | 1.00 | 3522 | 1 | 1 | 83238 |
| haejin/s19 | low | ✓ | 1 | 1.000 | 1.00 | 1.00 | 1.00 | 3273 | 1 | 1 | 78900 |

## 평균 (6장면 모두 두 팔이 끝남)

| | medium | low |
|---|---|---|
| 평균 Score | 0.9947 | 0.9997 |
| 통과 장면 | 6/6 | 6/6 |
| 렌더 출력 토큰/호출(합산) | 5123.9 | 4643.2 |
| 장면당 가중 토큰 | 135,915 | 104,997 |

## 판정

미리 정한 규칙: low 가 평균 Score 하락 ≤ 0.02, 통과 장면 수 감소 없음, 렌더 출력 토큰/호출 감소를 모두 만족하면 low.

- 평균 Score 하락(medium−low) = −0.0049 (low 가 오히려 약간 높음) → 만족
- 통과 6 대 6 → 만족
- 렌더 출력 토큰/호출 5123.9 → 4643.2 (−9.4%) → 만족

기본 effort: low

## 실제 호출 수

**30회**(예산 60). 오류 호출 0, 낭비 0. 예산 가드는 끊기지 않았다.

## 한계

- n=6 장면, 장면·팔당 1회 실행. 표준편차를 못 낸다.
- 차이가 작다. Score 차이는 odyssey 의 medium 재시도 1회(S2 0.86)와 macbeth 의 어투 0.02 가 거의 전부다. 한 번 더 돌리면 뒤집힐 수 있다.
- 토큰 감소는 장면에 따라 부호가 다르다(bible·chibi 는 low 가 더 많고 macbeth 는 크게 적다). 평균 −9.4% 는 macbeth 하나가 끌었다.
- 통과 기준이 ADR-019(환각 0·누출 0·누락 ≤ MaxMissing)라 점수 천장(1.0)에 많이 붙어 있어 변별력이 낮다.
- card 채점기는 sonnet/low 한 번 판정이다. 채점 잡음은 따로 재지 않았다.
