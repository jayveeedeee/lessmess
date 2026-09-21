
# DG-00: Chunk close-out jobs at ≤3 dirs with deduped ancestors

Status: tracked in the tool-owned JSON state (.lessmess/workflow/).

## Objective

Stop close-outs from creating one giant gardener job: `enqueueDocsRefresh` must split the touched dirs into chunks of at most 3 and enqueue one job per chunk, so a timeout flags and reverts only that chunk instead of the whole covered tree. This is the close-out-side fix for the 2026-09-19 incident, where a whole-repo change close produced a single job whose 15-minute failure marked all ten covered dirs stale at once.

## Dependencies

—

## Scope

- `internal/server/docsqueue.go`: `enqueueDocsRefresh`, `docsJobMaxDirs`.
- `internal/server/docsqueue_test.go`: new close-out chunking test; retuned fixtures.

## Implementation steps

1. Change `docsJobMaxDirs` from 10 to 3 and update its comment: it now bounds BOTH the manual refresh union and close-out jobs (the ≤3 size reflects the observed ~5 min/dir gardening pace; 10-dir chunks provably cannot finish inside a sane budget).
2. In `enqueueDocsRefresh`, after computing `dirs`:
   - For each chunk from `chunkDirs(dirs, docsJobMaxDirs)` (consecutive slices — deterministic order), compute `coveredAncestors(chunk, cfg)`.
   - Dedupe across chunks with a `seen` set: a chunk's `Ancestors` exclude any dir already claimed as an earlier chunk's Dir or Ancestor; after computing, add the chunk's Dirs and Ancestors to `seen`. Each directory is therefore updated or reviewed exactly once, ancestors landing in the earliest chunk that touches their subtree.
   - Enqueue one `DocsJob` per chunk with `Change`, `Title`, `Enqueued` as today.
3. Keep the guard order intact: `s.docsQ == nil` and empty-`dirs` still return without enqueuing (an empty-dirs change enqueues nothing — `TestCloseDocOnlyChangeEnqueuesNothing` must stay green).
4. `docsRefresh` needs no structural change — it inherits the new chunk size through the constant.

## Verification

- New test `TestCloseEnqueuesChunkedJobs` (follow the `TestCloseEnqueuesAndRunsJob` fixture pattern): close a change touching more than 3 dirs across nested subtrees; assert multiple jobs were enqueued, each with ≤3 `Dirs`, no dir appearing as Dir or Ancestor in more than one job, and the union of all jobs' `Dirs` equal to the touched set.
- Retune `TestDocsRefreshChunksLargeUnions` for chunks of 3.
- Existing queue tests stay green (small close-outs still produce exactly one job).
- `go vet ./... && go test ./...` green.

## Completion criteria

A close-out can never enqueue a job with more than 3 Dirs, and a failed job's stale set is bounded to one chunk plus its (deduped) ancestors.

## Files affected

- `internal/server/docsqueue.go`
- `internal/server/docsqueue_test.go`

## Notes

- The serialized queue drains the chunks sequentially, so a repo-wide change takes longer in total but each unit is small and independently recoverable — the intended trade.
- If multi-dir sessions still bust scaled budgets after this change, the fallback design is per-dir sessions (seeder pattern); explicitly out of scope here.
