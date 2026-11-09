> 공개용 스냅샷(2026-11-09). 설계 기준은 프로젝트 설계 노트.

# 파일 형식 — 프론트매터 + 언어 블록 + 산문 본문

> 한 파일 = 한 모듈 = `.dieg.md`. 마크다운 뷰어가 그대로 열 수 있다.
> 프론트매터는 언어의 **문법 설탕**이다. 언어 블록과 똑같은 IR로 컴파일되고, 같은 정보를 두 곳에 쓰면 컴파일 오류가 난다. → [language](language.md)

## 구조

````markdown
---                                  ← ① 프론트매터: 이 파일 자신의 정적 속성
scene: "1.3"
location: heath
when: after 1.2 by 0d..1d
characters: [witches, macbeth, banquo]
pov: macbeth
mode: authored
---

```sl                                ← ② 언어 블록: 동적 요소 (여러 개 가능, 순서대로 이어 붙는다)
beat prophecy:
  witch1 hails macbeth: holds(thane_of_glamis, macbeth)
  ...
exit witches
enter ross, angus
```

Upon the blasted heath the three shapes rose out of the fog...   ← ③ 산문 본문
````

| 층 | 담는 것 | 담지 않는 것 |
|---|---|---|
| ① 프론트매터 | 파일 단위의 속성: 모듈, import, 장면 ID, 장소, 시점 제약, **장면 내내** 있는 인물, 언급만 되는 인물, 시점 인물, 렌더 지시어, 작업 상태, 산문 모드. 선언 파일이면 엔티티 목록도 | 발화, 지각, 믿음, 기만, 인과, 중간 등장·퇴장, 역할 보유 구간처럼 다른 사건을 참조하는 시간 관계 |
| ② 언어 블록 | 위에서 프론트매터에 담지 않는 것 전부 | — |
| ③ 산문 | 작가가 쓴 문장 또는 렌더러가 생성한 문장 | 의미. 산문은 검사 대상이 아니다 |

경계를 정하는 규칙은 하나다: **다른 사건을 참조하지 않는 이 파일 자신의 속성이면 프론트매터에 쓸 수 있다.** 시간 관계는 언어 블록에 쓴다. 예외로, 장면 자신의 `when` 은 프론트매터에 허용한다.

- 프론트매터는 1행이 `---` 이어야 하고 다음 `---` 줄에서 닫힌다. 언어 블록은 0열에서 시작하는 ```` ```sl ```` ~ ```` ``` ```` 다.
- 확장자가 `.dieg` 이면 파일 전체가 언어 소스다(프론트매터·산문 없음).

### 최소 파일

프론트매터 없이 언어 블록만으로도 한 파일이 된다. 이 경우 첫 문장이 `module` 이다. 이 저장소의 문서 예제 시험(`TestDocsExamplesCompile`, `<!-- check -->` 표시가 붙은 블록)은 `module` 로 시작하는 블록은 그대로, 아니면 `module: docs.ex` 프론트매터를 붙여서 `.dieg.md` 한 파일로 컴파일한다.

<!-- check -->
```sl
module demo.minimal

work demo
  unit day
character anna "Anna" major
place hall

scene 1 at hall
  when after start
  enter anna
  anna says: "Hello." src 1.1
  exit all
```

## 설탕 → 언어 대응표 (명세의 일부)

모든 키에는 그것과 동등한 언어 문장이 정의되어 있다. 컴파일러는 파싱 직후에 프론트매터를 이 문장들로 바꾸고, 그 뒤 단계에서는 둘을 구분하지 않는다.

| 프론트매터 | 동등한 언어 문장 |
|---|---|
| `module: macbeth.act1` | `module macbeth.act1` |
| `import: [macbeth.cast]` | `import macbeth.cast` |
| `scene: "1.3"` + `location: heath` | `scene 1.3 at heath` (블록 전체가 이 장면의 몸체가 된다) |
| `when: after 1.2 by 0d..1d` (목록 가능) | `when after 1.2 by 0d..1d` |
| `date: 1805-07-?` / `time: dawn` | `when at 1805-07-? dawn` |
| `characters: [a, b]` | 장면 첫 줄에 `enter a, b`, 끝에 `exit all` |
| `mentions: [cawdor]` | `mention cawdor` (출석이 아니다) |
| `pov: macbeth` | `render` 블록의 `pov macbeth` |
| `render: {pace: slow, tension: 0.8}` | `render` 블록의 같은 키 |
| `thread: scotland` | `scene … in scotland` |
| `discourse: after 1.1` | `discourse after 1.1` |
| `characters:` 맵 (선언 파일) `macbeth: {name: Macbeth, tier: major}` | `character macbeth "Macbeth" major` |
| `places:` / `things:` / `factions:` / `facts:` / `groups:` 맵 | 각각의 선언 |
| `status`, `mode`, `rendered` | 메타데이터. 문장을 만들지 않는다 |

