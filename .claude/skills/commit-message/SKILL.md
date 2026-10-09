---
name: commit-message
description: Write git commit messages in the Conventional Commits format for this repository. Use whenever you create a commit, amend or reword a commit message, or name a merge/squash, and when the user asks for a commit or for the version impact of a change. Covers type, scope, subject, body, breaking changes, trailers and the git-flow branch to commit on.
---

# Commit messages for this repository

Commit messages follow [Conventional Commits](https://www.conventionalcommits.org). The semantic version of the app
is derived from them automatically (`scripts\version.ps1`), so the type is not cosmetic: a wrong type means a wrong
version. A `commit-msg` hook (`.githooks/commit-msg`) rejects malformed messages. Full rules: `CONTRIBUTING.md`.

## Steps

1. **Look at what is staged**: `git diff --cached --stat`, then the diff itself if the stat is not enough. Describe
   the change as it is, not the conversation that led to it. If unrelated changes are staged together, suggest
   splitting them into separate commits.
2. **Check the branch** (`git branch --show-current`). Work belongs on `feature/*`, `bugfix/*`, `release/*` or
   `hotfix/*`. Do not commit directly to `master`; commit to `develop` only for merges done by git-flow. If you are on
   one of them with a regular change, say so and offer `git flow feature start <name>`.
3. **Pick the type** (first match wins):

   | Type | When | Version |
   |---|---|---|
   | `feat` | new user-visible behavior | minor |
   | `fix` | a bug is fixed | patch |
   | `perf` | faster or lighter, same behavior | patch |
   | `revert` | undoes an earlier commit | patch |
   | `docs` | docs or code comments only | none |
   | `style` | whitespace/formatting only | none |
   | `refactor` | restructuring, no behavior change | none |
   | `test` | tests only | none |
   | `build` | build scripts, Docker, dependencies | none |
   | `ci` | CI configuration | none |
   | `chore` | tooling, releases, everything else | none |

   A breaking change (users or other code must change something) is marked with `!` and/or a `BREAKING CHANGE:` footer
   and bumps major (minor while the version is 0.y.z). Be honest about it; do not mark a change breaking just because
   internals moved.
4. **Pick a scope** (optional, lowercase): `audio`, `server`, `store`, `library`, `winaudio`, `shell` (tray, startup),
   `board` (public board views), `admin`, `frontend`, `docker`, `scripts`, `release`. Leave it out when the change spans
   several areas.
5. **Write the subject**: `<type>(<scope>): <description>`. Imperative mood ("add", not "added"), lowercase start, no
   trailing period, **at most 72 characters in total**, English.
6. **Write a body when the why is not obvious** (blank line after the subject, wrapped at about 72 characters): what
   changed and why, what was verified, what was not. Skip the body for trivial changes.
7. **Footers**: `BREAKING CHANGE: <description>`, `Refs: ...`, and any `Co-Authored-By:` trailer the harness asks for.
8. **Commit with a heredoc** so the formatting survives, then confirm with `git log -1`. If the hook rejects the
   message, fix the message; never bypass the hook with `--no-verify`.

## Language and style

English only, even though the user talks German. Use a plain `-`, not the en dash (U+2013). UI texts are German
but commit messages never are.

## Examples

```
feat(boards): add multiple boards with overview and admin tabs
fix(audio): fade out a stopped sound to avoid a click
perf(library): decode sounds on a worker pool
refactor: convert code, logs, errors and docs to English
docs: translate all code comments to English
build: derive the app version from git tags and conventional commits
chore(release): v0.2.0
```

Breaking change:

```
feat(store)!: store boards in a list in config.json

BREAKING CHANGE: configs written by older versions are no longer read;
the first start migrates them and keeps a .v1.bak copy.
```

## Wrong, and why

- `Added multiple boards` - no type, past tense.
- `feat: Add multiple boards.` - capitalized description and trailing period.
- `fix: stuff` - says nothing.
- `feat: translate everything to English` - a refactor/docs change, not a user-visible feature; `feat` would release a
  new minor version for nothing.
