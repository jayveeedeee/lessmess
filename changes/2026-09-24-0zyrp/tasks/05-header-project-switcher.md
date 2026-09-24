# MP-05: Header project switcher

## Goal

A dropdown in the sticky header lists all registered projects and jumps between them; the current project is highlighted; legacy single-project mode shows no switcher.

## Approach

- `pageData` gains `Projects []projectLink{Slug, Name, Base}` and `CurrentSlug`; the hub gives each project server a project-lister callback (`SetProjectLister`) so `render()` can fill them per request — names derive from each repo's effective `general.projectName` (fallback basename), never from stored config.
- `layout.html`: dropdown beside the existing location nav (respect the `{{if ne .Page "setup"}}` guard and the z-index ladder used by `#location-menu`), entries link to `<base>/` (project home), plus an "All projects" link to `/`; current project gets a highlight class; hidden entirely when `CurrentSlug` is empty (legacy mode).
- Vanilla JS toggle consistent with the existing location-dropdown pattern in `app.js`; keyboard/Escape handling mirrors it.

## Files affected

- `web/templates/layout.html`
- `web/static/app.js`
- `web/static/style.css` (dropdown styling, reuse location-menu patterns)
- `internal/server/render.go`, `internal/server/hub.go`

## Verification

- Test: hub-mode page render includes all projects with correct prefixed hrefs and the current highlight; legacy render contains no switcher markup.
- Manual: switch between two fixture projects from a board page; browser back returns to the previous project's page; "All projects" opens the landing.
