# EXP-03: De-docs the wizard and lessmess init

Status: see [../ledger.md](../ledger.md).

## Objective

Remove docs from all onboarding paths: the setup wizard loses its coverage
checkbox, exclusion picker, and docs-seed step (6 → 5 steps), and
`lessmess init` stops writing the default `agentsdocs.json`.

## Dependencies

EXP-02. Settings must own the complete docs-onboarding path before the wizard
path is removed.

## Scope

- `web/templates/setup.html`: delete the `#setup-coverage` checkbox +
  `#setup-exclude-list` picker from the bootstrap step and the whole
  `docs` step section + its `data-step-nav` item; update bootstrap completion
  and agent-step copy that still mentions docs/exclusion updates.
- `web/static/app.js` (`initSetup` ~5090): drop `docsCoverage`/`excludeDirs`
  from the bootstrap POST body, remove the docs-seed polling and step
  navigation bits.
- `internal/server/setup.go`: drop `DocsCoverage`/exclude fields from
  `bootstrapRequest`, and remove the `docs-coverage` onboarding-step
  bookkeeping in `bootstrap`. **Retain** `/api/setup/dirs`,
  `exclusionDirEntries`, `validExcludePattern`, and `updateConfigExcludes`:
  the normal Settings exclusions editor still uses all of them.
- Setup docs-seed endpoints: remove `POST /api/setup/docs-seed` and
  `GET /api/setup/docs-seed-status` routes and only the setupEnv handlers in
  `setupseed.go`; retain the shared seed job, request/status types, and
  `startDocsSeedJob` used by normal `POST /docs/seed`. Remove its obsolete
  onboarding `docs=seeded` bookkeeping. `prereqs.go`: remove the
  `docs-coverage` informational check.
- `internal/docs/init.go`: `Init` no longer writes `agentsdocs.json`; after the
  wizard caller is gone, remove obsolete `InitOptions`/`InitWithOptions` and
  wizard-only exclude initialization if no production caller remains. Keep the
  config-only initializer added by EXP-02.
- `cmd/lessmess/main.go`: `init` output no longer claims coverage creation;
  `docs seed` error text points at Settings → Docs (wording may already
  match EXP-01 — avoid double-editing).
- Tests: render tests for the 5-step wizard (`render_test.go` setup
  assertions), setup handler tests, `internal/docs` init tests.

Out of scope: the settings-side initialize flow (EXP-02), the re-run link in
Settings → General (the wizard remains re-runnable, just docs-free).

## Implementation steps

1. Remove the docs step + bootstrap docs controls from `setup.html` and the
   matching `initSetup` logic.
2. Remove only the setup docs-seed routes/handlers and the `docs-coverage`
   prereq; slim `bootstrapRequest` while preserving Settings exclusion helpers.
3. Change `internal/docs.Init` to skip the coverage file, remove obsolete init
   options, and adjust its tests.
4. Update `main.go` init/seed messaging; update render tests to the 5-step
   shape.
5. `go vet ./... && go test ./...`; grep for leftover
   `setup/docs-seed`/`setup-coverage`/`docsCoverage` references, while
   confirming normal `/docs/seed` and Settings exclusions still work.

## Verification

- `go test ./...` green; no `grep` hits for the removed ids/routes outside
  history.
- Manual: run `lessmess init` in a scratch repo → no `agentsdocs.json`
  created; open `/setup` → 5 steps, bootstrap step has no docs checkbox,
  no docs step in the nav; completing the wizard works end to end. Normal docs
  seed and exclusion editing remain available after enabling docs in Settings.

## Completion criteria

- Onboarding contains zero docs affordances; docs onboarding lives only in
  Settings (EXP-02). Existing repos' committed `agentsdocs.json` files are
  untouched by any of this.
