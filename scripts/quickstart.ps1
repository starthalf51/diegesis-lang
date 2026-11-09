# Diegesis quickstart: docker pg -> dieg -> migrate -> demo. No model calls. (Windows PowerShell 5.1+)
# Flags: -SkipDocker (PG already reachable at the DSN), -PrintReleaseUrl [-Tag vX.Y.Z] (no download)
param([switch]$SkipDocker, [switch]$PrintReleaseUrl, [string]$Tag = '')
$ErrorActionPreference = 'Stop'

$Repo = 'starthalf51/diegesis-lang'
$Dsn = 'postgres://sl:sl@localhost:55432/sl?sslmode=disable'
function Die($m) { [Console]::Error.WriteLine("quickstart: $m"); exit 1 }
# native docker calls: stderr must not throw under $ErrorActionPreference=Stop
function Dk { $q = $args[0] -eq "-Quiet"; $a = if ($q) { $args[1..($args.Count - 1)] } else { $args }
  $e = $ErrorActionPreference; $ErrorActionPreference = "Continue"
  try { if ($q) { & docker @a 2>&1 | Out-Null } else { & docker @a 2>&1 | Out-Host }; $script:DkCode = $LASTEXITCODE } finally { $ErrorActionPreference = $e }
  return $script:DkCode }
function DkOut { $e = $ErrorActionPreference; $ErrorActionPreference = "Continue"
  try { $o = & docker @args 2>&1 | Where-Object { $_ -isnot [Management.Automation.ErrorRecord] }; $script:DkCode = $LASTEXITCODE } finally { $ErrorActionPreference = $e }
  return ($o | Out-String).Trim() }

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
  if ((Dk -Quiet compose version) -ne 0) { Die "docker compose v2 not found - update Docker (the 'docker compose' plugin is required)" }
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
  $cid = DkOut @c ps -q pg
  if (-not $cid -and $script:DkCode -ne 0) { Die 'docker compose ps failed - is the Docker daemon running?' }
  $running = $cid -and ((DkOut inspect -f '{{.State.Running}}' $cid) -eq 'true')
  if ($running) { Write-Host 'pg container already running, reusing it' }
  else {
    $inUse = (-not $env:QUICKSTART_SKIP_PORT_CHECK) -and $null -ne (Get-NetTCPConnection -LocalPort 55432 -State Listen -ErrorAction SilentlyContinue)
    if ($inUse) { Die "port 55432 is in use by something other than this project's pg container - free it and run again" }
    if ((Dk -Quiet @c pull pg) -ne 0) { Write-Host 'prebuilt image not available, building locally (first run takes a few minutes)' }
    if ((Dk @c up -d pg) -ne 0) { Die 'docker compose up failed' }
  }
  $cid = DkOut @c ps -q pg
  if (-not $cid) { Die 'pg container not found after up' }
  $limit = if ($env:QUICKSTART_HEALTH_TIMEOUT) { [int]$env:QUICKSTART_HEALTH_TIMEOUT } else { 120 }
  for ($i = 0; (DkOut inspect -f '{{.State.Health.Status}}' $cid) -ne 'healthy'; $i += 2) {
    if ($i -ge $limit) { Dk @c logs --tail 20 pg | Out-Null; Die "pg not healthy after ${limit}s" }
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

Next steps
  1. Log in to claude:   claude          (/exit after the subscription login)
  2. Write a novel:      $Dieg write     (the output is in Korean)
  3. Set the DSN in a new shell:
       `$env:SL_PG_DSN = '$Dsn'
"@
