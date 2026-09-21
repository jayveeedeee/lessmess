# MAC-27: Full-width plan and task reading views with sticky mobile Contents

## Objective

Render plan and task documents as full-width compact reading views with a
sticky top Contents disclosure instead of a left TOC rail.

## Dependencies

- MAC-25.

## Scope

- Remove side-rail layout and reserved gutter at compact widths.
- Reuse current h2/h3 harvesting and scroll-spy data.
- Add an accessible sticky disclosure that closes after navigation.
- Account for sticky headers and the virtual keyboard when scrolling anchors.

## Implementation steps

1. Add compact full-width modal/reading rules.
2. Adapt `buildDetailTOC` to create a top disclosure on compact widths.
3. Preserve the desktop left rail unchanged.
4. Test task, plan, ledger, and review detail shapes.

## Verification

- Long plan/task documents at phone widths and with raised keyboard.
- Desktop TOC remains a left rail.

## Completion criteria

Compact documents have no content beside them and retain usable heading
navigation above the document.

## Files affected

- `web/static/app.css`
- `web/static/app.js`
- `internal/server/render_test.go`

## Notes

Keep the existing two-heading threshold for showing Contents.
