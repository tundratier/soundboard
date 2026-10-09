#!/bin/bash
# Runs inside the Docker container (see scripts/build-backend.ps1). Target: spike | soundboard | all
# Environment: VERSION (semver, from scripts/version.ps1), VERSION_NUMERIC (X.Y.Z.N for the Windows version resource)
set -euo pipefail
cd /src/backend
target="${1:-all}"
VERSION="${VERSION:-dev}"
VERSION_NUMERIC="${VERSION_NUMERIC:-0.0.0.0}"
mkdir -p /src/dist
if [[ "$target" == spike || "$target" == all ]]; then
  go build -trimpath -ldflags '-s -w' -o /src/dist/spike.exe ./cmd/spike
fi
if [[ "$target" == soundboard || "$target" == all ]]; then
  # Icon + manifest + version info as .syso (linked in automatically)
  (cd cmd/soundboard && env GOOS=linux CGO_ENABLED=0 go run github.com/tc-hib/go-winres@latest make \
    --in winres/winres.json --arch amd64 --file-version "$VERSION_NUMERIC" --product-version "$VERSION_NUMERIC")
  # GUI variant (no console) and console variant (logs visible)
  go build -trimpath -ldflags "-s -w -H windowsgui -X main.version=$VERSION" -o /src/dist/soundboard.exe ./cmd/soundboard
  go build -trimpath -ldflags "-s -w -X main.version=$VERSION" -o /src/dist/soundboard-console.exe ./cmd/soundboard
fi
echo "built version $VERSION"
