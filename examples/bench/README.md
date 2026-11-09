# 측정: AI 가 바로 쓰기(A) 대 `dieg write`(B)

같은 아이템, 같은 모델(`claude -p`, claude-opus-5-5, effort high, 구독 CLI), 같은 목표 장면 수로 두 방법을 비교했다.

- **A (직접)**: 데모(`examples/idea-to-novel`)와 같은 방법이다. AI 가 개요를 쓰고, 장면을 하나씩 쓴다. 장면마다 개요와 직전 장면만 본다.
  프롬프트는 데모의 `a1-outline.txt`·`a-scenes.sh` 를 장면 수만 바꿔 옮겼다(`tools/bench/main.go`).
- **B (Diegesis)**: `dieg write <idea> -o b --rounds 3`. 코드 작성 → `check`·`lint` 진단 되먹임 → 장면마다 검증 렌더(렌더 1 + 재추출 1, 최대 3회 시도).
- 아이템 파일 끝에 목표 장면 수(`장면 7개.`)를 적었다. B 는 아이템을 그대로 받고, A 는 같은 아이템과 함께 요구 사항에 같은 수를 받는다.

## 결과

오류 = 블라인드 판정 1회(X/Y 무작위)가 낸 지적을 내가 원문 인용으로 검증해 남긴 수. 토큰 = `calls.jsonl` 합.
claude CLI 는 프롬프트 대부분을 cache 필드에 넣으므로 **총 input = input + cache_read + cache_write** 다.

| 아이템 | 장면 | 오류 A / B | 총 input A / B | output A / B | 호출 A / B | 분 A / B | B 라운드 | B 검증 실패 장면 |
|---|---|---|---|---|---|---|---|---|
| lighthouse | 7 | **3 / 1** | 39,478 / 137,862 (3.5×) | 44,082 / 96,892 (2.2×) | 8 / 34 | 9.6 / 20.3 | 2 (clean) | 4/7 (s3 s4 s5 s6) |
| will | 6 | **2 / 2** | 33,280 / 116,526 (3.5×) | 37,364 / 82,464 (2.2×) | 7 / 28 | 8.1 / 18.1 | 2 (clean) | 2/6 (s3 s5) |
| mine (긴 것) | 20 | 판정 안 함 | 130,547 / 92,900* | 107,325 / 94,767* | 21 / 20* | 24.1 / 17.8* | 2 (clean) | 1/5 (s3) |
| ferris | 8 | 돌리지 않음(예산) | | | | | | |

\* mine 의 B 는 코드 20장면 전체 + **앞 5장면만 렌더**한 값이다(아래 "줄인 것"). 20장면을 다 렌더한 값이 아니다.

네 토큰 필드 원본(팔별 합):

| 아이템 | 팔 | input | cache_read | cache_write | output |
|---|---|---|---|---|---|
| lighthouse | A | 16 | 0 | 39,462 | 44,082 |
| lighthouse | B | 68 | 32,608 | 105,186 | 96,892 |
| will | A | 14 | 0 | 33,266 | 37,364 |
| will | B | 56 | 26,494 | 89,976 | 82,464 |
| mine | A | 42 | 0 | 130,505 | 107,325 |
| mine | B* | 40 | 18,342 | 74,518 | 94,767 |
| 판정 lighthouse | | 2 | 0 | 34,453 | 14,032 |
| 판정 will | | 2 | 0 | 30,629 | 12,363 |

산문 길이(공백 제외 글자 수, UTF-8 문자): lighthouse A 14,038 / B 7,857, will A 12,170 / B 7,386. A 가 1.6–1.8배 길다(장면당 약 2.0k 자로 요청한 "2500자 안팎"에 가깝다).
1만 자당 오류: lighthouse A 2.14 / B 1.27, will A 1.64 / B 2.71.

**요약.** 짧은 두 아이템에서 B 는 매번 **더 비쌌다**(총 input 토큰 3.5배, 캐시 가중 input 약 2.7배, output 2.2배, 호출 4배, 시간 2배). 오류는 한 번 이기고(3:1) 한 번 비겼다(2:2).

