
# PROJ-02: Settings General project-name field

Status: see [../ledger.md](../ledger.md).

## Objective

Make Settings → General a real, editable section where the project name is collected with source
badges, scope handling, and the ✦ change discussion.

## Dependencies

- PROJ-00 (schema, API fields, allowlist).

## Scope

- `web/templates/settings.html`, `web/static/app.js`
- No server-side changes beyond what PROJ-00 delivered.

## Implementation steps

1. `settings.html` general section: add the standard head (h2 General, `[data-save]` Save,
   `.settings-status`) and a string field `data-field="general.projectName"` with
   `.src-badge` + `.settings-change` button and help text ("Shown next to the logo and as the
   browser tab name. Empty uses the repository folder name."). Keep the onboarding wizard ghost
   anchor field below it.
2. `app.js` `initSettings`: the existing `render()` loop handles string fields generically — add
   the `general.projectName` cases: `fallbackFor` returns the other layer's raw value, else
   `view.defaultProjectName`; `placeholderFor` renders that fallback (e.g. the folder name) so an
   empty input visibly means "folder default".
3. Save flow: the existing `[data-save]` loop builds `payload.general.projectName` automatically
   from the dotted field path. After a successful save of the `general` section specifically,
   `location.reload()` so the server-rendered header and tab title pick up the new name.
4. Confirm the ✦ change button flow works for the new field (PROJ-00 added the allowlist case);
   no JS change expected.

## Verification

- Handler/template test: settings page renders the new field hooks (`data-field`,
  `data-save`) in the general section (follow the existing render-test assertions for settings
  hooks).
- Manual pass: Project and Personal scopes — save a name in each, verify the badge shows the
  source, Personal wins, clearing a layer's field restores inheritance (badge back to the lower
  layer/default, placeholder shows the folder name); header and tab update after each save's
  reload; ✦ change opens a discussion for `general.projectName`.

## Completion criteria

General collects the name in both scopes with correct badges and fallback placeholder; a save is
reflected in the chrome immediately (reload); existing settings interactions (nav, other sections,
exclusions editor) unaffected.

## Files affected

`web/templates/settings.html`, `web/static/app.js`, a render test.

## Notes

- The template AGENTS note says general must stay field-free — that constraint is intentionally
  lifted by this change; rename nothing (`data-section="general"` becomes a real PUT key, which
  PROJ-00's `patchSection` accepts).
- If the reload-on-save feels heavy during review, the fallback is a header/banner hint instead —
  but the reload is the recommended default (matches the `#overall-status` select precedent).
- Verification evidence 2026-09-17: render test pins `data-field="general.projectName"` and
  `data-group="general"`; live scratch-server check verified PUT project ("Atlas") and personal
  ("Mine") saves, personal winning, `general.projectName` source badge = personal, and
  `node --check app.js` clean. Interactive click-through of Save → reload awaits user review.