생성되는 문장의 위치는 프론트매터의 그 줄이다. 그래서 오류 위치가 원본 파일의 줄을 가리킨다.

### 한 곳에만 존재하게 하는 규칙

- **같은 사실을 두 곳에 쓰면 오류(`dup-decl`)다.** `characters` 에 있는 인물을 블록에서 다시 `enter` 하거나, `scene` 키와 블록의 `scene …` 머리줄이 동시에 있으면 오류가 난다.
- 그래서 장면 중간에 들어오는 인물은 **블록에만** 쓴다. `characters` 는 "처음부터 끝까지 있는 인물"이라는 뜻이다. `characters: []` 는 "아무도 없음"이다.
- 전체 출석부처럼 두 층을 합친 값은 **계산된 출력**이다. 손으로 쓰지 않는다.

### YAML 함정

- `scene: 1.3` 을 YAML은 실수로 읽고, `1.10` 은 `1.1` 로 바뀐다. `no` 는 false가 된다(Norway 문제).
- 그래서 컴파일러는 YAML을 타입 추론 없이 **노드 그대로** 읽는다. 스칼라는 전부 원문 문자열로 받은 뒤 위 대응표의 타입으로 직접 해석한다. 사용자가 따옴표를 빠뜨려도 결과가 같다.
- 앵커, 별칭, 병합 키(`&`, `*`, `<<`)는 금지한다(`yaml-anchor`). 정보가 두 곳에 있는 것처럼 보이게 만들기 때문이다. 모르는 키는 `unknown-key` 오류다.
- 값은 한 줄이어야 한다(블록 스칼라와 여러 줄 스칼라 불가).

## 산문 본문의 두 모드

| | `mode: authored` (작가 집필) | `mode: rendered` (렌더 생성) |
|---|---|---|
| 본문의 주인 | 작가 | 렌더러 |
| 컴파일러가 본문에 하는 일 | **손대지 않는다.** 구조만 검사한다 | 같다 |
| 기본값 | 본문이 있으면 이 모드 | — |

- **비트 앵커** `<!-- beat:prophecy -->`(선택): 미리보기에 보이지 않는 HTML 주석으로, 한 줄 전체가 이것이어야 한다. 산문의 위치를 비트에 묶는다.
- **블록을 산문 사이에 끼워 써도 된다.** 문단과 문단 사이에 ```` ```sl ```` 블록을 두면 순서대로 이어 붙는다.
- 렌더 산문의 낡음(`stale-prose`)과 손으로 고친 흔적(`rendered-edited`) 검사, 산문에서 구조를 재추출해 diff 하는 `verify-prose` 는 설계에만 있고 구현하지 않았다. 현재 렌더 산문의 낡음은 `dieg revise` 가 장면의 렌더 요청 해시와 세계 상태 요약 해시를 비교해서 판단한다 → [rendering](rendering.md)

## 디렉터리 관례

```
corpus/macbeth/
  cast.dieg.md          선언만 (프론트매터 맵 + 역할·사실은 블록)
  act1/
    1.2.dieg.md  1.3.dieg.md  1.5.dieg.md  offstage.dieg.md
  texts/              퍼블릭 도메인 원문 (src 줄 참조 대상)
  expect.yaml         합격 기준 (dieg test)
```

파일 하나에 장면 하나가 기본이다. 짧은 장면 여러 개를 한 파일에 넣을 때는 프론트매터를 쓰지 않고 블록 안에 `scene` 을 쓴다.

## 기존 도구와의 호환 (설계 단계 조사)

Story Skills(`danjdewhurst/story-skills`, MIT)는 장·장면·인물·복선·지식 상태를 YAML 프론트매터 마크다운으로 저장하는 소설 집필 스킬 묶음이다. 개념이 같은 키는 이름도 같게 했다(`location`, `characters`, `mentions`, `pov`, `date`, `time`, `status`). 그래서 그쪽 장면 프론트매터는 거의 그대로 이쪽 프론트매터로 읽힌다.

- 가져오기는 가능하다. 그쪽에 없는 것(발화, 기만, 상대 시간, 비트 위치)만 비어 있다.
- 내보내기는 IR에서 만드는 투영이다. 기만, 시간 제약, 독자 일정은 사라진다.
- 양방향 동기화는 하지 않는다. 정보가 두 곳에 생긴다.
- `dieg import story-skills` / `dieg export story-skills` 는 설계만 있고 구현하지 않았다.
