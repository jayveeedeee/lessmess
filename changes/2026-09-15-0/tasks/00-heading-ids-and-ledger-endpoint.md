
# MTOC-00: Heading anchors and ledger detail endpoint

Status: see [../ledger.md](../ledger.md).

## Objective

Give rendered markdown stable heading ids (the TOC's anchors) and make the
change ledger viewable in the modal via a new `GET /changes/{id}/ledger`
endpoint, removing the server-side gap behind the 404 complaint.

## Dependencies

None — first task of the change.

## Scope

- Goldmark parser options in `internal/server/render.go`.
- `store.LedgerFile` in `internal/store/store.go`.
- `ledgerDetail` handler + route in `internal/server/server.go`.
- `ledgerDetail` template partial in `web/templates/partials.html` and the
  `ledgerView` data type.
- Tests in `internal/server/render_test.go` and `internal/server/server_test.go`.

## Implementation steps

1. In `render.go`, create the goldmark instance with
   `goldmark.WithParserOptions(parser.WithAutoHeadingID())` (new `parser`
   import from the already-vendored goldmark module).
2. In `store.go`, add `LedgerFile(changeID) (string, error)` mirroring
   `PlanFile` (reads `ledger.md` from the change dir, `ErrNotFound` on miss).
3. In `server.go`, add route `GET /changes/{id}/ledger` and a handler shaped
   like `planDetail`: HTML/htmx renders the `ledgerDetail` partial with the
   body passed through `dropLeadingH1`; otherwise JSON `{"id", "body"}`.
4. In `partials.html`, add a `ledgerDetail` define (chip = change id, title
   "Ledger", markdown body) — initially in the current single-column shape;
   MTOC-01 restructures all three partials together.
5. Add `ledgerView` beside `planView` in `render.go`.

## Verification

- `go test ./internal/server/ -run TestLedgerDetail` (new): HTML request
  contains ledger content and the modal chrome; JSON request returns the
  body; unknown change id 404s.
- New assertion that markdown headings render with `id` attributes.
- `go vet ./... && go test ./...` green.

## Completion criteria

- Endpoint live, partial defined, heading ids present in rendered markdown,
  tests green.

## Files affected

- `internal/server/render.go`, `internal/server/server.go`,
  `internal/store/store.go`, `web/templates/partials.html`,
  `internal/server/render_test.go`, `internal/server/server_test.go`.

## Notes

- goldmark's `goldmark.New()` with no options does not emit heading ids
  (verified by running a conversion snippet during planning) — hence the
  explicit parser option.
- `TaskFile`'s path-traversal guard pattern is the reference if `LedgerFile`
  ever takes a file argument; here the filename is fixed.
- Verification evidence (2026-09-15): `TestLedgerDetail` (JSON + 404),
  `TestLedgerDetailHTML` (modal chrome, rendered table, dropped H1), and
  `TestMarkdownHeadingIDs` all pass; `go vet ./...` clean;
  `go test ./internal/server/ ./internal/store/` green.
