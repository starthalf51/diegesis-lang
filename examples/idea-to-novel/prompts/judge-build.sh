#!/usr/bin/env bash
# judge-build.sh <A-scenes-glob-dir> <A-suffix> <B-dir> <B-suffix> <X=A|B> <out-prompt>
# 두 소설을 X·Y 로 이름만 붙여 넣는다(어느 쪽이 어느 방식인지 판정자는 모른다). 체크리스트는 고정.
set -euo pipefail
cd "$(dirname "$0")/.."
ad=$1 as=$2 bd=$3 bs=$4 x=$5 out=$6
novel() { for i in $(seq 1 9); do printf '### 장면 %d\n\n' $i; cat "$1/s$i$2"; echo; done; }
{
cat <<'H'
# 과제: 두 소설의 연속성 오류 판정

같은 아이디어로 쓴 한국어 소설 두 편(X, Y)이 있다. 각 소설 안에서 **객관적인 연속성 오류**만 찾는다.
문체·재미·완성도는 판정하지 않는다. 두 소설을 서로 비교하지 않는다(서로 다른 이야기다).

## 체크리스트 (이 일곱 가지만)
1. knowledge — 인물이 그 시점에 알 수 없는 것을 안다(본 적도 들은 적도 없는 사실을 말하거나 그에 따라 행동한다)
2. absent-actor — 죽었거나 그 자리에 없는 인물이 말하거나 행동한다
3. timeline — 시각·순서·알리바이가 서로 모순된다
4. attribute — 나이·이름·관계·물건·장소 등 속성이 앞뒤로 다르다
5. unpaid-setup — 분명히 심어 둔 복선(물건·약속·단서)이 끝까지 회수되지 않는다
6. unsetup-payoff — 앞에서 심지 않은 것이 결정적 해결에 쓰인다
7. secret — 비밀·거짓말이 드러나는 방식이 앞뒤로 맞지 않는다(이미 드러난 것을 모르는 척하거나, 드러나지 않았는데 아는 척한다)

## 규칙
- 오류마다 소설 원문을 **그대로** 인용한다(한 글자도 바꾸지 않는다). 모순이면 서로 부딪히는 두 곳을 모두 인용한다.
- 해석으로 설명이 되는 것(인물의 의도적 거짓말, 서술상 생략, 독자에게 감춘 것)은 오류가 아니다.
- 확신이 없으면 넣지 않는다.

## 출력 (JSON 만, 다른 글 없이)
{"X": [{"category": "...", "scene": 3, "quotes": ["...", "..."], "why": "..."}], "Y": [...]}

H
if [ "$x" = A ]; then printf '\n## 소설 X\n\n'; novel "$ad" "$as"; printf '\n## 소설 Y\n\n'; novel "$bd" "$bs"
else printf '\n## 소설 X\n\n'; novel "$bd" "$bs"; printf '\n## 소설 Y\n\n'; novel "$ad" "$as"; fi
} > "$out"
