---
id: SPF-02
title: default_agent divergence notice on the Settings page
---

# SPF-02: default_agent divergence notice on the Settings page

Status: see [../ledger.md](../ledger.md).

## Objective

Show the user, on the Settings page, when the repository's `opencode.json`
declares a `default_agent` that differs from the effective `session.agent`
— the reason sessions created outside lessmess use a different agent.

## Dependencies

None at the code level; sequencing after SPF-01 keeps the board work first.
SPF-03 (align button) builds directly on the read path defined here.

## Scope

- Read-only detection of `default_agent` in the repository root's
  `opencode.json`.
- `GET /api/settings` response extension and Settings page advisory text.
- Unit tests.

Out of scope: any write to `opencode.json` (SPF-03), global opencode config.

## Implementation steps

1. Add a reader (settings package) that loads `<repoDir>/opencode.json`,
   extracts the top-level `default_agent` string, and reports a parse status:
   `ok` (key present), `absent`, or `unreadable` (missing file counts as
   `absent`; JSONC/malformed counts as `unreadable`). All failures degrade to
   "no notice" — read-only, fail-open, logged at debug.
2. Extend the settings response (the struct behind `GET /api/settings`) with
   `opencodeDefaultAgent`: declared value (or empty), file path, and status.
3. Render the advisory in `web/templates/settings.html` near the session
   agent field when status is `ok` and the declared value differs from the
   effective `session.agent`: explain that sessions created outside lessmess
   (opencode TUI/CLI) will use the declared agent. Show nothing on match,
   `absent`, or `unreadable` (optionally a subtle hint for `unreadable`).
4. Reserve UI space/logic for the align button (SPF-03) next to the notice,
   rendered only when that task lands.

## Verification

- Fixture repos: `opencode.json` with divergent `default_agent` → notice in
  HTML and response fields populated; matching value or absent file → no
  notice; JSONC content → status `unreadable`, no notice, no error.
- `go vet ./... && go test ./...`.

## Completion criteria

- The advisory appears exactly on divergence and never mutates anything.
- Response extension is covered by tests; fail-open behavior verified.

## Files affected

- `internal/server/settings.go` (reader) and `settingsapi.go`
  (response fields)
- `web/templates/settings.html` (advisory)
- `internal/server/settingsapi_test.go`, `render_test.go`

## Notes

- tevr-core's real-world divergence (`kg-orchestrator` vs `build`) is the
  canonical case this notice must surface.
- The scanner must be comment-aware enough to classify JSONC as
  `unreadable` rather than misparsing (shared with SPF-03's patcher —
  extract one helper both can use).
