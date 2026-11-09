# 아이디어 → 소설: Diegesis 로 쓰기 vs AI 가 바로 쓰기

아이디어 한 줄을 두 방법으로 소설 한 편(9장면)으로 만들었다. 모델은 둘 다 같은 `claude -p`(구독 CLI, claude-opus-5-5)다.

- **A (직접)**: AI 가 개요를 쓰고, 장면을 하나씩 쓴다. 장면마다 개요와 직전 장면만 본다.
- **B (Diegesis)**: AI 가 이야기를 `.dieg.md` 코드로 쓴다 → `dieg check`·`dieg lint` 진단을 AI 에게 돌려준다 → AI 가 고친다
  → `dieg render` 가 장면마다 산문을 쓴다.

두 방법은 같은 아이디어 문장과 같은 요구 사항(`prompts/common-requirements.txt`)을 받았다. B 가 더 가진 것은 언어와 도구뿐이다.

## 결과

| 축 | A 직접 | B Diegesis |
|---|---|---|
| 연속성 오류 (블라인드 판정 3회 다수결 → 내가 검증) | **7건** (1만 자당 3.8) | **2건** (1만 자당 1.7). 둘 다 렌더러가 지어낸 문장이고, 코드에서 온 것은 0건 |
| 계획한 구조 (6항목) | 6/6 있음, 보장 없음 | 6/6 있음. 복선·회수·거짓말·공개가 코드 줄로 있고, 린트가 회수와 순서를 검사한다 |
| 수정 요청 "범인을 바꿔라": 비용 | 개요 1회 + 장면 9개를 모두 다시 씀(호출 10회, 681초). 원문과 476줄이 다르다 | 코드 편집 1회 + 진단 수정 1회 + 장면 2개 다시 렌더(호출 4회, 그 밖에 출력을 잃은 호출 1회). 코드 +70/−28줄 |
| 수정 요청: 컴파일러가 잡은 하류 충돌 | 해당 없음(검사 수단 없음) | **1건** (`warn info-flow`: 바뀐 진상으로는 담임이 "제가 밀었어요"라고 말할 수 없다) |
| 수정 뒤 남은 오류 (판정 1회 → 검증) | **2건** (둘 다 수정 전부터 있던 것) | **3건** (렌더러가 수정 전부터 낸 것 1, 다시 렌더할 때 새로 낸 것 1, 코드에 있던 장소 오류 1) |
| 문체 교체 | 다시 써야 한다 | 같은 장면 코드를 백엔드만 바꿔 렌더(ko-haera → ko-hapsho, 호출 1회) |
| 비용 (원본) | 호출 10회, 706초 | 호출 12회, 1085초(코드 작성 530초 + 수정 2회 213초 + 렌더 9회 342초) |
| 판정 | 호출 4회, 619초 (두 팔 공통) | |

전체 호출은 42회다. 모든 호출은 `calls.tsv` 에 한 줄씩 남겼다(팔, 단계, 프롬프트, 출력, 초).

**정직한 요약.** 처음 쓴 원고는 B 가 분명히 앞섰다. A 의 오류 7건은 이야기 자체의 모순(머리핀 모양, 시각 계산, 누가 무엇을 봤는가)이다.
B 의 2건은 코드에 없는 문장을 렌더러가 덧붙인 것이다. 반면 수정 요청에서는 B 가 이기지 못했다. A 는 장면을 모두 다시 쓰면서
대체로 일관된 새 진상(관리인 오만석이 범인)을 만들었다. 판정자가 찾은 A 의 남은 오류는 2건으로, B 의 3건보다 적다.
B 는 호출과 수정 줄 수가 훨씬 적었지만, 컴파일러가 잡은 충돌은 1건이었다. B 의 남은 오류 가운데 둘은 렌더러에서, 하나는 지금 규칙이 보지 않는 곳에서 나왔다.

## 아이디어

`idea.txt`:

> 폭설로 길이 끊긴 산골 분교에서 교장 선생님이 숨진 채 발견된다. 열한 살 서진은 "그 시각 나와 함께 있었다"는 담임 선생님의 말을 믿었지만, 그날 울리지 않은 종이 그 거짓말을 드러낸다.

이 아이디어에는 거짓 알리바이, 그 거짓을 믿는 인물, 시각, 날마다 울리던 종(복선), 범인 공개가 들어 있다.
두 팔은 같은 아이디어로 서로 다른 이야기를 만들었다.

