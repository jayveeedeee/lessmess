
# SPS-05: Page-wide settings footer for layer explanation

Status: see [../ledger.md](../ledger.md).

## Objective

Move the Project/Personal layer explanation out of the sidebar footer into a
footer at the bottom that spans the width of the settings page, per user
request during review.

## Dependencies

SPS-02 (the sidebar footer exists to move from).

## Scope

- `web/templates/settings.html` — drop `.settings-side-foot` from the sidebar
  (which then holds only the title and nav); add a `<footer class="settings-foot">`
  after `.settings-split` with the explanation paragraph.
- `web/static/app.css` — remove `.settings-side-foot` rules; add
  `.settings-foot` (top border separator, small muted text, full page width).

## Implementation steps

1. Move the paragraph to the new footer element below the split.
2. Swap the CSS rules; keep the sidebar's sticky/max-height behavior unchanged.
3. Rebuild, verify, restart the 9090 instance.

## Verification

- Build + vet + tests + `./lessmess validate` green.
- Served `/settings`: footer present after the split, sidebar footer gone, and
  the render-test strings "lessmess.json" / ".lessmess/settings.json" still in
  the page (now in the footer).

## Completion criteria

- Explanation text appears only in the page-wide bottom footer.
- No JS changes; tests green.

## Files affected

- `web/templates/settings.html`
- `web/static/app.css`

## Notes

- The test-asserted strings move location but stay in the rendered page, so
  `render_test.go` keeps passing unchanged.
