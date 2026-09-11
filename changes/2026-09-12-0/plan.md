# 2026-09-12-0: opencode session integration

- Change ID: 2026-09-12-0
- Created: 2026-09-12
- Branch: main
- Status: see [ledger.md](ledger.md)

## Objective and context

Integrate opencode (the AI coding agent this environment runs on) into tasktracker: each change can have live agent sessions embedded in the board UI, and a "new change session" action scaffolds a change and starts a primed agent session on it. The board remains the live view — agents work the `changes/` workflow through their sessions and the board updates via the existing fsnotify/SSE pipeline.

Verified facts about the target system (from research on 2026-09-11/12 against the live service and V2 docs):

- opencode2 runs as a background service with an HTTP API (139 OpenAPI-documented endpoints); currently discovered via `opencode2 service status` (dynamic port).
- Auth: HTTP Basic, user `opencode`, password in `~/.config/opencode/service.json`.
- Relevant endpoints: `POST /api/session`, `GET /api/session`, `GET /api/session/{id}`, `POST /api/session/{id}/prompt`, `POST /api/session/{id}/rename`, `GET /api/session/{id}/message`, `GET /api/event` (SSE), PTY routes (`POST /api/pty`, `POST /api/pty/{id}/connect-token`, `GET /api/pty/{id}/connect` WebSocket).
- CLI attach: `opencode2 --session <sessionID>` resumes a session in the TUI (verified in `opencode2 --help`); also `--prompt` for priming and `--auto` for auto-approving permissions.
- Sessions are project-scoped (`location.directory`) and persist server-side; a PTY is just a disposable view onto them.
- `opencode2 serve` can start a dedicated server (fallback if shared-service discovery proves unreliable).

## Current behavior

tasktracker renders boards over the `changes/` files with no agent interaction; opencode sessions live only in the terminal TUI.

## Target behavior

- Each board has a "Sessions" panel listing that change's linked opencode sessions (multiple per change). Open one → an embedded terminal (xterm.js) shows the live opencode TUI attached to that session, full fidelity, interactive.
- "New session" on a board creates an opencode session (project = repo root), links it to the change, and opens the terminal on it.
- "New change session" on the index page: form (title, prefix) → tasktracker scaffolds the change via its existing endpoint → creates an opencode session → posts a primed prompt ("Continue change `<id>`: `<title>` per AGENTS.md") → links session → opens the board with the terminal attached.
- The opencode service password never reaches the browser: tasktracker proxies all API/WS traffic and injects Basic auth server-side.
- Permissions for change sessions are pre-approved via a committed `opencode.json` (per user decision; the agent acts without per-action prompts inside this repo).
- Session mapping lives in `.tasktracker/sessions.json` (gitignored tooling state, per AGENTS.md).

## Scope

In scope:

- `internal/opencode` client package: service discovery, Basic auth, minimal REST wrappers (session create/get/list, prompt, rename, PTY create/connect-token).
- Proxy layer in `internal/server`: `/api/oc/*` REST proxy (auth injected), SSE proxy for `/api/event`, WebSocket proxy for PTY `connect`.
- Session mapping store (`.tasktracker/sessions.json`) + JSON endpoints (list/create/delete mapping per change).
- Terminal UI: vendored xterm.js (+ fit addon, pinned in VENDOR.md), sessions panel per change, terminal view with reconnect.
- New-change-session flow (scaffold-first).
- `opencode.json` permissions config + README/docs updates.
- Tests for client/proxy/mapping; dogfood against this repository.

## Non-goals

- Custom chat-transcript console (user chose the embedded terminal instead).
- `persistentPty` prototype routes (reconnect spawns a fresh PTY onto the persisted session instead).
- Multi-user/remote access, auth beyond localhost binding.
- Rendering session activity on cards, session analytics, MCP/plugins integration.
- Editing opencode configuration beyond the permissions file.

## Design decisions

