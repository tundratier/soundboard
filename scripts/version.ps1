# Prints the semantic version of a git ref, derived from tags and Conventional Commits.
#
#   scripts\version.ps1                  0.2.0-dev.7+1a2b3c4   (unreleased build; "-dev.N" = commits since the last tag)
#   scripts\version.ps1 -Format core     0.2.0
#   scripts\version.ps1 -Format numeric  0.2.0.7               (for the Windows file/product version resource)
#   scripts\version.ps1 -Format json     everything, incl. the bump level and the next release version
#
# A commit that is exactly at a release tag (vX.Y.Z) yields the clean version X.Y.Z.
param(
  [ValidateSet('full', 'core', 'numeric', 'json')][string]$Format = 'full',
  [string]$Ref = 'HEAD'
)
$ErrorActionPreference = 'Stop'
. "$PSScriptRoot\lib\Semver.ps1"

try {
  $info = Get-VersionInfo -Ref $Ref
} catch {
  # e.g. building from a source archive without git history
  Write-Warning "Could not derive the version from git: $_"
  $info = [pscustomobject]@{ Full = '0.0.0-unknown'; Core = '0.0.0'; Numeric = '0.0.0.0' }
}

switch ($Format) {
  'full'    { $info.Full }
  'core'    { $info.Core }
  'numeric' { $info.Numeric }
  'json'    { $info | Select-Object LastTag, Level, Next, Core, Full, Numeric, IsRelease, CommitCount, Sha | ConvertTo-Json }
}
