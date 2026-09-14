# 2026-09-14-0: Settings split-shell layout and sticky header

- Change ID: 2026-09-14-0
- Created: 2026-09-14
- Branch: —
- Status: see [ledger.md](ledger.md)

## Objective and context

The Settings page feels condensed and its save-scope control is easy to miss: the
Project/Personal radios sit inline next to the page title with extra parenthetical
text, the side navigation is a minimal 9.5rem strip inside an 860px column, and three
paragraphs of intro text stack above the navigation. Separately, the global top menu
bar scrolls away on every scrolling page (Changes, Explorer, Settings).

The user wants a visual cleanup only: a proper side menu, a cleanly styled
Personal/Project segmented control in its own permanent row directly below the top
menu bar, and a top menu bar that stays visible while scrolling — hidden only when
the embedded terminal overlay is open.

## Current behavior

- `header` (brand, topnav, bell, theme toggle) is not sticky; it scrolls off-screen
  on index, explorer, and settings pages (`web/templates/layout.html`,
  `web/static/app.css:82`). The board page keeps it visible only because that page
  never scrolls (`body[data-page="board"]` is a 100vh flex with `overflow: hidden`).
- The terminal overlay is `position: fixed; inset: 0; z-index: 30`
  (`app.css:557`), so it already covers the whole viewport including the header.
- Settings scope radios are native inputs inside `.settings-head` next to the
  `h1`, labeled `Project (committed)` / `Personal (this machine)`
  (`web/templates/settings.html:5-8`).
- Settings body is `.settings-body` = `.settings-nav` (9.5rem sticky column of
  small mono pills) + `.settings-content`, capped by `.settings { max-width: 860px }`
  (`app.css:863-899`). Intro/error/hint/wizard paragraphs render above it
  (`settings.html:10-17`).
- `initSettings` in `web/static/app.js` drives everything from stable DOM hooks:
  `#settings-page`, `input[name="settings-scope"]` radios, `data-field` /
  `data-kind` / `[data-input]`, `.settings-nav` buttons with `data-group`,
  `[data-save]`, badge/status elements, and the two datalists.
- `internal/server/render_test.go` asserts settings HTML contains: `id="settings-page"`,
  `data-page="settings"`, "Prompt addenda", `class="settings-nav"`,
  `data-field="docs.autoGardenerOnClose"`, `class="settings-change"`,
  `name="settings-scope"`, "lessmess.json", ".lessmess/settings.json".

## Target behavior

1. **Sticky top menu bar (all pages).** `header` becomes `position: sticky; top: 0`
   with a z-index below the terminal overlay (30) and the board detail panel,
   e.g. 20. The overlay and detail panel continue to cover it; the banner and page
   content scroll under it. No behavior change on the board or setup pages beyond
   the header no longer scrolling away.
2. **Permanent scope row.** On the settings page, a dedicated full-width strip sits
   directly below the top menu bar and is sticky beneath it. It contains only the
   Personal/Project segmented control — no other text. The control keeps native
   `input[type=radio][name="settings-scope"]` elements (values unchanged) restyled
   as a segmented pill pair (active segment accent-filled, matching the app's
   existing pill idiom); labels are exactly `Personal` and `Project`.
3. **Split-shell settings layout.** The settings page becomes a two-pane layout
   mirroring the Explorer page's pattern: a roomy left sidebar (13–14rem) flush
   under the scope strip, holding the `Settings` title, the group navigation
   (Session, Prompts, Git, UI, Docs), and a small footer help block; and a content
   pane filling the rest. The sidebar is sticky with its `top` offset accounting
   for the sticky header + scope strip. The old 860px page cap and the
   intro-paragraph stack above the body are removed.
4. **De-cluttered content.** The layer-explanation text (project lives in
   `lessmess.json`, personal in `.lessmess/settings.json`, badge meaning) moves
   to the sidebar footer (review follow-up: the "Re-run the onboarding wizard"
   link moved again, into a display-only **General** group at the top of the
   side menu — see SPS-04), preserving the strings asserted by
   `render_test.go`.
5. **Unchanged functionality.** Section switching with URL-hash tracking,
   per-section save, scope toggling, source badges, placeholders, datalists,
   per-setting Change buttons, and the exclusions editor all behave exactly as
   today.

## Scope

- `web/templates/settings.html` — content restructure (scope strip, sidebar with
  title/nav/footer, content pane).
- `web/static/app.css` — sticky header + z-index; scope strip and segmented-control
  styles; split-shell styles (new rules replacing `.settings-head`,
  `.scope-toggle`, and the old `.settings-body` rules); responsive fallback;
  dark/light theme variants where needed.
- `web/static/app.js` — expected: no changes (goal is zero). Only touched if a DOM
  hook must move; any such move keeps selector compatibility.
- `internal/server/render_test.go` — only if an assertion actually drifts; the
  target is to keep all current assertions passing unchanged.
