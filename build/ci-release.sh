#!/bin/bash
# Bitbucket pipeline: builds the Windows exe for the release tag at HEAD and publishes it as a repository download.
# Bitbucket has no "releases" like GitHub; the tag vX.Y.Z marks the release and the downloads hold the artifacts.
#
# Run from the repository root with the frontend already built (backend/web/dist) and gcc-mingw-w64 + zip installed.
# Environment:
#   BITBUCKET_REPO_FULL_NAME  workspace/repo (set by Bitbucket Pipelines)
#   BB_ACCESS_TOKEN           repository access token with "Repositories: Write" (secured repository variable), or
#   BB_USERNAME + BB_APP_PASSWORD   Bitbucket user and app password/API token with repository write access
#   RELEASE_TAG               optional: use this tag instead of the one at HEAD (for testing)
#   DRY_RUN=1                 build and package, but do not upload
#   SKIP_BUILD=1              package the existing dist/*.exe instead of building (local testing)
#   BB_API_URL                API base URL (default https://api.bitbucket.org/2.0; overridable for tests)
set -euo pipefail

ROOT="${BITBUCKET_CLONE_DIR:-$(cd "$(dirname "$0")/.." && pwd)}"
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
rm -rf "${OUT:?}/$PKG" "$OUT/$PKG.zip" "$OUT/$PKG.zip.sha256" "$OUT/$PKG-release-notes.md"
mkdir -p "$OUT/$PKG"
cp "$OUT/soundboard.exe" "$OUT/soundboard-console.exe" README.md "$OUT/$PKG/"
[[ -f CHANGELOG.md ]] && cp CHANGELOG.md "$OUT/$PKG/"
(cd "$OUT" && zip -qr "$PKG.zip" "$PKG" && sha256sum "$PKG.zip" > "$PKG.zip.sha256")

# release notes: the matching section of CHANGELOG.md
if [[ -f CHANGELOG.md ]]; then
  awk -v v="$VERSION" 'index($0, "## [" v "]") == 1 { on = 1; next } /^## \[/ { on = 0 } on' CHANGELOG.md \
    | sed '/./,$!d' > "$OUT/$PKG-release-notes.md"
fi
[[ -s "$OUT/$PKG-release-notes.md" ]] || echo "Release $VERSION" > "$OUT/$PKG-release-notes.md"

echo "Packaged:"
ls -l "$OUT/$PKG.zip" "$OUT/$PKG.zip.sha256" "$OUT/$PKG-release-notes.md"
if [[ "${DRY_RUN:-0}" == "1" ]]; then
  echo "DRY_RUN=1: not uploading"
  exit 0
fi

# --- publish as repository downloads ----------------------------------------------------------------------------
: "${BITBUCKET_REPO_FULL_NAME:?BITBUCKET_REPO_FULL_NAME is not set}"
if [[ -n "${BB_ACCESS_TOKEN:-}" ]]; then
  AUTH=(-H "Authorization: Bearer $BB_ACCESS_TOKEN")
elif [[ -n "${BB_USERNAME:-}" && -n "${BB_APP_PASSWORD:-}" ]]; then
  AUTH=(--user "$BB_USERNAME:$BB_APP_PASSWORD")
else
  echo "error: set the secured repository variable BB_ACCESS_TOKEN (or BB_USERNAME and BB_APP_PASSWORD)" >&2
  exit 1
fi

# uploading a file with an existing name replaces it, so re-running a pipeline is safe
for f in "$OUT/$PKG.zip" "$OUT/$PKG.zip.sha256" "$OUT/$PKG-release-notes.md"; do
  echo "Uploading $(basename "$f")"
  curl --fail-with-body -sS -X POST "${AUTH[@]}" \
    "${BB_API_URL:-https://api.bitbucket.org/2.0}/repositories/$BITBUCKET_REPO_FULL_NAME/downloads" \
    --form "files=@$f"
  echo
done
echo "Published $TAG: https://bitbucket.org/$BITBUCKET_REPO_FULL_NAME/downloads/"
