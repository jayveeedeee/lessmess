---
id: DSC-00
title: Add empty-state rule to discussion prompt
---

# DSC-00: Add empty-state rule to discussion prompt

Status: see [../ledger.md](../ledger.md).

## Objective

Make a freshly created discussion session wait for the user's request instead of investigating the repository: reword `discussionPrompt` in `internal/server/changesession.go` to add the empty-state rule pinned in [../plan.md](../plan.md).

## Dependencies

None.

## Scope

- The body and doc comment of `discussionPrompt` only.
- New assertions in `TestDiscussionSession` (`internal/server/changesession_test.go`).
- The authoritative wording is the plan's "Authoritative prompt text" block — copy it verbatim (with the existing `%[1]s`/`%[2]s` format verbs).

## Implementation steps

1. Replace the `discussionPrompt` return string with the authoritative text: new intro sentence ("This session exists to plan a NEW change…"), step 0 empty-state rule, then steps 1–4 unchanged.
2. Keep these pinned substrings intact: "planning assistant", "DO NOT modify the repository", "EXPLICITLY agrees", "task-ID prefix", the exact `curl -s -X POST %[1]s/changes/scaffold …` line, and step 1's "Ask questions; help them decide".
3. Extend the assertion list in `TestDiscussionSession` with: "plan a NEW change", "Empty state", "do NOT investigate the repository", and `What would you like to build?`.
4. Run `go vet ./... && go test ./...` from the repo root.

## Verification

- `go vet ./... && go test ./...` passes.
- `TestDiscussionSession` fails against the old prompt text and passes against the new (spot-check by assertion content).
- `settingswiring_test.go` and `settingschange_test.go` pass unchanged.

## Completion criteria

- Prompt text matches the plan's authoritative block byte-for-byte (modulo Go formatting of the raw string).
- All tests green.

## Files affected

- `internal/server/changesession.go`
- `internal/server/changesession_test.go`

## Notes

- Do not touch `lessmess.json` or any other prompt builder — the Settings Change flow's "(nothing below it)" condition is what keeps it engaging immediately.
