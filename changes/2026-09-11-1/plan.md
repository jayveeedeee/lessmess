# 2026-09-11-1: Web kanban server for the changes/ workflow

- Change ID: 2026-09-11-1
- Created: 2026-09-11
- Branch: — (repository is not yet under version control)
- Status: see [ledger.md](ledger.md)

## Objective and context

Build a single-binary Go web server (`tasktracker`) that visualizes the repository's `changes/` tree as a kanban board and can write back to it. The `changes/` markdown files remain the canonical, agent-readable database; the server is a view and editor over them, never the owner of the data.

## Current behavior

The `changes/` workflow (defined in `AGENTS.md`, bootstrapped in change `2026-09-11-0`) is plain markdown only. Viewing board state means reading ledger files by hand; there is no UI, no live view, no tooling.

## Target behavior

Running `tasktracker serve` from a repository root starts a local web server showing:

- A change list built from the root ledger (`changes/ledger.md`).
- A kanban board per change: five columns (`Not started`, `In progress`, `Blocked`, `Done`, `Cancelled`), cards from the per-change ledger's task rows in row order (= priority).
- A task detail view rendering the task markdown file.
- Drag-and-drop card moves (status change / reorder) and create-task / create-change operations, all written back to the markdown files in the pinned formats.
- Live updates via SSE when files change on disk (e.g., an agent edits a ledger while the board is open).
- `tasktracker validate` enforcing the validation contract from `AGENTS.md`.

The whole thing ships as one static binary (`CGO_ENABLED=0 go build`) with all web assets embedded via `go:embed`. No runtime file dependencies, no Node toolchain.

## Scope

In scope:

- `tasktracker serve` (flags: `--port`, default 8080; `--dir`, default cwd; binds 127.0.0.1 by default).
- `tasktracker validate` (exit code 0/1 + human-readable report).
- Read path: scan `changes/` → in-memory model → board/change/task views.
- Write path: move card (status cell + row reorder in the per-change ledger table), create task (file + ledger row), create change (directory scaffold + root ledger row).
- fsnotify watching with debounced rescan; SSE push to connected boards.
- Validation per the seven rules in `AGENTS.md`, surfaced in CLI and as a UI banner.
- Unit and handler tests; dogfooding against this repository's own `changes/`.

## Non-goals

- Multi-user, authentication, or remote deployment (local single-user tool; binds localhost).
- Multi-project picker (one `--dir` per process).
- Editing task markdown body content in the UI (files remain editor/agent-owned; the server rewrites only the pinned table regions and creates new files from templates).
- Delete or archive operations from the UI (create + update only — the server never deletes files).
- SPA frontend, git integration, mobile-specific UI, release packaging/CI.

## Design decisions

1. **Frontend: htmx + SortableJS, server-rendered `html/template`, no build step.** Vendored pinned versions served from the embedded binary. Chosen over a React/Svelte SPA to keep zero Node tooling; the JSON API stays separate so an SPA could replace it later. (Records the option discussion outcome.)
2. **Server consumes the existing format; no `AGENTS.md` format changes.** The pinned schemas from change `2026-09-11-0` are the parse target. If a format gap is discovered, it becomes a new change rather than a silent format drift.
3. **Go stdlib HTTP mux (Go 1.22+ routing) with three external deps only**: `github.com/fsnotify/fsnotify` (watching), `gopkg.in/yaml.v3` (task frontmatter), `github.com/yuin/goldmark` (markdown rendering in task detail). No web framework.
4. **Atomic writes + freshness checks.** All writes go temp-file + rename. Before writing, the store re-stats the file; if it changed externally since last scan, it rescans and retries once, then surfaces a conflict (HTTP 409) rather than clobbering.
5. **Never write over unparseable files.** If a ledger fails validation, writes to it are refused (HTTP 422) and the UI shows a validation banner. Protects canonical data from corruption by the tool.
6. **Row order is card order**, per `AGENTS.md`; a drag reorder rewrites the task table rows in the new order, preserving all non-table ledger content byte-for-byte.
7. **Read model**: full scan at startup (milliseconds at expected scale), cached in memory, invalidated per-file by fsnotify events.

## Detailed implementation approach

Architecture:

```text
cmd/tasktracker/main.go    CLI entry: serve / validate subcommands (stdlib flag)
internal/model/            Types + parsers + serializers for root ledger, per-change
                           ledger (meta, task table, raw-preserving), task frontmatter
internal/store/            Scan, in-memory cache, fsnotify watch (debounced),
                           atomic writes with freshness check, Validate() (7 rules)
internal/server/           HTTP handlers, SSE hub, template rendering
web/templates/             html/template pages: index, board, task detail partial
web/static/                Vendored htmx + SortableJS (pinned, documented), app.css, app.js
```

Routes:

