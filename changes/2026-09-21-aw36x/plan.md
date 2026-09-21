# 2026-09-21-aw36x: Mobile API Chat

- Change ID: 2026-09-21-aw36x
- Created: 2026-09-21
- Branch: —
- Status: tracked in the tool-owned JSON state (.lessmess/workflow/)

## Objective and context

Make lessmess sessions fully usable from a phone by adding a structured,
mobile-first chat interface backed by the OpenCode HTTP API. The chat runs in
parallel with the existing embedded OpenCode terminal: both open the same
session, and the terminal remains available and unchanged.

Mobile access is assumed to be over a trusted internal network or VPN.
lessmess remains bound to localhost by default; public-internet exposure is not
part of this change.

## Current behavior

- Session creation, mapping, task association, and lifecycle operations already
  use the OpenCode service API.
- Continuing or talking to a session opens `opencode2 --session <id>` in a local
  PTY bridged to browser xterm.js.
- That terminal is effective on desktop but awkward with touch keyboards and
  small screens, and the PTY dependency blocks native Windows terminal use.
- The Go client does not currently expose messages, active state, interrupt,
  permissions, or forms.

## Target behavior

- Every mapped session offers both **Chat** and **Terminal** actions.
- Chat opens a full-screen responsive view of the same persistent OpenCode
  session, without spawning a PTY.
- The user can read the transcript, send prompts, see running/tool/error state,
  interrupt work, answer permission requests, and answer structured forms.
- Reopening or reconnecting reloads authoritative session state from OpenCode;
  no transcript is persisted by lessmess.
- On narrow screens the composer remains reachable above the virtual keyboard,
  controls are touch-sized, and associated tasks are available in a collapsible
  drawer rather than consuming permanent horizontal space.
- Unknown future OpenCode message parts render as a safe fallback instead of
  breaking the conversation.

## Scope

- Extend the existing thin Go OpenCode client only for the session operations
  needed by chat.
- Add narrow lessmess handlers that authenticate to OpenCode server-side and
  validate all mutations; do not expose OpenCode credentials to the browser.
- Use an initial transcript fetch plus modest polling while chat is visible.
  This deliberately avoids a reconnecting event-stream subsystem; each refresh
  converges on the authoritative transcript and pending interactions.
- Render text, code, reasoning, tool progress/results, errors, permissions, and
  forms as accessible structured blocks.
- Add a chat overlay and reuse the existing session bindings and terminal task
  context. Preserve unsent draft text and avoid stealing scroll position when
  the user has scrolled up.
- Cover API decoding, handler validation, interaction mutations, reconnect
  behavior, responsive layout, and terminal regression behavior.
- Update user-facing documentation for Chat versus Terminal and trusted-network
  mobile access.

## Non-goals

- Replacing or removing the existing terminal integration.
- Reproducing every OpenCode screen in the first release: attachments, session
  fork/revert/compaction, integration and MCP management, provider credentials,
  shell/PTY controls, and advanced diagnostics can follow based on use.
- Adding React or another frontend framework, a Node build, generated clients,
  or a new lessmess persistence layer.
- Native Windows packaging, ConPTY support, authentication, TLS, public hosting,
  or automatic VPN setup.

## Design decisions

1. **Parallel modes, one session.** Chat and Terminal are two clients for the
   same OpenCode session ID; creating a special mobile session would fragment
   context and workflow mappings.
2. **Server-side proxy boundary.** The browser calls explicit lessmess routes;
   lessmess calls OpenCode with its existing credentials. There is no generic
   pass-through endpoint.
3. **Polling before streaming.** A full refresh on open/reconnect plus polling
   while visible is simpler and cannot miss live-only events. The implementation
   should avoid overlapping requests and stop polling when hidden.
4. **Normalized rendering with fallback.** lessmess models the common message
   and interaction shapes needed for a good UI, while retaining enough raw type
   information to show an unknown-part placeholder safely.
5. **Existing frontend stack.** Add focused template, CSS, and vanilla-JS code
   beside the current overlay. Reuse the shared markdown renderer and task-panel
   behavior where practical rather than introducing a client application.
6. **Private access assumption.** Mobile use is supported on a trusted LAN or
   VPN. The default bind remains `127.0.0.1`; operators deliberately choose any
   broader bind.

## Phased roadmap

This is an umbrella change containing all five phases. Work proceeds through
the dependency-gated tasks in order, and each phase must be used and verified
before the next begins. Terminal stays available throughout; removing it or
changing the desktop default requires a separate user decision.

### Phase 1 — Core mobile conversation

Deliver the smallest complete agent loop on a phone:

- Transcript, prompts, reasoning, tool progress/results, and errors.
- Busy state, interruption, permissions, and structured forms.
- Reconnect-safe polling, draft preservation, intentional scrolling, and task
  context.
- Explicit Chat and Terminal entry points for the same session.

Gate: a user can start, supervise, unblock, and continue normal repository work
from a phone without opening the terminal. Unknown API parts fail visibly and
safely. MAC-00 through MAC-03 are the implementation tasks for this phase.

### Phase 2 — Rich coding-session controls

Bring the common desktop coding controls into Chat:

- File and image attachments, project references, and copy/download actions.
- Detailed tool cards with inputs, output, affected files, and readable diffs.
- Agent and model switching with the same project-aware options and fallback
  behavior as session creation.
- Commands and skills exposed through compact mobile pickers rather than a
  terminal command palette clone.
- Context/token usage, compaction warning, and clear provider/retry failures.
- Transcript pagination or incremental fetch so long-running sessions remain
  fast on mobile connections.

