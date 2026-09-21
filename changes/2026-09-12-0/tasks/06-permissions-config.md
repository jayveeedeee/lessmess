
# OCI-06: Permissions config and docs

Status: see [../ledger.md](../ledger.md).

## Objective

Pre-approve agent permissions for change sessions via a committed `opencode.json`, per the user's decision, and document the security posture.

## Dependencies

None (independent; needed before OCI-07 dogfood).

## Scope

In scope: `opencode.json` at the repo root with the exact V2 `permissions` schema (fetched from https://opencode.ai/v2/docs/config — no guessed fields), scoped to allow editing/running inside this project; README section on the integration and its security implications; `--auto` fallback note.
Out of scope: global/user-level opencode config.

## Implementation steps

1. Fetch the V2 config/permissions docs; extract the exact schema.
2. Write minimal `opencode.json` (with `$schema`) pre-approving repo-local edits/shell for change sessions.
3. Verify live: run a trivial session action that would otherwise prompt and confirm no prompt blocks it.
4. README: integration usage + security note (localhost-only, auto-approval scoped to this repo).

## Verification

- Live verification shows no approval prompt blocking an edit; docs updated.

## Completion criteria

- `opencode.json` committed and effective; README covers usage + security.

## Files affected

- `opencode.json` (new), `README.md`

## Notes

- V2 permissions schema (from https://opencode.ai/v2/docs/permissions): ordered rules `{action, resource, effect}`; last match wins; base policy allows everything except `external_directory` and `.env` reads (`ask`).
- `opencode.json` (committed): `*`/`allow` inside the project, `external_directory`/`deny`, `.env`/`deny` (with `.env.example` allow), `git push *`/deny.
- Live verification (2026-09-12): `opencode2 run` (NO `--auto`) executed a shell write + `ls` + delete inside the repo and finished in ~5s with no approval prompt. Notably, the same kind of turn hung indefinitely before the config existed — the config is what unblocks unattended sessions.
- README documents the integration and the security posture (auto-approval scoped to this repo, localhost-only, password server-side only).

