---
id: MTOC-04
title: End-to-end verification and checks
---

# MTOC-04: End-to-end verification and checks

Status: see [../ledger.md](../ledger.md).

## Objective

Verify the whole change on the rebuilt binary against the plan's acceptance
criteria, and confirm the repo-wide checks stay green.

## Dependencies

- MTOC-00, MTOC-01, MTOC-02, MTOC-03.

## Scope

- Full rebuild, restart, browser click-through, and repo checks. No feature
  code except small fixes discovered during verification.

## Implementation steps

1. `go vet ./... && go test ./...` from the repo root.
2. Rebuild: `CGO_ENABLED=0 go build -o lessmess ./cmd/lessmess`; restart the
   server (embedded assets ship in the binary).
3. Browser click-through on a long-plan change and a short-task change:
   TOC visibility rules, independent scrolling, 1px separator, scroll-spy,
   click-to-scroll, widened modal with TOC.
4. Link matrix: plan → task, task → ledger, ledger → plan, ledger → task,
   cross-change plan link; confirm no browser navigation and no 404s.
5. Regression sweep: notif modal, commit modal, terminal + its task panel,
   Escape/backdrop/✕ closing, board live updates (`htmx:afterSwap` on
   `#board` still refreshes the terminal task panel).

## Verification

- Each acceptance criterion in [../plan.md](../plan.md) checked one by one;
  results recorded in this task's Notes.

## Completion criteria

- All acceptance criteria pass; checks green; any regressions found are
  fixed or converted into new tasks.

## Files affected

- None expected (fixes only if verification finds gaps).

## Notes

- Remember the z-index ladder: `#detail` (40) sits above the terminal
  overlay (30); verify the modal still opens over an active terminal session.
- Verification evidence (2026-09-15): `go vet ./...` clean; `go test ./...`
  green across all packages; `lessmess validate` OK; binary rebuilt and the
  9090 server restarted. Server-rendered parts of the acceptance criteria
  (panes, heading ids, ledger table, endpoint) verified via curl. The
  browser click-through (TOC scrolling, scroll-spy, link matrix, modals over
  the terminal) is the remaining user acceptance step — the change is
  ready for `Test` → `Done` review.
