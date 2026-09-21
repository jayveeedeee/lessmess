
# LRN-07: Resumable seed on the normal server

Status: see [../ledger.md](../ledger.md).

## Objective

"Continue seeding what's left" becomes a first-class server operation:
`POST /docs/seed` + `GET /docs/seed-status` on the normal server, sharing
the setup wizard's job registry and the same resumable cursor, with the
pending-dirs count surfaced through `/api/validate`.

## Dependencies

None.

## Scope

- Extract the seed-job starter from the setup handler so both surfaces
  share it (same registry, same cursor, same SessionDefaults summarizer).
- New routes on the normal server; `docs.PendingDirs` helper.
- No UI in this task (bell button is LRN-08).

## Implementation steps

1. `setupseed.go`: extract `startDocsSeedJob(dir string, cfg *docs.Config,
   oc *opencode.Client, budget int) bool` from `env.docsSeed` (dir must be
   absolute; keeps the onboarding `docs=seeded` mark on success). The
   setup handler delegates.
2. `internal/docs`: `PendingDirs(root) ([]string, error)` — covered dirs
   not marked summarized in the cursor, sorted; nil when disabled.
3. Normal-server handlers (`POST /docs/seed` with `{budget}` body, `GET
   /docs/seed-status`): 503 without opencode or coverage, 409 when
   running, 202 on start, 200 status.
4. `/api/validate` payload gains `docsSeedPending` (count; omitted on
   lookup error).

## Verification

- Seed route starts a job with the fake summarizer (cursor honored:
  summarized dirs skipped), 409 on double-start, 503 without oc/config,
  status endpoint reports running/done and lines.
- `PendingDirs` fixture: empty cursor → all covered dirs; after marking
  summarized → remainder.
- Validate payload carries the pending count.

## Completion criteria

A partial seed continues server-side with one call; all suites green.

## Files affected

- `internal/server/setupseed.go`, `internal/server/server.go`,
  `internal/server/docsseed.go` (new, normal-server handlers)
- `internal/docs/seed.go` (PendingDirs) + test
- `internal/server/setupseed_test.go`, `server_test.go`

## Notes

- The registry is keyed by absolute dir; absolutize `s.st.Dir` before
  lookup (service requirement too).
