# PMF-00: Add provider filter to chat controls sheet

## Why

The Runtime menu's Model select lists every enabled model across all providers in one flat list. With several providers connected this is hard to scan, and the extended free-text filter is hidden in runtime-only mode, so there is no way to narrow the list.

## What

- Add a `Provider` select (label + `<select id="chat-provider-select">`) directly above the Model row in the shared controls sheet in `web/templates/layout.html`, visible in both Runtime and full Session controls modes.
- In `web/static/app.js`:
  - Populate the provider select in `renderChatControls` from the distinct sorted `ProviderID`s of `data.models`, with a default "All providers" option (value `""`); disable it when there are no models.
  - Add a combined visibility helper for model options applying both the provider choice and the existing free-text query; rewire the model select's filtering through it (agents/commands keep their current path).
  - Never hide the option matching the session's current `data.model`.
  - Keep the chosen provider in `cstate` so it survives re-renders after mutations; reset it when the chat session changes.
- `web/static/app.css`: only touch if the new row needs styling to match the existing rows.

## Files affected

- `web/templates/layout.html`
- `web/static/app.js`
- `web/static/app.css` (only if needed)

## Verification

- Rebuild (`CGO_ENABLED=0 go build -o lessmess ./cmd/lessmess`), restart, open the Runtime menu: Provider select lists each provider with models; picking one narrows the Model list; "All providers" restores it; current model stays visible when its provider is filtered out; the choice survives a model mutation re-render and clears on session switch.
