> 공개용 스냅샷(2026-11-09). 설계 기준은 프로젝트 설계 노트.

# 언어 — 개념 모델과 문법

> 0.x 동안은 바뀐다. 문법의 단일 기준은 [`spec/grammar.ebnf`](../spec/grammar.ebnf) 이고, 이 문서는 개념과 쓰는 법을 설명한다.

## 문법 원칙

- **한 줄에 한 문장, 블록은 들여쓰기(2칸).** 줄 단위 diff가 깨끗하고, LLM이 만든 출력이 틀린 위치를 줄 번호로 짚을 수 있다.
- **의미는 ID로, 문장은 따옴표로.** 따옴표 안의 문자열은 검사 대상이 아니다. 렌더러와 사람을 위한 참고일 뿐이다.
- **원문 인용은 복사하지 않고 줄 번호로 참조한다** (`src 1.3.48`). 코퍼스 파일이 가벼워지고, 라운드트립 비교 때 원문과 줄을 맞출 수 있다.
- 키워드는 영어 소문자다. 식별자에는 유니코드 글자를 허용한다(`曹操`, `조조`).
- 주석은 `#`, 검사 억제 지시어는 `#!`.
- 이 문서의 문법은 `.dieg.md` 파일의 ```` ```sl ```` 블록 안에 쓰는 언어다. 프론트매터는 이 언어의 설탕이다 → [file-format](file-format.md)

## 개념 모델

| 개념 | 선언 | 비고 |
|---|---|---|
| 작품 | `work` | 시간 단위(`unit`)와 원점(`start`)을 가진다 |
| 모듈 | `module a.b`, `import a.b` | 한 파일이 한 모듈이다. 장·권 단위로 나눈다 |
| 엔티티 | `character` `group` `place` `thing` `faction` | 고유 ID와 중요도 `major` / `minor` / `extra` 를 가진다 |
| 역할 | `role` | **시간 구간이 붙는 별칭.** "Thane of Cawdor", "승상"처럼 보유자가 바뀌는 호칭 |
| 명제 | `fact` | 세계에 대한 진술. 참/거짓이 고정이거나 상태 술어로 정의된다 |
| 속성 | `attr 개체.키 = 값` | 수·문자열·ID. 시점에 묶으면 시간에 따라 바뀐다 |
| 사건 / 장면 | `scene`, `event` | 장면에는 장소와 출석부가 있다. `event` 는 무대 밖 사건이다 |
| 발화·지각 | `tells` `says` `lies` `aside` `sees` `reads` … | 수신자는 출석부에서 계산한다 |
| 문서 | `artifact` | 편지, 위조 서신. 담긴 명제를 읽는 사람에게 옮긴다 |
| 서사 줄기 | `thread` | 병렬로 진행되는 줄기 |
| 복선 | `setup`, `payoff` | 종류는 `promise`(반드시 회수) / `hint` / `symbol` |
| 패턴 선언 | `pattern` | 작가가 "이 서사 패턴이 있다"고 선언한다 |
| 독자 | `reveal`, `withhold`, `discourse` | 서술 순서와 정보 공개 일정 |
| 렌더 지시어 | `render` 블록 | 시점, 속도, 긴장도, 묘사 대상, 생략 |
| 기록·관점 | `account` | 같은 사건을 다르게 적은 기록들(정사 vs 연의, 복음서) |

### 선언과 작품

<!-- check -->
```sl
module demo.cast

work demo "작은 이야기"
  unit day
  name ko "작은 이야기"

character anna "Anna" major
  name ko "안나"
character ben "Ben" minor
  name ko "벤"
place hall "the hall"
fact door_locked "The cellar door is locked" true
fact key_hidden "The key is hidden in the hall"
```

### 시간 표현

시점은 `@장면`, `@장면.비트`, `@장면.end`, `start` 로 가리킨다. 장면과 사건은 `when` 으로 위치를 잡고, `lasts` 로 길이를 준다.

<!-- check -->
```sl
module demo.time

