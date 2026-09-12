---
id: SCF-03
title: Index discussions UI
---

# SCF-03: Index discussions UI

Status: see [../ledger.md](../ledger.md).

## Objective

Show open discussions (unassigned sessions) on the index page with reopen-in-terminal, and make the "New change session" button open the terminal on the new discussion.

## Dependencies

SCF-01.

## Scope

In scope: index section listing `_unassigned` sessions (title, created, Open, Unlink), wired to `GET /api/discussions`; button click → create → openTerminal; unlink support.
Out of scope: board sessions panel (unchanged).

## Implementation steps

1. Index template: discussions section (list container) below the change table.
2. app.js: `loadDiscussions()` on index; create-on-click then `openTerminal(session, title)`; reopen on Open; unlink on ✕.
3. Terminal opening on the index page: ensure the terminal overlay exists/works there too (currently board-only — extend).
4. Screenshot of the index with a discussion listed.

## Verification

- Render/JS checks; screenshot; live reopen in dogfood (SCF-05).

## Completion criteria

- Discussions are visible and reopenable from the index.

## Files affected

- `web/templates/index.html`, `web/templates/board.html` (overlay sharing), `web/static/app.js`, `app.css`

## Notes

- None yet.
