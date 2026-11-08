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
| `--force` | (write) 이미 `<out>/story` 가 있어도 지우고 새로 쓴다. 없으면 write 는 거부하고 continue/revise 를 권한다 |
| `--styles dir` | 문체 폴더(기본 `styles`) |

출력: `story/`(Diegesis 코드), `novel/`(장면별 산문), `novel.md`(합친 소설), `calls.jsonl`(호출마다 토큰 기록), `diags.txt`(남은 진단).

종료 코드: `0` 코드가 깨끗하고 모든 장면이 검증 통과(`--no-render` 면 코드만 깨끗), `1` 라운드가 끝나도 error·warn 진단이 남음(warn 도 막는다 — 사물 위치·거짓 공개 린트 포함, 렌더하지 않음) 또는 입력·실행 오류, `3` 렌더한 장면 중 검증 실패가 있음(마지막 렌더는 소설에 들어간다). 장면은 카드 채점 통과(누출 0, 누락+종류 틀림 ≤ 1)일 때 통과한다(ADR-028. `--verifier open` 이면 환각 0, ADR-019).

## 이어 쓰기와 수정: `dieg continue` · `dieg revise`

```sh
dieg continue out/story1 --ep 2 [--brief "다음 화 방향" | --brief-file f] [--rounds N] [--scenes N] [--no-render]
dieg revise out/story1 "수정 요청 한 문장" [--rounds N] [--scenes N] [--no-render]
```

- `continue`: `dieg write -o <dir>` 로 만든 폴더에 다음 화(`story/epNN/`)를 더한다. AI 는 앞 화 전문이 아니라 PG 가 만든 **상태 요약**(참인 사실, 누가 무엇을 믿는지, 독자에게 아직 숨긴 것)을 입력으로 받는다. 과거 화 파일은 바뀌지 않는다(기존 경로를 건드린 묶음은 거부). 새 화의 장면만 렌더한다.
- `revise`: 요청대로 `story/` 를 고친다(파일 삭제 가능). 복사본에서 검사·린트 루프를 돌고, 깨끗할 때만 `story/` 를 원자적으로 바꾼다. 렌더 입력이 바뀐 장면과 새 장면만 다시 렌더한다(장면별 해시를 디스크에 둔다).
- 두 명령 모두 `write` 와 같은 모델·효과 플래그를 받는다. 종료 코드도 같다.

## 모델·효과·뼈대 모드

| 플래그 | 뜻 |
|---|---|
| `--code-model M` | 코딩 모델(기본 `claude-opus-5-5`) |
| `--render-model M` | 렌더 모델(기본 `claude-haiku-5-5`, 근거 `examples/renderer-ab`) |
| `--extract-model M` | 재추출 모델(기본 `claude-sonnet-5-5`) |
| `--code-effort e` | 코딩 추론 효과 `low`/`medium`/`high`(기본 `medium`) |
| `--render-effort e` | 렌더 추론 효과 `low`/`medium`/`high`(기본 `low`, 근거 `examples/effort-ab`) |
| `--verifier card\|open` | 장면 채점기(기본 `card`, `open` 은 예전 개방형 추출) |
| `--skeleton` | 뼈대 모드: 코드는 대사를 쓰지 않고 렌더러가 대사를 쓴다 |

