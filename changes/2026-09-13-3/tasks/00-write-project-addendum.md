
# DQP-00: Write discussion question policy to project settings

Status: see [../ledger.md](../ledger.md).

## Objective

Set `prompts.discussion` at the project settings layer to the authoritative addendum text from [../plan.md](../plan.md) so every newly created discussion session is primed with the question policy: no opening questions, stated assumptions instead of avoidable questions, multiple-choice-only when a question is unavoidable.

## Dependencies

- None. The text was agreed in discussion and the settings API is available on the running server.

## Scope

- One `PUT /api/settings?scope=project` with the `prompts.discussion` value (patch semantics preserve the rest of the layer).
- Read-back verification via `GET /api/settings` and the `/settings` page.
- Ledger updates for this change only.

## Implementation steps

1. Mark DQP-00 `In progress` in [../ledger.md](../ledger.md) and set the overall change status to `In progress`.
2. PUT the value, JSON-escaping newlines as `\n` and quotes as `\"`:

   ```sh
   curl -s -X PUT 'http://127.0.0.1:9090/api/settings?scope=project' \
     -H 'Content-Type: application/json' \
     -d '{"prompts":{"discussion":"<escaped authoritative text>"}}'
   ```

3. Read back and compare byte-for-byte against the plan's authoritative block:

   ```sh
   curl -s 'http://127.0.0.1:9090/api/settings'
   ```

   Expect `prompts.discussion` to equal the text and `sources["prompts.discussion"]` to be `project`.
4. Confirm root `lessmess.json` gained the `prompts.discussion` key with the same text and that all other existing sections are unchanged.
5. Run `go vet ./... && go test ./...` as a sanity gate (no code changed; existing `settingswiring_test.go` already covers addendum application).
6. Update [../ledger.md](../ledger.md): task to `Test` with verification evidence; overall status stays `In progress` pending user acceptance.

## Verification

- `GET /api/settings`: `prompts.discussion` byte-identical to the plan block; `sources["prompts.discussion"] == "project"`.
- Root `lessmess.json` contains the key; no unrelated content changed.
- `go vet ./... && go test ./...` passes.
- Optional live acceptance: the next newly created discussion session opens with no questions and asks only multiple-choice questions.

## Completion criteria

- Value present at the project layer, byte-identical to the plan text, with effective source `project`.
- Read-back and Settings-page render confirmed.
- Ledger updated with evidence; no unrelated settings modified.

## Files affected

- `lessmess.json` (root, committed — written by the server).
- `changes/2026-09-13-3/ledger.md` (bookkeeping).

## Notes

- Rollback: `PUT /api/settings?scope=project` with `{"prompts":{"discussion":""}}` restores the built-in default (tri-state clearing).
- The addendum affects newly created discussion sessions only; existing sessions keep their original prime.
- Server base URL is `http://127.0.0.1:9090` (same as the scaffold endpoint).
