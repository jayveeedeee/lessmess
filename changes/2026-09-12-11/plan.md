# 2026-09-12-11: Explorer master-detail redesign

- Change ID: 2026-09-12-11
- Created: 2026-09-12
- Branch: —
- Status: see [ledger.md](ledger.md)

## Objective and context

The `/explorer` page renders the docs-covered directory tree, but every
directory row crams `name/`, the ✦ chat button, and the full purpose text onto
one line, and every file row puts its blurb inline. Purposes vary from short
phrases to full sentences, so rows wrap at different points, descriptions start
at a different x-position per row, and nesting compounds the raggedness. The
user asked for a cleaner, clearer, more structured layout.

Chosen direction (discussed with the user): a **master/detail split** — a
compact directories-only navigation tree on the left, and a reading pane on the
right showing the selected directory's purpose and files. Bundled polish: tree
guide lines and an always-visible, right-aligned chat button.

## Current behavior

- `explorer.html` renders a single-column recursive `<details>` tree; each
  `<summary>` is `name/ [✦ chat] purpose-text` on one flex line, and each file
  row is `name blurb` inline.
- The chat button is invisible until row hover and sits mid-row.
- `/explorer/tree` returns the whole tree fragment; on `docs` SSE events
  `app.js` re-fetches it and restores which `<details>` were open
  (`data-rel`), with no notion of a "selected" directory.
- `/explorer/chat` (POST) creates a directory-scoped opencode session; it is
  independent of layout and stays unchanged.

## Target behavior

- **Left pane:** directories-only tree — names, carets, and vertical guide
  lines showing depth. No purposes, no files, no chat buttons (revised after
  user review: chat lives only in the detail pane header). The selected row
  shows a square orange highlight on the folder name itself.
- **Right pane (detail):** for the selected directory — name + relative path
  header with chat button, the purpose as a paragraph (muted
  "— no description yet" placeholder when absent), then the directory's files
  as rows: file name on one line, blurb beneath it (muted "—" when absent).
- Clicking a directory row selects it (highlighted) and loads its detail via
  htmx into the right pane. Root (`.`) is expanded and selected on initial
  load, with its detail rendered server-side so the pane is never empty.
- Live refresh preserved: a `docs` SSE event re-fetches the tree fragment,
  restoring both open nodes and the selection, and re-fetches the detail for
  the selected directory (falling back to root if it vanished).
- Disabled state (no `agentsdocs.json`) page stays exactly as-is.

## Scope

- `internal/server/explorer.go`: view model / handler for the new detail
  fragment; `internal/server/server.go`: one new route.
- `web/templates/explorer.html`: two-pane page shell, dirs-only tree
  fragments, new `explorerDetail` fragment.
- `web/static/app.css`: explorer section restyle (split layout, guide lines,
  selection, chat placement, detail typography).
- `web/static/app.js`: explorer selection state and refresh-restore behavior.
- Server tests for the new endpoint; README "Project explorer" section.

## Non-goals

- No files in the left tree (files live only in the detail pane).
- No per-file chat, no subdirectory quick-links in the detail pane.
- No changes to `/explorer/chat`, the docs data model, docs watcher, or the
  board/index pages.
- No change to empty-description placeholder wording.
- No hand edits to marker-guarded `STRUCTURE.md`/`AGENTS.md` auto sections —
  the doc gardener refreshes those at close.

## Design decisions

- **Master/detail over two-line rows, grid columns, or cards** — user choice;
  cleanest separation of navigate vs read.
- **Dirs-only tree** — user choice; the detail pane owns files and blurbs.
- **htmx fragment endpoint for detail** (`GET /explorer/detail?dir=<rel>`) —
  user choice; matches the codebase's existing htmx/fragment idiom and avoids
  embedding every directory's detail in the initial page.
- **Server-side initial detail**: the page renders root's detail inline, so
  first load needs no JS round-trip.
- **Selection is client-side state**: htmx performs the swap; `app.js` only
  tracks the selected `data-rel` for highlight and refresh restore.
- **Detail handler re-walks the tree** and locates the node by `Rel` — same
  cost profile as the existing `/explorer/tree` endpoint, no caching.
- **Chat lives only in the detail pane header** — removed from tree rows
  after user review; the tree is pure navigation.
- **Selection is a square orange highlight on the folder name** — replaces
  the initial inset accent bar + row background after user review; the rest
  of the UI is squared off, so the highlight is too.

## API changes

- New: `GET /explorer/detail?dir=<rel>` → HTML fragment `explorerDetail`.
  Validation mirrors `explorerChat`: `dir` defaults to `.`, must pass
  `docsQ.cfg.Covered` and exist on disk; 503 when docs are disabled, 422 for
  uncovered/missing directories.

## Implementation approach

1. Server: add `explorerDetail` handler + route, reusing
   `buildExplorerView`'s walk to find the requested node.
2. Templates: restructure `explorer.html` into split shell + dirs-only
   `explorerTree`/`explorerNode` + new `explorerDetail`; wire
   `hx-get`/`hx-target` on each directory `<summary>`.
3. CSS: restyle the explorer section for the two-pane layout.
4. JS: selection highlight/tracking, open+selected restore on tree refresh,
   detail re-fetch on `docs` events.
5. Tests, README, full verification.

## Safety and compatibility

- Pure presentation-layer change; `changes/` data and the docs subsystem are
  untouched. No migration, no rollback concerns beyond redeploying the binary.
- The explorer page degrades identically when docs are disabled.

## Observability

- Detail handler logs walk/lookup problems via `slog.Warn`, matching the
  existing explorer handlers.

## Rollout

- Single binary rebuild and restart; nothing staged.

## Risks and mitigations

- *htmx click vs `<details>` toggle interplay*: clicking a summary both
  toggles and fetches — acceptable and predictable; chat button keeps its
  propagation guard so it never triggers a selection fetch.
- *Refresh losing selection*: mitigated by restoring open nodes and selected
  `data-rel` after each tree swap, with root fallback.
- *Deep trees in a narrow left pane*: left pane scrolls independently with a
  fixed width (~300px); a simple media query stacks panes on narrow screens.

## Acceptance criteria

- Left pane shows directories only, with guide lines and no chat buttons;
  the selected row's folder name carries a square orange highlight.
- Clicking a directory highlights it and shows its purpose paragraph and file
  list (names with blurbs beneath) in the right pane.
- Root is expanded and selected on load; detail pane is populated without JS.
- A `docs` SSE event refreshes the tree preserving open nodes and selection,
  and refreshes the visible detail.
- `GET /explorer/detail` returns the fragment for covered dirs, 422 for
  uncovered/missing, 503 when docs are disabled.
- Chat flow (button → opencode session → terminal overlay) still works from
  the detail header.
- `go vet ./...`, `go test ./...`, and `tasktracker validate` pass; README
  explorer section matches the new layout.

## Tasks

1. [EXD-00](tasks/00-server-detail-endpoint.md) — detail endpoint + route
2. [EXD-01](tasks/01-explorer-templates.md) — split shell, dirs-only tree, detail fragment
3. [EXD-02](tasks/02-explorer-css.md) — two-pane CSS with guide lines and selection
4. [EXD-03](tasks/03-explorer-js.md) — selection state and refresh restore
5. [EXD-04](tasks/04-verify-and-docs.md) — tests, README, full verification
6. [EXD-05](tasks/05-tree-polish.md) — remove tree chat buttons and square orange selection
