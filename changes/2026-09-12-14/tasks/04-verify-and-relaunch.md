---
id: REN-04
title: Full verification and relaunch on :9090
---

# REN-04: Full verification and relaunch on :9090

Status: see [../ledger.md](../ledger.md).

## Objective

Prove the renamed app builds, passes all checks, migrates existing state, and serves the board on :9090 as `lessmess`.

## Dependencies

- REN-01, REN-02, REN-03.

## Scope

- `go vet ./...`, `go test ./...`.
- Fresh `CGO_ENABLED=0 go build -o lessmess ./cmd/lessmess`.
- `./lessmess validate` against this repo.
- Stop the current :9090 `tasktracker serve` process; start `lessmess serve` on :9090.
- Confirm auto-migration ran (`.lessmess/` exists with `sessions.json`, `docs-queue.json`, `docs-seed.json`; `.tasktracker/` gone) and sessions survived (Sessions panel / Discussions list populated).
- Confirm the UI brand shows lessmess (header brand and page title).

## Implementation steps

1. Run vet and tests; fix any stragglers from earlier tasks.
2. Rebuild the binary fresh.
3. Run `./lessmess validate`; expect no violations.
4. Identify the :9090 process, stop it, start `lessmess serve --port 9090` (detached), and confirm it answers.
5. Verify migration on disk and session visibility via the UI/API.
6. Fetch `/` and check the brand text and `<title>`.

## Verification

- All commands pass; board live on :9090 from the `lessmess` binary; state migrated; sessions listed; brand visible.

## Completion criteria

- Every acceptance criterion in `plan.md` is evidenced (command outputs and observations recorded in Notes).

## Files affected

- None new (verification only); build artifact `lessmess` refreshed.

## Notes

- Restart caveat: the relaunched server runs via `nohup` from this session, logging to the opencode temp dir (`lessmess-serve.log`); the user can stop/restart it like the old one.
- This session's own change binding lives in the opencode service, not the tasktracker server, so the restart does not interrupt the conversation.
- Verified 2026-09-12: vet+test green; fresh build; `./lessmess validate` rc=0 (4 docs-stale warnings expected — trees changed by the rename; gardener reconciles; a manual `/docs/refresh` was also fired, rc=202). Old PID 48375 (`./tasktracker serve --port 9090`) stopped; migration renamed `.tasktracker/` → `.lessmess/` with sessions.json byte-identical (diffed); new server logs "validation ok" + opencode connected; `GET /changes/2026-09-12-14/sessions` returns this session live; brand confirmed in `<title>` and brand link on index and board.
