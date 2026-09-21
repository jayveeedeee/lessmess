# lessmess

A single-binary kanban server for the change-management workflow. Workflow
state — changes, tasks, statuses, dependencies, decision logs — lives in a
tool-owned JSON store under `.lessmess/workflow/` (committed alongside the
code); narrative documents (plans, task prose) stay markdown under
`changes/`. The board visualizes the state, every mutation goes through
deterministic, rule-enforcing endpoints, and agent sessions receive their
instructions and the current state injected at spawn — nothing hand-edits
a table, and no stale instruction copies live in repositories.

## Build

```sh
CGO_ENABLED=0 go build -o lessmess ./cmd/lessmess
```

Produces one static binary with all web assets embedded; no runtime files or
Node toolchain required.

## Usage

```sh
# Start the board (http://127.0.0.1:8080)
lessmess serve [--host 127.0.0.1] [--port 8080] [--dir .]

# Check the workflow state against the validation contract
lessmess validate [--dir .]

# One-time conversion of a legacy markdown-ledger repo to the JSON store
# (serve and validate also auto-migrate on start)
lessmess migrate [--dry-run] [--dir .]

# Bootstrap an uninitialized directory as a workflow repository
lessmess init [--dir .]

# Initial run-through that seeds the repo docs (see below)
lessmess docs seed [--dry-run] [--budget N] [--dir .]
```

`--dir` points at a repository root (default: current directory). One
process serves one repository.

## The JSON workflow state

```
.lessmess/workflow/            # tool-owned state, committed
├── index.json                 # the change registry (id, title, prefix, branch, created)
└── changes/<id>.json          # one change: overall status, decision log,
                               #   task tree (id, title, status, deps, notes, order)

changes/<id>/                  # narrative markdown, committed
├── plan.md
└── tasks/<nn>-<slug>.md       # task prose (no tables, no frontmatter)
```

- The rest of `.lessmess/` (sessions, settings, queues) stays personal and
  gitignored; only `workflow/` is committed, via a `.gitignore` negation.
