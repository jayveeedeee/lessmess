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
- All file writes are atomic (temp file + rename) and constrained to the
  `changes/` tree.
- The server only **creates and updates** files — it never deletes anything.
- Writes to a file that fails validation are refused, so the tool cannot
  corrupt canonical data. External edits are never clobbered: every write is
  applied to the latest on-disk content.
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


## Development

```sh
go vet ./...
go test ./...
```

Layout:

```text
cmd/tasktracker/   CLI entry (serve, validate)
internal/model/    parsers + serializers for the AGENTS.md file formats
internal/store/    scan, cache, fsnotify watch, validation, safe writes
internal/server/   HTTP handlers, SSE, template rendering
web/               embedded templates and static assets (see web/static/VENDOR.md)
```

Vendored frontend assets (htmx, SortableJS) are pinned with checksums in
[`web/static/VENDOR.md`](web/static/VENDOR.md).
