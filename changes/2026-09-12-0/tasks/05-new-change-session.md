---
id: OCI-05
title: New change session flow
---

# OCI-05: New change session flow

Status: see [../ledger.md](../ledger.md).

## Objective

Index-page "New change session" action: scaffold the change, create and prime an opencode session, link it, and land the user on the new board with the terminal attached.

## Dependencies

OCI-04.

## Scope

In scope: index form (title, prefix); server flow orchestrating existing change scaffold + opencode session create + primed prompt ("Continue change `<id>`: `<title>` — follow AGENTS.md; refine plan and tasks as needed") + mapping; redirect to the new board with the terminal open.
Out of scope: agent-scaffolds-change variant (rejected in design), permissions config (OCI-06).

## Implementation steps

1. Endpoint `POST /changes/session` performing scaffold → session → prompt → map, returning the change ID (+ session ID).
2. Index form wired to it (htmx, redirect to board with `?session=<id>` to auto-open the terminal).
3. Error handling: scaffold succeeds but session create fails → surface error but keep the change (reported in UI), mapping untouched.
4. Tests for orchestration with fake client; manual e2e in OCI-07.

## Verification

- Flow test passes; e2e confirmed in dogfood (change appears, session primed, board live-updates).

## Completion criteria

- One click produces a scaffolded change with a primed, attached session.

## Files affected

- `internal/server/`, `web/templates/index.html`, `web/static/app.js`

## Notes

- (fill in during execution)
