
# NTD-07: README and end-to-end verification

Status: see [../ledger.md](../ledger.md).

## Objective

Document the feature for humans and prove the change end-to-end against its acceptance criteria.

## Dependencies

NTD-00, NTD-01, NTD-02, NTD-03, NTD-04, NTD-05, NTD-06

## Scope

- `README.md` and the final verification pass. No new behavior.

## Implementation steps

1. README: nested task concept (expand/decompose, containers, dotted IDs), board drill-down and badges, session auto-spawn behavior and manual retry, governance rule (user-instructed decomposition), recursive close-out.
2. Rebuild the binary (`CGO_ENABLED=0 go build -o lessmess ./cmd/lessmess`).
3. Full suite: `go vet ./... && go test ./...`.
4. `lessmess validate` on this repository — clean.
5. End-to-end manual pass on a real or fixture nested change mirroring the plan's walkthrough: expand → auto-spawn → subtask lifecycle → rollup propagation → recursive close-out; verify a flat legacy change is untouched.
6. Record verification evidence in this task's Notes and the ledger.

## Verification

- All commands green; each acceptance criterion in [../plan.md](../plan.md) checked off with evidence.

## Completion criteria

README current; every acceptance criterion verified and documented; change ready for user review (`Test` hand-off).

## Files affected

- `README.md`

## Notes

- 2026-09-16 verification evidence: `go vet ./...` clean; `go test ./...` all packages green; drift test green after the AGENTS.md regeneration; `lessmess validate` exit 0 on this repository and on the smoke repo.
- End-to-end smoke against the real binary (temp repo, server on :9191): `POST /expand` created `tasks/00-alpha/`; two `POST /tasks {parent}` produced `SMK-00.00`/`SMK-00.01`; drill-down board JSON scoped to children with `progressTotal: 2`; move of `SMK-00.00` wrote only the container ledger; close returned 422 with the open subtask and 200 after completing the tree; final validate clean; auto-spawn degraded silently with no opencode service (best-effort by design).
