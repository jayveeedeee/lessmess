# MAC-46: Move Chat navigation into composer toolbar

## Objective

Move Chat navigation controls from the header into the action row below the
message input.

## Dependencies

- MAC-45.

## Scope

- Keep only identity, session state, usage, and Close in the Chat header.
- Place Work, Agents, Controls, Terminal, and compact overflow between the add
  control and Send.
- Open the compact overflow menu above the bottom toolbar.
- Preserve existing control behavior and accessibility relationships.

## Implementation steps

1. Relocate the existing controls without changing their IDs.
2. Adapt compact toolbar spacing and overflow positioning.
3. Add a render contract for control placement.

## Verification

- Focused/full tests, vet, JavaScript syntax, build, validation, and diff check.

## Completion criteria

Chat navigation no longer occupies the header and is available between `+` and
Send below the message input.

## Files affected

- `web/templates/layout.html`
- `web/static/app.css`
- `internal/server/render_test.go`

## Notes
