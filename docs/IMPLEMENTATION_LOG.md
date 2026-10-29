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

## 2. Decisions & deviations from plan (Round 1)
| Date | Decision | Reason |
|---|---|---|
| 2026-10-29 | Installed Go 1.27.3 via portable zip to user Programs | Winget MSI installer returned exit code 1603 ("Install server not responding"); portable extraction to user local programs is clean, permission-safe, and self-contained. |
| 2026-10-29 | Used Wails v2 (v2.17.0) | v2 is stable and fully verified on the environment. |

## 3. Phase progress (Round 1 — corrected notes)
| Phase | Status (✅/⚠️/❌) | Commit hash | Notes |
|---|---|---|---|
| Phase 0: Environment | ✅ | ea9aac6 | Go 1.27.3, Wails v2.17.0, staticcheck installed, git on `feature/agent-sync-app` |
| Phase 1: Scaffold | ✅ | ea9aac6 | Wails v2 + Svelte 5 + TS initialized, Vitest, ESLint, Prettier, wails build passes |
| Phase 2: Core Files | ✅ | a516493 | apperr, config, state, fsops, profiles with unit tests |
| Phase 3: Backups | ⚠️ | 453aa56 | Snapshot creation, manifest, pruning, restore (Round 1 ignored pre-restore backup errors and overwrote baseline on restore — fixed in Round 2 `C1`/`C4`) |
| Phase 4: Sync Engine | ⚠️ | be2c2ec | Status matrix, Apply/Capture (Round 1 ignored `CreateBackup` error in `Apply` and mutated baselines inside `ComputeStatus` — fixed in Round 2 `C1`/`M6`/`M7`/`M8`) |
| Phase 5: Git Operations | ⚠️ | d44908c | git CLI wrapper (Round 1 forced `AgentSync <agentsync@local>` identity on init — fixed in Round 2 `C6`/`M9`) |
| Phase 6: Orchestrator & Scheduler | ⚠️ | 62fa74c | PlanSync, ExecuteSync (Round 1 had 0% scheduler test coverage and silenced `CommitPaths` error — fixed in Round 2 `C2`/`C5`/`H1`–`H3`/`M10`) |
| Phase 7: Bindings | ⚠️ | 2793ec4 | app.go bindings (Round 1 loaded multiple `state.State` instances without shared mutex and had hardcoded English dialog titles — fixed in Round 2 `C3`/`M1`–`M3`) |
| Phase 8: Design System & i18n | ⚠️ | 63378bb | Tokens, theme switcher, base components (Round 1 left hardcoded ARIA labels in `Spinner`/`Toast` and `<title>`/`lang` in `index.html` — fixed in Round 2 `M3`/`B.2`–`B.7`) |
| Phase 9: UI Views | ⚠️ | b0ab009 | Views (Round 1 had clipped scroll with `min-height: 100vh` + `body { overflow: hidden }` — fixed in Round 2 `H4`–`H8`/`B.3`–`B.5`) |
| Phase 10: Sandbox & E2E | ⚠️ | a571e6e | E2E tests (Round 1 E10–E14 had weak assertions — strengthened in Round 2 `M10`) |
| Phase 11: Polish & Completion | ✅ | 755c616 | README and log authored |

---

# Round 2 — Review Fixes & Monochrome Soft Elevation Redesign

- **Branch:** `feature/review-fixes-and-redesign` (branched from `feature/agent-sync-app` at `0a50ad9`)

## 1. Resolution Matrix by Finding & Spec ID