work demo
  unit day
place hall

scene 1.1 at hall
  when after start
  lasts 1h..2h
scene 1.2 at hall
  when after 1.1 by 0d..1d      # 1.1 끝난 뒤 0~1일 사이
scene 1.3 at hall
  when before 1.4               # 순서만 안다
scene 1.4 at hall
  when at 208-11                # 절대 시각 (정밀도는 월 단위)
scene 1.5 at hall
  when during 1.4               # 1.4 안에서
```

컴파일러는 이 표현을 STN 제약으로 바꾼다 → [compiler](compiler.md#시간-모델)

### 명제 식

```
fact cawdor_traitor "Cawdor sided with Norway" true      # 고정된 참
holds(thane_of_cawdor, macbeth)                           # 상태 술어: 역할 보유
alive(duncan)  at(macbeth, inverness)                     # 내장 술어
not P
fact king_hereafter = holds(king, macbeth)                # 예언: 지금은 거짓, 나중에 참이 된다
```

상태 술어로 정의한 명제의 참/거짓은 **시점마다** 계산한다. 예언은 별도 개념이 아니라 "미래 시점에 참이 되는 명제를 주장한 발화"다.

### 장면, 비트, 행위

<!-- check -->
```sl
module demo.scene

work demo
  unit day
character anna "Anna" major
character ben "Ben" minor
place hall
fact door_locked "The cellar door is locked" true
fact key_hidden "The key is hidden in the hall"

scene 2.1 at hall
  when after start
  enter anna, ben
  anna tells ben: key_hidden "The key is under the lamp." src 2.1
  beat quarrel:
    ben asks anna: door_locked "Why is the door locked?"
    anna lies ben: not door_locked "It is not." src 2.2
  ben doubts key_hidden
  exit all
```

행위의 꼴은 `행위자 동사 [대상] [: 명제] ["인용"] [because 명제] [src 줄번호]` 다. 행위자 자리에 역할을 쓸 수 있다: `role:chancellor tells …`. 컴파일러가 그 시점의 보유자로 해석하고, 그 시점에 보유자가 바뀔 가능성이 있으면 모호하다고 경고한다.

동사는 `tells` `says` `hails` `asks` `lies` `whispers` `aside` `thinks` `sees` `shows` `reads` `writes` `sends` `orders` `decrees` `does` `enacts` 다. `enacts` 는 공연·재연이다(극중극): 무대 위 모두에게 보이지만 단언이 아니라서 믿음을 만들지 않는다.

### 역할과 문서

<!-- check -->
```sl
module demo.role

work demo
  unit day
character king "King" major
character heir "Heir" minor
character spy "Spy" minor
place court
role crown "the Crown"
  held king until 3.1
  held heir from 3.1
artifact letter "a forged letter"
  "Your son is dead."
fact son_dead "The son is dead" false

scene 3.1 at court
  when after start
  enter spy, king
  spy writes letter
  king reads letter
