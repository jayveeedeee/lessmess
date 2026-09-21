
# TST-02: Test pill styling in both themes

Status: see [../ledger.md](../ledger.md).

## Objective

Give the `Test` status a distinct, legible pill color in dark and light
themes so the new column reads clearly on the board.

## Dependencies

- TST-00 (the status must exist in `TaskStatusOrder` for the column and pill
  to render)

## Scope

- `web/static/app.css` only: two `--st-test-*` variable pairs and one
  `.status-test` rule. The `statusClass` template helper already maps
  `Test` → `status-test`; no template or JS changes.

## Implementation steps

1. In the dark `:root` block (alongside `--st-not-started-*` …
   `--st-cancelled-*`), add `--st-test-bg` / `--st-test-fg` in a blue/teal
   tone distinct from the existing grey, orange, red, olive, and muted pills
   (e.g. bg ≈ `rgba(70, 130, 180, 0.16)`, fg ≈ `#6aa8d8`; tune to palette).
2. In the light-theme block, add the matching pair (e.g. bg ≈
   `rgba(40, 110, 170, 0.12)`, fg ≈ `#2f6ba3`).
3. Add `.status-test { background: var(--st-test-bg); color: var(--st-test-fg); }`
   next to the other `.status-*` rules, keeping the file's alignment style.

## Verification

1. Rebuild the binary, open any board, and move a card to `Test` (or hand-edit
   a ledger row): the pill renders with the new colors.
2. Toggle both themes: the pill is legible and visually distinct from
   `In progress` (orange) and `Blocked` (red) in each.

## Completion criteria

- `Test` pill styled in both themes with theme-variable-driven colors; no
  other CSS touched.

## Files affected

- `web/static/app.css`

## Notes

- Render verification happened in TST-03's smoke run: the board HTML carried
  the `Test` column (`data-status="Test"`) and `status-test` pills. Theme
  legibility eyeballed in a real browser is left to user acceptance.
