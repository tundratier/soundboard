# Shared helpers for scripts/version.ps1 and scripts/release.ps1:
# derive the semantic version from git tags (vX.Y.Z) and Conventional Commits. See CONTRIBUTING.md.
#
# Bump rules for the commits since the last release tag (the highest wins):
#   breaking change ("!" or a "BREAKING CHANGE:" footer)  -> major (while major is 0: minor)
#   feat                                                  -> minor
#   fix, perf, revert                                     -> patch
#   everything else (docs, style, refactor, test, build, ci, chore) -> no release on its own

Set-StrictMode -Version Latest

# git prints UTF-8; make sure commit subjects with non-ASCII characters are read correctly
try { [Console]::OutputEncoding = [System.Text.Encoding]::UTF8 } catch { }

# Runs git and returns its stdout lines. Never throws: Windows PowerShell 5.1 turns anything a native command
# writes to stderr into a terminating error under $ErrorActionPreference = 'Stop', even with 2>$null.
# Callers check the result (or $LASTEXITCODE) themselves.
function Get-GitOutput {
  $old = $ErrorActionPreference
  $ErrorActionPreference = 'Continue'
  try { return @(& git @args 2>$null) } finally { $ErrorActionPreference = $old }
}

$script:ReleaseTagPattern = '^v(\d+)\.(\d+)\.(\d+)$'

function Get-LastReleaseTag {
  param([string]$Ref = 'HEAD')
  # all stable tags reachable from $Ref, highest version first
  $tags = Get-GitOutput tag --merged $Ref --list 'v*' --sort=-v:refname
  foreach ($t in $tags) { if ($t -match $script:ReleaseTagPattern) { return $t } }
  return $null
}

function Get-CommitsSince {
  param([string]$Tag, [string]$Ref = 'HEAD')
  $range = if ($Tag) { "$Tag..$Ref" } else { $Ref }
  # records are separated by 0x1e, fields by 0x1f; merge commits are skipped (their content commits count)
  $raw = (Get-GitOutput log $range --no-merges '--format=%H%x1f%s%x1f%b%x1e') -join "`n"
  $commits = @()
  foreach ($rec in ($raw -split [char]0x1e)) {
    $rec = $rec.Trim()
    if (-not $rec) { continue }
    $f = $rec -split [char]0x1f, 3
    $subject = $f[1]
    $body = if ($f.Count -gt 2) { $f[2] } else { '' }
    $type = $null; $scope = $null; $bang = $false; $desc = $subject
    if ($subject -match '^(?<type>[a-z]+)(\((?<scope>[^)]+)\))?(?<bang>!)?: (?<desc>.+)$') {
      $type = $Matches['type']
      $scope = $Matches['scope']
      $bang = [bool]$Matches['bang']
      $desc = $Matches['desc']
    }
    $breaking = $bang -or ($body -match '(?m)^BREAKING[ -]CHANGE: ')
    $commits += [pscustomobject]@{
      Hash = $f[0]; Short = $f[0].Substring(0, 7); Subject = $subject; Body = $body
      Type = $type; Scope = $scope; Description = $desc; Breaking = $breaking
    }
  }
  return , $commits
}

function Get-BumpLevel {
  param($Commits)
  $rank = @{ none = 0; patch = 1; minor = 2; major = 3 }
  $best = 'none'
  foreach ($c in $Commits) {
    $level = if ($c.Breaking) { 'major' }
    elseif ($c.Type -eq 'feat') { 'minor' }
    elseif ($c.Type -in 'fix', 'perf', 'revert') { 'patch' }
    else { 'none' }
    if ($rank[$level] -gt $rank[$best]) { $best = $level }
  }
  return $best
}

