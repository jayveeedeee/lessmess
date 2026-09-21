
# SPS-00: Sticky top menu bar

Status: see [../ledger.md](../ledger.md).

## Objective

Make the global top menu bar (`header` in `layout.html`) stay visible while
scrolling on every page, while remaining hidden whenever the embedded terminal
overlay is open.

## Dependencies

None.

## Scope

- `web/static/app.css` — `header` rule: add `position: sticky; top: 0;` and a
  z-index of 20 (below the terminal overlay's 30 and the board detail panel).

## Implementation steps

1. Update the `header` rule in `app.css` (around line 82) with
   `position: sticky; top: 0; z-index: 20;`.
2. Confirm no stacking regressions: terminal overlay (z-30) and the board task
   detail panel still cover the header; `#banner` and page content scroll under
   it.
3. Spot-check each page after a rebuild: index (scroll discussions), explorer,
   settings, board (no page scroll — header already pinned by the 100vh flex),
   setup (centered wizard unaffected).

## Verification

- Rebuild: `CGO_ENABLED=0 go build -o lessmess ./cmd/lessmess`.
- `go vet ./... && go test ./...`.
- Manual: scroll each page and observe the header; open a session terminal from
  the board and confirm the overlay still fully covers the header.

## Completion criteria

- Header stays visible during scroll on all scrolling pages.
- Overlay/detail/banner layering unchanged.
- Build and tests pass.

## Files affected

- `web/static/app.css`

## Notes

- `position: sticky` (not `fixed`) preserves the board page's flex layout and the
  setup page's vertical centering without padding compensation.
- Evidence (2026-09-14): `go build` + `go vet` + `go test ./...` green; rebuilt
  binary on a spare port serves the sticky rule (served app.css line 86); z-order
  audit by inspection — header 20 < modals 25 < terminal overlay 30 < detail 40;
  board page unaffected (body never scrolls); interactive scroll confirmation is
  part of the user's SPS-03 visual acceptance.
