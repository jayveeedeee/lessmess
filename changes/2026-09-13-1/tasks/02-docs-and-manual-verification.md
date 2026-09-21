
# TUI-02: Docs and manual verification

Status: see [../ledger.md](../ledger.md).

## Objective

Document the managed embedded-chrome behavior in the README and verify the
feature end-to-end with the real binary, browser, and opencode service.

## Dependencies

- TUI-01 (feature fully wired).

## Scope

- `README.md`: short addition to the opencode-integration / embedded-terminal
  section describing that embedded sessions run with a lessmess-managed CLI
  config (tab strip and sidebar hidden), where the generated file lives, that
  the user's own `cli.json` is merged in and never modified, and the fail-open
  fallback.
- Manual verification evidence recorded in this task's Notes.

## Implementation steps

1. Update `README.md` (embedded terminal bullet(s)); keep it brief and factual.
2. Rebuild: `CGO_ENABLED=0 go build -o lessmess ./cmd/lessmess`.
3. Run `go vet ./... && go test ./...`.
4. Restart the local server, open a change session terminal in the browser:
   - no tab strip at the top, no right sidebar;
   - theme matches the user's own `cli.json` (merge works);
   - `cat ~/.config/opencode/cli.json` unchanged;
   - `.lessmess/xdg/opencode/cli.json` exists with the two forced keys plus
     the user's other settings.
5. Sanity-check agent behavior in the embedded session (e.g. a permission
   pre-approval from the repo `opencode.json` still applies) to confirm the
   XDG redirect did not break configuration.
6. Also open a standalone `opencode2` in a regular terminal to confirm it is
   unaffected (tabs/sidebar as configured by the user).

## Verification

- All acceptance criteria in [../plan.md](../plan.md) confirmed, with notes
  (what was observed) added below.
- `lessmess validate` clean.

## Completion criteria

- README updated; manual checks 4–6 observed and recorded; full test suite
  green.

## Files affected

- `README.md`

## Notes

- Manual verification evidence goes here during execution.
- 2026-09-13: rebuilt (`CGO_ENABLED=0 go build -o lessmess ./cmd/lessmess`),
  restarted the :9090 server with identical flags (PID 69276 → 73244),
  index returns HTTP 200. Live probe (temporary WS client under
  `.lessmess/`, removed after use) opened `/terminal/ws?session=ses_liveprobe`:
  the real opencode2 TUI spawned and `.lessmess/xdg/opencode/cli.json` was
  generated — user's `theme.name=aura`, `animations`, `diffs.wrap`,
  `session.scrollbar`/`thinking`, and the tekton TUI plugin all preserved;
  `tabs.enabled=false` and `session.sidebar="hide"` applied; `$schema` added.
  No `tui config` warnings in the server log.
- 2026-09-13: user completed the browser pass — no tab strip, no sidebar,
  theme intact, standalone TUI unaffected. README bullet "Chrome-free
  embedded TUI" added under opencode integration. Full `go vet`/`go test`
  green and `lessmess validate` clean. Accepted by user → Done.
