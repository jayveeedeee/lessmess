---
id: WCV-04
title: Repo remediation, README, end-to-end verification
---

# WCV-04: Repo remediation, README, end-to-end verification

Status: see [../ledger.md](../ledger.md).

## Objective

Make the source repository itself consistent with the new delivery model, document it, and prove the change end-to-end with tevr-core as the acceptance environment.

## Dependencies

WCV-00, WCV-01, WCV-03

## Scope

- This repository's root `AGENTS.md`, `README.md`, final verification. No new behavior.

## Implementation steps

1. Strip the workflow rulebook from this repository's root `AGENTS.md` (everything above the `tasktracker:begin` markers), leaving markers + learnings — only after WCV-01 is live so board sessions stay self-sufficient. The now-stale "workflow text above the markers is drift-pinned" learning is corrected by the docs gardener at close.
2. README: canon versioning, injection delivery, the zero-touch guarantee, `lessmess workflow print`, and precedence over legacy texts.
3. Rebuild; `go vet ./... && go test ./...`; `lessmess validate` clean on this repository after the strip.
4. Fixture end-to-end: serve a repo whose `AGENTS.md` still carries an old rulebook; spawn a change session → the prime carries current canon vN and the repository file is untouched.
5. tevr-core acceptance (needs a session or user action on that repo): a spawned session is primed with the current rules; agent-created sub-plan ledgers conform; its legacy text stays untouched and inert. The two existing container headers still need manual normalization there.
6. Record evidence here and in the ledger.

## Verification

- All checks green; fixture and tevr-core outcomes recorded.

## Completion criteria

Source repository remediated; README current; injection proven on a fixture and on tevr-core, or the tevr-core half explicitly handed to its own session with instructions.

## Files affected

- `AGENTS.md`, `README.md`

## Notes

The tevr-core container-header fixes (`- Task: BCM-04 (change 2026-09-18-m23jf)` shape) remain that repository's follow-up, not this change's.
