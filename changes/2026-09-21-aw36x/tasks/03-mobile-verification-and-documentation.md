# MAC-03: Mobile verification and documentation

## Objective

Prove the parallel Chat experience works against a real OpenCode service on
mobile-sized screens, document its operating boundaries, and leave the release
ready for follow-up API capabilities.

## Dependencies

- MAC-02

## Scope

- Full automated suite, static build, workflow validation, and live smoke test.
- Phone portrait checks for interaction, keyboard, scrolling, reconnect, and
  terminal regression.
- README usage and private-network/VPN safety guidance.
- Package learnings updated only for durable implementation facts.

## Implementation steps

1. Run focused package tests throughout, then `go vet ./...`, `go test ./...`,
   and `CGO_ENABLED=0 go build -o lessmess ./cmd/lessmess`.
2. Run `lessmess validate` and resolve change-document or workflow violations.
3. Rebuild/restart because web assets are embedded. With a throwaway OpenCode
   session, verify prompt, progress/tool rendering, interrupt, permission/form
   handling when available, close/reopen, and Chat/Terminal continuity.
4. Check representative 375px and 390px portrait viewports, including a long
   transcript, long code/tool output, scrolled-up refresh, and the task drawer.
5. Confirm the desktop terminal still accepts input, resizes, reconnects, and
   shows its task panel.
6. Update README with Chat versus Terminal, mobile access using a trusted LAN or
   VPN, the unchanged localhost default, and a warning not to expose an
   unauthenticated server publicly.
7. Record exact automated and manual evidence in the task notes/status through
   the workflow API before requesting user acceptance.

## Verification

- `go vet ./...`
- `go test ./...`
- `CGO_ENABLED=0 go build -o lessmess ./cmd/lessmess`
- `./lessmess validate`
- Real-service desktop and mobile-width smoke checks described above

## Completion criteria

All automated checks pass, the live mobile workflow meets the plan acceptance
criteria, terminal behavior is unchanged, documentation states the security
boundary clearly, and any deferred OpenCode features are listed rather than
silently implied to work.

## Files affected

- `README.md`
- Relevant package `AGENTS.md` files if durable learnings changed
- Tests and implementation files touched by MAC-00 through MAC-02

## Notes

The web UI is embedded in the binary. A rebuild and server restart are required
before manual results are meaningful.
