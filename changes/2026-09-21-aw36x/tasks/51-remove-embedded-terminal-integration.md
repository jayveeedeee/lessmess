# MAC-51: Remove embedded terminal integration

## Objective

Remove the embedded OpenCode terminal now that API Chat is the sole session UI.

## Dependencies

- MAC-50.

## Scope

- Open every session in Chat at all viewport sizes.
- Remove terminal buttons, overflow entries, overlay markup, browser behavior,
  styling, and bundled terminal-only assets.
- Remove the terminal WebSocket route, PTY bridge, managed TUI configuration,
  terminal-only settings, and dedicated backend package/dependencies.
- Update tests and user-facing documentation.
- Preserve OpenCode shell messages/tools shown inside Chat; those are not the
  embedded terminal integration.

## Implementation steps

1. Inventory terminal integration boundaries and retain generic shell support.
2. Cut session navigation over to Chat and remove frontend terminal surfaces.
3. Remove server/runtime terminal plumbing, settings, assets, and dependencies.
4. Update contracts and documentation, then run full verification.

## Verification

- Full tests and vet.
- JavaScript syntax, build, workflow validation, and diff check.
- Confirm no terminal route, UI hook, setting, or bundled xterm reference
  remains and existing sessions open in Chat on desktop and mobile.

## Completion criteria

lessmess no longer embeds or serves an OpenCode terminal; Chat is the only
session interface.

## Files affected

- `internal/server/server.go`
- `internal/server/render.go`
- `internal/server/settings.go`
- `internal/server/settingschange.go`
- `internal/server/chat.go`
- `internal/server/mapping.go`
- `internal/server/closepipeline.go`
- `internal/server/prereqs.go`
- `internal/server/*_test.go`
- `internal/server/terminal.go` (removed)
- `internal/server/terminal_test.go` (removed)
- `internal/server/tuiconfig.go` (removed)
- `internal/server/tuiconfig_test.go` (removed)
- `internal/terminal/` (removed)
- `web/templates/layout.html`
- `web/templates/settings.html`
- `web/templates/setup.html`
- `web/static/app.js`
- `web/static/app.css`
- `web/static/VENDOR.md`
- `web/static/xterm*` (removed)
- `go.mod`
- `go.sum`
- `README.md`
- `changes/2026-09-21-aw36x/plan.md`

## Notes

- Removed the browser xterm overlay, WebSocket/PTY server path, managed TUI
  config, terminal setting, vendored assets, Go dependencies, and dedicated
  terminal package.
- All session entry points now call Chat at every viewport width; the shared
  Work context and published shell/tool transcript rendering remain intact.
- Verified with `go vet ./...`, `go test ./...`, `node --check
  web/static/app.js`, `git diff --check`, a static `CGO_ENABLED=0` build, and
  served-binary checks confirming `/terminal/ws` and `/static/xterm.min.js`
  return 404 and `/api/settings` omits `autoOpenTerminal`.
