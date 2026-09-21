
# JSI-06: AGENTS.md shrink, init rework, docs

Status: see [../ledger.md](../ledger.md).

## Objective

Finish the contract migration: shrink root `AGENTS.md` to a short pointer plus the existing learnings markers, rework `lessmess init` for the JSON model, delete the drift-pin asset, and update user-facing documentation.

## Dependencies

JSI-02, JSI-03, JSI-04

## Scope

- Root `AGENTS.md`: replace the ~18 KB workflow text above the markers with a ≤ ~30-line pointer (repo uses lessmess; state is tool-owned JSON under `.lessmess/workflow/` — never edit by hand; change work happens in board-spawned sessions; prose conventions for `plan.md`/task bodies). Marker-guarded learnings sections stay byte-for-byte.
- `internal/docs/assets/workflow_agents.md`: becomes the pointer text that `init` writes; delete the drift-pin test/refresh procedure.
- `lessmess init`: bootstrap the new skeleton — pointer `AGENTS.md`, `.gitignore` with the `.lessmess/workflow/` negation, empty `.lessmess/workflow/` (index with no changes), starter `opencode.json`/`agentsdocs.json` unchanged; merge-safety and idempotence preserved.
- `README.md`: document the JSON state model, migration behavior for existing repos (auto-migrate on next serve), the instruction-injection behavior, and the new endpoints.
- Leave stale root learnings (those describing md ledger mechanics) to normal close-time doc gardening; do not hand-edit marker-guarded content.
- Update `lessmess validate` docs/help text where it references markdown rules.

## Implementation steps

1. Write the pointer text (shared by root AGENTS.md and the embedded asset).
2. Rework `init` outputs and idempotence checks; update init tests.
3. Remove the drift-pin test and refresh comments.
4. Rewrite README sections; sweep CLI help strings.
5. Final sweep for any remaining references to ledger md files in docs/help.

## Verification

`go vet ./... && go test ./...` green; `lessmess init` on a scratch repo produces a bootable new-model repo that `serve` opens without migration; this repo's `AGENTS.md` learnings markers unchanged (byte-compare of marker sections).

## Completion criteria

Contract surface fully migrated: pointer AGENTS.md, new-model init, no drift-pin, README accurate; no stale md-ledger references in user-facing docs.

## Files affected

- `AGENTS.md`, `internal/docs/init.go`, `internal/docs/assets/workflow_agents.md`, `cmd/lessmess/` (init), `README.md`, related tests

## Notes

- Root `AGENTS.md` shrunk from ~300 to 30 lines: a pointer (tool-owned JSON state, API-only mutations, board-spawned sessions with injected instructions) above the untouched marker-guarded learnings (302 → learnings left for normal close-time gardening per plan). The embedded asset `internal/docs/assets/workflow_agents.md` is the same pointer; the drift-pin test now pins the pointer (kept deliberately).
- `lessmess init` rework had already landed with JSI-02 (`initWorkflow` writes `.lessmess/workflow/index.json`; gitignore block replaces the whole-dir ignore); verified on a scratch repo: all five artifacts created, gitignore carries `.lessmess/*` + `!.lessmess/workflow/`.
- Template copy: setup wizard bootstrap list and settings branch help updated (JSI-05 pass).
- README rewritten for the new model: JSON-state overview + layout diagram, `migrate` subcommand, board/nested-task sections, new "Instruction injection" section, safety section (migration deletion is the one destructive exception), dev layout.
- Verification: `go vet ./... && go test ./...` green; scratch `init` bootstraps a new-model repo; drift test passes against the pointer text.