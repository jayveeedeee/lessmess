# 2026-09-16-7ueiv: Accent color palette

- Change ID: 2026-09-16-7ueiv
- Created: 2026-09-16
- Branch: —
- Status: see [ledger.md](ledger.md)

## Objective and context

lessmess has a single fixed visual identity: the orange accent `#e8641f` (light theme `#d5550f`), baked into `web/static/app.css`, `web/static/icon.svg`, and pre-generated raster icons (`favicon.ico`, `apple-touch-icon.png`, `icon-512.png`). The user wants an assortment of accent colors with three properties:

1. One is picked at random when the app is initialized, giving each install its own identity.
2. It can be changed in the Settings UI at either settings layer.
3. The browser favicon and app brand icon follow the chosen color, not just the page CSS.

An audit found the CSS is already variable-driven (`var(--accent)` / `--accent-hover` on all 35 usages), so the UI itself is trivially re-themeable. Three hardcoded accent derivatives need cleanup, and the favicon needs a dynamic-serving strategy because the rasters are static files.

## Current behavior

- `app.css` defines `--accent`/`--accent-hover` in `:root` (dark) and `[data-theme="light"]`; everything else consumes the variables.
- Hardcoded accent derivatives that do NOT follow the variables:
  - `app.css:1295` — `.card-expand:hover` background `rgba(232, 100, 31, 0.12)`.
  - `app.css:23-24, 55-56` — `--st-in-progress-bg`/`--st-in-progress-fg` are hand-tuned accent derivatives (dark: `rgba(232,100,31,.16)`/`#f48a50`; light: `rgba(213,85,15,.12)`/`#b8480b`).
  - `app.js:736` — xterm cursor `#e8641f`.
- `icon.svg` hardcodes `fill="#e8641f"`; it is linked as the SVG favicon and rendered as `<img>` in the header brand and terminal top bar (`layout.html`).
- `favicon.ico` / `apple-touch-icon.png` / `icon-512.png` are static, generated once with Pillow (change 2026-09-12-14). `icon-512.png` is referenced by no page.
- Settings (`internal/server/settings.go`) are layered: committed `lessmess.json` (project) over gitignored `.lessmess/settings.json` (personal) over built-in defaults; stateless reads, atomic per-layer writes, tri-state fields. `UISettings` currently holds only `ShowArchived`. `PUT /api/settings` validates agent/model against the live opencode service; `/api/settings/options` exposes service-discovered choices. `settingsFieldValue` (`settingschange.go`) is the dotted-path allowlist for the per-setting Change button.

## Target behavior

- A curated palette of ~10 named accent colors lives in Go as the single source of truth. Each entry carries an id, label, and hand-tuned dark + light `--accent`/`--accent-hover` values.
- On first need, if neither settings layer defines `ui.accent`, the server rolls a random palette entry and persists it to `.lessmess/settings.json` (personal layer). It then sticks; Settings can change it at either layer.
- The server injects the resolved palette values as a small inline `<style>` in the page head — no flash of the default color, no localStorage, palette defined once.
- `GET /icon.svg`, `GET /favicon.ico`, and `GET /apple-touch-icon.png` render brand assets with the effective accent injected. `layout.html` links these dynamic routes instead of the static files; the header brand and terminal icon follow the accent too.
- Settings → UI gains an `Accent` swatch picker (closed palette, not free text) with the standard Inherit/source-badge/per-layer save semantics.
- All hardcoded accent derivatives are replaced with `--accent`-derived values so the entire UI (status pills included) follows the palette.

## Scope

- Palette definition, `ui.accent` setting end to end (schema, merge, validation, options, Change-button allowlist, roll-once initialization).
- Dynamic brand-asset routes (SVG, ICO, PNG) in both the full server and the setup mux.
- Page-head accent style injection via `pageData`.
- CSS/JS cleanup of the three hardcoded accent derivatives.
- Settings UI swatch picker.
- `README.md` documentation of the new setting.

## Non-goals

- No arbitrary custom hex input — the assortment is closed (a custom-color escape hatch can be a future change).
- No re-rolling on every start; no theme scheduling.
- The static files under `web/static/` stay as-is (orange) — they remain as fallbacks for direct `/static/` access; no committed per-color raster set.
- The "lm" letterform is unchanged and stays SVG-only; raster fallbacks are letterless (no font dependency in Go).
- No changes to the dark/light theme toggle or defaults; xterm theming beyond the cursor color is untouched.

## Design decisions

