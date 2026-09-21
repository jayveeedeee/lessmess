
# SPS-04: General settings group for wizard re-entry

Status: see [../ledger.md](../ledger.md).

## Objective

Move the "Re-run the onboarding wizard" entry out of the settings sidebar footer
into a proper "General" group in the side menu, per user request during review.

## Dependencies

SPS-02 (the sidebar and its footer exist).

## Scope

- `web/templates/settings.html` —
  - Add a `General` button (first item) to `.settings-nav` with
    `data-group="general"`.
  - Add a `data-section="general"` section to `.settings-content` holding the
    wizard re-entry as a ghost-button action row; no fields, no `[data-save]`.
  - Remove the wizard paragraph from `.settings-side-foot` (the layer
    explanation stays).
- `web/static/app.css` — anchor fix for the ghost button
  (`display: inline-block`, `text-decoration: none`, `width: fit-content`).

## Implementation steps

1. Template edits per scope; keep `data-group`/`data-section` values aligned.
2. Confirm `initSettings` compatibility: `render()` skips fields without
   `data-field`; save binding only touches sections containing `[data-save]`;
   default landing stays `session`; `#general` hash works via the existing
   nav wiring.
3. Rebuild and verify.

## Verification

- `CGO_ENABLED=0 go build -o lessmess ./cmd/lessmess`, `go vet ./...`,
  `go test ./...`, `./lessmess validate`.
- Served `/settings` contains the General nav item and section, and the footer
  no longer contains the wizard link; `/settings#general` selects it (client
   behavior, confirmed by the unchanged `showGroup` wiring).
- Existing render-test assertions unaffected (none reference the wizard link).

## Completion criteria

- Wizard re-entry lives under the General menu point only.
- No JS changes; tests green; no new save payload surface (`general` never
  reaches `PUT /api/settings` because the section has no `[data-save]`).

## Files affected

- `web/templates/settings.html`
- `web/static/app.css`

## Notes

- `data-section` values double as `lessmess.json` section keys for sections
  with `[data-save]`; `general` is display-only with no fields, so no unknown
  section can ever be submitted.