"총 input" 은 토큰 양이지 비용이 아니다. 캐시 읽기 0.1×, 캐시 쓰기 1.25× 로 가중하면(input + 0.1·cache_read + 1.25·cache_write) lighthouse A 49,344 / B 134,811 (2.7×), will A 41,597 / B 115,175 (2.8×), mine A 163,173 / B* 95,022 다.
B 의 코드는 두 번 다 2라운드 만에 깨끗해졌지만, 렌더 검증에서 장면의 33–57% 가 3회 시도 끝에 실패했다. 실패한 장면도 마지막 렌더가 소설에 들어갔다.

## 긴 아이템(mine, 20장면): 장면별 입력 토큰

| 장면 | A 입력 | A 출력 | B render 입력 | B extract 입력 | B 출력 |
|---|---|---|---|---|---|
| 개요 / 코드 | 1,359 | 5,025 | code-r1 14,131 · fix-r2 27,111 | | 51,889 + 13,749 |
| s1 | 3,535 | 5,017 | 4,869 (2회) | 5,594 | 5,727 |
| s2 | 6,522 | 4,536 | 2,789 | 3,038 | 3,261 |
| s3 | 6,633 | 4,926 | 8,451 (3회, 실패) | 7,762 | 9,122 |
| s4 | 6,447 | 4,387 | 5,916 (2회) | 6,916 | 6,625 |
| s5 | 6,618 | 3,944 | 3,036 | 3,287 | 4,394 |
| s6–s20 | 5,724–6,918 (평균 6,629) | 3,9xx–7,684 | 렌더 안 함 | | |

읽는 법:

- **A 의 문맥은 쌓이지 않는다.** 데모의 A 방법은 장면마다 "개요 + 직전 장면"만 준다. 그래서 장면별 입력은 s2–s19 가 6.4–6.9k 로 평평하다(s20 은 5.7k).
  긴 작품에서 A 가 문맥 누적으로 비싸진다는 가설은 **이 A 설계로는 시험되지 않는다.** A 의 총비용은 장면 수에 정비례한다.
  (모든 장면을 문맥에 넣는 A 는 돌리지 않았다. 그러면 입력이 장면 수의 제곱으로 늘겠지만, 이번 측정 범위 밖이다.)
- **B 의 장면 단위 입력도 평평하다**(렌더 1회 2.8–4.9k + 재추출 3.0–5.6k). 다만 재시도가 붙으면 2–3배가 된다.
- **B 의 고정비는 코드 단계다.** 20장면 코드 작성 2호출이 input 41,242, output 65,638 이다. 같은 아이템 A 전체 output(107,325)의 61% 다.
  짧은 아이템의 코드 단계(lighthouse 35,441 / 42,265, will 33,526 / 30,752)보다 output 이 1.5–2배다.
- **B 를 20장면 다 렌더했다면(외삽, 측정 아님):** 렌더한 5장면 평균(장면당 input 10.3k, output 5.8k, 호출 3.6회)을 20장면에 곱하고 코드 단계를 더하면
  총 input ≈ 248k, output ≈ 182k, 호출 ≈ 74회다. A(131k / 107k / 21회)의 1.9배 / 1.7배 / 3.5배다. 장면이 늘어도 B 가 싸지지 않는다. 이 외삽은 B 를 낮게 잡았을 가능성이 크다. 렌더한 5장면의 검증 실패율(1/5)이 짧은 아이템(4/7, 2/6)보다 낮았기 때문이다.

## 오류 인용

전체 목록과 판정은 `*/judge/verification.md`, 판정자 원본은 `*/judge/run1.txt`, 인용 대조는 `*/judge/quotes.md` 에 있다.

