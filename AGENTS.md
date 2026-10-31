# AGENTS.md

Instructions for AI coding agents working on AgentSync (Wails v2 + Go backend, Svelte 5 + TypeScript frontend).

## Project map

- `internal/` — Go backend (`service` is what the app, CLI and MCP server call). See `docs/ARCHITECTURE.md`.
- `frontend/src/` — Svelte UI. Strings live in `frontend/src/lib/i18n/locales/{en,es}` (keep keys in parity).
- `cmd/agentsync-cli`, `internal/cli`, `internal/mcpserver` — CLI and MCP server. See `docs/CLI_AND_MCP.md`.
- `skills/agentsync/SKILL.md` — skill for agents that use AgentSync; update it when CLI/MCP behavior changes.

## Changelog (required)

`CHANGELOG.md` is shown to users inside the app (Settings → Help → What's new), so keep it current:

- Every user-visible change adds a bullet under `## [Unreleased]` in the right group:
  `### Added`, `### Changed`, `### Fixed` or `### Removed`.
- Write for users, not developers: short, plain sentences about what they can see or do. No file
  names, function names or internal details.
- On a release, rename `## [Unreleased]` to `## [x.y.z] - YYYY-MM-DD` (same version as
  `wails.json` `info.productVersion`) and start a new empty `## [Unreleased]` section above it.
- Internal-only changes (refactors, tests, CI) do not go in the changelog.

## Releases

See `docs/INSTALL_AND_UPDATES.md`. Bump `wails.json` `info.productVersion` and `frontend/package.json`,
finalize the changelog section, merge into `main`, then push a `vX.Y.Z` tag: the Release workflow builds
the installer and publishes the assets. The in-app updater downloads `agentsync-setup.exe` and checks it
against `SHA256SUMS.txt`, so never rename those assets.

## Checks before finishing

```bash
go vet ./... && go test ./...
cd frontend && npx eslint . && npx svelte-check --threshold warning && npx vitest run
wails build
```

## Conventions

- Conventional Commits in English; work on `feature/*`, `fix/*` or `chore/*` branches, merge into `development`.
- No hardcoded user-facing strings; no magic numbers.
- Go tests must not touch the real user config (`internal/testenv` is wired through every package's `TestMain`).
