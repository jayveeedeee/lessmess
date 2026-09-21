
# WCV-03: Root-row reconciliation in status sync

Status: see [../ledger.md](../ledger.md).

## Objective

Close the derivation blind spot: a hand edit that sets a change ledger's overall status to the correct derived value but skips the root-ledger row currently leaves permanent rule-6 drift.

## Dependencies

—

## Scope

- `internal/store/overall.go` only.

## Implementation steps

1. In `syncOverall`, after the derived-vs-stored comparison, also read the root row: if the row's status differs from the change ledger's overall (whatever its value), rewrite both via `SetChangeStatus` with the change-ledger value.
2. Guard: skip when the root ledger fails to parse (rule 5 already reports it).
3. Test reproducing the tevr-core violation: change ledger `In progress` (correct), root row `Planned` → `SyncOverallStatuses` heals both; validate clean afterwards.

## Verification

- New store test green; full suite green.

## Completion criteria

Root-row-only drift self-heals on the next sync; rule 6 violations of this shape no longer persist.

## Files affected

- `internal/store/overall.go`, `internal/store/overall_test.go`

## Notes

Independent of the canon work; landed in this change because it is the other half of the 2026-09-18 tevr-core incident.
