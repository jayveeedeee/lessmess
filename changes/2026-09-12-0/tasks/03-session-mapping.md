---
id: OCI-03
title: Session mapping store and endpoints
---

# OCI-03: Session mapping store and endpoints

Status: see [../ledger.md](../ledger.md).

## Objective

Persist the change→sessions mapping in `.tasktracker/sessions.json` and expose it via JSON endpoints.

## Dependencies

OCI-01 (session creation needed by the create endpoint).

## Scope

In scope: mapping file (atomic writes, schema: changeID → [{session, title, created}]), endpoints `GET /changes/{id}/sessions` (mapping enriched with live session titles via the client), `POST /changes/{id}/sessions` (create opencode session for the repo + map + return it), `DELETE /changes/{id}/sessions/{sessionID}` (unlink only; does not delete the opencode session). `.tasktracker/` created on demand; stays gitignored.
Out of scope: terminal UI (OCI-04).

## Implementation steps

1. Mapping store type with load/save (atomic via `model.WriteFileAtomic`), tolerant of missing/corrupt file (corrupt → error surfaced, never silently wiped).
2. Handlers with store/client wiring; enrichment errors degrade gracefully (mapping shows IDs if live lookup fails).
3. Tempdir tests: persistence, corrupt file, enrichment fallback, endpoint behavior.

## Verification

- `go test ./internal/server` (or mapping package tests) passes.

## Completion criteria

- Mapping CRUD works end-to-end via endpoints with persistence across restarts.

## Files affected

- `internal/server/` or small new package for the mapping store

## Notes

- (fill in during execution)