- A: 담임 윤재희는 위조 서류를 찾으려고 관사에 몰래 들어갔다. 범인은 교장의 아들 강태오이고, 흉기는 종의 추다.
- B: 범인은 담임 정혜원이다. 증거는 종탑 아래 묻힌 빨간 벙어리장갑이고, 회중시계는 두 시 오십이 분에 멈췄다.

## 같은 장면: 거짓 알리바이

A s4 (`a-direct/scenes/s4.md`):

> "저는 서진이랑 교실에 있었어요. 세 시부터 다섯 시까지요."
>
> "서진이가 감기 기운이 있어서 약 먹이고 난로 옆에 재웠어요. 네 시 종이 울려서 깨웠고, 받아쓰기 세 개 하고 다시 재웠어요. 화장실에 한 번 잠깐 다녀온 것 말고는 계속 같이 있었어요."

B s4 (`b-diegesis/novel/s4.ko-haera.md`):

> “선생님, 세 시 종 쳤어요? 깨워 준다고 했잖아요.”
>
> “벌써 쳤지. 종 울릴 때 네가 뒤척이길래, 너무 곤히 자서 안 깨웠어.”
>
> 선생님은 웃는 낯으로 그렇게 말하였다. 그날 오후 세 시에 분교의 종은 울렸고, 서진은 그것을 자면서 지나 보낸 셈이었다. (…)
>
> “선생님은 내내 여기 네 옆에 있었어.”

B 의 이 장면 코드(`b-diegesis/story/scenes/s4.dieg.md`):

```
beat alibi:
  teacher says seojin: bell_rang "벌써 쳤지. 종 울릴 때 네가 뒤척이길래, 너무 곤히 자서 안 깨웠어."
  teacher lies seojin: teacher_alibi "선생님은 내내 여기 네 옆에 있었어."
seojin believes teacher_alibi because @s4.alibi
seojin believes bell_rang because @s4.alibi
```

`lies`(거짓말)와 `says`(믿고 하는 말)가 구분된다. 담임은 주무관이 종을 대신 쳤으리라 믿는다(`teacher believes bell_rang`, 무대 밖 사건 three_oclock).
그래서 종 이야기는 거짓말이 아니라 착각이다. 서진이 두 가지를 다 믿게 되고, s7 에서 하람의 말(`bell_talk`)을 듣고 의심으로 돌아선다.
엔진은 이것을 시점마다 추적한다.

## A 의 오류와, B 가 같은 종류를 막은 방법

전체 목록과 인용은 `judge/verification-1.md` 에 있다.

1. **누가 무엇을 봤는가 (A6, knowledge).** A s5 에서 재희는 창고 안에서 "태오는 손을 오래 씻었다. (…) 그러고는 다시 보따리를 싸서 뒷문으로 나갔다." 까지만 본다.
   그런데 s9 에서는 "태오 씨가 창고 옆 눈사람 머리를 파고 그 안에 뭘 넣었습니다." 라고 증언한다.
   B 에서는 이런 증언이 행위(`tells`)로 적힌다. 화자가 그 전에 그 사실을 보거나 들은 적이 없으면 `info-flow` 규칙이 경고한다.
   실제로 수정 요청 때 이 규칙이 같은 종류를 잡았다(아래).
2. **속성 (A2, attribute).** 결정적 증거인 머리핀이 s2 에서는 "나비 날개 한쪽에 파란 큐빅이 박혀 있었는데, 다른 쪽 큐빅은 빠지고 없었다." 이다.
   s5 에서는 "날개 한쪽 칠이 벗겨진 나비였다." 로 바뀐다. B 에서는 물건의 속성을 `attr mitten.maker = "서진과 할머니"` 처럼 한 번 선언한다.
   두 곳에서 다르게 단언하면 `attribute-conflict` 가 잡는다. 다만 B1 이 보여 주듯, 렌더러가 선언하지 않은 속성을 산문에 지어내는 것은 막지 못한다.
3. **시각 (A3·A4, timeline).** A s5 는 "아홉 시가 되자" 와 "다섯 시간 전에도"(실제로는 15:00 부터 여섯 시간)를 함께 쓴다.
   B 는 장면과 사건의 시각을 `when after s3.end by 25m..25m`, `lasts 5m..10m` 처럼 제약으로 적는다. PG 의 `solve_time()` 이 순환과 모순을 찾는다.
   B 의 첫 수정 라운드에서는 info-flow 가 "order not determined"(5건, info)를 냈다. 사건 길이(`lasts`)가 빠져 앎의 순서가 정해지지 않는다는 뜻이다.
   AI 가 길이를 적어 넣자 진단이 사라졌다(`b-diegesis/rounds/r1.lint.txt` → `r2`).

