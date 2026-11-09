# Releasing (maintainers)

Status: **not yet exercised.** Neither workflow below has run. Expect to fix small things on the first real run.

## 1. PG image

`.github/workflows/pg-image.yml` builds `deploy/postgres` (AGE + pgvector) for amd64 and arm64 and pushes `ghcr.io/starthalf51/diegesis-pg:age1.8.0-pgvector0.8.7`.

1. Merge/push to `main` (or run the workflow manually).
2. The first push creates the ghcr package as **private**. Make it public once: GitHub profile, Packages, `diegesis-pg`, Package settings, Change visibility, Public. Until then `docker compose pull pg` fails for everyone else (compose then falls back to building locally).
3. Check from a logged-out shell: `docker pull ghcr.io/starthalf51/diegesis-pg:age1.8.0-pgvector0.8.7`.

## 2. Binaries

`.github/workflows/release.yml` runs on `v*` tags and uploads `dieg_<ver>_<os>_<arch>[.exe]` plus `checksums.txt` to the Release.

1. Make sure `version` in `cmd/dieg/main.go` matches the tag (the workflow also sets it by ldflags).
2. `git tag vX.Y.Z && git push origin vX.Y.Z`.
3. Check the Release has every asset and `checksums.txt`.

## 3. Verify

- `go install github.com/starthalf51/diegesis-lang/cmd/dieg@latest && dieg version` prints the new version.
- On a machine without Go: `./scripts/quickstart.sh` (and `quickstart.ps1`) downloads the asset, passes the sha256 check, and `dieg demo` runs. `--print-release-url` shows the URL the script will use.
