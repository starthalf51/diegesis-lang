#!/usr/bin/env bash
# b-check.sh <story-dir> <log-prefix>: dieg check --json 과 dieg lint 를 돌려 출력을 그대로 남긴다.
set -uo pipefail
dieg=${DIEG:-dieg}
"$dieg" check "$1" --json > "$2.check.json"; echo "check exit $?" > "$2.exit"
"$dieg" lint "$1" > "$2.lint.txt" 2>&1; echo "lint exit $?" >> "$2.exit"
cat "$2.exit"
