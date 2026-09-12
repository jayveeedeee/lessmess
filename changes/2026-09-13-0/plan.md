# 2026-09-13-0: Commit-all button on changes page

- Change ID: 2026-09-13-0
- Created: 2026-09-13
- Branch: —
- Status: see [ledger.md](ledger.md)

## Objective and context

Add a "Commit all" button to the changes index page (`GET /`) that commits
all uncommitted repository changes in one click, with an LLM-generated commit
message. Today, committing is only possible per change from the board page
(the `commit-btn` flow in `lifecycle.go`, which spawns an opencode session
that reviews the diff, writes a message, and commits). The index page has no
git awareness at all, so committing unrelated or cross-change work means
leaving the UI.

The new flow: the button sits next to "New change session", is disabled when
the working tree is clean (hidden entirely when the repo is not a git repo),
opens a confirmation modal listing the uncommitted changes, and on confirm
spawns an opencode session that writes the message and commits — the same
mechanism the board commit already uses.

## Current behavior

- `web/templates/index.html` renders only the "New change session" form in
  the `.section-head` of the Changes section.
- `POST /changes/{id}/commit` (`commitChange` in `internal/server/lifecycle.go`)
  spawns an opencode session primed with `commitPrompt(changeID)`, bound to
  one change; `GET /changes/{id}/commit-status?session=` polls whether the
  session is still busy (`commitStatus` reads only the query param, no path
  values).
- The server never executes git itself; all git work happens inside opencode
  sessions. `os/exec` is used only in `internal/opencode` (service status)
  and `internal/terminal` (PTY).

## Target behavior

1. **Button** — `index.html` gains a `Commit all` button (`#commit-all-btn`)
   next to the "New change session" form. The index handler computes git
   state server-side at page render:
   - not a git repo (or git unavailable) → button is not rendered;
   - clean tree → rendered `disabled`;
   - dirty tree → rendered enabled.
2. **Modal** — clicking the button fetches fresh status from
   `GET /api/git/status` and opens a modal (same `.modal` pattern as
   `#notif-modal`) showing one merged list: each uncommitted file with its
   two-letter status code, path, and per-file `+/-` line counts for
   tracked changes (`git diff --numstat HEAD`; untracked files carry no
   counts), plus a single `git diff --shortstat HEAD` summary line and
   Confirm/Cancel actions. If the fresh status comes back clean, the
   modal says so and Confirm is disabled.
3. **Confirm** — `POST /api/git/commit` re-checks dirtiness server-side
   (refuses with 422 "nothing to commit" if the tree went clean), then
   spawns an opencode session primed with a repo-wide variant of
   `commitPrompt` (no change-ID reference; `changes/` records are context,
   not the subject). The session is mapped to the unassigned Discussions
   bucket with title `repo — git commit`.
4. **Progress** — the modal switches to "Committing…" (spinner, Confirm
   disabled), polls `GET /api/git/commit-status?session=` every 3 s (same
   handler as the board commit status, registered at the new path — it is
   already change-agnostic), then shows "✓ Committed", disables
   `#commit-all-btn` on the page, and leaves the session inspectable under
   Discussions.
5. **Safety** — the prompt carries the same rails as the board commit:
   `git add -A` + commit only; never push, amend, rebase, reset, or switch
   branches. The server-side git helper is strictly read-only
   (`status --porcelain`, `diff --stat HEAD`, `rev-parse`).

## Scope

- New `internal/server/gitcommit.go`: read-only git helper
  (`gitStatus(dir)`) plus handlers `gitStatusAPI`, `commitAll`, and the
  repo-wide commit prompt builder; three route registrations in
  `server.go`.
- `internal/server/server.go` `index`: compute git state for the HTML
  render and pass it through `indexView` (new `GitRepo`/`GitDirty` fields
  in `render.go`).
- `web/templates/index.html`: the button and the `#commit-modal` markup.
- `web/static/app.js`: `initCommitAll()` — open modal, render preview,
  confirm, busy/poll/done states, error paths.
- `web/static/app.css`: styles for the commit modal (porcelain list,
  diffstat block), reusing `.modal` primitives.
- Tests: `internal/server/gitcommit_test.go` (helper against a temp git
  repo, status endpoint, commit endpoint with fake opencode client,
  clean-tree refusal) and a render assertion for the button state.

## Non-goals

- No pushing, ever; no branch management, no staging of individual files.
- No user-editable commit message (the LLM writes it; that was decision
  option A).
- No live SSE/polling of the button state on page load; state is computed
  at render, re-fetched when the modal opens, and re-checked on confirm.
- No changes to the per-change board commit flow (it keeps its own routes
  and prompt).
- No server-side commit path (no `git commit` executed by the server).

## Design decisions

- **Commit mechanism: reuse the opencode session** (user decision). The
  server runs git read-only for preview; the session writes the message and
  commits, consistent with the board commit.
- **Modal content: merged list + one-line summary** (revised 2026-09-13
  from the original "porcelain list + diffstat block" after user feedback
  that two lists of the same files read as redundant). `GET /api/git/status`
  returns per-file `+/-` counts (`git diff --numstat HEAD`) merged into the
  porcelain rows plus a `git diff --shortstat HEAD` summary line; untracked
  files appear without counts.
- **Progress UX: spinner in modal + poll** (user decision), mirroring
  `pollCommitStatus` in `app.js`.
- **Button state: page load + rechecks** (user decision) — render-time,
  modal-open fetch, and confirm-time server re-check; no live updates.
- **Session bookkeeping: unassigned Discussions bucket** (user decision),
  same as explorer chats, via `s.sessions.addUnassigned`.
- **Route shape**: repo-wide operations live under `/api/git/*`
  (`GET /api/git/status`, `POST /api/git/commit`,
  `GET /api/git/commit-status`) because they are not change-scoped; the
  existing change-scoped `commitStatus` handler is reused verbatim for the
  third route since it only reads the `session` query param.
- **Not-a-git-repo handling**: `gitStatus` treats `rev-parse` failure as
  "no repo" and returns `repo:false` (200), so the UI degrades silently.
- **Concurrency**: client-side busy flag disables Confirm while a commit
  session is in flight; no server-side lock (accepted risk, see Risks).

## Acceptance criteria

1. Index page in a dirty git repo shows an enabled "Commit all" button;
   clean repo → disabled; non-git repo → absent.
2. Clicking it opens the modal listing the current porcelain entries and a
   diffstat, fetched live.
3. Confirm spawns a commit session mapped to Discussions; the modal shows
   busy → "✓ Committed" when the session finishes; the page button becomes
   disabled.
4. If the tree is clean at confirm time, `POST /api/git/commit` returns 422
   and no session is created.
5. The commit session's prompt forbids push/amend/rebase/reset and contains
   no change-ID reference.
6. `go vet ./...` and `go test ./...` pass; the binary builds with
   `CGO_ENABLED=0 go build -o lessmess ./cmd/lessmess`.
7. `lessmess validate` reports no workflow violations for this change.

## Tasks

1. [CMT-00](tasks/00-git-status-helper-and-endpoint.md) — read-only git helper + `GET /api/git/status`
2. [CMT-01](tasks/01-commit-all-endpoint.md) — repo-wide commit endpoint + prompt + status reuse
3. [CMT-02](tasks/02-index-button.md) — index page button with server-rendered state
4. [CMT-03](tasks/03-commit-modal.md) — modal markup, CSS, and JS flow
5. [CMT-04](tasks/04-verify-and-docs.md) — end-to-end verification and docs updates
6. [CMT-05](tasks/05-watcher-chmod-loop.md) — fix watcher CHMOD loop triggered by git scans
