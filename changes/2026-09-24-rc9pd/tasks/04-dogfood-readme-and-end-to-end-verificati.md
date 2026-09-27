# EXP-04: Dogfood, README, and end-to-end verification

Status: see [../ledger.md](../ledger.md).

## Objective

Keep this repo's docs alive under the new default, document the changed
user-visible behavior, and run the full verification pass.

## Dependencies

EXP-00 through EXP-03 (documents the landed behavior; verification runs last).

## Scope

- `lessmess.json` (committed project layer, this repo): add
  `"docs": {"enabled": true}` so the local gardener/queue keeps running once
  the default flips to off.
- `README.md`: settings list gains `docs.enabled` (default off, experimental,
  restart-applied); the docs subsystem described as opt-in via Settings →
  Docs with the initialize-coverage flow; wizard description updated to 5
  steps with no docs step; `lessmess init` no longer lists
  `agentsdocs.json` among bootstrapped files; the `git.worktrees` mention
  noted as experimental.
- Root `AGENTS.md` workflow text: the docs-gating learning lives inside the
  machine-maintained markers — do not hand-edit; instead note the behavior
  change for the gardener (closing this change enqueues the refresh) and
  update only the human-facing prose if the gating sentence sits above the
  markers (it does not — verify before editing).
- `changes/2026-09-24-rc9pd/plan.md`: final pass so Current/Target behavior
  match what landed.
- Full verification: `go vet ./... && go test ./...`, `lessmess validate`,
  rebuild + restart, manual pass over the acceptance matrix.

Out of scope: any new feature work; fixes belong in EXP-00..03.

## Implementation steps

1. Set `"docs": {"enabled": true}` in `lessmess.json` and confirm the
   settings view shows project as the source.
2. Update README sections (settings, docs, wizard, init, worktrees).
3. Reconcile plan.md with the landed reality.
4. Rebuild (`CGO_ENABLED=0 go build -o lessmess ./cmd/lessmess`), restart,
   and walk the acceptance matrix from plan.md.

## Verification

- `go vet ./... && go test ./...` green; `lessmess validate` clean.
- This repo after restart: docs running (bell, gardener on close armed),
  Settings shows `docs.enabled` = project/true with Experimental badge.
- README renders the new behaviors; no stale claims about the wizard's docs
  step or init writing coverage.

## Completion criteria

- Docs stays on here, off elsewhere by default, and every user-visible
  surface (README, Settings, wizard, init) tells the same story.
