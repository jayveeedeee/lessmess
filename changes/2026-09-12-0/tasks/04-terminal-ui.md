
# OCI-04: Terminal UI with xterm.js

Status: see [../ledger.md](../ledger.md).

## Objective

Embed the live opencode TUI in the board UI: sessions panel per change plus a full terminal view using vendored xterm.js.

## Dependencies

OCI-02, OCI-03.

## Scope

In scope: vendor xterm.js + fit addon (pinned, checksummed in VENDOR.md); board "Sessions" panel (list mapped sessions with titles, open/new/delete-unlink actions); terminal view (modal or dedicated pane) attaching xterm.js to the WS bridge; PTY lifecycle (spawn `opencode2 --session <id>` via creack/pty, bridge, kill on close); reconnect on WS drop (fresh PTY on the same session); theme the terminal to match the UI; embedded assets only.
Out of scope: new-change flow (OCI-05), permissions (OCI-06).

## Implementation steps

1. Vendor pinned xterm.js assets; record in VENDOR.md.
2. Sessions panel partial + JS (list from mapping endpoint, actions).
3. Terminal view: xterm.js init, WS wire-up to our own bridge (`/terminal/{ptyID}/ws`), resize/fit, input handling, reconnect logic (fresh in-process PTY on the same session).
4. Style to match the charcoal reference aesthetic; cache-bust bump.
5. Headless-Chrome screenshot of panel; manual terminal smoke in OCI-07.

## Verification

- Panel renders mapped sessions; terminal view opens and attaches (verified live in dogfood); screenshots inspected.

## Completion criteria

- Terminal opens a live opencode TUI for a mapped session in the browser.

## Files affected

- `web/static/` (xterm assets, app.js, app.css), `web/templates/` (partials, board)

## Notes

- Vendored @xterm/xterm 5.5.0 (js + css at `css/xterm.css` — not `lib/`) and @xterm/addon-fit 0.10.0; pinned with SHA-256 in `VENDOR.md` (2026-09-12).
- Live verification (2026-09-12): started the server (log: `opencode service connected`), created a session via `POST /changes/2026-09-12-0/sessions` (title auto-derived from root ledger), connected to `/terminal/ws?session=…` — the bridge spawned the TUI and streamed ~393KB of real TUI output in 15s. Cleaned up (unlinked mapping, deleted opencode session).
- Trap noted: WS clients must set `binaryType = "arraybuffer"` before messages arrive (undici/Node default is Blob, which silently swallowed output in my first probe). Browser code in `app.js` sets it immediately.
- `opencode2 --session <id>` in a bare PTY boots and emits output immediately even with terminal capability queries unanswered (verified via `script(1)` capture too).
- The validator banner caught a real root-ledger drift during the smoke (root row for this change still said `Planned` while the ledger said `In progress`) — fixed the row; second time the tool has caught exactly this class of my own error.