Suggested task split: OpenCode contract additions; attachments/references;
tool-and-diff presentation; agent/model/command/skill controls; long-session
performance and mobile verification.

Gate: routine coding sessions, including reviewing edits and changing how the
agent runs, no longer require Terminal. Large transcripts and diffs remain
usable at phone widths.

### Phase 3 — Complete session lifecycle and multi-agent work

Cover recovery and advanced session operations:

- Fork a session at a selected message.
- Stage, inspect, commit, and cancel session reverts with destructive-action
  confirmation.
- Manual compaction and interruption/retry recovery.
- Inbox/queued follow-ups and steering while an agent is busy.
- Parent/child session navigation, subagent activity, pending inputs, and quick
  movement between a task session and its children.
- Session rename, export, delete/unlink distinctions, and clear ownership of
  change/task mappings.
- Mobile-safe shell output only where it is part of a session operation; this
  does not recreate an interactive terminal.

Suggested task split: lifecycle API and confirmation model; fork/revert/compact
UI; inbox and steering; subagent navigation; export/rename/delete; recovery and
end-to-end verification.

Gate: Chat can recover, branch, compact, steer, and navigate multi-agent work
without losing workflow bindings or requiring the TUI.

### Phase 4 — OpenCode service management

Add the setup and maintenance operations needed to stay mobile-only:

- Integration discovery and key/OAuth/command connection flows.
- MCP list, connect, disconnect, authentication state, and resources.
- Saved permission review/removal and active permission overview.
- Provider/model health, service health, plugin status, and actionable errors.
- Relevant project configuration surfaced through existing lessmess settings;
  do not build a second generic configuration editor.
- Sensitive values remain server-side, secret fields are never echoed, and
  every destructive or credential-changing action requires confirmation.

Suggested task split: service-health foundation; integrations and credentials;
MCP management; saved permissions; mobile security review and live verification.

Gate: a user can diagnose and resolve the common conditions that block a
session from a phone. Unsupported administrative endpoints are documented in a
maintained feature matrix rather than silently absent.

### Phase 5 — Native Windows and packaged releases

Make the API-driven experience easy to install on all target platforms:

- Fix platform-aware OpenCode service discovery and credential locations.
- Add native Windows CI for filesystem watching, atomic writes, Git worktrees,
  path handling, process shutdown, and the complete Chat flow.
- Publish signed/checksummed macOS, Linux, and Windows binaries for supported
  architectures through automated GitHub Releases.
- Add Homebrew plus a Windows package channel such as Winget or Scoop, with
  upgrade and uninstall documentation.
- Keep native Windows Terminal disabled or clearly labelled until a separate
  ConPTY implementation is proven; Chat is the supported Windows session UI.
- Verify first-run prerequisites and onboarding on clean machines.

Suggested task split: release/version foundation; Windows compatibility;
cross-platform CI/release artifacts; Homebrew; Windows package; clean-machine
installation verification.

Gate: a new user can install lessmess without a compiler, complete onboarding,
and run the full API-driven workflow on macOS, Linux, or Windows. WSL is optional,
not required.

### Mobile hierarchy refinement

Before treating the existing mobile surface as accepted, refine its information
hierarchy so Chat is the only persistent compact-width pane:

- Work, plan, tasks, agents, controls, and management use full-width temporary
  views with explicit return navigation instead of side drawers.
- Mobile document contents move from a left rail to a sticky top disclosure.
- The composer prioritizes the message field and Send; file, reference, and
  skill actions move behind one `+` menu, while Interrupt appears only when busy.
- The compact header keeps title/state and contextual Work or Agents actions;
  Terminal and infrequent controls move into overflow.
- Desktop Chat permits only one auxiliary panel at a time and protects the
  conversation's minimum readable width. Terminal remains unchanged.

Tasks MAC-25 through MAC-31 implement and verify this refinement. They depend
on the completed Chat foundation through MAC-19 and are independent of the
unauthorized Phase 5 packaging work.

### Cross-phase rules

- Each phase starts only after the prior gate is verified with a real OpenCode
  service and phone-sized browser.
- Prefer explicit endpoint adapters and small UI controls over a generic API
  proxy or a clone of the OpenCode TUI.
- Add capabilities to the existing Chat surface; do not introduce another
  frontend application or persistence layer.
- Keep a feature matrix from Phase 2 onward so “mobile complete” has a testable
  meaning as OpenCode's experimental API evolves.
- LAN/VPN remains the deployment boundary. Public authentication and TLS are a
  separate roadmap only if deployment requirements change.

## Acceptance criteria

- From a phone-sized viewport, a user can start or resume a mapped session,
  read its complete transcript, send a follow-up, observe progress, interrupt
  it, resolve a permission request, and answer a structured form.
- Text, code, reasoning, tool activity/results, and errors remain readable at
  375px and 390px viewport widths without horizontal page scrolling.
- The composer remains usable with the virtual keyboard, drafts survive
  transcript refreshes, and refreshes do not force a reader back to the bottom.
- Closing and reopening Chat restores server-authoritative history and pending
  interactions without duplicates or lessmess-owned transcript state.
- A malformed or unknown OpenCode message part degrades to a safe visible
  fallback and cannot inject HTML.
- OpenCode credentials never appear in browser responses or JavaScript.
- Existing terminal opening, input, resize, task panel, and desktop behavior
  continue to work.
- `go vet ./...`, `go test ./...`, the static build, and focused mobile manual
  checks pass; README documents the two session modes and private-network use.
- The umbrella change closes only after all five phase gates are met, clean
  package installs are verified on each supported OS, and the maintained
  feature matrix accounts for intentionally unsupported OpenCode capabilities.

## Tasks

1. (task breakdown is maintained by the tool; see the board)
