---
id: NAV-01
title: Flat SVG bell icon
---

# NAV-01: Flat SVG bell icon

Status: see [../ledger.md](../ledger.md).

## Objective

Replace the glossy `🔔` emoji in the header with a clean, flat inline SVG bell
that inherits the theme color.

## Dependencies

— (none)

## Scope

- `web/templates/layout.html` `#notif-bell` contents.
- `web/static/app.css` bell sizing/centering.
- Badge, modal, and JS behavior unchanged.

## Implementation steps

1. In `layout.html`, replace the `🔔` character inside `#notif-bell` with a
   small inline SVG bell (stroke style, `fill="none"`,
   `stroke="currentColor"`, ~18px viewBox square, bell body + clapper path).
   Keep the `#notif-badge` span exactly as-is and keep the button's
   id/title/aria-label/hidden attributes.
2. In `app.css`, make `#notif-bell` a 32×32 grid-centered icon button matching
   `#theme-toggle` proportions; set the SVG to `display: block` with an
   explicit size; keep the badge's absolute positioning working off the
   button's `position: relative`.
3. Because adding `display: grid` overrides the UA `hidden` rule, add
   `#notif-bell[hidden] { display: none; }` so the bell still hides at zero
   findings.

## Verification

1. `go build ./...`; run the server.
2. With no docs findings: bell not visible. With findings (e.g. stale docs):
   flat bell + amber badge; click opens the notifications modal; red badge
   when an error finding exists.
3. Icon color follows the theme (muted → text on hover) in dark and light.

## Completion criteria

- Flat SVG bell renders identically in both themes; hidden/badge/modal
  behavior identical to before; no new static assets (embed list untouched).

## Files affected

- `web/templates/layout.html`
- `web/static/app.css`

## Notes

- Emoji rendering varies by platform; inline SVG with `currentColor` is the
  flat, deterministic option.
- Verified 2026-09-12: classic stroke-bell paths (body + clapper), 18px in a
  32×32 grid button; badge re-anchored to `top: 0; right: -4px`;
  `#notif-bell[hidden]` keeps zero-finding hiding intact (bell absent on
  current finding-free pages). Screenshot-verified against the live CSS in
  dark and light themes; vet/test green.
