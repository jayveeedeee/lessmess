---
id: KAN-06
title: validate command and UI validation banner
---

# KAN-06: validate command and UI validation banner

Status: see [../ledger.md](../ledger.md).

## Objective

Wire the store's `Validate()` into the `tasktracker validate` CLI and surface persistent validation failures as a banner in the board UI.

## Dependencies

KAN-03.

## Scope

In scope: `tasktracker validate [--dir]` printing a human-readable report (OK or one line per violation with file context) and exiting 0/1; `serve` runs validation at startup and logs the result; board UI banner listing violations (polled via `GET /api/validate` and refreshed on SSE events).
Out of scope: auto-repair (validation reports only, by design).

## Implementation steps

1. Replace the KAN-00 stub: run store load + validate, print report, set exit code.
2. Startup validation log in `serve`.
3. UI banner partial fed by `/api/validate`.

## Verification

- Fixture repos with one deliberate violation per rule: CLI reports each and exits 1; clean repo (this repository) exits 0 with "OK".
- Banner appears on a deliberately broken fixture and disappears after fixing the file (via SSE refresh).

## Completion criteria

- CLI and banner behave as specified against clean and broken fixtures.

## Files affected

- `cmd/tasktracker/main.go`
- `internal/server/`, `web/templates/` (banner)

## Notes

- Keep CLI output stable and greppable (one violation per line, prefixed with file path).
