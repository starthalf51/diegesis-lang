> 공개용 스냅샷(2026-11-09). 설계 기준은 프로젝트 설계 노트.

# 컴파일러 구조

> Go로 쓴다. 파서 생성기 없이 손으로 쓴 재귀 하강 파서다. 컴파일러는 파싱·이름 해석·타입 검사·원시 사실 방출까지만 한다. 시간 풀이, 지식 파생, 린트, 패턴, 지표는 PostgreSQL이 한다. → [storage](storage.md)

## 파이프라인

```
.dieg.md 파일들
  │ split      프론트매터 / ```sl 블록 / 산문. 원래 줄 번호를 보존한다 (internal/syntax)
  │            (펜스는 0열에 있는 것만 인식한다)
  │ desugar    프론트매터 → 언어 문장 AST. 대응표는 file-format.md
  │            블록과 같은 사실이 겹치면 dup-decl 오류
  │ lexer      들여쓰기 → INDENT/DEDENT. 토큰마다 file:line:col
  │ parser     재귀 하강. 오류가 나도 줄 단위로 복구해서 오류를 전부 모은다
  ▼ AST
  │ sema (internal/sema)
  │   resolve  import 순환 검출(import-cycle), 이름 해석: 엔티티·장면·비트·복선 ID, `@` 시점, 모듈 간 참조, 별칭
  │   types    동사별 인자 종류, 선언 종류(예: `tag` 는 fact 만)
  │   presence 장면 출석 경계 계산: 없는 사람의 퇴장 등
  │   emit     시점·시간 간선·행위·믿음 선언·독자 지시를 IR 레코드로 낸다
  ▼ IR (JSON, internal/ir — 스키마는 PG 테이블과 1:1)
  │ load       PG에 한 트랜잭션으로 적재(작품 단위 교체) → derive 호출
  ▼
