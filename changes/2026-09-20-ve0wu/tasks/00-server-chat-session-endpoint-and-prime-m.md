
# CHAT-00: Server: chat session endpoint and prime module

Status: see [../ledger.md](../ledger.md).

## Objective

Add `POST /chat/session`: spawn a repo-root opencode session primed as a free agent for
general codebase chat, mapped to the unassigned bucket — the backend behind the header
Chat button.

## Dependencies

- None (first task of the change).

## Scope

- `internal/server/instructions.json` — one new module, audience `chat`.
- `internal/server/chat.go` (new) — handler + prompt composition.
- `internal/server/server.go` — route registration.
- Handler and prime tests in `internal/server`.

## Implementation steps

1. `instructions.json`: add a module with `"audiences":["chat"]` (no other module lists
   `chat`, so `renderPrime("chat", …)` composes exactly this one). Text: free-agent
   stance — you are a general-purpose assistant working in the repository root; discuss
   the codebase, read anything, answer questions; make code changes when the user
   explicitly asks; no change-workflow obligations, no scaffold trigger. Keep the
   `{{placeholders}}` convention only if a substitution is actually needed — the chat
   prime needs no state snapshot (empty `primeContext` must render clean; add a render
   test pinning that, following the discussion step-0 pin).
2. `chat.go`: `chatSession` handler with the house guard order — `s.oc == nil` → 503
   "opencode service unavailable"; `s.mapErr != nil` → 503 "session mapping unreadable".
   Then: spawn via `s.spawnSession(ctx, title)` (15s timeout, matching the other
   spawners), title `"Codebase chat " + time.Now().Format("15:04")` so repeat chats are
   distinguishable in Discussions; prime with `s.promptWith(renderPrime("chat", …))` —
   on prime failure delete the session and return 502 "prime chat session: …" so nothing
   unbound leaks; `s.sessions.addUnassigned(entry)` (delete + 500 on persist failure);
   log and return 201 `sessionResponse{Session, Title, Created, Live: true}`.
   Call `logPrime(sess.ID, "chat", modules)` for the audit line, mirroring the spawn
   path in `spawnChange`.
3. `server.go`: register `mux.HandleFunc("POST /chat/session", s.chatSession)` beside
   the other session spawns.
4. Tests (fake opencode client, fixture store — follow `changesession_test.go` /
   `explorer_test.go`): 201 + body shape + unassigned mapping contains the session; both
   503 guards; prime failure deletes the session and returns 502; `renderPrime("chat")`
   contains the chat module text and none of the discussion/change module markers.

## Verification

- `go test ./internal/server` covers the matrix above.
- Scratch server: `curl -X POST :8080/chat/session` → 201 with `ses_…`; session appears
  in the index Discussions list and the server log shows `session primed … audience=chat`
  with only the chat module id.

## Completion criteria

A single POST creates a live, correctly primed chat session in the unassigned bucket;
every failure path leaves no orphan session and returns the house error shape.

## Files affected

`internal/server/instructions.json`, `internal/server/chat.go`, `internal/server/server.go`,
`internal/server/chat_test.go` (new), possibly `instructions_test.go`.
