# Diegesis

소설을 위한 프로그래밍 언어. 인물·사실·장면·발화(`tells`, `lies`, `sees` …)를 `.dieg.md` 파일에 적으면
Go 컴파일러 `dieg` 가 이야기 IR 로 바꿔 PostgreSQL(Apache AGE + pgvector)에 적재한다.
판정(시간 모순, 누가 무엇을 아는가, 기만, 서사 패턴, 지표)은 전부 PG 안의 SQL 이 한다.
문법의 단일 기준은 `spec/grammar.ebnf`, 예제 코퍼스는 `corpus/`(macbeth chibi bible odyssey hamlet othello holmes).

## 준비물

- Go 1.27 이상
- Docker(compose v2)
- `dieg render`·`render-check` 를 쓰려면 렌더 백엔드(LLM CLI)

## PostgreSQL 띄우기

```sh
cd deploy
docker compose up -d pg            # 127.0.0.1:55432, 사용자/암호/DB = sl/sl/sl, 데이터는 볼륨 sl_pgdata
export SL_PG_DSN='postgres://sl:sl@localhost:55432/sl?sslmode=disable'
go run ./cmd/dieg db migrate       # db/migrations/*.sql 을 이름순으로 한 번씩 적용
```

마이그레이션은 추가만 한다(이미 적용된 파일은 고치지 않는다). 새 SQL 은 다음 번호의 파일로.

## 명령

```sh
go build -o dieg ./cmd/dieg
dieg check corpus/macbeth          # 구문·의미 검사(오류가 있으면 exit 1)
dieg test corpus/macbeth           # 컴파일·적재·린트·질의·패턴·변이 → expect.yaml 대로인지. 보고서 out/accept/macbeth.md
dieg patterns hamlet               # 서사 패턴 라이브러리(patterns/*.sql)를 작품 하나에
dieg stats macbeth                 # 지표(metric_*): 출석, 정보 비대칭, 긴장도, 아나크로니 …
dieg render-check corpus/macbeth   # 장면을 산문으로 렌더하고 다시 추출해 IR 과 비교(캐시 사용)
```

`dieg test` 는 변이 탐지율을 전체·move-before 제외·린트만(질의 탐지 제외, 보고용) 으로 찍는다.

## 아이템에서 소설까지: `dieg write`

```sh
dieg write idea.txt -o out/story1 [--rounds N] [--backend id] [--scenes N] [--no-render] [--styles dir]
dieg write "아이템 한 문장" -o out/story1
```

아이템(파일 또는 문장) → AI 가 Diegesis 코드를 쓴다 → `check`·`lint` 진단을 되먹임(최대 `--rounds`) → 깨끗해지면 장면마다 검증 렌더(렌더 + 재추출, 최대 3회 시도).
구독 `claude` CLI 만 쓴다(API 키 없음).

| 플래그 | 뜻 |
|---|---|
| `-o <dir>` | 출력 폴더(필수) |
| `--rounds N` | 코드 되먹임 최대 라운드(기본 4) |
| `--backend id` | 렌더 문체 |
| `--scenes N` | 앞 N개 보이는 장면만 렌더(호출 예산용) |
| `--no-render` | 코드만 쓰고 렌더하지 않는다 |
| `--styles dir` | 문체 폴더(기본 `styles`) |

출력: `story/`(Diegesis 코드), `novel/`(장면별 산문), `novel.md`(합친 소설), `calls.jsonl`(호출마다 토큰 기록), `diags.txt`(남은 진단).

종료 코드: `0` 코드가 깨끗하고 모든 장면이 검증 통과(`--no-render` 면 코드만 깨끗), `1` 라운드가 끝나도 error·warn 진단이 남음(warn 도 막는다 — 사물 위치·거짓 공개 린트 포함, 렌더하지 않음) 또는 입력·실행 오류, `3` 렌더한 장면 중 검증 실패가 있음(마지막 렌더는 소설에 들어간다). 장면은 환각 0 일 때만 통과한다(ADR-019).

