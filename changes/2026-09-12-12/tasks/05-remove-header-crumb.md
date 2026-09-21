
# NAV-05: Remove obsolete header crumb

Status: see [../ledger.md](../ledger.md).

## Objective

Drop the `.crumb` page-title text from the header — it is redundant now that
the top menu shows the active route.

## Dependencies

- NAV-00 (the menu that made it obsolete)

## Scope

- `web/templates/layout.html` — remove `<span class="crumb">{{.Title}}</span>`.
- `web/static/app.css` — remove the `.crumb` rule.
- `.Title` stays in `pageData`: it still feeds `<title>`.

## Implementation steps

1. Delete the crumb span from the header in `layout.html`.
2. Delete the `.crumb` CSS rule.
3. Rebuild, run tests, restart the `:9090` server, and screenshot-check the
   header on `/`, a board page, and `/explorer`.

## Verification

- Header shows brand, menu, bell, theme toggle only; no stray spacing gap
  (theme toggle keeps its `margin-left: auto`); `<title>` still set.

## Completion criteria

- Crumb gone from all pages; vet/test green; live pages confirmed.

## Files affected

- `web/templates/layout.html`
- `web/static/app.css`

## Notes

- Requested by the user after seeing NAV-00 live: the crumb duplicated the
  active menu item (index/explorer) and the board page's `<h2>` change ID.
- Verified 2026-09-12: no `crumb` reference left in rendered pages or code;
  `pageData.Title` still feeds `<title>`; vet/test/validate green; `:9090`
  rebuilt and restarted; screenshot of the board page shows the clean header
  (brand, menu, right-aligned icons only).
