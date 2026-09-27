# PMF-02: Stage control selections behind an Apply button

## Why

Agent / Model / Variant selects mutate the live session the instant an option is picked (`change` → `chatMutation`). With the provider filter narrowing the list, mis-clicks apply immediately with no confirmation — too precarious. Selections should be deliberate.

## What

- Stage Agent / Model / Variant selections in `cstate.stagedControls` instead of mutating on `change`; the Provider select stays instant (client-side filter only).
- Add a draft row under the selects in `web/templates/layout.html` — `Apply changes` (primary) + `Discard` (ghost) — hidden until the draft differs from session state; visible in both Runtime and full sheet (not `chat-controls-extended`).
- Apply: agent mutation first when changed, then one `model` mutation carrying `{model, variant}` together (endpoint already accepts both); on success clear the draft and reload controls; on failure keep the draft and surface `setChatStatus(err.message, true)`.
- Discard clears the draft and re-renders selects from session truth; the draft also clears on sheet close and session switch.
- Selects whose displayed value differs from session truth get an accent "unsaved" outline; the staged model option is pinned visible under the provider/search filters; the variant list reflects the staged model.
- Update `internal/server/render_test.go` snippets pinning the old instant-apply handlers.

## Files affected

- `web/templates/layout.html`
- `web/static/app.js`
- `web/static/app.css`
- `internal/server/render_test.go`

## Verification

- Pick a model → nothing applies until Apply; Discard reverts; closing the sheet discards; Apply commits model+variant in one request; staged model survives controls re-render and stays visible under filters. Gates: `node --check`, `go vet`, `go test ./...`, build, `lessmess validate`.
