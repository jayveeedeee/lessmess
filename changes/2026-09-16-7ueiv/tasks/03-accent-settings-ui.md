---
id: ACC-03
title: Accent swatch picker in the Settings UI
---

# ACC-03: Accent swatch picker in the Settings UI

Status: see [../ledger.md](../ledger.md).

## Objective

Expose `ui.accent` in Settings → UI as a swatch picker with the standard layered semantics: Inherit fallback, per-layer source badges, per-section save, and offline validation feedback.

## Dependencies

- ACC-00 (setting, validation, and the `accents` options payload).
- ACC-01 (so a save is immediately visible in the brand/favicon — the natural acceptance check).

## Scope

- New field in `web/templates/settings.html` (`data-field="ui.accent"`, new `data-kind="accent"`).
- `initSettings` in `web/static/app.js`: render swatches from `/api/settings/options`, handle the new kind in render and save paths.
- Swatch styles in `web/static/app.css`.
- Keep all existing settings DOM hooks intact (per templates/AGENTS.md).

## Implementation steps

1. `settings.html` UI section: add the field below `ui.showArchived` — label "Accent color", swatch container as the `[data-input]` element, source badge, Change button, and field-help text ("Picked randomly on first run; clear in both layers to roll again").
2. `app.js` `initSettings`: build the swatch row from `options.accents` (fall back to rendering nothing but a hint if options are unavailable); a swatch button per color plus an explicit "Inherit" affordance (empty value). Selection is client state on the container (`data-value`), not a native input.
3. Extend the render path for `data-kind="accent"`: highlight the configured layer value or the Inherit swatch using the existing `fallbackFor` logic (default orange when nothing is set anywhere); update the source badge exactly like other fields (`default`/`project`/`personal`).
4. Extend the save path: the per-section PUT payload for `ui.accent` sends the selected id or `""` to clear; surface 422 (unknown id) through the existing `.settings-status` error display.
5. `app.css`: swatch styles — color chips (the hex as background via inline style from JS), selected ring using the existing focus/accent conventions, Inherit chip visually distinct, keyboard-accessible buttons (`:focus-visible` treated like other settings controls).

## Verification

- `go vet ./...` and `go test ./...` green; `render_test.go` settings assertions still pass (field addition must not break the scope-radio/layers assertions).
- Rebuild and restart; manual pass: pick a swatch at Personal → Save → page brand/favicon update on next load; switch scope to Project → Inherit badge reflects the personal override; clear at both layers → next load re-rolls (per plan semantics); badge text matches the layer that set the value.

## Completion criteria

- The picker lists exactly the Go palette, obeys layered inheritance, saves and clears correctly at both scopes, and shows validation errors inline; no other settings behavior regresses.

## Files affected

- `web/templates/settings.html`
- `web/static/app.js`
- `web/static/app.css`

## Notes

- The field must keep the `data-field`/`data-kind`/`[data-input]` contract or `initSettings` will skip it; the display-only `general` section stays field-free.
- Palette lives only in Go — do not hardcode the color list in JS; always read `/api/settings/options` (graceful degradation when unavailable).
