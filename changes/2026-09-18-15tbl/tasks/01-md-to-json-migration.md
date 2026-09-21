
# JSI-01: md→JSON migration

Status: see [../ledger.md](../ledger.md).

## Objective

Implement the one-time, verified migration from the markdown workflow tree (root ledger, change ledgers, container ledgers, task frontmatter) into `.lessmess/workflow/` JSON, deleting the md ledgers only after round-trip verification.

## Dependencies

JSI-00

## Scope

- `lessmess migrate` CLI subcommand plus auto-run hook (called by `serve`/`validate` when md state is detected and `.lessmess/workflow/` is absent; refuses to run when JSON already exists).
- Read the existing tree with the current parsers: root ledger rows, per-change ledgers (including container ledgers for nested tasks), task files (frontmatter id/title, prose body), worktree-resolved changes.
- Emit `index.json` and `changes/<id>.json` per schema; keep prose md files in place, stripping frontmatter from task files; container directories keep their `tasks/` subdirs but lose `ledger.md`.
- Round-trip verification before deletion: render the md tables implied by the JSON and compare against the parsed source content (statuses, deps, order, titles, dates, notes, decisions); abort without deleting on any mismatch.
- Migration summary printed and logged (changes, tasks, containers, deletions, verification result).
- Update `.gitignore` with the `.lessmess/workflow/` negation as part of migration.

## Implementation steps

1. Extract reusable parse entry points from `internal/model` for the migration reader.
2. Implement the md→JSON collector (including archived changes under `changes/archive/` and legacy numeric IDs).
3. Implement round-trip verification.
4. Implement deletion of md ledgers + frontmatter stripping (atomic per file).
5. Wire the CLI subcommand (`lessmess migrate [--dry-run]`); the auto-run hook lands with the JSI-02 store cutover, where reading JSON state makes it meaningful.
6. Build fixtures from a snapshot of this repo's tree plus synthetic edge cases (empty change, empty container, archived, legacy IDs).

## Verification

Fixture tests migrate and verify cleanly; mutation tests (perturbed table) abort without deletion; dry-run on a copy of this repo's real `changes/` tree succeeds with a matching task count (compare against `lessmess validate` on the source tree).

## Completion criteria

Migration converts representative trees with verified round-trip, refuses unsafe runs, and is available as both CLI command and auto-hook.

## Files affected

- `cmd/lessmess/` (migrate subcommand), `internal/model/` (reader extraction), `internal/store/` (hook wiring point), `.gitignore` template/logic, fixtures

## Notes

- Implemented in `internal/store/migrate.go` (+ `migrate_test.go`): `MigrateWorkflow(opts)` with strict refusal (JSON already exists), full collection (root/change/container ledgers, frontmatter, decision logs, archived + legacy IDs, worktree resolver), per-level semantic round-trip verification inside `collectChange`, cross-checks in `verifyMigration`, then atomic writes → prose frontmatter stripping → ledger deletion → `.gitignore` patch. CLI: `lessmess migrate [--dry-run] [--dir]` in `cmd/lessmess/main.go`.
- Sequencing refinement: the serve/validate auto-run hook lands with JSI-02 (the store cutover), where JSON becomes the read path. Recorded in the plan.
- `.gitignore` patch: `.lessmess/` (or legacy `.tasktracker/`) becomes `.lessmess/*` + `!.lessmess/workflow/` (git cannot re-include under an ignored dir); idempotent, always newline-terminated.
- Empty-cell mapping: `—` → `""` on the way in. Worktree-backed changes resolve main-tree → archive → resolver; their state file is central, prose stays in the worktree.
- Crash window: JSON written before md deletion; a crash between leaves both, and re-runs refuse (remove `.lessmess/workflow/` to recover — md tree is still intact until deletion).
- Verification: `go vet ./... && go test ./...` green; fixtures cover dry-run, refusal, invalid trees (no frontmatter, row without file, stray dir, id mismatch), worktree change, gitignore variants; real-tree dry run on this repo: **41 changes, 197 tasks, 0 containers verified**.
