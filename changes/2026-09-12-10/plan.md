# 2026-09-12-10: Bind change sessions to their change

- Change ID: 2026-09-12-10
- Created: 2026-09-12
- Branch: —
- Status: see [ledger.md](ledger.md)

## Objective and context

A session opened from an existing change (the board's Sessions panel) must treat
everything the user asks for in that conversation as work on **that** change.
Today it does not: asking for new things in such a session led the agent to
create a brand-new change directory.

## Current behavior

1. `createChangeSession` (`internal/server/mapping.go`) creates and maps the
   opencode session but **never primes it** — it is the only session type with
   no prompt (discussion sessions get `discussionPrompt`, commit sessions
   `commitPrompt`, explorer chats `explorerPrompt`). The agent therefore has no
   knowledge of its change binding; the root `AGENTS.md` workflow ("every new
   change gets one directory under `changes/`") pushes it to scaffold a new
   change for each new request.
2. `POST /changes/scaffold` (`internal/server/changesession.go`) happily
   scaffolds for **any** session ID, including one already mapped to a change —
   there is no server-side guard. A discussion session that already scaffolded
   its change can also be driven to scaffold a second one.

## Target behavior

1. **Prime prompt (layer 1).** New change sessions are primed at creation with
   a change-scoped brief: the session is bound to change `<id> — <title>`; read
   its `plan.md`/`ledger.md` first; every request in this conversation belongs
   to this change; refine the plan and add task files/ledger rows under the
   existing ID prefix; **never** create a new change directory or call
   `/changes/scaffold`; for genuinely unrelated work, ask the user to start a
   new discussion from the index page. A prime failure fails the creation and
   deletes the session (same pattern as the discussion flow), so an unbound
   session is never leaked.
2. **Scaffold guard (layer 2).** `POST /changes/scaffold` looks up the calling
   session in the mapping; if it is already linked to a change, the endpoint
   refuses with `409 Conflict` and a JSON error that names the change and
   redirects the agent to continue within it. Sessions in the `_unassigned`
   bucket scaffold normally (moved on success); sessions not in the mapping at
   all keep the existing direct-link fallback. No exceptions to the guard.
3. Sessions created before this change are not retro-primed (the guard still
   protects them).

## Scope

- `internal/server/changesession.go`: `changePrompt(changeID, title)` builder;
  bound-session guard in `scaffoldChange`.
- `internal/server/mapping.go`: prime call in `createChangeSession`;
  `changeOf(session)` lookup on the mapping.
- Tests in `internal/server/` covering the prompt, the priming, the 409 guard,
  and the preserved existing paths.
- Live dogfood of both layers.

## Non-goals

- Re-priming or migrating sessions that already exist (no primed flag, no
  open-time hook).
- Escape hatch allowing a bound session to scaffold anyway (user decision:
  never — unrelated work starts as a new index discussion).
- UI changes; AGENTS.md workflow-text changes.

## Design decisions

1. **Two layers, prompt + deterministic guard** (user decision) — the prompt
   guides the agent; the endpoint guarantees the invariant even for unprimed
   or misled sessions.
2. **Absolute binding** (user decision) — a change-linked session can never
   start a separate change; the prompt points the user at the index discussion
   flow instead.
3. **Prime at creation only** (user decision) — old sessions stay unprimed but
   remain covered by the guard.
4. **Prime failure = creation failure** — mirrors the discussion flow; an
   unbound session is the bug being fixed, so it must not leak.
5. **Guard message is agent-facing** — the 409 body names the change and tells
   the agent to continue within it, so a misled agent self-corrects.

## Acceptance criteria

1. A newly created change session receives a prime prompt naming its change,
   restricting it to that change, and forbidding `/changes/scaffold`.
2. `POST /changes/scaffold` with a session mapped to a change returns 409 and
   creates nothing; unassigned and unmapped sessions behave as before.
3. `go vet ./...` and `go test ./...` are green; `tasktracker validate` stays
   clean.
4. Live dogfood: a session opened on an existing change adds requested work to
   that change; a scaffold attempt from a bound session is refused.

## Tasks

1. [BSB-00: Change-scoped prime prompt for change sessions](tasks/00-change-session-prompt.md)
2. [BSB-01: Scaffold guard for bound sessions](tasks/01-scaffold-guard.md)
3. [BSB-02: Live dogfood of session binding](tasks/02-dogfood.md)
