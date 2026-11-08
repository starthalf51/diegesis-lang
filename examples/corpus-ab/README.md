# 코퍼스 장면 A/B — 단일+haiku vs 계층형+haiku (검증기 수정 후, v0.5.0)

질문: 검증기 잡음 수정(v0.5.0) 뒤, render-check 코퍼스 장면에서 현재 기본(단일 검증 렌더러, haiku/medium 렌더, sonnet/low 재추출, 재시도 2)이 통과하는가. 실패하면 계층형+haiku 가 고치는가.

- 하네스는 `tools/corpus-ab` (`go run ./tools/corpus-ab -mode single|layered -budget N -out DIR [-pairs ...]`). renderer-ab 는 장면 4개가 코드에 박혀 있어 건드리지 않고 형제 도구로 만들었다. 호출 상한은 `counter` 로 강제한다.
- "장면 16개" 는 **장면 4개 × 백엔드 4개**(ko-haera·ko-hapsho·ko-gojeon·en-doyle)다. expect.yaml 에 render_scene 이 있는 코퍼스는 bible·chibi·macbeth·odyssey 4개뿐이다. 아래에서 한 쌍(장면×백엔드)을 한 실행이라 부른다. 같은 장면의 백엔드끼리는 독립 표본이 아니다.
- 통과 기준은 `render.Passed`: 환각 0, 누설 0, missing ≤ 1, 추출 오류 없음. (render-check 의 Jaccard·어투 조건은 안 쓴다.)
- 산출물: `p1/`(단일 16쌍, JSON·산문·calls.jsonl), `p2/`(계층형), `p1.log`·`p2.log`. 프롬프트·검증기는 바꾸지 않았다.

## 호출 수

| 단계 | 상한 | 사용 |
|---|---|---|
| Phase 1 단일+haiku, 16쌍 | 100 | **84** |
| Phase 2 계층형+haiku | 60 | **58** |

호출 오류 0건. 모델 호출은 구독 `claude` CLI 로만 했다.

## 장면

| 장면 | 행위 | 비트 |
|---|---|---|
| bible/mark_denial | 10 | 4 |
| chibi/45.88 | 24 | 10 |
| macbeth/1.3 | 26 | 11 |
| odyssey/19.1 | 29 | 10 |

## Phase 1 — 단일+haiku

**통과 5/16 (31%)**. 실패 11건.

| 장면 (비트) | ko-haera | ko-hapsho | ko-gojeon | en-doyle | 장면 통과 |
|---|---|---|---|---|---|
| bible/mark_denial (4) | ✓ (4호출) | ✗ | ✗ | ✗ | 1/4 |
| chibi/45.88 (10) | ✗ | ✗ | ✗ | ✗ | 0/4 |
| macbeth/1.3 (11) | ✓ (2) | ✓ (4) | ✓ (2) | ✗ | 3/4 |
| odyssey/19.1 (10) | ✗ | ✓ (6) | ✗ | ✗ | 1/4 |

실패 사유(missing / 환각 / 누설):

| 쌍 | 호출 | 글자 | missing | 환각 |
|---|---|---|---|---|
| bible en-doyle | 6 | 954 | 1 (`peter\|assert\|not peter_with_jesus`) | extra-act `peter\|assert\|not peter_knows_jesus` |
| bible gojeon | 6 | 376 | 2 (perceive 누락, assert 누락) | "베드로가 불에서 문간으로 물러난다" + extra-act 2 (사실형 종류 어긋남) |
| bible hapsho | 6 | 455 | 0 | extra-act `peter\|assert\|not peter_knows_jesus` |
| chibi en-doyle | 6 | 2952 | 3 (`jiang_gan\|read`, `wu_officers\|assert`, `zhou_yu\|assert`) | "장간이 서신을 로브에 숨긴다" |
| chibi gojeon | 6 | 830 | 2 | "서신을 챙기고 나중에 도망간다" + extra-act `jiang_gan\|assert\|not jiang_gan_envoy` |
| chibi haera | 6 | 922 | 2 | "장간이 편지를 몰래 소매 속에 감추었다" |
| chibi hapsho | 6 | 962 | 1 (`jiang_gan\|read`) | "장간이 서신을 챙긴다" |
| macbeth en-doyle | 6 | 5884 | 3 (`holds(king,macbeth)` ×2, `holds(thane_of_glamis)`) | – |
| odyssey en-doyle | 6 | 3301 | 2 | – |
| odyssey gojeon | 6 | 1315 | 3 | "아테나가 페넬로페의 주의를 돌린다" |
| odyssey haera | 6 | 1309 | 2 | – |

