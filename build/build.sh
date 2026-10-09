#!/bin/bash
# Cross-compiles the Windows exe(s). Runs inside the Docker container (see scripts/build-backend.ps1) and in the
# Bitbucket pipeline (see build/ci-release.sh). Target: spike | soundboard | all
# Environment:
#   ROOT             repository root (default /src, where the Docker build mounts it)
#   VERSION          semver embedded in the exe (default "dev")
#   VERSION_NUMERIC  X.Y.Z.N for the Windows file/product version resource (default 0.0.0.0)
set -euo pipefail
ROOT="${ROOT:-/src}"
cd "$ROOT/backend"
target="${1:-all}"
VERSION="${VERSION:-dev}"
VERSION_NUMERIC="${VERSION_NUMERIC:-0.0.0.0}"

# cgo cross-compile to Windows with MinGW-w64 (the Docker image sets the same values)
export GOOS=windows GOARCH=amd64 CGO_ENABLED=1 CC="${CC:-x86_64-w64-mingw32-gcc}"

mkdir -p "$ROOT/dist"
if [[ "$target" == spike || "$target" == all ]]; then
  go build -trimpath -ldflags '-s -w' -o "$ROOT/dist/spike.exe" ./cmd/spike
fi
if [[ "$target" == soundboard || "$target" == all ]]; then
  # Icon + manifest + version info as .syso (linked in automatically); go-winres itself runs natively
  (cd cmd/soundboard && env GOOS=linux CGO_ENABLED=0 go run github.com/tc-hib/go-winres@latest make \
    --in winres/winres.json --arch amd64 --file-version "$VERSION_NUMERIC" --product-version "$VERSION_NUMERIC")
  # GUI variant (no console) and console variant (logs visible)
  go build -trimpath -ldflags "-s -w -H windowsgui -X main.version=$VERSION" -o "$ROOT/dist/soundboard.exe" ./cmd/soundboard
  go build -trimpath -ldflags "-s -w -X main.version=$VERSION" -o "$ROOT/dist/soundboard-console.exe" ./cmd/soundboard
fi
echo "built version $VERSION"
