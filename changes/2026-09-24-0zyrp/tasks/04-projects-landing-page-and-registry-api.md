# MP-04: Projects landing page and registry API

## Goal

Add and remove projects from the browser: the landing page becomes the management UI, backed by a small registry API that hot-boots and cleanly unmounts projects without restarting the process.

## Approach

- API on the hub root mux:
  - `POST /api/projects` `{path}` → validate (absolute, exists, is a dir), `registry.Add` (slug dedupe), persist, hot-boot (full server or setup wizard per MP-01 rules), `201 {slug}`; duplicate path → `409`, invalid path → `422`, registry write failure → `500` with nothing left half-mounted (unboot on error).
  - `DELETE /api/projects/{slug}` → unmount (close store, watcher, SSE subscribers via the project's close func), remove from registry, persist, `204`; unknown slug → `404`.
  - `GET /api/projects` (from MP-01) gains per-project `name` (effective `general.projectName` fallback basename) and change counts are deliberately omitted (no cross-project aggregation).
- Landing page (`projects.html`): table of projects — name, slug, path, status (available / setup pending / unavailable) with a link per available project; Add form (path input + submit, inline error rendering); Remove button per row with confirm; empty state inviting the first add.
- Order of operations on remove matters: registry persist last so a failed unmount doesn't lose the entry (retryable), and a failed registry write after successful unmount is logged and reported.

## Files affected

- `internal/server/hub.go`
- `web/templates/projects.html`
- `web/static/app.js` (landing page behavior)
- `internal/server/hub_test.go`

## Verification

- API tests with fixture repos: add (happy, 409 duplicate, 422 invalid), boot-under-prefix immediately after add, remove → prefix 404s and registry file updated, remove-then-re-add same path gets the same slug.
- Race detector clean on add/remove loops (`go test -race ./internal/server/ -run Hub`).
- Manual: add a real project via the page, work a board in it, remove it, confirm the repo's `changes/` and `.lessmess/` are untouched on disk.
