# Quickstart

**Status: experimental preview** (0.x; commands and grammar may change).

Diegesis is Go plus PostgreSQL. Everything below uses the same commands on Linux, macOS and Windows (any POSIX-style shell or PowerShell; set the environment variable in the way your shell does).

## Fast path

Needs only Docker (Go optional: the script builds `dieg` from source when Go is installed, otherwise downloads a release binary and checks it; PostgreSQL is always built locally from the official `apache/age` image).

```sh
git clone https://github.com/starthalf51/diegesis-lang && cd diegesis-lang && ./scripts/quickstart.sh
```

Windows (PowerShell): `powershell -ExecutionPolicy Bypass -File scripts\quickstart.ps1`

It starts PostgreSQL, gets `dieg`, migrates and runs `dieg demo` (no model calls, no subscription). See [scripts/README.md](../scripts/README.md). The steps below are the same flow by hand.

## Prerequisites

- Go 1.27.3
- Docker with compose v2
- For `dieg write` / `continue` / `revise` / `render`: the `claude` CLI, logged in to a subscription (no API key is used)

## 1. Start PostgreSQL

```sh
git clone https://github.com/starthalf51/diegesis-lang && cd diegesis-lang
docker compose -f deploy/compose.yaml up -d pg    # 127.0.0.1:55432, user/password/db = sl/sl/sl; no .env needed
export SL_PG_DSN='postgres://sl:sl@localhost:55432/sl?sslmode=disable'
```

Only the optional Grafana stack (`ro-init`, `grafana`) needs passwords: copy `deploy/.env.example` to `deploy/.env` and change the values first. Those services stop with a message if they are empty.

## 2. Get `dieg`

Either install the binary:

```sh
go install github.com/starthalf51/diegesis-lang/cmd/dieg@latest
```

or build from the clone (the commands below then assume `dieg` is on your PATH; otherwise use `go run ./cmd/dieg` in place of `dieg`):

```sh
go build -o dieg ./cmd/dieg
```

## 3. Migrate and test (no model calls)

```sh
dieg db migrate            # applies db/migrations/*.sql once each
dieg test corpus/macbeth   # compile, load, lint, query, patterns, mutations; report in out/accept/macbeth.md
```

`dieg test` needs the repository checkout (it reads `corpus/`, `rules/` and `patterns/`). A passing run takes a couple of minutes and ends with a `PASS macbeth: ...` line and exit code 0.

Try `dieg demo` here too: it loads an embedded story and prints warnings, who knows what, and world state (no model calls).

## 4. Write a story (model calls)

```sh
claude                     # log in once with your subscription
dieg write "등대지기가 편지 한 통을 숨긴다" -o out/story1
```

The prose is Korean by default (backend `ko-haera`; the coding prompt requires Korean), so write the idea in Korean or expect Korean output either way. This asks the model for Diegesis code, feeds `check`/`lint` diagnostics back, then renders and verifies each scene. Output: `out/story1/story/` (code), `novel.md` (prose), `calls.jsonl` (token log). Use `--no-render` to stop at code, and `DIEG_CALL_CAP=N` to hard-limit model calls.
Then `dieg continue out/story1 --ep 2` adds an episode and `dieg revise out/story1 "change request"` edits the story.

More: [docs index](README.md). Design notes and ADRs referenced there are not published.
