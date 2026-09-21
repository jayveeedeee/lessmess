# MAC-38: Remove unsupported per-turn diff disclosure

## Objective

Remove the broken per-turn diff disclosure from user messages when the served
OpenCode API does not provide that resource.

## Dependencies

- MAC-37.

## Scope

- Stop attaching a diff URL to every user message unconditionally.
- Keep the bounded diff endpoint and renderer available for future capability
  wiring and direct contract coverage.
- Prevent `Review turn changes` from appearing in Chat.

## Implementation steps

1. Remove unconditional user-message diff URL construction.
2. Add a transcript regression contract for the absent disclosure.

## Verification

- Reproduce the upstream 404, then run focused/full tests, vet, JavaScript
  syntax, build, validation, and diff check.

## Completion criteria

User messages no longer offer a disclosure that fails with an OpenCode 404.

## Files affected

- `internal/server/chat.go`
- `internal/server/chat_test.go`

## Notes

The live service returned 404 for valid current-session user message IDs. A
future capability signal can re-enable this without restoring unconditional UI.
