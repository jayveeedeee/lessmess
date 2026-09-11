---
id: LIF-02
title: Commit-session endpoint
---

# LIF-02: Commit-session endpoint

Status: see [../ledger.md](../ledger.md).

## Objective

Implement `POST /changes/{id}/commit`: create an opencode session primed to review uncommitted changes, write a commit message, and `git commit` (no push); map it to the change; return it for terminal open.

## Dependencies

LIF-00 (LIF-01 independent).

## Scope

In scope: endpoint + prime prompt + mapping + tests.
Out of scope: UI button (LIF-03), server-side git (never).

## Implementation steps

1. Prompt: review `git status`/`git diff`, write a good commit message, `git add -A` (respecting `.gitignore`), `git commit`, never push/amend/branch; report the hash.
2. Endpoint: create session (`<id> — git commit`), prompt it, map, return IDs; 503 when the service is unavailable.
3. Tests: fake service asserts session create + prompt content; mapping persisted.

## Verification

- Endpoint tests pass.

## Completion criteria

- Endpoint creates, primes, and maps a commit session.

## Files affected

- `internal/server/` (+ tests)

## Notes

- None yet.
