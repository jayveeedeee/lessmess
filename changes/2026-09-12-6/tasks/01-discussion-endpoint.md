---
id: SCF-01
title: Discussion-session endpoint and prime prompt
---

# SCF-01: Discussion-session endpoint and prime prompt

Status: see [../ledger.md](../ledger.md).

## Objective

Repurpose `POST /changes/session` to create a discussion-only opencode session: no scaffold, new prime prompt carrying discussion-only instructions plus the exact scaffold API call (base URL + session ID injected), mapped to `_unassigned`.

## Dependencies

SCF-00.

## Scope

In scope: endpoint rewrite, new `discussionPrompt(apiBase, sessionID)` builder, unassigned mapping write.
Out of scope: scaffold endpoint (SCF-02), UI (SCF-03), placeholder removal (SCF-04).

## Implementation steps

1. Server needs its public base URL for prompt injection: capture `host:port` from `serve` flags into the `Server` (field set from main.go).
2. `discussionPrompt`: planning-assistant role; discuss objective/scope/design; DO NOT modify the repo; on explicit user approval choose title + 2–4-letter prefix and call `curl -s -X POST <base>/changes/scaffold -H 'Content-Type: application/json' -d '{"title":...,"prefix":...,"session":"<id>"}'`; then refine plan/tasks per AGENTS.md, keeping the user in the loop.
3. `POST /changes/session`: create session (`title` = "New change discussion" or caller-provided), map under `_unassigned`, return it.
4. Tests: prompt contains discussion-only language, the exact curl line with the injected base URL and session ID; mapping lands in `_unassigned`.

## Verification

- Endpoint + prompt-content tests pass.

## Completion criteria

- One call produces an unassigned discussion session with a fully-injected prompt.

## Files affected

- `internal/server/changesession.go`, `internal/server/server.go`, `cmd/tasktracker/main.go` (+ tests)

## Notes

- None yet.
