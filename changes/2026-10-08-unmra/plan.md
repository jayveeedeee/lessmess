# 2026-10-08-unmra: First-class chats

- Change ID: 2026-10-08-unmra
- Created: 2026-10-08
- Branch: —
- Status: tracked in the tool-owned JSON state (.lessmess/workflow/)

## Objective and context

Make standalone conversations first-class in lessmess: give them a dedicated
Chats section, meaningful automatic names, and an explicit path into tracked
change work without losing the conversation that established the context.

The user approved scaffolding and planning this change. Implementation requires
a separate explicit go-ahead; creating this plan and its task breakdown does not
authorize implementation or status transitions.

## Current behavior

- `web/templates/index.html` places a client-rendered Discussions list below
  the change cards. `web/static/app.js` loads it from `GET /api/discussions`.
- The header Chat button calls `POST /chat/session` and opens a new root-scoped
  free-form session with the fixed title "Codebase chat" (`internal/server/chat.go`).
- Change-planning discussions, explorer chats, settings discussions, repository
  commit helpers, and some forks share the `_unassigned` bucket in the personal,
  gitignored `.lessmess/sessions.json` mapping (`internal/server/mapping.go`).
  Being unassigned is not sufficient to identify a general-purpose chat.
- Free chats can edit the current checkout when explicitly asked; they do not
  require tracked change bookkeeping. New-change discussions remain read-only
  before approval to scaffold.
- `POST /changes/scaffold` already creates a change, renames the caller session,
  and moves its mapping out of `_unassigned` into the change. It preserves the
  underlying OpenCode session and history, but has no dedicated promotion UI.
- The browser follows a scaffolded session from the Changes index through
  session-to-change lookup; this handling is currently index-page-specific.
- Manual renaming and compatible OpenCode rename adapters already exist in
  `internal/server/sessionlifecycle.go` and `internal/opencode/lifecycle.go`.
  Initial primes are currently sent as prompts, so a transcript's first user
  message is not necessarily the first message actually authored by the user.

## Target behavior

- Changes and Chats are separate top-level navigation destinations. Chats has
  its own `/chats` page with familiar cards and a section-local New chat action,
  ordered by recent activity rather than creation date alone.
- Creation actions live only in their respective sections: New chat in Chats,
  New change session in Changes. Settings navigation uses an accessible gear icon.
- Opening a chat card resumes that exact conversation. A chat has a stable
  project-prefixed link and a breadcrumb using its current title; closing it
  returns to the Chats list. Browser reload/back and modified link clicks work.
- Remove the Discussions list from the Changes page. Keep New change session
  as an entry into planning; until scaffolded, that conversation belongs in Chats.
- New general-purpose chats and placeholder-titled planning discussions receive
  one short descriptive title after their first accepted, real user input.
  Setup prompts, synthetic instructions, and automatic helper activity do not
  trigger naming. Explicit/custom titles and manual renames are preserved.
- Promotion is available inside an eligible standalone chat. A confirmation
  proposes an editable change title and valid 2–4-character uppercase prefix.
  Confirmation explicitly authorizes scaffolding and planning only.
- The promotion action invokes the existing `lessmess-scaffold` skill in the
  same session. After the scaffold succeeds, the conversation leaves Chats and
  the change appears in Changes. Its session and complete history are retained.
- The promoted session receives its new binding, authoritative state, workflow
  rules, and any worktree guidance immediately; it no longer acts as an unbound
  free agent. The UI exposes the change's existing Work and Sessions surfaces.
- A failed or cancelled promotion does not hide the chat. Repeated confirmation
  does not create multiple changes, and late automatic naming cannot overwrite
  a manual title or the scaffolded change title.

## Scope

- Standalone-session metadata and listing, backward-compatible with existing
  mappings, including title ownership/naming state and activity timestamps.
- Dedicated Chats renderer, routes, card list, navigation, links, and responsive
  interaction using the existing Go templates and vanilla JS/CSS.
- Server-side automatic naming integrated with real user input admission through
  both ordinary prompt and queue/steer delivery routes.
- Same-session promotion confirmation, scaffold invocation, binding/instruction
  transition, and cross-page UI follow behavior.
- Focused Go and browser-glue tests, regression coverage for prefixed hub mode,
  and README documentation of the new user-visible workflow.

## Non-goals

- A separate chat transcript store, committed chat files, or chat entries in
  `.lessmess/workflow/`. OpenCode owns conversations; personal lessmess mapping
  metadata remains gitignored and tool-managed.
- A new client framework, Node build pipeline, or chat event-stream subsystem.
- Cross-project/global chat aggregation, automatic promotion, or implementation
  authorized implicitly by the promotion button.
