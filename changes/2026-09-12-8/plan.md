# 2026-09-12-8: Project explorer with directory chat

- Change ID: 2026-09-12-8
- Created: 2026-09-12
- Branch: —
- Status: see [ledger.md](ledger.md)

## Objective and context

Add a dedicated **project explorer** page to the tasktracker UI that renders the
repository's covered directory tree with the descriptions maintained by the docs
system (`2026-09-12-7`): each directory node shows its purpose line and can expand
to reveal its entries (files/subdirs) with their blurbs. Every directory node
offers a **chat action** that starts an opencode session running in the project
root but instructed to answer questions about that directory. The page live-updates
as the docs change (e.g. after a gardener run).

This dogfoods the docs system in the UI: the explorer's value is exactly the value
of the seeded `STRUCTURE.md`/`AGENTS.md` content.

## Current behavior

- The server renders two pages (index, board) with htmx-enhanced templates and a
  custom SSE channel (`/events`) fed by the store's fsnotify watch of `changes/`.
- The docs system (landed in `2026-09-12-7`) maintains per-folder
  `STRUCTURE.md`/`AGENTS.md` pairs, with purpose/blurb content parseable via
  `internal/docs` (currently unexported `carryForward`).
- opencode sessions are created/primed server-side (discussion, commit, gardener
  flows); unassigned sessions surface in the index Discussions list and open in
  the terminal overlay.

## Target behavior

1. **`GET /explorer`** renders a full page: a recursive, expandable tree of the
   covered directories (per `agentsdocs.json`). Each directory node shows its
   purpose line; expanding reveals immediate entries (covered subdirs and files)
   with their blurbs from `STRUCTURE.md`. When the docs system is disabled (no
   config), the page shows guidance instead of a tree.
2. **Live refresh**: a docs fsnotify watcher (covered dirs, debounced) emits
   `docs` events merged into the existing `/events` SSE stream; the explorer page
   re-fetches its tree fragment on such events, preserving expansion state. Other
   pages ignore `docs` events.
