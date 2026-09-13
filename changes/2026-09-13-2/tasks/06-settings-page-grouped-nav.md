---
id: SET-06
title: Settings page grouped left navigation
---

# SET-06: Settings page grouped left navigation

Status: see [../ledger.md](../ledger.md).

## Objective

Replace the settings page's single long stack of sections with a left-hand
group menu (Session, Prompts, Git, UI, Docs) that shows one group at a
time, so the page stays usable as it grows.

## Dependencies

- SET-04 (settings page exists)

## Scope

- `settings.html`: a `.settings-body` split — left `.settings-nav` with one
  button per group, right `.settings-content` holding the existing
  sections unchanged (fields, save buttons, statuses all stay).
- `app.js` (`initSettings`): nav wiring — clicking a group button shows
  only its section and marks the button active; the URL hash tracks the
  active group (`#prompts`) via `history.replaceState` and is honored on
  load (default group: Session). Scope toggle, badges, datalists, and
  saves are unaffected (they operate on the whole DOM, hidden or not).
- `app.css`: split layout (fixed-width nav, sticky, content flex),
  active-button styling in the existing visual language, stacking on
  narrow viewports.
- Render test: nav markup present with all five groups.

## Implementation steps

1. Restructure `settings.html` (nav + content wrapper).
2. Add `showGroup` + hash sync in `initSettings`; call on load.
3. CSS for the split and active state.
4. Extend `TestSettingsPageHTML`; run `go test ./...` and
   `node --check web/static/app.js`.
5. Rebuild, restart the local server, verify the page live.

## Verification

- `TestSettingsPageHTML` covers nav markup; full suite green.
- Live: each group shows alone, active state follows clicks, `#hash`
  survives reload, saves still work from any group.

## Completion criteria

- No "wall of sections": exactly one group visible at a time; all prior
  page behavior intact.

## Files affected

- `web/templates/settings.html`
- `web/static/app.js`
- `web/static/app.css`
- `internal/server/render_test.go`

## Notes

- Sections stay in the DOM (toggled via `hidden`), so the existing
  render/save logic in `initSettings` needs no restructuring.
- Verified 2026-09-13: `TestSettingsPageHTML` extended (nav + all five
  groups), full `go test ./...` green, `node --check` clean; rebuilt and
  restarted the :9090 server — `/settings` serves the nav markup live.
  Interaction (one group visible, active state, `#hash` restore on reload)
  is plain DOM toggling over the verified markup; browser spot-check left
  for user acceptance.
