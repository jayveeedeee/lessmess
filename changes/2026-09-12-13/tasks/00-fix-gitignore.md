
# IGN-00: Anchor artifact pattern and restore main.go tracking

Status: see [../ledger.md](../ledger.md).

## Objective

Fix `.gitignore` so only the root binary is ignored (not `cmd/tasktracker/`), restore `main.go` to the index, and cover `.DS_Store`.

## Dependencies

None.

## Scope

In scope: `.gitignore` edits, index restore, verification.
Out of scope: committing (user decision).

## Implementation steps

1. Change `tasktracker` → `/tasktracker` in `.gitignore`; append `.DS_Store`.
2. `git add cmd/tasktracker/main.go` (and cmd/tasktracker/ tree if needed).
3. Verify: `check-ignore` negative for the source dir, positive for the root binary; status shows main.go; validate OK.

## Verification

- All plan acceptance criteria pass.

## Completion criteria

- main.go staged and `check-ignore` correct on both paths.

## Files affected

- `.gitignore`, index entry for `cmd/tasktracker/main.go`

## Notes

- Verified (2026-09-12): `git check-ignore cmd/tasktracker` → not ignored; root `tasktracker` still matched by `/tasktracker`; `cmd/tasktracker/main.go` staged (`A`) along with `AGENTS.md`/`STRUCTURE.md` (docs files hidden by the same bug); `tasktracker validate` OK; tests untouched.
- Root cause detail: the bare pattern existed since the first `git add -A`, so `main.go` was absent from every prior commit (`git log --follow` empty). No history rewrite — the file simply starts being tracked now.

