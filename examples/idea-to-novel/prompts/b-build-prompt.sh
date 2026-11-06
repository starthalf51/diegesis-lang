#!/usr/bin/env bash
# B 1단계 프롬프트를 조립한다: 지시 + 공통 요구 + 아이디어 + 참고 자료(문법, README, 맥베스 예제 2개).
set -euo pipefail
cd "$(dirname "$0")/.."; R=../..
{
cat <<'H'
# 과제: 아이디어를 Diegesis 코드로 쓴다

Diegesis 는 소설을 위한 프로그래밍 언어다. 인물·사실·장면·행위(tells, says, lies, sees …)를 .dieg.md 파일에 적으면
컴파일러 `dieg check` 와 PostgreSQL 린트 `dieg lint` 가 검사하고(누가 무엇을 아는가, 시간 모순, 죽은 인물의 행동,
속성 충돌, 회수되지 않은 복선 …), 렌더러 `dieg render` 가 장면마다 산문을 쓴다. 너는 이야기를 코드로 쓴다.
산문은 렌더러가 쓰므로 너는 산문을 쓰지 않는다.

H
cat prompts/common-requirements.txt
printf '\n## 아이디어\n'; cat idea.txt
cat <<'H'

## 코드 작성 규칙
- 작품 ID 는 `bungyo`. 파일: `cast.dieg.md`(module bungyo.cast: work·인물·장소·물건·사실·역할 선언),
  `scenes/s1.dieg.md` … `scenes/s9.dieg.md`(module bungyo.scenes, import bungyo.cast). 장면 ID 는 "s1" … "s9".
  필요하면 `scenes/offstage.dieg.md`(무대 밖 사건, event)를 둔다.
- 모든 인물·장소·물건에 `name ko "…"` 를 단다. 사실(fact)의 문자열과 대사(인용 문자열)는 한국어로 쓴다 — 렌더러가 그대로 쓴다.
- 렌더러는 장면의 행위(act)만 순서대로 산문으로 옮긴다. 산문에 나와야 할 일은 전부 행위로 적는다. 장면마다 행위 12–20개.
- 장면마다 프론트매터에 `pov:` 를 둔다(시점 인물). `render:` 의 pace·tension 도 둔다.
- 시각과 알리바이는 `when`/`lasts`(분 단위 dur: 10m)와 `attr`(예: attr bell.rung = "아니오")로 적는다. 죽음은 `dies`.
- 거짓말은 `lies`, 믿음은 `believes`/`doubts`, 독자에게 감출 것은 `withhold P until <시점>`, 밝힐 것은 `reveal`.
- 앞에서 심은 물건·약속은 `setup <id> promise: …` 로, 회수는 `payoff bungyo.scenes/<id>` 로 적는다.
- `#! allow` 는 이야기가 일부러 의도한 경우에만, 사유와 함께 쓴다. 오류를 숨기려고 쓰지 않는다.
- 목표: `dieg check` 오류 0, `dieg lint` 경고 0(억제 사유가 있는 것 제외).

## 출력 형식
파일마다 아래처럼 머리줄을 쓰고 그 다음 줄부터 파일 내용을 그대로 쓴다. 다른 설명은 쓰지 않는다. 코드 펜스로 전체를 감싸지 않는다
(파일 안의 ```sl 블록은 그대로 둔다).

=== FILE: cast.dieg.md ===
(내용)
=== FILE: scenes/s1.dieg.md ===
(내용)

# 참고 자료
H
printf '\n## spec/grammar.ebnf\n\n'; cat $R/spec/grammar.ebnf
printf '\n## README.md\n\n'; cat $R/README.md
printf '\n## 린트 규칙 (rules/*.sql 머리말)\n\n'; for f in $R/rules/*.sql; do sed -n '1,5p' "$f"; echo; done
printf '\n## 예제: corpus/macbeth/cast.dieg.md\n\n'; cat $R/corpus/macbeth/cast.dieg.md
printf '\n## 예제: corpus/macbeth/act1/1.3.dieg.md\n\n'; cat $R/corpus/macbeth/act1/1.3.dieg.md
} > prompts/b1-code.txt
