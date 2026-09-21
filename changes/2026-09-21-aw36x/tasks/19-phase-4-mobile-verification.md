# MAC-19: Phase 4 mobile verification

## Objective

Verify a mobile user can diagnose and resolve common OpenCode service blockers safely.

## Dependencies

- MAC-18

## Scope

- Full Phase 4 live flows, negative/security cases, accessibility, documentation, and feature matrix.

## Implementation steps

1. Exercise healthy/degraded service, integration connection, MCP state, plugin/provider failure, and saved permissions.
2. Inspect browser/network/server logs for credential leakage and caching mistakes.
3. Verify unsupported API capabilities degrade to documented read-only or unavailable states.
4. Run phone-width accessibility checks and update operations/security guidance.

## Verification

- `go vet ./... && go test ./...`
- Static build, `lessmess validate`, live mobile checks, and secret-redaction evidence.

## Completion criteria

The Phase 4 gate is met and the feature matrix names every intentionally unsupported management capability.

## Files affected

- `README.md`
- Feature matrix and security guidance
- Relevant tests and learnings

## Notes

Public hosting remains out of scope even after management controls exist.
