# Project instructions

A Windows soundboard: Go backend (single exe), React/TypeScript frontend. See [README.md](README.md) for the layout,
build and test commands, and [CONTRIBUTING.md](CONTRIBUTING.md) for the rules below in detail.

## Language

Everything in the repo is English: code, comments, the log file, error messages, docs, plans, commit messages. The
user-facing UI is localized in German and English: frontend texts in `frontend/src/i18n/{de,en}.ts`, shell texts (tray,
dialogs, flag help, console output, example content) in `backend/internal/i18n` (`catalog_{de,en}.go`). A new UI text
needs an entry in both catalogs (the type checker and a Go test enforce that). The language is `-lang`, else the saved
`language` setting (tray menu), else the Windows language. The user talks to you in German. Device status errors
travel as an English message plus an `errorCode`; the frontend translates the code. Use a plain `-` instead of the
en dash (U+2013).

## Git

- Branching is git-flow: `master` = releases only, `develop` = integration, work happens on `feature/*`
  (`git flow feature start/finish`). Do not commit directly to `master` or `develop`.
- Commit messages follow Conventional Commits; use the `commit-message` skill when writing one. A commit-msg hook
  enforces the format (`scripts\setup-git.ps1` enables it after a clone).
- The version is derived automatically from tags and commits (`scripts\version.ps1`); never edit a version by hand.
  Releases are made with `scripts\release.ps1` from `develop`.
- Only commit when asked; pushing is a separate request. Rewriting published history needs an explicit request.

## Working here

- The host has no C compiler: the exe is built in Docker (`scripts\build-backend.ps1`, `scripts\build.ps1`). The pure Go
  packages are tested on the host: `go test ./internal/audio ./internal/library ./internal/store ./internal/server ./internal/i18n`.
- The frontend is built with `npm run build` in `frontend/` (output goes to `backend/web/dist`, which is embedded).
