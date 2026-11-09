# scripts

`quickstart.sh` (Linux/macOS/Git Bash) and `quickstart.ps1` (Windows PowerShell 5.1+): start the pg container, get `dieg`, migrate, run `dieg demo`. No model calls.

    sh scripts/quickstart.sh
    powershell -ExecutionPolicy Bypass -File scripts\quickstart.ps1

Flags: `--skip-docker` / `-SkipDocker` (PG already reachable at the DSN), `--print-release-url [tag]` / `-PrintReleaseUrl [-Tag tag]` (show the Release asset URL, no download).

Env hooks: `QUICKSTART_HEALTH_TIMEOUT` (seconds to wait for pg to become healthy, default 120), `QUICKSTART_SKIP_PORT_CHECK=1` (skip the port 55432 in-use check).
