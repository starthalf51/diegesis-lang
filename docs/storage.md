> 공개용 스냅샷(2026-11-09). 설계 기준은 프로젝트 설계 노트.

# 저장소 — PG 스키마와 AGE 그래프

> 원본은 관계형 테이블이다. AGE 그래프는 적재 후 테이블에서 다시 만드는 **투영**이다. 판정과 파생은 전부 PG 안에서 돈다. 코드: `db/migrations/` → [compiler](compiler.md)

## 이미지

- `deploy/postgres/Dockerfile`: `apache/age:release_PG18_1.8.0` 에 pgvector v0.8.7 을 소스 빌드해서 얹는다. 버전은 고정이다. AGE 1.8.0 은 PG18 전용이다.
- `000_extensions` 가 `age` · `vector` · `btree_gist` 를 켠다. 세션마다 age 를 미리 로드하고 `ag_catalog` 를 검색 경로에 둔다.
- 현재 pgvector 를 쓰는 테이블은 없다.
- 로컬 개발용 DSN 은 환경변수 `SL_PG_DSN` 으로 준다.

## 마이그레이션

번호만 늘린다. 이미 쓴 파일은 고치지 않는다(013 이 011·012 를 건드리지 않고 덧쓴 것이 그 예다). 예외 한 번: 003 은 2026-11-09 에 주석만 고쳤다(동작 변경 없음). `dieg db migrate` 가 적용한다.

| 번호 | 내용 |
|---|---|
| 000 | 확장, 세션 설정 |
| 001 | IR 테이블(`ir.Work` 슬라이스와 1:1)과 파생 테이블. 모든 행이 `work_id` 로 작품에 묶인다. 작품 삭제가 하위 행을 지운다 |
| 002 | 시간: `solve_time`, `point_dist`, `precedes`, 음수 사이클 진단 |
| 003 | 역할 보유 구간(`resolve_roles`), 지식 파생(`derive_beliefs`), `act_content` 뷰 |
| 004 | AGE 투영(`project_graph`)과 `derive` 진입점 |
| 005 | 린트 공통 조각(`lint_canon` · `lint_known` · `lint_opposite` · `lint_short` 등) |
| 006 | 의심(doubts): 믿음 행의 `until_pt` |
| 007 | 기만: 글로 한 거짓말과 읽은 문서의 의심 |
| 008 | 별칭·부정 해석 수정, derive 진단의 waiver 대상, 그래프 BELIEVES 의 `until` |
| 009 | `render_scene`: 렌더 요청 JSON |
| 010 | `attribute` 테이블(엔티티 속성) |
| 011 | 독자 모델 |
| 012 | 패턴: `pattern_decl` · `pattern_match` |
| 013 | 독자 모델 보정(`reader_lies`), `pattern_result` 뷰 |
| 014 | 지표 `metric_*` · `graph_*` |
| 015 | 읽기 전용 역할 `sl_ro` |
| 016 | `sl_ro` 권한 재정리: 대시보드용 함수는 전부 INVOKER, 실행권은 이름 목록에만 |

## 테이블

시간은 작품 원점에서 센 `int8` 초다. 모든 행은 `src` (파일·줄·열)를 참조한다.

| 묶음 | 테이블 |
|---|---|
| 구조 | `work` · `module` · `src` · `entity` · `group_member` · `name` · `role` · `role_holder` · `role_span` · `parent` · `distance` |
| 시간 | `point`(`earliest`/`latest` 는 풀이 결과) · `time_edge` · `point_dist` · `scene` · `presence` |
| 행위·지식 | `act` · `audience` · `belief`(선언) · `believes`(파생, `until_pt` 포함) · `uses` · `deception` · `fact`(`tag` 포함) · `artifact` · `account` |
| 독자 | `reveal` · `withhold`. 독자 믿음은 테이블이 아니라 `reader_*` 함수가 낸다 |
| 복선·패턴 | `setup` · `payoff` · `pattern_decl` · `pattern_match` |
| 속성 | `attribute`(kind: int / str / id, `at_pt` NULL = 처음부터 참) |
| 진단 | `diagnostic` · `waiver` · `directive` |

