#!/usr/bin/env bash
# b-render.sh <story-dir> <out-dir> <backend> <scene...>: dieg render 를 장면마다 한 번(구독 claude 1회). calls.tsv 에 기록.
# 렌더 프롬프트는 렌더러가 만든다(internal/render/prompt.go). 저장소 루트에서 돈다(styles/).
set -euo pipefail
here=$(cd "$(dirname "$0")/.." && pwd); story=$(realpath "$1"); out=$(realpath -m "$2"); b=$3; shift 3
mkdir -p "$out"; cd "$here/../.."
for s in "$@"; do
  ts=$(date -Iseconds); start=$(date +%s)
  "${DIEG:-dieg}" render "$story" "$s" --backend "$b" -o "$out/$s.$b.md"
  printf '%s\tB\trender-%s-%s\t(dieg render %s)\t%s\t%s\tend_turn\n' "$ts" "$b" "$s" "$s" "$(realpath --relative-to="$here" "$out/$s.$b.md")" $(( $(date +%s) - start )) >> "$here/calls.tsv"
done
