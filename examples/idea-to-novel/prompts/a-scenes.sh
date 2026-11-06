#!/usr/bin/env bash
# a-scenes.sh <outline> <out-dir> <prompt-prefix> [from] [to]: 장면을 하나씩 쓴다. 호출마다 개요 + 직전 장면만 본다.
set -euo pipefail
cd "$(dirname "$0")/.."
outline=$1 dir=$2 pp=$3 from=${4:-1} to=${5:-9}
mkdir -p "$dir"
for i in $(seq "$from" "$to"); do
  p="prompts/${pp}-s$i.txt"
  {
  printf '# 과제: 소설의 장면 %d/9 를 쓴다\n\n' "$i"
  cat prompts/common-requirements.txt
  printf '\n## 아이디어\n'; cat idea.txt
  printf '\n## 개요\n\n'; cat "$outline"
  if [ "$i" -gt 1 ]; then printf '\n## 직전 장면 (장면 %d)\n\n' $((i-1)); cat "$dir/s$((i-1)).md"; fi
  printf '\n## 지시\n개요의 장면 %d 를 한국어 산문으로 쓴다. 2500자 안팎. 해라체 서술. 제목·머리말·설명 없이 산문만 출력한다.\n' "$i"
  } > "$p"
  prompts/call.sh A "${pp}-s$i" "$p" "$dir/s$i.md"
done