- `GET /` — change list (root ledger).
- `GET /changes/{id}` — kanban board for one change.
- `GET /changes/{id}/tasks/{task}` — task detail (goldmark-rendered), also used as htmx partial.
- `POST /changes/{id}/tasks` — create task (template file + ledger row).
- `POST /changes/{id}/move` — move card: `{task, toStatus, toIndex}` → ledger rewrite.
- `POST /changes` — create change (allocate number per naming rules, scaffold plan/ledger/tasks, root row).
- `GET /events` — SSE stream; fired on store changes (fsnotify or local writes).
- `GET /api/validate` — validation report JSON.

Board interaction: SortableJS drag end → `POST /move` → store rewrites ledger → htmx swaps the board fragment. SSE events trigger htmx board refresh, covering external (agent) edits.

All filesystem operations are constrained to the `changes/` tree under `--dir` (path traversal guard).

## File-level impact

New files only (Go module, `cmd/`, `internal/`, `web/`, `README.md`). No modification of `AGENTS.md`, `changes/` format, or existing change records — except that dogfooding (KAN-07) exercises writes against this repository's own `changes/`.

## Data, API, message, configuration, or schema changes

None to the workflow format. The server reads and writes the pinned schemas defined in `AGENTS.md`. Configuration is CLI flags only; no config file. Any future server-side UI state goes in `.tasktracker/` (gitignored) per the tooling-state rule — the MVP does not need it.

## Safety, security, migration, and rollback considerations

- Binds `127.0.0.1` by default; no auth (accepted risk for a local single-user tool, documented in README).
- Path traversal guard on all file operations; writes constrained to `changes/`.
- Server performs create + update only; never deletes.
- Atomic writes prevent half-written files; unparseable ledgers are write-refused.
- Rollback: stop the server; files are plain markdown (once the repo is under git, `git diff`/`git checkout` covers it).
- Migration: none — format unchanged.

## Testing and verification strategy

- `internal/model`: fixture-based parse tests (valid + each malformation class) and parse→mutate→serialize→reparse round-trip tests; byte-preservation test for non-table content.
- `internal/store`: tempdir tests for scan, watch events, atomic write, freshness-conflict path, and all seven validation rules.
- `internal/server`: `httptest` handler tests for all routes including move semantics and conflict/validation error codes.
- Manual: run the binary against this repository's `changes/` (dogfood) and click through the board checklist (KAN-07).
- `go vet ./...` and `go test ./...` clean.

## Observability requirements

`log/slog` structured logging: startup scan summary (changes/tasks found, validation result), every file write (path, operation), validation failures, watcher errors. Validation failures also surface as a persistent UI banner.

## Rollout sequence

Local development tool; no deployment. Build with `CGO_ENABLED=0 go build -o tasktracker ./cmd/tasktracker` and run from the repository root. README documents build and usage.

## Risks and mitigations

- **Markdown table parsing brittleness** — mitigated by the pinned schemas, strict parser, and write-refusal on malformed files.
- **Race with external (agent) edits** — atomic writes, freshness checks with one rescan-retry, 409 on persistent conflict, SSE refresh so the board shows external changes immediately.
- **htmx/SortableJS UX ceiling** — accepted for MVP; API-first design keeps an SPA migration possible without backend changes.
- **Dependency supply chain** — only three deps, all mainstream; vendored JS pinned with documented URLs (KAN-05).

## Acceptance criteria

1. `tasktracker serve` from this repository root serves a board of the real `changes/` tree.
2. Board shows the five status columns with cards from ledger rows in row order.
3. Dragging a card between columns or reordering within one rewrites the ledger correctly: pinned schema preserved, non-table content untouched, file re-parses clean.
4. Creating a task or change from the UI produces spec-compliant files and ledger rows.
5. An external file edit appears on the board via SSE without a server restart.
6. `tasktracker validate` enforces all seven `AGENTS.md` rules and exits non-zero on violation.
7. `CGO_ENABLED=0 go build` produces a single static binary needing no runtime asset files.
8. `go test ./...` passes.

## Tasks

1. [KAN-00: Project scaffold and CLI entry](tasks/00-project-scaffold.md)
2. [KAN-01: Ledger and task-file parsers](tasks/01-ledger-and-task-parsers.md)
3. [KAN-02: Serializers and atomic writers](tasks/02-serializers-and-writers.md)
4. [KAN-03: Store — scan, cache, watch, validate](tasks/03-store-watch-validate.md)
5. [KAN-04: HTTP API and SSE](tasks/04-http-api-and-sse.md)
6. [KAN-05: Board UI](tasks/05-board-ui.md)
7. [KAN-06: validate command and UI validation banner](tasks/06-validate-command.md)
8. [KAN-07: Dogfood, end-to-end verification, README](tasks/07-dogfood-and-readme.md)
