# Kompletter Build: Frontend (Vite) → backend/web/dist → Windows-exe (Docker/MinGW) → dist\soundboard.exe
# Voraussetzungen: Node.js/npm und Docker. Kein Go und kein C-Compiler auf dem Host nötig.
$ErrorActionPreference = "Stop"
$root = Split-Path -Parent $PSScriptRoot

Push-Location "$root\frontend"
try {
  if (-not (Test-Path node_modules)) { npm install --no-audit --no-fund; if ($LASTEXITCODE) { throw "npm install fehlgeschlagen" } }
  npm run build
  if ($LASTEXITCODE) { throw "Frontend-Build fehlgeschlagen" }
} finally { Pop-Location }

& "$PSScriptRoot\build-backend.ps1" -Target soundboard
