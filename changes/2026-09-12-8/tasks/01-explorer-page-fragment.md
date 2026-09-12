---
id: EXP-01
title: Explorer page and tree fragment
---

# EXP-01: Explorer page and tree fragment

Status: see [../ledger.md](../ledger.md).

## Objective

Add `GET /explorer` (full page) and `GET /explorer/tree` (htmx fragment):
a recursive, expandable tree of covered directories with purposes and entry
blurbs, plus a disabled-state page when the docs system is off, and a nav link
in the layout.

## Dependencies

- EXP-00

## Scope

- View model: walk covered tree, attach `DirDocs` content per node.
- Template: recursive `<details>/<summary>` rendering (dirs first, then files,
  with blurbs; placeholders rendered as muted em dashes).
- Chat button per directory node (form/JS hook; endpoint lands in EXP-02).
- Disabled state: friendly guidance when `agentsdocs.json` is absent.
- Nav link in `layout.html`; CSS for the tree in `app.css`.

## Implementation steps

1. `internal/server/explorer.go`: `GET /explorer` and `GET /explorer/tree`
   handlers building the view model and rendering.
2. `web/templates/explorer.html`: page + `{{define "explorerTree"}}` fragment
   (recursive template).
3. Layout nav link; `app.css` tree styles.
4. Tests: view model over seeded fixture tree; fragment contains
   purposes/blurbs; disabled repo renders guidance; placeholder rendering.

## Verification

- `go test ./internal/server` passes; manual render check on this repo.

## Completion criteria

- The tree renders this repo's seeded docs accurately in a browser.

## Files affected

- `internal/server/explorer.go` (new)
- `internal/server/explorer_test.go` (new)
- `web/templates/explorer.html` (new)
- `web/templates/layout.html`
- `web/static/app.css`

## Notes

- 2026-09-12 — Implemented: `internal/server/explorer.go` (view model +
  `GET /explorer` + `GET /explorer/tree`), `web/templates/explorer.html`
  (recursive `<details>` tree via self-referencing `explorerNode` template),
  renderer gains an `explorer` template and the `partial` set now includes
  explorer.html, nav link in `layout.html`, tree CSS in `app.css` (chat button
  reveals on row hover). Decisions:
  - The page's "content" template receives the full `pageData`, so fields are
    referenced via `.Data.*` (a test caught the zero-value bug).
  - Chat button is inside `<summary>`; its JS (EXP-02) must stopPropagation so
    it doesn't toggle the node.
  - `agentsdocs.json` itself shows as a root file entry (honest, no hiding).
- Verification: `go test ./internal/server -count=1` — view model over an
  annotated fixture tree (nesting, purposes, blurbs, placeholders), fragment
  render contents, disabled-state guidance, enabled page; manual smoke on this
  repo (`serve` + curl `/explorer`) renders the real covered tree with seeded
  purposes. `gofmt`/`go vet` clean; full suite green.