# 2026-09-12-9: UI docs refresh button

- Change ID: 2026-09-12-9
- Created: 2026-09-12
- Branch: —
- Status: see [ledger.md](ledger.md)

## Objective and context

Give the UI a way to reconcile stale docs on demand, surfaced where the user
already looks: a **notification bell in the header** showing the docs findings
count, which opens a **modal** listing the findings and hosting a **refresh
button**. The button enqueues one manual gardener job covering every stale
directory; when the job finishes, the live SSE machinery updates the findings
(and the explorer tree) automatically.

Context: `POST /docs/refresh` currently reconciles only queue-stale dirs
(failed jobs). The common case in practice is hash-stale dirs (tree changed
under the docs, e.g. while a change is being implemented), so the endpoint
must cover both.

## Current behavior

- Docs findings render in the red banner (mixed with `changes/` violations);
  there is no way to trigger reconciliation from the UI.
- `POST /docs/refresh` enqueues a manual job for the queue's stale set only.
- `docs.ValidateDocs` computes hash-staleness but exposes no standalone helper.

## Target behavior

1. **Bell**: a header button (🔔) with a badge showing the docs findings count
   (hidden at zero; amber for warnings, red if any error-severity finding
   exists). The red banner is re-scoped to `changes/` violations only.
2. **Modal**: bell opens a modal listing docs findings grouped by severity,
   plus the refresh button. The button shows state (`Refresh stale docs` →
   `Refreshing…` → result note) and is disabled while a refresh runs.
3. **Refresh**: `POST /docs/refresh` enqueues one manual job for the union of
   queue-stale and hash-stale dirs (empty union → `nothing to refresh`). After
   the gardener finishes, docs SSE events drive a findings re-check, so the
   badge/modal (and explorer tree) update live.

## Scope

- `internal/docs`: `StaleDirs` helper (dirs whose STRUCTURE.md meta hash lags
  the tree).
- `internal/server`: `/docs/refresh` union semantics.
- `web/`: header bell + badge, modal (markup/CSS/JS), banner re-scope.

## Non-goals

- No per-dir selective refresh (one union job only).
- No progress bar for the gardener session (state text only).
- No changes to the queue schema or gardener flow.

## Design decisions

1. **Findings to the bell; violations keep the banner** (user direction):
   docs findings (warnings and errors) move to the bell/modal; the banner
   stays for `changes/` violations (red, loud, blocking-adjacent).
2. **Union refresh**: the button's usefulness depends on covering hash-stale
   dirs, so `StaleDirs` computes them directly (walk + meta compare) rather
   than parsing `ValidateDocs` message strings.
3. **No new state channels**: job progress surfaces via the existing docs SSE
   events + periodic `/api/validate` re-checks; the button's busy state is
   client-side with a timeout fallback.

## Implementation approach

1. `StaleDirs` in internal/docs + tests.
2. Endpoint union + tests.
3. Bell/modal UI + banner re-scope.
4. Dogfood + README touch-up.

## File-level impact

- `internal/docs/validate.go` (StaleDirs) + tests.
- `internal/server/docsqueue.go` (endpoint) + tests.
- `web/templates/layout.html` (bell + modal), `web/static/app.js`,
  `web/static/app.css`.
- `README.md` (small).

## Data, API, message, configuration, and schema changes

- `POST /docs/refresh` response adds `{"status":"nothing to refresh"}` for the
  empty case; success payload unchanged (`{"enqueued":[dirs]}`).
- No schema/config changes.

## Safety, security, rate-limit, and rollback

- Refresh is still a single serialized queue job; repeated clicks while busy
  are prevented client-side and harmless server-side (jobs enqueue FIFO).
- Rollback: revert routes/UI; no canonical data involved.

## Testing and verification strategy

- `StaleDirs`: seeded fixture → clean; file add → stale; meta missing → stale.
- Endpoint: union enqueued (202), empty → status message, disabled repo →
  disabled message (existing).
- Manual dogfood: bell badge counts, modal lists findings, button reconciles
  this repo's current hash-stale dirs live.
- `go vet ./... && go test ./...` green; `tasktracker validate` OK.

## Acceptance criteria

1. Bell badge reflects docs findings count and severity; zero → hidden.
2. Modal lists findings grouped by severity; banner shows only violations.
3. Button enqueues the stale union; empty union reports nothing to refresh;
   busy state prevents double-submit.
4. After a real reconciliation, badge/modal update without a page reload.
5. Suites green; validate OK.

## Tasks

1. [REF-00](tasks/00-staledirs-endpoint-union.md) — StaleDirs helper and /docs/refresh union
2. [REF-01](tasks/01-bell-modal-ui.md) — Notification bell, modal, banner re-scope
3. [REF-02](tasks/02-dogfood-readme.md) — Dogfood and README touch-up