설계 초안과 달라진 것:

- 쌍별 거리 테이블 `transition` 은 없다. `point_dist` 가 모든 점 쌍의 최단 거리를 담는다.
- `presence` 는 바깥 경계(`from_pt`/`to_pt`)만 담는다. 필연 구간은 린터가 계산한다. 같은 시간에 두 장소를 막는 배타 제약은 스키마에 없고, 규칙이 판정한다.
- 배타 제약은 `role_holder` 하나뿐이다(한 역할에 동시에 두 보유자. `shared` 역할은 예외).
- 장면 임베딩 테이블은 만들지 않았다. 렌더 지시는 `directive` 가 맡는다.

## 시간

- `time_edge` 한 행은 `min ≤ t_to − t_from ≤ max` 다. NULL 경계는 간선이 없다는 뜻이다.
- `solve_time` 이 `point` 의 `earliest/latest` 를 채운다. 원점에서의 최단 거리가 `latest`, 역방향이 `earliest` 다.
- `point_dist` 는 모든 점 쌍의 최단 거리(STN 전체 쌍)다. 상대 거리가 필요한 판정(이동 시간, 정보 획득과 사용의 순서, 속성의 경과 시간)이 이걸 읽는다. 계산량은 O(V²) 다.
- `precedes(a, b)` 는 확정 / 가능 / 불가를 돌려준다. 확정이 아니면 린트 등급을 info 로 낮춘다.
- 음수 사이클과 순서 순환은 `time-cycle` 진단으로 나온다. 순환을 만든 `when`/`born`/`dies` 문장이 waiver 대상이다.
- 절대 시각이 없으면 `earliest=0, latest=NULL` 이다. 지표는 이 점을 이야기 시간으로 치지 않는다.
- 장면·사건 길이는 `lasts a..b` 가 시작→끝 간선으로 넣는다.

## 지식과 독자

- `believes` 는 `derive_beliefs` 가 낸다. 발화·지각·읽기·선언에서 한 행씩 나오고 원인 사건(`cause_act`)이 붙는다.
- **doubts**: `X doubts P` 는 그 시점 이전에 *들어서*(단언 발화 청중 행) 얻은 믿음 행을 거둔다. 행의 `until_pt` 에 가장 이른 의심 시점이 들어간다. 지각·읽기·명시적 believes 는 거두지 않는다. 시점 없는 배경 doubts 는 아무것도 거두지 않는다.
- **deceit**: 의심은 읽은 문서에서 얻은 믿음도 거둔다. 쓴 사람이 쓰는 시점에 ¬P 를 확정적으로 알면 그 문서의 P 는 거짓말이고 `deception` 행이 생긴다. 위조 서신이 이 경로다.
- **독자 모델**: 독자는 *보이는* 장면의 단언·지각·방백·생각을 그 장면의 `discourse_ord` 에 받는다. 들은 대로 믿는다(거짓말도). 명제는 (기저 사실, 값)으로 정규화하고, 나중에 반대 값이 공개되면 그 위치에서 갱신한다. `withhold P until X` 는 X 가 나오는 위치 전까지 공개를 막는다.
- **독자와 거짓말**: 이미 거짓인 줄 아는 기만(거짓말, 위조 문서를 읽음)은 독자가 믿지 않는다. 앞선 위치에 기만이 아닌 공개로 반대 값이 있었으면 뺀다.
- 인물 지식은 이야기 시간을, 독자 지식은 서술 순서(`discourse_ord`)를 따른다. 지표의 x 축은 후자다.

## 패턴

