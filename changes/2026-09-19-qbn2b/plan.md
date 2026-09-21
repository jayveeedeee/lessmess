# 2026-09-19-qbn2b: Chunk and scale docs gardener jobs

- Change ID: 2026-09-19-qbn2b
- Created: 2026-09-19
- Branch: —
- Status: tracked in the tool-owned JSON state (.lessmess/workflow/)

## Objective and context

Every large doc-gardener job has been failing at exactly its hard 15-minute budget, flagging whole swaths of the covered tree stale. Evidence from `.lessmess/serve-9090.log` (2026-09-19): the close-outs of `2026-09-18-f52mn` (failed 01:13:15) and `2026-09-18-15tbl` (failed 01:28:15) and a manual 10-dir refresh chunk (failed 02:21:47) all died with `wait gardener: ... context deadline exceeded` precisely 15 minutes after starting, while the only job that ever succeeded — the single-dir `[web/templates]` reconciliation (done 02:31:48) — took roughly five minutes. The effective gardener model (`zai-coding-plan/glm-5.3-flash`, inherited from `session.model` because `docs.gardenerModel` is unset) needs on the order of five minutes per directory, so any multi-dir session busts a fixed 15-minute budget; the job then reverts its snapshots, marks every dir in `Dirs`+`Ancestors` stale, and a later `POST /docs/refresh` re-enqueues the same doomed chunk size. As of this writing all ten covered dirs sit queue-stale from the 02:21 failure.

Two structural causes, both in `internal/server/docsqueue.go`:

- the close-out hook `enqueueDocsRefresh` never chunks — exactly the "single giant gardener session whose failure flags every dir stale at once" that the manual path's `docsJobMaxDirs` comment warns about; and
- the serialized worker gives every job a constant `15*time.Minute` context regardless of how many directories the session must garden.

## Current behavior

- `enqueueDocsRefresh` enqueues ONE job per close-out: all touched covered dirs plus `coveredAncestors(dirs)`, unchunked.
- The worker's `run` gives every job a fixed 15-minute context; on `WaitDone` deadline the job fails, snapshots restore, and every dir in `Dirs`+`Ancestors` goes stale with the failure reason.
- `docsRefresh` (manual path) chunks its union at `docsJobMaxDirs = 10` — too large for the observed per-dir pace, and irrelevant to close-out jobs entirely.

## Target behavior

| Piece | Behavior |
| --- | --- |
| Close-out chunking | `enqueueDocsRefresh` splits the touched dirs into consecutive chunks of at most `docsJobMaxDirs` and enqueues one job per chunk; `coveredAncestors` is computed per chunk and deduped against dirs already claimed by earlier chunks, so each directory is updated or reviewed exactly once. |
| One chunk size | `docsJobMaxDirs` drops 10 → 3 for both the manual refresh and close-out paths. |
| Scaled budget | The worker's per-job context becomes `jobTimeout(job)`: 15 minutes base plus 5 minutes per target dir (`len(Dirs) + len(Ancestors)`), so the budget tracks the session's real workload. |
| Failure isolation | Unchanged mechanics, better bounds: a timeout still flags and reverts only that chunk's dirs, and the queue moves on to the next chunk instead of stalling the whole tree at once. |

## Scope

- `internal/server/docsqueue.go`: chunked `enqueueDocsRefresh` with ancestor dedupe, `docsJobMaxDirs` value, `jobTimeout` + worker use.
- `internal/server/docsqueue_test.go`: new and retuned tests.
- Rebuild/restart of the served binary and reconciliation of the live stale backlog.

## Non-goals

- A `docs.gardenerTimeoutMinutes` setting — the scaling removes the pressure, and a knob adds four lockstep surfaces (schema, `EffectiveSettings` merge, `settingsFieldValue` allowlist, settings API/UI). Follow-up if still wanted.
- Retrying failed jobs, splitting a timed-out job's remainder, or per-dir gardener sessions (the seeder's one-session-per-dir pattern) — chunking plus scaled budgets first; per-dir sessions remain the fallback design if multi-dir sessions still bust budgets.
- Touching `WaitDone`, the snapshot/verify/restore confinement machinery, or the stale/lint model.
- Switching the gardener model — operational, available any time via the existing `docs.gardenerModel` setting.

## Design decisions

