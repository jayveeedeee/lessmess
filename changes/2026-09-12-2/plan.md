# 2026-09-12-2: Remove plain new-change form from index

- Change ID: 2026-09-12-2
- Created: 2026-09-12
- Branch: main
- Status: see [ledger.md](ledger.md)

## Objective and context

The index page offers two change-creation forms: plain "New change" (scaffold only) and "New change session" (scaffold + primed opencode session). The user wants only the opencode-driven one.

## Current behavior

Two side-by-side forms in the index section head; the plain form posts to `POST /changes`.

## Target behavior

A single form: "New change session" (`POST /changes/session`). The plain form is removed from the UI.

## Scope

- Remove the plain "New change" form from `web/templates/index.html`.
- Keep the `POST /changes` endpoint (used by API clients; not part of the UI).

## Non-goals

- Removing or changing the `POST /changes` endpoint or `store.CreateChange`.
- Any other UI changes.

## Acceptance criteria

1. Index page shows exactly one change-creation form ("New change session").
2. `go test ./...` passes; `tasktracker validate` clean.

## Tasks

1. [UIX-00: Remove plain new-change form from index template](tasks/00-remove-plain-form.md)
