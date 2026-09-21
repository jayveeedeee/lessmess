
# UIX-00: Remove plain new-change form from index template

Status: see [../ledger.md](../ledger.md).

## Objective

Remove the plain "New change" form from the index page so only the "New change session" form remains.

## Dependencies

None.

## Scope

In scope: `web/templates/index.html` form removal.
Out of scope: the `POST /changes` endpoint, any other templates.

## Implementation steps

1. Delete the plain form block from `web/templates/index.html`.
2. Build, run tests, screenshot the index page.

## Verification

- Index HTML contains no form posting to `/changes` (plain) and exactly one form posting to `/changes/session`.
- `go test ./...` passes; screenshot confirms one form.

## Completion criteria

- Acceptance criteria in [../plan.md](../plan.md) met.

## Files affected

- `web/templates/index.html`

## Notes

- Verification (2026-09-12): served index HTML contains exactly one form (`hx-post="/changes/session"`); `go test ./...` green; `tasktracker validate` OK; headless screenshot shows the single orange form. An earlier check appeared to show both forms — traced to a stale process holding port 9090 (the recurring zombie-port pattern), not stale code; a fresh serve confirmed one form.

