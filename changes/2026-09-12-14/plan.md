# 2026-09-12-14: Rename app to lessmess

- Change ID: 2026-09-12-14
- Created: 2026-09-12
- Branch: —
- Status: see [ledger.md](ledger.md)

## Objective and context

Rename the application from `tasktracker` to `lessmess` across the module path, command/binary name, tooling-state directory, and all user-facing brand and text. The repository folder itself (`/Users/jd/GitHub/tasktracker`) stays where it is; the doc-marker syntax (`tasktracker:begin/end`) and the `tt-` frontend prefixes stay unchanged by explicit user decision (Tier 1+2 rename only).

## Current behavior

- Go module is `tasktracker`; 32 Go files import `tasktracker/internal/…`.
- CLI entry is `cmd/tasktracker`; the documented build produces a root `tasktracker` binary (gitignored via anchored `/tasktracker`, per 2026-09-12-13).
- Tooling state lives in `.tasktracker/` (`sessions.json`, `docs-queue.json`, `docs-seed.json`), created by `init` and referenced by `internal/docs/seed.go` (`seedCursorPath`), `internal/server/docsqueue.go` (`docsQueuePath`), `internal/server/server.go`/`mapping.go` (sessions.json), and `cmd/tasktracker/main.go`.
- Brand "tasktracker" appears in `web/templates/layout.html` (`<title>` + brand link), `web/templates/explorer.html` guidance, `cmd/tasktracker/main.go` usage text, `web/static/app.js`/`app.css` header comments, `README.md`, and the canonical workflow text in root `AGENTS.md` (drift-pinned to `internal/docs/assets/workflow_agents.md`).
- Agent-facing prompts (`changePrompt`, `discussionPrompt`, `gardenerPrompt`, `explorerPrompt`) refer to the tool as tasktracker.

## Target behavior

- `go.mod` declares `module lessmess`; all imports use `lessmess/internal/…`.
- CLI entry is `cmd/lessmess`; build is `CGO_ENABLED=0 go build -o lessmess ./cmd/lessmess`; the stale root `tasktracker` binary is removed once the new build is verified.
- Tooling state lives in `.lessmess/`; on startup, if `.lessmess/` is absent and `.tasktracker/` exists, the directory is renamed (auto-migration preserving all state files).
- All user-facing text says `lessmess`: UI brand/title, README, usage text, agent prompts, workflow text in root `AGENTS.md` (with the drift asset regenerated).
- `go vet ./... && go test ./... && lessmess validate` all pass; the board serves on :9090 as `lessmess serve` with migrated state and the new brand visible.

## Scope

Tier 1 (code/identity): module path, `cmd/` directory name, binary name, `.gitignore` anchored patterns.
Tier 2 (state + text): `.tasktracker/` → `.lessmess/` with startup auto-migration; brand and prose in templates, static comments, README, root `AGENTS.md` workflow text + drift asset, agent prompts, code comments.

## Non-goals

- Doc-marker syntax: `<!-- tasktracker:begin/end -->` (incl. `DocMarkerBegin`/`DocMarkerEnd` in `internal/model/docfile.go`) stays — renaming markers would invalidate every STRUCTURE.md/AGENTS.md and testdata fixture.
- `tt-` prefixes stay: `.tt-*` temp files (`internal/model/atomic.go`, ignored in `internal/store/watch.go`), `tt-theme`, `tt-last-session:`, `tt:docs-event` in the frontend.
- Repository folder name and location.
- Content inside doc markers of covered folders' STRUCTURE.md/AGENTS.md (the doc gardener reconciles those at change close), `changes/` history, and `internal/model/testdata/` fixtures.
- Renaming the opencode integration (`opencode.json`, service discovery) — unrelated to the app name.

## Design decisions