- 없는 모델 ID·효과·렌더러 값은 플래그 파싱 단계에서 exit 2 로 끝난다(호출 전, 조용히 기본값으로 바뀌지 않는다).
- 뼈대 모드는 기본 켜짐, `--skeleton=false` 로 끈다. `Scene.FreeDialogue`(코퍼스 렌더 경로)는 옵트인이라 코퍼스 장면의 렌더 프롬프트·캐시는 바이트 단위로 그대로다. 실행 사이에 모드를 바꾸면 모든 장면의 해시가 바뀌어 전부 다시 렌더된다.
- **동작 변경:** `dieg render --verify` 의 기본 모델이 sonnet 이 되었다(`--render-model`·`--extract-model` 로 바꾼다).
- **동작 변경(플랜 5):** 기본 렌더 모델이 haiku 가 되었다(`write`·`continue`·`revise`·`render --verify` 모두. 렌더러 A/B, `examples/renderer-ab`). 재추출은 sonnet 그대로다.
- **동작 변경(플랜 5, 검증기):** `ExtractHash` 가 바뀌어 다음 `dieg render-check` 는 캐시된 장면을 전부 다시 추출한다(장면당 K=3, 코퍼스 16장면이면 약 48회 호출). 산문 캐시는 유효하고 추출 캐시만 무효다.
- **동작 변경(플랜 6):**
  - 생성 경로(`write`·`continue`·`revise`·`render --verify`)의 기본 채점기는 체크리스트 카드(`render.ScoreCard`)다. `--verifier open` 으로 예전 개방형 추출 검증을 쓴다. `render-check` 는 그대로 개방형(K=3)이다.
  - 통과 = 누출 0, 누락 + 종류 틀림 ≤ 1, 채점 오류 없음. 지어낸 사건(G2)은 경고(`warnings`)로만 남고 재시도 피드백에 들어가지 않는다. 따라서 카드 통과는 "환각 0" 이 아니다(ADR-028).
  - `--render-effort` 기본이 `low` 가 되었다(`write`·`continue`·`revise`·`render --verify`, `examples/effort-ab`: 점수 0.9947→0.9997, 통과 6/6 동일, 호출당 출력 5124→4643 토큰. 토큰 이득은 macbeth 한 장면에 기댄다, n=6).
  - 계층형 렌더러를 삭제했다: `--renderer`·`--beat-retries` 는 없다(exit 2). 예전 `layered:` 장면 해시는 낡아 다시 렌더된다(ADR-029).
- 비용 측정: 뼈대 모드만으로는 코드 출력 토큰이 약 2% 줄었다(출력의 대부분이 추론 토큰). `--code-effort medium` 은 high 대비 코드 출력이 약 55% 적었고 깨끗해지는 라운드 수는 늘지 않았다(스모크 1회 기준).
- **동작 변경(플랜 7):**
  - 생성 경로(`write`·`continue`·`revise`)는 렌더 프롬프트 끝에 PG 가 만든 **세계 상태 요약**(`## 이 시점의 세계 상태 (어기지 말 것)`)을 붙인다. "독자가 이미 아는 것" 절은 뺐고 감춘 사실은 걸러낸다. 끄려면 `--no-digest`. `.hash` 는 `<생성해시> <상태해시>` 두 칸이고 상태해시는 상태 부분만 덮는다. 상태 요약이 바뀐 장면도 낡은 것으로 센다(요약 줄 `state_stale=`).
  - 예전(한 칸) `.hash` 는 그대로 읽힌다. 새 해시로 옮기려면 `dieg stamp <작품>` (모델 호출 없음). 생성 경로의 낡음 해시(`genHash`)는 `previously`(앞 장면 사실문)를 뺀다. 코퍼스 `RenderHash` 는 그대로다.
  - `dieg revise` 는 카드 모드에서 바뀐 장면을 `{"edits":[{"old","new"}]}` 문장 패치로 고친다. `old` 는 장면 산문에서 정확히 한 번(겹치는 출현은 모호로 센다)만 맞아야 하고 다른 편집과 겹치면 안 된다. 패치도 카드 채점을 거치고, 실패하면 피드백과 함께 한 번 다시 시도한 뒤 전체 렌더로 되돌린다. 디스크 산문이 예전 IR 의 산문과 같을 때만 패치한다(해시 가드). 보고서에 `patched` 가 남고 요약 줄에 `patched=N` 이 나온다. `--verifier open` 에서는 패치하지 않는다.
  - 새 경고 3개(`rules/relative-time-fact.sql`, `retell-known.sql`, `fact-conflict.sql`): 상대 시간 사실문, 이미 아는 것을 다시 말하기, 같은 사건의 두 원인. run3 B 이야기에서 B1·B9·B10 을 잡는다(`lens_not_switched` 는 놓친다. `retell-known` 은 run3 B 에서 12건 중 5건이 논쟁거리). 코퍼스는 새 적중 0(hamlet 의 예고된 2건 제외).
  - `ReviseLoop` 는 수정 전 이야기의 기준선에 없던 **새 진단**만 막거나 고친다. 이미 있던 경고가 `revise` 를 막지 않는다.
  - 추가: `dieg stamp`, `dieg revise --skip s1,s2`(그 장면은 다시 렌더하지 않음), 환경변수 `DIEG_CALL_CAP=N`(모델 호출 하드 상한).

