
# REN-00: Module path rename and import sweep

Status: see [../ledger.md](../ledger.md).

## Objective

Change the Go module identity from `tasktracker` to `lessmess` so every package imports `lessmess/internal/…`.

## Dependencies

— (none; first task)

## Scope

- `go.mod`: `module tasktracker` → `module lessmess`.
- All 32 Go files importing `tasktracker/internal/…` (production and test files alike).
- No other text changes in this task.

## Implementation steps

1. Edit `go.mod` module line to `module lessmess`.
2. Mechanical sweep: replace `"tasktracker/internal/` with `"lessmess/internal/` across all Go files (verify count goes 32 → 0).
3. Confirm no other `tasktracker/` import-path forms exist (e.g. bare `"tasktracker"` imports — none expected).

## Verification

- `grep -rn '"tasktracker/' --include='*.go' .` returns nothing.
- `go build ./...` succeeds.

## Completion criteria

- Module path is `lessmess`; all imports rewritten; whole module builds.

## Files affected

- `go.mod`; every `.go` file under `cmd/` and `internal/` that imports internal packages (32 files).

## Notes

- Keep the sweep limited to import strings; brand/prose edits belong to REN-03.
- Verified 2026-09-12: go.mod declares `module lessmess`; 32 files swept; `grep '"tasktracker/'` returns nothing; `go build ./...` OK. Remaining `"tasktracker` strings are `tasktracker-meta` syntax and the REN-03 brand test assertion.
