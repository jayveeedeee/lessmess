
# DG-01: Scale the per-job gardener budget with dir count

Status: tracked in the tool-owned JSON state (.lessmess/workflow/).

## Objective

Replace the fixed 15-minute job budget with one that tracks the session's real workload: 15 minutes base plus 5 minutes per target dir (`len(Dirs) + len(Ancestors)`). On 2026-09-19 three consecutive multi-dir jobs died at exactly 15:00 with `wait gardener: ... context deadline exceeded` while the session was still legitimately busy — the gardener model needs on the order of five minutes per directory.

## Dependencies

DG-00 (chunking first keeps the per-job budget arithmetic meaningful and the tests stable).

## Scope

- `internal/server/docsqueue.go`: new `jobTimeout`, `run`.
- `internal/server/docsqueue_test.go`: new tests.

## Implementation steps

1. Add named constants in `docsqueue.go` (with a comment citing the 2026-09-19 incident): `docsJobBaseTimeout = 15 * time.Minute`, `docsJobPerDirTimeout = 5 * time.Minute`.
2. Add `jobTimeout(job DocsJob) time.Duration` returning base + per-dir × (len(Dirs)+len(Ancestors)) — the session gardens both lists.
3. In `run`, replace `context.WithTimeout(context.Background(), 15*time.Minute)` with `jobTimeout(job)`; leave everything else (runner nil-check → stale, failure → `failStale`, success → stale-clear) untouched.
4. Do not touch `WaitDone`'s retry logic or the gardener session code — the budget is the only knob being changed.

## Verification

- Table test for `jobTimeout`: 1 dir → 20 min; 3 dirs + 2 ancestors → 40 min; 0 dirs (manual empty) → base.
- End-to-end: a fake `DocsRunner` capturing `ctx.Deadline()` (pattern: enqueue a job with known dir counts, assert deadline−start equals the expected budget).
- Full suite and `go vet ./...` green.

## Completion criteria

A job's context deadline is a function of its size, proven by tests; small jobs keep today's 15-minute base.

## Files affected

- `internal/server/docsqueue.go`
- `internal/server/docsqueue_test.go`

## Notes

- A `docs.gardenerTimeoutMinutes` settings knob was considered and deferred (see plan non-goals): scaling removes the pressure and the knob would add four lockstep settings surfaces.
- The serialized queue means one long job still delays the next; with ≤3-dir chunks that worst case is ~30–40 min, acceptable for a background worker.
