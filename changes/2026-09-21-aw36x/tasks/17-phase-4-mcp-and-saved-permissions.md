# MAC-17: Phase 4 MCP and saved permissions

## Objective

Expose the MCP and saved-permission controls needed to diagnose and unblock agent work remotely.

## Dependencies

- MAC-16

## Scope

- MCP list, status, connect/disconnect, authentication state, and resource visibility.
- Saved permission review and removal plus active permission overview.

## Implementation steps

1. Add narrow MCP and saved-permission client methods with capability checks.
2. Render MCP state and remedies; expose only supported safe mutations.
3. Show saved rules clearly and require confirmation before removal.
4. Link active requests back to their session Chat interaction.
5. Test unavailable/auth-required MCP, failed reconnect, rule removal, and redaction.

## Verification

- Fake-service contracts and live MCP connect/disconnect plus permission review where available.

## Completion criteria

Mobile users can identify MCP and permission blockers and perform supported remediation safely.

## Files affected

- `internal/opencode/`
- Management handlers/templates/tests
- `web/static/`

## Notes

Adding or editing arbitrary MCP configuration is out unless the V2 API provides a stable validated operation.
