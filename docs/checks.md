> 공개용 스냅샷(2026-11-09). 설계 기준은 프로젝트 설계 노트.

# 검사 — 층위 배치, 린터, 패턴

> 원칙: **오류**는 IR이 의미를 잃는 경우에만 낸다. 이야기 안의 모순은 전부 **차이(warn)** 나 **보고(info)** 로 낸다. 판정은 PG 안의 SQL이다. → [storage](storage.md)

## 층위 배치

| 검사 | 층위 | 등급 |
|---|---|---|
| 구문(`syntax`) | 파서 | error |
| 정의되지 않은 참조(`undefined-ref`), 동사 인자 종류(`arg-kind`), 알 수 없는 역할(`role-unknown`) | 컴파일러 resolve · types | error |
| import 순환(`import-cycle`) | 컴파일러 resolve | error |
| 프론트매터와 블록에 같은 정보(`dup-decl`) → [file-format](file-format.md) | 컴파일러 desugar | error |
| 억제의 대상·사유 오류(`waiver-target`, `waiver-reason`) | 컴파일러 | error |
| 없는 사람의 퇴장(`offstage-exit`) | 컴파일러 presence | warn |
| 한 역할의 보유 구간 겹침 `role-overlap`, 화자가 모호함 `role-ambiguous` | PG `resolve_roles` (derive 진단) | warn |
| 시간 제약 순환 `time-cycle` | PG `solve_time` (derive 진단) | warn |
| 한 역할에 동시에 두 보유자 | DB 배타 제약(`role_holder`) | error |
| 나머지 이야기 모순 | 린트 규칙 `rules/*.sql` | warn / info |
| 서사 패턴 | `patterns/*.sql` | 보고 (선언 누락만 warn) |

`stale-prose` · `rendered-edited` (렌더 산문이 낡았거나 손으로 고쳐졌음)는 설계에만 있고 구현하지 않았다.

## 린트 규칙 (`rules/*.sql`, 19개)

18개는 `layer: lint`(린터가 실행)이고, `time-cycle` 하나는 `layer: derive` 다(derive 가 이미 낸 진단을 보여 줄 뿐 린터는 실행하지 않는다).

| 규칙 | 등급 | 대상 | 의미 |
|---|---|---|---|
| `info-flow` | warn | act | 인물이 그 전에 알게 된 적 없는 명제를 말한다 |
| `letter-unknown-info` | warn | act | 쓴 사람이 쓰기 전에 믿지 않은 명제가 문서에 담겼다 |
| `because-unbelieved` | warn | act | `because P` 인데 그 시점에 행위자가 P 를 믿지 않는다 |
| `belief-contradiction` | warn | act | 직접 지각한 사실과 반대되는 명제를 진실하게 주장한다 |
| `undelivered-message` | warn | act | `sends m to t with P` 의 전갈이 t 에게 전해지지 않았다 |
| `dead-actor` | warn | act | 죽은 인물이 말하거나 행동한다 |
| `stays-gone` | warn | act | 이야기에서 떠났다고 선언된 인물이 다시 행동하거나 등장한다 |
| `object-location` | warn | act | 사물이 있다고 선언된 곳과 다른 장소의 장면에서 그 사물을 다룬다 |
| `retell-known` | warn | act | 청자가 이미 믿는 사실을 같은 화자가 같은 말로 다시 알려 준다 |
| `payoff-without-setup` | warn | payoff | 복선이 없거나 회수보다 앞서지 않는다 |
| `unpaid-promise` | warn | setup | promise 복선이 회수되지 않았다 |
| `attribute-conflict` | warn | attribute | 같은 엔티티의 같은 속성이 맞지 않는 값으로 단언된다 |
| `fact-conflict` | warn | fact | 같은 술어·같은 첫 인자인데 나머지 인자가 다른 참 사실이 둘 있다 |
| `relative-time-fact` | warn | fact | 사실 텍스트에 상대 시간("열이틀 뒤")이 박혀 있다 |
| `reveal-false` | warn | reveal | 거짓으로 선언된 사실을 독자에게 공개한다 |
| `pattern-missing` | warn | pattern | 작가가 선언한 패턴이 검출되지 않았다 |
| `account-variance` | info | act | 같은 사건의 기록(account)끼리 담긴 사실이 다르다 |
| `inferred-deception` | info | act | 쓴 사람이 거짓인 줄 알고 쓴 것으로 판정된 문서 — 작가의 실수인지 확인하라 |
| `time-cycle` | warn | when | 시간 제약이 순환한다 (derive) |


