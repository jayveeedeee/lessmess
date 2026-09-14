---
id: SPS-06
title: Pinned footer and flat full-width side-menu selections
---

# SPS-06: Pinned footer and flat full-width side-menu selections

Status: see [../ledger.md](../ledger.md).

## Objective

Per user review feedback: the settings footer must be stuck to the bottom of
the viewport, and the left-menu selections (active state and hover) must be
full-width with no rounded edges.

## Dependencies

SPS-05 (the page footer exists to pin).

## Scope

- `web/static/app.css` —
  - Settings becomes a full-height app page (board pattern):
    `body[data-page="settings"]` is a 100vh flex column with `overflow:
    hidden`; `main` and `.settings` fill it; `.settings-split` fills the
    middle; sidebar and content pane scroll internally; `.settings-foot`
    becomes a `flex: none` bar pinned flush to the viewport bottom (negative
    margins cancel main's padding).
  - `.settings-nav` buttons: drop `border-radius`, hover and active
    backgrounds cover the full button width; active state moves from the
    name-pill idiom to a full-width accent bar with white text.
  - Sidebar drops the now-inert sticky/max-height rules (no page scroll
    remains); mobile breakpoint restores normal page scroll, stacked layout,
    and normal footer flow.

## Implementation steps

1. Rework the settings CSS blocks per scope (layout, nav, footer, media
   query); no template or JS changes.
2. Rebuild, run tests, validate, restart the 9090 instance.

## Verification

- Build + vet + tests + `./lessmess validate` green.
- Served page unchanged markup-wise; browser check: footer visible at the
  viewport bottom regardless of section length; content pane scrolls under
  the pinned header/scope row; nav hover and active spans the full sidebar
  width with square corners.

## Completion criteria

- Footer pinned to the bottom on the settings page.
- Nav selections and hover full-width, no rounded edges.
- Mobile fallback intact; tests green; no JS/template changes.

## Files affected

- `web/static/app.css`

## Notes

- The sticky rules on the scope bar are kept: inert on the desktop settings
  page (no page scroll) but still meaningful under the mobile media query
  where page scrolling is restored.