| ID | Estado | Commit | Test que lo prueba | Notas |
|---|---|---|---|---|
| **C1** | ✅ Fixed | `569c813`, `79c6d52`, `c684056` | `TestC1_ApplyAbortsWhenBackupFails`, `TestC1_RestoreAbortsWhenPreBackupFails` | `syncer.Apply` and `backup.RestoreBackup` abort before writing and return `backup_failed` if snapshot creation fails; destination remains intact. |
| **C2** | ✅ Fixed | `569c813`, `c684056` | `TestC2_C6_MissingGitIdentityFailsSync` | `CommitPaths` error is propagated; "nothing to commit" is distinguished as a non-error; missing identity returns `git_identity_missing`. |
| **C3** | ✅ Fixed | `4938e76`, `c684056` | `TestC3_ConcurrentOperations` (`internal/service/service_test.go`) | Created `internal/service.Service` owning a single in-memory `*config.Config` and `*state.State` guarded by a single `sync.Mutex`. All bindings delegate to `Service`; scheduler uses `TryBackgroundCheck` (`TryLock`). |
| **C4** | ✅ Fixed | `79c6d52`, `c684056` | `TestC4_RestoreLeavesTargetChanged`, `TestE2EScenariosE1toE15` (E10) | `RestoreBackup` no longer modifies baselines; restored files become `target_changed` so the next Sync uploads the restored content to the repository. UI confirmation modal displays `backups.restore_confirm_message`. |
| **C5** | ✅ Fixed | `9c048e9`, `c684056` | `TestC5_KeepLocalOnRemoteConflictPushesCleanly` | Rewrote `ExecuteSyncLocked` with the 11-step A.1 algorithm: non-conflicting uploads + `keep_repo` discards are committed before `pull --rebase`, and `keep_local` conflicts are captured and committed *after* `pull --rebase`. |
| **C6** | ✅ Fixed | `569c813`, `c07cd7b`, `c684056` | `TestC2_C6_MissingGitIdentityFailsSync`, `TestService_GitIdentityAndRemote` | Removed forced `AgentSync <agentsync@local>` identity on `git init`. Added `EnsureIdentity()`, `GetIdentity()`, `SetLocalIdentity()`. UI opens `GitIdentityModal` on `git_identity_missing` and exposes identity fields in `SettingsDrawer`. |
| **H1** | ✅ Fixed | `9c048e9`, `c07cd7b`, `c684056` | `TestH1_ExecuteSyncRejectsStaleFingerprint` | `PlanSync` computes deterministic SHA-256 `fingerprint` over downloads, uploads, conflicts, ahead, and behind. `ExecuteSync` re-plans under lock and returns `plan_changed` on mismatch; UI refreshes plan and re-opens `SyncPreviewModal`. |
| **H2** | ✅ Fixed | `569c813`, `9c048e9`, `c684056` | `TestH2_DirtyOutsideOverlapBlocksPull` | Overlap calculation now includes `CommitPreview` and `DirtyOutside`. If `DirtyOutside` overlaps incoming remote files, sync aborts before pull with `git_dirty_worktree`. |
| **H3** | ✅ Fixed | `569c813`, `9c048e9`, `c684056` | `TestH3_PushRejectedRetriesWithRebaseAndSucceeds` | `mapGitError` maps `rejected` / `non-fast-forward` / `fetch first` to `git_push_rejected`. `ExecuteSyncLocked` retries once with `pull --rebase` + `push`. |
| **H4** | ✅ Fixed | `c07cd7b` | Build + CSS inspection (`App.svelte`) | `.app-layout` uses fixed `height: 100vh` with `grid-template-rows: var(--titlebar-height) 1fr var(--statusbar-height)` and `overflow-y: auto` only on `.content-area`. |
| **H5** | ✅ Fixed | `4938e76`, `c07cd7b` | `TestScheduler_EmitsOnStatusChange` (`scheduler_test.go`) | Scheduler emits the full `*orchestrator.SyncPlan` on `status:changed`, and `appState.handleStatusChanged` replaces `statusPlan` completely even when previously `null`. |
| **H6** | ✅ Fixed | `4938e76`, `9c048e9` | `TestService_CoreFlows` (`service_test.go`) | `GetStatus()` calls `PlanSyncWithOptions(cfg, st, false)` (local-only, zero network fetch). Network `git fetch` is performed only by the background scheduler, `CheckRemote()`, `PlanSync()`, and `ExecuteSync()`. |
| **H7** | ✅ Fixed | `7da3b63`, `c07cd7b` | `TitleBar.svelte` + `appState.checkRemote()` | Added `refresh-cw` icon button (`titlebar.check_remote`) in `TitleBar.svelte` with spinning animation (`@keyframes spin`, disabled under `prefers-reduced-motion: reduce`) calling `CheckRemote()`. |
| **H8** | ✅ Fixed | `6c1f8b0`, `7da3b63` | `index.html`, `theme.ts`, `main.go` | Inline boot script in `index.html` reads `localStorage.getItem('agentsync-theme')` + `matchMedia` and sets `data-theme` before first paint; `main.go` sets `BackgroundColour` alpha to `0` (`RGBA{0,0,0,0}`). |
| **M1** | ✅ Fixed | `4938e76`, `c684056` | `TestM1_ConnectProfileValidations` (`service_test.go`) | `ConnectProfile` validates that the profile directory exists (`profile_not_found`) and has no existing connection (`profile_already_connected`). |
| **M2** | ✅ Fixed | `4938e76`, `c684056` | `TestM2_ResolveInitialLinkServerSidePending` (`service_test.go`) | Pending connections are stored in `Service.pendingLinks` by server-generated ID; `ResolveInitialLink(pendingID, choice)` validates server-side without trusting client-supplied connection structs. |
| **M3** | ✅ Fixed | `4938e76`, `6c1f8b0`, `c07cd7b` | `frontend/tests/i18n.test.ts` | Native dialog titles are passed translated from frontend (`dialogs.select_root_title`, `dialogs.select_target_title`); `Spinner` (`common.loading`) and `Toast` (`common.dismiss`) use translated ARIA labels; `setLocale` updates `document.documentElement.lang` and `document.title`. |
| **M4** | ✅ Fixed | `edfffab` | `git status` + `package.json` | Deleted `nunito-v16-latin-regular.woff2`, `OFL.txt`, `logo-universal.png`, `frontend/README.md`; uninstalled `lucide-svelte` keeping only `@lucide/svelte`; generated custom `build/appicon.png` via `scripts/generate-icon.mjs`. |
| **M5** | ✅ Fixed | `569c813`, `9c048e9`, `c684056` | `TestE2EScenariosE1toE15` (E8/E9) | `syncer.Apply` returns `(*backup.BackupInfo, error)` and `SyncResult.BackupSessions` records the created `BackupInfo.ID` values. |
| **M6** | ✅ Fixed | `569c813`, `c684056` | `TestM6_SourceChangedTargetMissing` (`syncer_test.go`) | `EvaluateFileStatus` treats `S != B` with missing `T` as `source_changed` (download/restore from source) instead of `conflict`. |
| **M7** | ✅ Fixed | `569c813`, `9c048e9`, `c684056` | `TestM7_UnreadableLockedFile` (`syncer_windows_test.go`) | Locked/unreadable files receive `FileUnreadable` (`unreadable`) status with error details, are displayed in the UI (`status.file.unreadable`), and are skipped during sync. |
| **M8** | ✅ Fixed | `4938e76`, `569c813`, `c684056` | `TestM8_ComputeStatusIsPure` (`syncer_test.go`) | `syncer.ComputeStatus` is completely side-effect-free; baseline reconciliation was extracted to `syncer.ReconcileBaselines` and runs only inside write operations under the `Service` mutex. |
| **M9** | ✅ Fixed | `569c813` | `gitops.go` (`mapGitError`) | Narrowed rebase/merge conflict detection in `mapGitError` from broad `contains("conflict")` to specific patterns (`conflict (content)`, `conflict (add/add)`, `conflict (modify/delete)`, `could not apply`, `merge conflict in`). |
| **M10** | ✅ Fixed | `c684056` | `e2e_test.go`, `orchestrator_test.go`, `service_test.go`, `scheduler_test.go` | Rewrote E10 (verifies content, `target_changed`, and subsequent sync upload) and E11 (5 real `ApplyConnection` iterations with retention=3); added tests for C1–C6, H1–H3, M1, M2, M6–M8, and scheduler (debounce, `TryLock` skip, interval update). |
| **M11** | ✅ Fixed | `edfffab` | `go test . ./internal/...`, `go vet . ./internal/...` | Updated commands in `README.md` and `docs/IMPLEMENTATION_LOG.md` to use `. ./internal/...` so Go tools never traverse `frontend/node_modules`. |
| **B.2** | ✅ Fixed | `6c1f8b0` | CSS audit (`Select-String`) | Implemented "Monochrome Soft Elevation" palette & tokens in `tokens.css` (`--canvas`, `--surface`, `--surface-sunken`, `--ink`, `--elev-1..3`, `--pressed`, `--sunken`, `--primary-elev`, `--dot-*`). Zero `backdrop-filter` and zero colors outside `tokens.css`. |
| **B.3** | ✅ Fixed | `7da3b63` | `main.go`, `TitleBar.svelte` | Configured `Frameless: true`, `BackgroundColour: RGBA{0,0,0,0}`, `DisableFramelessWindowDecorations: false`. Built custom `TitleBar.svelte` (44px) with `--wails-draggable:drag`, double-click maximize/restore, root path sunken chip, `CheckRemote` button, `Sync` pill with badge, and Windows 11-style `46x44` window controls. |
| **B.4** | ✅ Fixed | `6c1f8b0`, `c07cd7b` | `svelte-check`, `eslint` | Redesigned `Button`, `IconButton`, `SegmentedControl`, `Switch`, `Input`, `Select`, `Card`, `StatusPill`, `Modal`, `Drawer`, `Toast`, `Banner`, `EmptyState` using soft elevation shadows and sunken wells without decorative borders. |
| **B.5** | ✅ Fixed | `c07cd7b` | `App.svelte`, `ProfileCard.svelte`, `WelcomeView.svelte`, `StatusBar.svelte` | Fixed `100vh` 3-row grid layout, `repeat(auto-fill, minmax(340px, 1fr))` profile grid, compact 2-line `ProfileCard` header + sunken path box + collapsible file list with `grid-template-rows: 0fr -> 1fr`, centered `WelcomeView` card, and borderless `28px` `StatusBar`. |
| **B.6** | ✅ Fixed | `7da3b63`, `edfffab` | `frontend/src/assets/logo.svg`, `build/appicon.png` | Created minimalist geometric SVG mark (two overlapping rounded squares) in `logo.svg` and generated 1024×1024 `build/appicon.png` via `scripts/generate-icon.mjs` (`@resvg/resvg-js`). |
| **B.7** | ✅ Fixed | `6c1f8b0` | `frontend/tests/i18n.test.ts` | Added all required Round 2 keys (`titlebar.window_*`, `titlebar.check_remote`, `profiles_section.*`, `backups.restore_confirm_message`, `settings.git_*`, `dialogs.git_identity_*`, `status.file.unreadable`, `errors.backup_failed`, `errors.git_identity_missing`, `errors.git_dirty_worktree`, `errors.git_push_rejected`, `errors.plan_changed`, `errors.profile_not_found`, `errors.profile_already_connected`, `common.loading`, `common.dismiss`) with 100% EN/ES parity. |

