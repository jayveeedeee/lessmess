---
id: SCF-00
title: Unassigned bucket in mapping + discussions list endpoint
---

# SCF-00: Unassigned bucket in mapping + discussions list endpoint

Status: see [../ledger.md](../ledger.md).

## Objective

Extend the session mapping with an `_unassigned` bucket for pre-scaffold discussion sessions, plus a list endpoint the index can render.

## Dependencies

None.

## Scope

In scope: mapping `addUnassigned`, `moveToChange(session, changeID)`; `GET /api/discussions` returning unassigned entries enriched live like change sessions.
Out of scope: the discussion/scaffold endpoints (SCF-01/02), UI (SCF-03).

## Implementation steps

1. Mapping: add entries under the reserved `_unassigned` key; `moveToChange` relocates an entry from `_unassigned` to a change ID (no-op-safe if absent).
2. Endpoint: list unassigned with live title enrichment (same pattern as `listChangeSessions`).
3. Tests: bucket add/move/persist; endpoint output.

## Verification

- Mapping + endpoint tests pass.

## Completion criteria

- Unassigned sessions persist, move correctly, and list via the endpoint.

## Files affected

- `internal/server/mapping.go` (+ tests)

## Notes

- None yet.
