#!/bin/sh
# Diegesis quickstart: docker pg -> dieg -> migrate -> demo. No model calls.
# Flags: --skip-docker (PG already reachable at the DSN), --print-release-url [tag] (no download)
set -eu

REPO=starthalf51/diegesis-lang
DSN='postgres://sl:sl@localhost:55432/sl?sslmode=disable'
SKIP_DOCKER=0; PRINT_URL=0; TAG=${QUICKSTART_TAG:-}
while [ $# -gt 0 ]; do
  case $1 in
    --skip-docker) SKIP_DOCKER=1 ;;
    --print-release-url) PRINT_URL=1; if [ $# -gt 1 ]; then TAG=$2; shift; fi ;;
    *) echo "unknown option: $1" >&2; exit 2 ;;
  esac
  shift
done
die() { echo "quickstart: $*" >&2; exit 1; }
HINT="install Go 1.27.3, or run again after a Release is published"

# release_asset: sets URL and ASSET for this OS/arch (finds the latest tag unless TAG given)
release_asset() {
  case $(uname -s) in Linux) os=linux; ext= ;; Darwin) os=darwin; ext= ;; MINGW*|MSYS*|CYGWIN*) os=windows; ext=.exe ;; *) die "unsupported OS: $(uname -s) - $HINT" ;; esac
  case $(uname -m) in x86_64|amd64) arch=amd64 ;; aarch64|arm64) arch=arm64 ;; *) die "unsupported CPU: $(uname -m) - $HINT" ;; esac
  if [ -z "$TAG" ]; then
    TAG=$(curl -fsSL -o /dev/null -w '%{url_effective}' "https://github.com/$REPO/releases/latest" | sed 's|.*/tag/||') || TAG=
    case $TAG in v*) ;; *) die "cannot find the latest release of $REPO - $HINT" ;; esac
  fi
  ASSET="dieg_${TAG#v}_${os}_${arch}${ext}"
  BASE=${QUICKSTART_RELEASE_BASE:-"https://github.com/$REPO/releases/download/$TAG"}
  [ -z "${QUICKSTART_RELEASE_BASE:-}" ] || echo "quickstart: QUICKSTART_RELEASE_BASE is set - checksums come from the same place, only the provenance check protects this download" >&2
  URL="$BASE/$ASSET"
}

if [ $PRINT_URL = 1 ]; then release_asset; echo "$ASSET"; echo "$URL"; exit 0; fi

cd "$(dirname "$0")/.."

# (1) docker + compose v2
if [ $SKIP_DOCKER = 0 ]; then
  command -v docker >/dev/null 2>&1 || die "docker not found - install Docker Desktop or Docker Engine first"
  docker compose version >/dev/null 2>&1 || die "docker compose v2 not found - update Docker (the 'docker compose' plugin is required)"
fi

# (2) dieg: PATH -> go build -> release download
if command -v dieg >/dev/null 2>&1 && dieg 2>&1 | grep -q "| demo"; then DIEG=dieg   # a stale dieg without demo falls through
elif command -v go >/dev/null 2>&1; then
  mkdir -p bin; go build -o ./bin/dieg ./cmd/dieg || die "go build failed"; DIEG=./bin/dieg
else
  command -v curl >/dev/null 2>&1 || die "no dieg, no Go, no curl: install Go 1.27.3 or wait for a public Release, then run again"
  release_asset
  mkdir -p bin; DIEG=./bin/dieg$ext
  curl -fsSL -o "$DIEG" "$URL" && curl -fsSL -o bin/checksums.txt "$BASE/checksums.txt" || { rm -f "$DIEG"; die "download failed - $HINT"; }
  want=$(grep "[ *]$ASSET\$" bin/checksums.txt | cut -d' ' -f1)
  if command -v sha256sum >/dev/null 2>&1; then have=$(sha256sum "$DIEG" | cut -d' ' -f1)
  elif command -v shasum >/dev/null 2>&1; then have=$(shasum -a 256 "$DIEG" | cut -d' ' -f1)
  else rm -f "$DIEG"; die "neither sha256sum nor shasum found, cannot verify the download - $HINT"; fi
  [ -n "$want" ] && [ "$want" = "$have" ] || { rm -f "$DIEG"; die "checksum mismatch for $ASSET"; }
  # build provenance (GitHub attestation); needs a logged-in gh
  if command -v gh >/dev/null 2>&1 && gh auth status >/dev/null 2>&1; then
    gh attestation verify "$DIEG" --repo "$REPO" --signer-workflow "$REPO/.github/workflows/release.yml" >/dev/null 2>&1 || { rm -f "$DIEG"; die "build provenance check failed for $ASSET"; }
    echo "provenance verified (gh attestation)"
  else echo "provenance not verified (install and log in to gh to verify)"; fi
  chmod +x "$DIEG"
fi
echo "dieg: $DIEG"

# (3)(4) pg container
if [ $SKIP_DOCKER = 0 ]; then
  C="docker compose -f deploy/compose.yaml"
  cid=$($C ps -q pg) || die "docker compose ps failed - is the Docker daemon running?"
  if [ -n "$cid" ] && [ "$(docker inspect -f '{{.State.Running}}' "$cid" 2>/dev/null)" = true ]; then
    echo "pg container already running, reusing it"
  else
    if [ -z "${QUICKSTART_SKIP_PORT_CHECK:-}" ] && command -v nc >/dev/null 2>&1 && nc -z 127.0.0.1 55432 >/dev/null 2>&1; then
      die "port 55432 is in use by something other than this project's pg container - free it and run again"
    fi
    $C up -d pg || die "docker compose up failed"
  fi
  cid=$($C ps -q pg)
  [ -n "$cid" ] || die "pg container not found after up"
  limit=${QUICKSTART_HEALTH_TIMEOUT:-120}
  i=0
  until [ "$(docker inspect -f '{{.State.Health.Status}}' "$cid" 2>/dev/null)" = healthy ]; do
    i=$((i + 2))
    if [ $i -gt "$limit" ]; then $C logs --tail 20 pg >&2 || true; die "pg not healthy after ${limit}s"; fi
    sleep 2
  done
fi

# (5)(6) migrate + demo
SL_PG_DSN=$DSN; export SL_PG_DSN
"$DIEG" db migrate
"$DIEG" demo

# (7) next steps
cat <<NEXT

Next steps
  1. Log in to claude:   claude          (/exit after the subscription login)
  2. Write a novel:      $DIEG write     (the output is in Korean)
  3. Set the DSN in a new shell:
       export SL_PG_DSN='$DSN'
NEXT
