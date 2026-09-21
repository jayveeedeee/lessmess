# MAC-16: Phase 4 integrations and credentials

## Objective

Support integration discovery and secure connection flows required to unblock mobile sessions.

## Dependencies

- MAC-15

## Scope

- Integration list/detail and key, OAuth, or command connection methods.
- Attempt progress, cancellation, completion, and redacted error handling.

## Implementation steps

1. Add explicit integration and connection-attempt adapters.
2. Build method-specific forms; never return stored secret values to the browser.
3. Handle OAuth handoff/return and command attempt progress on mobile.
4. Add confirmations for disconnect/replacement operations supported by the API.
5. Test secret redaction, expired attempts, cancellation, callback failure, and reconnect.

## Verification

- Security-focused handler tests and one live key plus one OAuth/command flow where available.

## Completion criteria

A user can discover and connect supported integrations from mobile without exposing credentials in HTML, JSON, logs, or history.

## Files affected

- `internal/opencode/`
- Integration handlers/templates/tests
- `web/static/`

## Notes

Use OpenCode's credential store; lessmess must not persist provider secrets.
