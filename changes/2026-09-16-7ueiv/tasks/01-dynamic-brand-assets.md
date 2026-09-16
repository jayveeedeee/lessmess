---
id: ACC-01
title: Dynamic brand assets and page-head accent injection
---

# ACC-01: Dynamic brand assets and page-head accent injection

Status: see [../ledger.md](../ledger.md).

## Objective

Make the visible brand follow the accent everywhere it appears: the SVG favicon and header/terminal brand icon, the raster favicon fallbacks, and the page CSS variables via an inline head style — all driven by the resolved `ui.accent`.

## Dependencies

- ACC-00 (palette and resolution must exist).

## Scope

- `GET /icon.svg`, `GET /favicon.ico`, `GET /apple-touch-icon.png` handlers with content types, ETag (accent id) and 304 handling.
- Page-head inline `<style>` injection through `pageData`/renderer; `layout.html` switches its icon/brand links to the dynamic routes.
- Same routes on the setup mux (default accent, no roll — no store in setup mode).
- Handler and render tests. Static files under `web/static/` stay untouched.

## Implementation steps

1. Add SVG rendering to `accent.go` (or a sibling): the existing `icon.svg` markup with the circle fill substituted by the accent's dark-theme hex; keep the "lm" text unchanged. Optionally embed a `prefers-color-scheme: light` rule switching the fill to the light hex (supported by Chromium/Firefox tab icons; harmless elsewhere).
2. Add raster generation in pure Go: circle-only `image.NRGBA` at 16/32/48 and a 180×180 solid accent tile for `apple-touch-icon.png` (iOS renders transparency poorly); encode PNG via `image/png`; wrap the three PNGs in a minimal ICO container (6-byte ICONDIR + entries with embedded PNG data — a standard encoding). No fonts, no dependencies.
3. Register the three routes in `server.go` (`Content-Type: image/svg+xml`, `image/x-icon`, `image/png`; `ETag: "<accent-id>"`; `Cache-Control: no-cache`; 304 on `If-None-Match`).
4. Extend the renderer with an accent-style hook: `pageData` gains the rendered fragment (`template.HTML`) — a one-liner `<style>` setting `--accent`/`--accent-hover` on `:root` (dark values) and `[data-theme="light"]`; `render()` fills it centrally so no handler call sites change; wire the hook in both `server.New` and the setup path (default orange there).
5. Update `layout.html`: emit the fragment in `<head>`; point the favicon `<link>`, `alternate icon`, `apple-touch-icon`, header `.brand` img, and terminal-head img at the dynamic routes.
6. Mount the three icon routes on the setup mux via the shared setup route registration, serving the default accent without rolling.
7. Tests: content types and key bytes (SVG fill hex, ICO magic `00 00 01 00`, PNG magic + dimensions); ETag/304; render test asserting the inline style and dynamic links appear; setup-mode route serves default orange.

## Verification

- `go vet ./...` and `go test ./...` green.
- Rebuild (`CGO_ENABLED=0 go build -o lessmess ./cmd/lessmess`) and restart; load `/` in a browser: tab icon, header brand, and terminal-head icon all show the resolved accent; changing `ui.accent` via API and reloading updates them with no stale cache (304 flow verified in DevTools network pane).
- `/setup` on a fresh repo serves the orange default and does not create a personal accent entry.

## Completion criteria

- All brand surfaces follow the resolved accent on every page (including setup); assets cache correctly; tests pass.

## Files affected

- `internal/server/accent.go`, `internal/server/server.go`, `internal/server/setup.go`, `internal/server/render.go`
- `web/templates/layout.html`
- `internal/server/accent_test.go`, `internal/server/render_test.go`, `internal/server/server_test.go` (route coverage as needed)

## Notes

- `web/` assets are embedded at build time — a running server shows template changes only after rebuild + restart (root AGENTS.md learning).
- The static orange files stay in `web/static/` untouched as fallbacks; nothing links to them anymore once this task lands.
- Keep z-index/ladder and template-name rules untouched; this is a head-and-img change only.
