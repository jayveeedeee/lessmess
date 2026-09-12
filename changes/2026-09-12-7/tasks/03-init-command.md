---
id: DOC-03
title: tasktracker init command
---

# DOC-03: tasktracker init command

Status: see [../ledger.md](../ledger.md).

## Objective

Add `tasktracker init [--dir .]`: bootstrap an uninitialized directory into a
workflow-ready repo — root `AGENTS.md` with the canonical change-management
instructions, `changes/` skeleton, `.gitignore` with `.tasktracker/`, starter
`opencode.json`, and default `agentsdocs.json` — merge-safe and idempotent, with no
git assumption.

## Dependencies

- DOC-00 (root AGENTS.md gets a marker-guarded auto section; merge semantics)
- DOC-01 (default config generator)

## Scope

- Embed the canonical workflow text as a binary asset (`go:embed`); add a drift test
  asserting this repo's root `AGENTS.md` contains it verbatim.
- Create `changes/ledger.md` skeleton matching the root-ledger schema.
- `.gitignore`: add `.tasktracker/` if a git repo or `.gitignore` exists/needs
  creating; never rewrite existing entries.
- Starter `opencode.json` only if none exists (copy of this repo's permission
  envelope, minus repo-specific bits if any).
- Default `agentsdocs.json` via DOC-01 generator, only if none exists.
- Root `AGENTS.md`: if absent, create with workflow text; if present, merge in a
  marker section pointing at the workflow — never overwrite human content.
- `--dir` flag consistent with `serve`/`validate`.

## Implementation steps

1. Add embedded asset (e.g. `internal/docs/assets/workflow_agents.md`) carrying the
   canonical instructions; add the drift-containment test.
2. `internal/docs/init.go`: ordered, individually merge-safe writers for each
   artifact, each reporting created/merged/skipped.
3. Wire `init` into `cmd/tasktracker/main.go`; print a summary of actions taken.
4. Tests in temp dirs: empty dir, pre-existing files (merge vs. skip), idempotent
   second run, resulting tree passes `tasktracker validate`.

## Verification

- `go test ./...` passes (new init tests + drift test).
- Manual: run `tasktracker init` in a scratch dir; `tasktracker validate --dir` on
  it passes; rerun shows all-skipped.

## Completion criteria

- Fresh dir becomes a valid workflow repo with one command.
- No existing file is ever clobbered (covered by tests).

## Files affected

- `internal/docs/init.go` (new)
- `internal/docs/assets/workflow_agents.md` (new embedded asset)
- `internal/docs/init_test.go` (new)
- `cmd/tasktracker/main.go` (subcommand wiring)

## Notes

- Decide where the embedded asset lives so the drift test can compare it against the
  repo-root `AGENTS.md` without import cycles; record the choice here.
- 2026-09-12 — Implemented in `internal/docs/init.go` + `assets/workflow_agents.md`,
  wired as `tasktracker init [--dir .]`. Decisions:
  - Asset lives at `internal/docs/assets/workflow_agents.md` (go:embed cannot reach
    the repo root). The drift test `TestWorkflowAssetDrift` asserts EXACT equality
    with the repo-root AGENTS.md; maintenance is `cp AGENTS.md
    internal/docs/assets/workflow_agents.md`. Note: DOC-08's AGENTS.md update must
    refresh this asset.
  - An existing AGENTS.md already containing the workflow text is skipped (this is
    what makes re-init idempotent); any other existing AGENTS.md gets the text
    appended in a marker-guarded auto section, so later inits can refresh it.
  - Starter `opencode.json` mirrors this repo's permission envelope verbatim.
  - No `--force` flag: merge covers refresh, skip preserves humans — clobbering is
    never needed.
  - Init reports per-artifact `created`/`merged`/`skipped` actions, printed by the
    CLI.
- Verification evidence: unit tests (`go test ./internal/docs -count=1`) cover
  empty-dir creation, validation of the initialized tree via `store.Validate`,
  second-run idempotence, human-AGENTS.md merge + head preservation + re-init skip,
  .gitignore merge/skip variants, existing-config preservation, and asset drift.
  Manual: `tasktracker init` in a scratch dir → 5 created; `tasktracker validate
  --dir` → OK; rerun → 5 skipped. `gofmt`/`go vet` clean; full suite green.
