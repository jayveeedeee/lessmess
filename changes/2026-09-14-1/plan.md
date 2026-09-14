# 2026-09-14-1: Discussion sessions open fresh

- Change ID: 2026-09-14-1
- Created: 2026-09-14
- Branch: —
- Status: see [ledger.md](ledger.md)

## Objective and context

Clicking **New change session** on the changes index page spawns a discussion session and immediately primes it. The prime (base `discussionPrompt` + the project question-policy addendum) tells the agent to "start from the user's initial request … do read-only investigation" — but at creation time no user request exists. The agent therefore spends its opening turns reading the ledger and reviewing open changes, so the session behaves like a "review the existing changes" session instead of a blank new-change session. The user wants a fresh session that waits for their idea.

## Current behavior

- `POST /changes/session` (`createDiscussionSession`, `internal/server/changesession.go`) spawns an opencode session and runs `s.promptWith(discussionPrompt(...), "discussion")` as the prime immediately, before the user types anything.
- The prime = base prompt (four numbered steps) + `prompts.discussion` addendum from `lessmess.json` (change 2026-09-13-3), which mandates "do read-only investigation" from the initial request — an instruction with no request to act on.
- Result: the freshly opened session audits `changes/` and open changes unprompted.
- The Settings-page per-field Change button (`settingschange.go:186`) reuses `discussionPrompt` but appends a context block naming the requested field — there the prime *does* carry a request.

## Target behavior

- A discussion session created from the index page performs **no repository reads** and produces **no summary of open changes** before the user's first message; it replies with a single short line inviting the request and stops.
- Once the user sends their first message, today's behavior applies unchanged: read-only investigation, findings/options/recommendation, zero-questions policy, scaffold on explicit approval.
- The Settings Change flow still engages immediately (its prime carries a context block = a request).
- Only newly created sessions are affected; existing sessions keep their original prime.

## Scope

- Rework `discussionPrompt` in `internal/server/changesession.go` to add an explicit empty-state rule (step 0) with precedence over all later instructions, including appended addenda.
- Extend `internal/server/changesession_test.go` to pin the new clauses.
- Rebuild the binary and verify live in the running UI.
- **Scope extension (DSC-02)**: keep the terminal overlay open when a discussion scaffolds a change, by following the session to its new board instead of full-reloading the index page (see "Scope extension" below).

## Non-goals

- No change to `lessmess.json` / the `prompts.discussion` addendum (user chose the base-prompt path; the addendum stays as committed by 2026-09-13-3).
- No changes to `changePrompt`, `explorerPrompt`, `gardenerPrompt`, commit prompts, or any endpoint/route/mapping behavior.
- No README, workflow-text, or Go server changes beyond the prompt reword; DSC-02 touches client JS only (`web/static/app.js`, docs-excluded).

## Scope extension: terminal persistence across scaffold (DSC-02)

Found during acceptance of DSC-01: when the planning agent scaffolds, the terminal closes and the user lands back on the changes index. Cause (verified): `scaffoldChange` writes `changes/`, the store watcher emits SSE `fs`/`write` events, and the index page's live-update handler (`web/static/app.js:106-111`) does `location.reload()` for `page === "index"` — a full reload that destroys the terminal overlay. Board pages survive because they refresh via an htmx fragment swap; the terminal-panel re-resolution code just below the reload (lines 112–118) never runs on the index because the reload executes first.

Fix direction: when `changes/` events arrive while the terminal is open on a non-board page, skip the full reload and resolve the open session's binding; once bound, navigate to `/changes/{id}?session={sid}` — the existing `autoOpenSession` flow (`app.js:776`) then reopens the same session on the new change's board with its task panel. While the session is still unbound, skip the reload (background list staleness is acceptable and self-heals on navigation).

## Design decisions

- **Empty-state rule as step 0, conditioned on "nothing below it"**: the Settings Change prime appends its context block *below* the base prompt, so gating on "no user request accompanies this message (nothing below it)" lets that flow proceed immediately while a bare index-page prime waits.
- **Explicit precedence sentence**: the question-policy addendum ("do read-only investigation") is appended *after* the base prompt, so step 0 states it wins over everything — "including any appended addendum" — until the user's first message. Without this, the last-read instruction would likely win.
- **Keep all pinned phrasing**: "planning assistant", "DO NOT modify the repository", "EXPLICITLY agrees", "task-ID prefix", the exact scaffold `curl`, and step 1's "Ask questions; help them decide" (the addendum overrides it by reference) are preserved, so `settingswiring_test.go` and `settingschange_test.go` stay green and `lessmess.json`'s override wording stays accurate.
- **Base prompt, not config**: chosen by the user; base prompts apply to every checkout and cannot be silently edited per repo.

