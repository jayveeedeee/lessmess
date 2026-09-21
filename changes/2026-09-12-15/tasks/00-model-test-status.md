
# TST-00: Add Test status to model and ledger template

Status: see [../ledger.md](../ledger.md).

## Objective

Introduce `Test` as a sixth task status in the model and have newly
scaffolded ledgers document it, so the board, parser, and move endpoint all
accept and render the new status.

## Dependencies

— (none)

## Scope

- `internal/model/model.go`: new `StatusTest TaskStatus = "Test"` constant;
  insert it into `TaskStatusOrder` immediately before `StatusDone`
  (order: Not started, In progress, Blocked, Test, Done, Cancelled).
- `internal/model/templates.go`: add the `Test` row to the status-definitions
  table written into new change ledgers.
- Unit-level coverage that `Test` parses/validates/serializes (extend an
  existing model test rather than adding a new file, if one fits).

## Implementation steps

1. Add the constant next to the other `TaskStatus` constants and place it in
   `TaskStatusOrder` before `StatusDone`; confirm `Valid()` (which iterates
   `TaskStatusOrder`) accepts it with no further change.
2. In `templates.go`, add a row to the ledger status-definitions table:
   `| Test | Verification passed; awaiting user acceptance. |` between the
   `Blocked` and `Done` rows.
3. Extend a model test (e.g. parse/serialize round-trip or `MoveTask` usage)
   to exercise `StatusTest`.
4. Do not update `server_test.go`/`render_test.go` column expectations here —
   that is TST-03; expect those tests to fail until then (or update them in
   passing and note it in TST-03's notes).

## Verification

1. `go build ./...` compiles.
2. `go test ./internal/model` passes, including the new `Test` coverage.
3. Scaffolding a throwaway change (or running the scaffold test) produces a
   ledger whose status-definitions table contains the `Test` row.

## Completion criteria

- `TaskStatusOrder` has six entries with `Test` before `Done`; `Test` is a
  valid status everywhere the order drives validation and rendering; new
  ledgers carry the `Test` definitions row.

## Files affected

- `internal/model/model.go`
- `internal/model/templates.go`
- one existing test file under `internal/model/`

## Notes

- `TestChangeLedgerMoveTaskToTest` covers validity, column order (Test
  immediately before Done), and a MoveTask round-trip to `Test`;
  `TestTemplatesRoundTrip` asserts the scaffolded ledger defines `Test`.
- `go test ./internal/model` green 2026-09-13; binary rebuilt so
  `./lessmess validate` recognizes the new status.