- Covered docs pair `web/templates/AGENTS.md` (+ STRUCTURE.md rollup) — update
  learnings that describe the settings page DOM if the hooks or structure notes
  change.

## Non-goals

- No changes to settings semantics, API, validation, or file formats.
- No JS logic changes, new endpoints, or new settings fields.
- No redesign of other pages beyond the shared sticky header.
- No changes to the setup wizard's step layout (it only inherits the sticky header).
- No mobile app-style navigation (keep the simple column-stack fallback).

## Design decisions

- **Radios-only scope row** (user choice): the sticky strip contains only the
  segmented control; the `Settings` title lives in the sidebar header instead of
  the strip.
- **Full split shell** (user choice): app-style two-pane layout modeled on the
  Explorer page (`#explorer-tree` sticky panel + flex detail), not merely a widened
  in-page column.
- **Labels exactly `Personal` / `Project`** (user choice): the `(committed)` /
  `(this machine)` suffixes are removed; the layer semantics stay discoverable via
  the sidebar footer help and the per-field source badges.
- **Native radios, restyled** — keeps `input[name="settings-scope"]` and the
  `change`-event wiring in `app.js` untouched and keeps the render-test assertion
  meaningful (the input is hidden visually, not removed from the DOM).
- **Sticky, not fixed, header** — `position: sticky` preserves document flow
  (board page flex, setup centering) with no padding compensation.
- **Layering contract:** header z-index 20 < terminal overlay 30 < board detail
  panel, so "header hidden only while the terminal is open" holds automatically.

## Implementation approach

1. Make `header` sticky (`position: sticky; top: 0; z-index: 20`); spot-check
   layering against overlay/detail/banner on board, index, explorer, settings,
   setup.
2. Restructure `settings.html`: new `settings-scopebar` strip after the page
   section opens (before the body), segmented radio markup, sidebar with
   `h1` + `.settings-nav` + footer help, content pane wrapping the existing
   five `.settings-section` blocks unchanged.
3. Style in `app.css`: strip (surface background, bottom border, sticky at
   `top: 52px`), segmented control (pill pair, accent active, focus-visible
   states), sidebar and content pane, remove superseded rules, responsive
   breakpoint (sidebar → horizontal row; strip remains).
4. Rebuild the binary, verify behavior and visuals, update the covered docs pair.

## Safety, compatibility, rollback

- Pure presentation change; no data, schema, API, or config impact.
- Rollback is reverting the template/CSS edits and rebuilding.
- Risk: sticky offsets depend on the header's fixed 52px height and the strip's
  height; both are constant, and `calc()` offsets keep them explicit.
- Risk: `render_test.go` assertions — tracked in SPS-03; goal is zero test edits.

## Testing and verification strategy

- `CGO_ENABLED=0 go build -o lessmess ./cmd/lessmess`, then `go vet ./... && go test ./...`.
- `./lessmess validate` (workflow data untouched; cheap sanity check).
- Manual UI pass on the rebuilt binary:
  - Header stays visible while scrolling index/explorer/settings; terminal overlay
    still covers everything; board page unchanged.
  - Scope strip sticky under the header; segmented toggle switches scope and
    re-renders badges/placeholders; save still writes the selected layer.
  - Nav switches sections, URL hash survives reload; per-setting Change buttons and
    exclusions editor work.
  - Dark and light themes; narrow viewport (<640px) fallback.
- Update `web/templates/AGENTS.md` learnings if the settings DOM notes changed.

## Rollout sequence

Single commit at change close (no partial rollout); server restart picks up the
rebuilt binary.

## Risks and mitigations

- Superseded CSS rules left behind → SPS-03 includes a dead-rule sweep of the
  settings section of `app.css`.
- Sticky strip inside `main`'s side padding looks inset rather than full-bleed →
  acceptable within content width, or negative-margin full-bleed decided during
  SPS-01 review.

## Acceptance criteria

1. Top menu bar remains visible while scrolling every page, and is hidden only
   while the terminal overlay is open.
2. Settings page shows a sticky radios-only row directly below the top menu bar
   with a cleanly styled segmented control labeled exactly `Personal` and `Project`.
3. Settings is a two-pane split shell: proper left side menu (title, groups,
   footer help) and roomy content pane; no intro-paragraph stack above it.
4. All settings functionality works as before; `go vet ./... && go test ./...`
   passes with no test edits (or only justified assertion updates reviewed with
   the user).

## Tasks

1. [SPS-00 — Sticky top menu bar](tasks/00-sticky-header.md)
2. [SPS-01 — Settings scope strip with segmented control](tasks/01-scope-strip.md)
3. [SPS-02 — Settings split-shell layout and content de-clutter](tasks/02-split-shell.md)
4. [SPS-03 — End-to-end verification and docs update](tasks/03-verification.md)
5. [SPS-04 — General settings group for wizard re-entry](tasks/04-general-section.md)
