# 2026-09-13-1: Embedded terminal improvements

- Change ID: 2026-09-13-1
- Created: 2026-09-13
- Branch: —
- Status: see [ledger.md](ledger.md)

## Objective and context

Two related improvements to the embedded opencode terminal in the browser.

**1. Chrome-free embedded TUI (TUI-00…TUI-02, implemented).** The embedded
opencode TUI (`opencode2 --session <id>` in a PTY) renders two chrome elements
the user does not want in the embedded context: the session tab strip across
the top and the session information sidebar on the right. OpenCode V2's CLI
config (`~/.config/opencode/cli.json`, schema `https://opencode.ai/v2/cli.json`)
exposes exactly the two switches needed — `tabs.enabled` (boolean) and
`session.sidebar` (`"auto" | "hide"`) — verified against the published schema.
lessmess manages a dedicated CLI config for the PTYs it spawns so embedded
sessions are always chrome-free without touching the user's global `cli.json`.

**2. lessmess-native task panel (TUI-03…TUI-05).** When a terminal is open on
a change board, a lessmess-native column on the right of the terminal window
(~20% width) lists the change's tasks with their statuses and refreshes live
in the background — no full-page refresh, no TUI-native pane. The board page
already holds all task data in the DOM and already live-refreshes it via the
existing SSE → debounced `refreshBoard()` → htmx swap pipeline, so the panel
is a client-side mirror of that data with zero new server surface.

## Current behavior

`terminalWS` (`internal/server/terminal.go`) spawns the TUI via
`terminal.Manager.Spawn` (`internal/terminal/terminal.go`), which inherits the
server process environment wholesale. The TUI therefore reads the user's real
`~/.config/opencode/cli.json`; with default settings it shows the tab strip
(`tabs.enabled` defaults to on) and the sidebar (`session.sidebar` defaults to
`"auto"` — shown when space permits). `Spawn` has no way to inject environment
variables.

## Target behavior

Every embedded TUI spawn gets `XDG_CONFIG_HOME` pointed at a lessmess-owned
directory (`.lessmess/xdg/`) containing `opencode/cli.json` with
`tabs.enabled: false` and `session.sidebar: "hide"` merged **over** the user's
own `cli.json` (theme, keybinds, and all other keys preserved). The user's real
config file is never read from or written to by anything except the merge read.
If generation fails for any reason, the spawn falls back to today's behavior
(inherit environment) with a logged warning — cosmetic config must never block
the terminal.

## Scope

- A generator in `internal/server` that builds the merged `cli.json` under
  `.lessmess/xdg/opencode/` (atomic write, per-spawn regeneration).
- Environment injection support in `internal/terminal` and wiring in
  `terminalWS`.
- Unit tests for merge/generation and the spawn wiring.
- A task panel inside the terminal window (`layout.html` markup, `app.css`
  styling): ~20% of the terminal width on the right, visible whenever a
  terminal is open on a change board.
- Client logic in `app.js`: render the panel from the board DOM (grouped by
  the six statuses in board column order, priority order within each group,
  empty groups hidden), keep it synced after every board swap, and open the
  task detail modal when a row is clicked.
- `#detail` z-index raised above the terminal overlay so task details open on
  top of the terminal.
- README documentation of both behaviors.
- Manual verification with a live server (browser checks + XDG side-effect
  check).

## Non-goals

- No writes to the user's real `~/.config/opencode/cli.json`.
- No UI toggle or lessmess-level option for the TUI chrome (always-on for
  embedded sessions; a follow-up change can add configurability if wanted).
- No switch to `opencode2 mini` or other interface changes.
- No changes to the opencode service, session mapping, or prompt priming.
- The task panel is read-only apart from opening task details: no dragging,
  no status edits, no add-task form inside it.
- No panel for unassigned terminals (index Discussions, explorer chats) and
  no collapse toggle for the panel (always-on; can be revisited).

## Design decisions

- **Per-spawn generation, not once at startup.** Reading + writing one small
  JSON file per spawn is negligible and keeps the merged config in sync when
  the user edits their `cli.json` while the server runs.
- **`.lessmess/xdg/` as the config root.** `.lessmess/` is the existing
  gitignored tooling-state home (per AGENTS.md); the generated config is
  tooling state, not repo data.
- **Merge over replace.** The override file must preserve unrelated user keys
  (theme, keybinds, scroll, …) so embedded sessions don't lose the user's look.
  Only `tabs.enabled` and `session.sidebar` are forced.
- **Best-effort JSONC tolerance.** Machine-written `cli.json` is strict JSON;
  if the user file has comments/trailing commas and can't be parsed after a
  conservative strip, fall back to an overrides-only file and log a warning
  (better minimal chrome with default theme than a broken terminal).
- **Fail open.** Any error in read/merge/write → log and spawn without the env
  override (current behavior).
- **Additive terminal API.** Add `SpawnWithEnv` (or equivalent) rather than
  changing `Spawn`'s signature, so existing callers/tests keep compiling;
  `Spawn` delegates with nil env (= inherit).
- **XDG blast radius is verified, not assumed.** Redirecting
  `XDG_CONFIG_HOME` could in principle hide global opencode config from the TUI
  *process*; the background service owns providers/permissions, and project
  `opencode.json` resolves from the working directory (unaffected). Verified
  in TUI-01: `opencode2 debug config` output is byte-identical with and
  without the override, so only `cli.json` follows `XDG_CONFIG_HOME`.
- **Panel data comes from the board DOM, not a new endpoint.** The board
  fragment (`boardFragment`) is a stable contract — `.cards[data-status]`
  columns containing `.card[data-task]` rows with `.card-title` anchors whose
  `hx-get` points at the task detail. The panel mirrors it, and refresh
  piggybacks on the existing SSE → `refreshBoard()` → `htmx:afterSwap` flow,
  so live updates need no new server code and never reload the page.
