# internal/model/testdata

<!-- tasktracker:begin -->
## Notes for agents

- This directory holds static test fixtures for the `internal/model` parsers and serializers, not live workflow data.
- Fixtures mirror the real formats: `root_ledger.md`, `change_ledger.md`, `task_file.md`, `task_ledger.md` (the container task-ledger fixture with minimal `- Task:`/`- Last updated:` headers and dotted child IDs like `EXC-00.00`), and the generated docs.
- `doc_structure.md` and `doc_agents_merged.md` are golden outputs of `RenderDocFile`/`MergeDoc`, while `doc_agents_human.md` has no markers and proves merge appends after human content; the docfile tests assert these bytes exactly.
- Keep fixtures byte-stable; parser and serializer tests assert exact content and formatting.
- Do not regenerate or "fix" fixture contents to match current output unless the test change explicitly calls for it.
- The `.md` files are copies of the repository's workflow formats, so edits belong with the corresponding parser/serializer tests.
<!-- tasktracker:end -->
