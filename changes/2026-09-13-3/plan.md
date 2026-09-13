# 2026-09-13-3: Discussion prompt question policy

- Change ID: 2026-09-13-3
- Created: 2026-09-13
- Branch: —
- Status: see [ledger.md](ledger.md)

## Objective and context

Discussion sessions (index-page "Start a discussion" and Settings-page Change buttons) are primed with the built-in discussion prompt, whose step 1 instructs the agent to "Ask questions; help them decide". The user wants the opposite default: discussions must open with no questions, work from the initial request, and when a question is genuinely unavoidable, ask it only as multiple choice.

The settings system supports exactly this via `prompts.discussion` — a free-text addendum appended verbatim to the built-in prompt at session-creation time (`Server.promptWith` in `internal/server/settings.go`; base prompts are never modified). This change sets that addendum at the project layer so it governs every discussion in this checkout.

## Current behavior

- `prompts.discussion` is unset; the effective value falls through to the built-in default (no addendum), so discussions are primed with the base prompt only.
- Base-prompt step 1 tells agents to open with questions, in whatever format they choose.

## Target behavior

- Every newly created discussion session receives the base prompt plus the agreed question-policy addendum (authoritative text below), stored at the project layer.
- Discussions open with zero questions; points the agent could resolve become explicitly stated assumptions; unavoidable questions are multiple choice only.
- Existing sessions keep their original prime — addenda apply at creation time only.

Authoritative addendum text (single source of truth for DQP-00):

```text
Question policy — this overrides the "Ask questions" instruction in step 1 of the base prompt:

- Never open a discussion with questions. Start from the user's initial request: state your understanding, do read-only investigation, then present findings, options, and a recommendation.
- Never ask about anything you can resolve yourself — from the repository, its docs, or a stated assumption. List assumptions explicitly so they can be corrected.
- When a question is truly unavoidable, always present it as multiple choice: 2–5 labeled options with brief descriptions, one marked recommended. Never ask open-ended questions.
```

## Scope

- Write the addendum value to the project settings layer via `PUT /api/settings?scope=project` (the server merges it section-scoped into root `lessmess.json`, atomically).
- Keep this change's plan/ledger/task files accurate.

## Non-goals

- No Go code, template, or static-asset changes (no rebuild needed).
- No changes to other prompt addenda (`change`, `commit`, `repoCommit`, `gardener`, `explorer`) or to any personal-layer settings.
- No changes to the base prompts themselves — they stay embedded and unmodified by design.
- No README updates: Settings docs already describe the prompts section and layering.

## Design decisions

- **Project layer, not personal**: the policy should govern every discussion in this repo, matching the Settings-page scope where this discussion was opened; `lessmess.json` is committed.
- **Override wording up front**: the base prompt explicitly says "Ask questions; help them decide", so the addendum opens by overriding that line rather than relying on position alone.
- **Assumptions instead of stalls**: prohibiting questions without an alternative pushes agents to stall; the addendum requires explicitly stated assumptions the user can correct.
- **Absolute multiple-choice rule**: "never open-ended" leaves no wiggle room and matches the harness's native option-dialog rendering.

## Implementation approach

One task: [DQP-00](tasks/00-write-project-addendum.md) — PUT the agreed value to the project layer, then read back and verify. The PUT is a patch (`applySettingsPatch`), so other sections of `lessmess.json` are preserved.

## File-level impact

- `lessmess.json` (root, committed): gains `prompts.discussion` — written by the server, atomically, section-scoped.
- `changes/2026-09-13-3/*`: plan/ledger/task bookkeeping only.

## Data, API, configuration, schema changes

- Configuration only: `lessmess.json` → `prompts.discussion` = the authoritative text above (newlines and quotes JSON-escaped in the PUT body).
- API used: `PUT /api/settings?scope=project` on the running server (`http://127.0.0.1:9090`); read-back via `GET /api/settings` (`prompts.discussion` effective value; `sources["prompts.discussion"] == "project"`).

## Safety, security, and rollback

- The write is atomic per layer and malformed files fail open to defaults; settings are re-read on every use, so no service restart is needed.
- Rollback: `PUT /api/settings?scope=project` with `{"prompts":{"discussion":""}}` — tri-state clearing restores inheritance to the built-in default — or remove the key from `lessmess.json`.
- No secrets, migrations, or rate limits involved.

## Testing and verification strategy

- `GET /api/settings` must return the exact authoritative text under `prompts.discussion` with `sources["prompts.discussion"] == "project"`.
- `GET /settings` page must render the value in the `prompts.discussion` field.
- Addendum application to the discussion prompt is already covered by `settingswiring_test.go`; no new tests are required for a config-only change. `go vet ./... && go test ./...` runs as a cheap sanity gate.

## Observability

- Existing server logging around settings writes suffices; no new metrics or events.

## Rollout sequence

1. Agree text (done in discussion).
2. DQP-00 writes the value and verifies read-back.
3. User acceptance on the next real discussion session.

## Risks and mitigations

- **Agents ignore the addendum**: mitigated by explicit override phrasing; if observed in practice, iterate on the wording via a follow-up settings write (cheap, no code).
- **Value drift between plan and file**: the plan block above is the single authoritative text; verification compares byte-for-byte against it.

## Acceptance criteria

1. Root `lessmess.json` contains `prompts.discussion` with text exactly matching the authoritative block above.
2. `GET /api/settings` reports the value with source `project`.
3. New discussion sessions are primed base prompt + addendum (covered by existing wiring tests; confirmed live during user acceptance).
4. Plan, ledger, and task files agree; task DQP-00 is `Test`.

## Tasks

1. [DQP-00](tasks/00-write-project-addendum.md) — Write discussion question policy to project settings.
