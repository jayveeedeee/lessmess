# 2026-09-15-0: Modal TOC and in-modal navigation

- Change ID: 2026-09-15-0
- Created: 2026-09-15
- Branch: —
- Status: see [ledger.md](ledger.md)

## Objective and context

The board's detail modal (used for `plan.md` and task files) renders long
markdown with no way to jump between sections, and every relative markdown
link inside it (`../ledger.md`, `tasks/00-foo.md`, `plan.md`) navigates the
browser to a nonexistent URL — a 404. Users want in-modal navigation: a
table of contents beside the content and links that open the referenced
document in the same modal, exactly like clicking a kanban card.

## Current behavior

- `planDetail` / `taskDetail` partials (`web/templates/partials.html`) swap a
  single-column `.modal` (head + `.modal-body`) into `#detail`; only the body
  scrolls.
- Markdown is rendered server-side by a bare `goldmark.New()`
  (`internal/server/render.go`): headings get **no `id` attributes**, so a
  TOC has nothing to anchor to.
- Links in markdown are plain `<a href="...">` relative URLs. The server only
  serves `GET /changes/{id}/plan` and `GET /changes/{id}/tasks/{file}`; there
  is no ledger endpoint, so `.md` links 404.
- The store exposes `TaskFile` and `PlanFile` but no raw-ledger reader.

## Target behavior

- Plan and task modals show a **table of contents** in a left pane built from
  the rendered h2/h3 headings (h3 indented). The TOC and the content scroll
  **independently**, separated by a **1px vertical line**
  (`border-right: 1px solid var(--border)`). Clicking a TOC entry
  smooth-scrolls the content pane; a scroll-spy highlights the section in
  view. Docs with fewer than 2 headings render without the TOC pane.
- Relative `.md` links inside the modal open the target **in the modal**
  instead of navigating:
  - `tasks/<file>.md` → `GET /changes/{id}/tasks/<file>` (same request a
    kanban card makes),
  - `plan.md` → `GET /changes/{id}/plan`,
  - `ledger.md` / `../ledger.md` → new `GET /changes/{id}/ledger`,
  - `YYYY-MM-DD-N/plan.md` (root-ledger cross-change links) → that change's
    plan modal.
  Optional `#fragment` suffixes scroll to the heading after load.
- The change ledger itself becomes viewable in the modal via a new
  `ledgerDetail` partial.

## Scope

- `internal/server/render.go` — goldmark auto heading IDs; `ledgerView` type.
- `internal/store/store.go` — `LedgerFile(changeID) (string, error)` mirroring
  `PlanFile`.
- `internal/server/server.go` — `ledgerDetail` handler + route
  `GET /changes/{id}/ledger` (HTML partial when htmx/HTML requested, JSON
  otherwise, matching `planDetail`'s shape).
- `web/templates/partials.html` — restructure `planDetail` / `taskDetail` /
  `ledgerDetail` into head + two-pane body (`nav.modal-toc` / `.modal-body`).
- `web/static/app.css` — two-pane layout, independent scroll, 1px separator,
  TOC styling + active state, widened modal when a TOC is present.
- `web/static/app.js` — after-swap TOC builder (h2/h3, click-to-scroll,
  scroll-spy, hide when < 2 headings) and delegated `.md` link interception.
- Tests: `internal/server/render_test.go`, `internal/server/server_test.go`
  (ledger endpoint, heading ids).

## Non-goals

- No changes to the notif, commit, or terminal modals (they share `.modal`
  CSS but not the new pane structure).
- No server-side rewriting of markdown links into htmx attributes; client
  interception covers all link shapes in one place.
- External (absolute) links stay ordinary links.
- No editing surface for the ledger; read-only viewing like plan/task modals.

## Design decisions

- **Heading anchors server-side, TOC client-side.** Enable goldmark's
  `parser.WithAutoHeadingID()` so headings carry stable ids; `app.js` then
  builds the TOC from the swapped DOM. One code path serves plan, task, and
  ledger modals, and no Go heading-extraction logic is needed. Auto IDs
  slugify heading text ("Objective and context" → `objective-and-context`).
- **Client link interception.** A delegated click handler on `#detail`
  intercepts relative `*.md` links and fetches the mapped endpoint into
  `#detail` (same `fetch` + `innerHTML` pattern as `refreshExplorerDetail`).
  The close button keeps working because its handler is already delegated.
- **New ledger endpoint.** `GET /changes/{id}/ledger` returns the
  `ledgerDetail` partial; the store gains `LedgerFile` reading raw
  `ledger.md`. The leading H1 is dropped in the modal (same `dropLeadingH1`
  treatment as tasks) since the modal head already titles it.
- **GFM table extension.** Enabling it emerged as a hard requirement during
  MTOC-00: the ledger (and several plan sections) are table-shaped, and
  bare goldmark renders tables as plain text. Only `extension.Table` is
  enabled — no linkify/strikethrough behavior changes.
- **Modal width.** Base stays `min(680px, 100%)`; when the TOC pane is shown
  JS adds a `has-toc` class widening the modal to `min(880px, 100%)` so
  content is not squeezed.
- **Layout.** `.modal` keeps its column: head, then a new `.modal-panes` flex
  row (`flex: 1; min-height: 0`) holding the TOC pane (fixed ~210px, own
  `overflow-y`) and `.modal-body` (`flex: 1; min-width: 0`, own `overflow-y`).
  The separator is the TOC pane's `border-right`. Other `.modal-body` users
  are untouched because `.modal-panes` only exists in the three detail
  partials.

## Acceptance criteria

1. Opening plan, task, or ledger in the modal shows a left TOC (h2 + h3,
   h3 indented) when the document has ≥ 2 such headings; TOC and content
   scroll separately with a visible 1px vertical separator.
2. Clicking a TOC entry scrolls the content to that heading; scrolling the
   content highlights the current section in the TOC.
3. Short documents (fewer than 2 h2/h3) show no TOC pane and keep the
   current single-column look at 680px.
4. `tasks/*.md`, `plan.md`, and `ledger.md` links inside any modal open the
   target document in the same modal — no browser navigation, no 404.
5. Cross-change `YYYY-MM-DD-N/plan.md` links open that change's plan modal.
6. `GET /changes/{id}/ledger` serves the HTML partial (htmx/HTML) and JSON
   otherwise, 404 for unknown changes — matching `planDetail`.
7. `go vet ./...` and `go test ./...` pass, including new tests for the
   ledger endpoint and heading ids.

## Tasks

1. [MTOC-00](tasks/00-heading-ids-and-ledger-endpoint.md) — Heading anchors + ledger detail endpoint (store, server, partial).
2. [MTOC-01](tasks/01-two-pane-modal-layout.md) — Two-pane modal layout in templates + CSS.
3. [MTOC-02](tasks/02-client-toc-and-scrollspy.md) — Client TOC builder with click-to-scroll and scroll-spy.
4. [MTOC-03](tasks/03-inmodal-md-links.md) — In-modal `.md` link interception.
5. [MTOC-04](tasks/04-endtoend-verification.md) — End-to-end verification and checks.
