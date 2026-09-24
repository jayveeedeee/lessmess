# UI-02: Add Resume session to the breadcrumb location menu

## Why

Resuming a change's session must not depend on a board page. The user decided: the header's
breadcrumb dropdown (location menu) gains a "Resume session" entry at its bottom whenever a change
context exists — chat open or closed.

## Current behavior

- The app bar's location control was reworked by change 2026-09-21-aw36x (tasks 64/66, commit
  be42a6c): a back chevron (`#location-back`) activates the previous trail entry, and
  `#location-toggle` opens the `#location-menu` dropdown listing the full trail
  (`#location-trail`); entries switch chat panels, open detail documents (restoring the prior
  trail on close), and navigate between pages. The trail machinery (`locationTrail`,
  `renderLocationTrail`, `activateLocation`) survived intact.
- The change crumb appears only while a change-bound chat is open (`syncLocationFromChat`);
  closing the chat resets the trail to [Changes].
- Resume logic exists on the board's Continue button (`initContinue`): prefer the stored
  `tt-last-session:<change>` session if still listed, else the newest created, else create a
  session (`POST /changes/{id}/sessions`) and open it; the label flips Continue/Start.

## Target behavior

- Extract that continue/start logic into a shared helper `resumeChangeSession(changeID)` (UI-03's
  card tap reuses it).
- `renderLocationTrail` appends a distinct "Resume session" action row at the bottom of
  `#location-menu` when the trail contains a change item; label flips "Resume session"/"Start
  session" from the sessions payload. Activating it runs the helper and closes the menu.
- The change crumb persists in the trail for the browser session once a change context is entered
  (entering = resuming a change's session or opening its chat), so the dropdown stays the
  re-entry point after the chat closes; picking another change switches the crumb.
- Drop the "Chat" crumb for change-bound sessions: the trail reads `Changes / <Change Name>` with
  the change as the current location (aria-current and the `#location-current` toggle label) —
  being in the chat IS being in the change. Panel and detail crumbs (Work / Agents / Runtime /
  document titles) stack on top of the change crumb as today and remain the way back; activating
  the change crumb resumes the session in place (a no-op menu close while its chat is open).
  Unbound discussion chats keep `Changes / Chat`, where "Chat" remains the current location.
- With the board gone (UI-04), activating the change crumb stops navigating: it enters the change
  in place via `resumeChangeSession` (chat overlay opens on its session) and closes the menu —
  same behavior as tapping a change card. The `Changes` crumb keeps its `/` navigation; the
  change item loses its `/changes/<id>` href. Deep links to `/changes/<id>` still work through
  UI-04's redirect to `/?change=<id>`, which runs this same resume flow after load.
- Keyboard/AT: the action is a real button, focus returns to the toggle on close (existing
  pattern).

## Verification

- Fresh index page: no Resume row. After entering a change (chat open or closed): row present at
  the bottom; label says Start when the change has no sessions, Resume otherwise.
- Resume continues the last-opened session (per `tt-last-session:<change>`), falls back to newest
  after localStorage is cleared, and creates + opens one when none exist.
- Menu closes and focus lands sensibly after activation; Escape still closes without acting.
- Tapping the change crumb enters the change in place — chat opens on its session with no page
  reload; the `Changes` crumb still navigates to `/`.
- In a change-bound chat the toggle label and current crumb show the change name (not "Chat");
  Work/Runtime panel crumbs stack above it and return to the main view; an unbound discussion
  still shows `Changes / Chat`.
- `go vet ./... && go test ./...` pass.