- **Auto-migration over dual-read**: a single rename (`.tasktracker/` → `.lessmess/`) at startup keeps exactly one canonical state dir; no fallback reads, no split state. Migration is a no-op when `.lessmess/` exists or neither exists.
- **Migration hook placement**: run in `cmd` command dispatch (serve/validate/docs paths) before any state access, and defensively in `server.New`, so every entry point that touches state migrates first.
- **Anchored gitignore patterns**: follow 2026-09-12-13 (IGN)'s approach — `.lessmess/` for state, `/lessmess` for the root binary only, with the comment updated to `cmd/lessmess/`. IGN is already `Done` (verified in both ledgers), so no open-change coordination is needed despite the plan's initial assumption.
- **Brand test update**: `internal/server/render_test.go:48` asserts rendered HTML contains "tasktracker"; it flips to "lessmess" with the template change (same task).
- **Hidden-dir test fixture**: `internal/docs/config_test.go` uses `.tasktracker` as a hidden-dir fixture (semantics test, not brand) — left unchanged.
- **Old binary removal timing**: the root `tasktracker` binary is removed only after the `lessmess` build succeeds; the running :9090 process is unaffected (inode stays alive) and is replaced in REN-04.

## File-level impact

- `go.mod`; 32 Go files (import paths).
- `cmd/tasktracker/` → `cmd/lessmess/` (incl. its STRUCTURE.md/AGENTS.md doc pair, moved as-is).
- `internal/docs/init.go` + `init_test.go`, `internal/docs/seed.go`, `internal/server/docsqueue.go`, `internal/server/server.go`, `internal/server/mapping.go`, `cmd/lessmess/main.go` (state-dir paths); new migration helper + test.
- `.gitignore` (`.tasktracker/` → `.lessmess/`, `/tasktracker` → `/lessmess`).
- `web/templates/layout.html`, `web/templates/explorer.html`, `web/static/app.js`, `web/static/app.css`, `internal/server/render_test.go`.
- `README.md`, root `AGENTS.md` (workflow text only), `internal/docs/assets/workflow_agents.md` (regenerated via documented awk).
- `internal/server/changesession.go`, `internal/server/docssession.go`, `internal/server/explorer.go` (prompt strings/comments).

## Safety and rollback

- Everything is text edits plus one directory `git mv`-style move; rollback is reverting the edits. No data migration risk: the state-dir rename preserves file bytes and only fires when `.lessmess/` is absent.
- The running :9090 server keeps working throughout; it is only replaced at the final relaunch step after all checks pass.

## Testing and verification strategy

1. `go vet ./...` and `go test ./...` after each code-touching task.
2. Fresh `CGO_ENABLED=0 go build -o lessmess ./cmd/lessmess`; `./lessmess validate`.
3. Migration: with a fixture `.tasktracker/` containing state files, start the new binary and confirm `.lessmess/` appears with identical contents and `.tasktracker/` is gone.
4. Relaunch on :9090 as `lessmess serve`; confirm sessions survived (Sessions panel / Discussions list populated from migrated `sessions.json`) and the UI brand shows lessmess.

## Acceptance criteria

- No remaining `tasktracker` references outside the explicit non-goals (markers, `tt-` prefixes, `changes/` history, testdata, covered-folder marker sections).
- `go vet ./...`, `go test ./...`, `lessmess validate` all pass.
- Auto-migration verified with real state files; existing sessions survive the relaunch.
- Board serves on :9090 from the `lessmess` binary with the lessmess brand.

## Tasks

1. [REN-00](tasks/00-module-path.md) — Module path rename and import sweep
2. [REN-01](tasks/01-cmd-and-binary.md) — Rename cmd/tasktracker to cmd/lessmess and rebuild binary
3. [REN-02](tasks/02-state-dir.md) — State dir .tasktracker → .lessmess with startup auto-migration
4. [REN-03](tasks/03-brand-and-text.md) — Brand and user-facing text sweep
5. [REN-04](tasks/04-verify-and-relaunch.md) — Full verification and relaunch on :9090
