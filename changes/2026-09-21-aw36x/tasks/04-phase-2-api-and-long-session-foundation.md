# MAC-04: Phase 2 API and long-session foundation

## Objective

Extend Chat's API contract and loading model for rich controls and long-lived sessions without making initial load or polling grow without bound.

## Dependencies

- MAC-03

## Scope

- Cursor/pagination support for message history and incremental refresh.
- Typed attachment, reference, model/agent, command, skill, usage, and retry fields needed by Phase 2.
- Backward-compatible unknown-part handling.

## Implementation steps

1. Pin the OpenCode operations and fields Phase 2 consumes with fake-service fixtures.
2. Add bounded initial history, load-older, and incremental refresh methods.
3. Extend normalized chat views without changing Phase 1 routes unnecessarily.
4. Add long-transcript, cursor, duplicate, and API-compatibility tests.

## Verification

- `go test ./internal/opencode ./internal/server`
- A large fixture loads in pages and converges without duplicates or full-history polling.

## Completion criteria

Later Phase 2 UI tasks have stable, bounded server APIs and long sessions remain responsive.

## Files affected

- `internal/opencode/`
- `internal/server/chat.go`
- Chat handler tests

## Notes

Do not generate the complete OpenAPI client; retain narrow adapters.