1. **Chunk the close-out path like the manual path** rather than special-casing it: one shared `docsJobMaxDirs` keeps job semantics uniform and deletes the asymmetry that caused the incident.
2. **Chunk size 3, not 1** — single-dir sessions (the only proven successes) maximize robustness but multiply spawns and lose ancestor-review batching; 3 dirs plus deduped ancestors yields a ~30–40-minute budget at the observed pace, comfortably above the workload.
3. **The budget scales linearly on `Dirs`+`Ancestors`** because one session gardens both lists; the base stays 15 minutes so small jobs are unchanged. Named constants carry a comment documenting the 2026-09-19 timeouts.
4. **Ancestor dedupe across chunks, first-claim wins** — chunk order is deterministic (consecutive slices of the touched-dirs list), so each ancestor is reviewed exactly once, in the earliest chunk that touches its subtree.
5. **Stale-clear persistence folded in (during DG-02)** — `run()` cleared stale flags only in memory on success, so a restart resurrected them from disk and refreshes re-gardened already-fresh dirs (observed live: the 10:44 success of `2026-09-17-39avm` never reached `docs-queue.json`). One `persistLocked()` call under the same lock, pinned by an assertion in `TestDocsRefreshReconcilesStale`.

## Detailed implementation approach

1. DG-00 adds close-out chunking: reuse `chunkDirs`, track a `seen` set to skip any dir already claimed as an earlier chunk's Dir or Ancestor, drop `docsJobMaxDirs` to 3 and update its comment; `docsRefresh` inherits the new size through the constant alone.
2. DG-01 replaces the worker's fixed `15*time.Minute` with `jobTimeout(job)` (base + per-dir constants), tested both as a table test and end-to-end via a fake runner asserting the ctx deadline.
3. DG-02 rebuilds and restarts the served binary, reconciles the live stale backlog via `POST /docs/refresh`, and confirms the queue drains with jobs completing inside their budgets.

## File-level impact

- `internal/server/docsqueue.go` — `enqueueDocsRefresh`, `docsJobMaxDirs`, new `jobTimeout`, `run`.
- `internal/server/docsqueue_test.go` — new `TestCloseEnqueuesChunkedJobs`, new `TestJobTimeoutScalesWithDirs`, retuned `TestDocsRefreshChunksLargeUnions`, and any close-out fixture touching more than 3 dirs.

## Data, schema, and configuration changes

- None: `DocsJob` shape, `docs-queue.json` format, and settings are untouched.

## Safety, migration, and rollback

- Purely in-process scheduling changes with no persisted-format change; rollback is a rebuild of the previous binary. Stale entries already on disk reconcile through the unchanged `POST /docs/refresh` path.

## Testing and verification strategy

- Unit: chunked close-out enqueue (multiple jobs of ≤3 dirs, ancestor dedupe across jobs, union of Dirs preserved); `jobTimeout` table test; fake-runner ctx-deadline assertion; retuned manual-refresh chunk test.
- Repo: `go vet ./... && go test ./...` green.
- Live acceptance: after rebuild and restart, the current ten queue-stale dirs (stale since 02:21) clear via refresh jobs that complete rather than time out.

## Acceptance criteria

1. A close-out touching more than 3 dirs enqueues multiple jobs of ≤3 dirs each; no dir appears as Dir or Ancestor in more than one job; the union of Dirs equals the touched set.
2. A job's context deadline equals 15 minutes + 5 minutes × (len(Dirs)+len(Ancestors)), proven by a fake-runner deadline assertion and a table test.
3. `docsJobMaxDirs` is 3 for both enqueue paths.
4. `go vet ./... && go test ./...` green.
5. Live: the served binary is rebuilt and restarted; `POST /docs/refresh` enqueues chunks that complete (`docs job done` in the log, stale map empties).

## Ordered task breakdown

1. [DG-00 — Chunk close-out jobs at ≤3 dirs with deduped ancestors](tasks/00-chunk-close-out-jobs-at-3-dirs-with-dedu.md)
2. [DG-01 — Scale the per-job gardener budget with dir count](tasks/01-scale-the-per-job-gardener-budget-with-d.md)
3. [DG-02 — Rebuild, restart, reconcile the live stale backlog](tasks/02-rebuild-restart-reconcile-the-live-stale.md)
