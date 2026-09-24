# MP-02: Server-side base-path plumbing

## Goal

Every server-emitted URL carries the project prefix when the server runs hub mode, and stays byte-identical to today when the base is empty (legacy `--dir` mode).

## Approach

- `Server` gains a `Base` field (`""` legacy, `/p/<slug>` hub), set at construction by the boot path; `render()` injects it into `pageData.BasePath` for every page and partial.
- Fix Go-side absolute emissions:
  - `HX-Redirect: "/changes/"+id` in `createChange` (server.go) → `s.Base + "/changes/" + id`.
  - Breadcrumbs / `BoardURL` / drill-down hrefs in `render.go` and the tasks-feed row builder → prefix via the view structs, filled from `s.Base`.
  - Setup-mode redirect to `/` (setup.go) → base-aware redirect.
  - Confirm no other `http.Redirect`/`Location` emissions exist (grep audit, list findings here).
- `PublicBase` per project already includes the prefix (set by MP-01's `BootFunc`), which flows into prime curl instructions — verify with a spawned-session prime render in a fixture repo.
- Golden-style tests: render key pages (index, board, setup) with base `""` and base `/p/x`, asserting prefixed outputs in the latter and unchanged bytes in the former.

## Files affected

- `internal/server/server.go`
- `internal/server/render.go`
- `internal/server/setup.go`
- `internal/server/*_test.go`

## Verification

- New tests assert prefix-carrying `HX-Redirect`, breadcrumbs, and setup redirects, plus unchanged legacy output.
- `go test ./internal/server/ -run BasePath -v` green; full `go test ./...` green.
