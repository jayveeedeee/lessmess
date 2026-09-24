# MP-06: Integration verification and docs

## Goal

Prove the acceptance criteria end-to-end on a real two-project instance and leave the human-facing docs accurate.

## Approach

- End-to-end pass (record evidence for each):
  - Fixture registry with two real repos (this repo + a scratch fixture); `lessmess serve` (no `--dir`); both prefixes fully usable side-by-side in two browser tabs; SSE updates stay scoped per tab.
  - Add/remove via the landing page; removed project's repo untouched on disk; re-add works.
  - Agent-facing: spawn a session in project A, confirm its prime curls carry `/p/<slugA>/`; `POST /p/<slugA>/changes/scaffold` lands the change in A.
  - Setup mode: register a fresh non-repo dir → wizard under its prefix → bootstrap → full server swaps in without restart.
  - Legacy: `lessmess serve --dir` regression pass over the main flows.
  - Per-repo accents visibly differ per project; landing uses the default accent.
- `go vet ./...`, `go test ./...` (and `-race` on server/hub packages), `lessmess validate` on affected fixture repos.
- Docs: README — new serve modes, global config location/schema, UI management, prefix URL scheme, compatibility note for pre-upgrade agent sessions. Check `README.md` claims about "one process serves one repository" and correct them.

## Files affected

- `README.md`

## Verification

- Acceptance criteria in plan.md all demonstrated with recorded evidence (command output, screenshots where visual).
- `lessmess validate` clean for this repo.
