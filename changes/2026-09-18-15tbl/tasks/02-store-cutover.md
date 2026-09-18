---
id: JSI-02
title: Store cutover to JSON state
---

# JSI-02: Store cutover to JSON state

Status: see [../ledger.md](../ledger.md).

## Objective

Switch `internal/store` to treat `.lessmess/workflow/` JSON as the sole state source: open, reload, watch, validate, tree building, and all mutations operate on JSON, with prose md files referenced (not parsed) for bodies.

## Dependencies

JSI-00, JSI-01

## Scope

- `Open`/`Reload` load the JSON store (running the JSI-01 migration hook when md state is detected); `ErrNoChanges` semantics move to absence of `.lessmess/workflow/` (both fresh repo and partial/invalid state per current sentinel rules).
- `Watch` observes `.lessmess/workflow/` plus prose files/directories under `changes/`; keep debounce and CHMOD-ignoring behavior.
- Tree building (`tree.go`) constructs `TaskNode`s from JSON (`parent`/dotted IDs/file href), preserving `Node`/`WalkTasks`/`SubtreeStats`/`AllTaskStats` shapes for consumers.
- Mutations (`CreateChange`, `CreateTask`, `MoveTask`, `DecomposeTask`, `SetChangeStatus`) rewrite JSON atomically with the same re-read→apply→write conflict safety; overall status stays derived and is written by the tool only; array order is priority.
- `Validate` becomes JSON-integrity checking: enum membership, unique IDs, seq/filename agreement, dependency existence, prose-file existence, container-directory agreement, index↔state consistency.
- Worktree resolver applies to prose directories only; state is always main-tree.
- Port/adjust existing store tests; delete md-splice mutation paths no longer reachable.

## Implementation steps

1. Introduce the JSON-backed cache alongside the loader; swap `Open`/`Reload`.
2. Rework tree building and stats over JSON nodes.
3. Rework each mutation; keep sentinel errors (`ErrNotFound`, `ErrInvalid`) and ID-minting rules (random suffix + collision check) unchanged.
4. Rework `Validate` and the `lessmess validate` output.
5. Extend `Watch`; verify no reload loops on the committed JSON subtree.
6. Port the test suite; add JSON-specific cases (drift detection, missing prose file).

## Verification

`go vet ./... && go test ./...` green; on a migrated copy of this repo: board loads, task create/move/decompose/status flows work, external JSON edit triggers reload, `lessmess validate` reports seeded violations correctly.

## Completion criteria

Store operates entirely on JSON state with the existing consumer-facing API shapes preserved; md ledger code paths are gone from the store.

## Files affected

- `internal/store/` (store.go, tree.go, validate.go, overall.go, watch.go + tests), `cmd/lessmess/` (validate wiring)

## Notes

This is the cutover point: after it lands, old primes referencing md ledgers are stale until JSI-04 lands — keep the gap short. Record any consumer-shape changes needed by `internal/server` here.
