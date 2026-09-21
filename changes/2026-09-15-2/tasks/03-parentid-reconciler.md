
# PSB-03: parentID reconciler on session list

## Objective

Map subagent children that were never explicitly bound: when a change's session
list is served, discover unmapped sessions whose `parentID` points at one of the
change's bound sessions and add them.

## Dependencies

- PSB-01 (schema fields).
- PSB-00 informally (confirms `parentID` semantics; the mechanism works regardless).

## Scope

- `internal/server/mapping.go` (or a small `taskreconcile.go`) + tests; the change
  session-list handler only.

## Implementation steps

1. In the handler serving `GET /api/changes/{id}/sessions`, after gathering stored
   entries: if the opencode client is available, run one `ListSessions`.
2. Filter candidates: `ParentID` non-empty and equal to one of the change's stored
   session IDs; candidate not already mapped anywhere (`changeOf` miss).
3. Infer the task by parsing a `^([A-Z0-9]+-\d+):\s*` prefix from the live title;
   accept the task only if it exists in the change's ledger (else map taskless).
4. Append entries (`Task` and `Parent` set, `Title`/`Created` from the live
   session), persist once, and `slog.Info("reconciled task sessions", ...)` when
   at least one was added.
5. Fail-open: service errors leave the stored list served as today (log at debug).
6. Unit tests: new child mapped with task from title; child with unknown-task title
   maps taskless; already-mapped child untouched; service failure degrades cleanly;
   no duplicate appends across repeated calls.

## Verification

- `go test ./internal/server/` green; repeated board loads do not duplicate rows in
  `sessions.json`.

## Completion criteria

- Unbound children appear on the next session-list fetch, exactly once.

## Files affected

- `internal/server/mapping.go`, `internal/server/mapping_test.go` (and the list
  handler's file if kept separate).

## Notes

- The regex must accept this change's own prefix style (`PSB-03`) — task IDs are
  `PREFIX-\d\d`, so `^[A-Z0-9]{2,4}-\d{2}:` is the working shape; confirm against
  `TaskStatusOrder`-adjacent helpers if one exists.
