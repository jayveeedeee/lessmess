
# SET-08: Per-setting Change button with session reuse

Status: see [../ledger.md](../ledger.md).

## Objective

Every setting field gets a Change button that opens a change terminal
running the normal discussion-first flow, primed with exactly which
setting was clicked. While the settings-initiated change is open, further
clicks reuse the same session; once the change is explicitly closed, the
next click starts a fresh discussion.

## Dependencies

- SET-04 (settings page), SET-06 (grouped page)

## Scope

- `POST /api/settings/change` (`settingschange.go`): body
  `{field, scope}`. The server computes the field's effective value and
  source itself (never trusts the client for them).
  - Reuse: state file `.lessmess/settings-change.json` holds the active
    settings-discussion session id. It is reusable when the session still
    exists AND (it is still an unassigned discussion OR its bound change's
    overall status is not Done/Cancelled). Reuse = `oc.Prompt` a
    "another setting" context message; response `reused:true`.
  - Fresh: `spawnSession` (agent/model defaults apply) titled
    `settings: <field>`, primed with the standard `discussionPrompt` plus
    the discussion addendum plus a context block (setting path, group,
    effective value, source, page scope); mapped to the unassigned
    Discussions bucket; state file written.
  - Validation: unknown field → 422; no opencode service → 503; corrupt
    state file → treated as absent.
- `settings.html`: every field label gains
  `<button class="settings-change">✦ change</button>` (same pill idiom as
  the explorer's chat button).
- `app.js`: one delegated click handler posts the field + current scope
  and opens the terminal with the returned session (deliberate click —
  the auto-open gate does not apply).
- `app.css`: `.settings-change` pill, right-aligned in the label row.
- Tests for fresh/reuse/closed-change-fresh/unknown-field/no-service.

## Implementation steps

1. State helpers + `settingsChangeContext` prompt builder (pure,
   table-tested) + handler + route.
2. Template buttons (one edit via the shared badge span) + delegated JS +
   CSS.
3. Handler tests; full suite; `node --check`.
4. Rebuild + restart; live-verify fresh → reuse → closed → fresh.

## Verification

- Tests: fresh click primes a discussion with the field context; second
  click reuses the same session id with a fold-in message; a closed bound
  change or a dead session triggers a fresh discussion; unknown field 422.
- Live: two clicks share one session; closing the change resets.

## Completion criteria

- One active settings-change discussion at a time; explicit close is the
  only thing that ends reuse; the agent always knows exactly which setting
  was clicked, its value, source, and scope.

## Files affected

- `internal/server/settingschange.go` (new), `settingschange_test.go` (new)
- `internal/server/server.go` (route)
- `web/templates/settings.html`, `web/static/app.js`, `web/static/app.css`
- `.lessmess/settings-change.json` (runtime state, gitignored)

## Notes

- Reuse identity is the session id; the session→change binding comes from
  the existing mapping (`changeOf`), and the change's overall status from
  the store — no extra bookkeeping beyond the one-file state.
- The reuse prompt keeps the discussion addendum semantics: only the
  fresh discussion prime includes the base prompt.
- Verified 2026-09-13: handler tests (fresh, reuse, closed-change-fresh,
  dead-session-fresh, 422/400/503 rejects) plus live check on :9090 —
  first POST created `settings: session.model` (`reused:false`), second
  POST for `ui.showArchived` returned the same session with `reused:true`;
  state file written and cleaned up after the smoke. Full suite green,
  `lessmess validate` exit 0.
