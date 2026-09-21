
# NTD-05: Sessions dotted binding, task prompt, auto-spawn

Status: see [../ledger.md](../ledger.md).

## Objective

Make sessions first-class at every tree node: dotted-ID binding and delegation, the task-scoped prime prompt, and best-effort once-only auto-spawn per container.

## Dependencies

NTD-04

## Scope

- `internal/server/mapping.go`, `internal/server/changesession.go`, new `.lessmess/autosession.json` state. No UI work.

## Implementation steps

1. Extend `taskTitleRe` to `^([A-Z][A-Z0-9]{1,3}-\d+(?:\.\d+)*):` and build `reconcileTaskSessions`' known-task set from the whole tree.
2. Add `taskPrompt(changeID, taskID)`: bound to one task; read the change plan, the governing ledger, and own task file first; task is decomposed — work/delegate children with dotted prefixes; keep the container ledger current; stop at `Test`, never set `Done`; never scaffold; propose (never perform) further decomposition. Append the standard prompt addenda via `promptWith`.
3. Auto-spawn reconciliation: after each store reload/write event, find container nodes with no session bound and no marker in `.lessmess/autosession.json`; spawn + prime + map (title `"<task-id>: <task title>"`), then set the marker. Bounded per batch; skip when the service is absent; log attempts and failures.
4. The expand endpoint (NTD-04) triggers the same spawn path directly and sets the marker, so UI and file-write paths behave identically.
5. `POST /changes/{id}/task-sessions` accepts dotted IDs; `listByTask` unchanged.
6. Per-task continue support: session listing/`enrich` already generic; expose the task-scoped session list for sub-board panels (root board keeps the full list).

## Verification

- `go test ./internal/server/` green with the fake opencode client: dotted delegation attaches to subtask cards; auto-spawn happens exactly once per container (marker), not on unlink, not on reload; service-down leaves the marker unset and logs; task prompt contains the governance wording and never appears on change sessions.
- Manual sanity: unlinking an auto-spawned session does not respawn on reload.

## Completion criteria

Every container gets exactly one task-scoped session, created best-effort at decomposition time regardless of who wrote the files; delegation works at any depth; no respawn storms.

## Files affected

- `internal/server/mapping.go`
- `internal/server/changesession.go`
- `internal/server/server.go` (reload hook wiring)
- `.lessmess/autosession.json` (new tooling state, gitignored)
- `internal/server/changesession_test.go`, `internal/server/mapping_test.go`

## Notes

Coordinate with in-flight 2026-09-15-2 (per-task subagent sessions): this change extends its regex and binding surfaces; land on top of its final shape, not around it.
