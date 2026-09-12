---
id: DOC-05
title: Close-out hook and serialized refresh queue
---

# DOC-05: Close-out hook and serialized refresh queue

Status: see [../ledger.md](../ledger.md).

## Objective

React to change close-out: compute the touched-folder set from the change's task
files ("Files affected"), map onto covered dirs, and enqueue a serialized,
persistent gardener job — with stale-flag fallback when execution is impossible and
strict recursion exemption. No-op when `agentsdocs.json` is absent.

## Dependencies

- DOC-00 (doc file format)
- DOC-01 (coverage config)

## Scope

- Hook in `closeChange` (`internal/server/lifecycle.go`) after the status flip;
  failures here must not fail the close itself (log + stale-flag instead).
- Touched-folder computation: parse "Files affected" sections of the change's task
  files; resolve to repo-relative dirs; map through DOC-01 matcher to covered dirs
  (a touched file outside covered dirs bubbles to its nearest covered ancestor).
- Persistent queue in `.tasktracker/docs-queue.json` (FIFO, one active job —
  single-writer discipline); stale set in `.tasktracker/docs-stale.json`.
- Queue drain on server start (recover after restart) and after each job completes.
- Recursion exemption: gardener-originated writes never enqueue; close of a change
  that only touched doc files enqueues nothing.
- `POST /docs/refresh` manual endpoint: reconcile stale dirs now (used by agents or
  the UI).

## Implementation steps

1. `internal/server/docsqueue.go`: queue/stale persistence (atomic writes),
  enqueue/drain lifecycle, serialization (single goroutine worker).
2. Touched-set extraction helper (+ tests over fixture change dirs).
3. Wire into `closeChange` behind config-presence gate.
4. `POST /docs/refresh` endpoint + stale reconciliation path.
5. Tests (httptest): close → queue entry with correct dir set; service-down →
  stale flags; restart → drain resumes; doc-only change → no enqueue; no config →
  no-op.

## Verification

- `go test ./internal/server` passes with new cases.
- Existing close behavior tests unmodified and green (hook is purely additive).

## Completion criteria

- Close of a change touching covered dirs reliably produces exactly one queued job
  with the correct folder set.
- Failure paths degrade to stale flags, never to lost or duplicated work.

## Files affected

- `internal/server/docsqueue.go` (new)
- `internal/server/docsqueue_test.go` (new)
- `internal/server/lifecycle.go` (hook)
- `internal/server/server.go` (route wiring)
- `.tasktracker/docs-queue.json`, `docs-stale.json` (runtime, gitignored)

## Notes

- Job payload: change ID, title, touched covered dirs, enqueued-at timestamp —
  enough for the gardener prompt (DOC-06) without re-deriving.
- 2026-09-12 — Implemented in `internal/server/docsqueue.go` + `touched.go`,
  hooked into `closeChange`, `POST /docs/refresh` added. Decisions:
  - ONE state file `.tasktracker/docs-queue.json` holds pending + stale
    (deviation from this file's two-file sketch): one atomic write keeps them
    consistent. Same gitignored tooling-state location.
  - The worker drains immediately; a missing runner fails the job straight to
    stale — that IS the service-down fallback, no special casing.
  - Touched-set heuristic: first backtick-stripped token of each bullet in
    "## Files affected"; prose/URLs dropped; paths under changes/, hidden
    paths, and AGENTS.md/STRUCTURE.md are excluded (recursion exemption — a
    doc-only change enqueues nothing, proven by test). Mapping to covered dirs
    is config-based (nearest covered ancestor), deliberately not
    existence-based; the runner (DOC-06) must tolerate dirs that no longer
    exist.
  - Hook runs after the status flip, is best-effort, and can never fail the
    close (logs only). `POST /docs/refresh` enqueues one "manual" job covering
    the whole stale set. `Server.Close` stops the worker.
  - Server creates the queue only when `agentsdocs.json` loads — zero behavior
    change for unconfigured repos (proven by test).
- Verification evidence: `go test ./internal/server -count=1` — touched-set
  extraction (bubbling, excludes, dir paths, prose), close → exactly one job
  with correct identity/dirs, close without runner → stale + persisted state,
  refresh → manual reconciliation job → stale cleared, doc-only change →
  nothing, disabled-without-config (close unaffected, refresh reports
  disabled), pending-state restore across "restart" then drain when a runner
  appears. Existing close/lifecycle tests untouched and green; full suite
  green; `gofmt`/`go vet` clean.
