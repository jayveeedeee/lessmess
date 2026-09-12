---
id: EXD-01
title: Explorer templates — split shell, dirs-only tree, detail fragment
---

# EXD-01: Explorer templates — split shell, dirs-only tree, detail fragment

Status: see [../ledger.md](../ledger.md).

## Objective

Restructure `web/templates/explorer.html` into a two-pane page: a
directories-only tree fragment on the left and a new `explorerDetail`
fragment on the right, with root's detail rendered server-side on first load.

## Dependencies

- EXD-00 (the `/explorer/detail` endpoint the htmx attributes target, and the
  agreed `explorerDetail` template name)

## Scope

- `web/templates/explorer.html` only.

## Implementation steps

1. `content`: keep the disabled-state branch verbatim. For the enabled
   branch, replace the single `#explorer-tree` div with a split shell:
   `<div class="explorer-split">` containing `<nav id="explorer-tree">` (tree
   fragment) and `<section id="explorer-detail">` (initial detail rendered
   inline via `{{template "explorerDetail" .Data.Root}}`).
2. `explorerTree` / `explorerNode`: render directories only — drop
   `.Purpose` from the summary and drop the `.Files` loop. Keep
   `<details data-rel>`, the recursive `{{template "explorerNode" .}}`
   reference, and the `.explorer-chat` button inside `<summary>` (with its
   `data-dir`).
3. Put `hx-get="/explorer/detail?dir={{.Rel}}"`,
   `hx-target="#explorer-detail"`, `hx-trigger="click"` on each directory
   `<summary>` (on the summary, not the `<details>`, so nested rows don't
   double-fire). Add a hook class (e.g. `xdir`) for JS selection in EXD-03.
4. New `explorerDetail` template: header with directory name + `.Rel` path
   and the `.explorer-chat` button; purpose paragraph (muted
   "— no description yet" when empty); file list — each row block with
   `.xfile-name` and the blurb beneath it (muted "—" when empty).
5. Root node's `<details>` renders with the `open` attribute so the tree
   starts expanded at the top level.

## Verification

- `CGO_ENABLED=0 go build -o tasktracker ./cmd/tasktracker` succeeds
  (templates parse at startup).
- With the server running, `/explorer` shows the dirs-only tree and root's
  detail; clicking a directory swaps the detail pane via htmx.
- `/explorer/tree` still returns a valid fragment with `data-rel` attributes.

## Completion criteria

- Tree fragment contains no purposes or files; detail fragment renders name,
  path, purpose, and file blurbs; initial page works without JS.

## Files affected

- `web/templates/explorer.html`

## Notes

- Kept template names globally unique and the recursive
  `{{template "explorerNode" .}}` reference intact; the `.explorer-chat`
  button stays inside `<summary>` so the JS propagation guard keeps working.
- Root `<details>` renders `open` and its summary carries `selected`, so the
  initial page needs no JS; JS normalizes the selection after tree refreshes.
- htmx attributes sit on the `<summary>` (not `<details>`) so nested rows
  don't double-fire.
- Verified 2026-09-12: full `go test ./...` green; live curl of `/explorer`,
  `/explorer/tree`, and `/explorer/detail?dir=internal/model` returned the
  expected markup on the rebuilt server; `TestExplorerTreeFragmentRender` now
  pins dirs-only (asserts purposes/files are absent from the tree).
