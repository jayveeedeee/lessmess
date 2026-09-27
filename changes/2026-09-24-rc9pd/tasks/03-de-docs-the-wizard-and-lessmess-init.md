# EXP-03: De-docs the wizard and lessmess init

Status: see [../ledger.md](../ledger.md).

## Objective

Remove docs from all onboarding paths: the setup wizard loses its coverage
checkbox, exclusion picker, and docs-seed step (6 → 5 steps), and
`lessmess init` stops writing the default `agentsdocs.json`.

## Dependencies

None strictly; landing after EXP-01/EXP-02 keeps the UI coherent (Settings is
the docs onboarding surface by then). Do not land before EXP-02 — that would
leave no way to enable docs.

## Scope

- `web/templates/setup.html`: delete the `#setup-coverage` checkbox +
  `#setup-exclude-list` picker from the bootstrap step and the whole
  `docs` step section + its `data-step-nav` item.
- `web/static/app.js` (`initSetup` ~5090): drop `docsCoverage`/`excludeDirs`
  from the bootstrap POST body, remove the docs-seed polling and step
  navigation bits.
- `internal/server/setup.go`: drop `DocsCoverage`/exclude fields from
  `bootstrapRequest` (accept-and-ignore or remove outright — prefer remove,
  since the wizard client is the only caller), remove
  `updateConfigExcludes` if now unused, remove the `docs-coverage`
  onboarding-step bookkeeping in `bootstrap`.
- Setup docs-seed endpoints: remove `POST /api/setup/docs-seed` and
  `GET /api/setup/docs-seed-status` routes and handlers (`setup.go`,
  `setupseed.go`); `prereqs.go`: remove the `docs-coverage` informational
  check.
- `internal/docs/init.go`: `Init` no longer writes `agentsdocs.json` (keep
  `InitOptions.Config` + `InitWithOptions` — EXP-02's endpoint and re-runs
  still use them); update init tests.
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
2. Remove the setup docs-seed routes/handlers and the `docs-coverage` prereq;
   slim `bootstrapRequest`.
3. Change `internal/docs.Init` to skip the coverage file; adjust its tests.
4. Update `main.go` init/seed messaging; update render tests to the 5-step
   shape.
5. `go vet ./... && go test ./...`; grep for leftover
   `setup/docs-seed`/`setup-coverage`/`docsCoverage` references.

## Verification

- `go test ./...` green; no `grep` hits for the removed ids/routes outside
  history.
- Manual: run `lessmess init` in a scratch repo → no `agentsdocs.json`
  created; open `/setup` → 5 steps, bootstrap step has no docs checkbox,
  no docs step in the nav; completing the wizard works end to end.

## Completion criteria

- Onboarding contains zero docs affordances; docs onboarding lives only in
  Settings (EXP-02). Existing repos' committed `agentsdocs.json` files are
  untouched by any of this.
