---
id: PROJ-01
title: Project name in header, terminal head, and tab title
---

# PROJ-01: Project name in header, terminal head, and tab title

Status: see [../ledger.md](../ledger.md).

## Objective

Render the project name server-side next to the logo on every page (including setup and the
terminal overlay) and make it the permanent browser tab title.

## Dependencies

- PROJ-00 (`effectiveProjectName` accessor).

## Scope

- `internal/server/render.go`, `server.go`, `setup.go`
- `web/templates/layout.html`, `web/static/app.css`
- Render tests

## Implementation steps

1. `render.go`: add `ProjectName string` to `pageData`; add a `projectName func() string` hook to
   `renderer` (nil-safe, "" when unwired); fill `pd.ProjectName` centrally in `render()` next to
   `AssetsV`/`AccentStyle`.
2. `server.go`: wire `s.rend.projectName = func() string { return effectiveProjectName(s.st.Dir) }`
   beside the accent hook. `setup.go`: wire the setup shell's renderer the same way over its dir.
3. `layout.html`: replace `<title>{{.Title}} · lessmess</title>` with
   `<title>{{.ProjectName}}</title>`; inside the `a.brand` anchor add
   `<span class="brand-name">{{.ProjectName}}</span>` after the icon; in the terminal overlay's
   `.terminal-head` add the same span between `img.brand-icon` and `#terminal-title`.
4. `app.css`: style `.brand-name` (semibold, `white-space: nowrap`, inherits header colors) and
   confirm the terminal head flex layout still behaves with a longer name; add a graceful narrow
   viewport treatment only if the manual check shows overflow.
5. Keep every `pageData{...}` call site unchanged (`Title` stays populated — it becomes unused by
   the template but keeps a lone-template revert safe).

## Verification

- Extend `render_test.go` (patterns already assert board/index markup): each page renders the
  configured name in `<title>`, the header `.brand-name`, and the terminal-overlay `.brand-name`;
  the setup page renders the name too; a bare renderer (hook nil) still renders with the
  `lessmess` guard.
- Handler-level test: set `general.projectName` in a fixture layer, request `/` and `/settings`,
  assert the name appears (proves hook wiring, not just template).
- Build + restart the local binary and eyeball: header on index/board/explorer/settings/setup,
  open a terminal overlay and confirm the name sits next to the icon before the session title,
  and the tab reads exactly the project name on every page.

## Completion criteria

Name visible next to the logo everywhere including the terminal overlay; tab title equals the
project name on all pages; unset setting falls back to the directory basename; render/handler
tests green.

## Files affected

`internal/server/render.go`, `internal/server/server.go`, `internal/server/setup.go`,
`web/templates/layout.html`, `web/static/app.css`, `internal/server/render_test.go` (or a new
test file beside them).

## Notes

- The wizard page renders through the same layout, so the setup shell wiring matters — do not
  leave the hook nil in `NewSetup`.
- SSE-driven index reloads re-fetch full pages, so the title/name refresh without extra JS.
- Verification evidence 2026-09-17: `projectnamechrome_test.go` asserts title + exactly two
  `.brand-name` spans on /, /settings, /explorer, a board, and /setup, plus directory fallback and
  HTML escaping. Live scratch-server check confirmed `<title>` = name on all five pages and two
  spans on the index (header + terminal overlay head).
