
# REN-01: Rename cmd/tasktracker to cmd/lessmess and rebuild binary

Status: see [../ledger.md](../ledger.md).

## Objective

Move the CLI entrypoint to `cmd/lessmess` and make the documented build produce a `lessmess` binary.

## Dependencies

- REN-00 (module path must be renamed first so the moved package builds).

## Scope

- `mv cmd/tasktracker cmd/lessmess`; the folder's doc pair (`STRUCTURE.md`, `AGENTS.md`) moves along untouched (gardener reconciles marker content at close).
- Update the build command anywhere it is executable guidance in code comments (`cmd/lessmess/main.go` if present).
- Build `CGO_ENABLED=0 go build -o lessmess ./cmd/lessmess`.
- Remove the stale root `tasktracker` binary — only after the new build succeeds (the running :9090 process keeps its inode; relaunch happens in REN-04).

## Implementation steps

1. `mv cmd/tasktracker cmd/lessmess`.
2. Fix any path references to `cmd/tasktracker` in live code/comments (excluding `changes/` history and test fixtures that build their own trees — see Notes).
3. Build the new binary at the repo root.
4. Smoke-run `./lessmess` (usage output) to confirm the binary works.
5. `rm` the old root `tasktracker` binary.

## Verification

- `ls cmd/` shows only `lessmess`.
- Fresh build succeeds; `./lessmess` prints usage mentioning `lessmess` (full usage-text rebrand lands in REN-03; this task only needs a working binary).
- Root `tasktracker` binary gone; `lessmess` present.

## Completion criteria

- Entrypoint lives at `cmd/lessmess`; new binary builds and runs; old binary removed.

## Files affected

- `cmd/tasktracker/**` → `cmd/lessmess/**` (including `main.go`, `STRUCTURE.md`, `AGENTS.md`).
- Root build artifacts: remove `tasktracker`, add `lessmess`.

## Notes

- `internal/server/docsqueue_test.go` references `cmd/tasktracker` in self-contained fixture data; evaluate whether it still passes (fixtures build their own temp trees) and adjust only if the rename actually breaks it.
- Verified 2026-09-12: `cmd/lessmess/` holds main.go + doc pair; `go vet` OK; `CGO_ENABLED=0 go build -o lessmess ./cmd/lessmess` OK; `./lessmess` prints usage; old root `tasktracker` binary removed while :9090 kept serving (inode). docsqueue_test fixture left as-is (self-contained, still green).
