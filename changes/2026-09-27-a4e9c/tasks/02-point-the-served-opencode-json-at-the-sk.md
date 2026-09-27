# SKIL-02: Point the served opencode.json at the skills catalog

## Why

OpenCode only discovers catalog skills when the project's
`opencode.json` lists the catalog URL under `skills`. lessmess already
owns a surgical config-patch path (the Align button rewrites only
`default_agent`, order-preserving, JSONC refused —
`internal/server/opendefault.go`); the skills URL needs the same
treatment so the platform lights up without hand-editing.

## What

- Extend the config-patch machinery to add/refresh the `skills` array
  entry pointing at this server's catalog URL
  (`<apiBase>/skills/`, hub-prefix-aware via the same PublicBase logic
  that feeds `apiBase()` in `changesession.go`).
- Idempotent: an existing correct entry is left byte-identical; a stale
  URL is updated; user-added extra skills entries are preserved.
- Refuse JSONC like the Align path (non-plain JSON is reported, never
  rewritten).
- Wire the patch at server boot (and/or behind the existing settings
  surface, matching whichever pattern fits with least surprise —
  propose in the task PR description if ambiguous).

## Files affected

- `internal/server/opendefault.go` (or a sibling file)
- `internal/server/opendefault_test.go`

## Verification

- `go vet ./... && go test ./...` with table tests: missing entry added,
  stale entry updated, extra user entries preserved, JSONC refused,
  byte-identical no-op.
- Manual: boot against this repo, confirm `opencode.json` gains the
  entry and the Chat skill listing shows the four `lessmess-*` skills.
