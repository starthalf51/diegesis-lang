# 스파이크 — 수리 패스(H1) + 추출 어휘(H2)

일회용 실험. 입력은 Phase 1 의 산문·판정(`../p1`)이고 새로 렌더하지 않았다. 코드는 `tools/corpus-ab/spike.go`(`-spike h1|h2`), 호출 상한 가드(`counter`)·`Meter`·`ExtractK`/`Vote`/`Diff`/`Passed` 를 그대로 썼다. 프로덕션 추출 프롬프트는 건드리지 않았다(변형은 spike.go 안). 항목별 JSON·수리 산문은 이 폴더(`h1_*`, `h2_*`), 호출 로그는 `calls.jsonl`.

## 호출
H2 24 + H1 40 = **64 / 75**. 오류 0. 구독 `claude` CLI 만 사용.

## H2 — 추출기에 지은이 행위 어휘 주기 (24호출)
변형: 추출 프롬프트에 장면 IR 의 `does` 행위(동작어: 행위자 -> 대상)를 나열하고 "문장이 이 행위를 다시 말하면 정확히 그 동작어로 보고" 지시. 8개 산문 × K=3 sonnet/low.

**사전 기준**: 잡음 5건 전부 사라짐, 실제 7건 전부 유지, 새 환각(실제든 잡음이든) 없음.

| 쌍 | 이전 환각 | 이후 환각 | 비고 |
|---|---|---|---|
| chibi en-doyle | 잡음 "로브에 숨긴다" | 없음 | 잡음 제거 |
| chibi gojeon | 잡음 "pockets…flees" + 실제 extra-act `not jiang_gan_envoy` | 새 항목 `extra-act:jiang_gan\|perceive\|commanders_traitors` | 잡음 제거. 실제 항목 소실. 새 항목은 missing `jiang_gan\|read` 의 종류 어긋남(실제) |
| chibi haera | 잡음 | 없음 | 잡음 제거 |
| chibi hapsho | 잡음 | 없음 | 잡음 제거(→ 통과) |
| odyssey gojeon | 잡음 "주의를 돌린다" | 없음 | 잡음 제거 |
| bible en-doyle | 실제 `not peter_knows_jesus` | 동일 | 유지 |
| bible hapsho | 실제 `not peter_knows_jesus` | 없음(→ 통과) | **실제 항목 소실** |
| bible gojeon | 실제 3(물러난다, extra-act 2) | 실제 1(`extra-act:peter\|assert\|not peter_knows_jesus`) | 3건 소실, 이 항목은 새 것(IR 에 `lies … not peter_knows_jesus` 1회라 초과분=실제) |

- 잡음 제거 **5/5**.
- 실제 유지: JSON 기준 실제 항목은 6건(en 1, hapsho 1, gojeon 3, chibi gojeon 1; README 의 "7" 과 1건 차이 — Phase 1 표가 12건으로 적었으나 JSON 환각 합계는 11건)이고 그중 **1/6 유지**(bible en-doyle). 소실 5건은 동의어 잡음이 아니라 `extra-act`(사실형·중복)·꾸밈이어서 어휘 주입이 겨냥하지 않은 항목이다. 즉 어휘가 실제 환각을 가렸다기보다 K=3 재추출 자체의 변동이 크다(같은 항목 군이 두 방향으로 바뀜).
- 새 항목 2건, 모두 실제(잡음 0): bible gojeon `not peter_knows_jesus`, chibi gojeon `jiang_gan|perceive|commanders_traitors`.
- **판정: 실패.** 잡음 제거는 5/5 로 성공했지만 실제 환각 5건이 사라졌고 새 실제 항목 2건이 생겼다. 어휘 주입 자체는 잡음에 듣는 것으로 보이나, 재추출 변동 때문에 "실제 환각 보존"을 이 표본으로 확인하지 못했다.

## H1 — 누락 사실 수리 패스 (40호출, 쌍당 haiku/medium 1 + sonnet/low K=3)
Phase 1 실패 중 missing ≥ 1 인 10쌍 전부(기대 약 8 → 실제 10; 호출 4×10). 프롬프트: 시스템은 영어 지시문(렌더 프롬프트 관례) + 한국어 백엔드는 대사·어투 규칙 꼬리를 그대로 붙임(문체 샘플은 뺌), 사용자 프롬프트에 기존 산문 / 빠진 행위를 렌더 프롬프트의 `## Acts` 줄 형식 그대로 / `Must not state or imply` 목록. 지시: "add only these facts with minimal edits…".

