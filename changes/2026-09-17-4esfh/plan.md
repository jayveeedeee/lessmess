# 2026-09-17-4esfh: Project name branding

- Change ID: 2026-09-17-4esfh
- Created: 2026-09-17
- Branch: —
- Status: see [ledger.md](ledger.md)

## Objective and context

lessmess currently has no user-facing name for the repository it serves: the header shows only
the logo icon, the browser tab says `<page> · lessmess`, and the embedded terminal head shows the
icon plus a session title. The user wants a configurable **project name** that personalizes the
UI: collected in General settings and during onboarding, defaulted to the served directory's
basename, displayed next to the logo everywhere (including the terminal overlay), and used as the
permanent browser tab title.

## Current behavior

- Settings schema (`internal/server/settings.go`) has sections `session`, `prompts`, `git`,
  `ui`, `docs` — there is no `general` section. The Settings → General block
  (`web/templates/settings.html`) is deliberately display-only: no fields, no `[data-save]`.
- `web/templates/layout.html` renders `<title>{{.Title}} · lessmess</title>` (per-page titles from
  the five `pageData{...}` call sites) and a brand that is only `a.brand > img.brand-icon`.
- The terminal overlay in the same layout shows `img.brand-icon` + `#terminal-title`
  (session title, set by `app.js`).
- The onboarding wizard (`web/templates/setup.html` + `initSetup` in `web/static/app.js`) has five
  steps: prereqs, bootstrap, agent, docs, finish. The agent step shows the established pattern for
  collecting a value with a Project/Personal scope choice and saving via `PUT /api/settings`.
- The setup-mode mux already serves `GET /api/settings` (`env.getSettings`), `PUT /api/settings`,
  and `GET /api/settings/options`.
- The Explorer page `<h1>` shows `filepath.Base(s.st.Dir)` — stays as-is (user decision).

## Target behavior

- A new layered setting `general.projectName` (project `lessmess.json` / personal
  `.lessmess/settings.json`). Unset ⇒ the effective name falls back to
  `filepath.Base(repoDir)` at read time, so the feature works zero-config and follows folder
  renames until overridden.
- Settings → General becomes a real section with a "Project name" field (source badge, ✦ change
  discussion, Save) and keeps the onboarding link.
- The onboarding wizard gains a dedicated "Project name" step right after Prerequisites with the
  same scope pattern (Project scope checked by default) and a "use the folder name" skip.
