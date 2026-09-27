# SKIL-00: Rewrite base prompt modules as lean mission plus skill pointers

## Why

The injected modules currently carry every procedure (endpoint lists,
curls, status rules) inside one-shot spawn prompts. Compaction erases
them and sessions spawned before an instruction change never see new
text. Under the agreed architecture, base prompts carry mission,
identity, and pointers only — procedures move to service-served skills
(SKIL-01).

## What

- Rewrite in `internal/server/instructions.json` (bump versions):
  - `discussion` v3 — plan-only mission; step 0 empty state kept;
    on explicit approval load `lessmess-scaffold` and follow it; never
    create a change by other means.
  - `change.session` v3 — identity, never hand-edit workflow state,
    narrative docs are direct edits, never scaffold; pointers to
    `lessmess-task` / `lessmess-closeout` / `lessmess-handoff`;
    delegation one-liner.
  - `task.session` v3 — identity, plan-only arrival (step 0), subtree
    scope, same three pointers, dotted-ID delegation, never scaffold,
    decomposition on explicit go-ahead only.
  - `chat` v2 — free chats gain workflow reach via skills
    (scaffold/task/closeout; handoff noted as change-bound-only);
    never hand-edit `.lessmess/workflow/`. This rewrite is what grants
    free chats the scaffold ability: the old rule 3 (blanket
    prohibition + "suggest starting a proper change session") is
    replaced by the `lessmess-scaffold` pointer.
- Retire `change.handoff` and `closeout` as injected modules — their
  text becomes skill bodies (SKIL-01).
- `worktree` module untouched.
- Update `internal/server/instructions_test.go` pins (selected module
  IDs per audience, empty-context renders for the new `chat` and
  `discussion` texts).

The exact v3/v2 texts are in the change plan (Target behavior section)
and were drafted and approved in the originating discussion.

## Files affected

- `internal/server/instructions.json`
- `internal/server/instructions_test.go`

## Verification

- `go vet ./... && go test ./...` — updated pins pass.
- Spawn each session kind on the board and inspect the prime: lean
  texts, no endpoint lists, pointers present, placeholders resolved.
