---
# 2026-09-20-ve0wu: General codebase chat

- Change ID: 2026-09-20-ve0wu
- Created: 2026-09-20
- Branch: —
- Status: tracked in the tool-owned JSON state (.lessmess/workflow/)

## Objective and context

The board wants a way to talk to the codebase without starting a change: today every
session spawn is change-shaped — `POST /changes/session` primes the full workflow
discussion aimed at scaffolding, `POST /explorer/chat` is a directory-scoped Q&A that
503s without the docs system, and settings/commit flows are narrow-purpose. This change
adds a **Chat** button in the header next to Settings that spawns a fresh repo-root
opencode session — a free agent for general discussion and codebase work — and opens the
terminal overlay on it, with no change-workflow binding and no docs-system dependency.

## Current behavior

- `layout.html` renders `topnav-right` with only **Settings** (plus the docs bell and
  theme toggle), guarded off on the setup page.
- All session spawns prime with audience-keyed instruction modules (`discussion`,
  `change`, `task`) or purpose prompts (`explorer`, `commit`, `repoCommit`, `gardener`);
  unassigned sessions (explorer chats, discussions) surface in the index Discussions list.
- `openTerminal(sessionID, title)` attaches the terminal overlay to any session.

## Target behavior

- A **Chat** button in `topnav-right` (before Settings, inside the setup guard) on every
  page. Clicking it POSTs `POST /chat/session`; on success the terminal overlay opens on
  the new session.
- `POST /chat/session` spawns an opencode session in the repository root, primes it with
  a new `chat` audience module (free-agent stance: discuss the codebase, read anything,
  edit when explicitly asked — no workflow instructions, no scaffold trigger), maps it to
  the unassigned bucket (Discussions), and returns 201 with the usual `sessionResponse`.
- Every click creates a fresh session; earlier chats stay openable from Discussions.

## Scope

- `internal/server`: new `chat.go` (handler + prompt composition), a `chat` module in
  `instructions.json`, route registration in `server.go`, handler/prime tests.
- `web/templates/layout.html` (button), `web/static/app.js` (click handler), `app.css`
  only if the button needs styling beyond the nav link look; render-test coverage.
- `README.md`: user-facing note for the new button and endpoint.

## Non-goals

- No `prompts.chat` settings addendum key (built-in prime only; can be added later the
  same way `prompts.explorer` exists).
- No session reuse/persistence: one click = one new session; no chat-history UI beyond
  the Discussions list and the terminal overlay.
- No changes to change/task/discussion flows or the explorer chat.
- No worktree involvement: chats run in the main tree by design.

## Design decisions

- **Free agent stance** (user decision): the session may read and edit code on request,
  directly in the main tree. Accepted consequence: edits bypass plan/task/ledger and
  worktree tracking entirely and surface as plain working-tree changes (visible to the
  Commit all flow).
- **New session per click** (user decision): predictable, matches explorer-chat;
  Discussions keeps them accessible.
- **Minimal settings surface** (user decision): the prime is built-in text only.
- **Versioned instruction module**: the prime ships as an `instructions.json` module with
  audience `chat` — deterministic selection, `logPrime` audit, and visibility under
  `GET /workflow/instructions` — rather than a private prompt string. No other module
  lists `chat`, so the prime is exactly that module.

## Acceptance criteria

- On every page except setup the header shows **Chat** next to Settings; clicking spawns
  a session and opens the terminal; on failure the user sees the server's error and the
  button stays usable.
- `POST /chat/session` returns 201 `{session,title,created,live}`; 503 when opencode is
  unavailable or the session mapping is unreadable; a prime failure deletes the session
  and returns 502 (nothing unbound leaks); the session lands in the unassigned bucket.
- The chat prime contains no workflow modules; the audit log records audience `chat`.
- `go vet ./...`, `go test ./...`, and `lessmess validate` are clean; README documents
  the button and endpoint.

## Tasks

1. (task breakdown is maintained by the tool; see the board)
