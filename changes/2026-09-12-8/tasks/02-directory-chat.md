---
id: EXP-02
title: Directory chat sessions
---

# EXP-02: Directory chat sessions

Status: see [../ledger.md](../ledger.md).

## Objective

Add `POST /explorer/chat`: create an opencode session scoped to the repo root,
prime it with a directory-focused prompt embedding that dir's STRUCTURE.md and
AGENTS.md, map it to the unassigned bucket (Discussions list), and return the
session ID so the UI opens it in the terminal overlay.

## Dependencies

- EXP-01 (button hook in the tree)

## Scope

- `explorerPrompt(dir, structure, agents)` pure builder (tested): names the
  directory, embeds both docs, instructs root-context/directory-focus/no writes
  without explicit user request.
- Handler: validate `dir` against the covered set (rejects uncovered/arbitrary
  paths), 503 when the service is unavailable, create + prime + map unassigned,
  JSON `{session, title}`.
- JS: tree chat button posts, then opens the terminal overlay for the returned
  session (reuse the discussions open path).

## Implementation steps

1. Prompt builder + tests (content: dir name, both docs embedded, constraints).
2. `POST /explorer/chat` handler in `explorer.go` (mirror the
   createDiscussionSession flow; validation via the docs config).
3. `app.js`: click handler on `.explorer-chat` buttons → fetch → open terminal.
4. Tests with fake opencode client: happy path, bad dir rejection, service down.

## Verification

- `go test ./internal/server` passes; manual: chat from a tree node lands in
  Discussions and opens.

## Completion criteria

- A chat started from the `internal/server` node answers directory questions
  grounded in its docs, running in the repo root.

## Files affected

- `internal/server/explorer.go`
- `internal/server/explorer_test.go`
- `web/static/app.js`
- `web/templates/explorer.html` (button wiring)

## Notes

- 2026-09-12 — Implemented in `internal/server/explorer.go`: `explorerPrompt`
  (dir name + both docs embedded + root-context/focus/no-writes instructions),
  `POST /explorer/chat` (covered-dir validation via config AND existence,
  create → prime → map unassigned → 201 `{session,title,...}`; 503 with no
  oc/docs, 422 for bad dirs). `app.js` has a delegated click handler on
  `.explorer-chat` (stopPropagation/preventDefault since the button sits in
  `<summary>`; disables the button in-flight; opens the returned session in
  the terminal overlay). Decisions: validation rejects uncovered dirs even if
  they exist (no prompt injection via path); missing docs embed as
  "(not present)".
- Verification evidence: `go test ./internal/server -count=1` — happy path
  (201, prompt embeds purpose+blurb+constraints, session in /api/discussions),
  bad-dir rejections (uncovered/missing/changes), service-down 503, prompt
  content. `gofmt`/`go vet` clean; full suite green.