# 2026-09-12-6: Discussion-first sessions with API-triggered scaffold

- Change ID: 2026-09-12-6
- Created: 2026-09-12
- Branch: main
- Status: see [ledger.md](ledger.md)

## Objective and context

Replace the immediate-scaffold "new change session" flow with a discussion-first model: clicking the button creates only a primed opencode session (no change directory). The user discusses the objective with the agent; when the user explicitly approves starting, the agent calls a deterministic tasktracker API endpoint that builds the change scaffold with the agreed title/prefix, renames the session, and links it — all in the same session.

Supersedes the placeholder flow from 2026-09-12-3 (GEN): no placeholder title is ever created, because the change only comes into existence once the title is known.

## Current behavior

`POST /changes/session` scaffolds a change immediately with a random `untitled-*` placeholder title and primes the session to start planning at once. No discussion phase; placeholder replaced later by the agent.

## Target behavior

1. **New change session** button → creates an opencode session only (no repo writes), primed with discussion instructions plus the scaffold API contract (URL, payload, its own session ID). Session lands in the mapping's `_unassigned` bucket; the index lists open discussions (reopenable in the terminal).
2. **Discussion** — the agent asks questions and discusses scope/design; zero repository writes.
3. **Trigger** — with the user's explicit approval, the agent calls `POST /changes/scaffold` with `{title, prefix, session}` (title/prefix chosen from the discussion).
4. **Endpoint (deterministic)** — allocates the change ID, scaffolds `changes/<id>/` (plan/ledger/tasks templates with the real title) + root-ledger row (title + prefix), renames the opencode session to `<id> — <title>` server-side, moves the session from `_unassigned` to the new change in the mapping.
5. **Same session continues** — it refines plan/tasks per AGENTS.md (standing instructions from the prime prompt). The board live-shows the new change with the session already mapped.

## Scope

- Mapping: `_unassigned` bucket support + move-on-scaffold; discussions list endpoint.
- `POST /changes/session` repurposed: create discussion-only session (new prime prompt with injected API base URL + session ID), map as unassigned.
- `POST /changes/scaffold`: deterministic scaffold trigger (allocate, scaffold, root row, session rename, mapping move).
- Index UI: discussions list (title, created, reopen terminal).
- Remove the placeholder-title machinery and old prime prompt; rewrite affected tests.
- Live dogfood of the full flow.

## Non-goals

- A UI "scaffold" button for the user (agent-triggered only, per user decision; can be added later).
- Multiple scaffolds per discussion session (one discussion → one change; further work continues on the board).
- Editing the scaffold title afterward (agent/user edits files directly per workflow).

## Design decisions

1. **Agent fires the trigger, only with explicit user approval** (user decision) — encoded in the prime prompt.
2. **Title/prefix travel in the scaffold call** (user decision) — chosen from the discussion; no placeholder anywhere in the system.
3. **Server renames the session at scaffold time** — deterministic; the agent never manages naming.
4. **`_unassigned` bucket** in `.tasktracker/sessions.json` (user decision) — discussions are first-class, listed on the index, reopenable.
5. **One prompt, standing instructions** — the prime prompt carries the whole lifecycle (discuss → trigger → refine); no second prompt after scaffolding since the agent receives the endpoint's JSON response directly.

## Acceptance criteria

1. Button creates a discussion-only session (no repo writes); it appears in the index discussions list and reopens in the terminal.
2. Prime prompt contains discussion-only instructions and the exact scaffold call (base URL + session ID injected).
3. `POST /changes/scaffold` with `{title, prefix, session}` produces a spec-compliant change with the given title/prefix, renames the session, moves the mapping; `validate` stays clean.
4. Full live flow: discussion → approved trigger → same session refines the scaffolded change; board shows it live.
5. Placeholder machinery and old-flow tests removed; `go test ./...` green.

## Tasks

1. [SCF-00: Unassigned bucket in mapping + discussions list endpoint](tasks/00-unassigned-bucket.md)
2. [SCF-01: Discussion-session endpoint and prime prompt](tasks/01-discussion-endpoint.md)
3. [SCF-02: Scaffold trigger endpoint](tasks/02-scaffold-endpoint.md)
4. [SCF-03: Index discussions UI](tasks/03-discussions-ui.md)
5. [SCF-04: Remove placeholder machinery, rewrite tests](tasks/04-cleanup-tests.md)
6. [SCF-05: Dogfood full discussion flow](tasks/05-dogfood.md)
