---
id: CID-00
title: Update the workflow spec text
---

# CID-00: Update the workflow spec text

Status: see [../ledger.md](../ledger.md).

## Objective

Make the canonical workflow text describe the new change-ID scheme so the
code changes that follow implement an already-documented spec.

## Dependencies

None — this is the first task.

## Scope

The naming rules and the machine-validation item in `AGENTS.md`, plus the
byte-pinned embedded copy of that text in
`internal/docs/assets/workflow_agents.md`. No Go code changes.

## Implementation steps

1. In `AGENTS.md`, rewrite the "Change directory naming" rules: the ID is
   `YYYY-MM-DD-xxxxx` where the suffix is exactly five lowercase
   alphanumeric characters (`[a-z0-9]`) generated randomly; uniqueness comes
   from random generation with a collision check against existing
   directories (including `archive/`), not from allocation order. Delete the
   zero-based numbering, per-date allocation, inspect-before-allocating, and
   gap/no-reuse rules.
2. Keep the date-prefix rule (current local date, `YYYY-MM-DD` format) and
   the one-directory-per-change rule unchanged.
3. State explicitly that legacy `YYYY-MM-DD-N` directories (numeric
   suffixes of any length) remain valid and are not renamed.
4. Update the Validation section item 1 to describe the accepted formats:
   `changes/YYYY-MM-DD-(N|xxxxx)/` with a valid date, where `N` is the legacy
   numeric form and `xxxxx` is the new five-character lowercase
   alphanumeric form.
5. Regenerate the embedded asset so the drift test passes:
   `awk '/^<!-- tasktracker:begin/{exit} {print}' AGENTS.md > internal/docs/assets/workflow_agents.md`.
6. Grep the workflow text for leftover mentions of `ChangeNumber`,
   `YYYY-MM-DD-N` as the only format, "zero-based", and the gap rule; fix
   stragglers.

## Verification

- `go test ./internal/docs/...` — the workflow drift test passes.
- `grep -nE 'ChangeNumber|zero-based' AGENTS.md` returns nothing.
- The regenerated asset and `AGENTS.md` text above the markers are identical.

## Completion criteria

- The workflow text specifies exactly the scheme in [../plan.md](../plan.md).
- Embedded asset regenerated; no drift-test failure.

## Files affected

- `AGENTS.md`
- `internal/docs/assets/workflow_agents.md`

## Notes

- The asset is regenerated wholesale from `AGENTS.md`; never hand-edit it.
- Existing change directories in this repo keep validating after this task
  because validation still accepts the legacy format (widened in CID-01,
  text-only until then).
