---
id: SET-02
title: Wire settings into server behavior
---

# SET-02: Wire settings into server behavior

Status: see [../ledger.md](../ledger.md).

## Objective

Make the server actually use effective settings: agent/model at every
session creation point, prompt addenda in every prompt builder, default
branch in change creation, and the gardener/archived gates.

## Dependencies

- SET-00 (settings store)
- SET-01 (`CreateSessionWith`)

## Scope

- `server.New` loads the settings holder (fail-open); `Server` gains the
  field. `SetOpencode` unchanged apart from passing an addendum provider.
- Session creation points switch to `CreateSessionWith` with the effective
  agent/model (nil/empty = service default):
  - `createDiscussionSession`, `createChangeSession` (mapping.go),
    `explorerChat`, `commitChange` (lifecycle.go), `commitAll`
    (gitcommit.go), gardener runner (docssession.go).
- Prompt addenda: builders stay pure; call sites append
  `"\n\n" + addendum` when non-empty (small shared helper). The gardener
  runner (no `Server` access) receives an addendum-provider closure from
  `SetOpencode` so it reads live settings.
- `store.CreateChange` gains a `branch` parameter (empty = `model.Empty`);
  both server call sites pass effective `git.defaultBranch`; store tests
  updated.
- `closeChange` skips `enqueueDocsRefresh` when
  `docs.autoGardenerOnClose` is false.
- `Server.index` filters root-ledger rows whose `Href` starts with
  `archive/` when `ui.showArchived` is false (both HTML and JSON paths).
- Tests for each behavior with a fixture store + fake opencode client.

## Implementation steps

1. Add the settings field to `Server`, load in `New`, and a small
   `effectiveSettings()` accessor.
2. Thread agent/model through the six creation points.
3. Append addenda at the six prompt call sites (builder signatures
   unchanged).
4. `CreateChange` signature + callers + store tests.
5. Gardener gate and archived filter; unit/handler tests for all of the
   above.

## Verification

- `go test ./...` passes, including new cases: addendum appended vs
  byte-identical base prompt, branch recorded in root row, gardener skipped
  when disabled, archived rows hidden when disabled, `CreateSessionWith`
  receives configured agent/model.

## Completion criteria

- Every behavior in the plan's settings table is applied server-side and
  covered by a test; no behavior changes when all settings are defaults
  (byte-identical prompts, `—` branch, all rows listed, gardener runs).

## Files affected

- `internal/server/server.go`
- `internal/server/changesession.go`, `mapping.go`, `explorer.go`,
  `lifecycle.go`, `gitcommit.go`, `docssession.go`
- `internal/store/store.go`, `internal/store/store_test.go`
- New/updated tests in `internal/server/`

## Notes

- Keep prompt builders' existing table tests passing unchanged — addendum
  logic lives at call sites, not inside builders.
- Follow the fail-open style: settings read errors never fail requests.
