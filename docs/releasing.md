# Releasing (maintainers)

## 1. PG image

No PG image is published. `deploy/postgres/Dockerfile` builds AGE + pgvector locally (`diegesis-pg:local`) on top of the official `apache/age` image, pinned by digest. To move the base: `docker buildx imagetools inspect apache/age:<tag>`, copy the index `Digest`, update the `FROM` line, and rebuild (`docker compose -f deploy/compose.yaml build pg`).

## 2. Binaries

`.github/workflows/release.yml` runs on `v<digit>*` tags. It builds `dieg_<ver>_<os>_<arch>[.exe]`, writes `checksums.txt`, records GitHub build provenance for each binary, and uploads them to the Release. The workflow is read-only except the release job; every action is pinned to a commit SHA (Dependabot proposes updates).

1. Make sure `version` in `cmd/dieg/main.go` matches the tag (the workflow also sets it by ldflags).
2. `git tag vX.Y.Z && git push origin vX.Y.Z`.
3. Check the Release has every asset and `checksums.txt`.

## 3. Verify

- `go install github.com/starthalf51/diegesis-lang/cmd/dieg@latest && dieg version` prints the new version.
- `gh attestation verify dieg_<ver>_linux_amd64 --repo starthalf51/diegesis-lang` passes for a downloaded asset.
- On a machine without Go: `./scripts/quickstart.sh` (and `quickstart.ps1`) downloads the asset, passes the checksum and provenance checks, and `dieg demo` runs. `--print-release-url` shows the URL the script will use.

## 4. Repository settings (once, by hand)

- Account: two-factor authentication, preferably a passkey.
- Settings, Rules: protect `main` (no force push, no deletion) and tags matching `v*` (only maintainers create, no deletion or update).
- Settings, Security: enable **Private vulnerability reporting** (see [SECURITY.md](../SECURITY.md)) and Dependabot alerts.
- Clean-up after v0.9.0: delete the `diegesis-pg` package (profile, Packages, Package settings, Delete) and the v0.9.0 Release assets (built before provenance), then tag `v0.9.1`.
