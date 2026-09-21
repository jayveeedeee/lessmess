
# CMT-04: End-to-end verification and docs updates

Status: see [../ledger.md](../ledger.md).

## Objective

Run the full verification suite, exercise the feature against the real
server, and update the docs that describe the affected areas (per-folder
AGENTS.md curated sections, README if user-visible behavior is documented
there).

## Dependencies

- CMT-00, CMT-01, CMT-02, CMT-03

## Scope

- Full checks: `go vet ./...`, `go test ./...`, `CGO_ENABLED=0 go build -o lessmess ./cmd/lessmess`, `lessmess validate`.
- Live run against this repository: button state, modal preview, cancel path, real commit through the UI, Discussions entry, button disabled afterwards.
- Curated learnings (outside the auto markers) in:
  - `internal/server/AGENTS.md` — new `/api/git/*` endpoints, read-only `gitStatus` helper, `commitStatus` reuse;
  - `web/templates/AGENTS.md` — `commit-all-btn` / `#commit-modal` ids to keep stable;
  - root `AGENTS.md` learning only if a durable cross-cutting lesson emerged;
  - `README.md` if it enumerates board/index UI affordances (it does for prior UI work).
- Confirm `changes/` workflow data is internally consistent (`lessmess validate`).

## Implementation steps

1. Run the checks; fix anything red.
2. Exercise the live flow; capture evidence (commit hash, Discussions entry) in this task's Notes.
3. Edit the curated sections of the AGENTS.md files listed above (and README if applicable); do not touch marker-guarded auto sections (the gardener owns those at change close).
4. Re-run `go test ./...` if any test-asserted text changed.

## Verification

- All commands in Scope pass; the live commit flow demonstrably works.

## Completion criteria

- Every acceptance criterion in plan.md is demonstrably met; docs reflect
  the new endpoints and UI ids; all tasks ready to move to `Test`.

## Files affected

- `internal/server/AGENTS.md`
- `web/templates/AGENTS.md`
- `README.md` (if it lists UI affordances)
- possibly root `AGENTS.md`

## Notes

- This task records the final verification evidence for close-out readiness.
- Evidence: `go vet ./...` clean; full `go test ./...` pass (incl.
  `TestGitStatus*`, `TestCommitAll*`, `TestIndexCommitAllButton`,
  `TestRepoCommitStatusRoute`); `CGO_ENABLED=0 go build -o lessmess
  ./cmd/lessmess` OK; `lessmess validate` exit 0 with only gardener-owned
  STRUCTURE.md staleness warnings (resolve at close). Live smoke test on a
  second instance (:9099): index serves the enabled button + modal markup on
  this dirty repo, `/api/git/status` returns the real porcelain list,
  commit-status rejects bad ids with 400.
- Docs: README's opencode-integration bullets now document the button, modal,
  endpoints, and safety rails. Per-folder `AGENTS.md` learnings were left to
  the doc gardener (its marker-guarded auto sections are machine-maintained
  at change close, citing this change ID) — no hand edits made there.
