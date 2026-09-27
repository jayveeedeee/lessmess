# SKIL-04: Verification pass and end-to-end smoke

## Why

The change touches agent-facing prompts, a new public endpoint family,
project config, and chat delivery. The pieces interlock: prompts point
at skills, skills come from the catalog, the catalog must be reachable
at the URL written into `opencode.json`, and the re-prime must not
disturb any of it. One integration pass before close.

## What

- Full gate: `go vet ./... && go test ./...`, `lessmess validate`.
- End-to-end smoke on the running server:
  1. Spawn a change discussion → prime is lean, no endpoint lists.
  2. Approve a plan in it → agent loads `lessmess-scaffold` and
     scaffolds successfully.
  3. In the new change session, create a task via `lessmess-task`;
     set it In progress; finish at Test with evidence.
  4. Queue a compaction on the session → next prompt carries the
     binding line + snapshot wrap.
  5. Free chat: skill listing shows the four `lessmess-*` skills;
     scaffold from it binds the session.
  6. `GET /workflow/instructions` reflects the new module versions.
- Update `README.md` if any user-visible surface changed beyond the
  `/skills/` endpoints already covered by SKIL-01.

## Files affected

- `README.md` (only if needed)

## Verification

- This task is the verification; its evidence is the smoke transcript
  summarized in the Test status evidence field.
