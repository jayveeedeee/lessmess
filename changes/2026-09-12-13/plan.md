# 2026-09-12-13: Fix gitignore swallowing cmd/tasktracker source

- Change ID: 2026-09-12-13
- Created: 2026-09-12
- Branch: main
- Status: see [ledger.md](ledger.md)

## Objective and context

The `.gitignore` pattern `tasktracker` (intended for the root build artifact) matches any path component named `tasktracker`, which silently ignores the `cmd/tasktracker/` source directory. `git log` confirms `cmd/tasktracker/main.go` has never been tracked in any commit; new files there are silently skipped too (`check-ignore` verified).

## Current behavior

- `git check-ignore cmd/tasktracker` → ignored by pattern `tasktracker`.
- `git ls-files cmd/` → only `AGENTS.md`, `STRUCTURE.md`; `main.go` untracked since repository init.
- macOS `.DS_Store` not covered.

## Target behavior

- Root binary ignored via anchored `/tasktracker`; `cmd/tasktracker/` fully visible to git.
- `cmd/tasktracker/main.go` restored to the index.
- `.DS_Store` ignored.

## Scope

- `.gitignore`: anchor the artifact pattern, add `.DS_Store`.
- Re-add `cmd/tasktracker/main.go` to the index.
- No other content changes.

## Acceptance criteria

1. `git check-ignore cmd/tasktracker` reports not ignored; `/tasktracker` (root) still ignored.
2. `git status` shows `cmd/tasktracker/main.go` as a new tracked file.
3. `tasktracker validate` OK; tests unaffected.

## Tasks

1. [IGN-00: Anchor artifact pattern and restore main.go tracking](tasks/00-fix-gitignore.md)
