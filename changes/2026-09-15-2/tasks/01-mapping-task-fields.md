---
id: PSB-01
title: Mapping schema — task and parent fields
---

# PSB-01: Mapping schema — task and parent fields

Status: see [../ledger.md](../ledger.md).

## Objective

Extend the session mapping so an entry can record which task a session belongs to
and which session spawned it, without breaking existing mapping files — and expose
the service's `parentID` on the Go `Session` struct so the reconciler can use it.

## Dependencies

- None.

## Scope

- `internal/server/mapping.go`, `internal/server/mapping_test.go`, and the
  `internal/opencode` `Session` struct (+ client tests if they pin the shape).

## Implementation steps

1. Add optional fields to `SessionEntry`:
   `Task string \`json:"task,omitempty"\`` and
   `Parent string \`json:"parent,omitempty"\``.
2. Add `listByTask(change, task string) []SessionEntry` returning entries whose
   `Task` matches (empty `task` matches taskless entries under the change).
3. Add `ParentID string \`json:"parentID,omitempty"\`` to `opencode.Session` so
   `ListSessions`/`GetSession` carry the child relationship (verified present on
   the live service by PSB-00).
4. Confirm `changeOf` needs no change: a sub is appended under the same change key,
   so the existing 409-style guard naturally covers subs.
5. Tests: a pre-change-shaped `sessions.json` (no new fields) loads and round-trips
   without adding empty keys; entries with `task`/`parent` survive a
   load → list → save cycle; `listByTask` filtering is exact; client fakes that
   return `parentID` parse into the struct.

## Verification

- `go test ./internal/server/ -run TestMapping` (and the full package suite) green.

## Completion criteria

- Schema is additive; old files load unchanged; new accessors tested.

## Files affected

- `internal/server/mapping.go`, `internal/server/mapping_test.go`,
  `internal/opencode/client.go`.

## Notes

- Keep JSON tags `omitempty` so unbound sessions render exactly as before on disk.
