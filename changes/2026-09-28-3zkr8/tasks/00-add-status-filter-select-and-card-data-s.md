# CSF-00: Add status filter select and card data-status in template + CSS

## Why

The changes page has no way to filter the card list; this task lays the markup
and styling groundwork so the client logic (CSF-01) has hooks to work with.

## What

- In `web/templates/index.html`, add `<select id="change-filter-status">` in the
  controls row beside `#change-sort` with options: All (value `""`, default),
  Planned, In progress, Blocked, Done, Cancelled — the fixed five overall
  statuses from `internal/model/model.go:36-40`.
- Add `data-status="{{.Status}}"` to each `a.change-card` (the raw status
  string, e.g. `data-status="In progress"`), mirroring the existing
  `data-status-rank` attribute.
- In `web/static/app.css`, seat the second select in the same controls row
  (reuse/extend the `.change-sort` styling; keep the row intact at narrow
  viewports).

## Files affected

- `web/templates/index.html`
- `web/static/app.css`

## Verification

- Rebuild the binary (`CGO_ENABLED=0 go build -o lessmess ./cmd/lessmess`) and
  confirm the rendered index page contains the new select and that each card
  carries a `data-status` attribute matching its pill.
