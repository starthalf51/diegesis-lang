#!/usr/bin/env bash
# unpack.sh <raw-output> <dir>: "=== FILE: path ===" 머리줄로 나뉜 모델 출력을 파일로 푼다(내용은 손대지 않는다).
set -euo pipefail
rm -rf "$2"; mkdir -p "$2"
awk -v d="$2" '/^=== FILE: .* ===$/ { f=$0; sub(/^=== FILE: /,"",f); sub(/ ===$/,"",f); out=d"/"f; system("mkdir -p \"$(dirname \""out"\")\""); printf "" > out; next }
  out { print >> out }' "$1"
find "$2" -type f | sort