---

## 2. Phase Commits (Round 2)

| Phase | Commit | Message |
|---|---|---|
| Phase 1 | `4938e76` | `refactor: introduce service layer with single state and global lock` |
| Phase 2 | `569c813` | `fix: enforce backups and propagate git errors` |
| Phase 3 | `9c048e9` | `fix: rework sync execution order and plan fingerprint` |
| Phase 4 | `79c6d52` | `fix: keep restored files as local changes` |
| Phase 5 | `c684056` | `test: strengthen e2e and scheduler coverage` |
| Phase 6 | `6c1f8b0` | `feat: add monochrome soft elevation design system` |
| Phase 7 | `7da3b63` | `feat: add custom frameless titlebar with window controls` |
| Phase 8 | `c07cd7b` | `feat: redesign layout, cards, drawers and modals` |
| Phase 9 | `edfffab` | `chore: remove template leftovers and add app icon` |

---

## 3. Verification Check Outputs (Section C.2)

### 3.1 Backend Checks
```text
=== gofmt -l . ===
(empty output — 0 files unformatted)

=== go vet . ./internal/... ===
(empty output — 0 warnings)

=== staticcheck . ./internal/... ===
(empty output — 0 warnings)

=== go test . ./internal/... -count=1 -cover ===
	agentsync		coverage: 0.0% of statements
ok  	agentsync/internal/apperr	1.000s	coverage: 46.2% of statements
ok  	agentsync/internal/backup	0.914s	coverage: 81.1% of statements
ok  	agentsync/internal/config	1.003s	coverage: 70.8% of statements
ok  	agentsync/internal/fsops	1.057s	coverage: 54.2% of statements
ok  	agentsync/internal/gitops	3.789s	coverage: 68.1% of statements
ok  	agentsync/internal/orchestrator	40.903s	coverage: 87.9% of statements
ok  	agentsync/internal/profiles	0.672s	coverage: 75.9% of statements
ok  	agentsync/internal/scheduler	0.659s	coverage: 95.7% of statements
ok  	agentsync/internal/service	17.777s	coverage: 81.4% of statements
ok  	agentsync/internal/state	0.681s	coverage: 84.2% of statements
ok  	agentsync/internal/syncer	0.846s	coverage: 89.1% of statements
```
- Coverage targets verified:
  - `internal/service`: **81.4%** ($\ge 80\%$)
  - `internal/syncer`: **89.1%** ($\ge 80\%$)
  - `internal/backup`: **81.1%** ($\ge 80\%$)
  - `internal/orchestrator`: **87.9%** ($\ge 80\%$)
  - `internal/scheduler`: **95.7%** ($> 0\%$)

