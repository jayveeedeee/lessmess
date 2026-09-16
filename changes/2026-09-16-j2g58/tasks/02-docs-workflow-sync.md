---
id: STS-02
title: Workflow wording, embedded asset, and README
---

# STS-02: Workflow wording, embedded asset, and README

Status: see [../ledger.md](../ledger.md).

## Objective

Point agents and humans at the deterministic path in the docs, and keep the
embedded copy of the workflow text in sync.

## Dependencies

STS-00 (documenting an endpoint that exists).

## Scope

- AGENTS.md status-workflow rule 3 (and related prose) states that overall-status
  changes go through the board control or `POST /changes/{id}/status`, not
  hand-edits; the root-ledger note about keeping rows in sync stays accurate.
- `internal/docs/assets/workflow_agents.md` regenerated from AGENTS.md (the awk
  recipe in AGENTS.md's learnings) so the embed drift test passes.
- `README.md` documents the endpoint and the board control.

## Implementation steps

1. Edit the workflow text in AGENTS.md above the `tasktracker:begin` marker:
   rule 3 becomes "set the overall status via the board control or
   `POST /changes/{id}/status` (Done only via close)"; keep it short and
   convention-shaped.
2. Regenerate the embedded asset exactly per the pinned recipe:
   `awk '/^<!-- tasktracker:begin/{exit} {print}' AGENTS.md > internal/docs/assets/workflow_agents.md`.
3. Run the docs drift test to confirm embed and file agree.
4. Update README with the endpoint (request/response shape, accepted statuses,
   Done → 409 guidance) and one line about the board control.

## Verification

- `go test ./...` green, including the workflow-asset drift test.
- `grep` confirms AGENTS.md and the embedded asset contain the new wording.
- `lessmess validate` clean.

## Completion criteria

- Agents reading AGENTS.md are told the deterministic path for every overall
  transition; embed and README agree with the shipped behavior.

## Files affected

- `AGENTS.md`
- `internal/docs/assets/workflow_agents.md`
- `README.md`

## Notes

- Do not edit anything inside the `tasktracker:begin`/`tasktracker:end` markers.
- The workflow text is embedded at build time, so the binary needs a rebuild
  before serve-time behavior reflects the new wording.