- Demoting a change back to chat, merging multiple chats, or redesigning chat
  deletion, archives, task management, and the existing handoff workflow.
- Changing worktree policy or copying pre-promotion main-tree edits into a new
  worktree. Existing scaffold response warnings and worktree handling remain.

## Design decisions

### Identity and compatibility

Reuse the existing session ID and mapping rather than introduce a second
conversation identity or copy history to a fresh session. Add optional metadata
to session entries where necessary; do not require a destructive migration.
Keep existing discussion endpoints compatible while introducing a dedicated
Chats feed if its shape needs to differ.

The Chats list includes user-facing standalone general chats, pre-scaffold
planning discussions, settings discussions, and explorer chats. Bound change/task
sessions stay with their change. Known automated commit/gardener helpers and
transient subagents are not promoted to top-level chat cards. Legacy entries
with insufficient kind metadata remain discoverable rather than being silently
dropped; classify conservatively using recorded modules and creation context.
Existing meaningful titles are retained; no wholesale historical auto-renaming.

### Naming

Trigger naming at the lessmess server's successful admission of the first real
user input, not by counting messages in a primed transcript or depending on a
browser-only flag. Persist eligibility/ownership so reloads and repeated sends
cannot repeatedly rename a conversation.

Use a bounded asynchronous, tool-free title-generation request that does not add
messages to the conversation. OpenCode V2's published API offers stateless
`POST /api/experimental/generate` and transient session-context generation;
prefer stateless generation from the first input with the session's model when
available, and verify installed capability through OpenAPI before use. A short,
sanitized first-input title is the fallback when generation is unavailable,
times out, or produces unusable output. Naming never blocks or fails a normal
chat send. File-only input receives a safe attachment-based fallback without
reading attachment contents just to create a title.

Bound title size and strip formatting/control characters. Update OpenCode and
the persisted fallback consistently, and serialize/recheck late writes against
manual renaming, unlink/delete, and change binding. Report/degrade appropriately
when service-side renaming is unavailable; do not repeatedly retry on each poll.

### Promotion and authorization

Use a small confirmation surface in the chat's existing action menu, with title
and prefix suggestions that the user can edit. Only standalone, eligible,
project-owned sessions may promote; reject already-bound sessions and avoid
promotion while the session is busy or its confirmation is stale.

Confirmation sends an explicit scaffold-and-plan instruction with the confirmed
title/prefix and `lessmess-scaffold` skill to the same session through an
authenticated lessmess endpoint. The existing scaffold endpoint remains the
authoritative creator and binding transition; no alternative change-scaffolding
implementation is introduced. User-directed scaffold requests typed in chat
continue to work without requiring the button.

Make the mapping transition durable and safe against repeated/late operations.
Refresh binding facts and change instructions after success rather than waiting
for a later compaction. Honor worktree information from the existing scaffold
response; do not incorrectly describe the retained session's main-tree directory
as the worktree. Subsequent work follows the existing worktree behavior.

### Navigation and freshness

Extend the current chat shell and location navigation to Chats. All URLs must
respect `Server.Base`/`BASE` so single-project mode and `/p/<slug>` hub mounts
behave identically. List title/activity refresh and promotion follow must work
from Chats as well as Changes without resetting an unsent draft or the visible
transcript. Reuse existing visibility-aware polling/board refresh conventions;
do not add a chat SSE subsystem.

## Acceptance criteria

1. Chats is reachable independently of Changes, with a New chat action and
   recently active, resumable chat cards; Changes contains no Discussions list.
2. Existing user-facing standalone conversations survive the upgrade without
   history loss, duplicated mapping, forced renaming, or a manual migration.
3. The first real input names an eligible new chat once; primes do not. Naming
   failure leaves a usable fallback, and manual titles are never overwritten.
4. Names are visible consistently in cards, open chat titles, and breadcrumbs,
   including after reload and when the service is temporarily unavailable.
5. Confirmed promotion invokes the existing scaffold flow in the same session,
   preserving history and producing a change under Changes while removing its
   standalone chat card only after the binding succeeds.
6. Promotion updates workflow instructions, current state, and change controls.
   It does not start implementing tasks or authorize unrelated code edits.
7. Busy/stale/already-bound requests, double clicks, failures, and late naming
   results cannot produce duplicate changes or corrupt session ownership.
8. Navigation, links, first-send naming, and promotion work on desktop/mobile
   and with both empty and non-empty project URL prefixes.
9. Focused tests, `go vet ./...`, `go test ./...`, and workflow validation pass.
   User-visible behavior is documented in README.md. UI checks use rebuilt
   embedded assets rather than a stale running binary.

## Tasks

The task breakdown, dependencies, order, and statuses are maintained through
the workflow API and visible on the board. All tasks begin at Not started.
