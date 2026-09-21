
# CHW-01: Create changes/ scaffolding and tooling ignore file

Status: see [../ledger.md](../ledger.md).

## Objective

Create the repository artifacts the amended workflow requires: the root ledger `changes/ledger.md` (with a row for this change) and a `.gitignore` entry for tooling state.

## Dependencies

- CHW-00 (the root ledger schema and tooling-state rules must be defined in `AGENTS.md` first).

## Scope

In scope: `changes/ledger.md`, `.gitignore`.
Out of scope: creating `changes/archive/` (created on first use; empty directories are not tracked by git).

## Implementation steps

1. Create `changes/ledger.md` using the pinned root schema, with the required header statement and a row for change `2026-09-11-0`.
2. Create `.gitignore` containing `.tasktracker/`.

## Verification

- Read `changes/ledger.md`; confirm header statement, pinned columns, and the row for `2026-09-11-0` linking to its `plan.md`.
- Read `.gitignore`; confirm it contains `.tasktracker/`.

## Completion criteria

- Both files exist and match the formats specified in `AGENTS.md`.

## Files affected

- `changes/ledger.md`
- `.gitignore`

## Notes

- Bootstrap chicken-and-egg: the root ledger did not exist when this change was created (it is defined by this change), so the row records the status current at file creation rather than `Planned`. Recorded as a known, accepted deviation.
