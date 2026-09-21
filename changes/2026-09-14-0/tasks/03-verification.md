
# SPS-03: End-to-end verification and docs update

Status: see [../ledger.md](../ledger.md).

## Objective

Prove the whole change with the rebuilt binary across pages, themes, and viewport
sizes, keep the covered docs pair accurate, and stage the change for user review.

## Dependencies

SPS-02 (all implementation tasks complete).

## Scope

- Full manual verification pass (list below).
- `web/templates/AGENTS.md` — update the settings-page learnings if the DOM
  structure notes changed (e.g. scope strip, sidebar, moved hooks); the doc
  gardener will refresh the auto section at close, but the curated notes should
  not be left stale.
- Dead-CSS sweep of the settings section of `app.css`.
- Ledger updates with verification evidence; tasks move to `Test`.

## Implementation steps

1. Rebuild: `CGO_ENABLED=0 go build -o lessmess ./cmd/lessmess`.
2. Run `go vet ./... && go test ./...` and `./lessmess validate`.
3. Manual pass:
   - Header sticky on index, explorer, settings; hidden only under the terminal
     overlay; board page unchanged; setup wizard unaffected.
   - Settings: scope strip pinned under header; segmented Personal/Project
     toggling re-renders badges/placeholders and saves to the right layer.
   - Side menu switches all five sections; URL hash survives reload; per-section
     save, per-setting Change buttons, datalists, and exclusions editor work.
   - Dark and light themes; viewport below 640px uses the stacked fallback.
4. Sweep superseded CSS rules; update `web/templates/AGENTS.md` learnings.
5. Record evidence in task notes and the change ledger; set tasks to `Test`.

## Verification

- All commands above pass; manual checklist observed on the rebuilt binary.
- `git diff` review shows only the intended files: `web/templates/settings.html`,
  `web/static/app.css`, optionally `internal/server/render_test.go` (justified),
  `web/templates/AGENTS.md`, and this change's files.

## Completion criteria

- Every acceptance criterion in [../plan.md](../plan.md) is met and evidenced.
- No leftover dead settings CSS; docs learnings accurate.
- All tasks `Test`; change reported ready for user review and close.

## Files affected

- `web/templates/AGENTS.md`
- `web/static/app.css` (dead-rule sweep only)
- Change files (ledger, task notes)

## Notes

- If any `render_test.go` assertion had to change, summarize the justification
  here for the user's review; the goal was zero test edits.
- Outcome: zero test edits were needed — all pre-existing assertions pass
  against the new markup unchanged.
- Docs decision: no manual edits to `web/templates/AGENTS.md` — its content is
  entirely inside the marker-guarded auto section (machine-maintained), and the
  existing settings learnings remain accurate since every DOM hook
  (`#settings-page`, `settings-scope` radios, `.settings-nav`, `data-*`) is
  unchanged. The doc gardener adds the split-shell learning when the change
  closes. STRUCTURE.md is unaffected (no entry purposes changed).
- Evidence (2026-09-14): `CGO_ENABLED=0 go build -o lessmess ./cmd/lessmess`,
  `go vet ./...`, `go test -count=1 ./...` all green; `./lessmess validate`
  exit 0 after syncing the root-ledger row to In progress (the four docs
  warnings pre-date this change and are unchanged); dead-CSS sweep done — no
  rule in the settings region targets removed markup; layering audit: header 20
  < modals 25 < terminal overlay 30 < detail panel 40.
- Remaining for the user: restart the long-running `lessmess serve` (the :9090
  process still runs the pre-change binary; embedded assets require the
  rebuild + restart) and do the visual pass — sticky header on scroll, strip
  pinned under it, segmented Personal/Project toggle, sidebar navigation, both
  themes, narrow viewport.
