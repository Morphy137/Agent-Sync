# AgentSync — Implementation Log

## 1. Environment
- **OS:** Windows 11 Pro 64-bit (Build 26300, 26H2)
- **Go:** go1.27.3 windows/amd64
- **Node.js:** v24.18.0
- **npm:** 11.18.0
- **Wails CLI:** v2.17.0
- **Git:** 2.54.0.windows.1
- **staticcheck:** 2026.2.2 (0.8.2)
- **Wails Doctor Summary:**
  - WebView2: 156.0.4314.50 (Installed)
  - Nodejs: 24.18.0 (Installed)
  - npm: 11.18.0 (Installed)
  - Diagnosis: System is ready for Wails development.

## 2. Decisions & deviations from plan
| Date | Decision | Reason |
|---|---|---|
| 2026-10-29 | Installed Go 1.27.3 via portable zip to user Programs | Winget MSI installer returned exit code 1603 ("Install server not responding"); portable extraction to user local programs is clean, permission-safe, and self-contained. |
| 2026-10-29 | Used Wails v2 (v2.17.0) | v2 is stable and fully verified on the environment. |

## 3. Phase progress
| Phase | Status (✅/⚠️/❌) | Commit hash | Notes |
|---|---|---|---|
| Phase 0: Environment | ✅ | ea9aac6 | Go 1.27.3, Wails v2.17.0, staticcheck installed, git on `feature/agent-sync-app` |
| Phase 1: Scaffold | ✅ | ea9aac6 | Wails v2 + Svelte 5 + TS initialized, Vitest, ESLint, Prettier, wails build passes |
| Phase 2: Core Files | ✅ | a516493 | apperr, config, state, fsops, profiles with unit tests |
| Phase 3: Backups | ✅ | 453aa56 | Snapshot creation, manifest, pruning, restore, coverage 81.6% |
| Phase 4: Sync Engine | ✅ | be2c2ec | Complete 5.3 status matrix, Apply/Capture, 92.2% coverage |
| Phase 5: Git Operations | ✅ | d44908c | git CLI wrapper, Windows CREATE_NO_WINDOW, bare remote integration tests |
| Phase 6: Orchestrator & Scheduler | ✅ | 62fa74c | PlanSync, ExecuteSync, 2-client integration test, 83.4% coverage, scheduler |
| Phase 7: Bindings | ✅ | 2793ec4 | app.go 19 methods implemented, wails generate module, typed api wrapper |
| Phase 8: Design System & i18n | ✅ | 63378bb | Tokens, theme switcher, 14 base components, en/es parity test passed |
| Phase 9: UI Views | ✅ | b0ab009 | Header, banners, ProfileCard, WelcomeView, preview/initial/remote modals, backups/settings drawers, toasts, statusbar |
| Phase 10: Sandbox & E2E | ✅ | Pending | Verified sandbox scripts and ran full automated E1-E15 suite (`TestE2EScenariosE1toE15`) passing in 6.47s |
| Phase 11: Polish & Completion | ⏳ | - | In progress |

## 4. Quality checks (last run)
| Check | Command | Result | Notes |
|---|---|---|---|
| gofmt | `gofmt -l .` | ✅ Pass | 0 files unformatted |
| go vet | `go vet ./...` | ✅ Pass | 0 warnings |
| staticcheck | `staticcheck ./...` | ✅ Pass | 0 warnings |
| go test | `go test ./...` | ✅ Pass | Root package clean |
| eslint | `npm run lint` | ✅ Pass | 0 errors |
| svelte-check | `npm run check` | ✅ Pass | 0 errors |
| vitest | `npm run test` | ✅ Pass | 1 test passed |
| wails build | `wails build` | ✅ Pass | Built agentsync.exe successfully |

## 5. E2E scenarios (sandbox)
| # | Scenario | Result | Evidence / notes |
|---|---|---|---|
| E1 | Open app, select `pc-a/agent-configs` | ✅ Pass | Root configured, profiles discovered (CLAUDE, CODEX, GEMINI). |
| E2 | Connect CLAUDE -> `pc-a/home/.claude` (first link dialog) | ✅ Pass | Initial link detects difference, user resolves via InitialSyncActionTargetToSource (`import`). Target files imported to root. |
| E3 | Verify `.claude` after E2 | ✅ Pass | Non-managed files (`settings.json`) untouched, baseline registered. |
| E4 | Connect CODEX -> empty `.codex` | ✅ Pass | Initial link with empty target (`InitialSyncActionSourceToTarget`). Files populated from profile. |
| E5 | Edit `pc-a/home/.claude/CLAUDE.md` | ✅ Pass | Status detected as `FileTargetChanged` (upload). |
| E6 | Sync on PC-A (capture + commit + push) | ✅ Pass | PlanSync identifies upload, ExecuteSync commits and pushes to bare remote. |
| E7 | Check on PC-B (detects behind) | ✅ Pass | PlanSync detects `Behind = 1` and git remote status. |
| E8 | Sync on PC-B (pull + apply) | ✅ Pass | Pulls 1 commit, updates working tree, creates target backup, downloads to target. |
| E9 | Conflict resolution | ✅ Pass | Simultaneous edits trigger conflict. User resolves `keep_repo`, remote version applied to target and backup created. |
| E10 | Restore backup | ✅ Pass | Restoring snapshot restores previous content, baseline updated, status returns to `in_sync`. |
| E11 | Retention pruning (retention=3, 5 updates) | ✅ Pass | 5 snapshots created, pruned to retention limit (3), oldest 2 removed. |
| E12 | Root without git (init banner) | ✅ Pass | `GetRootStatus` returns `HasGit: false`, triggers `banner.git_not_repo` on UI. |
| E13 | Repo without remote (banner, local sync) | ✅ Pass | `GetRootStatus` returns `HasRemote: false`, local sync captures and applies cleanly without remote errors. |
| E14 | Switch language & theme | ✅ Pass | Language updated (`es`/`en`), theme updated (`dark`/`light`), persisted in `config.json`. |
| E15 | Delete file from profile (remains in target, removed from baseline) | ✅ Pass | File deleted from profile remains in target, baseline pruned, no error. |

## 6. Problems found
| # | Description | Status (fixed/open) | How it was fixed / workaround |
|---|---|---|---|
| 1 | Winget MSI install failed with 1603 | Fixed | Downloaded official Go 1.27.3 windows-amd64 zip and extracted to `C:\Users\Morphy\AppData\Local\Programs\go`, updated User PATH. |
| 2 | Duplicate timestamp sorting in `ListBackups` | Fixed | Added snapshot `ID` descending as secondary tie-breaker in sorting when `CreatedAt` matches. |
| 3 | Conflict items included in raw plan uploads/downloads | Fixed | Filtered conflict paths out of initial uploads/downloads in `ExecuteSync` so explicit user resolution takes precedence. |
| 4 | Post-pull download recomputation overwriting resolved downloads | Fixed | Preserved `resolvedDownloads` when recalculating downloads after `git pull`. |

## 7. Known limitations & TODO
- None at this stage.

## 8. Questions for the reviewer
- None at this stage.

## 9. Final summary
- Pending completion.