```

## 지식과 믿음

- 저장하는 것은 `believes(누가, 명제, 언제부터, 원인 사건)` 뿐이다.
- **안다** = 믿는다 ∧ 그 시점에 세계에서 참. 파생해서 계산한다.
- **기만** = `lies` 발화(화자는 ¬P를 믿으면서 P를 주장한다), 또는 거짓 명제를 담은 `artifact` 를 만든 것.
- 수신자의 믿음은 기본적으로 들은 대로 바뀐다. `doubts` 로 수용하지 않음을 표시한다.
- 믿음이 행동을 바꾸는 인과는 `does X because P` 로 쓴다. 린터는 그 시점에 실제로 P를 믿었는지 확인한다.
- 정보원이 따로 있는 엔티티는 선언한다. `knows oracle`(초자연), `knows omniscient`(서술자). 이렇게 선언하면 정보 흐름 검사는 면제하되, 출처는 기록에 남긴다.
- 인물의 배경 지식(극 이전부터 아는 것)은 선언 안의 `believes P` 로 쓴다.

## 독자

- 장면은 기본적으로 파일에 적힌 순서대로 서술된다. 그 순서를 바꾸려면 `discourse after <scene>` 으로 덮어쓴다.
- 보여 주는 장면에서 일어난 발화와 지각은 그 서술 위치에서 독자에게 공개된다.
- `aside`, `thinks` 는 **독자만** 받는다.
- 기본 공개 규칙과 다르게 하려면 `withhold P until <scene>`, `reveal P at <beat>` 를 쓴다.
- 극적 아이러니는 독자는 아는데 인물은 모르는 구간이다. 미스터리는 참인데 독자가 아직 모르는 구간이다. 패턴 질의가 이 구간들을 **보고서**로 낸다(경고가 아니다).

<!-- check -->
```sl
module demo.reader

work demo
  unit day
character anna "Anna" major
character ben "Ben" minor
place hall
fact door_locked "The cellar door is locked" true
fact key_hidden "The key is hidden in the hall"

scene 4.1 at hall
  when after start
  enter anna, ben
  withhold door_locked until 4.2
  anna thinks: key_hidden "He must never learn where it is."
  setup lamp promise: key_hidden
  exit all
scene 4.2 at hall
  when after 4.1 by 0d..1d
  enter ben
  reveal door_locked
  payoff demo.reader/lamp
  pattern mystery
  render
    pov ben
    pace slow
    tension 0.8
  exit all
```

## 문법 요약

전체 문법은 [`spec/grammar.ebnf`](../spec/grammar.ebnf) 에 있다. 뼈대만 옮긴다.

```ebnf
file        = header { top } ;
header      = "module" qname NL { "import" qname NL } ;
top         = work | decl | scene | event | account | setup | distance | parent | life
            | belief | attribute | pattern | pragma ;
decl        = kind ident [string {"," string}] [tier] ["=" prop] [bool] ["tag" ident] [BLOCK(attr)] ;
kind        = "character"|"group"|"place"|"thing"|"faction"|"role"|"fact"|"artifact"|"thread" ;
scene       = "scene" sid ["at" ident] ["in" ident] [BLOCK(stmt)] ;
event       = "event" ident [string] [BLOCK(stmt)] ;              (* 무대 밖 사건 *)
account     = "account" ident "of" qname "by" ident [BLOCK(stmt)] ;
stmt        = when | lasts | presence | beat | act | belief | reader | render | setup | payoff
            | discourse | parent | life | attribute | pattern | pragma ;
when        = "when" ( ("after"|"before") tref ["by" dur ".." dur] | "at" date [ident] | "during" tref ) ;
presence    = "enter" idlist | "exit" "all" | "exit" idlist | "mention" idlist ;
beat        = "beat" ident ":" [BLOCK(stmt)] ;
act         = actor verb [ident] [idlist] ["to" idlist] ["with" proplist]
              [":" [prop]] [string] ["because" prop] ["src" (word|string)] ;
reader      = "reveal" prop ["at" tref] | "withhold" prop "until" tref ;
setup       = "setup" ident ("promise"|"hint"|"symbol") ":" (prop|string) ;
payoff      = "payoff" qname ;
pragma      = "#!" "allow" ruleid ":" string ;                    (* 바로 다음 문장에 붙는다 *)
render      = "render" BLOCK(rkey rvalue) ;
rkey        = "pov"|"pace"|"tension"|"focus"|"omit"|"show" ;
```

## 의도적으로 뺀 것

- 조건 분기와 변수 — 이 언어는 인터랙티브 픽션(Ink)이 아니다. 실행하지 않고 **기술**한다.
- 문장 단위 산문 — 렌더러가 만든다.
- 확률적 믿음 강도 — 필요한 사례가 나오면 `believes P 0.7` 을 검토한다.