1. **A: 개요의 진상이 장면에 새어 든다(lighthouse A3, knowledge).** s4 에서 은솔이 찢긴 쪽의 압흔을 문지르지만
   "끝자리는 삼촌의 볼펜 글씨에 가려 보이지 않았다", "'차단기'가 나왔다. 그 뒤는 뭉개져 있었다." 이다.
   s7 에서는 "그저께 밤 당직실에서 연필을 눕혀 문지른 종이" 를 들고 "20:15. 회전 정지, 차단기 내려짐. 도철? 선착장 확인." 을 다 읽는다.
   개요 시각표의 문장 그대로다. 같은 아이템의 A2 도 같은 꼴이다. 삼촌이 "사흘 동안 되풀이한 말" 이 절벽길에서 진상의 장소(방파제)로 바뀐다.
   B 는 이 종류를 막는 구조를 가진다. 거짓말은 `lies` + `false` 사실로, 앎은 `info-flow` 로 검사되고, 이번 B 산문에는 이 종류가 없었다.
2. **B: 코드에 있지만 검사 밖인 오류(will B3, knowledge).** 정숙은 속봉투를 본 적도 들은 적도 없는데 s4 에서
   "속봉투를 가지고 마루로 나와라. 거기서 말하마." 라고 한다. 코드 그대로다:
   ```
   jeongsuk says daon: "속봉투를 가지고 마루로 나와라. 거기서 말하마."
   ```
   사실을 달지 않은 인용이라 `info-flow` 가 보지 못한다. 속봉투는 s3 에서 다온·수아만 안다(`setup inner_envelope promise: open_together`).
   코드는 check 오류 0, lint 경고 0 이었다.
3. **B: 렌더러가 지어낸 시각(will B1, timeline).** 코드 s5 는 `when: after s4 by 10m..30m` 이고 s4 는 저녁이다.
   렌더 산문 s5 는 "안쪽은 새벽의 서늘한 기운이 그대로 남아 있었다" 이면서 "내일 재에서, 할머니께 같이 인사드려요." 라고 하고,
   s6 은 그 대화를 "어젯밤에 고모랑 무슨 얘기 했어?" 로 부른다. lighthouse B1(위에 있어야 할 일지를 "겨드랑이에 낀" 채 계단을 오름)도
   렌더러가 코드 밖 사실을 지어낸 것이다. B 의 확정 오류 3건 가운데 2건이 렌더러, 1건이 코드에서 나왔다.
   (will B2, 날짜가 "맨 윗줄" ↔ "편지 끝자락" 은 뺀 A3 와 같은 정도의 경계 사례라 대칭으로 뺐다.)

## 검증 실패 장면 (B, 원문 그대로)

`*/b.log`(dieg write 의 stderr)에서 실패 장면만 옮긴다. 모두 3회 시도했고 실패한 마지막 렌더가 소설에 들어갔다.

```
lighthouse s3: attempts=3 pass=false missing=[] hallucinated=[extra-act:eunsol|perceive|flashlight_left 은솔은 날이 새면 등롱실에 올라가 아버지의 일지를 확인하기로 결심한다.] leaked=[]
lighthouse s4: attempts=3 pass=false missing=[] hallucinated=[Eunsol hides her father's log inside the front of her jacket. extra-act:eunsol|perceive|motor_works extra-act:eunsol|read|last_page] leaked=[]
lighthouse s5: attempts=3 pass=false missing=[] hallucinated=[extra-act:eunsol|perceive|boxes_hidden extra-act:jeongseok|assert|smuggling extra-act:mikyung|perceive|boxes_hidden] leaked=[]
lighthouse s6: attempts=3 pass=false missing=[] hallucinated=[extra-act:mikyung|perceive|radio_works] leaked=[]
will s3: attempts=3 pass=false missing=[] hallucinated=[extra-act:daon|read|jeongsuk_absent extra-act:sua|read|jeongsuk_absent 다온이 셋째 항아리 바닥에서 아무도 손대지 않은 봉해진 속봉투를 찾아낸다.] leaked=[]
will s5: attempts=3 pass=false missing=[] hallucinated=[extra-act:daon|assert|raised_by_jeongho extra-act:daon|assert|raised_by_jeongho extra-act:daon|perceive|daon_birth extra-act:jeongsuk|act|open_together extra-act:jeongsuk|assert|daon_birth] leaked=[]
mine s3: attempts=3 pass=false missing=[] hallucinated=[extra-act:chulsu|assert|four_trapped extra-act:chulsu|perceive|cavein extra-act:dongsu|perceive|four_trapped 오만길의 다리가 무너진 돌에 깔렸다.] leaked=[]
```