- `info-flow` · `letter-unknown-info` · `because-unbelieved` 는 같은 검사를 대상 행위 종류로 나눈 것이다. 한 결함에 진단 하나가 나온다. `lies` 와 거짓 문서는 이 검사에서 빠진다(기만이 정보 흐름 위반이 아니므로).
- 앎 판정은 `lint_known` 이 한다. 의심으로 거둔 믿음(`until_pt`)은 알지 못하는 것으로 본다. 순서가 확정되지 않았으면(`precedes` = 가능) info 로 낮춘다.
- `attribute-conflict` 는 정적 충돌(예: 부상 부위 shoulder 대 leg)과 나이 같은 수치 속성의 경과 시간 모순을 본다. 나이는 `point_dist` 로 시점 간 간격의 상한·하한을 따로 따져서, 한쪽 경계만 있어도 판정한다.

## 규칙 파일 형식

규칙은 `rules/` 에 두고 바이너리에 내장한다(`rules/embed.go`). 파일 하나가 규칙 하나다.

```sql
-- rule: letter-unknown-info
-- severity: warn
-- layer: lint | derive
-- target: act            (억제 지시어를 어디에 붙이는지)
-- summary: …
SELECT … AS target, …
```

- 러너는 결과 행마다 `target` 의 `src` 를 붙여 `diagnostic` 에 넣고, `waiver` 와 대조해 억제 여부를 표시한다.
- 장르별 하위 폴더 로딩과 `--rules` 플래그는 구현하지 않았다.

## 패턴 라이브러리 (`patterns/*.sql`, 9개)