## 디제시스를 쓸 이유

디제시스가 있을 이유는 이야기 한가운데의 변경을 빠짐없이 퍼뜨리는 것이다. 잰 표본(재설정 3건, 31장면 중 다시 렌더한 장면은 재설정마다 7·14·21개)에서 직접 쓰기는 모순 4개를 남겼고 디제시스는 0개를 남겼다. 상관없는 글자는 직접 쓰기가 945자, 디제시스가 33자 바꿨고, 비용(가격 가중)은 거의 같았다(1.02배. 호출은 93 대 15). 비용 자체는 이유가 아니다(`examples/value/cost-curve`: 길어져도 직접 쓰기가 비싸지지 않는다). 판정은 사전 기준으로 "약함"이다: 비용 조건이 2% 차로 빗나갔고 이는 단일 실행 잡음 안이다. 첫 실행(229호출)은 비용 2.42배, 남은 모순 B 2 대 A 4였는데 원인이 플랜 7 자신의 경고와 해시였고, 고쳐서 B 만 다시 쟀다. 첫 실행의 모순 우위는 세는 방식에 민감했다: 자리별로 세면 B 4 대 A 4 다. 공개: 재실행에서 상태만 바뀐 29장면의 패치는 사실상 빈 패치였다(변경 내역이 비어 모델이 `{"edits":[]}` 를 냈다; 지금은 고쳤다). 그래서 1.02배는 상태 변경을 실제로 퍼뜨리는 비용을 낮게 잡은 값이고, 다시 쟬 계획이다. n=3, 단일 실행이다. [`examples/value/retcon2`](examples/value/retcon2/README.md)

## 측정 결과

직접 쓰기(A) 대 `dieg write`(B)는 [`examples/bench/README.md`](examples/bench/README.md) 에 있다.
짧은 아이템에서 B 는 캐시 가중 input 토큰이 A 의 2.7–2.8배라 "더 싸다"는 지지되지 않고, 일관성(오류 수)은 3:1 과 2:2 로 섞여 있다. 아이템 3개·판정 1회라 통계가 아니라 사례다.

연재(12화 계획, 11화에서 잘림, 수정 2회)는 [`examples/serial/README.md`](examples/serial/README.md) 에 있다. 가설은 서지 않았다: 오류는 원 수치로 B 3 대 A 4 지만 1만 자당으로는 B 1.24 대 A 0.89 (미충족), 가중 입력 토큰은 B 가 A 의 1.89배 (가격 기준은 단가표가 없어 미정), 수정 잔여는 B 4 대 A 1 (미충족). 아이템 1개·판정 1회다.

렌더러·렌더 모델 A/B(장면 2개, n=2)는 [`examples/renderer-ab/README.md`](examples/renderer-ab/README.md) 에 있다. 단일+haiku 는 단일+sonnet 과 통과율이 같고 가중 토큰은 약 절반이다. 계층형(v0.5.0 에 있었고 v0.6.0 에서 삭제)은 호출이 3~4배인데 통과율이 오르지 않았다. 그래서 기본은 단일 렌더러+haiku 렌더다(결정은 가중치와 무관하다. haiku 의 비용 우위는 가중치에 따라 달라진다: A/B 에서 출력 합계는 +44%(S-son 12,725 대 S-hai 18,368 토큰, `examples/renderer-ab/README.md`), 연재 run2 에서는 호출당 약 2.3배(3,819 대 1,630)). 표본이 작아 통계가 아니라 사례다.

연재 재측정(run2, 같은 아이템, 10화에서 잘림)은 [`examples/serial/README.md`](examples/serial/README.md) 의 "run2" 에 있다. (a) 오류: 이 표본에서 충족(B 3 대 A 8)이지만 A 의 run 간 흔들림이 차이와 비슷해 n=1 잡음 범위 안이다. (b) 비용: 가정한 가중치(s=0.6, h=0.2)에서 B 가 A 의 약 1.12배라 미충족. (c) 수정 잔여: 동률이라 미충족. 다음 후보는 haiku 렌더 effort low 와 하네스 `modelOf` 수정이다.

