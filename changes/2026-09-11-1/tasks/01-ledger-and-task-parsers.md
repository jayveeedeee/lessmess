
# KAN-01: Ledger and task-file parsers

Status: see [../ledger.md](../ledger.md).

## Objective

Implement `internal/model`: types and strict parsers for the root ledger, per-change ledgers, and task files, targeting the pinned schemas in `AGENTS.md`.

## Dependencies

KAN-00.

## Scope

In scope: types (`RootLedger`, `RootRow`, `ChangeLedger`, `TaskRow`, `TaskFile`), parsers for the root-ledger table, per-change ledger (header fields, task table, status definitions, decision log — preserving raw source for round-trip), and task-file frontmatter (`id`, `title`) via `yaml.v3`. Strict errors on schema deviation (unknown columns, bad status vocabulary, literal `|` in cells, missing required files).
Out of scope: writing/serializing (KAN-02), cross-file validation (KAN-03).

## Implementation steps

1. Define types and status enums (`Not started`, `In progress`, `Blocked`, `Done`, `Cancelled`; overall `Planned`, …, `Cancelled`).
2. Implement a markdown table extractor (header + separator + rows) with pinned-column matching.
3. Implement `ParseRootLedger`, `ParseChangeLedger` (keeps raw bytes + table region offsets), `ParseTaskFile` (frontmatter split + YAML).
4. Add fixtures: copies of this repo's real files plus malformed variants (renamed column, bad status, pipe in cell, missing frontmatter, ID/filename mismatch).

## Verification

- `go test ./internal/model` passes: valid fixtures parse to expected values; every malformed fixture returns a descriptive error.

## Completion criteria

- All three parsers handle the real repository files and reject each malformation class with a typed error.

## Files affected

- `internal/model/` (new)
- `internal/model/testdata/` (fixtures)

## Notes

- Parser must keep raw ledger content and the exact table region so KAN-02 can rewrite only the table.
