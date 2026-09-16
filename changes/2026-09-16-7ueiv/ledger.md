# Ledger — 2026-09-16-7ueiv

- Change ID: 2026-09-16-7ueiv
- Plan: [plan.md](plan.md)
- Branch: —
- Overall status: Done
- Last updated: 2026-09-16

## Status definitions

| Status | Meaning |
| --- | --- |
| Not started | Work has not begun. |
| In progress | Implementation or verification is actively underway. |
| Blocked | Work cannot continue until a documented dependency, decision, approval, or external condition is resolved. |
| Test | Implementation and verification are complete; awaiting user acceptance before Done. |
| Done | All verification and completion criteria in the task file have passed. |
| Cancelled | The task was intentionally removed from scope and the reason is recorded. |

## Tasks

Row order is display and priority order; top row is highest priority.

| Task | Title | Status | Depends on | Updated | Notes |
| --- | --- | --- | --- | --- | --- |
| [ACC-00](tasks/00-accent-palette-and-settings.md) | Accent palette, ui.accent setting, roll-once | Done | — | 2026-09-16 | Palette+schema+roll-once+validation landed. Evidence: `go vet ./...` + full `go test ./...` green; accent_test.go covers roll-once persistence, layer precedence, fail-open on unknown ids, personal-UI preservation, 422 on unknown accent (offline incl. setup shell), options payload, allowlist. API-drivable end to end. |
| [ACC-02](tasks/02-accent-css-cleanup.md) | Replace hardcoded accent derivatives | Done | — | 2026-09-16 | Pills (both themes) + card-expand now color-mix from --accent; xterm cursor reads computed style with legacy fallback. Evidence: grep sweep returns only theme definitions + JS fallback + static fallback SVG; served app.css confirmed after rebuild/restart. Palette-wide legibility = user visual pass. |
| [ACC-01](tasks/01-dynamic-brand-assets.md) | Dynamic brand assets and head accent | Done | ACC-00 | 2026-09-16 | brandassets.go (SVG/ICO/PNG gen) + routes + pageData accent style + setup-mux (no-roll). Evidence: unit tests green; live server rebuilt+restarted (PID 62522): /icon.svg serves rolled fuchsia #d946ef, /favicon.ico ICO magic + /apple-touch-icon.png 180×180 verified, ETag 304 flow works, PUT teal → icon+head style flip to #14b8a6 instantly, unknown accent 422, layout links dynamic routes. |
| [ACC-03](tasks/03-accent-settings-ui.md) | Accent swatch picker in Settings | Done | ACC-00, ACC-01 | 2026-09-16 | Swatch chips built from /api/settings/options (offline-safe), Auto chip + data-value container, per-section save/422 path, README documented. Evidence: served markup/JS verified; exact JS payload simulated live (null+accent section save round-trips, restore done); full suite green. Browser interaction = user visual pass. |

## Dependencies

- ACC-01 and ACC-03 depend on ACC-00. ACC-02 is independent. Recommended order: ACC-00 → ACC-01 → ACC-02/ACC-03, then one combined visual pass.

## Decision log

- 2026-09-16: Random pick semantics — roll once, lazily, on first resolution when `ui.accent` is unset in both layers; persisted to `.lessmess/settings.json` (personal layer); clearing both layers re-rolls. User-selected option.
- 2026-09-16: Raster favicons are generated at runtime in pure Go (circle-only rasters; "lm" stays SVG-only). User-selected option over pre-generated per-color files and SVG-only recoloring.
- 2026-09-16: The In-progress status pill follows the accent via `color-mix` rather than keeping a fixed orange identity. User-selected option.
- 2026-09-16: Palette lives only in Go (`internal/server/accent.go`); CSS gets values via an inline head style, the settings UI via `/api/settings/options` — no duplicated color lists.
- 2026-09-16: Brand icon uses the palette's dark-theme base value as the single identity color (no `prefers-color-scheme` media query inside the SVG): a favicon cannot observe the app's theme toggle, and one stable color per install is the goal. Rasters are letterless (no Go font dependency); apple-touch tile is a solid accent square (iOS composites transparency onto black).
