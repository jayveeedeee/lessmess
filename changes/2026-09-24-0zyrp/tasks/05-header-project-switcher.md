# MP-05: Project level in the breadcrumb trail

## Goal

The project level is part of the breadcrumb location nav, not a separate
header control: the trail's root is **Projects** (the landing page), below
it sits the **project's name** (linking to that project's change list), and
below that the change/panel crumbs as before. There is no project dropdown.

## Approach

- Layout exposes two body attributes the client trail reads:
  `data-base` (mount prefix, "" single-project) and `data-project-name`
  (effective `general.projectName`). No `{{.Projects}}` template surface —
  the earlier header-dropdown shape (project-nav/#project-menu,
  SetProjectSwitcher, pageData.Projects/CurrentSlug, BootFunc returning
  *Server) was removed again.
- `locationBaseTrail()` in app.js branches on `BASE`: hub mode builds
  `[Projects → /, <project name> → <base>/]` (+ change crumb when a
  change-bound session is open); single-project mode keeps today's
  `[Changes → /]` root byte-for-byte. The existing trail machinery
  (`#location-current`, back button, `activateLocation`) handles the rest
  unchanged — at the change list the current location reads as the project
  name; back goes to Projects.
- BootFunc returns a plain http.Handler again (nothing needs the *Server
  after boot).

## Revision

2026-09-24: originally a standalone Projects dropdown in the header
(shipped and live-verified); redesigned same day by user decision —
projects belong in the breadcrumbs as the root level. The dropdown
(template, CSS, JS, and the Go wiring that fed it) was removed; the trail
integration above replaced it.

## Files affected

- `web/templates/layout.html`
- `web/static/app.js`
- `web/static/app.css`
- `internal/server/render.go`, `internal/server/server.go`, `internal/server/hub.go`
- `cmd/lessmess/main.go`

## Verification

- `TestProjectBreadcrumbData` (hub page carries data-base/data-project-name,
  zero dropdown markup, landing has no location nav) and
  `TestLegacyModeKeepsRootTrail` (legacy attrs, Changes root label, no
  dropdown) pass; full suite + race green.
- Live: `/p/tasktracker/` emits `data-base="/p/tasktracker"
  data-project-name="lessmess"`; no project-menu markup anywhere.
