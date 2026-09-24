# UI-03: Redesign the changes list as mobile cards with tap-to-resume

## Why

The changes page is a 7-column sortable table with no mobile adaptation — the main pain point. The
user specified the card contents exactly: change name, task count, status, date. Nothing more. The
board is going away, so tapping a change must enter it: resume the change's session in the chat
overlay.

## Current behavior

- `index.html` renders `.change-table` (ID link, Title, Prefix, Status, Tasks, Updated, Plan
  button); `initIndexSort` sorts on `data-*` attributes per row, persisted in `tt-index-sort`;
  render tests assert the attribute names and `statusRank` pipe.
- `scheduleRefresh` full-reloads the index on `changes/` SSE events when no session overlay is
  open; with a chat open it only follows scaffolds (`followSession` → board) and leaves the list
  stale.

## Target behavior

- Replace the table with a card list. Each card shows exactly: title (primary line), status pill,
  task count as `x/y` (complete/total), updated date. The change ID survives only as `data-change`
  / href material, never as visible text.
- Cards keep sortable metadata as element attributes (`data-tasks`, `data-status-rank`,
  `data-updated`); sorting moves to a compact control (select or segmented row) reusing the
  `tt-index-sort` storage and default (newest first); render tests updated to the new DOM.
- Tapping a card resumes the change's session in the chat overlay via UI-02's
  `resumeChangeSession` (no page navigation).
- Header ("New change session", "Commit all") and the Discussions list get the same mobile
  treatment (wrapping, ≥44px targets).
- SSE: on `changes/` events while the index is visible, re-render the card list in place from the
  existing index JSON (`GET /{$}` → `changeSummary` rows) instead of `location.reload()` — an open
  chat is never interrupted; the scaffold-follow behavior keeps working but its destination
  changes to the in-place flow (final wiring lands in UI-04).

## Verification

- At 390×844 the list shows cards with exactly the four fields, no horizontal scroll; sort control
  works and persists across reloads; corrupt stored sort still fails open.
- Tapping a card opens the change's session in chat (continue/start semantics as UI-02).
- With a chat open, an SSE `changes/` event updates the cards without reloading the page or
  closing the chat.
- `go vet ./... && go test ./...` pass; index render tests assert the new card attributes.
