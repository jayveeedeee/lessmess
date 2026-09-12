---
id: NAV-00
title: Top menu with active route highlight
---

# NAV-00: Top menu with active route highlight

Status: see [../ledger.md](../ledger.md).

## Objective

Turn the header into a proper two-item menu — **Changes** (`/`) and
**Explorer** (`/explorer`) — always visible, with the active route highlighted
in accent orange (text + 2px underline).

## Dependencies

— (none)

## Scope

- `web/templates/layout.html` header restructure: brand | nav group | spacer |
  crumb | bell | theme toggle.
- `web/static/app.css` nav and `.active` styles.
- Server-side active class from `pageData.Page`; no client JS.

## Implementation steps

1. In `layout.html`, wrap the two routes in a `<nav class="topnav">` containing
   `<a href="/">Changes</a>` and `<a href="/explorer">Explorer</a>`; keep the
   brand to their left and the crumb, bell, and theme toggle to their right
   (spacer via `margin-left: auto` on the crumb or a dedicated flex spacer).
2. Render the active item with an `active` class using the existing
   `pageData.Page` value: `index` and `board` → Changes active; `explorer` →
   Explorer active. Use a small template helper or `{{if}}` comparisons —
   whichever matches how the renderer passes data today.
3. In `app.css`, style `.topnav a` as full-height header items (so the
   underline sits on the header's bottom border): muted text by default,
   `--text` on hover; `.topnav a.active` gets `color: var(--accent)` and a 2px
   `var(--accent)` bottom border (or `box-shadow inset` to avoid layout shift).
4. Remove or fold in the old `.nav-link` rule; keep header height at 52px and
   the crumb styling unchanged.

## Verification

1. `go build ./...` compiles; `go test ./...` passes (render tests reference
   the header).
2. Run the server and check `/`, a board page, and `/explorer`: both items
   always visible, exactly one underlined orange per page (Changes on index
   and board, Explorer on explorer).
3. Toggle light/dark theme: active state reads correctly in both.

## Completion criteria

- Both routes always rendered; active highlight correct on all three pages in
  both themes; no layout shift in the header; existing header IDs untouched.

## Files affected

- `web/templates/layout.html`
- `web/static/app.css`
- possibly `internal/server/render.go` (only if a template helper is needed)

## Notes

- User picked the orange underline style over filled-pill and soft-wash
  options.
