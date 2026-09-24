# MAC-65: Merge Chat send and stop controls

## Objective

Replace the separate written Send and Interrupt controls with one compact
Play/Stop action button that follows the current Chat interaction state.

## Dependencies

- MAC-64

## Scope

- Show a Play icon when an idle message can be sent.
- Replace Play with Stop while OpenCode is processing.
- While busy, switch back to Play when the focused composer contains text so a
  durable follow-up can be queued.
- Return to Stop when the busy composer is blurred or cleared.
- Preserve keyboard submission and accessible labels.

## Implementation steps

1. Replace Send/Interrupt markup with one stateful icon button.
2. Derive its state from busy, focus, and non-empty composer state.
3. Route Play through existing submit delivery and Stop through interrupt.
4. Update tests and compact styling.

## Verification

- Run focused Chat rendering/controller tests.
- Run full tests, vet, JavaScript syntax, validation, build, and served checks.

## Completion criteria

One button occupies the composer action position and switches deterministically
between accessible Play and Stop behavior without losing busy follow-up queueing.

## Files affected

- `web/templates/layout.html`
- `web/static/app.js`
- `web/static/app.css`
- `internal/server/render_test.go`

## Notes