Authoritative prompt text (single source of truth for DSC-00; `%[1]s`/`%[2]s` are the existing `apiBase`/session-ID format verbs):

```text
You are a planning assistant for a repository that uses the change-management workflow defined in AGENTS.md. This session exists to plan a NEW change from the user's own request.

0. Empty state: this prime message may arrive before the user has typed anything. If no user request accompanies this message (nothing below it), do NOT investigate the repository — do not read changes/, any ledger, or open changes — and do not summarize anything. Reply with a single short line inviting the request (for example: "What would you like to build?") and stop. Every other instruction in this message — above, below, or in an appended addendum — applies only from the user's first message onward.
1. Discuss with the user what they want to build: objective, context, scope, and design options. Ask questions; help them decide.
2. DO NOT modify the repository in any way — no change directories, no edits, no scaffolds. Discussion only.
3. When the user EXPLICITLY agrees to start the work, choose a concise change title and a 2–4 letter uppercase task-ID prefix, then scaffold the change by running exactly this (replacing <title> and <prefix>):

curl -s -X POST %[1]s/changes/scaffold -H 'Content-Type: application/json' -d '{"title":"<title>","prefix":"<prefix>","session":"%[2]s"}'

This call creates the change directory, registers your title and prefix, renames this session, and links it to the new change. Report the returned change ID to the user.
4. Then refine changes/<id>/plan.md and break the work into verifiable tasks per AGENTS.md (task files plus matching ledger rows), keeping the user in the loop before any implementation.
```

## File-level impact

- `internal/server/changesession.go` — `discussionPrompt` body only (plus its doc comment).
- `internal/server/changesession_test.go` — new substring assertions in `TestDiscussionSession` (empty-state invite, no-investigation rule, precedence over addenda).
- No other files. `settingswiring_test.go` / `settingschange_test.go` are expected to pass unchanged and act as regression guards.

## Data, API, configuration, schema changes

- None. Same endpoints, same mapping behavior, no settings or workflow-file changes.

## Safety, security, and rollback

- Prompt text only; no repository writes at prime time. Rollback is reverting the one function body and rebuilding.

## Testing and verification strategy

1. `go vet ./... && go test ./...` from the repo root, with the new assertions failing against the old prompt and passing against the new one.
2. Rebuild: `CGO_ENABLED=0 go build -o lessmess ./cmd/lessmess`; restart the running server (embedded assets and prompt text are compiled in).
3. Live: create a discussion from the index page and confirm the session opens with the one-line invite and zero tool calls; then send an idea and confirm normal planning behavior.
4. Live regression: use a Settings-page Change button and confirm the discussion engages on the named field immediately.

## Observability

- Existing `discussion session created` log line suffices; no new events.

## Rollout sequence

1. DSC-00 rewords the prompt and updates tests; `go vet ./... && go test ./...`.
2. DSC-01 rebuilds, restarts, and live-verifies both flows.
3. User acceptance on a real new-change discussion.

## Risks and mitigations

- **Model still investigates despite step 0**: the addendum is appended last, so precedence could lose. Mitigation: DSC-01 verifies live; if ignored, fall back to a small `prompts.discussion` wording tweak (config-only, no rebuild) as a follow-up.
- **Settings Change flow stalls**: mitigated by the "(nothing below it)" condition; regression-checked by existing tests plus DSC-01 live check.
- **Scaffold curl drift**: text is copied verbatim from the current builder; `TestDiscussionSession` pins the exact call.

## Acceptance criteria

1. A new index-page discussion session opens with a single invite line and performs no repository reads before the user's first message.
2. The Settings Change flow still engages immediately on its context block.
3. `go vet ./... && go test ./...` passes with the new prompt assertions in place.
4. Existing sessions are unaffected; no settings, workflow, or endpoint changes.
5. Plan, ledger, and task files agree.
6. (DSC-02) When a discussion opened from the index scaffolds a change, the terminal stays open: the user lands on the new change's board with the same session reconnected and the task panel visible — no kick back to the index.

## Tasks

1. [DSC-00](tasks/00-empty-state-prime-rule.md) — Add empty-state rule to `discussionPrompt` and pin it in tests.
2. [DSC-01](tasks/01-rebuild-and-live-verify.md) — Rebuild the binary and verify the fresh opening live.
3. [DSC-02](tasks/02-keep-terminal-on-scaffold.md) — Keep the terminal open when a discussion scaffolds (follow the session to its board).