연재 run3(같은 아이템, 양쪽 12화, 호출 146회)은 [`examples/serial/README.md`](examples/serial/README.md) 의 "run3" 에 있다. B 는 카드 채점기와 렌더 effort low 로 돌렸다. (a) 오류: 미충족(A 8 대 B 9, 1만 자당 1.71 대 2.32. B11 파이프라인 잘림 1건을 빼면 8 대 8, 2.07). (b) 비용: 가정 가중치에서 B 가 A 의 0.86배(B 가 렌더하지 않은 s32~s35 를 더하면 약 0.88배)라 충족(A 는 run2 대비 49% 늘어 비교 기준이 흔들렸다). (c) 수정 잔여: 미충족(A 1 대 B 2). 카드는 31/31 통과, 재렌더 0(채점 호출은 렌더 37회에 44회, 2단 재질문 7회)이지만 카드가 지어냄으로는 실패할 수 없어 정의가 바뀐 몫이 크다. B 의 확정 오류 9건은 IR 3, 산문 5, 파이프라인 1, 판정 0 이고 산문 오류는 장면 간 소품·장소·앎의 표류라 장면별 검사가 못 본다. 아이템 1개·판정 1회(n=1)라 사례이며 대체 판단은 여전히 보류다.

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
  인물이 이야기에서 떠나면 `attr X.status = gone`, 돌아오면 `attr X.status = present` 로 적는다. 떠난 뒤의 행위·등장은 `stays-gone` 이 경고한다.
- `enacts` = 공연·재연. 무대 위 모두가 보지만 단언이 아니라 믿음을 만들지 않는다(예: 햄릿 3.2 극중극).

## 테스트

```sh
go test -count=1 ./...   # SL_PG_DSN 필요(PG 가 떠 있어야 한다)
go vet ./...
```

## 버전

[SemVer](https://semver.org/lang/ko/)(`MAJOR.MINOR.PATCH`)를 따른다. 1.0 전(0.x)에는 문법과 IR 이 MINOR 판에서 바뀔 수 있다. 판은 git 태그(`v0.5.0`)로 붙이고 `dieg version` 이 출력한다.

| 판 | 내용 |
|---|---|
| v0.1.0 | 플랜 1: 컴파일러, PG 판정, 린트, 렌더러 4종, 4편 합격 |
| v0.2.0 | 플랜 2: 정답 셋 3편, 시간 변이, 속성, 독자 모델, 패턴 9종, 지표, Grafana 대시보드 |
| v0.3.0 | 플랜 3: 거짓 사실 표시, 검증 렌더, 호출 계량(Meter), 사물 위치·거짓 공개 린트, `dieg write`, 직접 쓰기 대 Diegesis 측정 |
| v0.4.0 | 플랜 4: 역할별 모델·`--code-effort`, 뼈대 모드(`--skeleton`), `dieg continue`, `dieg revise`, 연재 측정 하네스(`tools/serial`). `render --verify` 기본 모델 sonnet |
| v0.5.0 | 플랜 5: 계층형 렌더러(`--renderer layered`, 옵트인; v0.6.0 에서 삭제), 기본 렌더 모델 haiku, 검증기가 저작된 사실 없는 행위·등장을 환각으로 세지 않음, stays-gone 규칙(`attr X.status = gone`), 연재 공정 재측정(run2) |
| v0.6.0 | 플랜 6: 기본 채점기 card(`--verifier open` 으로 예전 방식), 지어낸 사건은 경고만, 기본 `--render-effort low`, 계층형 렌더러(`--renderer`·`--beat-retries`) 삭제, 연재 run3 |
| v0.7.0 | 플랜 7: 렌더 프롬프트에 세계 상태 요약(`--no-digest`), 상태 해시 낡음·`dieg stamp`, `revise` 문장 패치(`patched=N`), 경고 3개, `revise` 는 새 진단만 막음, `--skip`·`DIEG_CALL_CAP`, 수정 재측정(retcon2) |