실패는 모두 hallucinated(코드에 없는 행위·사실을 재추출이 찾음)다. leaked 는 0건이다. lighthouse s3 의 지적("등롱실에 올라가 … 일지를 확인하기로")은
판정자가 찾은 B1 과 같은 줄기다. 검증기는 그 장면을 실패로 표시했지만 렌더를 바로잡지는 못했다.
lighthouse s1·s2·s7, will s1·s2·s4·s6, mine s1·s2·s4·s5 는 통과했다(mine s2 는 missing 1건으로 기준 안).

## 한계

- **아이템 3개, 판정 2개, 실행 한 번, 모델 하나.** 통계가 아니라 사례다. 오류 차이(3:1, 2:2)는 판정자 한 번의 재현율 안에 있다.
- **판정은 1회다.** 데모는 3회 다수결이었다. 판정자가 놓친 오류는 세지 않았다. 내가 따로 찾은 오류도 넣지 않았다.
  A 지적 7건 가운데 2건(lighthouse A4, will A3)을 확신이 없어 뺐다. B 지적 4건 가운데 1건(will B2)을 같은 기준으로 뺐다. 두 판정 모두 X=B 로 뽑혀 위치(X/Y)는 바뀌지 않았다.
- **길이가 다르다.** A 는 장면마다 "2500자 안팎"을 받았지만 장면당 공백 제외 약 2.0k 자를 써서 요청에 가깝다. B 는 산문 길이를 정하지 않는다. A 가 1.6–1.8배 길다.
  긴 글은 오류가 날 자리가 많고 토큰도 많다. 1만 자당 수치를 함께 적었다.
- **두 팔은 서로 다른 이야기를 만들었다.** 오류 수에 이야기의 복잡도 차이가 섞인다.
- **A 의 문맥은 쌓이지 않는다**(위). "긴 작품일수록 A 가 비싸다"는 이 설계로 볼 수 없다.
- **수정 요청은 돌리지 않았다.** "수정에 강하다"는 주장은 이번 측정이 다루지 않는다(데모의 수정 실험이 유일한 근거이고, 거기서도 B 가 이기지 못했다).
- **긴 아이템은 토큰만 쟀고, B 는 5/20 장면만 렌더했다.** 20장면 B 비용은 외삽이다.
- **B 의 오류 원천.** 확정 오류 3건 중 2건은 렌더러가 코드 밖 사실을 지어낸 것이다. 검증 렌더는 이를 표시(실패)하지만 고치지 못하고, 실패 렌더도 소설에 들어간다.
- 출력 토큰에는 모델의 사고 토큰이 섞여 있다(CLI 가 나눠 주지 않는다). A 의 장면 하나(산문 ~2.0k 자)가 output 4–7k 토큰인 이유다.

## 주장별 판정

| 주장 | 이번 결과 |
|---|---|
| Diegesis 가 더 일관된다 | **섞여 있다.** 3:1 로 한 번 이기고 2:2 로 한 번 비겼다. 1만 자당으로 보정하면 B 1.27 대 A 2.14, B 2.71 대 A 1.64 다. A 의 오류는 이야기 자체의 모순(속성, 진상 누설)이었고, B 의 오류는 대부분 렌더러에서 나왔다. B 코드는 두 번 다 진단 0건으로 수렴했지만, 사실을 달지 않은 대사 속의 앎(will B3)은 검사 밖이다. 이번 데이터로 "더 일관된다"고 말할 수 없다. |
| Diegesis 가 더 싸다 | **지지되지 않는다.** 짧은 아이템에서 B 는 총 input 3.5배, output 2.2배, 호출 4배였다. 원인은 코드 작성(참고 자료 14k 토큰 + 코드 25–52k 토큰 출력)과 검증 렌더의 재시도(실패 장면은 렌더 3 + 재추출 3)다. |
| 긴 작품일수록 Diegesis 가 유리하다 | **지지되지 않는다(비용).** A 는 장면당 입력이 평평하고(6.6k), B 도 장면당은 평평하다. B 의 코드 단계는 장면 수와 함께 커진다(20장면 코드 output 65.6k). 외삽한 20장면 B 는 A 의 1.9배(input) / 1.7배(output) 다. 일관성은 긴 아이템에서 재지 않았다. |
| 수정에 강하다 | **이번에는 재지 않았다.** |

