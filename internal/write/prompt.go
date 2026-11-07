package write

import (
	"fmt"
	"strings"
)

// System is short on purpose: the claude CLI takes the system prompt as an argument (Windows caps a command
// line near 32K chars), so the reference material rides in the user prompt on stdin.
const System = "You write novels as Diegesis code (.dieg.md). You output only a file bundle in the exact format the user specifies — no prose commentary."

// task is the coding instruction, adapted from examples/idea-to-novel/prompts/b-build-prompt.sh and
// common-requirements.txt (the demo whose code reached 0 errors / 0 warns in two fix rounds).
const task = `# 과제: 아이디어를 Diegesis 코드로 쓴다

Diegesis 는 소설을 위한 프로그래밍 언어다. 인물·사실·장면·행위(tells, says, lies, sees …)를 .dieg.md 파일에 적으면
컴파일러 ` + "`dieg check`" + ` 와 PostgreSQL 린트 ` + "`dieg lint`" + ` 가 검사하고(누가 무엇을 아는가, 시간 모순, 죽은 인물의 행동,
속성 충돌, 회수되지 않은 복선 …), 렌더러가 장면마다 산문을 쓴다. 너는 이야기를 코드로 쓴다.
산문은 렌더러가 쓰므로 너는 산문을 쓰지 않는다.

## 이야기 요구 사항
- 아래 아이디어 하나로 한국어 중편 소설을 만든다. 아이디어가 따로 정하지 않으면 장면 9개, 등장인물 6–8명.
- 긴 이야기의 일관성이 핵심이다: 비밀, 일부 인물만 믿는 거짓말, 누가 언제 무엇을 아는가,
  시각·알리바이, 앞에서 심어 둔 물건이 끝에서 회수되는 것.
- 인물의 이름·나이·관계·물건·시각은 처음 정한 대로 끝까지 지킨다.

## 코드 작성 규칙
- 작품 ID(` + "`work <id> \"제목\"`" + `)는 이야기에 맞는 새 영문 소문자 ID 로 정한다. 기존 작품 ID 는 쓰지 않는다: %s.
  파일: ` + "`cast.dieg.md`" + `(module <id>.cast: work·인물·장소·물건·사실·역할 선언),
  ` + "`scenes/s1.dieg.md`" + ` … (module <id>.scenes, import <id>.cast). 장면 ID 는 "s1", "s2" ….
  필요하면 ` + "`scenes/offstage.dieg.md`" + `(무대 밖 사건, event)를 둔다. 무대 밖 사건 안에는 enter/exit 를 쓰지 않는다.
- 모든 인물·장소·물건에 ` + "`name ko \"…\"`" + ` 를 단다. 사실(fact)의 문자열과 대사(인용 문자열)는 한국어로 쓴다 — 렌더러가 그대로 쓴다.
- 렌더러는 장면의 행위(act)만 순서대로 산문으로 옮긴다. 산문에 나와야 할 일은 전부 행위로 적는다. 장면마다 행위 8–12개. 더 길어지면 장면을 나눈다.
- 장면은 떠나는 모든 인물의 명시적 ` + "`exit`" + ` 행위(또는 ` + "`exit all`" + `)로 끝낸다. 렌더러가 퇴장을 지어내지 않게 한다.
- 인용 문자열(대사·생각)이 선언된 사실을 말하면 반드시 그 사실을 단다: ` + "`doha says seojin: road_cut \"길이 끊겼대.\"`" + `.
  사실을 달지 않은 인용에는 선언된 사실을 담지 않는다. 재추출이 그것을 코드에 없는 사실 행위로 세어 렌더가 실패한다.
- 장면마다 프론트매터에 ` + "`pov:`" + ` 를 둔다(시점 인물). ` + "`render:`" + ` 의 pace·tension 도 둔다. 장면에는 ` + "`location:`" + ` 을 둔다.
- 시각과 알리바이는 ` + "`when`/`lasts`" + `(분 단위 dur: 10m)와 ` + "`attr`" + `(예: attr bell.rung = "아니오")로 적는다. 죽음은 ` + "`dies`" + `.
- 물건의 위치는 ` + "`attr <물건>.at = <장소>`" + ` 로 적는다(` + "`X.at`" + ` 규약). 그 물건을 다루는 장면의 location 이 다르면
  ` + "`object-location`" + ` 이 경고한다. ` + "`.at`" + ` 은 이야기 전체에 한 곳뿐이다(두 번 적으면 ` + "`attribute-conflict`" + `).
  그러니 한자리에 머무는 물건에만 적고, 인물이 들고 다니거나 장면 사이에 옮겨지는 물건에는 ` + "`.at`" + ` 을 적지 않는다.
- 말하기 동사를 구분한다:
  - ` + "`says`" + ` = 화자가 참이라고 믿고 하는 말. 그 사실이 실제로 거짓이어도(착각) says 다.
  - ` + "`lies`" + ` = 화자가 거짓인 줄 알면서 하는 말.
  - ` + "`thinks`" + ` = 소리 내지 않은 생각. 다른 인물은 듣지 못한다.
- 거짓인 사실(누가 믿거나 거짓말로 퍼뜨리는 사실)은 거짓이라고 선언한다: ` + "`fact alibi \"…\" false`" + `. 그래야 렌더러의
  서술자가 그 사실을 참으로 단정하지 않는다. 거짓 사실을 ` + "`reveal`" + ` 하지 않는다(` + "`reveal-false`" + ` 경고).
- 믿음은 ` + "`believes`/`doubts`" + `, 독자에게 감출 것은 ` + "`withhold P until <시점>`" + `, 밝힐 것은 ` + "`reveal`" + `.
- 앞에서 심은 물건·약속은 ` + "`setup <id> promise: …`" + ` 로, 회수는 ` + "`payoff <id>.scenes/<id>`" + ` 로 적는다.
- 인물이 남에게 전하는 사실은 그 전에 그 인물이 보거나 듣거나 겪었어야 한다(` + "`info-flow`" + `).
- ` + "`#! allow`" + ` 는 이야기가 일부러 의도한 경우에만, 사유와 함께 쓴다. 오류를 숨기려고 쓰지 않는다.
- 목표: ` + "`dieg check`" + ` 오류 0, ` + "`dieg lint`" + ` 경고 0(억제 사유가 있는 것 제외).

## 출력 형식 (반드시 지킨다)
파일마다 아래처럼 머리줄을 쓰고 그 다음 줄부터 파일 내용을 그대로 쓴다. 마지막 파일 뒤에 ` + "`" + End + "`" + ` 한 줄로 끝낸다.
다른 설명은 쓰지 않는다. 코드 펜스로 전체를 감싸지 않는다(파일 안의 ` + "```sl" + ` 블록은 그대로 둔다).
경로는 상대 경로이고 확장자는 .dieg.md 다. 빈 파일을 쓰지 않는다.

=== FILE: cast.dieg.md ===
(내용)
=== FILE: scenes/s1.dieg.md ===
(내용)
` + End + `
`