- 패턴 정의는 `patterns/*.sql` 이다. 파일 하나가 패턴 하나이고 본문은 `$1` = 작품인 SELECT 다. 러너(`internal/pg/patterns.go`)는 결과를 `pattern_match` 에 옮겨 담기만 한다. 판정은 패턴 SQL 과 독자 모델이 한다.
- 작가 선언 `pattern <id> [at <장면>]` 은 `pattern_decl` 이다. 선언했는데 매치가 없으면 `pattern-missing`(warn). 매치만 있고 선언이 없으면 보고만 한다.
- `pattern_result` 뷰는 가장 최근 실행의 매치를 선언과 합쳐 보여 준다. 지표와 `dieg patterns` 가 이걸 읽는다.
- 목록과 의미 → [checks](checks.md#패턴-라이브러리-patternssql-9개)

## 지표 (014)

모든 함수는 `(work text)` 를 받고, 빈 작품이면 0행이다. x 축은 보이는 장면의 `discourse_ord` 다.

| 함수 | 내용 |
|---|---|
| `metric_presence` | 장면별 출석과 행위 수 |
| `metric_asymmetry` | 장면에 있는 인물이 모르는(또는 반대로 믿는) 참인 사실의 수 |
| `metric_open_setups` | 열린 복선 수 |
| `metric_active_deceptions` | 진행 중인 기만 수 |
| `metric_tension` | 열린 복선 + 활성 기만 + 그 위치를 덮는 dramatic-irony·mystery 매치를 항별 최댓값으로 정규화해 가중합. 가중치는 `metric_weights` 테이블 |
| `metric_anachrony` | 서술 순서 대 이야기 시간. 시각이 제약되지 않은 점은 제외한다 |
| `metric_pace` | 장면의 이야기 시간 폭(초) 대 행위 수. 시각이 없는 점이 끝이면 폭은 NULL |
| `graph_nodes`, `graph_edges` | 인물 그래프. 간선 = 함께 출석한 장면 수 + TOLD 수 |

- `metric_tension` 의 패턴 항은 `dieg patterns` 를 먼저 돌려야 0 이 아니다.
- `metric_anachrony` 는 절대 시각이 없는 작품에서 0행이다. 오류가 아니다.

## 읽기 전용 역할 sl_ro (015, 016)

- 대시보드(Grafana) 용이다. 마이그레이션에서는 `NOLOGIN` 으로만 만들고, 비밀번호는 `deploy/compose.yaml` 의 `ro-init` 서비스가 `ALTER ROLE` 로 건다(비밀번호를 저장소에 두지 않는다).
- 테이블 `SELECT`, 함수는 이름 목록(`metric_*` · `graph_*` · `lint_short` 와 그 함수들이 부르는 읽기 전용 헬퍼)의 `EXECUTE` 만 준다. 모든 함수는 `SECURITY INVOKER` 이고, 동적 SQL 을 만드는 헬퍼는 실행권이 없다.
- 한계: 부여는 현재 테이블 기준이다. 새 테이블은 기본 권한에 따라 SELECT 만 따라간다.

## AGE 그래프

`derive` 의 마지막 단계(`project_graph`)가 테이블을 읽어 작품마다 그래프 하나(`g_<work>`)를 새로 만든다.

| 정점 | 간선 |
|---|---|
| `Entity` · `Point` · `Fact` · `Setup` | `BEFORE`(시점 순서) · `TOLD`(화자→수신자) · `BELIEVES`(`from`, `until`) · `PRESENT_AT` · `CAUSES` · `FORESHADOWS` · `PARENT_OF` / `SPOUSE_OF` · `HOLDS` |

- 그래프를 쓰는 곳은 경로가 답인 질의(전달 경로, 순서 도달 가능성, 족보, 인과 사슬)다. 그 밖의 검사는 SQL 이다.
- cypher 질의의 그래프 이름이 상수(`g_chibi` 등)다. 작품 id 를 받는 래퍼는 아직 없다.

## 비용

전부 로컬 Docker Compose(PG + Grafana)다. 돈이 드는 곳은 렌더러와 쓰기 파이프라인의 LLM 호출뿐이다.
