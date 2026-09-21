
# DG-02: Rebuild, restart, reconcile the live stale backlog

Status: tracked in the tool-owned JSON state (.lessmess/workflow/).

## Objective

Prove the fix in the served repository: rebuild the binary (the web layer and Go changes are embedded/compiled — invisible to a running server until restart), restart `lessmess serve`, and drain the real backlog — all ten covered dirs have been queue-stale since the 02:21 manual-chunk failure, and the 10:32 close-out of `2026-09-17-39avm` may have added more.

## Dependencies

DG-00, DG-01 (code complete, tests green).

## Scope

- Build + restart of the served process on this repository.
- `POST /docs/refresh` and observation of the docs queue.
- No source changes (verification only); doc updates land via the gardener itself.

## Implementation steps

1. Build: `CGO_ENABLED=0 go build -o lessmess ./cmd/lessmess` from the repo root; confirm `go vet ./... && go test ./...` green first.
2. Restart the server on this repo (same `--dir`); confirm it is serving.
3. Check the current queue state (`.lessmess/docs-queue.json` and the log) — jobs enqueued by earlier closes may still be pending or have re-failed; let any in-flight job finish or fail first.
4. `POST /docs/refresh`; confirm the enqueued manual jobs chunk at ≤3 dirs.
5. Watch the log for `docs job done` lines (each job now has minutes proportional to its size — expect the first results within ~20–40 min) and confirm `docs-queue.json`'s stale map empties and `pending` drains.
6. Confirm the doc pairs were actually updated: spot-check a refreshed `STRUCTURE.md` meta stamp and `AGENTS.md` learnings on a previously stale dir.
7. Optional (operational, no code): if the flash model is still slower than desired, set `docs.gardenerModel` to a faster model.

## Verification

- Log shows refresh jobs completing (`docs job done`), no new `wait gardener ... context deadline exceeded` at the old 15:00 mark.
- `.lessmess/docs-queue.json`: `pending: []`, `stale: {}`.
- `lessmess validate` clean (no new docs warnings beyond pre-existing hash-staleness, which the refresh itself clears).

## Completion criteria

The live queue drains to empty with chunked, budget-scaled jobs; the previously stale dirs carry fresh doc pairs.

## Files affected

- None in source; `STRUCTURE.md`/`AGENTS.md` doc pairs under covered dirs are updated by the gardener sessions themselves.

## Notes

- Closing THIS change will itself enqueue gardener jobs for `internal/server` (and ancestors) — that close is the final live-fire exercise of the new chunking and budget.
