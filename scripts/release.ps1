# Creates a release following git-flow: release/X.Y.Z branch from develop, CHANGELOG entry, merge into master,
# tag vX.Y.Z, merge back into develop. The version is derived from the Conventional Commits since the last tag
# (see CONTRIBUTING.md); nothing is pushed unless -Push is given.
#
#   scripts\release.ps1 -DryRun            show the version and the changelog entry, change nothing
#   scripts\release.ps1                    release locally
#   scripts\release.ps1 -Push              release and push master, develop and the tag
#   scripts\release.ps1 -Version 1.0.0     override the computed version (e.g. for the first stable release)
param(
  [string]$Version,
  [switch]$DryRun,
  [switch]$Push
)
$ErrorActionPreference = 'Stop'
Set-Location (Split-Path -Parent $PSScriptRoot)
. "$PSScriptRoot\lib\Semver.ps1"

function Invoke-Checked {
  param([string]$What, [scriptblock]$Command)
  # git and git-flow report progress on stderr; Windows PowerShell 5.1 would treat that as an error
  $ErrorActionPreference = 'Continue'
  & $Command
  if ($LASTEXITCODE -ne 0) { throw "$What failed (exit code $LASTEXITCODE)" }
}

# --- preconditions -----------------------------------------------------------------------------------------
$branch = (git branch --show-current).Trim()
if ($branch -ne 'develop') { throw "Releases are started from 'develop' (current branch: '$branch')." }
if (git status --porcelain) { throw 'The working tree is not clean; commit or stash your changes first.' }
Get-GitOutput flow version | Out-Null
if ($LASTEXITCODE -ne 0) { throw "git-flow (AVH edition) is not installed; see CONTRIBUTING.md for the plain git commands." }

# --- version -----------------------------------------------------------------------------------------------
$info = Get-VersionInfo -Ref HEAD
if ($Version) {
  if ($Version -notmatch '^\d+\.\d+\.\d+$') { throw "-Version must look like 1.2.3 (got '$Version')." }
  if ($info.LastTag -and ([version]$Version -le [version]$info.LastTag.Substring(1))) {
    throw "-Version $Version is not greater than the last release $($info.LastTag)."
  }
} else {
  if (-not $info.Next) {
    $since = if ($info.LastTag) { "since $($info.LastTag)" } else { 'in the history' }
    throw "No releasable commits (feat, fix, perf or a breaking change) $since."
  }
  $Version = $info.Next
}
if (git tag --list "v$Version") { throw "Tag v$Version already exists." }

$section = New-ChangelogSection -Version $Version -Commits $info.Commits
$last = if ($info.LastTag) { $info.LastTag } else { '(none)' }
Write-Host "Last release: $last   bump: $($info.Level)   next release: v$Version   commits: $(@($info.Commits).Count)"
Write-Host ''
Write-Host $section

if ($DryRun) { Write-Host '(dry run, nothing changed)'; return }

# --- release -----------------------------------------------------------------------------------------------
$env:GIT_MERGE_AUTOEDIT = 'no'   # git flow merges without opening an editor
Invoke-Checked 'git flow release start' { git flow release start $Version }

$path = Join-Path (Get-Location) 'CHANGELOG.md'
$intro = "# Changelog`n`nAll notable changes to this project are documented in this file. It is generated from the`nConventional Commits by scripts/release.ps1; versions follow [Semantic Versioning](https://semver.org).`n`n"
$head = $intro; $rest = ''
if (Test-Path $path) {
  $existing = [System.IO.File]::ReadAllText($path)
  $idx = $existing.IndexOf('## [')
  if ($idx -ge 0) { $head = $existing.Substring(0, $idx); $rest = $existing.Substring($idx) } else { $head = $existing.TrimEnd() + "`n`n" }
}
$text = ($head + $section.TrimEnd() + "`n`n" + $rest).TrimEnd() + "`n"
[System.IO.File]::WriteAllText($path, $text, (New-Object System.Text.UTF8Encoding $false))

Invoke-Checked 'git add' { git add CHANGELOG.md }
Invoke-Checked 'git commit' { git commit -q -m "chore(release): v$Version" }

$finish = @('flow', 'release', 'finish', '-m', "Release $Version")
if ($Push) { $finish += '-p' }
$finish += $Version
Invoke-Checked 'git flow release finish' { git @finish }

Write-Host ''
Write-Host "Released v$Version (branch: $((git branch --show-current).Trim()))."
if (-not $Push) { Write-Host 'Push with:  git push origin master develop --follow-tags' }
