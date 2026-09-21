
# LRN-06: README, package docs, and full verification

Status: see [../ledger.md](../ledger.md).

## Objective

Human-facing docs describe the new behaviors, and the whole change
passes its gates plus a live end-to-end pass.

## Dependencies

- LRN-01, LRN-02 (ancestor behavior to document)
- LRN-04 (refresh union to document)
- LRN-05 (setting to document)

## Scope

- `README.md` only, plus execution of the full verification matrix.
- Package `AGENTS.md` learnings are updated by the doc gardener at
  close-out (automatic) — not hand-edited here.

## Implementation steps

1. README docs-management section: ancestor review-and-fix on close;
   stale-reference lint findings in the bell and `lessmess validate`;
   the extended refresh union; manual jobs naming flagged refs.
2. README settings table: `docs.gardenerModel` row (fallback chain,
   validation, badge behavior).
3. Run gates: `go vet ./...`, `go test ./...`,
   `CGO_ENABLED=0 go build -o lessmess ./cmd/lessmess`,
   `lessmess validate`.
4. Live E2E evidence (record in task notes):
   - Learning citing a deleted path → warning appears; refresh routes a
     manual job whose prompt names the ref; post-job the warning clears.
   - Close a scratch change whose tasks list a removed file → job
     prompt contains both sections; ancestors snapshotted/verified.
   - Set `docs.gardenerModel` (personal layer) → next docs job session
     created with it (job log line); clear → session model used.

## Verification

- All gates exit 0; `lessmess validate` reports no new errors.
- E2E matrix above executed against the live binary and opencode
  service, with outcomes recorded in Notes.

## Completion criteria

README matches shipped behavior; gates green; E2E evidence recorded;
the change is ready for user acceptance.

## Files affected

- `README.md`
- `changes/2026-09-13-5/` (verification evidence in ledger/task notes)

## Notes

- If a stale-STRUCTURE.md warning for covered dirs appears mid-change,
  it reconciles at close per the standard flow — not a gate failure.