- **Palette lives in Go, not CSS.** The server needs hex values anyway (inline style + icon rendering + settings options), so `internal/server/accent.go` is the single source; CSS carries no duplicate palette blocks.
- **Roll once, lazily, persisted to the personal layer.** Effective `ui.accent` is checked whenever the accent is resolved; unset in both layers → uniform random pick, atomically written to `.lessmess/settings.json`, cached per process (`sync.Once`). Rationale: a random identity is per-install, so it must not be committed project policy (`lessmess.json`); lazy rolling covers existing repos without touching bootstrap. Failure to persist logs a warning and still uses the rolled value for the session.
- **Semantic of clearing:** emptying `ui.accent` in both layers makes the next resolution roll a fresh random color (documented; chosen over a "has rolled" marker file to avoid extra state).
- **Invalid stored values fail open:** a hand-edited unknown id logs a warning and renders the default orange; the file is never silently rewritten.
- **Inline `<style>` from `pageData`, not `data-accent` CSS blocks.** Avoids duplicating the palette in app.css and keeps the palette extensible without CSS regen. Rendered before first paint → no flash.
- **Rasters are generated at runtime in pure Go** (`image/draw` circle; ICO as a hand-rolled container around embedded PNGs — a standard, widely supported encoding; no new dependencies, no font). `favicon.ico` = 16/32/48 circle; `apple-touch-icon.png` = 180×180 solid accent tile (iOS dislikes transparency); exact composition finalized in the visual pass. `icon.svg` keeps the "lm" text; optionally embeds a `prefers-color-scheme` media query so the tab icon adapts.
- **In-progress pill follows the accent** via `color-mix`, per user decision — the whole UI moves together.
- **Cache strategy for dynamic assets:** `ETag` derived from the accent id + `Cache-Control: no-cache`, so accent changes propagate immediately with cheap 304s.
- **Setup mode:** the setup mux serves the icon routes with the default accent and never rolls (no store yet); rolling starts once the real store exists.
- **Validation is offline and always enforced** (unlike agent/model, which skips when the service is down): `PUT` with an unknown accent id → 422.

## Detailed implementation approach

1. **`internal/server/accent.go` (new):** `AccentColor{ID, Label, Dark, DarkHover, Light, LightHover}`; `AccentPalette` slice (orange first = default); lookup helpers; `ResolveAccent(settings)` + roll-and-persist logic with `sync.Once` caching; SVG/ICO/PNG render helpers. Starting assortment (final values tuned in the visual pass; light hues chosen one step darker in the dark theme to keep white text legible): orange `#e8641f/#f47a3a` · `#d5550f/#b8480b` (current values, default), teal, green, blue, violet, pink, fuchsia, red, amber, cyan — each with dark and light pairs.
2. **`settings.go`:** add `Accent string` to `UISettings` and `EffectiveUISettings` (patch/merge/clear semantics mirror the existing plain-string fields such as `git.defaultBranch`).
3. **`settingsapi.go`:** PUT validation (422 unknown id); `/api/settings/options` gains `accents: [{id, label, hex}]` so the settings UI needs no duplicated palette; sources unaffected.
4. **`settingschange.go`:** `settingsFieldValue` allowlist gains `ui.accent`.
5. **`render.go` / `layout.html`:** renderer gains an accent-style hook wired from the server; `pageData` carries the rendered `<style>` fragment; layout emits it in `<head>`; icon/brand links switch to the dynamic routes. `setup.go` gets the same routes via the shared registration path (default accent).
6. **Routes in `server.go`:** `GET /icon.svg`, `/favicon.ico`, `/apple-touch-icon.png` with correct content types, ETag, and 304 handling.
7. **CSS/JS cleanup:** `app.css` `.card-expand:hover` → `color-mix(in srgb, var(--accent) 12%, transparent)`; `--st-in-progress-bg/fg` → `color-mix` derivatives in both theme blocks (dark fg lightens toward white, light fg darkens toward black — exact ratios in the visual pass); `app.js` xterm cursor reads `getComputedStyle(document.documentElement).getPropertyValue("--accent")` with the old hex as fallback.
8. **Settings UI:** `settings.html` adds the `ui.accent` field (`data-kind="accent"`, swatch container as `[data-input]`); `initSettings` builds swatches from `/api/settings/options`, handles Inherit/fallback highlighting, and includes the field in the per-section PUT payload; swatch styles in `app.css`.
9. **`README.md`:** document the Accent setting and the roll-once behavior.

## File-level impact

