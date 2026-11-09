# Diegesis quickstart: docker pg -> dieg -> migrate -> demo. No model calls. (Windows PowerShell 5.1+)
# Flags: -SkipDocker (PG already reachable at the DSN), -PrintReleaseUrl [-Tag vX.Y.Z] (no download)
param([switch]$SkipDocker, [switch]$PrintReleaseUrl, [string]$Tag = '')
$ErrorActionPreference = 'Stop'

$Repo = 'starthalf51/diegesis-lang'
$Dsn = 'postgres://sl:sl@localhost:55432/sl?sslmode=disable'
function Die($m) { [Console]::Error.WriteLine("quickstart: $m"); exit 1 }

function Get-ReleaseAsset {
  $arch = if ($env:PROCESSOR_ARCHITECTURE -match 'ARM64') { 'arm64' } else { 'amd64' }
  $t = $script:Tag
  if (-not $t) {
    try {
      [Net.ServicePointManager]::SecurityProtocol = [Net.SecurityProtocol]::Tls12
      $r = Invoke-WebRequest -UseBasicParsing "https://github.com/$Repo/releases/latest" -MaximumRedirection 5
      $t = ($r.BaseResponse.ResponseUri.AbsoluteUri -replace '.*/tag/', '')
    } catch { $t = '' }
    if ($t -notmatch '^v') { Die "cannot find the latest release of $Repo" }
  }
  $asset = "dieg_$($t.TrimStart('v'))_windows_${arch}.exe"
  @{ Asset = $asset; Base = "https://github.com/$Repo/releases/download/$t"; Url = "https://github.com/$Repo/releases/download/$t/$asset" }
}

if ($PrintReleaseUrl) { $a = Get-ReleaseAsset; $a.Asset; $a.Url; exit 0 }

Set-Location (Join-Path $PSScriptRoot '..')

# (1) docker + compose v2
if (-not $SkipDocker) {
  if (-not (Get-Command docker -ErrorAction SilentlyContinue)) { Die 'docker not found - install Docker Desktop first' }
  & docker compose version *> $null
  if ($LASTEXITCODE -ne 0) { Die "docker compose v2 not found - update Docker (the 'docker compose' plugin is required)" }
}

# (2) dieg: PATH -> go build -> release download
$bin = Join-Path (Get-Location) 'bin'
if (Get-Command dieg -ErrorAction SilentlyContinue) { $Dieg = 'dieg' }
elseif (Get-Command go -ErrorAction SilentlyContinue) {
  New-Item -ItemType Directory -Force $bin | Out-Null
  $Dieg = Join-Path $bin 'dieg.exe'
  & go build -o $Dieg ./cmd/dieg
  if ($LASTEXITCODE -ne 0) { Die 'go build failed' }
} else {
  try {
    $a = Get-ReleaseAsset
    New-Item -ItemType Directory -Force $bin | Out-Null
    $Dieg = Join-Path $bin 'dieg.exe'
    Invoke-WebRequest -UseBasicParsing $a.Url -OutFile $Dieg
    $sums = (Invoke-WebRequest -UseBasicParsing "$($a.Base)/checksums.txt").Content -split "`n"
    $want = ($sums | Where-Object { $_ -match [regex]::Escape($a.Asset) + '\s*$' } | Select-Object -First 1) -split '\s+' | Select-Object -First 1
    if (-not $want -or $want -ne (Get-FileHash $Dieg -Algorithm SHA256).Hash.ToLower()) { Remove-Item $Dieg -Force; throw 'checksum mismatch' }
  } catch { Die "no dieg and no Go, and the Release download failed ($_): install Go 1.27.3 or run again after the Release is public" }
}
Write-Host "dieg: $Dieg"

# (3)(4) pg container
if (-not $SkipDocker) {
  $c = @('compose', '-f', 'deploy/compose.yaml')
  $cid = (& docker @c ps -q pg 2>$null)
  $running = $cid -and ((& docker inspect -f '{{.State.Running}}' $cid 2>$null) -eq 'true')
  if ($running) { Write-Host 'pg container already running, reusing it' }
  else {
    $inUse = $null -ne (Get-NetTCPConnection -LocalPort 55432 -State Listen -ErrorAction SilentlyContinue)
    if ($inUse) { Die "port 55432 is in use by something other than this project's pg container - free it and run again" }
    & docker @c pull pg *> $null
    if ($LASTEXITCODE -ne 0) { Write-Host 'prebuilt image not available, building locally (first run takes a few minutes)' }
    & docker @c up -d pg
    if ($LASTEXITCODE -ne 0) { Die 'docker compose up failed' }
  }
  $cid = (& docker @c ps -q pg)
  for ($i = 0; (& docker inspect -f '{{.State.Health.Status}}' $cid 2>$null) -ne 'healthy'; $i += 2) {
    if ($i -ge 120) { & docker @c logs --tail 20 pg; Die 'pg not healthy after 120s' }
    Start-Sleep 2
  }
}

# (5)(6) migrate + demo
$env:SL_PG_DSN = $Dsn
& $Dieg db migrate
if ($LASTEXITCODE -ne 0) { Die 'dieg db migrate failed' }
& $Dieg demo
if ($LASTEXITCODE -ne 0) { Die 'dieg demo failed' }

# (7) next steps
@"

다음 단계
  1. claude 로그인:   claude          (구독 로그인 후 /exit)
  2. 소설 쓰기:       $Dieg write     (결과물은 한국어)
  3. 새 셸에서 DSN 설정:
       `$env:SL_PG_DSN = '$Dsn'
"@
