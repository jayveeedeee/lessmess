
# SET-05: End-to-end verification and docs

Status: see [../ledger.md](../ledger.md).

## Objective

Verify the whole feature against the plan's acceptance criteria on a live
server, and leave plan/ledger/docs consistent.

## Dependencies

- SET-04

## Scope

- Full automated suite: `go vet ./... && go test ./...`;
  `lessmess validate` clean.
- Live smoke against a running server with the opencode service up:
  1. Save settings at project scope → `lessmess.json` appears in
     `git status`; at personal scope → `.lessmess/settings.json` written,
     still gitignored.
  2. Set a non-default agent/model; spawn a discussion session; confirm
     via `GET /api/session/{id}` (or the service) that the session uses
     them; set an intentionally invalid agent and confirm the fallback
     still creates the session with a warning logged.
  3. Add a prompt addendum; trigger the matching flow; confirm the
     appended text reaches the session; clear it and confirm byte-identical
     base prompt.
  4. Set `git.defaultBranch`, scaffold a change, confirm the root ledger
     Branch cell.
  5. Toggle gardener-off and close a sacrificial change → no docs job
     enqueued (`.lessmess/docs-queue.json` unchanged); toggle back.
  6. Toggle show-archived off → archived rows vanish from `/`; on → back.
  7. Malformed `lessmess.json` → server still serves, warning visible on
     the settings page; restore the file.
- Update plan.md / task notes with any deviations discovered; confirm
  README accuracy.

## Implementation steps

1. Run automated checks; fix anything red via the owning task.
2. Rebuild the binary, run the server, execute the smoke script above.
3. Record evidence in this task's Notes; sweep ledgers/plan for drift.

## Verification

- Every acceptance criterion in plan.md is checked off with evidence.

## Completion criteria

- Criteria 1–8 of plan.md verified; ledgers and plan agree with reality;
  change ready to report for close-out.

## Files affected

- `changes/2026-09-13-2/` (plan/task notes only)

## Notes

- Smoke steps that mutate the repo (sacrificial change, `lessmess.json`)
  must be cleaned up or folded into the final commit deliberately.
- Verified 2026-09-13 (throwaway repo at /tmp/lm-smoke + real opencode
  service; all artifacts removed afterwards):
  1. Project save wrote `lessmess.json` at the repo root (not gitignored);
     personal save wrote gitignored `.lessmess/settings.json`; personal won;
     clearing restored the project value.
  2. Spawned session carried agent `plan` (verified via the service's
     session info); configured model landed on the session
     (`accounts/fireworks/models/deepseek-v4p1-flash`).
  3. KEY FINDING: the service accepts unknown agents/models at creation
     without error and the session never runs (0 tokens, no messages) —
     the 400-retry fallback cannot fire. Fixed with save-time validation
     (SET-03 scope addition, see ledger decision log): unknown agent → 422,
     unknown model → 422 live; valid pair → 200.
  4. Discussion addendum reached the real session prompt verbatim with the
     base prompt intact (message log check).
  5. `git.defaultBranch` recorded by both create routes (handler tests).
  6. Gardener gate off → no job and no queue file; on → job runs
     (TestGardenerGateOnClose). showArchived off hides archived rows
     (TestIndexArchivedFilter).
  7. Malformed `lessmess.json` → defaults plus surfaced loadError
     (TestSettingsAPILoadErrorSurfaced); server stays up.
  8. `go vet ./...` and `go test ./...` pass; `lessmess validate` reports
     no `changes/` violations (root ledger row synced to In progress);
     STRUCTURE.md stale warnings remain BY DESIGN until the gardener runs
     at change close.
  9. `node --check web/static/app.js` clean; four smoke sessions deleted
     from the service; smoke repo removed.
- Deviations from plan are recorded in plan.md (save-time validation,
  datalist pickers) and the ledger decision log. Client-side dynamics
  (badges, datalists, offline hint, auto-open gate) follow verified
  endpoints; browser spot-check left for user acceptance.
