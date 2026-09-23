# MAC-49: Promote compact action in session controls

## Objective

Move manual context compaction near the top of Session controls.

## Dependencies

- MAC-48.

## Scope

- Place Compact context directly below active usage.
- Keep it above Agent, Model, Commands, Skills, and Conversation history.
- Preserve capability gating, busy state, pending progress, and usage details.

## Implementation steps

1. Relocate the existing compaction markup without changing its DOM hooks.
2. Add compact spacing and a render-order regression assertion.

## Verification

- Focused/full tests, vet, JavaScript syntax, build, validation, and diff check.

## Completion criteria

The manual compaction action is visible near the top of Controls without
scrolling through the lower conversation-history section.

## Files affected

- `web/templates/layout.html`
- `web/static/app.css`
- `internal/server/render_test.go`

## Notes
