# MAC-14: Phase 3 recovery verification

## Objective

Verify advanced lifecycle and multi-agent workflows recover cleanly under real mobile use.

## Dependencies

- MAC-13

## Scope

- Full Phase 3 regression, destructive-action safety, reconnect/race testing, docs, and feature matrix.

## Implementation steps

1. Exercise fork, revert, compact, inbox, steering, subagents, export, rename, unlink, and delete against a real service.
2. Repeat key flows across refresh, network loss, stale state, and double tap.
3. Verify mappings and worktree/task context after every lifecycle mutation.
4. Run accessibility/mobile checks and update supported/deferred documentation.

## Verification

- `go vet ./... && go test ./...`
- Static build, `lessmess validate`, and recorded live recovery scenarios.

## Completion criteria

The Phase 3 gate is met with no lifecycle action capable of silently losing context or corrupting mappings.

## Files affected

- `README.md`
- Feature matrix
- Relevant tests and learnings

## Notes

Treat any ambiguous destructive outcome as a blocker, not a documentation caveat.
