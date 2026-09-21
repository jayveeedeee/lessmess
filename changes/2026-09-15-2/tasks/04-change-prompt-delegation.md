
# PSB-04: changePrompt delegation and bind instructions

## Objective

Teach newly created change sessions the delegation convention: when to spawn a
subagent for a task and how to title it so the board can map it.

## Dependencies

- PSB-00 (spike results: prompt teaches description-prefixing; no bind curl).
- PSB-02 no longer gates this task's wording (the endpoint is user-facing, not
  agent-facing) but the task stays ordered after it for rollout simplicity.

## Scope

- `changePrompt` in `internal/server/changesession.go` and its tests. Nothing else.

## Implementation steps

1. Add a delegation section to `changePrompt` (spike-amended): default to doing
   work inline; when delegating, spawn the subagent via the task tool with the
   description prefixed by the task ID — `TSK-NN: slug` — because the child session
   title is exactly that description (verified by PSB-00) and the board's
   reconciler reads it. Do NOT include a bind curl: the parent model cannot know
   the child session ID. Add the capability steer: prefer a build-capable subagent
   when the user may continue the sub later.
2. Keep the wording consistent with the existing prime style (numbered rules,
   exact curl line like the scaffold instruction).
3. Preserve every phrasing the pinned tests assert (scaffold curl line, prefix
   rule, workflow constraints) — only add.
4. Update `TestChangeSession`-style prompt assertions to cover the new section.

## Verification

- `go test ./internal/server/` green, including the prompt-pin tests.
- A freshly created change session (dev server) receives the new prime.

## Completion criteria

- New sessions know the convention; old sessions unaffected; pins intact.

## Files affected

- `internal/server/changesession.go`, `internal/server/changesession_test.go`.

## Notes

- Prompt edits reach only sessions created after the binary rebuild — call that out
  in the change's rollout note so manual verification uses a fresh session.
- Policy is "inline first" (user decision, 2026-09-15): the prompt must present
  delegation as an option for tasks that benefit from it, never as mandatory. Also
  include the capability steer: when the user may later continue a sub directly,
  spawn it with a build-capable agent, not a restricted read-only one.