func refsBlock(r Refs) string {
	var b strings.Builder
	b.WriteString("\n# 참고 자료\n\n## spec/grammar.ebnf\n\n" + r.Grammar + "\n## 키워드와 린트 규칙\n\n" + r.Keywords)
	names := []string{"corpus/macbeth/cast.dieg.md", "corpus/macbeth/act1/1.3.dieg.md"}
	for i, e := range r.Examples {
		n := fmt.Sprintf("example %d", i+1)
		if i < len(names) {
			n = names[i]
		}
		b.WriteString("\n## 예제: " + n + "\n\n" + e)
	}
	return b.String()
}

// CodePrompt is round 1: instructions, the idea, then the references.
func CodePrompt(idea string, r Refs, reserved []string, skeleton bool) string {
	t := task
	if skeleton {
		t = strings.Replace(t, quoteRule, skeletonRule, 1)
	}
	return fmt.Sprintf(t, strings.Join(reserved, ", ")) + "\n## 아이디어\n\n" + strings.TrimSpace(idea) + "\n" + refsBlock(r)
}

// FixPrompt is rounds 2…: round 1's prompt, the current bundle, and the diagnostics verbatim.
func FixPrompt(idea string, r Refs, reserved []string, skeleton bool, bundle string, diags []Diag) string {
	var b strings.Builder
	b.WriteString(CodePrompt(idea, r, reserved, skeleton))
	b.WriteString("\n# 네가 앞서 쓴 코드\n\n")
	if bundle == "" {
		b.WriteString("(아직 형식에 맞는 코드가 없다)\n")
	} else {
		b.WriteString(bundle)
	}
	b.WriteString("\n# 도구 진단 (dieg check, 그다음 dieg lint — 원문 그대로)\n\n")
	for _, d := range diags {
		b.WriteString(d.String() + "\n")
	}
	b.WriteString(`
# 지시
위 진단을 모두 해결하도록 코드를 고쳐라. 진단이 이야기의 실제 모순을 가리키면 이야기를 고치고,
코드를 잘못 쓴 것이면 코드를 고친다. 의도한 것이면 ` + "`#! allow`" + ` 와 사유를 단다. ` + "`bundle`" + ` 진단은 출력 형식 위반이다.
같은 출력 형식으로 **모든 파일**을 다시 출력하고 ` + End + ` 로 끝낸다.
`)
	return b.String()
}

// quoteRule is the non-skeleton quote instruction; skeletonRule replaces it in skeleton mode.
const quoteRule = "- 인용 문자열(대사·생각)이 선언된 사실을 말하면 반드시 그 사실을 단다: `doha says seojin: road_cut \"길이 끊겼대.\"`.\n  사실을 달지 않은 인용에는 선언된 사실을 담지 않는다. 재추출이 그것을 코드에 없는 사실 행위로 세어 렌더가 실패한다.\n"

const skeletonRule = "- 뼈대 모드: 행위에 인용 문자열을 쓰지 않는다(`doha says seojin: road_cut` 처럼 쓴다). 대사는 렌더러가 쓴다. 예외는 원작의 상징적 대사가 아이템에 명시된 경우뿐이다.\n  말·생각 행위(says, lies, thinks, tells)는 반드시 사실을 달거나, 아무 사실도 전하지 않는 행위로 둔다. 렌더러는 그 사실만 말한다.\n"
