
# REN-02: State dir .tasktracker → .lessmess with startup auto-migration

Status: see [../ledger.md](../ledger.md).

## Objective

Move tooling state from `.tasktracker/` to `.lessmess/`, migrating existing repositories automatically on startup so sessions and docs queues survive the rename.

## Dependencies

- REN-00 (code edits land on top of the renamed module).

## Scope

- Path constants/references: `internal/docs/seed.go` (`seedCursorPath`), `internal/server/docsqueue.go` (`docsQueuePath`), `internal/server/server.go` + `internal/server/mapping.go` (sessions.json path and comment), `cmd/lessmess/main.go` (queue read).
- New auto-migration: if `.lessmess/` is absent and `.tasktracker/` exists, rename the directory; no-op otherwise. Hook it into command dispatch (serve/validate/docs paths, before any state access) and defensively into `server.New`.
- `.gitignore`: `.tasktracker/` → `.lessmess/`; `/tasktracker` → `/lessmess`; update the comment to reference `cmd/lessmess/` (keep IGN 2026-09-12-13's anchored-pattern approach).
- `internal/docs/init.go`: gitignore output and comments write `.lessmess/`; update `init_test.go` expectations.
- Tests covering state paths (`mapping_test.go`, `docsqueue_test.go`, `init_test.go`) and a new migration test.

## Implementation steps

1. Add `migrateStateDir(root string) error` (rename-only; never merge, never delete).
2. Call it early in `cmd/lessmess/main.go` for state-touching commands and in `server.New`.
3. Update all `.tasktracker` path constants and references to `.lessmess`.
4. Update `.gitignore` and `init.go`'s generated/merged gitignore content.
5. Update affected tests; add a migration test (fixture `.tasktracker/` with files → migrates; `.lessmess/` present → no-op; neither → no-op).
6. Decide per-fixture whether test data mentioning `.tasktracker` is brand (update) or hidden-dir semantics (leave); record decisions in Notes.

## Verification

- `go vet ./... && go test ./...` pass.
- Migration unit test covers all three cases.

## Completion criteria

- No live code path reads or writes `.tasktracker/`; migration renames an existing legacy dir on startup; gitignore and init output use the new names.

## Files affected

- `internal/docs/seed.go`, `internal/docs/init.go`, `internal/docs/init_test.go`
- `internal/server/docsqueue.go`, `internal/server/server.go`, `internal/server/mapping.go`, `internal/server/mapping_test.go`, `internal/server/docsqueue_test.go`
- `cmd/lessmess/main.go`; new migration helper file + test
- `.gitignore`

## Notes

- IGN (2026-09-12-13) is already `Done` per both ledgers — the coordination concern in the original plan brief is moot; just follow its anchored-pattern convention.
- `internal/docs/config_test.go` uses `.tasktracker` as a hidden-dir fixture (semantics, not brand) — left unchanged as decided.
- Do not hand-migrate the real repo's `.tasktracker/`; REN-04 verifies the auto-migration live.
- Verified 2026-09-12: `store.MigrateStateDir` added (rename-only; no-op when current exists or neither exists) with 3 passing tests; wired into serve/validate/docs-seed dispatch + `server.New`. All state paths, `.gitignore` (`.lessmess/`, `/lessmess`), and init.go output updated; `go vet ./... && go test ./...` green. Live migration verified in REN-04.
