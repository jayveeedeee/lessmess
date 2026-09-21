# MAC-12: Phase 3 subagent navigation

## Objective

Make parent and child agent activity understandable and navigable from the mobile chat surface.

## Dependencies

- MAC-11

## Scope

- Parent/child session tree, live state, task binding, pending input, and quick switching.
- Return path to the owning change/task without losing drafts.

## Implementation steps

1. Extend existing reconciliation output with the session state needed by Chat.
2. Add a compact child-activity drawer and parent breadcrumb.
3. Preserve drafts and scroll position per session during switches.
4. Test deep nesting, dead children, unmapped children, refresh reconciliation, and task bindings.

## Verification

- Mapping/server tests and a real parent plus multiple subagents at phone width.

## Completion criteria

Users can identify active subagents, open the right conversation, respond, and return without context loss.

## Files affected

- `internal/server/mapping.go`
- Chat/session handlers and tests
- Chat templates/static assets

## Notes

Reuse authoritative `parentID` relationships; never infer hierarchy from titles.
