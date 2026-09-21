# MAC-15: Phase 4 service health foundation

## Objective

Provide a small, safe service-management foundation so mobile users can diagnose why sessions cannot run.

## Dependencies

- MAC-14

## Scope

- OpenCode server identity/readiness, provider/model health, plugin status, and actionable errors.
- Feature-capability detection for evolving experimental APIs.

## Implementation steps

1. Add read-only client adapters for service, provider, model, and plugin status.
2. Normalize health findings into user-actionable states without leaking secrets.
3. Add a mobile service-status view linked from Chat failures and Settings.
4. Test partial outage, unsupported endpoints, stale service, and redaction.

## Verification

- Focused client/server/render tests and live healthy plus degraded-service checks.

## Completion criteria

Users can distinguish service, provider, model, plugin, and network failures from the phone UI.

## Files affected

- `internal/opencode/`
- Service-management handlers/templates/tests
- `web/static/`

## Notes

This is diagnosis, not a generic OpenCode administration console.
