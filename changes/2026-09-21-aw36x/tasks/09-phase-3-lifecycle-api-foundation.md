# MAC-09: Phase 3 lifecycle API foundation

## Objective

Define safe lessmess adapters and UI state for advanced session lifecycle operations before exposing destructive controls.

## Dependencies

- MAC-08

## Scope

- Fork, revert stages, compaction, inbox/steering, export, and session update contracts.
- Confirmation metadata and consistent conflict/error handling.

## Implementation steps

1. Pin the required OpenCode V2 operations with request/response fixtures.
2. Add narrow client methods and handlers, separating read/preview from mutation.
3. Define confirmation requirements and mapping effects for destructive operations.
4. Test stale state, busy sessions, missing messages, conflicts, and unknown API variants.

## Verification

- `go test ./internal/opencode ./internal/server`
- No destructive handler succeeds without its required confirmation input.

## Completion criteria

Phase 3 UI tasks can use tested explicit routes with predictable safety and mapping semantics.

## Files affected

- `internal/opencode/`
- `internal/server/chat.go` or focused lifecycle handler
- Associated tests

## Notes

Do not expose a generic lifecycle pass-through endpoint.
