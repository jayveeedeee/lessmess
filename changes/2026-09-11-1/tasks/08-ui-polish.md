---
id: KAN-08
title: UI polish — cleaner board and task detail
---

# KAN-08: UI polish — cleaner board and task detail

Status: see [../ledger.md](../ledger.md).

## Objective

Redesign the board and task-detail UI to be cleaner and slicker, per user feedback during KAN-05 verification: the board looks terrible and the task detail layout is ghastly.

## Dependencies

KAN-05 (this supersedes the visual aspect of its manual checklist).

## Scope

In scope: full `app.css` redesign (spacing, typography, palette, cards, columns, drag states, empty states); task detail presentation (proper prose typography, header with status, modal or refined panel); small template tweaks needed for structure. Theme direction per user decision recorded in the notes.
Out of scope: functional changes, new endpoints, JS framework changes.

## Implementation steps

1. Record the user's theme preference.
2. Rewrite `web/static/app.css` with a refined design system (palette, spacing scale, shadows, radii).
3. Adjust `web/templates/partials.html` (card meta structure, detail header) and `board.html`/`index.html` if structure needs it.
4. Improve the task-detail presentation (modal with backdrop, close via backdrop/✕/Escape) and markdown prose styling.
5. Update render tests if structure changed; run full test suite; `validate` clean.

## Verification

- `go test ./...` and `go vet ./...` pass.
- User confirms the board and task detail look clean and slick in the browser.

## Completion criteria

- User approves the visual result; all tests and validation pass.

## Files affected

- `web/static/app.css`
- `web/templates/partials.html`, possibly `board.html`, `index.html`, `layout.html`
- `internal/server/render_test.go` (if structure changes)

## Notes

- Theme decision (2026-09-11, user): build refined dark AND clean light themes with a header toggle; task detail opens as a centered modal (backdrop click / ✕ / Escape to close).
- Visual target (2026-09-11, user): restyle the dark theme toward the reference in `../ui-reference.png` (copied from `ui.PNG` at repo root): deep charcoal surfaces with minimal borders, bright orange (~#e8641f) as the single warm accent, muted olive secondary tone, slate blue-gray primary buttons, thin typography, flat and calm.
- Implemented (2026-09-11): full `app.css` redesign as a dual-theme design system (CSS custom properties, `[data-theme="light"]` override, stored in localStorage, applied pre-paint via inline script to avoid flash); softer status pills (alpha backgrounds); redesigned cards (hover elevation, ID chip + date meta, dashed empty-column hint, rotate/shadow drag states); sticky-height columns with internal scrolling; refined index table (row hover, rounded container); task detail as centered modal (backdrop blur, pop-in animation, ID chip + status pill + title header, full prose typography for rendered markdown: headings, lists, tables, code, blockquotes); theme toggle button in header (☾/☀).
- Cache-busting: asset URLs carry `?v=` params (v=2 after this redesign) because static responses are cache-controlled immutable.
- Verified: `go build`, `go vet`, all `go test` pass; `node --check app.js` clean; smoke confirmed theme init/toggle markup, ID chips, modal structure, and status pill (`status-done` on KAN-00); `tasktracker validate` OK.
- Pending: user visual approval (restart server, hard-refresh once for the new assets).
- Restyle toward `../ui-reference.png` (2026-09-11): dark theme now deep charcoal (#171717/#1f1f1f/#2b2b2b) with borderless flat surfaces separated by tone; orange #e8641f reserved for accents (brand, In progress pill, hover/focus); olive Done pills; slate blue-gray buttons; light theme aligned to the same accent language. Fixed `.cards:empty` whitespace bug (template emitted whitespace text nodes) so empty columns show the dashed "Drop tasks here" hint; modal no longer renders the task file's duplicate H1 (`dropLeadingH1`).
- Self-verified with headless Chrome screenshots (no user needed for iteration): board dark, index dark, task modal dark, board light — all inspected and matching the reference aesthetic. `go test ./...` + `validate` clean. Screenshots in /tmp (shot-board2, shot-modal2, shot-light2).
- Iteration 2 (2026-09-11, user feedback "rounded everythings", "columns aren't full height"): all border-radius flattened to 2–3px (pills are now squared tags); board page is a full-viewport flex column so all five columns stretch to the bottom of the window (scoped to `body[data-page="board"]` so the index still scrolls normally); columns lost their `max-height` calc, `.cards` flexes and scrolls internally. Verified via headless-Chrome screenshot (shot-board3).
- Root cause of the "first card covered by column title" report (2026-09-11): the `.card:hover` `translateY(-1px)` lift slid the first card's top edge under the column header/clip zone — visible only on hover, only on the top card, which is why static screenshots never reproduced it. Debugging was further complicated by a zombie pre-v4 server holding the user's port (killing the `go run` wrapper doesn't kill its child binary), so the user saw old CSS for several rounds. Fix: hover lift removed; hover is now ring + shadow only (v7 CSS). Also mitigations shipped along the way: margin above the scroll area, extra card-top padding, `?v=` cache-busting, repo-root build artifact deleted + gitignored.
- 2026-09-11: user approved the final result ("looks good") after the hover-lift fix and card-font slimming (0.8rem, weight removed, v8). Completion criteria met.
- Housekeeping note: `ui.PNG`, `overlap.png`, `mine.png` at repo root were ad-hoc user/debug screenshots; the canonical reference copy is `../ui-reference.png`.