B 코드 작성 로그: 처음 쓴 코드(r0)는 컴파일 오류 16건이었다(무대 밖 사건 안의 enter/exit, `shows` 의 대상이 물건).
1회차 수정 뒤 오류와 경고는 0건이고 info 5건이 남았다. 2회차 뒤에는 0건이 됐다. 원본 출력은 `b-diegesis/rounds/r*.check.json`, `r*.lint.txt` 에 있다.

## 계획한 구조

| 아이디어가 요구하는 것 | A | B | B 코드의 근거 |
|---|---|---|---|
| 날마다 울리는 종(복선) | 있음 (s1 교장 "4시 종" 자랑) | 있음 | s1 `principal tells seojin, haram: bell_daily …`, `setup promise_bell promise:` → s9 `payoff bungyo.scenes/promise_bell` |
| 거짓 알리바이 | 있음 (s4) | 있음 | s4·s5·s6 `teacher lies …: teacher_alibi` |
| 서진이 그 말을 믿는다 | 있음 (s4 끄덕임) | 있음 | s4 `seojin believes teacher_alibi because @s4.alibi`, s6 `seojin says officer: teacher_alibi` |
| 울리지 않은 종이 거짓을 드러낸다 | 있음 (s6 "받아쓰기 기억이 없다") | 있음 | s7 `seojin doubts teacher_alibi because @s7.bell_talk`, `reveal teacher_left` |
| 물건 복선 → 회수 | 있음 (눈사람 → 추) | 있음 | s2 `setup promise_mitten` → s8 `payoff …/promise_mitten`; s1 `setup promise_watch` → s6 `payoff …/promise_watch` |
| 범인 공개 | 있음 (s9 체포) | 있음 | s1 `withhold teacher_pushed until s9` → s9 `reveal teacher_pushed` |

A 에도 여섯 가지가 모두 있다. 차이는 보장이다. B 에서 `setup … promise` 가 회수되지 않으면 `unpaid-promise` 가 경고하고,
회수가 복선보다 앞서면 `payoff-without-setup` 이 경고한다. A 는 다 쓴 원고를 사람이 읽어야만 알 수 있다.

## 수정 요청 과정

요청(`prompts/change.txt`, 두 팔 공통): "범인을 바꿔라 … 담임의 거짓말과 울리지 않은 종은 그대로 남긴다."

**A**: 개요를 고쳤다(새 범인은 관리인 오만석). 그다음 장면 9개를 하나씩 다시 썼다. 매번 고친 개요, 원래 장면, 직전에 고친 장면을 줬다.
산문이 476줄 달라졌다. 판정에서 남은 오류는 수정 전부터 있던 2건이다(외투 색, "다섯 시간 전").

**B**: AI 에게 "진상을 정하는 줄만 가장 작게 고쳐라"고 했다. AI 는 `cast.dieg.md` 와 `scenes/offstage.dieg.md` 두 파일을 고쳤다(+28/−10줄).
범인은 조리 실무사 최말순이 됐고, 담임은 시신을 발견했을 뿐인 사람이 됐다. 컴파일과 린트 결과, 원본 그대로(`b-diegesis/change/c1.check.json`, `c1.lint.txt`):

```
[]
```
```
up to date
loaded bungyo
b-diegesis/change/c1/scenes/s9.dieg.md:33:3: warn info-flow: teacher says teacher_pushed without having learned it before
```

s9 의 고백 `teacher tells officer: teacher_pushed "서류를 달라고 매달렸어요. 계단에서… 제가 밀었어요."` 는 바뀐 진상으로는 성립하지 않는다.
담임이 직접 밀었다는 사건이 더는 없기 때문이다. 손대지 않은 장면 파일 속 모순을 엔진이 짚었다.
AI 는 이 진단을 받아 1회 만에 고쳤다(`c2.diff`: 담임의 진실 고백, 탄 동의서, 실무사의 자백, 새 복선 `promise_form`). 다시 돌린 check 와 lint 는 0건이었다.

다시 렌더할 장면은 렌더 요청을 비교해 골랐다(`tools/scenereq`, `change/req-before|after`). 행위가 바뀐 장면은 s8·s9 뿐이었다.
s1–s7 은 "말하면 안 되는 사실" 목록의 ID 만 `teacher_pushed` → `cook_pushed` 로 바뀌었고 행위는 같았다. 그래서 원래 산문을 그대로 썼다.
**두 장면만 다시 렌더했다.**

