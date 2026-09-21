
# DOC-06: Doc gardener session and confinement guard

Status: see [../ledger.md](../ledger.md).

## Objective

Execute queued jobs with an unattended opencode session ("doc gardener") that
updates the doc pairs for the job's folder set, then verify server-side that only
marker sections changed — reverting and logging on violation — and stamp freshness
metadata on success.

## Dependencies

- DOC-05 (queue to consume)

## Scope

- Gardener prompt builder: change ID + title + plan summary, touched folders,
  current doc contents, write constraints (only marker sections; learnings cite the
  change ID; concise blurbs; no other files).
- Session lifecycle via existing `internal/opencode` client: create, prime,
  `WaitDone` with generous timeout, best-effort cleanup/rename (`<change> — docs`).
- Pre-run snapshot of target files; post-run diff via DOC-00 parse: changes must be
  confined to auto sections of the intended files; violations → restore snapshot,
  log, flag stale.
- On success: stamp freshness metadata (date, change ID, tree hash via DOC-02
  hashing), clear stale flags for the job's dirs, mark job done.
- Graceful degradation: service unavailable mid-job → job back to pending + stale
  flags (DOC-05 fallback).

## Implementation steps

1. `internal/server/docssession.go`: prompt builder (pure function, table-tested)
   and job runner (snapshot → session → verify → finalize).
2. Integrate runner with the DOC-05 queue worker.
3. Tests with a fake opencode client (follow existing `server` test patterns):
   prompt content, happy-path finalize, marker-violation revert, session failure →
   stale, metadata stamping.

## Verification

- `go test ./internal/server` passes with new cases.
- Real-service behavior validated in DOC-08 dogfooding (record evidence there).

## Completion criteria

- A well-behaved gardener run ends with updated docs, stamped metadata, cleared
  stale flags.
- A misbehaving run changes nothing on disk permanently and is visibly logged.

## Files affected

- `internal/server/docssession.go` (new)
- `internal/server/docssession_test.go` (new)
- `internal/server/docsqueue.go` (runner integration)

## Notes

- Snapshot/restore must use the same atomic write path; restores are themselves
  exempt from re-triggering.
- 2026-09-12 — Implemented in `internal/server/docssession.go`; `SetOpencode`
  attaches the runner when docs are enabled. Decisions:
  - Runner flow: whole-tree deterministic skeleton refresh (changed trees
    stamped with the job's change ID) → snapshot job dirs → ONE gardener
    session per job covering its dir set → verify → settle rollups. Per-dir
    session splitting is a noted future refinement if quality suffers.
  - The skeleton refresh is deliberate whole-tree, not just job dirs:
    structure is machine-owned, and this is what tolerates deleted dirs and
    settles parent rollups after fresh purposes land (same pattern as seed's
    phase 3).
  - Confinement's teeth: bytes OUTSIDE markers must be identical to the
    snapshot (strict Prefix/Suffix compare), meta hash must match the walked
    tree hash, no snapshot file deleted, a created AGENTS.md must use markers.
    Auto-section mischief inside STRUCTURE.md self-heals via the settle pass,
    so it is not separately checked. Source-file protection relies on the
    opencode.json permission envelope + prompt, not the verifier — recorded as
    a known limit.
  - `Walk` does not compute hashes; the runner uses `docs.TreeHashes`
    (exported alongside `RefreshSkeletons` for DOC-07) rather than relying on
    `Dir.Hash` — a test caught the empty-hash false violation.
- Verification evidence: `go test ./internal/server -count=1` — prompt content
  (change ID citation requirement, constraints), happy path (placeholders
  annotated, meta stamped with change ID, learning with citation, rollup
  settle), outside-marker edit reverted with human bytes intact and skeleton
  surviving, meta tamper reverted, markerless created AGENTS.md removed,
  missing dirs tolerated without a session, session failure propagates. Full
  suite green; `gofmt`/`go vet` clean.
