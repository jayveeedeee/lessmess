---
id: MTOC-03
title: In-modal .md link interception
---

# MTOC-03: In-modal .md link interception

Status: see [../ledger.md](../ledger.md).

## Objective

Relative `.md` links inside the modal open the referenced document in the
same modal — like clicking a kanban card — instead of navigating to a 404.

## Dependencies

- MTOC-00 (ledger endpoint exists for `ledger.md` targets).

## Scope

- `web/static/app.js` — delegated click handler on `#detail`.

## Implementation steps

1. Delegate a click listener on `#detail` for `a[href$=".md"]` with relative
   hrefs (no scheme/host); `preventDefault` and map:
   - `tasks/<file>.md` → `GET <board>/tasks/<file>`,
   - `plan.md` → `GET /changes/<id>/plan`,
   - `ledger.md` or `../ledger.md` → `GET /changes/<id>/ledger`,
   - `YYYY-MM-DD-N/plan.md` (leading change-id segment) → that change's plan.
   The current change id comes from `location.pathname` on board pages.
2. Fetch with `{headers:{Accept:"text/html"}}`, set
   `#detail.innerHTML = html` (same pattern as `refreshExplorerDetail`),
   then run `buildDetailTOC()` and, if the original href carried a
   `#fragment`, scroll to that heading.
3. Absolute URLs and non-`.md` links are left untouched.
4. Guard against a change-id segment that doesn't exist: let the fetch 404
   fall through to the existing error toast rather than navigating.

## Verification

- From the plan modal: a task link opens that task's modal (identical to
  clicking the kanban card); from a task modal: `../ledger.md` opens the
  change ledger; from the ledger: `plan.md` and task links work.
- Open another change's plan via a `YYYY-MM-DD-N/plan.md` link if a root
  ledger is reachable in a modal.
- Backdrop click, ✕ button, and Escape still close the modal after an
  in-modal navigation.

## Completion criteria

- All four link shapes open in-modal; no browser navigation; modal chrome
  keeps working after swaps.

## Files affected

- `web/static/app.js`.

## Notes

- The close handlers are already delegated at document level, so innerHTML
  swaps preserve them (no re-binding needed).
- Verification evidence (2026-09-15): `node --check app.js` clean; all four
  mapped endpoints answer 200 on the restarted server (plan, ledger, task,
  cross-change plan). Actual click behavior needs the user's browser pass
  (part of MTOC-04 handoff).
