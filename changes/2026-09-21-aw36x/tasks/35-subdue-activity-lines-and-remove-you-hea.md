# MAC-35: Subdue activity lines and remove You headers

## Objective

Reduce transcript chrome by removing user labels and visually recessing routine
reasoning and command activity.

## Dependencies

- MAC-34.

## Scope

- Do not render a `You` heading above user messages.
- Tighten user-message padding while retaining its readable distinction.
- Make reasoning, tool, and shell lines smaller, thinner, and less saturated.
- Keep errors and diffs legible at their existing semantic emphasis.

## Implementation steps

1. Exclude user messages from transcript heading rendering.
2. Subdue routine activity colors, rule weight, and typography.
3. Add render contracts for the removed heading.

## Verification

- Focused and full Go tests, vet, JavaScript syntax, build, validation, and
  diff check.

## Completion criteria

User bubbles contain only their content, and routine reasoning/commands remain
identifiable without competing with conversation text.

## Files affected

- `web/templates/partials.html`
- `web/static/app.css`
- `internal/server/chat_test.go`

## Notes

System and compaction labels remain because they identify exceptional events.
