
# SET-07: Settings nav in explorer selection style

Status: see [../ledger.md](../ledger.md).

## Objective

Restyle the settings page's left group menu to match the explorer's
item-selection look instead of the custom bordered-button style introduced
in SET-06.

## Dependencies

- SET-06

## Scope

- `settings.html`: nav buttons carry a `<span class="snav-name">` around
  the label so the selected state can pill the name (explorer pattern).
- `app.css`: replace the `.settings-nav button` rules with the explorer
  selection idiom — row `padding: 0.14rem 0.25rem`-scale, `border-radius:
  4px`, hover `background: var(--surface-2)`, and the active item's name
  filled `background: var(--accent); color: #fff; padding: 0 0.2rem;
  margin: 0 -0.2rem` with `ui-monospace` 0.88rem 600-weight names. No left
  border, no custom active background.

## Implementation steps

1. Template: wrap labels in `.snav-name` spans.
2. CSS: swap the SET-06 nav rules for the explorer idiom (keep the SET-06
   layout: column, sticky, stacking on narrow viewports).
3. Rebuild + restart; confirm the look matches `/explorer`.

## Verification

- `TestSettingsPageHTML` still passes (nav markup unchanged apart from the
  span); full suite green; live visual check against `/explorer`.

## Completion criteria

- The settings nav reads exactly like explorer item selection: hover row
  highlight, accent-filled name on the active item, no custom border
  styling.

## Files affected

- `web/templates/settings.html`
- `web/static/app.css`

## Notes

- The explorer idiom lives in `.xtree summary` / `.xdir-name` +
  `.selected` (app.css ~line 691); the settings nav is flat, so guide
  lines do not apply.
- Verified 2026-09-13: full suite green, server rebuilt + restarted; nav
  markup live on :9090 with the accent-pill active state on `.snav-name`
  (same rule as explorer `.xdir-name` selected).