## 줄인 것 (예산 ≤ 150 호출)

- 예산 추정: 짧은 아이템 하나에 A 8 + B 30–45 + 판정 1. 첫 아이템의 B 가 34호출이었다.
- **컷 1 (긴 아이템):** B 를 `--scenes 5` 로 돌렸다. 코드는 20장면 전체를 쓰고(라운드·토큰 기록), 렌더는 앞 5장면만 했다. A 는 20장면을 다 썼다. 판정은 하지 않았다(토큰 전용).
- **컷 2:** 세 번째 짧은 아이템 `ferris`(8장면)를 돌리지 않았다. 남은 30호출로는 A 9 + B ≈30 + 판정 1 이 들지 않는다. 아이템 파일은 `ideas/ferris.txt` 에 남겼다.
- B 의 `--rounds` 는 3 으로 줄였다(기본 4). 세 아이템 모두 2라운드에서 깨끗해져 결과에는 영향이 없다.
- 실제 호출: **120회**(lighthouse A 8 · B 34 · 판정 1, will A 7 · B 28 · 판정 1, mine A 21 · B 20). 별도 시험 호출은 없다. 실패한 호출은 0회다.

## 파일

- `ideas/*.txt` — 아이템(원작 없음). 끝 줄이 목표 장면 수다.
- `<아이템>/a/` — `outline.md`, `scenes/s<i>.md`, `prompts/`(호출마다 보낸 프롬프트), `calls.jsonl`
- `<아이템>/b/` — `dieg write` 출력 그대로: `story/`(최종 코드), `rounds/`(라운드별 원본 출력과 코드), `diags.txt`, `novel/`(장면 산문 + 검증 보고 `*.report.json`), `novel.md`, `calls.jsonl`
- `<아이템>/b.log` — `dieg write` 의 stdout+stderr(요약 줄, 장면별 검증 결과)
- `<아이템>/judge/` — `mapping.txt`(X/Y 배정), `run1.txt`(판정자 원본), `quotes.md`(인용 대조), `verification.md`(내 검증), `calls.jsonl`
  - 판정 프롬프트 `prompt.txt` 는 500줄이 넘어 커밋하지 않았다. 체크리스트(`tools/bench/judge.go` 의 `judgeHead`, 데모의 `judge-build.sh` 와 같은 글) + 두 소설로 다시 만들 수 있고, sha256 을 `SHA256SUMS-uncommitted.txt` 에 남겼다.
- 산출물은 손대지 않았다. 컴파일러·규칙·렌더러·프롬프트도 측정 중에 바꾸지 않았다.

## 다시 돌리기

```sh
export PATH=<go-install>/bin:$PATH SL_PG_DSN='postgres://sl:sl@localhost:55432/sl?sslmode=disable'
go test ./tools/bench                       # TestBenchDryRun: 가짜 Completer 로 한 아이템을 끝까지
go run ./tools/bench a -idea examples/bench/ideas/lighthouse.txt -n 7 -out examples/bench/lighthouse
go run ./tools/bench b -idea examples/bench/ideas/lighthouse.txt -rounds 3 -out examples/bench/lighthouse
go run ./tools/bench judge -n 7 -out examples/bench/lighthouse
go run ./tools/bench b -idea examples/bench/ideas/mine.txt -rounds 3 -scenes 5 -out examples/bench/mine
go run ./tools/bench sum examples/bench/lighthouse examples/bench/will examples/bench/mine
```

A 와 판정은 이미 있는 파일을 다시 묻지 않는다(이어 돌리기). B 는 매번 처음부터 돈다.