누설은 16쌍 모두 0건이다. 통과한 macbeth 3쌍도 `holds(king,macbeth)` 1건이 빠졌다(허용 한도 1).

### 환각 분류 (IR 대조)

환각 항목 12건을 코퍼스 IR(`corpus/*/…dieg.md`)과 하나씩 맞댔다.

| 항목 | IR | 판정 |
|---|---|---|
| chibi en-doyle "로브에 숨긴다" | `jiang_gan does steal forged_letter` | **잡음(신규 유형)** |
| chibi gojeon "서신을 챙기고 나중에 도망" | `does steal` + `does flee` | **잡음(신규 유형)** |
| chibi haera "소매 속에 감추었다" | `does steal` | **잡음(신규 유형)** |
| chibi hapsho "서신을 챙긴다" | `does steal` | **잡음(신규 유형)** |
| odyssey gojeon "아테나가 주의를 돌린다" | `athena does divert penelope` (19.30) | **잡음(신규 유형)** |
| bible en-doyle, hapsho extra-act `not peter_knows_jesus` | mark_denial 에 `lies … not peter_knows_jesus` 는 1회(마가 14:71). 산문이 첫 부인(`not peter_with_jesus`)을 "그를 모른다" 로 바꾸거나 한 번 더 말함 | 실제(사실 치환·중복) |
| bible gojeon "물러난다" | 행위로 쓰인 적 없음("went out on the porch" 는 `sees` 의 인용문에만 있음) | 실제(꾸밈) |
| bible gojeon extra-act ×2 | perceive→assert, assert→act 로 종류가 바뀜. missing 2건과 쌍 | 실제(사실형 종류) |
| chibi gojeon extra-act `not jiang_gan_envoy` | IR 의 거짓말은 사실 `not_envoy` 하나 | 실제 또는 중복 진술(확정 못 함) |

- **잡음 5건, 실제 7건.** 잡음은 모두 사실 없는 `does` 를 산문이 다른 동사로 다시 말한 것이다. `authored()` 가 `new_event.Action` 과 IR 행위의 동사를 **문자열 완전 일치**로 비교해서(steal≠hide/pocket, flee≠도망, divert≠돌리다) v0.5.0 수정이 이 경우를 못 거른다. 수정 전 잡음(등퇴장·reach·hide)과 달리 **동의어 동사 불일치**라는 새 유형이다. 검증기는 고치지 않았다.
- 잡음만 제거한 반사실: chibi hapsho 는 환각 0·missing 1 이라 통과한다. 나머지 4건(chibi 3, odyssey gojeon)은 missing 이 ≥2 라 계속 실패한다. 반사실 통과율 **6/16**. 실제 실패 **10/16**.
- 실제 실패의 주된 형태는 **긴 장면의 사실 누락**(chibi·odyssey·macbeth en-doyle 의 missing 2~3). 짧은 bible 은 누락이 아니라 사실형 치환·종류 어긋남이다.

### 비트 수별 통과율

| 비트 | 실행 수 | 통과 | 비고 |
|---|---|---|---|
| 4 | 4 | 1 (25%) | bible 1장면 |
| 10 | 8 | 1 (13%) | chibi, odyssey |
| 11 | 4 | 3 (75%) | macbeth 1장면 |