하지만 남은 오류는 B 가 더 많았다(3 대 2, `judge/verification-2.md`). 다시 렌더한 s9 가 "눈을 떴을 때 비어 있던 의자" 라는 회상을 지어냈다.
이 회상은 s4 와 부딪힌다. 컴파일러는 1건만 잡았다. 실무사가 앞 장면들에서 한 행동(담임을 의심하는 눈짓, 주무관을 몰아가는 말)은
고친 진상 아래에서 의도와 무관하게 '은폐'로 읽히게 됐다. 이것은 오류는 아니지만, 그렇게 읽히는 것을 엔진이 확인한 것도 아니다.

## 문체 교체

같은 B s4 코드, 렌더 백엔드만 다르다. `dieg render b-diegesis/story s4 --backend ko-hapsho`.

| ko-haera (`b-diegesis/novel/s4.ko-haera.md`) | ko-hapsho (`b-diegesis/styles/s4.ko-hapsho.md`) |
|---|---|
| 침대 곁 걸상에 앉아 있던 정혜원 선생님이 손을 뻗어 서진의 이마를 짚었다. “깼어? 열은 좀 내렸네.” | 그때 침대 곁에서 정혜원 선생님이 서진의 이마에 손을 얹으며, “깼어? 열은 좀 내렸네.” 하였습니다. |
| 선생님은 웃는 낯으로 그렇게 말하였다. 그날 오후 세 시에 분교의 종은 울렸고, 서진은 그것을 자면서 지나 보낸 셈이었다. | 하였습니다. 그날 오후 세 시에도 분교의 종은 울렸던 것입니다. |

대사는 코드의 인용 문자열이라 두 렌더가 한 글자도 다르지 않다. 서술의 어투만 바뀐다.
`dieg render-check` 는 돌리지 않았다. 장면 하나에 렌더 4회와 재추출 12회, 모두 16회 호출이 들어 호출 한도(55) 안에 들지 않아 뺐다.

## 한계

- **아이디어 하나, 실행 한 번.** 통계가 아니라 사례다. 두 팔은 같은 아이디어로 서로 다른 이야기를 만들었다. 오류 수에는 이야기의 복잡도 차이도 섞여 있다.
- **길이가 다르다.** A 에는 장면마다 2500자를 요청했다. B 가 몇 자를 낼지 미리 알 수 없어서였다. 공백을 빼면 A 는 18,400자, B 는 12,062자다.
  그래서 1만 자당 수치를 함께 적었다.
- **판정자의 재현율.** 판정 1 은 3회 다수결, 판정 2 는 1회다(호출 한도). 판정 1 에서 나온 A 오류 5건이 판정 2 에 다시 나오지 않았다.
  고쳐졌는지 놓쳤는지는 가리지 않았다. 내가 따로 찾은 B 의 코드 오류(s9 장소)는 판정 1 집계에 넣지 않았다.
- **렌더러 변동.** B 의 오류는 모두 렌더러가 코드 밖의 사실을 지어낸 것이거나(B1·B2·B3) 지금 규칙이 보지 않는 것이다(B4).
  코드가 맞아도 산문이 틀릴 수 있다. `render-check`(재추출 대조)가 이것을 재는 도구지만 이번에는 돌리지 않았다.
- **산문 품질.** A 는 더 길고, 장면 사이 묘사가 자유롭다. B 는 행위를 순서대로 옮기므로 덜 자연스러운 곳이 있다.
  예: s4 첫 줄의 생각(`thinks`)이 따옴표 대사로 렌더됐다. 품질은 판정하지 않았다.
- **코드 작성 비용.** B 의 첫 코드 작성 호출 한 번이 530초로, A 의 원고 전체(706초)에 가깝다. 프롬프트에 문법·README·예제가 들어가 매 수정 호출이 1,100줄 이상이다.
- **같은 모델, 같은 시스템 프롬프트**(`prompts/call.sh`). 두 팔 다 도구를 끈 `claude -p` 다. B 의 렌더 호출은 렌더러 자체 프롬프트를 쓴다(`internal/render/prompt.go`).
- 출력을 잃은 호출이 1회 있다(B change-edit, 출력 디렉터리 없음). `calls.tsv` 에 적었다.

## 플랜 3 후보 (언어·도구의 빈틈)

