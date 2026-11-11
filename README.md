# Diegesis

A programming language for novels. You write characters, facts, scenes and speech acts (`tells`, `lies`, `sees` ...) in `.dieg.md` files. The Go compiler `dieg` loads them into PostgreSQL (Apache AGE + pgvector), where SQL checks time, who knows what, and deception. An AI then writes, renders and revises the prose against that checked state.

**Status: experimental preview.** The grammar and IR may change in any 0.x release. Prose output is Korean by default (see [Output language](#output-language)).

- **Change the middle of a serial, and only what should change does.** In a measured retcon test, `dieg revise` touched 47 characters of unrelated text where a direct AI edit touched 2,247, with fewer contradictions left (3 vs 6). Not shown to be cheaper. [Details](#why-diegesis).
- **Runs on local models too.** The coding step can use an Ollama model (`--code-model ollama:gemma4:26b`); the compiler and SQL checks catch its mistakes and feed them back. `dieg convert` turns public-domain stories into Diegesis code the same way. Experimental; one end-to-end episode (local draft, haiku finish and render): [`examples/local-hybrid`](examples/local-hybrid/README.md).

```sl
enter lady_macbeth
lady_macbeth reads macbeth_letter src 1.5.1
lady_macbeth thinks: "Yet do I fear thy nature; / It is too full o' the milk of human kindness" src 1.5.16
enter messenger
messenger tells lady_macbeth: duncan_comes "The king comes here to-night." src 1.5.32
```

한국어 문서: [README.ko.md](README.ko.md)

## Why Diegesis

The claim is narrow: when you change something in the middle of a story, Diegesis propagates the change more completely than asking an AI to edit the text directly. It is not shown to be cheaper.

Measured in [`examples/value/retcon3`](examples/value/retcon3/README.md) (v0.7.0, 6 retcons of a 12-episode story, one run per arm, one verifier; B = `dieg revise`, A = direct AI edit):

| | A: direct AI edit | B: `dieg revise` |
|---|---|---|
| Confirmed residual + new contradictions | 6 (8 with boundary items) | **3** (5) |
| Unrelated text changed | 2,247 chars | **47** chars |
| Priced weighted cost | 1 | 0.93x (1.12x if R4 is treated like R6) |
| Model calls | 33 | 221 |

- The pre-registered rule is met, but a cost advantage is **not** established: 0.93x depends on A editing nothing in R6. 12 episodes favour A, and A's cost varies a lot (A's R2 cost was 3.7x the previous run).
- 1,069 of A's 2,247 unrelated characters come from a keyword-dictionary gap in R3 e04; excluding it gives about 1,178 vs 47, so the conclusion holds.
- Limitations: one run per arm, one verifier, 12 episodes. R6 A may be under-counted (the judge possibly missed e08:41); this favours A.
- Cost alone is not a reason to use Diegesis: [`examples/value/cost-curve`](examples/value/cost-curve) found direct writing's per-episode cost flat.

A concrete side-by-side case: [`examples/showcase`](examples/showcase/README.md).

## Quickstart

Needs only Docker (Go optional: the script builds `dieg` from source when Go is installed, otherwise downloads a release binary and checks it; PostgreSQL is always built locally from the official `apache/age` image).

```sh
git clone https://github.com/starthalf51/diegesis-lang && cd diegesis-lang && ./scripts/quickstart.sh
```

Windows (PowerShell): `powershell -ExecutionPolicy Bypass -File scripts\quickstart.ps1`

The script starts PostgreSQL in Docker, gets `dieg` (PATH, then `go build`, then a Release download with a sha256 check), migrates, and runs `dieg demo`. No model calls. Details: [scripts/README.md](scripts/README.md).

Try it without a Claude subscription: `dieg demo`. It loads a small embedded Korean story (a lighthouse) and prints what the SQL layer finds, with no model calls:

```text
== Consistency warnings ==
1.3.dieg.md:13:19: warn stays-gone: dobi enters after leaving (status gone at 1.3.dieg.md:12)
1.3.dieg.md:14:1: warn stays-gone: dobi says after leaving (status gone at 1.3.dieg.md:12)
cast.dieg.md:35:1: warn relative-time-fact: fact storm_coming names a relative time: 폭풍이 열흘 뒤에 온다

== Who knows what (scene 1.3) ==
- 도비: 「등대 문은 잠겨 있다」 — believes it is true
- 미나: 「폭풍이 열흘 뒤에 온다」 — believes it is true
- 하루: 「폭풍이 열흘 뒤에 온다」 — believes it is true
- 하루: 「등대 문은 잠겨 있다」 — believes it is true

== World state for scene 1.3 == (excerpt)
### 장소와 시간
- 이 장면의 장소: 항구(harbor)

### 코드에 적힌 속성
- 도비: 이미 이야기에서 떠났다(status)
- 놋쇠 열쇠(key): at = 등대
- 이 장면의 장소(항구)에 없는 물건: 놋쇠 열쇠(등대). 장면에서 직접 옮기는 행위가 없으면 그 자리에 둔다.
```

The world-state block is the Korean prompt text the renderer receives.

## Manual setup

### Prerequisites

- Go 1.27.3
- Docker with compose v2
- For `dieg write` / `continue` / `revise` / `render`: the [`claude`](https://claude.com/claude-code) CLI, logged in to a subscription (no API key is used)

The commands are the same on Linux, macOS and Windows (set the environment variable the way your shell does).

### 1. Start PostgreSQL

```sh
git clone https://github.com/starthalf51/diegesis-lang && cd diegesis-lang
docker compose -f deploy/compose.yaml up -d pg    # 127.0.0.1:55432, user/password/db = sl/sl/sl; no .env needed
export SL_PG_DSN='postgres://sl:sl@localhost:55432/sl?sslmode=disable'
```

Only the optional Grafana stack (`ro-init`, `grafana`) needs passwords: copy `deploy/.env.example` to `deploy/.env` and change the values first.

### 2. Get `dieg`

```sh
go install github.com/starthalf51/diegesis-lang/cmd/dieg@latest
# or, from the clone:
go build -o dieg ./cmd/dieg
```

### 3. Migrate and test (no model calls)

```sh
dieg db migrate            # applies db/migrations/*.sql once each
dieg test corpus/macbeth   # compile, load, lint, query, patterns, mutations; report in out/accept/macbeth.md
```

`dieg test` needs the repository checkout (it reads `corpus/`, `rules/` and `patterns/`). A passing run takes a couple of minutes and ends with a `PASS macbeth: ...` line and exit code 0.

### 4. Write a story (model calls)

```sh
claude                     # log in once with your subscription
dieg write "등대지기가 편지 한 통을 숨긴다" -o out/story1
dieg continue out/story1 --ep 2
dieg revise out/story1 "편지를 지도로 바꿔줘"
```

`write` asks the model for Diegesis code, feeds `check`/`lint` diagnostics back until the code is clean, then renders and verifies each scene. Output: `out/story1/story/` (code), `out/story1/novel.md` (prose), `out/story1/calls.jsonl` (token log). `continue` adds an episode from the PG state; `revise` edits the code, then patches only the affected sentences of affected scenes. Use `--no-render` to stop at code, and `DIEG_CALL_CAP=N` to hard-limit model calls (local Ollama calls count too; leave room for `--finish-model` rounds).

The same steps, with more detail: [docs/quickstart.md](docs/quickstart.md).

## Output language

`dieg write` / `continue` / `revise` produce Korean prose by default (backend `ko-haera`; the coding prompt requires Korean facts and dialogue), whatever language the idea is in. An English rendering backend (`en-doyle`) exists for the corpus, but the generation path is Korean-first in this preview.

## Commands

| Command | What it does | Model calls |
|---|---|---|
| `dieg check <dir>` | Parse and check a work (exit 1 on errors) | no |
| `dieg test <corpus-dir>` | Compile, load, lint, patterns, mutation test against `expect.yaml` | no |
| `dieg lint <work>` | Run the PG lint rules (`rules/*.sql`) | no |
| `dieg patterns <work>` | Run the narrative pattern library (`patterns/*.sql`) | no |
| `dieg stats <work>` | Metrics: presence, information asymmetry, tension, anachrony ... | no |
| `dieg demo` | Load the embedded demo story; print warnings, who knows what, world state (needs PG) | no |
| `dieg write` / `continue` / `revise` | Idea → code → checked → rendered prose; next episode; edit | yes |
| `dieg convert <text.txt> -o <dir>` | Public-domain story → Diegesis code through the same check loop (no render; `--force` to overwrite) | yes |
| `dieg render <work> <scene> --backend <id> [--verify]` | Render one scene | yes |
| `dieg stamp <work-dir>` | Add state hashes to stories written before v0.7.0 | no |

Model names of the form `ollama:<name>` (e.g. `--code-model ollama:gemma4:26b`) run on a local Ollama server. `--finish-model M` hands code that is still unclean after the main rounds to another model, and `--allow-warn` lets a round pass with warnings only (`write` and `convert`). See [Local models](docs/quickstart.md#5-local-models-ollama).

`dieg` with no arguments prints the full usage.

## How it works

1. **Code.** A story is `.dieg.md` files: front matter, `sl` code blocks (entities, facts, scenes, acts), and optional prose. Grammar: [`spec/grammar.ebnf`](spec/grammar.ebnf).
2. **Check.** The compiler builds an IR and loads it into PostgreSQL. SQL derives time order (a simple temporal network), beliefs and knowledge, and reader state, and runs 19 lint rules (for example, a character acting after leaving, or a fact told to someone who already knows it).
3. **Render.** Each scene is rendered by a small model with a world-state digest from PG (where things are, who knows what, elapsed time), then scored by a checklist verifier (facts present, fact kind, no leaks of withheld facts).
4. **Revise.** A change request edits the code; scenes whose code or world state changed are found mechanically and patched sentence by sentence.

## Documentation

Reference docs: [docs/README.md](docs/README.md) (language, file format, compiler, checks, storage, rendering). Design notes and ADRs referenced in the docs are not published.

## Tests

```sh
go test -count=1 ./...   # needs SL_PG_DSN (PostgreSQL running)
go vet ./...
```

## Security

Experimental preview, no warranty. The default install builds from source; PostgreSQL is built locally from the official `apache/age` image (no Diegesis image is published). Release binaries come with `checksums.txt` and GitHub build provenance (`gh attestation verify <file> --repo starthalf51/diegesis-lang --signer-workflow starthalf51/diegesis-lang/.github/workflows/release.yml`). When the quickstart scripts download a binary they always check the checksum, and check provenance when a logged-in `gh` is available. Report vulnerabilities privately: see [SECURITY.md](SECURITY.md).

## Versioning

[SemVer](https://semver.org/). Before 1.0, grammar and IR may change in MINOR releases. Releases are git tags (`v0.9.0`); `dieg version` prints the version. Release history: [README.ko.md](README.ko.md#버전) (Korean).

## License

Apache License 2.0 ([LICENSE](https://github.com/starthalf51/diegesis-lang/blob/main/LICENSE)). Sources of the corpus and style texts (all public domain) are listed in [NOTICE](https://github.com/starthalf51/diegesis-lang/blob/main/NOTICE).
