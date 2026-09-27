# 2026-09-27-hc6f8: One-click chat revert

- Change ID: 2026-09-27-hc6f8
- Created: 2026-09-27
- Branch: —
- Status: tracked in the tool-owned JSON state (.lessmess/workflow/)

## Objective and context

Reverting a chat conversation to an earlier user message currently takes four-plus steps spread across the Controls drawer: pick "Revert to here" in the message menu, read a read-only preview panel, tick a files checkbox, click "Stage this revert", then type a fingerprint string to unlock "Commit revert" behind a `window.confirm`. The fingerprint is pure client-side friction — the server never sees it; its real guard is a `confirmation` payload (`sessionID`, `updated`, `messageID`) the client already holds.

Goal: clicking "Revert to here" opens a confirm dialog immediately, one click on Confirm performs the revert, and afterwards the transcript truncates to before that message while the reverted message's text lands in the composer, focused and ready to edit/resend.

## Current behavior

- `partials.html` renders "Revert to here" (`data-chat-revert`) only on user messages' action menus.
- `app.js` `previewChatRevert()` opens the Controls drawer (`openChatControls()`) and renders a read-only preview (transcript range + affected-files diff from `GET /api/sessions/{id}/chat/diff?from=`) into `#chat-revert-panel`, plus a files checkbox.
- "Stage this revert" posts `POST /api/sessions/{id}/revert/stage` (`messageID`, `files`, confirmation `sessionID`+`updated`); the panel then demands the user retype a fingerprint (`sessionID:revertMessageID:updated`) before "Commit revert" (plus `window.confirm`) posts `POST /api/sessions/{id}/revert/commit` with confirmation `sessionID`+`updated`+`messageID`.
- `POST /api/sessions/{id}/revert/clear` cancels a staged revert; `internal/server/sessionlifecycle.go` owns all four routes; capabilities (`revertStage`/`revertCommit`/`revertClear`) gate availability.
- After commit, `app.js` calls `refreshChat()`; the poll snapshot reflects the truncated history. Nothing restores the reverted message's text to the composer.

## Target behavior

- "Revert to here" opens a dedicated confirm dialog (Commit-all-modal pattern in `layout.html`, ids wired in `app.js`) without opening the Controls drawer. Contents: message excerpt, collapsible affected-files summary (lazily fetched from the existing diff endpoint), an "Also restore file changes" checkbox defaulting to **unchecked** (history-only by default), and the caveat that file restoration after clearing a staged revert is not guaranteed.
- Confirm posts one new combined endpoint, `POST /api/sessions/{id}/revert`, which server-side stages (with the chosen `files` flag), re-reads session state, and commits — same entry `confirmation` guard and capability gating, one round trip, no client-visible staged interval.
- Dialog shows "Reverting…" with controls disabled while in flight; on success it closes, `refreshChat()` truncates the transcript via the existing poll, the reverted user message's raw text (from the message's hidden `.chat-markdown-source` span, falling back to rendered text) is placed in `cstate.drafts` + `#chat-prompt`, and the composer is focused. Status line confirms the revert.
- The staged-revert box in the Controls drawer remains solely as a stale-stage recovery path (e.g., combined endpoint failed between stage and commit); its copy points there. The preview/stage/fingerprint flow and the lifecycle fingerprint input are deleted. The `layout.html` lifecycle warning text is updated to match the new flow.
- Revert stays blocked while OpenCode is busy; unsupported services keep a disabled/gated affordance (`capabilities.revertStage`/`revertCommit` false → menu item disabled or dialog explains unavailability).

## Scope

- `internal/server/sessionlifecycle.go` (+ route registration in `internal/server/server.go`): new combined `POST /api/sessions/{id}/revert` handler with Go tests.
- `web/templates/layout.html`: confirm-dialog markup, updated lifecycle warning copy; `web/templates/partials.html` only if menu markup needs a disabled state hook.
- `web/static/app.js`: dialog wiring, combined-endpoint call, composer restore on success; delete `previewChatRevert`/staging/fingerprint UI paths (`renderChatLifecycle` revert branches, click handlers, `lifecycleFingerprint` input gate).
- `web/static/app.css`: dialog styling consistent with the Commit modal.
- Render tests (`internal/server/*_test.go`) that pin revert markup/copy.

## Non-goals

- No change to OpenCode revert semantics: the chosen message and everything after it is removed; files restore only when the checkbox is ticked.
- No change to fork, compact, or other lifecycle flows.
- No new event-stream/subsystem work — transcript freshness stays poll-based.
- No staged-revert API removal: the existing stage/commit/clear endpoints stay for the recovery path.

## Design decisions

- Combined server-side endpoint over client-side chaining: avoids the extra lifecycle round-trip between stage and commit (commit's confirmation needs post-stage `updated`), removes the race window where a staged revert sits idle, and keeps the client to one request.
- Real dialog instead of `window.confirm`: the files checkbox cannot live in a native confirm; the Commit-all modal is the established in-app pattern.
- Files checkbox defaults to unchecked (user decision): history-only by default, working tree untouched unless explicitly opted in.
- Fingerprint typing dropped: it was client-only friction; the server's `confirmation` payload guard remains the real concurrency check.

## Acceptance criteria

- Clicking "Revert to here" shows the confirm dialog; the Controls drawer does not open.
- Confirm performs the whole revert in one user action; no fingerprint typing, no separate stage click.
- After success: transcript shows history truncated to before the chosen message; the composer contains that message's text, focused; status line confirms.
- "Also restore file changes" defaults to unchecked; ticking it restores files (when the service supports it).
- Busy sessions block Confirm with the existing disabled-state behavior; capability-gated services see an unavailable explanation instead of a broken flow.
- A failure after staging (stale stage) is surfaced with a pointer to the Controls-drawer recovery box, which still works to commit or clear.
- `go vet ./... && go test ./...` clean; `lessmess validate` clean.

## Tasks

1. (task breakdown is maintained by the tool; see the board)