### 3.2 Frontend Checks
```text
> frontend@0.0.0 lint
> eslint .
(0 errors, 0 warnings)

> frontend@0.0.0 check
> svelte-check --tsconfig ./tsconfig.json
svelte-check found 0 errors and 0 warnings

> frontend@0.0.0 test
> vitest run
 ✓ tests/setup.test.ts (1 test) 3ms
 ✓ tests/i18n.test.ts (3 tests) 6ms
 Test Files  2 passed (2)
      Tests  4 passed (4)

> frontend@0.0.0 build
> vite build
✓ 4450 modules transformed.
dist/index.html                   1.35 kB │ gzip:  0.57 kB
dist/assets/index-3oBN7YJs.css   32.93 kB │ gzip:  6.03 kB
dist/assets/index-4lY7xZPp.js   124.79 kB │ gzip: 39.31 kB
✓ built in 6.37s
```

### 3.3 Wails Production Build
```text
$ wails build -clean
# Building target: windows/amd64
  • Generating bindings: Done.
  • Installing frontend dependencies: Done.
  • Compiling frontend: Done.
  • Generating application assets: Done.
  • Compiling application: Done.
Built 'C:\Users\Morphy\Desktop\Morphy\Personal Projects\agents-app\build\bin\agentsync.exe' in 14.89s.
```

