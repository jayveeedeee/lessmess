# MAC-18: Phase 4 management UI security

## Objective

Unify service-management controls into a coherent mobile surface and harden every sensitive boundary.

## Dependencies

- MAC-17

## Scope

- Navigation and responsive presentation for health, integrations, MCP, plugins, and permissions.
- CSRF/same-origin assumptions, redaction, no-store responses, confirmations, and audit-friendly logs.
- Relevant project settings only; no second generic config editor.

## Implementation steps

1. Integrate management sections with existing Settings patterns and source semantics.
2. Add consistent loading/error/unsupported states and return paths to Chat.
3. Review all new routes for method validation, same-origin behavior, secret handling, logging, and cache headers.
4. Add security regression tests for reflected secrets, hostile labels, forged mutations, and stale confirmations.

## Verification

- Handler/render tests, browser mobile checks, and a documented manual security review.

## Completion criteria

The management surface is usable at phone widths and no sensitive value crosses or persists beyond its intended boundary.

## Files affected

- Settings/management handlers and templates
- `web/static/`
- Security-focused tests

## Notes

LAN/VPN trust does not justify exposing OpenCode credentials to browser code.
