# CHAT-04: Quiet boot-time opencode.json skills patch

## Why

`EnsureSkillsCatalog` (`internal/server/skillsconfig.go`, called from the
boot closure in `cmd/lessmess/main.go`) rewrites the repo's
`opencode.json` whenever the computed skills URL (`http://<PublicBase>
/skills/`) differs from the stored one. The OpenCode service watches
that file: every write fires `config.updated` and triggers a **full
location re-initialization** — watchers torn down and re-subscribed,
providers/models/agents/commands re-read, plugins reloaded, skills
re-synced — which degrades or stalls every running session in the
location. Forensics from the 2026-09-29 outage: a `config.updated`
cascade at 19:54:38Z (seconds after a decompose click, with the skills
cache hash flipping to a newer catalog) coincided with "all sessions
could no longer talk to the backend"; and the 19:56:12Z restart of
lessmess itself wrote `opencode.json` (mtime + event second-aligned),
firing another cascade. Restarts with different host/port flags or URL
drift therefore reliably detonate the service-wide re-init.

## What

- Before replacing a differing lessmess-managed entry (any URL ending
  in `/skills/`), **probe the existing URL**: a short-timeout
  (≈2–3s, best-effort) GET of its `index.json`.
- If the old URL still serves a live catalog, **skip the rewrite** and
  log — a stale-but-live entry beats a service-wide re-init. The next
  boot after that server is gone heals the entry (probe fails → patch
  as today).
- If the probe fails or the response is not a catalog, patch exactly as
  today (healing stays the safe direction).
- Unchanged behavior: identical URL ⇒ byte-identical no-op; absent
  `opencode.json` ⇒ created; foreign (non-`/skills/`) entries always
  preserved.
- Accepted trade-off (record in plan.md): the old URL may serve an
  older build's skill bodies until it dies — content staleness is
  cheaper than stalling every session in the location.

## Files affected

- `internal/server/skillsconfig.go`
- `internal/server/skillsconfig_test.go`

## Verification

- Tests with an httptest server standing in for the stale-but-live
  catalog URL:
  1. live old URL + different computed URL ⇒ **no write**, file bytes
     unchanged, skip logged;
  2. dead old URL ⇒ patched, write happens;
  3. identical URL ⇒ no write (existing pin);
  4. absent file ⇒ created with the computed URL (existing pin);
  5. probe that returns garbage (non-catalog) ⇒ patched (heals).
- `go vet ./... && go test ./...` from the repo root.