패턴은 위반이 아니라 **구조의 검출**이다. 파일 하나가 패턴 하나이고, 본문은 `$1` = 작품인 SELECT 다. `dieg patterns` 가 결과를 `pattern_match` 에 적재하고 `pattern_result` 뷰로 본다 → [storage](storage.md#패턴)

| 패턴 | 층 | 의미 |
|---|---|---|
| `chekhov-gun` | foreshadow | promise 복선이 회수된다 |
| `prophecy-fulfilled` | foreshadow | 예언자가 단언한 명제가 그때는 참이 아니었다가 나중에 참이 된다 |
| `dramatic-irony` | information | 독자는 참인 P 를 알고, 같은 장면의 인물은 P 를 모르거나 반대로 믿는다(장면 단위, 장면에 있는 인물만) |
| `mystery` | information | 참인 P 가 독자에게 withhold 되어 있고, 그동안 P 를 묻거나 의심하는 장면이 있다 |
| `red-herring` | information | 독자가 거짓 사실을 한동안 믿다가 나중에 반대를 받는다 |
| `reversal-reveal` | information | 독자가 믿던 사실이 뒤집히고, 그것이 복선이나 행동의 근거였다 |
| `deception-chain` | deception | 기만 → 수신자가 믿는다 → 그 믿음을 `because` 로 쓴 행동 |
| `counter-deception` | deception | 기만의 수신자가 의심하거나 이미 반대를 믿는다 — 속지 않는다 |
| `recognition` | identity | `tag identity` 사실을 믿는 인물 집합이 서술 순서대로 커지고 적어도 한 번은 `sees` 로 늘어난다 |

- 작가가 `pattern <id> [at <장면>]` 으로 선언하면 매치를 기대한다. 없으면 `pattern-missing`.
- 코퍼스마다 기대 패턴 수가 `expect.yaml` 에 있고 `dieg test` 가 대조한다.
- 알려진 한계: 일부 패턴은 상태 술어 제외나 순서 검사가 느슨하다(reversal-reveal, counter-deception 의 의심 기간 무제한, `pattern_covers` 가 긴 구간에서 느슨함).

## 억제 (nolint)

억제 지시어는 바로 다음 문장에 붙는다. 사유는 필수다.

<!-- check -->
```sl
module demo.waiver

work demo
  unit day
character anna "Anna" major
character ben "Ben" minor
place hall
fact door_locked "The cellar door is locked" true

scene 1 at hall
  when after start
  enter anna, ben
  ben sees door_locked
  #! allow belief-contradiction: "의도된 모순 — 벤이 잘못 본 것이다"
  ben says anna: not door_locked "It is open." src 1.1
  exit all
```

- 사유가 비어 있으면 컴파일 오류(`waiver-reason`)다.
- 억제된 진단도 지우지 않고 `waived=true` 로 남긴다. 그래서 "이 작품의 의도된 불일치 목록"이 하나의 산출물이 된다. `dieg test` 는 선언되지 않은 억제를 실패로 본다.
- 작품 단위 억제는 `work` 블록의 `allow rule-id: "…"` 다.
- 한계: `when` 억제는 장면 전체가 대상이다.

## 평가 — `dieg test`

코퍼스 하나가 PASS 이려면: 컴파일 오류 0, 기대 밖 경고(오탐) 0, 선언 안 된 억제 0, 기대 진단·기대 패턴·선언된 질의 일치, 변이 탐지율이 기준 이상이어야 한다. 기대값은 코퍼스의 `expect.yaml` 에 있고, 도구 출력에 맞춰 고치지 않는다.

### 변이 시험

원본 포팅본에 오류를 하나씩 주입하고, 그 오류를 잡는 규칙이나 선언된 질의가 반응하는지 본다. 연산자 9개:

| 연산자 | 기대하는 규칙 |
|---|---|
| `drop-enter` | info-flow 계열 |
| `drop-tells` | undelivered-message, info-flow 계열 |
| `move-before` | info-flow 계열 |
| `flip-perceived` | belief-contradiction, info-flow 계열 |
| `drop-setup` / `drop-payoff` | payoff-without-setup / unpaid-promise |
| `kill-early` | dead-actor |
| `shift-duration` / `swap-order` | time-cycle, dead-actor, info-flow 계열 (+ 선언된 질의 불일치) |

- **합격 기준**: 변이 전체 탐지율 ≥ 0.8, **move-before 를 뺀 탐지율 ≥ 0.7**. 이 둘이 게이트다.
- 탐지는 변이 지점 근처(변이한 행위·문장·장면·그 명제를 쓰는 행위, 시간 변이는 변이한 시점을 끝점으로 하는 간선의 다른 끝)의 새 warn 만 센다. 작품 전체의 같은 규칙 경고를 부수 탐지로 세지 않는다.
- 질의 탐지: 변이로 `expect.yaml` 질의 결과가 바뀌면 지점과 무관하게 탐지로 센다. 진단을 읽는 질의는 제외한다.
- 한계: 시간 변이의 일부는 규칙이 아니라 질의가 잡는다. `dieg test` 는 린트 단독 탐지율을 따로 보고하지 않는다.
- 변이가 이야기를 바꾸지 않는 경우(동등 변이)는 사람이 표시해서 집계에서 뺀다.

## 정답 셋 (알려진 정합성 이슈)

원작 텍스트 근거로 포팅했고, 결함 단서 문장을 지우면 진단이 사라지는 음성 대조까지 확인했다.

| 작품 | 이슈 | 잡는 규칙 |
|---|---|---|
| 햄릿 | 1.2 의 학생(20세) 대 5.1 의 30세 | `attribute-conflict` |
| 오셀로 | 이중 시간 | `time-cycle` |
| 홈즈 | 왓슨 부상 부위 shoulder 대 leg | `attribute-conflict` |
| 홈즈 | 왓슨 이름 John 대 James | `attribute-conflict` |
| 맥베스 | 코더 영주의 반역 | `belief-contradiction` (선언된 억제 1건) |
