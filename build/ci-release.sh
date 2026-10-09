#!/bin/bash
# CI (GitHub Actions): builds the Windows exe for the release tag at HEAD and publishes it as a GitHub release.
# The tag vX.Y.Z marks the release; the release gets the zip (exes, README, CHANGELOG), its SHA-256 checksum and the
# changelog section of the version as release notes.
#
# Run from the repository with the frontend already built (backend/web/dist), gcc-mingw-w64 and zip installed and
# the GitHub CLI (gh) available. Environment:
#   GH_TOKEN      token for gh; in Actions the workflow passes github.token (the job needs "contents: write")
#   GH_REPO       owner/repo (the workflow passes github.repository)
#   RELEASE_TAG   optional: use this tag instead of the one at HEAD (for testing)
#   DRY_RUN=1     build and package, but do not publish
#   SKIP_BUILD=1  package the existing dist/*.exe instead of building (local testing)
set -euo pipefail

ROOT="${GITHUB_WORKSPACE:-$(cd "$(dirname "$0")/.." && pwd)}"
cd "$ROOT"

# --- which release is this? ---------------------------------------------------------------------------------
git fetch --tags --force origin >/dev/null 2>&1 || echo "warning: could not fetch tags"
TAG="${RELEASE_TAG:-$(git describe --exact-match --tags --match 'v[0-9]*.[0-9]*.[0-9]*' HEAD 2>/dev/null || true)}"
if [[ -z "$TAG" ]]; then
  echo "HEAD is not at a release tag (vX.Y.Z), so there is nothing to publish."
  echo "Releases are made with scripts/release.ps1 on develop and pushed with: git push origin master develop --follow-tags"
  exit 0
fi
if ! [[ "$TAG" =~ ^v[0-9]+\.[0-9]+\.[0-9]+$ ]]; then
  echo "error: '$TAG' is not a release tag (vX.Y.Z)" >&2
  exit 1
fi
VERSION="${TAG#v}"
echo "Releasing $TAG"

# --- build ----------------------------------------------------------------------------------------------------
[[ -f backend/web/dist/index.html ]] || { echo "error: the frontend build (backend/web/dist) is missing" >&2; exit 1; }
export ROOT VERSION VERSION_NUMERIC="$VERSION.0"
if [[ "${SKIP_BUILD:-0}" == "1" ]]; then
  echo "SKIP_BUILD=1: using the existing exes in dist/"
else
  bash build/build.sh soundboard
fi

# --- package ------------------------------------------------------------------------------------------------
PKG="soundboard-$TAG-windows-amd64"
OUT="$ROOT/dist"
NOTES="$OUT/$PKG-release-notes.md"
rm -rf "${OUT:?}/$PKG" "$OUT/$PKG.zip" "$OUT/$PKG.zip.sha256" "$NOTES"
mkdir -p "$OUT/$PKG"
cp "$OUT/soundboard.exe" "$OUT/soundboard-console.exe" README.md "$OUT/$PKG/"
[[ -f CHANGELOG.md ]] && cp CHANGELOG.md "$OUT/$PKG/"
(cd "$OUT" && zip -qr "$PKG.zip" "$PKG" && sha256sum "$PKG.zip" > "$PKG.zip.sha256")

# release notes: the matching section of CHANGELOG.md
if [[ -f CHANGELOG.md ]]; then
  awk -v v="$VERSION" 'index($0, "## [" v "]") == 1 { on = 1; next } /^## \[/ { on = 0 } on' CHANGELOG.md \
    | sed '/./,$!d' > "$NOTES"
fi
[[ -s "$NOTES" ]] || echo "Release $VERSION" > "$NOTES"

echo "Packaged:"
ls -l "$OUT/$PKG.zip" "$OUT/$PKG.zip.sha256" "$NOTES"
if [[ "${DRY_RUN:-0}" == "1" ]]; then
  echo "DRY_RUN=1: not publishing"
  exit 0
fi

# --- publish as a GitHub release ----------------------------------------------------------------------------------
command -v gh >/dev/null || { echo "error: the GitHub CLI (gh) is not installed" >&2; exit 1; }
[[ -n "${GH_TOKEN:-}" ]] || { echo "error: GH_TOKEN is not set (in Actions: env GH_TOKEN: \${{ github.token }})" >&2; exit 1; }

ASSETS=("$OUT/$PKG.zip" "$OUT/$PKG.zip.sha256")
if gh release view "$TAG" >/dev/null 2>&1; then
  # re-running for an existing release (retry, or the tag was already released): update it instead of failing
  echo "Release $TAG exists: updating notes and assets"
  gh release edit "$TAG" --title "$TAG" --notes-file "$NOTES"
  gh release upload "$TAG" "${ASSETS[@]}" --clobber
else
  # --verify-tag: never create the tag implicitly; it must already exist on the remote
  gh release create "$TAG" "${ASSETS[@]}" --title "$TAG" --notes-file "$NOTES" --verify-tag --latest
fi
echo "Published $TAG"