- **Panel shows only for change terminals.** `openTerminal` derives this from
  `page === "board"` plus a present `#board[data-change]`; index Discussions
  and explorer chats keep the plain full-width terminal.
- **Grouped by status, always-on, ~20%.** Groups follow the board's column
  order with a header and count each, empty groups hidden, rows in ledger
  priority order within a group; width 20% with a min-width floor; the
  existing `ResizeObserver` on `#terminal-container` refits xterm and resizes
  the PTY automatically when the panel appears.
- **Click opens task detail above the terminal.** Panel rows reuse the card
  anchor's `hx-get` URL targeting `#detail`; `#detail`'s z-index (20) moves
  above the terminal overlay (30) so the modal opens on top and closing it
  returns to the terminal.

## Implementation approach

1. `internal/server/tuiconfig.go`: `ensureTUIConfig(stateDir string) (xdgDir string, err error)`
   — reads the user's `cli.json` (if any), overlays the two forced keys,
   writes `.lessmess/xdg/opencode/cli.json` atomically (`model.WriteFileAtomic`),
   returns the dir to use as `XDG_CONFIG_HOME`.
2. `internal/terminal`: add env-aware spawn (nil env = inherit).
3. `terminalWS`: call the generator before spawn; on success pass
   `XDG_CONFIG_HOME=<xdgDir>`, on error log and spawn as today.
4. `layout.html`: flex row under the terminal head — `#terminal-container`
   (flex 1) plus `#terminal-tasks` (20%, min-width floor, hidden by default);
   `app.css` styles for groups, rows, status pills; `#detail` z-index above
   the overlay.
5. `app.js`: `syncTerminalTasks()` builds the panel from `#board`, wires
   `htmx.process`, runs on `openTerminal` (board pages) and board swaps.
6. Tests + README + manual verification.

## File-level impact

- `internal/server/tuiconfig.go` (new), `internal/server/tuiconfig_test.go` (new)
- `internal/server/terminal.go` (wire generator into `terminalWS`)
- `internal/terminal/terminal.go` (env-aware spawn)
- `internal/terminal/terminal_test.go` (new: env injection coverage)
- `web/templates/layout.html` (panel markup), `web/static/app.css` (panel
  styles + `#detail` z-index), `web/static/app.js` (panel mirror + sync)
- `README.md` (embedded-terminal and task-panel docs)
- `.lessmess/xdg/opencode/cli.json` (generated at runtime; already gitignored)

## Data, API, and configuration changes

- No workflow (`changes/`) format changes; no HTTP API changes.
- New runtime-generated file `.lessmess/xdg/opencode/cli.json` (tooling state).
- Embedded PTY processes get one extra env var, `XDG_CONFIG_HOME`.

## Safety, security, and rollback

- The merge only ever **reads** the user's config; writes stay inside
  `.lessmess/` and are atomic.
- `XDG_CONFIG_HOME` is injected only into the spawned TUI process, not the
  server.
- Rollback: delete the wiring; the generated directory is inert if unused.

## Testing and verification strategy

- Unit tests: merge with missing user file, strict-JSON user file (keys
  preserved, overrides win), unparseable/JSONC user file (overrides-only
  fallback), atomic write location; env-aware spawn passes env (and nil =
  inherit); `terminalWS` falls back cleanly when generation fails.
- `go vet ./... && go test ./...`.
- Manual: rebuild binary, open a session terminal in the browser — no tab
  strip, no sidebar, theme preserved; confirm via `opencode2 debug paths` what
  the XDG override does and does not move; confirm a permission-gated action
  still behaves per the repo's `opencode.json`.

## Observability requirements

- One warning log line when config generation fails (including the cause);
  silent success path otherwise (spawns are already logged).

## Rollout sequence

Single change; merge logic first, then wiring, then docs + manual verification.

## Risks and mitigations

- **XDG override hides needed global config from the TUI process** → verified
  in TUI-01/TUI-02; if it breaks behavior, fall back to documenting the manual
  `cli.json` snippet instead of the env override.
- **User config is JSONC and resists parsing** → conservative comment/comma
  strip, then overrides-only fallback with a warning; never spawn nothing.

## Acceptance criteria

1. Embedded terminal sessions render without the tab strip and without the
   right sidebar, regardless of the user's own `cli.json`.
2. A user's theme/keybinds in `~/.config/opencode/cli.json` still apply in the
   embedded terminal.
3. The user's real `cli.json` is byte-identical before and after; all writes
   stay under `.lessmess/`.
4. With the generator forced to fail, the terminal still opens (today's
   chrome) and a warning is logged.
5. Opening a terminal on a change board shows the task panel at ~20% width on
   the right; opening a Discussion or explorer-chat terminal does not.
6. The panel reflects task/status edits (agent ledger writes, board drags,
   added tasks) within the existing debounce, without a page reload and
   without disturbing the terminal session.
7. Clicking a panel row opens the task detail modal above the terminal;
   closing it returns to the session.
8. `go vet ./... && go test ./...` pass; README documents both behaviors.

## Tasks

1. [TUI-00](tasks/00-merged-tui-config-generator.md) — Merged TUI cli.json generator
2. [TUI-01](tasks/01-spawn-env-injection-and-wiring.md) — Spawn env injection and terminalWS wiring
3. [TUI-02](tasks/02-docs-and-manual-verification.md) — Docs and manual verification
4. [TUI-03](tasks/03-task-panel-layout-and-styling.md) — Task panel layout and styling
5. [TUI-04](tasks/04-task-panel-client-mirror.md) — Task panel client mirror and sync
6. [TUI-05](tasks/05-panel-docs-and-manual-verification.md) — Panel docs and manual verification
