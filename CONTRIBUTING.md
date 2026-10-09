# Contributing

Everything in this repository is written in English (code, comments, logs, error messages, docs, commit messages).
Only the user-facing UI is German: the frontend texts and, on the Go side, `backend/cmd/soundboard/ui_de.go`.

## Setup after cloning

```powershell
scripts\setup-git.ps1
```

This enables the versioned git hooks (`core.hooksPath = .githooks`, which includes the commit message check) and writes
the git-flow configuration. Building and testing is described in [README.md](README.md).

## Branching model: git-flow

| Branch | Purpose |
|---|---|
| `master` | Released versions only. Every release is a merge commit tagged `vX.Y.Z`. Never commit here directly. |
| `develop` | Integration branch for the next release. Features are merged here. |
| `feature/<name>` | One feature or change. Branches off `develop`, merged back with `--no-ff`. |
| `bugfix/<name>` | Like a feature, for a non-urgent fix on `develop`. |
| `release/X.Y.Z` | Created by `scripts\release.ps1`; stabilizes and releases a version. |
| `hotfix/X.Y.Z` | Urgent fix of a released version. Branches off `master`, merged into `master` and `develop`. |

With the `git flow` command (git-flow AVH edition, included in Git for Windows):

```powershell
git flow feature start my-change        # branch feature/my-change from develop
# ... commit ...
git flow feature finish my-change       # merge --no-ff into develop, delete the branch

git flow hotfix start 0.2.1             # from master; use the next patch version of the last release
# ... commit the fix (type "fix") ...
git flow hotfix finish -m "Hotfix 0.2.1" 0.2.1   # merge into master and develop, tag v0.2.1
```

Without the tool, the same with plain git: `git switch -c feature/my-change develop`, then
`git switch develop; git merge --no-ff feature/my-change; git branch -d feature/my-change`.

Pushing (always both long-lived branches and the tags): `git push origin master develop --follow-tags`.

## Commit messages: Conventional Commits

```
<type>[(scope)][!]: <description>

[body]

[footer(s)]
```

- **description**: imperative, lowercase start, no trailing period, the whole first line at most 72 characters.
- **body** (optional): explain *what and why*, wrapped at about 72 characters, separated by a blank line.
- **breaking change**: put `!` after the type/scope and/or add a `BREAKING CHANGE: <what breaks>` footer.
- Trailers such as `Co-Authored-By:` or `Refs: #12` go into the footer.

| Type | Use for | Version effect |
|---|---|---|
| `feat` | a new user-visible feature | minor |
| `fix` | a bug fix | patch |
| `perf` | a performance improvement | patch |
| `revert` | reverting an earlier commit | patch |
| `docs` | documentation and code comments only | none |
| `style` | formatting only (no code change) | none |
| `refactor` | restructuring without behavior change | none |
| `test` | adding or fixing tests | none |
| `build` | build system, scripts, dependencies, Docker | none |
| `ci` | CI configuration | none |
| `chore` | everything else (tooling, releases) | none |

Any type with `!` or a `BREAKING CHANGE:` footer is a breaking change: major, or minor while the version is `0.y.z`.

Suggested scopes (omit the scope when a change spans several areas): `audio`, `server`, `store`, `library`,
`winaudio`, `shell` (tray, startup, single instance), `board` (the public board views), `admin`, `frontend`,
`docker`, `scripts`, `release`.

Examples:

```
feat(boards): add multiple boards with overview and admin tabs
fix(audio): prevent a click when a sound is stopped
refactor: convert code, logs, errors and docs to English
feat(store)!: rename the board list in config.json

BREAKING CHANGE: configs written by older versions are no longer read.
```

The commit-msg hook rejects messages that do not follow this format (merge commits and `fixup!`/`squash!`/`amend!`
commits created by git pass). The `commit-message` skill in `.claude/skills` teaches Claude Code the same rules.

## Versioning (Semantic Versioning, automatic)

The version is never edited by hand. It is derived from the git tags (`vX.Y.Z`) and the Conventional Commits:

- The last release tag reachable from the commit is the base (`0.0.0` if there is none).
- The highest bump among the commits since that tag decides the next version (see the table above).
- A commit exactly at a release tag has the clean version `X.Y.Z`. Anything else is a pre-release of the next
  version, e.g. `0.2.0-dev.7+1a2b3c4` (`dev.7` = commits since the last tag, `+...` = commit, `.dirty` if the working tree
  has uncommitted changes). If only non-releasing commits were made, the patch version is bumped.
- While the major version is `0`, breaking changes bump the minor version. Use `scripts\release.ps1 -Version 1.0.0`
  for the first stable release.

```powershell
scripts\version.ps1                  # 0.2.0-dev.7+1a2b3c4
scripts\version.ps1 -Format json     # last tag, bump level, next release version, ...
```

The build embeds the version: `soundboard.exe -version`, the file/product version in the exe properties and the header of
the admin UI show it.

## Releasing

From a clean `develop`:

```powershell
scripts\release.ps1 -DryRun    # show the next version and the changelog entry
scripts\release.ps1            # release/X.Y.Z, CHANGELOG.md, merge into master, tag vX.Y.Z, merge back into develop
git push origin master develop --follow-tags
```

`scripts\release.ps1 -Push` pushes as part of the release. A release needs at least one `feat`, `fix`, `perf` or breaking
commit since the last tag. Rebuild the exe afterwards on the tag (`scripts\build.ps1`) to get the clean version.

## CI release (GitHub Actions)

Every push to `master` runs `.github/workflows/release.yml`: it builds the frontend, cross-compiles the exe for Windows and
publishes a **GitHub release**. `master` only receives git-flow releases, so a push there is a release; the tag `vX.Y.Z`
at `HEAD` (pushed together with `--follow-tags`) names it. If `HEAD` has no release tag the workflow does nothing.

The release `vX.Y.Z` gets:

- `soundboard-vX.Y.Z-windows-amd64.zip` - `soundboard.exe`, `soundboard-console.exe`, `README.md`, `LICENSE`, `CHANGELOG.md`
- `soundboard-vX.Y.Z-windows-amd64.zip.sha256` - checksum
- the changelog section of this version as the release notes

Re-running the workflow for an existing release updates its notes and replaces its assets. A workflow run can also be
started by hand (*Actions > Release > Run workflow*), for example if the tag was pushed after the branch.

One-time setup on GitHub (no secrets or tokens are needed; the workflow uses the built-in `GITHUB_TOKEN`):

1. *Settings > Actions > General > Actions permissions*: allow actions (at least GitHub-authored ones).
2. *Settings > Actions > General > Workflow permissions*: **Read and write permissions**. The workflow asks for
   `contents: write` itself, but an organization or repository policy that forces read-only tokens overrides that.
3. If `master` is a protected branch, the release script's push (merge commit and tag) must be allowed for you; the
   workflow itself never pushes.

Tags are not pushed by a plain `git push`: use `git push origin master develop --follow-tags` (or
`scriptselease.ps1 -Push`). Pushing only a tag does not start the workflow (it runs on pushes to `master`); use the
manual run for that. `v0.1.0` and `v0.1.1` predate the workflow and therefore have no GitHub release.

The packaging logic lives in `build/ci-release.sh` and can be tried locally without publishing, using the existing exes:
`RELEASE_TAG=v0.1.1 DRY_RUN=1 SKIP_BUILD=1 bash build/ci-release.sh` (needs `zip`).
