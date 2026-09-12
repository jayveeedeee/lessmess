# tasktracker

A single-binary kanban server for the `changes/` workflow defined in
[`AGENTS.md`](AGENTS.md). It visualizes a repository's `changes/` tree as a
kanban board and writes operations back to the markdown files — which remain
the canonical, agent-readable database. The server is a view and editor over
the files, never the owner of the data.

## Build

```sh
CGO_ENABLED=0 go build -o tasktracker ./cmd/tasktracker
```

Produces one static binary with all web assets embedded; no runtime files or
Node toolchain required.

## Usage

```sh
# Start the board (http://127.0.0.1:8080)
tasktracker serve [--host 127.0.0.1] [--port 8080] [--dir .]

# Check the changes/ tree against the AGENTS.md validation contract
tasktracker validate [--dir .]

# Bootstrap an uninitialized directory as a workflow repository
tasktracker init [--dir .]

# Initial run-through that seeds the repo docs (see below)
tasktracker docs seed [--dry-run] [--budget N] [--dir .]
```

`--dir` points at a repository root containing `changes/` (default: current
directory). One process serves one repository.

### The board

- **`/`** — change list, built from the root ledger (`changes/ledger.md`).
- **`/changes/<id>`** — kanban board with five columns (`Not started`,
  `In progress`, `Blocked`, `Done`, `Cancelled`); cards are the change
  ledger's task rows in row order (= priority, per `AGENTS.md`).
- **Drag a card** between columns or reorder within one: rewrites the task
  table in the change's `ledger.md` (status cell + row order), preserving all
  other file content byte-for-byte.
- **Add task / New change**: creates spec-compliant task files, change
  directories, and ledger rows.
- **Live updates**: the server watches `changes/` with fsnotify; edits made
  by other tools (e.g. an agent updating a ledger) appear on the board via
  SSE without a restart or reload.
- **Validation banner**: any breach of the `AGENTS.md` validation rules is
  shown in a banner and refuses writes to the affected file.

## Safety

- Binds `127.0.0.1` by default; **no authentication** — it is a local
  single-user tool. Do not expose it on a network interface.
- All file writes are atomic (temp file + rename). Change-workflow writes are
  constrained to the `changes/` tree; docs-system writes are additionally
  confined to the marker sections of `STRUCTURE.md`/`AGENTS.md` in covered
  directories, verified after every LLM pass with rollback on violation.
- The server only **creates and updates** files. The single exception: rolling
  back a docs LLM pass removes a file that pass created (restoring the
  pre-pass state).
- Writes to a file that fails validation are refused, so the tool cannot
  corrupt canonical data. External edits are never clobbered: every write is
  applied to the latest on-disk content, and doc writes preserve all bytes
  outside the markers.
- Everything is plain markdown; inspect or undo any change with your editor
  (or git, once the repository is under version control).

## opencode integration

If an [opencode](https://opencode.ai) V2 background service is running,
tasktracker connects to it automatically (discovery via
`opencode2 service status`, credentials from
`~/.config/opencode/service.json` — never sent to the browser; all service
calls are made server-side).

- **Sessions panel** on each board: create, list, open, and unlink multiple
  opencode sessions per change. Mappings persist in
  `.tasktracker/sessions.json` (gitignored tooling state).
- **Embedded terminal**: opening a session renders the live opencode TUI in
  the browser (xterm.js). tasktracker spawns `opencode2 --session <id>` in
  its own PTY and bridges it over a WebSocket; the session persists in the
  opencode service, so reconnecting resumes it.
- **New change session** (index page): scaffolds a change, creates and
  primes an opencode session, and opens the board with the terminal
  attached. The agent works the `changes/` workflow; the board updates live.

### Security posture of agent sessions

`opencode.json` in this repository pre-approves agent permissions so change
sessions run unattended: broad `allow` **inside this project**, with denies
for external directories, `.env` files, and `git push`. This means an agent
can edit files and run shell commands in this repo without per-action
prompts — only run this on a repository you are comfortable letting an agent
work in autonomously. The embedded terminal and the service API are
localhost-only, and the opencode service password is never exposed to the
browser (injected server-side).

If the service is unreachable, tasktracker starts normally without the
integration (a warning is logged).

## Repo docs management

Beyond the change workflow, tasktracker bootstraps and maintains agent-facing
docs across a repository — so an agent entering any folder cold gets a map and
the local learnings. Two files per covered folder:

- **`STRUCTURE.md`** — a machine-owned navigation map (entries, purposes,
  child rollups, freshness metadata). Regenerated wholesale, deterministically;
  never hand-edit inside its `<!-- tasktracker:begin/end -->` markers.
- **`AGENTS.md`** — curated learnings and instructions for that area. Refined
  and appended, never regenerated; everything outside the markers is
  human/agent-authored and preserved byte-for-byte.

Coverage is configured by a committed [`agentsdocs.json`](agentsdocs.json)
(include/exclude globs; hidden dirs and `changes/` are never covered). Without
it, the whole subsystem is inert. `tasktracker init` writes it along with a
root `AGENTS.md` carrying the canonical workflow instructions, the `changes/`
skeleton, `.gitignore` handling, and a starter `opencode.json`.

- **Seed**: `tasktracker docs seed` walks the tree bottom-up, writes
  `STRUCTURE.md` skeletons, then runs one unattended opencode session per
  directory to fill purposes and write first-pass `AGENTS.md` learnings.
  Resumable (`.tasktracker/docs-seed.json`), budget-capped (`--budget`),
  dry-runnable; offline it writes skeletons only.
- **Refresh**: closing a change computes the touched folders from its tasks'
  "Files affected" and enqueues a serialized doc-gardener job — one unattended
  session updates those folders' docs, with each new learning citing the
  change ID. If the service is down, folders are flagged stale and reconciled
  by the next run or `POST /docs/refresh`.
- **Confinement**: after every LLM pass the server verifies that only the
  marker sections changed (bytes outside are compared; freshness hashes must
  match) and rolls back violations.
- **Visibility**: docs findings surface in a header **notification bell** (badge
  counts findings; red if any are errors). The bell opens a modal listing them
  grouped by severity, with a **Refresh stale docs** button that reconciles
  every stale directory (queue-stale and hash-stale alike) as one manual
  gardener job — findings and the explorer tree update live as it finishes.
  `tasktracker validate` reports the same findings (warnings; structural
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
cmd/tasktracker/   CLI entry (serve, validate, init, docs seed)
internal/model/    parsers + serializers for the AGENTS.md file formats
internal/store/    scan, cache, fsnotify watch, validation, safe writes
internal/server/   HTTP handlers, SSE, template rendering, docs queue + gardener
internal/docs/     repo docs: coverage config, tree walk, STRUCTURE.md generation, seed
web/               embedded templates and static assets (see web/static/VENDOR.md)
```

Vendored frontend assets (htmx, SortableJS) are pinned with checksums in
[`web/static/VENDOR.md`](web/static/VENDOR.md).
