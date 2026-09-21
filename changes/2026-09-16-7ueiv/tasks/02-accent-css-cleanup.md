
# ACC-02: Replace hardcoded accent derivatives in CSS and JS

Status: see [../ledger.md](../ledger.md).

## Objective

Remove the three remaining hardcoded accent colors so the entire UI — including the In-progress status pill and the terminal cursor — follows `--accent` no matter which palette entry is active.

## Dependencies

- None; independent of ACC-00/ACC-01 (works against the existing `--accent` variables). Landing it before or alongside ACC-03 makes the visual pass coherent.

## Scope

- `app.css` `.card-expand:hover` hardcoded rgba.
- `app.css` `--st-in-progress-bg` / `--st-in-progress-fg` in both theme blocks → `color-mix` derivatives of `--accent`.
- `app.js` xterm cursor color → computed `--accent`.
- No new features; no markup changes.

## Implementation steps

1. `app.css:1295`: `.card-expand:hover` background → `color-mix(in srgb, var(--accent) 12%, transparent)`.
2. `app.css` dark block: `--st-in-progress-bg: color-mix(in srgb, var(--accent) 16%, transparent)`; `--st-in-progress-fg: color-mix(in srgb, var(--accent) 75%, white)` (approximates the current lighter-than-accent `#f48a50`).
3. `app.css` light block: `--st-in-progress-bg: color-mix(in srgb, var(--accent) 12%, transparent)`; `--st-in-progress-fg: color-mix(in srgb, var(--accent) 80%, black)` (approximates the current darker-than-accent `#b8480b`).
4. `app.js` terminal theme: replace `cursor: "#e8641f"` with the computed style value `getComputedStyle(document.documentElement).getPropertyValue("--accent").trim() || "#e8641f"`, evaluated where the xterm theme object is built.
5. Sweep for any other accent-derived literals missed by the audit (grep for the dark/light hexes and their rgba forms) and convert the same way.

## Verification

- `go vet ./...` and `go test ./...` green (no Go behavior change expected; guards against accidental breakage).
- Rebuild and restart; visually verify in both themes with the default orange AND at least one contrasting palette entry (set `ui.accent` via API): In-progress pill legibility, card-expand hover, terminal cursor, focus rings.

## Completion criteria

- `grep -rn "e8641f\|f47a3a\|d5550f\|b8480b\|232, 100, 31\|213, 85, 15\|f48a50" web/static` returns only the intentional fallbacks (theme definitions and the JS fallback string); both themes stay legible with a non-orange accent.

## Files affected

- `web/static/app.css`
- `web/static/app.js`

## Notes

- `color-mix` is already used elsewhere in `app.css` (focus rings, badges), so no new browser-support surface.
- Ratios in steps 2–3 are starting points; fine-tune during the visual pass so the pill keeps its current perceived contrast with orange before calling it done.
