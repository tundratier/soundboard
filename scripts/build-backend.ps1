# Builds the Windows exe(s) via Docker (cgo/MinGW) - no C compiler needed on the host.
# Usage: scripts\build-backend.ps1 [-Target spike|soundboard|all]
param([string]$Target = "all")
$ErrorActionPreference = "Stop"
$root = Split-Path -Parent $PSScriptRoot
$env:MSYS_NO_PATHCONV = "1"

docker build -q -t soundboard-build -f "$root\build\Dockerfile" "$root\build" | Out-Null
if ($LASTEXITCODE -ne 0) { throw "Could not build the Docker image" }

docker run --rm `
  -v "${root}:/src" `
  -v soundboard-gomod:/go/pkg/mod -v soundboard-gocache:/root/.cache/go-build `
  soundboard-build bash /src/build/build.sh $Target
if ($LASTEXITCODE -ne 0) { throw "Build failed" }
Get-ChildItem "$root\dist\*.exe" | Select-Object Name, @{n = "MB"; e = { [math]::Round($_.Length / 1MB, 1) } }
