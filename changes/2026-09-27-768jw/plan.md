# 2026-09-27-768jw: Provider filter for runtime model menu

- Change ID: 2026-09-27-768jw
- Created: 2026-09-27
- Branch: —
- Status: tracked in the tool-owned JSON state (.lessmess/workflow/)

## Objective and context

The Runtime menu (compact chat controls sheet) lists every enabled model from every provider in one flat `<select>`, which is unwieldy once several providers are connected. Users want to narrow the list to one provider. The free-text filter input that could partially serve this is part of the extended controls and is hidden in runtime-only mode, so a dedicated provider filter is warranted.

The controls API (`GET /api/sessions/{id}/chat/controls`, `internal/server/chat.go` `chatControls`) already returns `ProviderID` on every model option (`chatModelOpt` → `settingsModelOpt`), so the filter is a frontend-only change.

## Current behavior

- `web/templates/layout.html` renders the shared controls sheet with Agent / Model / Variant selects (`#chat-agent-select`, `#chat-model-select`, `#chat-variant-select`).
- `renderChatControls` (web/static/app.js) fills the model select from `data.models` via `fillChatSelect`; each option's `value` is `providerID/modelID` and the provider ID is embedded in the option's search text only.
- `#chat-controls-search` hides options via `option.hidden` for a free-text query; that input carries `chat-controls-extended` and is suppressed in runtime-only mode (`.chat-controls-sheet.runtime-only .chat-controls-extended { display: none !important; }` in app.css).
- Controls re-render after every mutation (`loadChatControls`), so any UI state must be re-applied on render.

## Target behavior

- A new `Provider` select sits directly above the Model select in the shared controls sheet, visible in both the Runtime sheet and the full Session controls sheet.
- Agent / Model / Variant selections stage as a local draft instead of applying on `change`; an `Apply changes` / `Discard` row (visible in both sheets, hidden while clean) commits or reverts them. Apply sends the agent mutation when changed, then one model mutation carrying model+variant together. The draft clears on apply, discard, sheet close, and session switch. The Provider filter itself remains instant — it never touches session state.
- Options are the distinct, sorted `ProviderID`s of the enabled models, plus a default "All providers" option (value `""`). No models → the select is disabled.
- Choosing a provider filters the Model select to that provider's models. The filter composes with the existing free-text search (one combined visibility pass over model options; agents/commands keep their current filtering path).
- The session's currently selected model option is never hidden by the filters, so the select always displays what the session is actually using even when its provider is filtered out.
- The chosen provider persists across controls re-renders (kept in `cstate`, re-applied in `renderChatControls`) and resets when the chat session changes.

## Scope

- `web/templates/layout.html`: add the Provider select (and label) above the Model row, plus the Apply/Discard draft row under the selects.
- `web/static/app.js`: populate the provider select, combined model-option filtering helper, persistence in `cstate`, reset on session switch; staged control drafts with Apply/Discard.
- `web/static/app.css`: styling for the draft row and the "unsaved" select cue.
- `internal/server/render_test.go`: extend the JS-snippet assertions for the new filter code.

## Non-goals

- No server/API changes; `chatControls` already exposes `ProviderID`.
- No provider grouping or display-name mapping (raw provider IDs are the labels).
- No changes to the Settings page or setup wizard model lists.
- No new event-stream or persistence of the provider choice beyond page state.

## Design decisions

- Frontend-only filtering: the API payload already carries `ProviderID`, so no endpoint work.
- Show the filter in both Runtime and Session controls sheets: they share one DOM tree; splitting visibility would add special-casing for no real benefit.
- Pin the current selection instead of letting filters hide it, so the visible value never lies about the session's model.
- Provider choice lives in `cstate` (page state), not storage: transient preference, reset on session switch.

## Acceptance criteria

- Runtime menu shows a Provider select listing each provider that has at least one enabled model, with "All providers" as default.
- Picking a provider leaves only that provider's models in the Model select (plus the current selection if it belongs to another provider).
- Selecting an agent, model, or variant applies nothing until `Apply changes` is pressed; `Discard` or closing the sheet reverts the draft; Apply commits agent and model+variant (one request) and the controls reload from session truth.
- "All providers" restores the full list; the free-text filter still composes with the provider filter.
- The choice survives the re-render triggered by a model/agent mutation and clears when switching sessions.
- `CGO_ENABLED=0 go build -o lessmess ./cmd/lessmess`, `go vet ./...`, and `go test ./...` pass; `lessmess validate` is clean.

## Tasks

1. (task breakdown is maintained by the tool; see the board)
