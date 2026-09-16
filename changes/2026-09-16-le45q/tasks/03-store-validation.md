---
id: NTD-03
title: Store recursive validation and close-out readiness
---

# NTD-03: Store recursive validation and close-out readiness

Status: see [../ledger.md](../ledger.md).

## Objective

Enforce the recursive workflow rules in `Validate()` and expose the recursive close-out readiness check the server will gate change closure on.

## Dependencies

NTD-02

## Scope

- `internal/store/validate.go`: recursive rule extensions, container checks, readiness helper. No server changes.

## Implementation steps

1. Rule 2 (recursive): every container contains parseable `ledger.md` and a `tasks/` directory; report per path.
2. Rule 3 (recursive, per level): every child task file has exactly one row in its governing ledger and vice versa; frontmatter `id` == row ID; last dotted ID segment == filename sequence; container directory name == sibling task file name minus `.md`.
3. Stray-directory rule: a directory under any `tasks/` without a matching sibling task file is a violation (previously silently ignored).
4. Container ledger parse errors surface as rule-5-style violations with the nested file path.
5. `CloseOutReady(change)`: every non-cancelled task in the entire tree is `Test` or `Done`; expose the offending IDs for error messages.
6. Keep violation sorting stable (file, then rule).

## Verification

- `go test ./internal/store/` green: fixture trees exercising each violation (stray dir, name mismatch, missing ledger, broken dotted sequence, orphan row/file, deep parse error) and a clean three-level tree; `CloseOutReady` true/false cases including cancelled-exclusion and grandchild-blocks-close.
- `lessmess validate` clean on this repository (flat changes unaffected).

## Completion criteria

All nested-structure violations are reported with precise file paths; close-out readiness walks the full tree and matches the spec's semantics.

## Files affected

- `internal/store/validate.go`
- `internal/store/validate.go` tests (nested fixtures)

## Notes

Rules are the machine-checkable mirror of the AGENTS.md text landed in NTD-00 — keep the numbering and wording aligned.
