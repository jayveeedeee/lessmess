---
id: HOF-02
title: README documentation and full validation pass
---

# HOF-02: README documentation and full validation pass

Status: see [../ledger.md](../ledger.md).

## Objective

Document the handoff workflow for humans and close the change with a full verification pass across the whole feature.

## Dependencies

HOF-00 and HOF-01 (documented and validated behavior must exist).

## Scope

- `README.md`: the spawn-change endpoint, the `handoff*.md` artifact convention, the agent-driven flow (change session writes the artifact, then calls the endpoint), and the board action.
- Full-suite validation and an end-to-end manual handoff on a scratch copy of the repo.

Out of scope: code changes beyond documentation fixes discovered by validation.

## Implementation steps

1. README: add a short "Change handoff" section (when to use, artifact convention, both flows, endpoint table row if the README tabulates endpoints).
2. Run `go vet ./... && go test ./...`.
3. Build and run against a scratch copy: perform one agent-driven and one board-driven handoff; run `lessmess validate` afterwards; confirm the new change validates and the spawned session is mapped.
4. Fix anything the pass uncovers (in HOF-00/01 scope, recorded here in notes).

## Verification

- README renders correctly and matches actual behavior (endpoint names, statuses, file conventions).
- `lessmess validate` clean; `go vet ./... && go test ./...` clean.

## Completion criteria

Docs accurate; whole change verified end-to-end; change ready for close-out review.

## Files affected

- `README.md`
- Possibly small fixes in files from HOF-00/01.

## Notes

- Per AGENTS.md the per-folder AGENTS.md learnings are refreshed by the docs gardener at close; no hand edits needed inside markers.
