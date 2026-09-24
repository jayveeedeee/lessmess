# MAC-68: Embed Chat actions inside the input field

## Objective

Combine the Chat textarea and its circular Options and Play/Stop controls into
one rounded input surface without reducing the initial writing area.

## Dependencies

- MAC-67

## Scope

- Place both 44px action buttons visually inside the composer field.
- Preserve the textarea's existing initial text height above the controls.
- Keep automatic textarea growth behavior.
- Match the field's 22px corner radius to the buttons' circular radius.
- Move focus treatment to the unified outer surface.

## Implementation steps

1. Style the existing compose row as the shared bordered field.
2. Make the textarea borderless and transparent within it.
3. Pad the embedded action row without changing button dimensions.
4. Add focused and responsive CSS contracts.

## Verification

- Run focused Chat composer rendering/CSS tests.
- Run full tests, vet, JavaScript syntax, validation, build, and served checks.

## Completion criteria

Text and both actions read as one rounded composer, the initial text space is
unchanged, and growth/submission/menu behavior remains intact.

## Files affected

- `web/static/app.css`
- `internal/server/render_test.go`

## Notes
