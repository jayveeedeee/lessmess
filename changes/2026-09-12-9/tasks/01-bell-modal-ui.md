---
id: REF-01
title: Notification bell, modal, banner re-scope
---

# REF-01: Notification bell, modal, banner re-scope

Status: see [../ledger.md](../ledger.md).

## Objective

Move docs findings out of the red banner into a header notification bell with
a badge, opening a modal that lists findings grouped by severity and hosts the
refresh button with busy/result states.

## Dependencies

- REF-00 (union endpoint behind the button)

## Scope

- Header bell button (🔔) + badge: hidden at zero findings; amber normally,
  red when any error-severity finding exists.
- Modal: grouped findings list (docs errors, docs warnings), refresh button
  (`Refresh stale docs` → disabled `Refreshing…` → result note), close
  affordances (✕, backdrop click, Esc).
- Banner: violations only.
- app.js: cache latest `/api/validate` payload; re-render badge + modal content
  on each check and on docs SSE events; modal fetch-on-open.
- CSS consistent with existing modal patterns.

## Implementation steps

1. `layout.html`: bell button + badge + modal skeleton.
2. app.js: findings cache, badge renderer, modal open/close + list render,
   refresh button handler (POST /docs/refresh, busy state, result text,
   re-check on docs SSE), banner re-scope.
3. app.css: bell/badge/modal styles (reuse modal conventions).
4. Manual render checks; JS behavior verified in dogfood (REF-02).

## Verification

- Modal shows this repo's current findings; button hits the union endpoint;
  badge updates after reconciliation without reload (dogfood evidence).

## Completion criteria

- Acceptance criteria 1, 2, 4 in plan.md demonstrated; button wiring works
  against REF-00.

## Files affected

- `web/templates/layout.html`
- `web/static/app.js`
- `web/static/app.css`

## Notes

- 2026-09-12 — Implemented: bell + badge in `layout.html` header (hidden at
  zero, amber default, `.err` red on error findings), modal with grouped
  findings and the refresh button (busy state, result text, backdrop/✕/Esc
  close). `checkValidation` now scopes the banner to violations only and caches
  docs findings for badge+modal; docs SSE events dispatch `tt:docs-event`
  (re-enables the refresh button), re-run `checkValidation`, and refresh the
  explorer tree on that page. z-index 25 keeps the modal under the terminal
  overlay (30).
- Verification evidence: markup smoke-checked via curl (bell, badge, modal,
  button all render). Badge/modal behavior verified live in REF-02's run: 6
  findings → refresh enqueued → job done → findings 0 with no reload (SSE
  re-check). `go vet` clean; full suite green.