#!/usr/bin/env bash
# call.sh <arm> <step> <prompt-file> <out-file>
# 구독 claude CLI 한 번 호출. 렌더러(internal/render/claudecli.go)와 같은 플래그, 빈 임시 디렉터리에서 돈다.
# 시스템 프롬프트는 두 팔 공통으로 고정. 호출마다 calls.tsv 에 한 줄(arm step prompt out 시작 초 stop_reason).
set -euo pipefail
arm=$1 step=$2 prompt=$(realpath "$3") out=$(realpath -m "$4")
here=$(cd "$(dirname "$0")/.." && pwd)
sys="You are a careful Korean novelist and story engineer. Follow the user's instructions exactly. Output only what is asked."
tmp=$(mktemp -d); start=$(date +%s); ts=$(date -Iseconds)
(cd "$tmp" && claude -p --model claude-opus-5-5 --effort high --tools "" --strict-mcp-config --setting-sources "" \
  --no-session-persistence --output-format json --system-prompt "$sys" < "$prompt") > "$tmp/raw.json"
node -e 'const o=JSON.parse(require("fs").readFileSync(process.argv[1],"utf8"));if(o.is_error){console.error(o);process.exit(1)}require("fs").writeFileSync(process.argv[2],o.result+"\n");console.log(o.stop_reason)' "$tmp/raw.json" "$out" > "$tmp/stop"
secs=$(( $(date +%s) - start ))
rel() { realpath --relative-to="$here" "$1"; }
printf '%s\t%s\t%s\t%s\t%s\t%s\t%s\n' "$ts" "$arm" "$step" "$(rel "$prompt")" "$(rel "$out")" "$secs" "$(cat "$tmp/stop")" >> "$here/calls.tsv"
rm -rf "$tmp"