3. **Directory chat**: `POST /explorer/chat {dir}` creates an opencode session
   scoped to the repository root, primes it with a directory-focused prompt
   (embedding the dir's STRUCTURE.md and AGENTS.md content), maps it to the
   unassigned bucket (so it appears in the index Discussions list), and returns
   the session ID; the UI opens it in the terminal overlay.

## Scope

- `internal/docs`: export a reader for per-dir docs content (purpose, blurbs,
  freshness meta).
- `internal/server`: explorer routes (page, tree fragment, chat), docs watcher,
  SSE merge, explorer prompt builder.
- `web/`: explorer template/fragment, small JS (tree refresh, chat trigger,
  terminal open), CSS as needed, nav link in the layout.

## Non-goals

- No editing of docs from the explorer UI (read-only view).
- No uncovered-directory descriptions (uncovered dirs are simply absent).
- No per-file chat actions (directory granularity only).
- No change-session semantics for explorer chats (they stay unassigned
  discussions; no board integration).
- No open-state persistence across full page loads (only across SSE refreshes).

## Design decisions

1. **Docs-backed tree** (user decision): the tree is the covered set from
   `agentsdocs.json`, and all descriptions come from `STRUCTURE.md` — no parallel
   source of truth, and the explorer validates the docs system by using it.
2. **`<details>/<summary>` tree**: native expand/collapse with zero JS for the
   baseline interaction, matching the project's htmx/minimal-JS style.
3. **Server-rendered fragment + SSE trigger** (user chose live refresh): a docs
   watcher emits events; the page swaps the tree fragment via htmx on `docs`
   events, preserving open `<details>` by path in JS. The store stays free of
   docs dependencies: the watcher lives in `internal/server` and merges into the
   SSE handler, not into the store.
4. **Chat in root context with a directory-scoped prompt** (user requirement):
   `location.directory` is the repo root (full permissions/context); the prime
   prompt names the directory and embeds its two doc files so answers are
   grounded without a cold read.
5. **Unassigned bucket for explorer chats** (user decision): they appear in the
   index Discussions list, are resumable, and reuse the terminal overlay — no new
   session-tracking machinery.

## Implementation approach

1. Export `DirDocs` from `internal/docs` (thin wrapper over the existing
   carry-forward parser) returning purpose, blurbs, and meta for a dir.
2. Build the explorer view model server-side (`docs.Walk` + `DirDocs` per dir) and
   render the recursive fragment; page route wraps it in the layout with a nav
   link.
3. Chat endpoint: prompt builder (pure, tested), session create/prime/map in the
   existing unassigned flow, JS to open the returned session in the terminal
   overlay.
4. Docs watcher: fsnotify over covered dirs, rebuild the watch set on structural
   events, 300ms debounce, emit into `/events` as kind `docs`; explorer JS
   refreshes the fragment and restores open state; other pages skip `docs`
   events.
5. Dogfood against this repository; update README.

## File-level impact

- `internal/docs/readdocs.go` (+ tests) — exported per-dir docs reader.
- `internal/server/explorer.go` — routes, view model, chat handler, prompt.
- `internal/server/docswatch.go` — fsnotify watcher + SSE merge.
- `internal/server/server.go` — route registration, watcher lifecycle.
- `web/templates/explorer.html` (new), `layout.html` (nav link), `web/static/app.js`
  (refresh + chat JS), `web/static/app.css` (tree styling).
- `README.md` — explorer documentation.

## Data, API, message, configuration, and schema changes

- `GET /explorer` — page (HTML).
- `GET /explorer/tree` — tree fragment (HTML partial for htmx swaps).
- `POST /explorer/chat` — JSON `{dir}` → `{session}`; 400/503 on bad dir or
  unavailable service.
- SSE `/events` gains event kind `docs` (additive; existing consumers unaffected
  beyond ignoring it).
- No changes to the `changes/` contract, session mapping schema, or config
  formats.

## Safety, security, rate-limit, and rollback

- Read-only over the docs; the chat endpoint only creates sessions (existing
  opencode permission envelope applies).
- Chat endpoint validates `dir` against the covered set (no arbitrary prompt
  injection via path).
- Debounced watcher bounds event churn; page swaps are fragment-sized.
- Rollback: remove routes/template/watcher; no canonical data is touched.

## Testing and verification strategy

- `internal/docs`: `DirDocs` over fixture docs (purpose/blurbs/meta, missing
  files, placeholder handling).
- `internal/server`: view-model build over a seeded fixture tree; fragment render
  contains purposes/blurbs; disabled-state page; chat endpoint with a fake
  opencode client (prompt content, mapping, validation of dir); watcher unit
  tests (event → debounced emission, watch-set rebuild on dir add/remove).
- Manual dogfood on this repo: tree renders seeded docs, chat session answers
  directory questions, gardener run visibly refreshes the page via SSE.
- Existing suites stay green (`go vet ./... && go test ./...`).

## Observability

- `slog` for chat creation, watcher start/rebuild, and event emissions (debug
  level for per-event noise).
- The explorer itself surfaces docs-system state (stale/missing docs visible as
  placeholder blurbs and the existing banner).

## Rollout sequence

1. Reader + view model + page/fragment (navigable immediately).
2. Chat endpoint + terminal hookup.
3. Watcher + SSE live refresh.
4. Dogfood + README.

## Risks and mitigations

- **Watch-set churn** on rapidly changing trees — mitigated by debounce and
  full-set rebuild only on structural events.
- **Large trees** — fragment swaps are small (covered set only); `<details>`
  keeps render cost client-trivial.
- **Prompt injection via crafted dir names** — chat dir is validated against the
  covered set and serialized into a fixed prompt template.
- **SSE event leakage to other pages** — JS filters on event kind; board/index
  ignore `docs`.

## Acceptance criteria

1. `/explorer` renders the covered tree with purposes and entry blurbs from this
   repo's seeded docs; disabled repos see guidance instead.
2. A `docs` SSE event refreshes the tree fragment without a page reload and
   preserves which nodes are expanded; board/index pages ignore `docs` events.
3. `POST /explorer/chat` for a covered dir creates a root-scoped session, primes
   it with that dir's docs, lists it under Discussions, and opens the terminal
   overlay; invalid dirs are rejected.
4. A gardener/seed run against this repo is visible live in the explorer.
5. `go vet ./... && go test ./...` green; existing behavior unchanged.

## Tasks

1. [EXP-00](tasks/00-dir-docs-reader.md) — Exported per-dir docs reader in internal/docs
2. [EXP-01](tasks/01-explorer-page-fragment.md) — Explorer page and tree fragment
3. [EXP-02](tasks/02-directory-chat.md) — Directory chat sessions
4. [EXP-03](tasks/03-docs-watcher-sse.md) — Docs watcher and SSE live refresh
5. [EXP-04](tasks/04-dogfood-docs.md) — Dogfood and documentation
