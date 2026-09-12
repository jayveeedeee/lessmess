# Ledger — 2026-09-12-11

- Change ID: 2026-09-12-11
- Plan: [plan.md](plan.md)
- Branch: —
- Overall status: Done
- Last updated: 2026-09-12

## Status definitions

| Status | Meaning |
| --- | --- |
| Not started | Work has not begun. |
| In progress | Implementation or verification is actively underway. |
| Blocked | Work cannot continue until a documented dependency, decision, approval, or external condition is resolved. |
| Done | All verification and completion criteria in the task file have passed. |
| Cancelled | The task was intentionally removed from scope and the reason is recorded. |

## Tasks

Row order is display and priority order; top row is highest priority.

| Task | Title | Status | Depends on | Updated | Notes |
| --- | --- | --- | --- | --- | --- |
| [EXD-05](tasks/05-tree-polish.md) | Remove tree chat buttons and square orange selection | Done | EXD-01, EXD-02, EXD-03 | 2026-09-12 | Chat button gone from tree (detail header keeps it); selection is now a square orange highlight on the folder name. Tests green, validate OK, screenshot-verified on the rebuilt :9090 server. |
| [EXD-00](tasks/00-server-detail-endpoint.md) | Detail endpoint and route | Done | — | 2026-09-12 | `GET /explorer/detail` + `findExplorerNode` + route; vet clean, 3 new tests pass (lookup, 422s, 503); 200-fragment test lands with EXD-01's template. |
| [EXD-01](tasks/01-explorer-templates.md) | Explorer templates — split shell, dirs-only tree, detail fragment | Done | EXD-00 | 2026-09-12 | `explorer.html` rewritten: split shell, dirs-only tree with hx-get on summaries, root open+selected, `explorerDetail` fragment; tests updated and passing (old inline-layout assertions replaced). |
| [EXD-02](tasks/02-explorer-css.md) | Explorer CSS — two-pane layout, guide lines, selection | Done | EXD-01 | 2026-09-12 | Explorer section restyled: split panes (sticky 300px tree), guide lines, selected-row accent, always-visible right-aligned chat, detail typography, 720px stacking; live UI check deferred to the rebuild+restart in EXD-03/EXD-04. |
| [EXD-03](tasks/03-explorer-js.md) | Explorer JS — selection state and refresh restore | Done | EXD-01 | 2026-09-12 | Selection tracking with root fallback, open+selected restore, detail re-fetch on docs events; `htmx.process` after tree swaps; chat handler moved to capture phase. Live-checked via rebuilt server on :9090 (screenshots: wide split + narrow stacked). |
| [EXD-04](tasks/04-verify-and-docs.md) | Verification, tests, and README | Done | EXD-02, EXD-03 | 2026-09-12 | Build/vet/test all green, validate OK, README explorer section rewritten, live UI pass (wide + narrow + disabled state) done on the rebuilt :9090 server. |

## Decision log

- 2026-09-12: Layout direction chosen with the user — master/detail split (left dirs-only navigation tree, right reading pane) over two-line rows, grid columns, or per-directory cards.
- 2026-09-12: Files live only in the detail pane; the left tree is directories only (user decision).
- 2026-09-12: Detail content comes from a new htmx fragment endpoint `GET /explorer/detail?dir=<rel>` rather than embedding all details in the initial page (user decision); root detail is server-rendered on first load so the pane is never empty.
- 2026-09-12: Bundled polish — tree guide lines and an always-visible, right-aligned chat button; placeholder wording unchanged.
- 2026-09-12: Selection is client-side state; tree refreshes restore open nodes and the selected `data-rel`, and the detail pane re-fetches for the selected dir on docs SSE events.
- 2026-09-12 (implementation): htmx attaches its own click listener to each `hx-get` summary, so the chat button handler moved to the capture phase — its stopPropagation now prevents detail fetches on chat clicks (the old bubble-phase handler ran too late). Tree fragments swapped via innerHTML must be passed through `htmx.process` to wire the new summaries.
- 2026-09-12 (user review, EXD-05): chat buttons removed from the left tree entirely — chat now lives only in the detail pane header. Selection restyled from the inset accent bar + surface background to a square orange highlight on the folder name itself, matching the UI's squared-off look.
