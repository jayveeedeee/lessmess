# 2026-09-27-a4e9c: Workflow skills and compaction re-prime

- Change ID: 2026-09-27-a4e9c
- Created: 2026-09-27
- Branch: —
- Status: tracked in the tool-owned JSON state (.lessmess/workflow/)

## Objective and context

Workflow instructions currently reach an agent exactly once: as a large
"prime" prompt composed at session spawn from the embedded modules in
`internal/server/instructions.json`. Two failure modes motivated this
change, both observed in practice:

1. Compaction erases the prime — after a few compactions the agent has
   lost the task-creation/status/handoff rules entirely (this happened
   to a real session, which triggered this change).
2. Sessions spawned before an instruction change never receive the new
   text — primes are render-once at spawn.

The agreed fix restructures delivery into two tiers and makes the
capability tier compaction-proof. As part of it, **free chats gain the
ability to scaffold changes**: the scaffold endpoint already accepts
unbound sessions (a successful scaffold renames the session and binds
it to the new change), so the work is to grant it at the prompt layer —
replace the `chat` module's blanket workflow prohibition with skill
pointers — and to guarantee a `lessmess-scaffold` skill that welcomes
unbound sessions instead of guarding them away.

## Current behavior

- `instructions.json` holds seven modules: `discussion`, `change.session`,
  `change.handoff`, `worktree`, `task.session`, `closeout`, `chat`.
- `renderPrime` composes the audience-selected modules plus a state
  snapshot into one spawn prompt (`internal/server/instructions.go`,
  `internal/server/changesession.go`). Injected once, never again.
- No skills are served by the Go service; no HTTP skill catalog exists.
- Compaction detection exists only as transcript display states
  (`internal/server/chat.go` compaction case); nothing reacts to it.

## Target behavior

### Tier 1 — base prompts (spawn-only, lean)

Mission + identity + skill pointers only. No endpoint lists, no curls,
no procedure text. New versions:

- `discussion` v3: plan a new change; read-only until explicit approval;
  step 0 empty state unchanged; on approval load `lessmess-scaffold` and
  follow it; never create a change by other means.
- `change.session` v3: identity (bound to `{{changeId}}`, never hand-edit
  `.lessmess/workflow/`, narrative docs are direct edits); everything is
  THIS change, never scaffold; pointers to `lessmess-task`,
  `lessmess-closeout`, `lessmess-handoff`; delegation one-liner (prefix
  subagent description with real task ID; prefer write-capable
  subagents).
- `task.session` v3: identity (task `{{taskId}}` of change
  `{{changeId}}`); plan-only arrival (step 0); subtree scope; same three
  pointers; delegation with dotted IDs; never scaffold; further
  decomposition only on explicit go-ahead.
- `chat` v2: free chats gain workflow reach through skills —
  `lessmess-scaffold` (session becomes the change's session on
  scaffold), `lessmess-task`, `lessmess-closeout`; `lessmess-handoff`
  noted as change-bound-only; never hand-edit `.lessmess/workflow/`.

### Tier 2 — capabilities as service-served skills

Four skills, rendered from Go (single source of truth with the
modules), served as an OpenCode V2 HTTP skill catalog:

- `GET /skills/index.json` — `{"skills":[{"name","version","files"}]}`,
  `version` = content hash of the rendered set so caches refresh.
- `GET /skills/<name>/<name>.md` — frontmatter (name, description with
  trigger phrasing) + body.

Bodies absorb the retired injected modules:

- `lessmess-scaffold` (from `discussion` steps 3–4): approval gate;
  title/prefix rules; the scaffold curl; reading the response;
  validator run; post-scaffold planning; transition note ("this session
  is now the change's session; load `lessmess-task`"). Unlike the
  other skills' guards, this one must **explicitly welcome unbound
  (free chat) sessions** — scaffolding from a free chat is a supported
  path, and the body should say the session becomes the change's
  session on success.
- `lessmess-task` (from `change.session` step 2 + task flows): create
  task (201 returns prose path; `# <ID>: <title>` heading + standard
  sections), statuses (Test requires `evidence`; Done refused for
  agents), update/reorder, `expand` user-instructed only, decisions
  endpoint, delegation detail, validator after structural writes.
- `lessmess-handoff` (from `change.handoff`): requires change binding
  (else scaffold instead); write `handoff-<topic>.md`; spawn-change
  curl; explicit approval gate.
- `lessmess-closeout` (from `closeout`): finish at Test with evidence;
  user owns Done and close.

Each body opens with its behavioral gate so it is safe when loaded
standalone. Bodies are written generically (no per-session
placeholders; "your change"/"your task"); apiBase baked at render.

### Compaction re-prime (snapshot-only)

- Mark on queue: `sessionCompact` (`internal/server/sessionlifecycle.go`)
  marks the session after a successful `CompactSession`.
- Observe completions: the chat transcript walk already classifies
  compaction states (`internal/server/chat.go` ~442); record the newest
  completed compaction per session — this covers service-side
  auto-compaction.
- Wrap next prompt: in `chatPrompt` and `sessionDeliver`, for a bound
  session with an uncompensated compaction, prepend a compact re-prime —
  binding line ("you are task X.Y of change X") + fresh state snapshot +
  "context was compacted; continue where you left off — procedures are
  in your `lessmess-*` skills" — then the user's text; clear the marker.
  Fail-open: any lookup/read error sends the raw text.
- Memory-only markers (restart loses them: worst case one
  uncompensated compaction). No event subscription (volatile by
  contract; the transcript we already poll is the detection source).

### Config

Served repositories' `opencode.json` gains
`"skills": ["<apiBase>/skills/"]`, patched through the existing
order-preserving single-key machinery pattern (`opendefault.go`).

## Scope

- `internal/server/instructions.json` — module rewrites, retire
  `change.handoff` + `closeout` as injected modules (text moves into
  skill bodies).
- `internal/server/instructions.go` + new skill-rendering/catalog code
  and routes; `server.go` route table entries.
- `internal/server/chat.go`, `sessionlifecycle.go` — re-prime hooks.
- Config patch helper for the `skills` entry.
- Tests: `instructions_test.go` pins for new versions; catalog handler
  tests; re-prime tests (pending mark, transcript-observed completion,
  unbound never wrapped, fail-open, `sessionDeliver` parity).
- `README.md` for the new `/skills/` endpoints.

## Non-goals

- The `worktree` module stays exactly as-is: spawn prime only for
  worktree-backed changes; no skill, no re-inject.
- Machine-spawned sessions (commit, reviewer, gardener, settings) keep
  their own prompts; out of scope.
- No event-stream subscription to the opencode service.
- Prompts sent from outside lessmess (opencode TUI) bypass the wrap —
  accepted.
- If no prompt follows a compaction, nothing is restored — accepted
  (no turn to protect).

## Design decisions

- Granularity: one skill per user-triggerable moment (four total);
  decisions and validator fold into `lessmess-task`/`lessmess-scaffold`
  rather than new skills.
- Capabilities are NOT re-injected as text after compaction — the skill
  listing is re-supplied by OpenCode outside message history at every
  model step, making capability *availability* compaction-proof by
  construction. Only the personalized state snapshot + binding line are
  re-injected.
- Free chats may scaffold (endpoint already accepts unbound sessions;
  session becomes the change's session on success). Handoff remains
  change-bound-only, enforced by the endpoint and mirrored in the skill
  guard.
- Identity prompts are never re-injected; only the binding line inside
  the snapshot wrap is restored.

## Acceptance criteria

- All four base prompts render lean (no endpoint lists); version bumps
  pinned by tests.
- `GET /skills/index.json` lists exactly the four skills with a stable
  content-hash version; each markdown downloads and parses as a valid
  OpenCode skill.
- After a queued or auto compaction, the next prompt to a bound session
  carries binding line + fresh snapshot + skills reminder; unbound
  sessions and clean sessions are untouched; failures never block a
  prompt.
- **Free-chat scaffold works end to end**: a fresh free chat sees
  `lessmess-scaffold` in its skill listing, loads it, scaffolds on user
  approval, and ends up renamed and bound as the change's session.
- `go vet ./... && go test ./...` green; `lessmess validate` clean.

## Tasks

1. (task breakdown is maintained by the tool; see the board)
