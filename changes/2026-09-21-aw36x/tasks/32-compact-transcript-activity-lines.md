# MAC-32: Compact transcript activity lines

## Objective

Reduce transcript chrome by rendering non-interactive activity as compact,
color-coded disclosure lines rather than stacked cards.

## Dependencies

- MAC-31.

## Scope

- Use one shared line treatment for reasoning, tools, diffs, shell operations,
  errors, and unknown message parts.
- Distinguish activity types with restrained semantic colors and labels.
- Keep detail content lazy/collapsible and readable when expanded.
- Retain card treatment for user messages and actionable permissions/forms.

## Implementation steps

1. Add semantic line classes to transcript markup.
2. Replace card borders/backgrounds/padding with a compact colored rule.
3. Convert shell output to a collapsed disclosure line.
4. Add render/style contracts and verify lazy details still work.

## Verification

- Focused Chat render tests, full Go tests/vet, JavaScript syntax, static build,
  and diff check.

## Completion criteria

Routine agent activity consumes roughly one text line while collapsed, remains
visually identifiable by type, and expands without losing existing details.

## Files affected

- `web/templates/partials.html`
- `web/static/app.css`
- `internal/server/chat_test.go`
- `internal/server/render_test.go`

## Notes

Permission and form interactions remain prominent because they require action.
