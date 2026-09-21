
# KAN-05: Board UI

Status: see [../ledger.md](../ledger.md).

## Objective

Implement the user-facing kanban UI: server-rendered templates, vendored htmx + SortableJS, drag-and-drop wiring, SSE-driven refresh, all embedded via `go:embed`.

## Dependencies

KAN-04.

## Scope

In scope: `web/templates/` (layout, change list, board with five status columns, card partials, task detail with goldmark-rendered markdown), `web/static/` (`app.css`, `app.js`, vendored `htmx.min.js` and `Sortable.min.js` pinned to specific versions with sources recorded in `web/static/VENDOR.md`), `go:embed` FS wiring, drag-end → `POST /move` → board fragment swap via htmx, create-task / create-change forms, SSE-triggered board refresh, validation banner placeholder (wired fully in KAN-06).
Out of scope: SPA features, theming, mobile layout polish.

## Implementation steps

1. Templates: layout + pages + partials; handlers render HTML when `Accept: text/html`.
2. Vendor pinned htmx (2.x) and SortableJS (1.15.x) downloads into `web/static/`, record version + URL in `VENDOR.md`.
3. `app.js`: SortableJS init per column, on-end POST move, htmx fragment swaps, SSE subscription with board refresh, create forms.
4. `app.css`: column layout, cards, status color coding.
5. Embed FS; serve under `/static/` with immutable cache headers.

## Verification

- Manual checklist against a tempdir fixture repo and this repository: board renders five columns in ledger row order; drag between columns updates status in the ledger file; drag reorder rewrites row order; create task/change appear immediately; editing a ledger file externally refreshes the board via SSE without reload; browser console clean.
- `go test ./...` still passes (template parse test in CI of tests).

## Completion criteria

- Full manual checklist passes; all assets served from the embedded binary (verified by running with no `web/` dir access — e.g., from a different cwd).

## Files affected

- `web/templates/`, `web/static/` (new)
- `internal/server/` (HTML rendering paths, embed wiring)

## Notes

- Vendored htmx 2.0.4 and SortableJS 1.15.6 from jsdelivr; versions, URLs, and SHA-256 recorded in `web/static/VENDOR.md` (2026-09-11).
- Evidence so far: `go test ./internal/server` render tests pass (full page, HX fragment, markdown detail, static assets, form-encoded creates); smoke run from `/tmp` with `--dir` proved embedded assets serve without `web/` access, board renders five columns and 8 cards, and a live `POST /move` rewrote the real ledger correctly.
- Pending: interactive browser click-through (drag-and-drop feel, console cleanliness) — no headless browser available in this environment; folded into the KAN-07 dogfood checklist for the user to confirm.
- 2026-09-11: user completed the browser click-through across the KAN-08/KAN-09 iterations (hover states, modal, plan view, create forms, board interactions) and approved the result ("looks good"). All functional behavior additionally covered by unit/handler tests and live dogfood runs recorded in KAN-07. Task complete.