function Step-Version {
  param([string]$Version, [string]$Level)
  if ($Version -notmatch '^(\d+)\.(\d+)\.(\d+)$') { throw "Not a semantic version: $Version" }
  $major = [int]$Matches[1]; $minor = [int]$Matches[2]; $patch = [int]$Matches[3]
  # 0.y.z is the initial development phase: breaking changes bump the minor version
  if ($Level -eq 'major' -and $major -eq 0) { $Level = 'minor' }
  switch ($Level) {
    'major' { return "$($major + 1).0.0" }
    'minor' { return "$major.$($minor + 1).0" }
    'patch' { return "$major.$minor.$($patch + 1)" }
    default { return $Version }
  }
}

function Get-VersionInfo {
  param([string]$Ref = 'HEAD')

  $lastTag = Get-LastReleaseTag -Ref $Ref
  $base = if ($lastTag) { $lastTag.Substring(1) } else { '0.0.0' }
  $commits = Get-CommitsSince -Tag $lastTag -Ref $Ref
  $level = Get-BumpLevel -Commits $commits
  # the version the next release would get; $null if nothing releasable happened
  $next = if ($level -eq 'none') { $null } else { Step-Version -Version $base -Level $level }

  $sha = ((Get-GitOutput rev-parse --short=7 $Ref) -join "").Trim()
  $range = if ($lastTag) { "$lastTag..$Ref" } else { $Ref }
  $count = [int]((Get-GitOutput rev-list --count $range) -join "").Trim()
  $exact = (Get-GitOutput describe --exact-match --tags --match 'v*' $Ref) -join ''
  $isRelease = [bool]($exact -and ($exact -match $script:ReleaseTagPattern))

  if ($isRelease) {
    $core = $exact.Substring(1)
    $full = $core
    $count = 0
  } else {
    # unreleased build: the version it is heading to, as a pre-release, plus build metadata
    $core = if ($next) { $next } else { Step-Version -Version $base -Level 'patch' }
    $full = "$core-dev.$count+$sha"
    $dirty = Get-GitOutput status --porcelain
    if ($dirty) { $full += '.dirty' }
  }

  return [pscustomobject]@{
    LastTag    = $lastTag
    Commits    = $commits
    Level      = $level
    Next       = $next
    Core       = $core
    Full       = $full
    # for the Windows file/product version resource: 4 numeric parts, each at most 65535
    Numeric    = "$core.$([math]::Min($count, 65535))"
    IsRelease  = $isRelease
    CommitCount = $count
    Sha        = $sha
  }
}

function New-ChangelogSection {
  param([string]$Version, $Commits, [string]$Date = (Get-Date -Format 'yyyy-MM-dd'))

  $sections = [ordered]@{
    'Breaking changes' = { param($c) $c.Breaking }
    'Features'         = { param($c) $c.Type -eq 'feat' -and -not $c.Breaking }
    'Bug fixes'        = { param($c) $c.Type -eq 'fix' -and -not $c.Breaking }
    'Performance'      = { param($c) $c.Type -eq 'perf' -and -not $c.Breaking }
    'Reverts'          = { param($c) $c.Type -eq 'revert' -and -not $c.Breaking }
  }
  $lines = @("## [$Version] - $Date", '')
  foreach ($title in $sections.Keys) {
    $items = @($Commits | Where-Object { & $sections[$title] $_ })
    if ($items.Count -eq 0) { continue }
    $lines += "### $title"
    $lines += ''
    foreach ($c in $items) {
      $scope = if ($c.Scope) { "**$($c.Scope):** " } else { '' }
      $lines += "- $scope$($c.Description) ($($c.Short))"
    }
    $lines += ''
  }
  # a release without feat/fix/perf/breaking commits (forced with -Version): list the internal changes instead
  if ($lines.Count -eq 2) {
    $other = @($Commits | Where-Object { $_.Type -and -not ($_.Type -eq 'chore' -and $_.Scope -eq 'release') })
    $lines += '### Other changes'
    $lines += ''
    $lines += 'No user-visible changes.'
    $lines += ''
    foreach ($c in $other) {
      $scope = if ($c.Scope) { "**$($c.Scope):** " } else { '' }
      $lines += "- $($c.Type): $scope$($c.Description) ($($c.Short))"
    }
    $lines += ''
  }
  return ($lines -join "`n")
}