| Area | Files |
| --- | --- |
| New | `internal/server/accent.go`, `internal/server/accent_test.go` |
| Settings | `internal/server/settings.go`, `settingsapi.go`, `settingschange.go` (+ their tests) |
| Server/render | `internal/server/server.go`, `setup.go`, `render.go`, `render_test.go` |
| Web | `web/templates/layout.html`, `web/templates/settings.html`, `web/static/app.css`, `web/static/app.js` |
| Docs | `README.md` |

Note: `web/` assets are embedded at build time — rebuild (`CGO_ENABLED=0 go build -o lessmess ./cmd/lessmess`) and restart before verifying in the browser.

## Data, API, configuration, and schema changes

- `lessmess.json` / `.lessmess/settings.json` gain optional `ui.accent` (string, palette id). Additive and backward compatible; old binaries ignore it.
- `GET /api/settings/options` gains an `accents` array.
- `PUT /api/settings` can return 422 for unknown `ui.accent` values.
- New public routes: `/icon.svg`, `/favicon.ico`, `/apple-touch-icon.png`.
- New server state: none beyond the optional settings field (roll-once writes the standard personal settings file).

## Safety, security, rate limits, migration, rollback

- No migration: additive settings field, no changes to `changes/` data.
- Writes stay confined to the gitignored personal layer; atomic; never touch the committed project file.
- Icons are generated content from a fixed internal palette — no user-supplied bytes, no injection surface (SVG fill values come only from palette constants).
- Rollback = revert and rebuild; a stored `ui.accent` is simply ignored by the old binary. Worst-case artifact: a `ui.accent` line in `.lessmess/settings.json`.

## Testing and verification strategy

- Unit: palette lookup; roll-once (first resolve persists a valid id to the personal layer, second resolve returns the same id, existing project/personal values are respected, invalid stored value falls back to default without rewrite); PUT validation (422 unknown, known ok); allowlist; merge/clear semantics.
- Handler tests: `/icon.svg` content type + accent hex present; `/favicon.ico` ICO magic bytes; `/apple-touch-icon.png` PNG magic + dimensions; ETag/304.
- Render tests: layout emits the inline accent style and dynamic icon links.
- Full gate: `go vet ./... && go test ./...`; rebuild + restart; manual visual pass — switch through several palette entries in both dark and light themes, check status pills, buttons, focus rings, terminal cursor, header brand, and the browser tab favicon (and DevTools: no 404s, 304s after first load).

## Observability

- `slog` info line when the initial roll persists ("rolled initial accent color: <id>").
- `slog` warning when a stored accent id is unknown (fail-open to default).
- No new metrics/SSE surface; settings changes already surface through the standard settings-status UI.

## Rollout sequence

1. ACC-00 (palette + settings plumbing + roll-once) → 2. ACC-01 (dynamic assets + rendering) → 3. ACC-02 (CSS/JS cleanup, independent) and ACC-03 (settings UI) → manual visual pass → ready for review.

## Risks and mitigations

- **ICO-via-PNG on ancient browsers:** fallback favicon could fail where PNG-in-ICO is unsupported; modern browsers all support it, and the SVG icon is primary. Accepted.
- **Contrast per palette entry:** mitigated by hand-tuned dark/light pairs per entry, dark-theme values one step darker for light hues, and a visual pass in both themes.
- **`color-mix` support:** already used elsewhere in `app.css`; no new browser requirement.
- **Racing resolutions double-roll:** harmless (atomic writes, one process per repo, last write wins, both values valid).
- **User clears setting expecting orange, gets a re-roll:** documented semantic; setting an explicit value always wins.

## Acceptance criteria

1. A repo with no `ui.accent` anywhere gets a random palette color on first use, persisted to `.lessmess/settings.json`; it survives restarts; both themes and the favicon/brand icon reflect it.
2. Settings → UI offers the Accent swatch picker at both layers with correct Inherit/source badges; saving applies on the next page load without a rebuild; the API rejects unknown ids with 422.
3. No hardcoded accent derivatives remain: the card-expand hover, In-progress pill (both themes), and terminal cursor all follow `--accent`.
4. `go vet ./...` and `go test ./...` pass; `lessmess validate` reports no new findings.
5. `README.md` documents the setting.

## Tasks

1. [ACC-00](tasks/00-accent-palette-and-settings.md) — Palette, `ui.accent` setting, roll-once initialization, validation.
2. [ACC-01](tasks/01-dynamic-brand-assets.md) — Dynamic `/icon.svg`, `/favicon.ico`, `/apple-touch-icon.png` and page-head accent injection.
3. [ACC-02](tasks/02-accent-css-cleanup.md) — Replace hardcoded accent derivatives in CSS/JS.
4. [ACC-03](tasks/03-accent-settings-ui.md) — Accent swatch picker in the Settings UI.