1. **Embedded terminal via in-process PTY + xterm.js** (user): real TUI in the browser; no transcript re-implementation. tasktracker spawns `opencode2 --session <id>` in its own PTY (creack/pty) and bridges it to a WebSocket it serves; the service holds the session, so reconnects spawn a fresh PTY on the same session. **Pivot (OCI-00):** the service's PTY `connect-token` route is 403-gated (verified, including via the official CLI), so the service PTY routes are not used; the service API is used for plain REST only (session create/list/rename/prompt/delete — all verified).
2. **Scaffold-first change creation** (user): deterministic — the change row exists before the agent starts; the session is primed with the change ID and title.
3. **Permissions pre-approved in `opencode.json`** (user): exact V2 schema taken from the config docs at implementation time (no guessed fields). `--auto` on the spawned TUI is the documented fallback if config proves insufficient.
4. **Many sessions per change** (user): mapping is change ID → list of session IDs with titles and timestamps.
5. **Proxy with server-side auth injection**: the service password stays in `~/.config/opencode/service.json`, read by the Go process; the browser only talks to tasktracker (localhost).
6. **Mapping in `.tasktracker/`** per the tooling-state rule: `changes/` never carries tool-specific IDs.
7. **Shared background service** as the API target (sessions stay visible to the user's normal TUI too); `opencode2 serve` dedicated instance is the documented fallback.

## Detailed implementation approach

```text
internal/opencode/   discovery (service status URL), auth (service.json),
                     REST client (session, prompt), types
internal/terminal/   in-process PTY manager (creack/pty)
internal/server/     + terminal WS bridge (spawn opencode2 --session <id>,
                     bridge, kill on close)
                     + mapping endpoints: GET/POST/DELETE /changes/{id}/sessions
internal/store/      (unchanged)
.tasktracker/        sessions.json (gitignored): changeID -> [{session, title, created}]
web/templates/       sessions panel partial, terminal view
web/static/          xterm.js + fit addon (vendored, pinned), app.js additions
```

All service calls are made server-side via the opencode client; the browser only talks to tasktracker (mapping endpoints + terminal WS), so the service password never reaches it.

Terminal flow: UI "open session" → tasktracker spawns `opencode2 --session <id>` in a creack/pty PTY (cwd = repo root) → bridges PTY output to a WebSocket it serves → xterm.js renders; input flows back over the WS. Resize messages update the PTY window size. Close kills the PTY process (session persists in the service).

New-change-session flow: index form → existing `POST /changes` scaffold → `POST /api/oc/session` (directory = repo root) → `POST /api/oc/session/{id}/prompt` with primed text → mapping append → redirect to new board with terminal open.

## File-level impact

New: `internal/opencode/`, `.tasktracker/` (gitignored), `opencode.json`, xterm assets.
Modified: `internal/server` (routes/proxy), `web/templates` (board, index, partials), `web/static/app.js`, `web/static/app.css`, `web/static/VENDOR.md`, `README.md`, `AGENTS.md` only if a workflow rule changes (none anticipated).

## Data, API, message, configuration, or schema changes

- New `opencode.json` (project config, committed; permissions pre-approval).
- New `.tasktracker/sessions.json` (gitignored).
- No changes to the `changes/` workflow formats.

## Safety, security, migration, and rollback considerations

- Localhost-only, as today; the proxy never exposes the service password to the browser.
- Pre-approved permissions mean the agent can edit/run inside this repo without per-action prompts — accepted by the user, documented in README; config scopes auto-approval to this project only.
- PTY/WS proxy restricted to PTYs created via the proxy (token-gated by upstream connect-token).
- Rollback: stop the server; the integration is additive, board/file behavior unchanged without it.

## Testing and verification strategy

- `internal/opencode`: recorded-fixture tests for discovery/auth/REST; optional live smoke test gated behind an env var.
- Proxy: `httptest` fake upstream verifying auth injection, SSE pass-through, WS handshake.
- Mapping: tempdir tests incl. corrupt-file handling.
- UI: headless-Chrome screenshots of the sessions panel and terminal view; manual dogfood (OCI-07).
- `go vet`, `go test ./...`, `tasktracker validate` clean throughout.

## Observability requirements

slog for discovery results, proxy errors, mapping writes, PTY lifecycle (create/connect/close). No session content logged.

## Rollout sequence

Local tool; build and run as today. No migration.

## Risks and mitigations

- **~~PTY routes are experimental~~** → materialized in OCI-00: `connect-token` is 403-gated even via the official CLI. Mitigated by the pivot to an in-process PTY (creack/pty); the service API surface used is plain verified REST.
- **CLI attach semantics change** → OCI-00 verified `--session` resume via `opencode2 run`; fallback: start fresh TUI in PTY and adopt its new session ID via `GET /api/session`.
- **xterm.js vendoring/licensing** → MIT; pin + checksums in VENDOR.md as with htmx/SortableJS.
- **Service discovery flakiness** → cache URL, re-discover on failure; document `service restart` recovery.
- **Permission pre-approval misconfiguration** → schema from V2 docs, verified live in OCI-06 dogfood.

## Acceptance criteria

1. Board shows a Sessions panel per change; create/list/open multiple sessions per change; mapping persists across server restarts in `.tasktracker/`.
2. Opening a session renders the live opencode TUI in the browser (xterm.js), interactive; agent file edits appear on the board via existing fsnotify/SSE without reload.
3. "New change session" scaffolds a spec-compliant change and opens a primed session attached to it; `tasktracker validate` stays clean afterward.
4. The service password is never sent to the browser (verified by inspecting proxied responses/headers).
5. xterm.js vendored + pinned in VENDOR.md; single static binary still self-contained.
6. `go test ./...` passes; `opencode.json` permissions work (no approval prompts block a change session edit).

## Tasks

1. [OCI-00: Verify opencode API contracts](tasks/00-verify-api-contracts.md)
2. [OCI-01: opencode client package](tasks/01-opencode-client.md)
3. [OCI-02: API/SSE/WS proxy layer](tasks/02-proxy-layer.md)
4. [OCI-03: Session mapping store and endpoints](tasks/03-session-mapping.md)
5. [OCI-04: Terminal UI with xterm.js](tasks/04-terminal-ui.md)
6. [OCI-05: New change session flow](tasks/05-new-change-session.md)
7. [OCI-06: Permissions config and docs](tasks/06-permissions-config.md)
8. [OCI-07: Dogfood and acceptance verification](tasks/07-dogfood.md)
