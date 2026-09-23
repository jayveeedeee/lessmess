# MAC-42: Descriptive choice rows for Chat forms

## Objective

Render single- and multiple-choice form answers with their OpenCode-provided
descriptions instead of hiding that context inside native selects.

## Dependencies

- MAC-41.

## Scope

- Render string options as radio rows and multiselect options as checkbox rows.
- Show each option's label and optional description.
- Preserve defaults, required validation, accessible grouping, and typed reply
  payloads.

## Implementation steps

1. Replace option selects with semantic fieldsets and labeled choices.
2. Update form serialization for radio and checkbox groups.
3. Add compact responsive styling and render/JavaScript contracts.

## Verification

- Focused/full tests, vet, JavaScript syntax, build, validation, and diff check.

## Completion criteria

Every answer option can display its description and selected values submit in
the same API shape as before.

## Files affected

- `web/templates/partials.html`
- `web/static/app.js`
- `web/static/app.css`
- `internal/server/chat_test.go`

## Notes

Free-text, numeric, boolean, and external fields keep their current controls.