- The header shows the project name next to the logo on every page, including setup and the
  terminal overlay head (next to the overlay's icon, before the session title).
- The browser tab title is exactly the project name on every page.

## Scope

- `internal/server`: settings schema/merge/patch plumbing, project-name accessor + renderer hook,
  `settingsResponse` default exposure, `settingsFieldValue` allowlist, wiring in `server.New` and
  `NewSetup`, tests.
- `web/templates/layout.html`: title, header brand name, terminal overlay name.
- `web/templates/settings.html`: General section becomes editable.
- `web/templates/setup.html`: new wizard step.
- `web/static/app.js`: `initSettings` placeholder/fallback handling for the new field, wizard step
  wiring.
- `web/static/app.css`: `.brand-name` styling.
- `README.md`: document the `general` section / project name.

## Non-goals

- The Explorer `<h1>` keeps showing the raw directory basename (explicit user decision).
- No favicon/manifest/PWA name changes; no changes to the "lessmess" product branding (aria-labels
  stay).
- No renaming of onboarding step keys or any existing `data-*` hooks.
- No docs-gardener or STRUCTURE.md behavior changes (docs refresh runs on close as usual).

## Design decisions

1. **Storage: `general.projectName`, tri-state, layered** — matches every other setting; source
   badges and ✦ change discussions come for free; old binaries reading a new `lessmess.json` are
   unaffected (unknown fields are ignored on read; `patchSection` gates writes).
2. **Default is computed, not persisted** — `loadEffectiveSettings` applies
   `filepath.Base(repoDir)` when both layers leave the field empty. Renaming the folder updates
   the default automatically; saving an explicit value freezes it.
3. **Tab title = project name only** (user decision) — `<title>{{.ProjectName}}</title>` in
   `layout.html`; per-page `pageData.Title` becomes unused for the title element.
4. **Server-rendered, centrally filled** — `pageData` gains `ProjectName`, filled in
   `renderer.render` via a `projectName func() string` hook wired like the accent hook in both
   `server.New` and `NewSetup`; degenerate empty names fall back to `lessmess`.
5. **Terminal overlay** — the name is a static span next to the overlay's icon; `#terminal-title`
   stays the JS-driven session title after it.
6. **Wizard step placement: directly after Prerequisites** (user decision), scope radios with
   **Project checked by default** (the name is shared repo branding, unlike the personal-defaulted
   agent step); prefill from `GET /api/settings` effective value (already served in setup mode).
7. **Settings → General save reloads the page** on success so the server-rendered header and tab
   pick up the new name immediately (a targeted `location.reload()` for that section only).

## Detailed implementation approach

- **Schema (`settings.go`)**: add `GeneralSettings{ProjectName string}` + `Settings.General`;
  `EffectiveGeneralSettings{ProjectName string}` + `EffectiveSettings.General`; merge with
  `pickStr("general.projectName", ...)`; apply the `filepath.Base(repoDir)` fallback in
  `loadEffectiveSettings` (which has `repoDir`); `patchSection` gains `case "general"`.
- **Accessor**: `effectiveProjectName(repoDir string) string` — effective value with fallback
  (and a final `lessmess` guard); used by the render hook and `settingsResponse`.
- **API (`settingsapi.go`)**: `settingsResponse` gains `DefaultProjectName string
  \`json:"defaultProjectName,omitempty"\`` so the settings UI can placeholder it;
  `settingsFieldValue` allowlist (`settingschange.go`) gains `general.projectName`.
- **Renderer (`render.go`)**: `pageData.ProjectName`; `renderer.projectName` hook filled in
  `render()` next to `AssetsV`/`AccentStyle`.
- **Chrome (`layout.html`, `app.css`)**: `<title>{{.ProjectName}}</title>`; brand anchor gains
  `<span class="brand-name">{{.ProjectName}}</span>` after the icon; terminal overlay head gains
  the same span before `#terminal-title`; `.brand-name` styled (weight 600, nowrap) and resilient
  on narrow widths.
- **Settings UI (`settings.html`, `app.js`)**: General section gets head + Save + status, the
  `data-field="general.projectName"` string field (existing render/save loops handle it via the
  dotted-path convention), `placeholderFor`/`fallbackFor` special-cases to show the directory
  default, and a successful general save reloads the page. The wizard ghost anchor stays.
- **Wizard (`setup.html`, `app.js`)**: new `data-step="name"` section + `data-step-nav="name"`
  item after prereqs; input `#setup-project-name` prefilled from `GET /api/settings`
  (`effective.general.projectName`); scope radios (`setup-scope`-style, project checked);
  `#setup-name-save` PUTs `{general:{projectName}}` and continues to bootstrap; `#setup-name-skip`
  continues without saving; prereqs-next retargets to the name step; finish summary lists the
  chosen name when saved.

## File-level impact

| File | Change |
| --- | --- |
| `internal/server/settings.go` | `General` section, effective view, fallback, `patchSection` |
| `internal/server/settingsapi.go` | `DefaultProjectName` in the settings payload |
| `internal/server/settingschange.go` | allowlist case `general.projectName` |
| `internal/server/server.go` | wire `rend.projectName` hook |
| `internal/server/setup.go` | wire the hook on the setup shell |
| `internal/server/render.go` | `pageData.ProjectName` + hook fill |
| `web/templates/layout.html` | title, brand name, terminal head |
| `web/templates/settings.html` | editable General section |
| `web/templates/setup.html` | Project name step |
| `web/static/app.js` | settings field handling + wizard step |
| `web/static/app.css` | `.brand-name` |
| `README.md` | document the setting |

## Data, API, configuration, schema changes

- `lessmess.json` / `.lessmess/settings.json` gain an optional `general.projectName` key
  (backward/forward compatible).
- `GET /api/settings` payload gains `defaultProjectName` and `effective.general.projectName`;
  `PUT /api/settings` accepts a `general` section.
- No changes to `changes/` data, store formats, or workflow endpoints.

## Safety, compatibility, rollback

- Purely additive schema; unset fields keep the old (directory-name/tab) behavior with the name
  defaulting to the folder basename, so rollback is a revert with no data migration.
- Writes stay atomic and layer-confined via the existing `applySettingsPatch`.
- `pageData.Title` remains populated at call sites (harmless) so a revert of the template alone
  restores the old title.

## Testing and verification strategy

- Go tests following existing patterns: settings merge/fallback and layered override
  (`settingswiring_test.go` style), `applySettingsPatch` general-section accept/reject,
  `settingsResponse` default exposure, allowlist case, render tests asserting the new title,
  `.brand-name` spans (normal + setup + terminal head).
- `go vet ./... && go test ./...`, `CGO_ENABLED=0 go build -o lessmess ./cmd/lessmess`,
  `lessmess validate`.
- Manual pass after rebuild/restart: header and tab on index/board/explorer/settings/setup,
  terminal overlay head, settings General save (both scopes + badge), wizard step save/skip,
  empty-value fallback to the folder name.

## Observability

- None beyond the existing `settings saved` slog line (scope already logged). No new endpoints.

## Rollout sequence

1. PROJ-00 backend plumbing → 2. PROJ-01 chrome rendering → 3. PROJ-02 settings UI and
4. PROJ-03 wizard step (independent of each other) → manual verification pass.

## Risks and mitigations

- **Global title change surprises deep-linked tabs** — accepted; it is the requested behavior and
  trivially revertible.
- **Long names squeeze the header/terminal head** — `white-space: nowrap` plus existing flex
  layout; verify on narrow windows during the manual pass.
- **Wizard step order regressions** — keep all existing `data-step*` keys; only insert; render
  test asserts the new step, existing tests pin the others.

## Acceptance criteria

1. `general.projectName` is collected in Settings → General and in a dedicated wizard step, with
   working source badges, scopes, and ✦ change discussions.
2. With the setting unset, the displayed name (header, terminal head, tab) equals the served
   directory's basename; with it set in either layer, the layering rules apply (personal >
   project > folder default).
3. The header shows the name next to the logo on every page, including setup and the terminal
   overlay.
4. The browser tab title is exactly the project name on every page.
5. `go vet`, `go test`, build, and `lessmess validate` pass; README documents the setting.

## Tasks

1. [PROJ-00](tasks/00-general-section-plumbing.md) — `general` settings section, fallback, API exposure
2. [PROJ-01](tasks/01-project-name-chrome.md) — header, terminal head, and tab title rendering
3. [PROJ-02](tasks/02-settings-general-ui.md) — Settings → General project-name field
4. [PROJ-03](tasks/03-onboarding-name-step.md) — onboarding wizard Project name step
