---
id: LRN-08
title: Exclusions editor and bell seed button
---

# LRN-08: Exclusions editor and bell seed button

Status: see [../ledger.md](../ledger.md).

## Objective

The exclusion folder list becomes editable and saved (`agentsdocs.json`)
without re-running the wizard, and pending seed dirs get a one-click
"Seed pending docs" button in the bell modal.

## Dependencies

- LRN-07 (seed endpoint the button posts to).

## Scope

- `GET /docs/exclusions` (top-level entries with excluded state) and
  `POST /docs/exclusions` (`{excludeDirs}`) writing through
  `updateConfigExcludes` semantics.
- Settings Docs section: folder checkbox list + Save.
- Bell modal: conditional seed button wired to the LRN-07 endpoint.

## Implementation steps

1. Extract the top-level entry builder from `setupDirs` so the normal
   server can serve the same shape (`dirs`, `hasConfig`); nested lazy
   expansion stays setup-only.
2. `GET /docs/exclusions`: 200 with entries (503 when no config? no —
   return `hasConfig:false` so the UI can explain; POST returns 503
   without a config).
3. `POST /docs/exclusions`: validate every pattern with
   `validExcludePattern` (422 otherwise), then `updateConfigExcludes`
   (503 when the config file is absent — bootstrap owns creation); reply
   `{saved: bool}`.
4. `settings.html` Docs section: "Excluded folders" widget (checkboxes;
   default-excluded rows checked and disabled; Save button). `app.js`
   `initSettings` fetches, renders, and saves; status text mirrors the
   section's save pattern.
5. `layout.html` bell modal: `#docs-seed-btn` (hidden by default), label
   carrying the pending count from `/api/validate`'s `docsSeedPending`;
   `app.js` shows it when > 0, posts `/docs/seed`, busy state, re-enable
   on the next docs SSE event (mirror of the refresh button).

## Verification

- Endpoint tests: GET shape/round-trip after POST; POST replaces picker
  patterns and preserves hand-authored globs (fixture with `web/static`);
  422 invalid pattern; 503 without config; changes/ never listed.
- Render test: Settings page contains the widget; bell modal contains
  the seed button (hidden by default).

## Completion criteria

Exclusion edits persist to `agentsdocs.json` and round-trip; the bell
offers seeding exactly when dirs are pending; suites green.

## Files affected

- `internal/server/setup.go` (extract entry builder),
  `internal/server/docsseed.go`, `internal/server/server.go` (routes)
- `web/templates/settings.html`, `web/templates/layout.html`,
  `web/static/app.js`
- `internal/server/*_test.go`, `render_test.go`

## Notes

- The root directory is never listed and never excludable (config design).
