# Ledger — 2026-09-12-14

- Change ID: 2026-09-12-14
- Plan: [plan.md](plan.md)
- Branch: —
- Overall status: Done
- Last updated: 2026-09-12

## Status definitions

| Status | Meaning |
| --- | --- |
| Not started | Work has not begun. |
| In progress | Implementation or verification is actively underway. |
| Blocked | Work cannot continue until a documented dependency, decision, approval, or external condition is resolved. |
| Done | All verification and completion criteria in the task file have passed. |
| Cancelled | The task was intentionally removed from scope and the reason is recorded. |

## Tasks

Row order is display and priority order; top row is highest priority.

| Task | Title | Status | Depends on | Updated | Notes |
| --- | --- | --- | --- | --- | --- |
| [REN-00](tasks/00-module-path.md) | Module path rename and import sweep | Done | — | 2026-09-12 | go.mod → module lessmess; 32 files swept; `go build ./...` OK; remaining "tasktracker strings are tasktracker-meta syntax + REN-03 brand test. |
| [REN-01](tasks/01-cmd-and-binary.md) | Rename cmd/tasktracker to cmd/lessmess and rebuild binary | Done | REN-00 | 2026-09-12 | cmd/lessmess moved with doc pair; vet OK; lessmess binary builds and runs; old tasktracker binary removed; :9090 still serving (200). |
| [REN-02](tasks/02-state-dir.md) | State dir .tasktracker → .lessmess with startup auto-migration | Done | REN-00 | 2026-09-12 | store.MigrateStateDir (rename-only, no merge) wired into serve/validate/docs-seed and server.New; all state paths + .gitignore + init.go output → .lessmess; 3 migration tests; vet+test green. |
| [REN-03](tasks/03-brand-and-text.md) | Brand and user-facing text sweep | Done | REN-00 | 2026-09-12 | Brand/title/templates/static comments/usage/README/AGENTS.md workflow text + drift asset regenerated; prompts needed no app-name edits (only apiBase comment); render_test flips to lessmess; vet+test green. |
| [REN-04](tasks/04-verify-and-relaunch.md) | Full verification and relaunch on :9090 | Done | REN-01, REN-02, REN-03 | 2026-09-12 | vet+test green; fresh build; validate rc=0; old PID 48375 stopped; .tasktracker→.lessmess migrated (sessions byte-identical); lessmess serve live on :9090; session API returns this session live; brand in title+header. |
| [REN-05](tasks/05-app-icon-and-favicon.md) | App icon and favicon | Done | REN-01, REN-03 | 2026-09-12 | icon.svg + icon-512.png + favicon.ico (16/32/48) + apple-touch-icon.png; white "lm" on #e8641f (UI accent); links in layout head; icon also brands the header and terminal top bar (session-id chip removed); full suite green; server restarted, markup verified served. |

## Decision log

- 2026-09-12: Task-ID prefix for this change registered as `REN`.
- 2026-09-12: Tier 1+2 rename confirmed by user: markers (`tasktracker:begin/end`), `tt-` prefixes, and the repo folder all stay unchanged.
- 2026-09-12: IGN (2026-09-12-13) verified already `Done` in both ledgers — no open-change coordination needed; follow its anchored gitignore-pattern approach.
- 2026-09-12: `internal/server/render_test.go:48` asserts the rendered brand "tasktracker"; flips to "lessmess" with the REN-03 template change.
- 2026-09-12: Migration helper lives in `internal/store` (`MigrateStateDir`, rename-only, no merge/delete) because store is the lowest shared package (no import cycle with docs/server/cmd); wired into serve/validate/docs-seed dispatch and defensively into `server.New`.
- 2026-09-12: Agent prompts (discussion/change/gardener/explorer) contain no app-name mentions — only marker syntax, which stays; sole edit was the `apiBase` comment.
- 2026-09-12: Left unchanged by decision: `config_test.go` `.tasktracker` hidden-dir fixture (semantics, not brand); `docsqueue_test.go` self-contained fixture strings (`cmd/tasktracker`, `.tasktracker/docs-queue.json` in a temp tree — still pass); all STRUCTURE.md/AGENTS.md marker-section content (gardener reconciles at close).
- 2026-09-12: A manual `POST /docs/refresh` (rc=202) was fired during REN-04 verification; the gardener job reconciled rename-stale docs (incl. root AGENTS.md learnings) ahead of close-out, within marker confinement.
- 2026-09-12: Icon work added as REN-05 (continuation of the rebrand on the still-open change). Circle uses `#e8641f` (the UI `--accent`) for visual consistency; rasters generated with Pillow in an isolated temp-dir venv (no repo tooling added).
