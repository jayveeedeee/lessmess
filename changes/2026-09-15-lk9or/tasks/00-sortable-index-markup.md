---
id: IDX-00
title: Sortable index table markup and server-emitted sort keys
---

# IDX-00: Sortable index table markup and server-emitted sort keys

Status: see [../ledger.md](../ledger.md).

## Objective

Make the change index table's markup sort-ready: clickable header buttons carrying a sort-column identity, and per-row `data-*` attributes exposing machine-readable sort keys so client JS never scrapes display text.

## Dependencies

- None (first task of the change).

## Scope

- `web/templates/index.html`: the six data `<th>` cells become sort buttons (`data-sort-col`, direction-indicator span); body rows gain `data-tasks`, `data-status-rank`, and `data-updated`.
- `internal/server/render.go`: new `statusRank` template func (index in `model.TaskStatusOrder`; large fallback rank for unknown statuses).
- `internal/server/render_test.go`: assertions for the new attributes and header buttons.

## Implementation steps

1. In `render.go`, add `"statusRank": func(s string) int` to `templateFuncs`, returning the position of `s` in `model.TaskStatusOrder` and e.g. `len(model.TaskStatusOrder)` when absent.
2. In `index.html`, replace each data `<th>` with `<th data-col="…"><button type="button" class="sort-btn" data-sort-col="…">Label<span class="sort-ind"></span></button></th>` (or the minimal equivalent consistent with existing styles). Keep the trailing Plan `<th>` a plain static header.
3. Add to each body `<tr>`: `data-tasks="{{.Tasks}}"`, `data-status-rank="{{.Status | statusRank}}"`, `data-updated="{{.Updated}}"`.
4. Extend `TestIndexHTML` (or add a focused test) asserting: six `data-sort-col` buttons, `data-status-rank` present on rows and matching workflow order for known statuses, `data-updated`/`data-tasks` present.

## Verification

- `go vet ./... && go test ./...` passes, including the extended render test.
- Rebuild (`CGO_ENABLED=0 go build -o lessmess ./cmd/lessmess`), restart, and confirm the page renders unchanged visually except inert sort buttons (no JS behavior yet).

## Completion criteria

- All markup and attributes from the steps exist and are covered by tests; server sort order is untouched (`TestIndexNewestFirst` still passes).

## Files affected

- `web/templates/index.html`
- `internal/server/render.go`
- `internal/server/render_test.go`

## Notes

- Templates are embedded at build time; a running server must be rebuilt to show changes (repo learning 2026-09-13-0/manual).
- Keep `changeSummary` and the JSON API untouched by design.
