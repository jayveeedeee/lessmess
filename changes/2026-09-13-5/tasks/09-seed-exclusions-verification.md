
# LRN-09: README and verification for seed/exclusions

Status: see [../ledger.md](../ledger.md).

## Objective

Document the seed-continuation and exclusions surfaces, and verify the
added scope end-to-end.

## Dependencies

- LRN-07, LRN-08.

## Scope

- README only, plus the live verification pass for LRN-07/08.

## Implementation steps

1. README usage section: `POST /docs/seed`, `GET /docs/seed-status`,
   `GET/POST /docs/exclusions`; the bell seed button; the Settings
   exclusions widget; a note that CLI/wizard/server seed runs share one
   resumable cursor.
2. Gates: vet, test, build, `lessmess validate`.
3. Live E2E on a scratch repo: partial seed (budget) → pending count in
   `/api/validate` → `POST /docs/seed` continues (no re-run of
   summarized dirs) → pending reaches 0; exclusion POST → GET round-trip
   → `agentsdocs.json` diff shows replaced patterns; seed honors the
   saved exclusion.

## Verification

- All gates green; live matrix evidence recorded in the ledger notes.

## Completion criteria

README matches shipped behavior; the seed/exclusions scope passes the
same bar as the rest of the change.

## Files affected

- `README.md`
- `changes/2026-09-13-5/` (evidence in ledger notes)

## Notes

- Live runs use a scratch repo; the running :9090 server keeps the old
  binary until the user restarts it.
