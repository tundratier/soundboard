# Full build: frontend (Vite) → backend/web/dist → Windows exe (Docker/MinGW) → dist\soundboard.exe
# Prerequisites: Node.js/npm and Docker. No Go and no C compiler needed on the host.
$ErrorActionPreference = "Stop"
$root = Split-Path -Parent $PSScriptRoot

Push-Location "$root\frontend"
try {
  if (-not (Test-Path node_modules)) { npm install --no-audit --no-fund; if ($LASTEXITCODE) { throw "npm install failed" } }
  npm run build
  if ($LASTEXITCODE) { throw "Frontend build failed" }
} finally { Pop-Location }

& "$PSScriptRoot\build-backend.ps1" -Target soundboard
