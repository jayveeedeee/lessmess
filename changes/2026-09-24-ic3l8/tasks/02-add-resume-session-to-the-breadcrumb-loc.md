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
- ~~A separate "Resume session" row~~ Revised during review: the row showed
  permanently once any change context existed (persisted per visit) and duplicated the crumb's
  action, so it was removed. The change crumb itself doubles as the resume affordance — tooltip
  "Resume session", accent-tinted — and activating it enters the change
  (continues its session) from anywhere, including while it is the current location.
- Revised on user review (second iteration): no persisted change state. The change crumb exists
  only while that change's session is open in Chat — closing the chat or returning to the list
  leaves just `Changes`, and re-entry is the change cards. (An earlier sessionStorage-backed
  "persists for the visit" crumb read as stale, always-selected state in the breadcrumbs.)
- Drop the "Chat" crumb for change-bound sessions: the trail reads `Changes / <Change Name>` with
  the change as the current location (aria-current and the `#location-current` toggle label) —
  being in the chat IS being in the change. Panel and detail crumbs (Work / Agents / Runtime /
  document titles) stack on top of the change crumb as today and remain the way back; activating
  the change crumb resumes the session in place (a no-op menu close while its chat is open).
  Unbound discussion chats keep `Changes / Chat`, where "Chat" remains the current location.
- Activating the change crumb while its chat is open returns to the main chat view (closing
  panels); the `Changes` crumb keeps its `/` navigation. Deep links to `/changes/<id>` work
  through UI-04's redirect to `/?change=<id>`, which resumes the change after load.
- Revised on user review: with a plan/task document open above Chat, breadcrumb activation
  (change crumb or the back chevron) closes the document first — back means back to the
  conversation, not a no-op behind the modal.
- Keyboard/AT: the action is a real button, focus returns to the toggle on close (existing
  pattern).

## Verification

- Fresh index page: no Resume row. After entering a change (chat open or closed): row present at
  the dropdown shows the change crumb (accent-tinted, "Resume session" tooltip) whenever a change
  context exists; activating it resumes or creates the session.
- Resume continues the last-opened session (per `tt-last-session:<change>`), falls back to newest
  after localStorage is cleared, and creates + opens one when none exist.
- Menu closes and focus lands sensibly after activation; Escape still closes without acting.
- Tapping the change crumb enters the change in place — chat opens on its session with no page
  reload; the `Changes` crumb still navigates to `/`.
- In a change-bound chat the toggle label and current crumb show the change name (not "Chat");
  Work/Runtime panel crumbs stack above it and return to the main view; an unbound discussion
  still shows `Changes / Chat`.
- After closing the chat or navigating back, the breadcrumb reads just `Changes` — no stale
  change crumb, no persisted selection across reloads.
- `go vet ./... && go test ./...` pass.
