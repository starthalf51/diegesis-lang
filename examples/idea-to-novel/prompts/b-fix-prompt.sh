#!/usr/bin/env bash
# b-fix-prompt.sh <story-dir> <log-prefix> <out-prompt>: 1단계 프롬프트 + 지금 코드 + 도구 진단(그대로) → 수정 요청.
set -euo pipefail
cd "$(dirname "$0")/.."
{
cat prompts/b1-code.txt
printf '\n# 네가 앞서 쓴 코드\n\n'
for f in $(cd "$1" && find . -name '*.dieg.md' | sed 's|^\./||' | sort); do printf '=== FILE: %s ===\n' "$f"; cat "$1/$f"; done
printf '\n# 도구 진단 (dieg check --json, 그다음 dieg lint — 출력 그대로)\n\n'
cat "$2.check.json"; echo; cat "$2.lint.txt"
cat <<'H'

# 지시
위 진단을 모두 해결하도록 코드를 고쳐라. 진단이 이야기의 실제 모순을 가리키면 이야기를 고치고,
코드를 잘못 쓴 것이면 코드를 고친다. 의도한 것이면 `#! allow` 와 사유를 단다. 같은 출력 형식으로 **모든 파일**을 다시 출력한다.
H
} > "$3"
