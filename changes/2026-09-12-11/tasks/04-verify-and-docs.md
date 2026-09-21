
# EXD-04: Verification, tests, and README

Status: see [../ledger.md](../ledger.md).

## Objective

Prove the redesign end to end: server tests for the new endpoint, full build
and test suite, workflow validation, README update, and a manual UI pass.

## Dependencies

- EXD-02, EXD-03 (the complete implementation)

## Scope

- `internal/server` tests for `/explorer/detail` (if not fully covered in
  EXD-00).
- `README.md` "Project explorer" section.
- Full verification pass from the repo root.

## Implementation steps

1. Ensure tests cover: fragment 200 for a covered dir (contains purpose and
   blurb), 422 for uncovered/missing dirs, 503 when docs are disabled.
2. `CGO_ENABLED=0 go build -o tasktracker ./cmd/tasktracker`.
3. `go vet ./... && go test ./...`.
4. `./tasktracker validate` (no new findings).
5. Update README's "Project explorer" section to describe the master/detail
   layout (dirs-only tree, detail pane, selection, live refresh).
6. Manual UI pass against the running server: initial load, selection,
   guide lines, chat from both panes, live refresh after editing a
   STRUCTURE.md, disabled-state page (temporarily rename `agentsdocs.json`
   or point `--dir` at a repo without one).
7. Do NOT hand-edit marker-guarded sections of STRUCTURE.md/AGENTS.md — the
   doc gardener refreshes those at close.

## Verification

- All commands above pass; manual pass confirms every acceptance criterion in
  [../plan.md](../plan.md).

## Completion criteria

- Tests green, validate clean, README accurate, plan acceptance criteria met;
  ledger updated with evidence.

## Files affected

- `internal/server/explorer_test.go` (or nearest existing test file)
- `README.md`

## Notes

- Restarted the running server before UI checks (stale processes serve old
  templates, per templates/AGENTS.md); rebuilt binary is live on :9090.
- Verified 2026-09-12:
  - `CGO_ENABLED=0 go build -o tasktracker ./cmd/tasktracker` OK
  - `go vet ./...` clean, `go test ./...` all packages OK
  - `./tasktracker validate` → OK
  - Live UI pass (headless Chrome): split layout, guide lines, selection,
    chat buttons in both panes, narrow-screen stacking
  - Disabled-state page checked against a temp repo without
    `agentsdocs.json` (renders init/seed guidance)
- No hand edits to marker-guarded doc sections; the gardener refreshes those
  at close.
