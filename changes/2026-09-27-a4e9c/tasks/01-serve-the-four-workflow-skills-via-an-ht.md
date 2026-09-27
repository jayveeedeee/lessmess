# SKIL-01: Serve the four workflow skills via an HTTP catalog

## Why

Capabilities (scaffold, task operations, handoff, closeout) must be
available on demand and compaction-proof. OpenCode V2 loads skills from
HTTP catalogs: a base URL with `index.json`, each entry downloaded as
`<name>/<name>.md` (opencode.ai/v2/docs/skills). Serving them from the
lessmess binary means zero local skill files and one source of truth.

## What

- New file (e.g. `internal/server/skills.go`):
  - `GET /skills/index.json` — `{"skills":[{"name","version","files":[...]}]}`
    for exactly `lessmess-scaffold`, `lessmess-task`, `lessmess-handoff`,
    `lessmess-closeout`. `version` is a content hash of the rendered
    bodies so OpenCode refreshes its cache when content changes.
  - `GET /skills/<name>/<name>.md` — frontmatter (`name`, `description`
    with trigger phrasing) + body rendered from the retired module texts
    (scaffold/task/handoff/closeout), written generically (no per-session
    placeholders; apiBase baked at render), each body opening with its
    behavioral gate so standalone loads are safe. The
    `lessmess-scaffold` body must **explicitly welcome unbound (free
    chat) sessions** — scaffolding from a free chat is a supported
    path, and on success the session becomes the change's session.
- Route registrations in `server.go`; hub-mode prefixing works like any
  project route (project handler sees unprefixed paths).
- Handler tests: manifest shape, hash stability on unchanged content,
  hash change on edited content, per-skill markdown fetch, 404 for
  unknown names.
- Also verify SKIL-04's free-chat smoke: a fresh free chat loads
  `lessmess-scaffold`, scaffolds, and ends up bound as the change's
  session.

## Files affected

- `internal/server/skills.go` (new)
- `internal/server/server.go`
- `internal/server/skills_test.go` (new)
- `README.md` (new endpoints)

## Verification

- `go vet ./... && go test ./...`.
- `curl <apiBase>/skills/index.json` lists four skills; download each
  markdown and confirm OpenCode loads them (skill appears in Chat's
  skill listing via `ListSkillsFor`).
