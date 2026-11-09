# Security

Diegesis is an experimental preview (0.x), provided "as is" without warranty under the [Apache License 2.0](LICENSE).

## Supported versions

Only the latest 0.x release gets fixes.

## Reporting a vulnerability

Use GitHub private vulnerability reporting: the repository's **Security** tab, **Report a vulnerability**. Please do not open a public issue for a vulnerability.

## What is distributed

- Source code. Building from source (`go install`, or `./scripts/quickstart.sh` with Go installed) is the default install path.
- `dieg` binaries on GitHub Releases, built by `.github/workflows/release.yml` (actions pinned to commit SHAs) with a `checksums.txt` and GitHub build provenance.
- No container image. `deploy/postgres/Dockerfile` builds PostgreSQL locally on top of the official `apache/age` image, pinned by digest.

## Verifying a release binary

```sh
sha256sum -c checksums.txt --ignore-missing
gh attestation verify dieg_<ver>_<os>_<arch> --repo starthalf51/diegesis-lang
```

`scripts/quickstart.sh` and `quickstart.ps1` run both checks when they download a binary (the provenance check needs a logged-in `gh`) and delete the file if either fails.