- **Never edit the JSON by hand.** All mutations go through the store's
  atomic writes, exposed as HTTP endpoints (used by the board and by
  agents): task create/status/update/reorder, decomposition, decision log.
  The workflow rules are enforced there — `Test` transitions require
  recorded verification evidence, and `Done` is user-gated (agent calls
  are refused; the board is the user's hands).
- Derived change statuses (`Planned` / `In progress` / `Blocked`) are
  recomputed from the task tree on every change and converge automatically;
  `Done` and `Cancelled` are user-set.
- `lessmess validate` checks the whole contract: state schema, prose/state
  agreement, index/tree consistency.

### First run: onboarding wizard

Point `lessmess serve` at a directory that has no `changes/` tree and the
server starts in **setup mode**: instead of exiting, it serves a first-run
wizard at `http://127.0.0.1:8080/` (all normal routes refuse with 503 or
redirect there until setup completes). The wizard walks through:

1. **Prerequisites** — per-check status with fix hints and a Re-check
   button: the `opencode2` binary, the opencode background service and its
   credentials, `git`, and that the directory is writable. The wizard
   detects and instructs; it never tries to start anything itself.
2. **Bootstrap** — creates the workflow files merge-safely (same artifacts
   as `lessmess init`: `AGENTS.md`, `.lessmess/workflow/index.json`,
   `.gitignore`, `opencode.json`), with a separate choice of whether to
   enable docs coverage (`agentsdocs.json`) and an **exclusion picker** for
   it: a lazy directory tree (expand ▸ for nested folders) where checked
   directories and their subtrees get no doc pairs — top-level picks also
   exclude same-named directories elsewhere, and built-in exclusions like
   `node_modules` are pre-checked and disabled.
   The full UI **hot-opens in place** — no restart.
3. **Default agent and model** — picked from live lists served by the
   opencode service, saved to the personal layer (`.lessmess/settings.json`)
   or the project layer (`lessmess.json`), or skipped to use the service
   defaults.
4. **Docs seeding — explicit opt-in** — "Generate docs now" runs a budgeted
   seed (one opencode session per covered directory, honoring the chosen
   agent/model) with live progress; skipping means nothing runs. Already-
   summarized directories are skipped on re-runs.
5. **Finish** — completion is recorded in `.lessmess/onboarding.json`
   (gitignored) and the wizard never nags again.

On an already-initialized repository whose onboarding is incomplete, the
index shows a dismissible banner linking to `/setup`; the Settings page has
a permanent "Re-run the onboarding wizard" link. The CLI (`lessmess init`,
`lessmess docs seed`) stays available for scripted setups, and `docs seed`
honors the configured `session.agent`/`session.model` like every other
session lessmess spawns.

Setup API (for the wizard and other clients): `GET /setup`,
`GET /api/setup/prereqs`, `GET /api/setup/dirs`, `POST /api/setup/bootstrap`,
`POST /api/setup/docs-seed`, `GET /api/setup/docs-seed-status`,
`POST /api/setup/complete`, `POST /api/setup/dismiss`.

### The board

- **Top menu** — Changes and Explorer are always visible in the header; the
  active route is highlighted. On the right, **Chat** spawns a general
  codebase chat (`POST /chat/session`): a fresh opencode session in the
  repository root, primed as a free agent — it answers questions, explains
  code, and edits files when you explicitly ask, with no change binding or
  bookkeeping (edits land in the main tree as ordinary working-tree
  changes). Every click opens a new session; chats are unassigned — they
  appear in the index Discussions list — and open in the terminal overlay.
  For directory-scoped, docs-grounded Q&A there is the explorer's per-
  directory chat (see below).
- **`/`** — change list, built from the workflow index plus each change's
  state, newest change first.
- **`/changes/<id>`** — kanban board with six columns (`Not started`,
  `In progress`, `Blocked`, `Test`, `Done`, `Cancelled`); cards are the
  change's tasks in priority order. Agents stop at `Test` once verification
  passes; `Done` is user-gated — the user drags the card there or explicitly
  accepts, which is enforced server-side (agent-side `Done` calls are
  refused with guidance).
- **Drag a card** between columns or reorder within one: writes the task's
  status and position to the change's JSON state atomically.
- **New change session**: creates the change (state + prose skeleton) and a
  primed planning session; tasks are created through the API by the session's
  agent.
- **Derived change status**: the status pill always mirrors the state —
  `Planned` while no task has started, `Blocked` when every open task is
  blocked, `In progress` otherwise. The server derives it from the task
  tree on every task change and repository rescan. `Done` is user-gated
  (**Close change**), and new open work on a closed change flips it back
  to `In progress`.
- **Live updates**: the server watches the workflow store and the prose
  trees with fsnotify; external edits appear on the board via SSE without
  a restart or reload.
- **Validation banner**: any breach of the workflow validation rules is
  shown in a banner and refuses writes to the affected state.

### Nested tasks (sub plans)

Any task can be expanded into a sub plan when it needs detailed work: the
task keeps its prose file and gains a container directory
(`tasks/<nn>-<slug>/tasks/`) holding its subtasks with dotted IDs
(`EXC-00` → `EXC-00.00` → `EXC-00.00.01`), recursively. Nesting lives in
the JSON state (`parent` + dotted IDs); there are no per-container ledger
files.

- **⤢ Expand** on a card creates the container (the user-instructed
  decomposition action). Agents propose decompositions when work reveals
  complexity but never expand unprompted.
- **Drill down**: the `x/y ✓` badge on a decomposed card opens that task's
  sub-board (`/changes/<id>?task=<id>`) — the same kanban scoped to its
  children, with a breadcrumb back up.
- **Progress is display-only**: badges on decomposed cards are computed from
  that task's descendants (`Test` + `Done` count as complete, `Cancelled`
  leaves the denominator). Nothing is ever written by rollup — each status
  lives in the state, and `Done` stays user-gated.
- **Close-out is recursive**: closing a change requires every non-cancelled
  task in the whole tree to be `Test` or `Done`.
