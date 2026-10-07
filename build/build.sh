#!/bin/bash
# Läuft im Docker-Container (siehe scripts/build-backend.ps1). Ziel: spike | soundboard | all
set -euo pipefail
cd /src/backend
target="${1:-all}"
mkdir -p /src/dist
if [[ "$target" == spike || "$target" == all ]]; then
  go build -trimpath -ldflags '-s -w' -o /src/dist/spike.exe ./cmd/spike
fi
if [[ "$target" == soundboard || "$target" == all ]]; then
  # Icon + Manifest + Versionsinfo als .syso (wird automatisch eingelinkt)
  (cd cmd/soundboard && env GOOS=linux CGO_ENABLED=0 go run github.com/tc-hib/go-winres@latest make --in winres/winres.json --arch amd64)
  # GUI-Variante (ohne Konsole) und Konsolen-Variante (Logs sichtbar)
  go build -trimpath -ldflags '-s -w -H windowsgui' -o /src/dist/soundboard.exe ./cmd/soundboard
  go build -trimpath -ldflags '-s -w' -o /src/dist/soundboard-console.exe ./cmd/soundboard
fi