**사전 기준**: 수리된 렌더의 절반 이상 통과, 수리된 렌더에 실제 환각·누설 증가 없음.

| 쌍 | missing 전→후 | 환각 전→후 | 누설 | 글자 전→후 | 편집(단어 diff 글자) | 통과 |
|---|---|---|---|---|---|---|
| bible en-doyle | 1→0 | 1→0 | 0 | 954→985 | 31 | **✓** |
| bible gojeon | 2→1 | 3→4 | 0 | 376→376 | 0 (산문 동일) | ✗ |
| chibi en-doyle | 3→2 | 1→1 | 0 | 2952→2966 | 72 | ✗ |
| chibi gojeon | 2→2 | 2→1 | 0 | 830→830 | 0 (동일) | ✗ |
| chibi haera | 2→1 | 1→1 | 0 | 922→943 | 21 | ✗ (환각 잡음) |
| chibi hapsho | 1→1 | 1→1 | 0 | 962→982 | 28 | ✗ (환각 잡음) |
| macbeth en-doyle | 3→4 | 0→0 | 0 | 5884→5871 | 41 | ✗ |
| odyssey en-doyle | 2→2 | 0→0 | 0 | 3301→3410 | 181 | ✗ |
| odyssey gojeon | 3→2 | 1→2 | 0 | 1315→1349 | 42 | ✗ |
| odyssey haera | 2→2 | 0→0 | 0 | 1309→1309 | 0 (동일) | ✗ |

- **통과 1/10** (기준 5/10 미달). 누설 0건.
- 새 환각 분류(IR 대조): (1) odyssey gojeon `extra-act:telemachus|perceive|beggar_is_odysseus` — **실제**, 수리가 만든 것(빠진 assert 대신 perceive 로 씀). (2) bible gojeon `extra-act:mark_bystanders|assert|peter_with_jesus` — **실제**(IR 에서 mark_bystanders 는 말하지 않고 maid 가 그들에게 `peter_disciple` 을 말함)이지만 산문이 수리 전과 **글자 그대로 같아서** 수리가 만든 것이 아니라 재추출 변동이다. (3) chibi 3건의 환각 문장은 steal/flee 동의어 잡음(Phase 1 과 같은 유형), 새 것 아님. macbeth 는 missing 이 3→4(`thane_of_cawdor` 추가)로 악화.
- 수리 프롬프트를 받고도 3쌍(bible gojeon, chibi gojeon, odyssey haera)에서 haiku 가 산문을 한 글자도 바꾸지 않았다.
- **판정: 실패.** 통과 1/10, 수리가 만든 실제 환각 1건(odyssey gojeon).

## 편차(itemized)
1. H2 쌍 수는 8(잡음 5 + 실제 7 을 렌더 단위로 합침: chibi 4, odyssey gojeon, bible 3). 사전 기대 36호출 대신 24.
2. 실제 환각은 README 의 7건이 아니라 JSON 기준 6건으로 셌다(위).
3. H1 은 8 대신 10쌍(bible en-doyle 의 missing 1 포함, macbeth en-doyle 포함); 40호출.
4. H1 시스템 프롬프트는 문체 샘플을 뺀 영어 지시 + 한국어 어투 꼬리다. "백엔드 언어로 쓴 프롬프트"를 렌더 프롬프트 관례(영어 지시문)로 해석했다.
5. H2 첫 실행에서 예산 가드(`used+4>budget`) 때문에 odyssey gojeon 이 건너뛰어져 별도 호출 3회로 돌렸다(`-pairs` 스파이크 플래그 추가). 합계 호출에는 영향 없음.
6. 편집 크기는 단어 LCS 로 센 "추가·삭제된 단어의 글자 수 합"이다(거친 값).

## 한계
- n: H2 8 렌더(장면 3개), H1 10 렌더(장면 4개). 같은 장면의 백엔드끼리는 독립 표본이 아니다.
- 단일 실행. 재추출은 K=3 이어도 변동이 커서(같은 산문이 환각 3→4건) 개별 항목의 소실·출현은 표본 변동일 수 있다. 판정은 방향 증거일 뿐 반복 측정 아니다.
- H2 는 잡음 제거 5/5 라는 긍정 신호가 있으나 기준(실제 유지·새 항목 없음)을 충족하지 못해 "실패"로 적었다. 어휘 변형이 `extra-act` 에 어떻게 작용하는지는 따로 반복 측정이 필요하다.