- **Sessions**: every decomposed task gets one auto-spawned, task-scoped
  opencode session (best-effort, exactly once — unlinking never respawns;
  the sub-board's Start/Continue button is the manual retry). Sub-boards
  list the sessions bound to that task; delegation with dotted title
  prefixes (`EXC-00.01: …`) attaches subagent sessions at any depth.

## Instruction injection

Agent sessions are primed at spawn with exactly the instructions that
concern them, selected deterministically from versioned modules embedded in
the binary (`internal/server/instructions.json`) — repositories carry only a
short pointer in `AGENTS.md`, so refining an instruction needs no
repository-file updates and can never go stale in downstream repos.

- **Modules** (`discussion`, `change.session`, `change.handoff`, `worktree`,
  `closeout`, `task.session`) are selected by session kind and change/task
  state — e.g. the `worktree` module is injected only for worktree-backed
  changes; a discussion session never sees task rules.
- **The current state is injected too**: change and task primes carry a
  deterministic view of the change's state (the same generated markdown the
  board's Ledger modal shows), so agents never parse state files.
- **Auditability**: every spawn logs and records the injected module IDs
  (`.lessmess/sessions.json`), and `GET /workflow/instructions` serves the
  module manifest.

## Settings

The **Settings** page (top menu, right) edits defaults that used to be
hard-coded. Settings are layered:

- **Project** — `lessmess.json` at the repo root, committed and shared with
  everyone using the repository. Edits via the page land in git status and
  flow through the normal commit path like any other project file.
- **Personal** — `.lessmess/settings.json`, gitignored tooling state on
  this machine. Per field, personal wins over project, which wins over the
  built-in default. Each field's badge shows which layer supplies its
  current value; saving always writes the selected scope only.

Everything is optional: a missing file means built-in defaults, and a
malformed file falls back to defaults with a warning on the page. Saved
values apply to new activity immediately — no restart.

| Setting | Effect |
| --- | --- |
| `general.projectName` | Project display name shown next to the logo (including the terminal overlay) and used as the browser tab title on every page. Empty uses the repository folder's basename, so renaming the folder updates the default until you set an explicit name. Also collected by the onboarding wizard. |
| `session.agent` | opencode agent for newly spawned sessions (change sessions, discussions, explorer chats, commits, doc gardener). Unknown values are rejected at save time when the service is reachable. |
| `session.model` | Model for new sessions as `provider/model` (e.g. `anthropic/claude-sonnet-4-5`). Same validation. |
| `session.autoOpenTerminal` | Open the embedded terminal automatically after a session is created (default on). |
| `prompts.discussion` / `change` / `commit` / `repoCommit` / `gardener` / `explorer` | Free text **appended** to the corresponding built-in prompt. Base prompts are never modified, so workflow safeguards stay intact. |
| `git.defaultBranch` | Base branch for new change worktree branches (`change/<id>` is cut from it; empty uses the current branch at scaffold time). Always recorded on the change's index entry. |
| `git.worktrees` | **Worktree per change** (default off — see the Worktree pipeline section below). When on, scaffolding creates a git branch and worktree per change, change sessions work there, and closing pushes the branch, opens a PR, and runs an agent review. |
| `git.reviewModel` | Model for PR review sessions as `provider/model`. Empty inherits `session.model`. |
| `ui.showArchived` | List archived changes on the Changes page (default on). |
| `ui.accent` | Accent color: one of a fixed palette (orange, teal, green, blue, violet, pink, fuchsia, red, amber, cyan). It tints the whole UI and the favicon/brand icon. With no value in either layer, the first run rolls a random color and saves it to the personal layer; setting **Auto** in both layers rolls a fresh random color on the next page load. Unknown values are rejected at save time. |
| `docs.autoGardenerOnClose` | Run the doc gardener automatically when a change closes (default on). |
| `docs.gardenerModel` | Model for doc-gardener sessions, as `provider/model`. Empty inherits `session.model`; save-time validation applies when the service is reachable. |

Agent and model fields suggest live values from the opencode service
(primary agents, available models, service default shown as placeholder);
with the service down the fields stay editable as free text. Agent/model
apply only to sessions created after saving — never retroactively.

API: `GET /api/settings` (effective + layers + sources),
`PUT /api/settings?scope=project|personal` (section-scoped writes; empty
or null clears a field from that layer; submitted agent/model values are
validated against the live service when reachable — the service accepts
unknown names at creation but then never runs the session),
`GET /api/settings/options` (agent/model lists scoped to this repository,
plus the static accent palette; `available:false` when the service is
down — the palette is still served).

## Worktree pipeline (optional)

Off by default. With **Settings → Git → Worktree per change** enabled, each
change is developed in its own git worktree on its own branch, isolated from
the main tree and from other changes:

- **Scaffold** cuts branch `change/<id>` from the configured base branch
  (or the current branch), registers a worktree at
  `<parent>/<repo>-worktrees/<id>`, and creates `changes/<id>/` *inside the
  worktree*. The workflow state stays in the main tree — the server
  maintains it there centrally, so parallel branches never conflict. If the
  main tree has uncommitted files, the scaffold still succeeds but the
  agent is warned which files the new worktree will not contain.
- **During the change**, every session bound to the change (change session,
  task subagents, commit sessions, the per-change terminal) works inside the
  worktree. The board shows the branch, worktree health (active / uncommitted
  / missing), PR link, and review state on the change page.
- **Close is gated**: it refuses while the worktree has uncommitted *code*
  (the change's own `changes/<id>/` metadata — status flips, the review file
  — is exempt), then pushes the branch, opens a PR with the plan as its body,
  runs an unattended reviewer session that writes `changes/<id>/review.md`
  and posts it as a PR comment, and only then marks the change Done. The
  review verdict is informational — you decide what to do with it. Closing
  requires a configured `origin` remote and the
  [gh CLI](https://cli.github.com/) to be authenticated. A close attempt that
  fails after the PR was created reuses that PR on the next attempt.
- **The reviewer session stays** after finishing: it is bound to the change,
  shows up in the Sessions list, and you can open it (Talk/terminal) to ask
  follow-up questions — it remembers its own review. The review text is also
  readable on the board via the **Review** button next to the PR link.
- **Reopen** reattaches the existing worktree. **Remove worktree** is a
  manual board action (refused while the worktree is dirty); branches and
  commits are never deleted by lessmess, and nothing is removed
  automatically.

Repos without git, without a remote, or with the setting off behave exactly
as before: no git operations, no gating, no worktrees.

## Safety

- Binds `127.0.0.1` by default and has **no authentication**. A broader
  `--host` bind is suitable only on a trusted private network or behind a VPN
  such as Tailscale; never expose lessmess directly to the public internet.
- All file writes are atomic (temp file + rename). Workflow writes touch
  only the JSON store under `.lessmess/workflow/` and the prose files under
  `changes/`; docs-system writes are additionally confined to the marker
  sections of `STRUCTURE.md`/`AGENTS.md` in covered directories, verified
  after every LLM pass with rollback on violation.
- The server only **creates and updates** files, with two deliberate
  exceptions: the one-time markdown→JSON migration deletes the legacy
  ledger files (round-trip-verified first, git-recoverable), and rolling
  back a docs LLM pass removes a file that pass created.
- Writes that fail validation are refused, so the tool cannot corrupt
  canonical data. External edits are never clobbered: every write applies
  to the latest on-disk content, and doc writes preserve all bytes outside
  the markers.
- Workflow state is committed JSON and prose is plain markdown — inspect or
  undo anything with git (or your editor) as with any other file.

## opencode integration

If an [opencode](https://opencode.ai) V2 background service is running,
lessmess connects to it automatically (discovery via
`opencode2 service status`, credentials from
`~/.config/opencode/service.json` — never sent to the browser; all service
calls are made server-side).

- **Sessions panel** on each board: create, list, open, and unlink multiple
  opencode sessions per change. Mappings persist in
  `.lessmess/sessions.json` (gitignored tooling state).
- **Subagent sessions per task**: a change session may delegate a task to an
  opencode subagent. When it titles the subagent's description `TSK-NN: …`
  (the change prompt teaches this), the board attaches the subagent session
  to that task card with a **Talk** button — opening a terminal chat on the
  subagent directly, including after it has finished. Unbound subagent
  sessions surface on the board header; bindings live in
  `.lessmess/sessions.json` (`task`/`parent` fields), never in `changes/`.
  `POST /changes/{id}/task-sessions` binds a subagent session explicitly.
- **Continue session** button on each board: one click resumes the session
  you last opened for that change — or starts a new one when the change has
  none. Narrow screens open Chat; desktop screens retain the Terminal default.
- **Mobile Chat**: every mapped session offers a structured Chat view alongside
  Terminal. It renders the authoritative OpenCode transcript, reasoning, tool
  progress/results, errors, permission requests, and structured forms; users
  can send prompts or interrupt work without a PTY. The view polls only while
  visible, restores drafts, preserves a reader's scroll position, and exposes
  change tasks as a mobile drawer. Chat and Terminal attach to the same session,
  so switching modes does not split context.
- **Embedded terminal**: opening a session renders the live opencode TUI in
  the browser (xterm.js). lessmess spawns `opencode2 --session <id>` in
  its own PTY and bridges it over a WebSocket; the session persists in the
  opencode service, so reconnecting resumes it.
- **Chrome-free embedded TUI**: embedded terminals run with a lessmess-managed
  opencode CLI config — generated per spawn at `.lessmess/xdg/opencode/cli.json`
  by forcing `tabs.enabled: false` and `session.sidebar: "hide"` on top of your
  own `~/.config/opencode/cli.json` (theme, keybinds, and plugins carry over;
  the file itself is only ever read). If generation fails, the terminal falls
  back to your normal setup with a logged warning. Your standalone `opencode2`
  is unaffected.
- **Task panel**: terminals and desktop Chat views opened on a change board
  show a lessmess-native panel on the right mirroring the board's tasks grouped by
  status. It updates live as tasks change (no page reload), clicking a row
  opens the task detail above the terminal, and a fixed Plan button at the
  bottom opens the change plan. Chat turns the panel into an on-demand drawer
  at phone widths. Unassigned sessions keep the full-width conversation view.
- **New change session** (index page): scaffolds a change, creates and
  primes an opencode session, and opens the board with the terminal
  attached. The agent works the `changes/` workflow; the board updates live.
- **Sortable change list** (index page): click a column header (Change,
  Title, Prefix, Status, Tasks, Updated) to sort the table; click again to
  flip direction. Status sorts in board workflow order, and your chosen
  sort is remembered across reloads. Without a selection the list stays
  newest-first.
- **Commit all** (index page, next to New change session): one click commits
  every uncommitted change in the repository. The button is disabled when
  the working tree is clean and hidden outside git repositories. It opens a
  confirmation modal listing the uncommitted files plus a diffstat (fetched
  live from `GET /api/git/status`); confirming spawns an opencode session
  (`POST /api/git/commit`) that writes the commit message and commits — the
  same rails as the board's per-change Commit: commit only, never push. The
  session appears under Discussions as "repo — git commit".

### Mobile Chat capability matrix

OpenCode's V2 HTTP API is experimental and differs between releases. lessmess
uses explicit adapters and shows unavailable controls when the connected
service does not advertise the required operation; it never forwards arbitrary
OpenCode paths. This matrix tracks the supported mobile surface:

| Capability | lessmess support | OpenCode qualification |
| --- | --- | --- |
| Transcript, prompt, interrupt | Supported | Authoritative polling; no lessmess transcript storage |
| Permissions and forms | Supported | Pending interactions are reloaded from OpenCode |
| Long history | Supported | Bounded 50-message pages with opaque cursors |
| File/image attachments | Supported | In-memory only; 10 files, 20 MiB each and total |
| Project references | Supported | Aliases are resolved server-side; browser paths are rejected |
| Historic attachment download | Supported | Projected message bytes only; arbitrary file URIs are never dereferenced |
| Tool details | Supported | Loaded on demand in bounded chunks; not upstream-paginated |
| Turn diffs | Capability-gated | Current V2 session-diff API; older services may not provide it |
| Agent/model switch | Supported | Project-scoped options; failed changes retain the prior selection |
| Commands and prompt skills | Supported | Project-scoped discovery and validation |
| Standalone skill activation | Capability-gated | Endpoint path differs across V2 releases |
| Exact context pressure | Unavailable | UI estimates from latest usage and model context limit |
| Fork/revert/compact/inbox | Supported | Version-aware; revert and cancellation limitations are shown in the UI |
| Parent/child navigation | Supported | Authoritative `parentID` traversal with deep pagination and cycle guards |
| Rename/unlink/delete | Supported | Unlink preserves OpenCode data; delete confirms and removes descendants |
| Session export | Capability-gated | Experimental in current V2 and absent from some beta services |
| Service status | Supported | Read-only identity, project scope, provider/model/plugin metadata, and exact operation capabilities at Settings → OpenCode service |
| Integration connections | Supported | Key, OAuth, command, and service-environment methods use OpenCode's credential store; transient attempts are never persisted by lessmess |
| MCP and saved permissions | Supported | Project-scoped status/resources, runtime connect/disconnect, active requests, and confirmed saved allow-rule removal |
| Interactive shell | Terminal only | Chat may show bounded session shell output but never provides a PTY |
| Public hosting | Unsupported | Use localhost or a trusted LAN/VPN; no built-in authentication or TLS |

In particular, lessmess does not claim atomic revert previews, editable queued
prompt text, authoritative inbox-cancellation outcomes, provider health pings,
service restart, plugin updates, generic OpenCode configuration editing, or MCP
configuration/authentication. MCP authentication links to a tested integration
when OpenCode supplies one; otherwise the UI gives the OpenCode CLI/TUI remedy.

`/settings/opencode` provides the mobile service-diagnostics view. Its explicit
`GET /api/opencode/status` payload reports healthy, partial, degraded, and
unsupported states without forwarding OpenCode errors, credentials, headers,
settings, process details, connection URLs, temporary paths, or local plugin
paths. Provider/model/plugin entries describe catalog availability only; they
are not network health pings. “Rediscover service” re-reads and validates the
registered endpoint and credentials without starting, stopping, or restarting
OpenCode.

The same page lists integrations and their supported connection methods. Key
and provider-form values are submitted directly from a password-sensitive form
to a narrow lessmess adapter, forwarded to OpenCode, and immediately discarded.
OAuth and command attempt details are transient, non-cacheable, and polled only
while their dialog is visible. Saved credential labels can be changed, activated,
or deleted after a fresh-target check; connections supplied by the OpenCode
service environment must instead be changed in that environment and the service
restarted. Automatic loopback OAuth remains provider- and device-dependent.

The MCP section exposes configured server status and redacted resource
identities. Connect and disconnect use only the exact runtime operation
advertised by the attached OpenCode service; the state may not survive an
OpenCode restart. lessmess does not add, remove, or edit MCP configuration and
does not assume an HTTP OAuth/logout endpoint exists. The permissions section
shows repository-scoped active requests, links known mapped sessions back to
Chat, and lists only this repository project's saved allow rules. Saved rules
can be removed after a fresh-target confirmation; there is no create or edit
API on this page.

### Security posture of agent sessions

`opencode.json` in this repository pre-approves agent permissions so change
sessions run unattended: broad `allow` **inside this project**, with denies
for external directories, `.env` files, and `git push`. This means an agent
can edit files and run shell commands in this repo without per-action
prompts — only run this on a repository you are comfortable letting an agent
work in autonomously. lessmess binds localhost by default; even when deliberately
bound to a trusted LAN/VPN interface, the OpenCode service password is never
exposed to the browser because all service calls are made server-side.

If the service is unreachable, lessmess starts normally without the
integration (a warning is logged).

## Repo docs management

Beyond the change workflow, lessmess bootstraps and maintains agent-facing
docs across a repository — so an agent entering any folder cold gets a map and
the local learnings. Two files per covered folder:

- **`STRUCTURE.md`** — a machine-owned navigation map (entries, purposes,
  child rollups, freshness metadata). Regenerated wholesale, deterministically;
  never hand-edit inside its `<!-- tasktracker:begin/end -->` markers.
- **`AGENTS.md`** — curated learnings and instructions for that area: a short,
  bounded set of current-state facts (at most 15 per file) that the doc
  gardener consolidates in place rather than appends to. Everything outside the
  markers is human/agent-authored and preserved byte-for-byte.

Coverage is configured by a committed [`agentsdocs.json`](agentsdocs.json)
(include/exclude globs; hidden dirs and `changes/` are never covered). Without
it, the whole subsystem is inert. `lessmess init` writes it along with a
root `AGENTS.md` workflow pointer, the workflow state skeleton
(`changes/` + `.lessmess/workflow/index.json`), `.gitignore` handling, and
a starter `opencode.json`.

- **Seed**: `lessmess docs seed` walks the tree bottom-up, writes
  `STRUCTURE.md` skeletons, then runs one unattended opencode session per
  directory to fill purposes and write first-pass `AGENTS.md` learnings.
  Resumable (`.lessmess/docs-seed.json`), budget-capped (`--budget`),
  dry-runnable; offline it writes skeletons only. Sessions honor the
  configured `session.agent`/`session.model` defaults. The server exposes
  the same run: `POST /docs/seed` targets the covered directories that
  **do not yet have their doc files** (file existence decides — a stale
  cursor never hides a missing dir), and `{"force":true}` (or the bell's
  **Force — redo every directory** checkbox, or `--force` on the CLI)
  re-runs everything regardless. `GET /docs/seed-status` reports
  progress; `/api/validate` includes the missing-docs count, and the bell
  shows **Run missing docs (N)** while any directory is incomplete.
- **Exclusions editor**: the Settings page's Docs section has the same
  lazy expandable folder tree as the onboarding wizard — top-level and
  nested picks alike — and saving persists the selection to
  `agentsdocs.json` (`GET`/`POST /docs/exclusions`) with the same
  semantics: picker-representable patterns are replaced, hand-authored
  globs and stale names are preserved. The root directory is always
  covered and never listed.
- **Refresh**: closing a change computes the touched folders from its tasks'
  "Files affected" and enqueues a serialized doc-gardener job — one unattended
  session updates those folders' docs by **consolidating** the learnings
  section: adding only durable knowledge, rewording or deleting entries the
  change superseded, keeping every learning phrased as how the code works now
  (never change narration, no change-ID prefixes — attribution lives in git
  history), and holding each section to at most 15 entries. The job also names
  **ancestor directories** as review-and-fix targets: the gardener checks
  whether the change invalidated learnings in the parents (a removed feature, a
  moved file) and fixes or deletes those learnings, accounting for every
  removal in its reply. If the service is down, folders are flagged stale and
  reconciled by the next run or `POST /docs/refresh`.
- **Stale-reference lint**: a deterministic check scans every covered
  `AGENTS.md`'s learnings for backticked path-like references that no longer
  resolve anywhere in the repository and surfaces them as warnings. The
  refresh flow folds those directories into its reconciliation job, with the
  flagged references named in the gardener prompt so the fix is targeted.
  The lint is conservative: Go-style symbols (`server.New`), model IDs, and
  bare directory mentions never lint.
- **Confinement**: after every LLM pass the server verifies that only the
  marker sections changed (bytes outside are compared; freshness hashes must
  match) and rolls back violations.
- **Visibility**: docs findings surface in a header **notification bell** (badge
  counts findings; red if any are errors). The bell opens a modal listing them
  grouped by severity, with a **Refresh stale docs** button that reconciles
  every stale directory (queue-stale, hash-stale, and lint-flagged alike) as
  one manual gardener job — findings and the explorer tree update live as it
  finishes.
  `lessmess validate` reports the same findings (warnings; structural
  corruption is an error). The red banner remains for `changes/` violations.

### Project explorer

The **`/explorer`** page (linked in the header) is a master/detail browser
for those docs. The left pane is a compact directories-only tree with guide
lines; clicking a directory loads its purpose and its files with their
blurbs into the right-hand detail pane (served as an htmx fragment by
`GET /explorer/detail?dir=…`). It refreshes live — a docs fsnotify watch
emits SSE events as docs change (e.g. after a gardener run), and the tree
swaps in place without losing which nodes are expanded or selected, while
the visible detail refreshes along with it.

Every directory has a **chat button** (tree row or detail header): it
creates an opencode session scoped to the repository root, primed with that
directory's STRUCTURE.md and AGENTS.md and instructed to answer questions
about the directory. Explorer chats are unassigned sessions: they appear in
the index Discussions list and open in the terminal overlay.


## Development

```sh
go vet ./...
go test ./...
```

Layout:

```text
cmd/lessmess/       CLI entry (serve, validate, migrate, init, docs seed)
internal/model/     JSON workflow state types + legacy markdown readers (migration)
internal/store/     state store: scan, cache, fsnotify watch, validation, safe writes, migration
internal/server/    HTTP handlers, SSE, instruction injection, template rendering, docs queue + gardener
internal/docs/      repo docs: coverage config, tree walk, STRUCTURE.md generation, seed
web/                embedded templates and static assets (see web/static/VENDOR.md)
```

Vendored frontend assets (htmx, SortableJS) are pinned with checksums in
[`web/static/VENDOR.md`](web/static/VENDOR.md).
