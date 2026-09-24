# 2026-09-24-0zyrp: Multi-project serving with project switcher

- Change ID: 2026-09-24-0zyrp
- Created: 2026-09-24
- Branch: —
- Status: tracked in the tool-owned JSON state (.lessmess/workflow/)

## Objective and context

lessmess currently serves exactly one repository per process (`lessmess serve --dir <repo>`), so running it for several projects means several processes on several ports. The goal is a single long-running instance (e.g. on a server) that serves **all** of the user's projects at once, with a user-level global config persisting which projects are registered and a UI for switching between them. Nothing else about the product changes: same templates, same workflow, same per-project look and feel (each project keeps its own rolled accent color).

## Current behavior

- `cmd/lessmess` `serve` takes a single `--dir`, opens one `store.Store`, builds one `server.Server` (all routes at root), and serves it. A missing `changes/` tree starts the setup wizard, which hot-swaps the handler in-process.
- Every URL is root-absolute: server-side emissions (`HX-Redirect`, breadcrumbs, `BoardURL`, setup redirects, prime curl bases via `PublicBase`) and client-side (`fetch("/…")` in `app.js`, `EventSource("/events")`, template `href`/`hx-*` attributes).
- Accent color rolls once per repo and persists in that repo's `.lessmess/settings.json`; `general.projectName` (default: folder basename) labels each repo.
- No process-global state anywhere: store, settings, gitops, docs queue, and session mappings are all parameterized by the repo root; the opencode client is one shared service client that creates sessions with an explicit working directory.

## Target behavior

- `lessmess serve` **without** `--dir` reads the global registry (`~/.config/lessmess/config.json`, XDG-respecting) and mounts every registered project at `/p/<slug>/` via `StripPrefix`, each with its own store, watcher, SSE stream, docs queue, and setup wizard (unbootstrapped dirs mount the wizard under their prefix).
- `/` becomes a projects landing page: list registered projects (name, path, link) with Add (validated path input) and Remove (unmount + forget). Add hot-boots the project without a restart; remove closes its store/watcher cleanly and deletes the registry entry. The repo's own `changes/` and `.lessmess/` state is never touched by remove.
- A project switcher dropdown in the sticky header lists all projects (label = effective `general.projectName`) plus a link to the landing page; the current project is highlighted.
- `lessmess serve --dir X` keeps working exactly as today: single project at root with no prefix (backward-compatible base).
- Per-project `PublicBase` becomes `host:port/p/<slug>` so newly created agent sessions prime curl instructions that hit the right project; sessions created before the upgrade keep their old primes (accepted).
- A project whose directory has disappeared is listed as unavailable instead of breaking the boot.

## Scope

- New registry package: global user-level config load/save (atomic writes, fail-open reads), slug generation from folder basename (lowercase `[a-z0-9-]`, deduped), display name derived at render time from per-repo settings (fallback: path basename).
- New hub/multi-project supervisor in `serve`: registry-driven boot of one store+server per project, prefix mounting, shared opencode client, landing handler and global registry API (`/api/projects`).
- Base-path plumbing: server-side (`pageData.BasePath`, `HX-Redirect`, breadcrumbs/`BoardURL`, setup redirects, per-project `PublicBase`) and client-side (templates + `app.js` fetch/`EventSource` via one injected base constant).
- Projects landing page (add/remove) and header switcher UI.
- Tests for registry, mounting/isolation, URL emission in both base modes, and the new endpoints; README + doc learnings updates.

## Non-goals

- Authentication, TLS, or any change to the trusted-LAN/VPN deployment model.
- Per-project enable/disable, start/stop, or ordering controls (every registered project is always served).
- Cross-project search, aggregate boards, or any global workflow view.
- CLI verbs for registry management (UI-only management, per decision).
- Any change to workflow semantics, store formats, or per-project settings schema.

## Design decisions

- **`/p/<slug>` prefix mounts over cookie switching**: multiple projects must be usable side-by-side in separate browser tabs; bookmarks stay stable. The slug never collides with root routes because all project traffic lives under `/p/`.
- **Registry in `~/.config/lessmess/config.json`**: user-level (not per-repo, not committed), written only by the server, `model.WriteFileAtomic`-style atomic writes, malformed file degrades to empty registry with a warning (fail-open like all lessmess state). Schema: `{"projects":[{"slug","path","added"}]}` — display names are *not* stored; they derive from each repo's settings at render time.
- **UI-only management**: add/remove happen on the landing page; the server is the only writer of the global config.
- **Legacy `--dir` mode preserved**: `--dir` present → today's single-project root mount with empty base; absent → registry mode. The base-path plumbing handles both (empty base emits today's URLs byte-for-byte).
- **Per-repo accents kept**: each project's brand routes resolve its own accent through the existing per-mount handlers; the landing page itself uses the default accent (no roll — there is no repo to persist to).
- **Shared opencode client**: one discovered service client; all spawns already take an explicit directory, so per-project sessions need no new client machinery.

## Acceptance criteria

- `lessmess serve` (no `--dir`, registry with ≥2 projects) serves both projects at `/p/<slug>/`, each with a working board, SSE updates, chat, and settings; `/` lists both with working add/remove.
- Adding a valid directory hot-boots it without restart; adding an invalid path returns a clear error and registers nothing; removing unmounts it and deletes the registry entry while leaving the repo untouched.
- `lessmess serve --dir <repo>` behaves exactly as before (regression-tested).
- Two projects open side-by-side in two tabs; each tab's SSE stream updates only its own project.
- An agent session spawned in project A receives prime curl instructions based at `host:port/p/<slugA>`, and `POST /p/<slugA>/changes/scaffold` lands the change in project A.
- Each project renders its own accent; `go vet ./...`, `go test ./...`, and `lessmess validate` are clean.

## Tasks

1. (task breakdown is maintained by the tool; see the board)
