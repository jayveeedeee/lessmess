# MAC-48: Move context donut into composer toolbar

## Objective

Move the active-context donut from the Chat header into the composer toolbar.

## Dependencies

- MAC-47.

## Scope

- Place the donut immediately to the right of Work, Agents, Controls, Terminal,
  and their compact overflow representation.
- Keep it before the flexible gap and Send control.
- Preserve usage polling, warning state, tooltip, and accessible label.

## Implementation steps

1. Relocate the existing donut markup without changing its DOM ID.
2. Assert its position within the composer control sequence.

## Verification

- Focused/full tests, vet, JavaScript syntax, build, validation, and diff check.

## Completion criteria

The Chat header has no context donut; it appears below the text input directly
after the navigation controls.

## Files affected

- `web/templates/layout.html`
- `internal/server/render_test.go`

## Notes
