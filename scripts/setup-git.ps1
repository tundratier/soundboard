# One-time setup after cloning: versioned git hooks and the git-flow configuration.
# The git-flow keys are written directly, so no interactive "git flow init" is needed.
$ErrorActionPreference = "Stop"
Set-Location (Split-Path -Parent $PSScriptRoot)

git config core.hooksPath .githooks

$flow = [ordered]@{
  "gitflow.branch.master"      = "master"
  "gitflow.branch.develop"     = "develop"
  "gitflow.prefix.feature"     = "feature/"
  "gitflow.prefix.bugfix"      = "bugfix/"
  "gitflow.prefix.release"     = "release/"
  "gitflow.prefix.hotfix"      = "hotfix/"
  "gitflow.prefix.support"     = "support/"
  "gitflow.prefix.versiontag"  = "v"
}
foreach ($key in $flow.Keys) { git config $key $flow[$key] }

# make sure a local develop branch exists
git show-ref --verify --quiet refs/heads/develop
if ($LASTEXITCODE -ne 0) {
  git show-ref --verify --quiet refs/remotes/origin/develop
  if ($LASTEXITCODE -eq 0) { git branch develop origin/develop } else { git branch develop master }
}

Write-Host "Done: hooks path = .githooks, git-flow configured (master/develop, tag prefix v)."
if (-not (Get-Command git-flow -ErrorAction SilentlyContinue)) {
  Write-Host "Note: the 'git flow' command is not installed; the plain git commands in CONTRIBUTING.md work as well."
}
