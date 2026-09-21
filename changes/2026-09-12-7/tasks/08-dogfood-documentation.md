
# DOC-08: Dogfood on this repo and documentation

Status: see [../ledger.md](../ledger.md).

## Objective

Prove the system end-to-end on the tasktracker repository itself, tune prompts from
observed behavior, and document the subsystem in `README.md` and the root
`AGENTS.md`.

## Dependencies

- DOC-03, DOC-04, DOC-05, DOC-06, DOC-07

## Scope

- Run `tasktracker docs seed --dry-run` on this repo; review plan; run the real
  seed; review generated docs for quality (record samples in Notes).
- Close a real (small) change and observe the full close-out → queue → gardener →
  metadata → validate cycle; confirm no recursion and byte-identical human content.
- Exercise the stale fallback (stop the opencode service, close a change, restart,
  reconcile).
- Tune gardener/seed prompts based on findings.
- Update `README.md` (new commands, safety model, config) and root `AGENTS.md`
  (docs-system section: what the files are, how agents should treat them,
  requirement to keep area learnings accurate).
- Record any scope/design deviations in the ledger decision log and sync `plan.md`.

## Implementation steps

1. Seed dry-run + review; adjust config/blurb prompts as needed.
2. Real seed; commit-quality review of output (human read-through).
3. Close-out dogfood with service up; verify confinement, provenance, idempotence.
4. Stale-fallback dogfood with service down.
5. Documentation updates; final `go vet ./... && go test ./...`.

## Verification

- All change-level acceptance criteria in `plan.md` checked off with evidence here.
- Fresh eyes pass: generated docs are accurate and useful to an agent entering a
  folder cold.

## Completion criteria

- End-to-end cycle demonstrated on this repo with evidence recorded.
- Documentation merged; prompts tuned; no known false claims in `README.md` /
  `AGENTS.md`.

## Files affected

- Generated `STRUCTURE.md` / `AGENTS.md` files across this repo (covered dirs)
- `README.md`
- `AGENTS.md`
- `agentsdocs.json` (tuning, if needed)
- Prompt strings in `internal/docs/summarize.go` / `internal/server/docssession.go`
  (tuning, if needed)

## Notes

- Dogfooding evidence and prompt-tuning decisions go here as the work proceeds.
- 2026-09-12 — Seed dry-run on this repo: 14 covered dirs in correct post-order,
  no writes.
- 2026-09-12 — First real seed FAILED at session creation (opencode API 500 for
  all 14 dirs). Root cause: the CLI passed `--dir` verbatim (`"."`) as the
  session location, and the service requires an absolute directory (the server
  path was unaffected — `store.Open` absolutizes). Fixed in `runDocsSeed`
  (`filepath.Abs`). Skeletons from phase 1 were already on disk and carried
  forward. Verified the service behavior with curl (relative → 500, absolute →
  created).
- 2026-09-12 — Before the rerun, seed verification was strengthened to full
  gardener-grade confinement for BOTH doc files: out-of-marker bytes must stay
  identical to the pre-pass snapshot, no deletions, created AGENTS.md must use
  markers (previously only parse+meta were checked — an LLM editing human
  content would have passed). `TestWorkflowAssetDrift` now ignores the repo
  AGENTS.md's machine auto section (asset = curated portion only), since
  seed/gardener append learnings there.
- 2026-09-12 — Second live failure (seed rerun in progress): sessions taking
  >30s failed at `wait` with a client-side timeout. Root cause: the service's
  /wait endpoint blocks server-side past the HTTP client's 30s cap. Fixed
  `Client.WaitDone` to retry transport timeouts until ctx expires. Subtlety:
  `os.IsTimeout` does NOT see through `c.do`'s fmt wrapping (its unwrap chain
  only covers os/url error types) — `isTimeoutErr` uses
  `errors.Is(context.DeadlineExceeded)` + `net.Error` instead. Covered by
  `TestWaitDoneRetriesTransportTimeout` / `TestWaitDoneCtxExpiryReportsBusy`.
  Knock-on behavior verified safe: wait-timeout dirs roll back to skeletons
  (DeleteSession kills the agent, restore rewrites), so the rerun resumes
  cleanly from the cursor.
- 2026-09-12 — SEED COMPLETE across 4 runs (resume proven): 13/14 dirs seeded
  by LLM, quality reviewed (samples: internal/store, web/templates — accurate
  purposes/blurbs and genuinely useful learnings, e.g. the globally-unique
  template-name gotcha). Two deterministic confinement CAUGHTs:
  `internal/docs` (agent wrote two marker pairs) and root (agent edited the
  270-line workflow canon outside markers) — both rolled back byte-cleanly.
  Prompt tuning (HARD RULES: exactly one marker pair, append-only at end)
  fixed internal/docs; root failed 3× deterministically — resolution:
  hand-seeded the root auto section, marked "." in the cursor; the gardener
  maintains it from here with confinement as the guard.
- 2026-09-12 — CLOSE-OUT DRILL PASSED (second instance on :9099, real
  service): reopen+close 2026-09-12-2 → job dirs=[web/templates] → gardener
  done in ~3.8 min → 2 accurate learnings prefixed `(2026-09-12-2)` appended
  inside the markers only; STRUCTURE.md untouched (no placeholders); no
  recursion; queue empty; validate OK.
- 2026-09-12 — STALE DRILL PASSED with a finding: instance with opencode2
  stripped from PATH → close → stale={web/templates: opencode service
  unavailable} → validate reports the stale warning. Reconciliation via a
  SECOND healthy instance returned "no stale dirs": queue state is loaded at
  startup, not shared live between instances. Accepted as within the
  documented one-process-one-repo model (recorded as a known limitation, not
  a bug); after restart the SAME healthy instance reconciled via
  POST /docs/refresh → manual job → stale cleared.
- 2026-09-12 — Manual-job prompt tuned after observation: the agent
  improvised sane "(manual)"-prefixed learnings; gardenerPrompt now has an
  explicit manual branch (no change-record read, "(manual)" citation).
- 2026-09-12 — Self-reference hazard found and fixed: writing the literal
  marker pair in AGENTS.md prose (docs section + asset-maintenance command)
  formed extra marker pairs → "multiple tasktracker marker pairs" errors.
  Canonical text now writes `tasktracker:begin`/`tasktracker:end` without the
  HTML-comment wrapper, and the asset-maintenance command matches the marker
  prefix line only. Asset regenerated; init's skip/contains checks now
  TrimRight-normalize trailing newlines.
- Final state: `tasktracker validate` OK (0 findings), `go vet ./...` clean,
  full `go test ./... -count=1` green (5 packages), `gofmt` clean. README and
  root AGENTS.md document the subsystem; embedded asset in sync (drift test).
