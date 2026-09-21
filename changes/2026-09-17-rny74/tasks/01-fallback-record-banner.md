
# SPF-01: Persisted fallback record and board warning banner

Status: see [../ledger.md](../ledger.md).

## Objective

Make spawn fallbacks visible: record the outcome in
`.lessmess/spawn-fallback.json` and render a warning banner on the board
index until the next clean spawn clears it.

## Dependencies

SPF-00 (the ladder defines the outcome steps to record).

## Scope

- New state file `.lessmess/spawn-fallback.json`, written atomically via
  `model.WriteFileAtomic` (one-file-per-feature `.lessmess/` convention).
- Board index banner: server-rendered from the record.
- Unit tests.

Out of scope: dismissal persistence (record is server truth, self-healing),
settings page surfaces (SPF-02).

## Implementation steps

1. Define the record shape: `time` (RFC 3339), `attemptedAgent`,
   `attemptedModel`, `outcome` (`agent-only` | `model-only` | `plain`),
   `serviceError` (message), and the `step` descriptions that failed.
2. Write the record from `spawnSessionWithModel` when any de-escalation
   step succeeds after a failure; clear it (remove file or write empty) when
   a spawn succeeds on step 1 or when nothing was configured (clean spawn).
   Loading failures fail open (no banner).
3. Surface the record to the index view (same pattern as `GitDirty`): a
   render-time read in `indexView` (or a small helper) exposed to
   `web/templates/index.html` as a banner with attempted agent/model,
   outcome, and the service error.
4. Banner links to the Settings page (where the agent/model are configured).
5. Template copy states what happened, e.g. "Session spawn fell back:
   model `x/y` rejected — created with agent `build` only."

## Verification

- Record written with correct fields on each ladder outcome (extend the
  SPF-00 fake-transport tests); cleared on clean spawn.
- Absent/malformed record file → no banner, no error.
- Index HTML test: banner present with record, absent without.
- `go vet ./... && go test ./...`.

## Completion criteria

- A fallback spawn is explained in the UI without reading server logs.
- Record lifecycle (write on fallback, clear on clean) is covered by tests.

## Files affected

- `internal/server/settings.go` (record write/clear hooks)
- `internal/server/index.go` or wherever `indexView` lives (read + expose)
- `web/templates/index.html` (banner)
- `internal/server/settings_test.go` / new `spawnfallback_test.go`,
  `render_test.go` (banner markup)

## Notes

- `.lessmess/` is already gitignored in initialized repositories; no
  ignore-file change needed.
- Keep the record read cheap (small JSON, read per index render like
  `GitDirty`).
