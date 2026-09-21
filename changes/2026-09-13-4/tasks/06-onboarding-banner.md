
# ONB-06: Index banner and Settings re-entry

Status: see [../ledger.md](../ledger.md).

## Objective

On initialized repos whose onboarding is incomplete, surface a dismissible banner on the index linking to `/setup`, and add a permanent "Re-run onboarding" entry point on the Settings page.

## Dependencies

- ONB-00 (pending/dismiss state)
- ONB-05 (the `/setup` page the banner links to)

## Scope

- `internal/server/server.go`: `index` loads `onboardingPending(dir)` (cheap stateless read) into the view model.
- `web/templates/index.html`: banner partial ("Finish setting up lessmess" + link to `/setup` + Dismiss button → `POST /api/setup/dismiss`, then hide).
- `web/templates/settings.html`: a small "Re-run onboarding" link (to `/setup`) — placement consistent with the page's section layout.
- `web/static/app.js`: dismiss wiring (fetch + remove banner, no reload).
- Completing the wizard (`POST /api/setup/complete`, ONB-05) sets `completedAt`, which also kills the banner.

## Implementation steps

1. Extend `indexView` + handler; render the banner only when pending.
2. Add the dismiss endpoint wiring in `setup.go`'s registrar (state write via ONB-00 helpers) if not already present from ONB-01's skeleton.
3. Template + JS + minimal CSS (reuse existing banner/muted styles).
4. Render tests: banner present when pending, absent after completed/dismissed.

## Verification

- `go test ./internal/server -run 'Index|Banner|Onboarding' -v` passes.
- Manual on this repo: banner appears → dismiss → gone across reloads and restarts; Settings link opens `/setup`.

## Completion criteria

- One-time, dismissible, persisted banner on initialized repos; permanent Settings re-entry; no nag for completed setups.

## Files affected

- `internal/server/server.go`, `internal/server/setup.go`
- `web/templates/index.html`, `web/templates/settings.html`
- `web/static/app.js`, `web/static/app.css`
- `internal/server/render_test.go`

## Notes

- This repo itself will show the banner once after upgrade — expected (decision: absent state file = incomplete); dismiss persists.
- Keep the banner out of setup mode (the wizard is already the whole UI there).
