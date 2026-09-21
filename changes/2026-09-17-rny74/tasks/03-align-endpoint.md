
# SPF-03: Align endpoint and button for opencode.json default_agent

Status: see [../ledger.md](../ledger.md).

## Objective

Give the user an explicit one-click action to align the repository's
`opencode.json` `default_agent` with the effective `session.agent`,
patching only that key while preserving all other content and key order.

## Dependencies

SPF-02 (notice and read path; the button renders inside the notice).

## Scope

- New endpoint `POST /api/settings/opencode-default-agent`.
- Order-preserving patcher for `opencode.json` (JSON objects only).
- Settings page button wiring and tests.

Out of scope: silent sync, `opencode.jsonc` creation, global config,
`~/.config/opencode`.

## Implementation steps

1. Endpoint behavior:
   - Validate the effective `session.agent` against the live opencode
     service with the same rules as settings save (known primary, non-hidden
     agent for the served directory); 422 when invalid, 503 when the service
     is unreachable (stricter than settings save — a write deserves
     validation).
   - Require an existing `opencode.json` with a `default_agent` key (the
     notice only renders in that case); 422 otherwise.
   - Patch the file: replace the `default_agent` value with the effective
     agent; every other byte of structure, key order, and whitespace is
     preserved. Write atomically via `model.WriteFileAtomic`.
   - Respond with old/new values; log the change.
2. Patcher implementation:
   - Token-level rewrite (encoder/decoder or a minimal ordered map) that
     keeps unknown keys untouched; refuse with a clear 422 message when the
     file contains comments or trailing commas (JSONC) — never rewrite a
     file Go cannot round-trip. Share the string-aware comment scanner with
     SPF-02's reader.
   - Never write outside the served repository root.
3. UI: render an **Align** button inside the SPF-02 advisory; on success,
   reload the settings data (notice disappears; no page-level state).
4. Tests.

## Verification

- Golden test: input `opencode.json` with several keys (including nested
  objects like `watcher.ignore`) → output identical except the
  `default_agent` value; key order preserved.
- JSONC file → 422, file byte-identical afterwards.
- Invalid agent (e.g. a subagent mode id) → 422; opencode down → 503.
- Absent file or absent key → 422 (button never shows, endpoint defends).
- Success path updates the divergence notice to disappearance on reload.
- `go vet ./... && go test ./...`.

## Completion criteria

- Align works end-to-end against a fixture repo and a fake opencode
  service; all refusal paths leave the file untouched.

## Files affected

- `internal/server/settingsapi.go` (endpoint), `settings.go`
  (patcher/scanner), `web/templates/settings.html` (button)
- `internal/server/settingsapi_test.go`, new patcher test file

## Notes

- Concurrency stance: last-writer-wins, same as `applySettingsPatch`; no
  locking introduced (documented in the plan).
- After aligning, opencode-side sessions (TUI/CLI) created in the repo pick
  up the new default on their next read of `opencode.json`.
