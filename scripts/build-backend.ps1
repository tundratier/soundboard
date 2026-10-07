# Baut die Windows-exe(s) per Docker (cgo/MinGW) – kein C-Compiler auf dem Host nötig.
# Aufruf: scripts\build-backend.ps1 [-Target spike|soundboard|all]
param([string]$Target = "all")
$ErrorActionPreference = "Stop"
$root = Split-Path -Parent $PSScriptRoot
$env:MSYS_NO_PATHCONV = "1"

docker build -q -t soundboard-build -f "$root\build\Dockerfile" "$root\build" | Out-Null
if ($LASTEXITCODE -ne 0) { throw "Docker-Image konnte nicht gebaut werden" }

docker run --rm `
  -v "${root}:/src" `
  -v soundboard-gomod:/go/pkg/mod -v soundboard-gocache:/root/.cache/go-build `
  soundboard-build bash /src/build/build.sh $Target
if ($LASTEXITCODE -ne 0) { throw "Build fehlgeschlagen" }
Get-ChildItem "$root\dist\*.exe" | Select-Object Name, @{n = "MB"; e = { [math]::Round($_.Length / 1MB, 1) } }