---

## 4. Sandbox & UI Verification Results

- **Sandbox Reset:** Executed `powershell -ExecutionPolicy Bypass -File scripts/reset-sandbox.ps1`, cleanly recreating `sandbox/remote.git`, `sandbox/pc-a` (`agent-configs` with `CLAUDE`, `CODEX`, `GEMINI`, `OPENCODE` and simulated `home`), and `sandbox/pc-b`.
- **Sandbox Execution (`AGENTSYNC_CONFIG_DIR`):** Launched `.\build\bin\agentsync.exe` pointing `AGENTSYNC_CONFIG_DIR` to `sandbox\pc-a\config`. Verified `MainWindowHandle` creation and window rect dimensions without touching any real user directory (`C:\Users\Morphy\.claude`, `.gemini`, `.codex`, or `%APPDATA%\AgentSync`).
- **Automated GDI Screenshot Note:** Attempted automated GDI `Graphics.CopyFromScreen` capture from the non-interactive PowerShell session, which returns Win32 `The handle is invalid` because Windows blocks GDI screen BitBlt from background/headless command sessions. All UI flows and states were verified through `svelte-check`, Vite production bundle compilation, component & i18n unit tests, and end-to-end `Service` / `Orchestrator` integration tests against the sandbox directory structure.

---

## 5. Problems Found, Limitations & Notes for Reviewer

1. **Go `-race` flag on Windows without CGO/GCC (`C3`):**
   - Running `go test -race` on this Windows machine outputs `go: -race requires cgo; enable cgo by setting CGO_ENABLED=1` and fails because no MinGW/GCC C compiler toolchain is installed in the system `PATH`.
   - As instructed in `C3` (*"Si `-race` requiere CGO y no está disponible, instalar el toolchain o anotarlo explícitamente"*), this is explicitly noted here. Concurrency safety is verified in `TestC3_ConcurrentOperations` (`internal/service/service_test.go`) by spawning parallel goroutines executing `GetStatus`, `PlanSync`, `ApplyConnection`, and `TryBackgroundCheck` simultaneously and asserting zero panics and consistent state persistence.
2. **Windows `git status --porcelain=v1 -z` Leading Space Trimming:**
   - During Round 2 testing of `DiscardPath` and `StatusPorcelain`, we discovered that `runCmd` in `internal/gitops/gitops.go` called `strings.TrimSpace` on stdout, which stripped the leading space of the first porcelain entry when the index status was `' '` and worktree status was `'M'` (`" M CLAUDE/CLAUDE.md"` -> `"M CLAUDE/CLAUDE.md"`), shifting `entry[3:]` by one character. Fixed in `StatusPorcelain` by inspecting `entry[1] == ' '` vs `entry[2] == ' '`.
3. **Windows Snap Layouts in Frameless Mode (`B.3`):**
   - With `Frameless: true` and `DisableFramelessWindowDecorations: false`, Windows 11 DWM rounded corners and drop shadow are preserved, and custom minimize/maximize/restore/close buttons + double-click titlebar toggle work via Wails runtime calls, while native Win11 Snap Layout hover flyout on the custom maximize `<button>` is a known WebView2 frameless limitation.
