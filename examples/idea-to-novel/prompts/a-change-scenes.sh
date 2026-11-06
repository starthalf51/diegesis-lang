#!/usr/bin/env bash
# a-change-scenes.sh: 고친 개요로 장면을 하나씩 다시 쓴다. 호출마다 고친 개요 + 원래 장면 + 직전(고친) 장면.
set -euo pipefail
cd "$(dirname "$0")/.."
mkdir -p a-direct/change/scenes
for i in $(seq 1 9); do
  p="prompts/a4-change-s$i.txt"
  {
  printf '# 과제: 이미 쓴 소설의 장면 %d/9 를 수정 요청에 맞게 다시 쓴다\n\n' "$i"
  cat prompts/common-requirements.txt
  printf '\n## 아이디어\n'; cat idea.txt
  echo; cat prompts/change.txt
  printf '\n## 고친 개요\n\n'; cat a-direct/change/outline.md
  printf '\n## 원래 장면 %d\n\n' "$i"; cat "a-direct/scenes/s$i.md"
  if [ "$i" -gt 1 ]; then printf '\n## 직전 장면 (고친 장면 %d)\n\n' $((i-1)); cat "a-direct/change/scenes/s$((i-1)).md"; fi
  printf '\n## 지시\n고친 개요에 맞게 장면 %d 를 다시 쓴다. 바꿀 필요가 없는 부분은 원래 장면을 살린다. 2500자 안팎. 해라체 서술. 산문만 출력한다.\n' "$i"
  } > "$p"
  prompts/call.sh A "a4-change-s$i" "$p" "a-direct/change/scenes/s$i.md"
done