1. **물건 위치 ↔ 장면 장소 검사가 없다.** `attr bell.hung_at = tower` 인데 `location: staffroom` 장면에서 `seojin does ring bell` 이 통과한다.
2. **`shows` 가 물건을 대상으로 받지 못한다.** `principal shows watch to seojin` 은 `arg-kind`("shows object "watch" is a thing, want character or group") 오류다.
   AI 는 이를 피하려고 "교장 선생님이 주머니에서 회중시계를 꺼내 보였다." 같은 서술을 대사 인용 문자열 안에 넣었다. 문법(EBNF)에도 이 제약이 적혀 있지 않다.
3. **거짓 사실을 `reveal`·`withhold` 해도 검사하지 않는다.** 수정 1단계 코드는 `teacher_pushed` 를 `false` 로 바꾸고도 `reveal teacher_pushed` 를 남겼다.
   린트는 이를 통과시켰다. 결과적으로 info-flow 가 같은 장면의 고백을 잡았지만, 거짓을 독자에게 공개한다는 줄 자체에 대한 규칙은 없다.
4. **"범인이 공개되는가" 같은 이야기 수준의 요구를 선언할 수단이 약하다.** `pattern` 선언은 있지만, 이번 이야기에 맞는 패턴(예: 거짓 알리바이의 붕괴)이 없다.
5. **렌더러의 사실 추가를 렌더 단계에서 막지 못한다.** 렌더 규칙은 "명세에 없는 사실을 더하지 말라"고 하지만 B1·B2·B3 이 나왔다.
   렌더 결과를 매번 `render-check` 로 재추출해 대조하는 기본 경로가 필요하다(지금은 코퍼스의 `expect.yaml` 장면 하나만 대상이다).
6. **앎의 순서가 정해지지 않는 것(info "order not determined")** 은 경고가 아니라 info 다. 이번에는 AI 가 고쳤지만, 경고로 올릴지 검토할 만하다.

## 파일

- `idea.txt`, `calls.tsv`(모든 호출 기록), `prompts/`(프롬프트, 조립 스크립트, `call.sh`)
  - 500줄이 넘는 조립 프롬프트(문법·예제·코드·두 소설을 통째로 넣은 것)와 모델 원본 출력(`*.raw.txt`)은 커밋하지 않았다.
    대신 스크립트로 다시 만들 수 있고 sha256 을 `prompts/SHA256SUMS-uncommitted.txt` 에 남겼다. 원본 출력은 `unpack.sh` 로 푼 파일로 들어 있다.
- `a-direct/`: `outline.md`, `scenes/s1–s9.md`, `change/`(고친 개요와 장면)
- `b-diegesis/`: `story/`(최종 코드), `rounds/`(r0–r2 코드와 회차별 진단), `novel/`(ko-haera 렌더), `styles/`(ko-hapsho),
  `change/`(c1 = 1단계 편집, c2 = 진단 수정, `*.diff`, 진단, 렌더 요청 비교, `novel/`)
- `judge/`: 판정자 원본 출력(`judge*-run*.json`), X/Y 배정(`mapping.txt`), 내 검증(`verification-1.md`, `verification-2.md`)
- `tools/scenereq/`: 장면 렌더 요청을 꺼내 비교하는 작은 도구(컴파일러·렌더러는 고치지 않았다)

## 다시 돌리기

```sh
export PATH=<go-install>/bin:$PATH SL_PG_DSN='postgres://sl:sl@localhost:55432/sl?sslmode=disable'
go build -o dieg ./cmd/dieg && export DIEG=$PWD/dieg
cd examples/idea-to-novel
prompts/b-build-prompt.sh && prompts/call.sh B code-r0 prompts/b1-code.txt b-diegesis/rounds/r0.raw.txt
prompts/unpack.sh b-diegesis/rounds/r0.raw.txt b-diegesis/rounds/r0 && prompts/b-check.sh b-diegesis/rounds/r0 b-diegesis/rounds/r0
prompts/b-fix-prompt.sh b-diegesis/rounds/r0 b-diegesis/rounds/r0 prompts/b2-fix-r1.txt   # … r2 까지
prompts/b-render.sh b-diegesis/story b-diegesis/novel ko-haera s1 s2 s3 s4 s5 s6 s7 s8 s9
prompts/call.sh A outline prompts/a1-outline.txt a-direct/outline.md && prompts/a-scenes.sh a-direct/outline.md a-direct/scenes a2
prompts/judge-build.sh a-direct/scenes .md b-diegesis/novel .ko-haera.md B prompts/judge1.txt
```
