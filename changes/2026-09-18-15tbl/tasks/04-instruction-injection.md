
# JSI-04: Instruction modules and injection engine

Status: see [../ledger.md](../ledger.md).

## Objective

Replace the hardcoded session prompt bodies with versioned instruction modules embedded in the binary as JSON, selected deterministically per session kind and change/task state, rendered into primes together with a state snapshot from the JSON store.

## Dependencies

JSI-02 (state snapshot source)

## Scope

- Embedded module assets: `discussion`, `change.session`, `task.session`, `task.planning`, `decomposition`, `worktree`, `handoff`, `closeout`, `gardener`, `explorer`, `commit`, `repoCommit` — each with `id`, `version`, `audience`, optional `conditions`, `text` with `{{placeholder}}` substitution (`apiBase`, `changeId`, `taskId`, `taskHref`, `sessionId`, …).
- Selection function `f(session kind, change state, task state)` returning an ordered module set; deterministic and total (every kind covered; e.g., `worktree` only for worktree-backed changes, `decomposition`/`closeout` included in task-session payloads).
- Prime renderer: binding header + state snapshot (change/task from JSON: scope, status, deps, notes summary) + modules + existing settings addenda (`promptWith` mechanism unchanged).
- Migrate `discussionPrompt`, `changePrompt`, `taskPrompt`, `gardenerPrompt`, `explorerPrompt`, `commitPrompt`, `repoCommitPrompt` and `withWorktreeRule` to module composition; delete the hardcoded bodies.
- Auditability: log the injected module IDs at spawn and record them on the session mapping entry; `GET /workflow/instructions` serves the current module manifest (IDs, versions, audiences, rendered example).
- Modules must be self-contained (no cross-module references) — enforced by review checklist and fixture tests.

## Implementation steps

1. Author the module JSON texts from the current prompt bodies and the still-relevant parts of the AGENTS.md workflow contract (dropping everything the API now enforces).
2. Implement selection + rendering with placeholder substitution and escaping.
3. Wire into every spawn path (discussion, change bind, task bind, autosession, handoff, gardener, explorer, commit, repoCommit).
4. Add audit logging, mapping fields, and the instructions endpoint.
5. Tests: selection table pinned per session kind/state combination; rendering snapshots for each prompt type; spawn-path tests updated to the new primes (including the discussion step-0 empty-state pin).

## Verification

`go vet ./... && go test ./...` green; manual: spawn each session type on the migrated repo and inspect primes — correct module subset, no ledger-schema instructions anywhere, state snapshot accurate.

## Completion criteria

No hardcoded prompt bodies remain; injection is deterministic, auditable, and test-pinned; every spawn path uses the engine.

## Files affected

- `internal/server/` (changesession.go, mapping.go, docssession.go, explorer.go, gitcommit.go, lifecycle.go, new instructions endpoint, tests), new embedded assets location (e.g., `internal/server/assets/instructions/` or equivalent)

## Notes

Keep module texts concise; the contract is now "call the API". Record the final module list and any placeholders added beyond the plan here.
