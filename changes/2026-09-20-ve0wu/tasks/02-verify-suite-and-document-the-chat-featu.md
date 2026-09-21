
# CHAT-02: Verify suite and document the chat feature

Status: see [../ledger.md](../ledger.md).

## Objective

Whole-repo verification and the user-facing README note for the Chat button and
`POST /chat/session`.

## Dependencies

- CHAT-00 and CHAT-01 (the feature being verified and documented).

## Scope

- Repo-root verification commands; `README.md`.

## Implementation steps

1. From the repo root: `go vet ./...` and `go test ./...` — both clean.
2. `lessmess validate` — the workflow contract for this change (plan + task files +
   ledger rows) is clean.
3. `README.md`: document the header Chat button (general codebase chat, new session per
   click, free agent that edits only when asked) and the `POST /chat/session` endpoint
   where the other session endpoints are described — README must stay current with
   user-visible endpoints per the repository convention.
4. Manual end-to-end for the evidence trail: rebuild the binary
   (`CGO_ENABLED=0 go build -o lessmess ./cmd/lessmess`) and restart — embedded
   templates/JS are invisible until rebuild — then click Chat, confirm the terminal
   opens a fresh "Codebase chat …" session, ask it a repo question, confirm the session
   lists under Discussions, and confirm a second click creates a second session.

## Verification

This task is verification: paste the command outcomes and the manual click-through
summary as completion evidence.

## Completion criteria

Suite green, workflow validation clean, README updated, and the button proven
end-to-end against a rebuilt binary.

## Files affected

`README.md`.