## 측정 결과

직접 쓰기(A) 대 `dieg write`(B)는 [`examples/bench/README.md`](examples/bench/README.md) 에 있다.
짧은 아이템에서 B 는 캐시 가중 input 토큰이 A 의 2.7–2.8배라 "더 싸다"는 지지되지 않고, 일관성(오류 수)은 3:1 과 2:2 로 섞여 있다. 아이템 3개·판정 1회라 통계가 아니라 사례다.

## 대시보드(Grafana)

`deploy/.env` 에 두 비밀번호를 둔다(커밋하지 않는다):

```sh
SL_RO_PASSWORD=...      # 읽기 전용 역할 sl_ro 의 로그인 비밀번호
GF_ADMIN_PASSWORD=...   # Grafana admin
```

순서가 중요하다. `sl_ro` 는 마이그레이션(015·016)이 만들므로 **`dieg db migrate` 를 먼저** 돌린 뒤:

```sh
cd deploy && docker compose up -d   # ro-init 이 sl_ro 에 비밀번호를 걸고, grafana 가 뜬다
```

- Grafana 는 `127.0.0.1:3000` 에만 묶인다(익명 Viewer 가 켜져 있으므로 밖에 열지 않는다).
- Rancher Desktop(Windows)에서는 127.0.0.1 publish 가 호스트에서 닿지 않는다. 대신 `go run ./tools/dockerfwd deploy-grafana-1 3000 3300` 을 띄우고 http://127.0.0.1:3300/d/diegesis/diegesis 로 연다. 이 도구는 `docker exec` 로 연결을 이어 주므로 포트를 LAN 에 열지 않는다.
- 데이터소스는 `sl_ro` 로 붙는다. sl_ro 는 테이블 SELECT 와 대시보드 함수 목록(016)의 EXECUTE 만 가진다.
  SECURITY DEFINER 함수는 없다. 권한 검사는 `go test ./internal/dash/`(TestReadOnlyRole).

## 키워드

- 선언 id 로 쓸 수 없는 예약어: `when lasts enter exit mention beat render reveal withhold setup payoff discourse
  parent born dies all not to with because src major minor extra enacts` (`spec/grammar.ebnf` 머리말).
- `pattern`·`tag`·`attr` 는 문장 키워드다. 자리에 따라 키워드로 읽히니(`pattern dramatic-irony`, `fact x "X" tag identity`,
  `attr hamlet.age = 30`) 인물·사실 이름으로 쓰지 않는다.
  사물의 위치는 `attr bell.at = tower` 로 적는다(`X.at` 규약). 그 사물을 다루는 장면의 장소가 다르면 `object-location` 이 경고한다.
- `enacts` = 공연·재연. 무대 위 모두가 보지만 단언이 아니라 믿음을 만들지 않는다(예: 햄릿 3.2 극중극).

## 테스트

```sh
go test -count=1 ./...   # SL_PG_DSN 필요(PG 가 떠 있어야 한다)
go vet ./...
```

## 버전

[SemVer](https://semver.org/lang/ko/)(`MAJOR.MINOR.PATCH`)를 따른다. 1.0 전(0.x)에는 문법과 IR 이 MINOR 판에서 바뀔 수 있다. 판은 git 태그(`v0.3.0`)로 붙이고 `dieg version` 이 출력한다.

| 판 | 내용 |
|---|---|
| v0.1.0 | 플랜 1: 컴파일러, PG 판정, 린트, 렌더러 4종, 4편 합격 |
| v0.2.0 | 플랜 2: 정답 셋 3편, 시간 변이, 속성, 독자 모델, 패턴 9종, 지표, Grafana 대시보드 |
| v0.3.0 | 플랜 3: 거짓 사실 표시, 검증 렌더, 호출 계량(Meter), 사물 위치·거짓 공개 린트, `dieg write`, 직접 쓰기 대 Diegesis 측정 |