비트 수와 통과율은 단조가 아니다. 장면 3~4개가 가른 수치라 장면 고유 성질(예: macbeth 는 누락 1건이 한도 안)이 비트 수 효과보다 크다.

## Phase 2 — 계층형+haiku (`narrate.RenderLayered`, 비트 재시도 기본)

예산 때문에 2쌍만 돌렸다(긴 장면 우선). chibi haera 와 odyssey haera 는 추정 28회가 남은 예산(27회)을 넘어 **돌리지 못했다**(chibi 는 1회 차이).

| 쌍 | 단일 | 계층형 | 호출(단일→계층) | 비트별 시도 | 채택본 |
|---|---|---|---|---|---|
| macbeth/1.3 en-doyle (11비트) | ✗ missing 3 | **✗** missing 2 (`holds(king)`, `holds(thane_of_glamis)`), 환각 0 | 6 → 33 (5.5×) | 1,1,1,2,1,1,1,1,1,3,1 (비트 10 실패) | unstitched |
| bible/mark_denial ko-hapsho (4비트) | ✗ 환각 1 | **✗** missing 1, 환각 1(extra-act `not peter_knows_jesus`) | 6 → 25 (4.2×) | 3,3,3,1 (비트 1 실패) | stitched |

계층형은 macbeth 에서 missing 을 3→2 로 줄였지만 통과 한도(≤1)에 못 미쳤다. bible 에서는 환각이 그대로다.

## 호출·토큰

| 단계 | 호출 | haiku(렌더) in/out | sonnet(재추출) in/out |
|---|---|---|---|
| Phase 1 (16쌍) | 84 (쌍당 평균 5.3) | 149,712 / 250,600 | 125,994 / 34,712 |
| Phase 2 (2쌍) | 58 (쌍당 평균 29) | 46,511 / 60,342 | 71,392 / 8,035 |

in 은 input+cache_read+cache_write. Phase 2 는 같은 두 쌍의 단일 호출(12)과 비교하면 약 4.8배다. 쌍별 토큰은 각 JSON 에 있다.

## 판정

사전 규칙: 긴 장면(≥8비트)의 Phase 1 실패 중 절반 이상을 계층형이 통과로 바꾸고 총 호출 ≤ 단일의 4배이면 "계층형 = 긴 장면 전용으로 유지", 아니면 "계층형 삭제 후보".

- 긴 장면 Phase 1 실패는 8건(chibi 4, odyssey 3, macbeth 1). 절반이면 4건 전환이 필요하다.
- 계층형으로 돌린 긴 장면은 1건이고 **0건 통과**, 호출은 5.5배(>4배).
- **판정: 계층형 삭제 후보** (규칙 그대로). 단 근거는 약하다. 긴 장면 n=1 이라 "계층형이 효과 없다" 가 아니라 "효과를 보인 증거가 없고 비용은 4~5배" 까지만 말할 수 있다. 계층형이 통과로 바꾼 것은 0/2 이다.
- 부수 결과: 현 기본(단일+haiku)의 통과율은 5/16 (잡음 제거 시 6/16). 실제 실패는 긴 장면의 사실 누락이 주이고, 검증기 쪽에는 동의어 동사 불일치 잡음이 새로 남아 있다.

## 한계

- 표본 16쌍은 장면 4개의 백엔드 변형이라 독립이 아니다. 장면 단위로는 n=4.
- Phase 2 는 2쌍(긴 장면 1). 예산 때문에 chibi·odyssey 는 계층형으로 못 돌렸다. 나머지 9건은 미측정이다.
- 장면당 1회 실행이라 실행 간 분산을 모른다. macbeth en-doyle 도 한 번의 계층형 결과다.
- 분류 중 `not jiang_gan_envoy` 는 IR 의 사실 정규화까지 확인하지 않아 확정하지 못했다.
