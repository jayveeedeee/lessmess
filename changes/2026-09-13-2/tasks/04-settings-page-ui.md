---
id: SET-04
title: Settings page UI and client wiring
---

# SET-04: Settings page UI and client wiring

Status: see [../ledger.md](../ledger.md).

## Objective

Build the user-facing settings page linked from the right side of the top
menu, and wire the client-side settings (terminal auto-open) into `app.js`.

## Dependencies

- SET-02 (server wiring)
- SET-03 (settings API)

## Scope

- `GET /settings` route rendering a new `web/templates/settings.html` via
  the layout (`Page: "settings"`).
- `layout.html`: right-aligned Settings link in the header (new
  `.topnav-right` group before the notification bell), active highlight on
  the settings page.
- Page sections matching the plan's settings table: Session (agent +
  model dropdowns, auto-open toggle), Prompts (six textareas), Git
  (default branch), UI (show archived), Docs (gardener on close).
- UI mechanics (client-side in `app.js`):
  - Scope toggle: "Project (committed)" vs "Personal (this machine)";
    saves go to `PUT /api/settings?scope=...` for the selected scope.
  - Fields show effective values with a source badge
    (Default / Project / Personal); string fields use the inherited value
    as placeholder; booleans are tri-state selects (Inherit / On / Off).
  - Agent/model dropdowns populated from `GET /api/settings/options`;
    when `available:false`, show a "service unavailable — enter names
    later" hint with the fields still editable as text.
  - Prompt textareas carry helper text ("appended to the built-in X
    prompt") and a character-level hint that the base prompt is unchanged.
  - Save button per section with inline success/error feedback; reload
    state after save.
  - On every page load, cache effective settings and honor
    `session.autoOpenTerminal` in the session-spawn flows (discussion,
    board new session, commit flows) — no auto-open when false.
- `web/static/app.css` styles for the form, badges, scope toggle.
- README "Settings" section documenting the page, both files, layering,
  and each setting.

## Implementation steps

1. Template + route + nav link; page renders with defaults.
2. Client JS: load effective settings + options, render badges/dropdowns,
   per-section save, reload.
3. Auto-open gate in existing spawn flows.
4. Styles, then README.

## Verification

- `go test ./...` (render tests if added) passes.
- Manual: page loads at `/settings`; save at each scope; badges update;
  dropdowns list live agents/models; offline hint appears with the service
  stopped; auto-open off suppresses the terminal overlay on session
  creation.

## Completion criteria

- All acceptance criteria 1, 2, 6, and 7 (UI portions) of plan.md hold in
  a live server; README section committed.

## Files affected

- `internal/server/server.go` (route), `internal/server/render.go` (template parse)
- `web/templates/settings.html` (new), `web/templates/layout.html`
- `web/static/app.js`, `web/static/app.css`
- `README.md`

## Notes

- Keep element ids stable and follow existing `app.js` patterns
  (`data-page` guards, fetch + JSON, inline status spans).
- Read `web/templates/AGENTS.md` before editing templates.
- Deviation (improvement): agent/model pickers are text inputs with a
  `<datalist>` of live suggestions instead of `<select>` dropdowns — same
  live-dropdown UX when the service is up, and free text automatically when
  it is down (no control swapping). Only primary, non-hidden agents and
  `providerID/id` model values are suggested.
- Deviation: the auto-open gate covers the discussion form, the board's
  New session button, and Start session; explorer chat deliberately still
  opens (a chat click is an explicit request to talk). Documented in
  README's settings table entry for `session.autoOpenTerminal` scope.
- `#theme-toggle`'s `margin-left: auto` moved to the new `.topnav-right`
  group so the Settings link sits flush right.
- Verified 2026-09-13 against a live server on a throwaway repo with the
  real opencode service: `/settings` renders (nav group + all fields),
  project PUT writes `lessmess.json` (sources become `project`), personal
  PUT writes `.lessmess/settings.json` and wins, clearing a personal field
  restores the project value, 400/422 rejections, and
  `/api/settings/options` returns live data (agents `build`,`plan`; 107
  models; correct service default). `node --check web/static/app.js` clean.
  `TestSettingsPageHTML` covers the template and nav. Client-side dynamics
  (badges, datalist rendering, offline hint, auto-open gate) follow the
  verified endpoints; browser spot-check left for user acceptance.
