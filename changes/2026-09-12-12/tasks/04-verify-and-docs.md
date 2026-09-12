---
id: NAV-04
title: End-to-end verification and README update
---

# NAV-04: End-to-end verification and README update

Status: see [../ledger.md](../ledger.md).

## Objective

Bring the docs in line with the UI changes and verify the whole change
end-to-end against a freshly built binary.

## Dependencies

- NAV-00
- NAV-01
- NAV-02
- NAV-03

## Scope

- `README.md` updates for the header menu, list ordering, and session
  shortcut.
- Full verification pass of every acceptance criterion in `plan.md`.

## Implementation steps

1. Update `README.md`: the board/usage sections should mention the top menu
   with active highlight, newest-first change list, and the board's
   Continue/Start session button.
2. Run `go vet ./...` and `go test ./...` from the repo root.
3. Rebuild `tasktracker` (`CGO_ENABLED=0 go build -o tasktracker
   ./cmd/tasktracker`), restart the server, and walk `/`, a board page, and
   `/explorer` against the plan's acceptance criteria (nav highlight, flat
   bell, list order, Continue button — including the no-sessions variant).
4. Record verification evidence in the ledger and each task file.

## Verification

- Every acceptance criterion in `plan.md` is confirmed; `tasktracker validate`
  reports the `changes/` tree clean.

## Completion criteria

- README matches behavior; all checks green; evidence recorded in the ledger;
  change reported ready for the user to close.

## Files affected

- `README.md`
- `changes/2026-09-12-12/ledger.md` (evidence)
- task files' Notes (evidence)

## Notes

- Remember: a stale server process holding the port can serve old HTML —
  restart before concluding an edit did not take.
- Verified 2026-09-12: README "The board" + opencode sections updated;
  `go vet ./...`, `go test ./...`, fresh build, and `tasktracker validate`
  all green; final live walk on the rebuilt binary confirmed newest-first
  index, active nav on all three pages, flat bell markup (hidden at zero
  findings), and the dual-mode button. All acceptance criteria in plan.md
  confirmed.
- Caveat learned: `pkill -f "tasktracker serve --port N"` matches the calling
  shell's own cmdline and kills it — use a pidfile for the temp server.
- Follow-up for the user: the long-running `:9090` server still runs the old
  binary in memory; restart it to pick up this UI.
