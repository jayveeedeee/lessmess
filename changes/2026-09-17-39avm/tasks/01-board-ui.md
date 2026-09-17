---
id: HOF-01
title: Handoffs listing endpoint and board "Spawn new change" action
---

# HOF-01: Handoffs listing endpoint and board "Spawn new change" action

Status: see [../ledger.md](../ledger.md).

## Objective

Human-driven handoff from the board: the sessions panel on a change board offers "Spawn new change" — title, optional prefix, and a picker of `handoff*.md` artifacts in the change directory — and session rows badge sessions spawned from another change.

## Dependencies

HOF-00 (the endpoint this UI drives, and the `SpawnedFrom` mapping field it displays).

## Scope

- `internal/server/server.go`: route `GET /changes/{id}/handoffs`.
- New small handler (in `changesession.go`): list `handoff*.md` filenames at the root of `changes/<id>/`, sorted; 404 unknown change; empty list is a 200 with `[]`.
- `web/templates/board.html`: "Spawn new change" form in the sessions panel (title input, prefix input, artifact select, submit).
- `web/static/app.js`: load the artifact picker from `/changes/{id}/handoffs` when the panel opens; POST to `/changes/{id}/spawn-change`; surface errors (409/422 messages) inline; refresh the sessions list on success; badge rows whose mapping entry has `spawnedFrom` (e.g. "from 2026-09-10-4").

Out of scope: index-page entry points, any changes to discussion UI.

## Implementation steps

1. `listHandoffs` handler: `filepath.Glob("handoff*.md")` under the change dir, basename + sort; reuse the store's change-dir resolution rather than hand-joining paths.
2. Wire the route next to the other change routes.
3. Extend the sessions-panel markup: a collapsed-by-default "Spawn new change" details/section with the three fields; keep the existing panel's styling vocabulary (btn-ghost, sessions-list classes).
4. app.js: fetch handoffs on panel open (lazy, like the sessions fetch), populate the select, disable submit until title + artifact are set; on success clear the form and refresh sessions; on error show the server's message.
5. Badge session rows from the `spawnedFrom` field returned by `GET /changes/{id}/sessions` (mapping entries serialize it automatically via omitempty).

## Verification

- `go vet ./... && go test ./...`; handler test for `listHandoffs` (sorted, empty, 404).
- Manual check with a running build (`CGO_ENABLED=0 go build -o lessmess ./cmd/lessmess` — embedded assets require a rebuild): handoff file written by hand → panel lists it → spawn creates the new change, board shows the new session badged "from <A>".
- Empty case: no handoff files → submit disabled with a hint naming the convention.

## Completion criteria

All verification passes; the board flow works end-to-end without touching the API by hand.

## Files affected

- `internal/server/server.go`, `internal/server/changesession.go`, `internal/server/changesession_test.go`
- `web/templates/board.html`, `web/static/app.js`

## Notes

- The web UI is embedded at build time — template/static changes are invisible until the binary is rebuilt and restarted.
- Keep the artifact convention strict (`handoff*.md`, root of the change dir) so the picker cannot offer plan/ledger/tasks files.
- `sessionResponse` now carries `SpawnedFrom` (set in `enrich`) so the badge works off the sessions listing JSON.
- Board submit opens the new session in the terminal as feedback (the spawned session belongs to the new change, so the source board's session list is unchanged).
- Smoke test 2026-09-17 (scratch repo, built binary): board HTML contains spawn form + button; `GET .../handoffs` returns `{"handoffs":["handoff-demo.md"]}`; 404 unknown change; 422 non-handoff artifact; spawn endpoint routed. `go vet` clean, `go test ./...` all 6 packages pass.
- Smoke-test cleanup note: the first scratch run unexpectedly reached the local opencode service (spawn returned 201) and created one real stray session — deleted via the service API (204, confirmed 404). Later runs verified nothing else leaked.
