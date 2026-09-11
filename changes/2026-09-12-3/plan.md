# 2026-09-12-3: Agent-driven change titling

- Change ID: 2026-09-12-3
- Created: 2026-09-12
- Branch: main
- Status: see [ledger.md](ledger.md)

## Objective and context

Creating a change session currently asks the user for a title and prefix up front. The user wants a single input-free button: the agent determines the real title and prefix from the conversation. A random placeholder is used at scaffold time and the agent updates it.

## Current behavior

Index form requires `title` (and optional `prefix`); the server scaffolds with it and primes the session with that title.

## Target behavior

- The index shows a single "New change session" button with no inputs.
- The server scaffolds with a random placeholder title (e.g. `untitled-ember-fox`) and empty (`—`) prefix, creates the session, and primes the agent to: learn the objective from the user, update the change title in `plan.md` and the root-ledger row, choose and register a task-ID prefix in the root ledger, rename the opencode session via `opencode2 api`, then refine plan/tasks per AGENTS.md.
- The API still accepts an explicit title for programmatic callers.

## Scope

- Server: placeholder generation; title optional in `POST /changes/session`; new prime prompt with titling/prefix/rename instructions.
- UI: remove the two inputs; button-only form.
- Tests updated (placeholder path + explicit-title path + prompt content).

## Non-goals

- Renaming flows beyond what the agent does itself.
- Validation of agent-chosen titles/prefixes (the normal validator already enforces schema).

## Acceptance criteria

1. One button, no inputs; clicking it produces a scaffolded change with a placeholder title and a primed session whose prompt contains the titling/prefix/rename instructions.
2. Explicit-title API path still works.
3. `go test ./...` passes; `tasktracker validate` clean.

## Tasks

1. [GEN-00: Placeholder titling flow and button-only UI](tasks/00-placeholder-titling.md)