PostgreSQL  derive: solve_time · resolve_roles · derive_beliefs · project_graph(AGE)
            └ lint (rules/*.sql) · patterns (patterns/*.sql) · metric_* · render_scene
```

- 컴파일 단계는 DB가 필요 없다. `dieg check` 는 몇 초 안에 끝나서 LLM 포팅 루프(생성 → 오류 → 수정)가 빨리 돈다.
- 파일 확장자는 `.dieg.md`, 코드 펜스 태그는 `sl` 이다.
- 빌드마다 `work` 선언이 하나 있어야 한다(시간 원점을 정의한다). 없으면 `no-work` 오류다.

## 시간 모델

**STN**(단순 시간 네트워크)으로 표현한다. 컴파일러는 제약을 `time_edge`(`min ≤ t_to − t_from ≤ max`)로 낼 뿐이고, 풀이는 PG의 `solve_time` / `point_dist` 가 한다.

| 소스 | 제약 |
|---|---|
| `when after X by a..b` | `a ≤ t − X.end ≤ b` |
| `when before X` | `t.end ≤ X.start` |
| `when at 208-11` | 원점 기준 절대 구간 |
| `lasts a..b` | 장면·사건·기록의 시작→끝 간선 `[a, b]`. 날짜 없는 작품에서 경과 시간의 상한을 줄 수 있는 유일한 문장 |
| 장면 안 문장 순서 | `X.start ≤ beat1 ≤ beat2 ≤ X.end` |
| `born` / `dies` | 생애 간선(태어남 ≤ 죽음) |
| `distance p q a..b` | 이동 검사의 입력 (`distance` 테이블) |

- 모순(음수 사이클)은 `time-cycle` 진단이다. 억제하면 그 간선은 풀이에서 빠지고 진단만 남는다. 이중 시간(오셀로)은 `time-cycle` 이 기대 진단이다.
- 절대 시각이 없는 작품은 `latest` 가 ∞ 다. 그래서 상대 거리가 필요한 판정은 `point_dist` 를 쓴다.

## 지식 파생 (PG)

| 문장 | 수신자 | 남는 레코드 |
|---|---|---|
| `A tells B: P` | B. 그 자리에 있던 나머지도 엿듣는다(`whispers` 면 B만) | B와 출석자 believes P, 원인은 이 발화 |
| `A says: P` | 출석자 전원 | 같다 |
| `A lies B: P` | 같다 | 수신자 believes P, `deception` 행 |
| `A aside` / `thinks` | 독자만 | 독자 공개 |
| `A sees …` | A | 지각에 의한 믿음 |
| `A reads art` | A | 문서에 담긴 명제 전부 |
| `A writes art` | — | `uses(A, P)`. 쓴 사람이 쓰기 전에 P 를 믿었어야 한다. ¬P 를 확정적으로 알면 `deception` 행 |
| `A does X because P` | — | `uses(A, P)` |
| `X doubts P` (시점 t) | — | 들은·읽은 P 의 믿음 행을 t 부터 거둔다(`until_pt`) |
| 화자의 진실한 발화 | — | `uses(화자, P)` |

판정은 린터가 한다. 파생은 believes 와 uses 를 만들 뿐이다.

## 언어 기능

| 문장 | 의미 | 판정 |
|---|---|---|
| `attr <개체>.<키> = <값> [src …]` | 속성. 값은 수·문자열·ID. 시점에 묶으면 시간에 따라 바뀐다 | `attribute-conflict` |
| `lasts a..b` | 길이 제약 | `time-cycle` |
| `fact … tag <이름>` | 사실 분류. `identity` = 정체 사실 | `recognition` 패턴이 고른다 |
| `pattern <id> [at <장면>]` | 작가 선언: 이 서사 패턴이 있다 | `pattern-missing` |
| `reveal P [at X]` / `withhold P until X` | 독자 공개 제어 | 독자 모델 |
| `enacts` | 공연·재연(극중극): 단언이 아니다 | 믿음 파생 없음 |

문법 전체는 [`spec/grammar.ebnf`](../spec/grammar.ebnf). `attr`, `pattern` 은 예약어가 아니다(다음 단어가 동사·believes·doubts 면 같은 이름의 행위·믿음이다).

속성과 시간 제약을 함께 쓰는 예:

<!-- check -->
```sl
module demo.attr

work demo
  unit day
character hamlet "Hamlet" major
place castle

scene 1 at castle
  when after start
  lasts 1h..3h
  enter hamlet
  attr hamlet.age = 20 src 1.2
  exit all
scene 2 at castle
  when after 1 by 300d..400d
  enter hamlet
  attr hamlet.age = 21 src 2.1
  exit all
```

## 오류를 보는 법

진단 형식은 `file:line:col: severity rule: 메시지` 다. 예를 들어 위 예제에서 `hamlet` 을 `hamlt` 로 틀리게 쓰면 `dieg check` 가 `error undefined-ref: undefined actor "hamlt"` 를 낸다. 오류가 하나라도 있으면 종료 코드 1이다.

## IR

- 정규화된 JSON 하나. 테이블 단위 배열로 [storage](storage.md) 스키마와 1:1이다. 행마다 `src: {file, line, col}` 이 붙는다.
- 안정 ID: `작품/모듈/로컬ID`. 파일을 재배치해도 바뀌지 않는다.
- 테스트: 컴파일러 단위 테스트는 PG 없이 돌고, PG 판정 테스트는 Docker PG가 필요하다(`internal/pg`).

## CLI (`dieg`)

```
dieg db migrate                         마이그레이션 적용
dieg check <dir> [--json]               파싱과 의미 분석만. DB 불필요
dieg build <dir> -o ir.json
dieg load  <ir.json|dir>                PG 적재 (derive 포함, AGE 투영 포함)
dieg lint  <work|dir|ir.json> [--json]  rules/*.sql 실행
dieg patterns <work|dir> [--json]       patterns/*.sql 실행, pattern_match 갱신
dieg stats <work> [--metric name] [--json]   metric_* 조회
dieg test <corpus-dir> [--seed N] [--mutants N]   합격 판정: 기대 진단·질의·패턴 + 변이 시험
dieg render <work|dir> <scene> --backend <id> [--verify] [-o file]   → rendering.md
dieg render-check <corpus...> [--force | --force-extract] [--only <work>/<scene>.<backend>]
dieg write / continue / revise / stamp  아이디어 → 코드 → 소설 파이프라인 (LLM 호출)
```

- 진단은 `--json` 이면 같은 내용을 배열로 낸다.
- 설계에는 있었으나 구현하지 않은 것: `explain`, `fmt`, `show`, `verify-prose`, `import/export story-skills`, `lint --rules`(규칙은 `rules/` 에 `embed.go` 로 묶여 있다).

## 안 하는 것

- 증분 컴파일 — 작품 하나를 통째로 컴파일한다.
- LSP — 진단 형식이 `file:line:col` 이라 나중에 얇은 래퍼만 씌우면 된다.
