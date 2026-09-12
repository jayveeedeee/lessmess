---
id: NAV-03
title: Prominent Continue/Start session button
---

# NAV-03: Prominent Continue/Start session button

Status: see [../ledger.md](../ledger.md).

## Objective

Give the board page a single, prominent, always-visible button that resumes
the last-opened session in one click — or creates and opens a session when the
change has none.

## Dependencies

— (none; independent of NAV-00/01/02, shares `board.html`/`app.js` surface)

## Scope

- `web/templates/board.html` — new first control in `.board-head`.
- `web/static/app.js` — label state, click behavior, last-opened tracking.
- `web/static/app.css` — only if `btn-accent` needs a prominence tweak.
- No server or mapping changes.

## Implementation steps

1. In `board.html`, add `<button type="button" class="btn-accent"
   id="continue-session-btn">Continue session</button>` as the first button in
   `.board-head` (before Plan/Sessions), so it reads as the primary action.
2. In `app.js`, on board init fetch `GET /changes/{id}/sessions` (reuse the
   `loadSessions` fetch path): with zero sessions set the label to "Start
   session", otherwise "Continue session".
3. Track last-opened per change: whenever a session terminal is opened from
   the board page (panel Open, New session, or this button), store
   `localStorage["tt-last-session:" + changeID] = sessionID`.
4. Click behavior: read the stored ID and validate it against the fetched
   list; if present → `openTerminal(storedID, title)`; else if the list is
   non-empty → open the last entry (newest created) and update the stored ID;
   else → `POST /changes/{id}/sessions` (existing create flow), then open the
   new session. Reuse the existing `.then/.catch` + alert error pattern.
5. Guard double-clicks while the create request is in flight (disable +
   re-enable), matching existing button conventions.

## Verification

1. `go build ./...`; `go test ./...` passes (`render_test.go` asserts board
   markup — keep existing IDs stable).
2. Board with sessions: one click opens the last-opened session in the
   terminal overlay; open a different session from the panel, reload, and the
   button resumes that one. Unlink the stored session: the button falls back
   to the newest created.
3. Board without sessions: the button says "Start session" and creates + opens
   one (requires the opencode service; otherwise the existing alert path
   shows).
4. Button is visually the loudest element in the board header in both themes.

## Completion criteria

- One-click resume works for the three cases above; label reflects state;
  sessions panel and other board controls unchanged.

## Files affected

- `web/templates/board.html`
- `web/static/app.js`
- `web/static/app.css` (if needed)

## Notes

- User picked: always-visible dual-mode button; "last session" = most recently
  opened, tracked client-side in `localStorage` (single-user localhost tool),
  validated against the live list with fallback to newest created.
