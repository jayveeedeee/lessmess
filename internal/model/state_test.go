package model

import (
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
)

func sampleIndex() *WorkflowIndex {
	return &WorkflowIndex{
		Version: StateVersion,
		Changes: []IndexEntry{
			{ID: "2026-09-17-d2x3h", Title: "Old change", Prefix: "OLD", Created: "2026-09-17"},
			{ID: "2026-09-18-15tbl", Title: "JSON workflow state and instruction injection", Prefix: "JSI", Branch: "main", Created: "2026-09-18"},
			{ID: "2026-09-10-0", Title: "Archived legacy", Created: "2026-09-10", Archived: true},
		},
	}
}

func sampleChangeState() *ChangeState {
	return &ChangeState{
		Version: StateVersion,
		ID:      "2026-09-18-15tbl",
		Title:   "JSON workflow state and instruction injection",
		Prefix:  "JSI",
		Status:  ChangeStatus{Value: OverallInProgress, Derived: true},
		Created: "2026-09-18",
		Updated: "2026-09-18",
		Decision: []DecisionEntry{
			{Date: "2026-09-18", Decision: "Committed .lessmess/workflow/ subtree"},
		},
		Tasks: []TaskState{
			{ID: "JSI-00", Seq: 0, Title: "JSON state schema", File: "tasks/00-schema.md", Status: StatusNotStarted, Updated: "2026-09-18"},
			{ID: "JSI-01", Seq: 1, Title: "Migration", File: "tasks/01-migration.md", Status: StatusNotStarted, DependsOn: []string{"JSI-00"}, Updated: "2026-09-18", Notes: "round-trip first"},
			{ID: "JSI-00.00", Seq: 0, Parent: "JSI-00", Title: "Types", File: "tasks/00-schema/tasks/00-types.md", Status: StatusNotStarted, Updated: "2026-09-18"},
			{ID: "JSI-00.01", Seq: 1, Parent: "JSI-00", Title: "Tests", File: "tasks/00-schema/tasks/01-tests.md", Status: StatusNotStarted, DependsOn: []string{"JSI-00.00"}, Updated: "2026-09-18"},
		},
	}
}

func TestIndexRoundTrip(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "index.json")
	idx := sampleIndex()
	if err := idx.Save(path); err != nil {
		t.Fatalf("save: %v", err)
	}
	got, err := LoadWorkflowIndex(path)
	if err != nil {
		t.Fatalf("load: %v", err)
	}
	if len(got.Changes) != len(idx.Changes) {
		t.Fatalf("changes: %d, want %d", len(got.Changes), len(idx.Changes))
	}
	for i := range idx.Changes {
		if got.Changes[i] != idx.Changes[i] {
			t.Errorf("entry %d: %+v, want %+v", i, got.Changes[i], idx.Changes[i])
		}
	}
	// Re-saving the loaded value must be byte-stable.
	first, _ := os.ReadFile(path)
	if err := got.Save(path); err != nil {
		t.Fatalf("resave: %v", err)
	}
	second, _ := os.ReadFile(path)
	if string(first) != string(second) {
		t.Errorf("save not byte-stable:\n%s\n---\n%s", first, second)
	}
}

func TestChangeStateRoundTrip(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "2026-09-18-15tbl.json")
	st := sampleChangeState()
	if err := st.Save(path); err != nil {
		t.Fatalf("save: %v", err)
	}
	got, err := LoadChangeState(path)
	if err != nil {
		t.Fatalf("load: %v", err)
	}
	if got.ID != st.ID || got.Status != st.Status || len(got.Tasks) != len(st.Tasks) {
		t.Errorf("loaded state drift: %+v", got)
	}
	for i := range st.Tasks {
		if !reflect.DeepEqual(got.Tasks[i], st.Tasks[i]) {
			t.Errorf("task %d: %+v, want %+v", i, got.Tasks[i], st.Tasks[i])
		}
	}
}

func TestStateJSONShape(t *testing.T) {
	// Pins the serialized shape: field names, nesting, and empty-value
	// omission. Byte-stable output is what makes committed JSON diffs
	// reviewable.
	st := &ChangeState{
		Version: StateVersion,
		ID:      "2026-09-18-15tbl",
		Title:   "T",
		Status:  ChangeStatus{Value: OverallPlanned, Derived: true},
		Created: "2026-09-18",
		Updated: "2026-09-18",
		Tasks: []TaskState{
			{ID: "JSI-00", Seq: 0, Title: "First", File: "tasks/00-first.md", Status: StatusNotStarted, Updated: "2026-09-18"},
		},
	}
	dir := t.TempDir()
	path := filepath.Join(dir, "shape.json")
	if err := st.Save(path); err != nil {
		t.Fatalf("save: %v", err)
	}
	raw, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	out := strings.TrimSuffix(string(raw), "\n")
	want := `{
  "version": 1,
  "id": "2026-09-18-15tbl",
  "title": "T",
  "status": {
    "value": "Planned",
    "derived": true
  },
  "created": "2026-09-18",
  "updated": "2026-09-18",
  "tasks": [
    {
      "id": "JSI-00",
      "seq": 0,
      "title": "First",
      "file": "tasks/00-first.md",
      "status": "Not started",
      "updated": "2026-09-18"
    }
  ]
}`
	if out != want {
		t.Errorf("JSON shape drift:\n%s\n--- want ---\n%s", out, want)
	}
}

func TestLoadStateStrictness(t *testing.T) {
	dir := t.TempDir()

	cases := []struct {
		name string
		json string
		want string
	}{
		{"unknown field", `{"version":1,"id":"x","title":"T","status":{"value":"Planned","derived":true},"created":"2026-09-18","updated":"2026-09-18","tasks":[],"bogus":1}`, "unknown field"},
		{"bad version", `{"version":2,"id":"x","title":"T","status":{"value":"Planned","derived":true},"created":"2026-09-18","updated":"2026-09-18","tasks":[]}`, "unsupported state version"},
		{"trailing data", `{"version":1,"id":"x"} {}`, "trailing"},
		{"broken json", `{"version":1,`, "invalid JSON"},
	}
	for _, tc := range cases {
		path := filepath.Join(dir, "state.json")
		if err := os.WriteFile(path, []byte(tc.json), 0o644); err != nil {
			t.Fatal(err)
		}
		_, err := LoadChangeState(path)
		if err == nil {
			t.Errorf("%s: expected error", tc.name)
			continue
		}
		if !strings.Contains(err.Error(), tc.want) {
			t.Errorf("%s: error %q, want substring %q", tc.name, err, tc.want)
		}
	}

	if _, err := LoadChangeState(filepath.Join(dir, "missing.json")); !os.IsNotExist(err) {
		t.Errorf("missing file: err = %v, want IsNotExist", err)
	}
}

func TestValidateChangeState(t *testing.T) {
	if issues := sampleChangeState().Validate(); len(issues) != 0 {
		t.Errorf("clean sample has issues: %v", issues)
	}

	mut := func(f func(*ChangeState)) *ChangeState {
		st := sampleChangeState()
		f(st)
		return st
	}
	cases := []struct {
		name string
		st   *ChangeState
		want string
	}{
		{"bad overall status", mut(func(s *ChangeState) { s.Status.Value = "Later" }), `invalid overall status "Later"`},
		{"bad created date", mut(func(s *ChangeState) { s.Created = "2026-9-18" }), `invalid created date`},
		{"bad task status", mut(func(s *ChangeState) { s.Tasks[0].Status = "Wip" }), `invalid status "Wip"`},
		{"duplicate task id", mut(func(s *ChangeState) { s.Tasks[1].ID = "JSI-00" }), "duplicate task id JSI-00"},
		{"seq/id mismatch", mut(func(s *ChangeState) { s.Tasks[0].Seq = 7 }), `id segment "00" does not match seq 07`},
		{"seq/file mismatch", mut(func(s *ChangeState) { s.Tasks[0].File = "tasks/09-schema.md" }), `does not start with seq 00`},
		{"file not md", mut(func(s *ChangeState) { s.Tasks[0].File = "tasks/00-schema.txt" }), `must be an .md path`},
		{"bad dotted segment", mut(func(s *ChangeState) {
			s.Tasks[2].ID = "JSI-00.1"
			s.Tasks[2].File = "tasks/00-schema/tasks/01-types.md"
			s.Tasks[2].Parent = "JSI-00"
		}), "two-digit"},
		{"parent mismatch", mut(func(s *ChangeState) { s.Tasks[2].Parent = "JSI-01" }), `parent "JSI-01" does not match id`},
		{"unknown parent", mut(func(s *ChangeState) {
			s.Tasks[2].Parent = "NOPE-99"
			s.Tasks[2].ID = "NOPE-99.00"
			s.Tasks[2].File = "tasks/00-schema/tasks/00-types.md"
		}), `unknown parent "NOPE-99"`},
		{"unknown dep", mut(func(s *ChangeState) { s.Tasks[0].DependsOn = []string{"GONE-00"} }), `unknown dependency "GONE-00"`},
		{"self dep", mut(func(s *ChangeState) { s.Tasks[0].DependsOn = []string{"JSI-00"} }), "depends on itself"},
	}
	for _, tc := range cases {
		issues := tc.st.Validate()
		found := false
		for _, msg := range issues {
			if strings.Contains(msg, tc.want) {
				found = true
			}
		}
		if !found {
			t.Errorf("%s: issues %v, want one containing %q", tc.name, issues, tc.want)
		}
	}
}

func TestValidateDependencyCycle(t *testing.T) {
	st := sampleChangeState()
	st.Tasks[1].DependsOn = []string{"JSI-00.01"} // JSI-01 -> JSI-00.01 -> JSI-00.00 (clean) — make a real cycle:
	st.Tasks[3].DependsOn = []string{"JSI-01"}    // JSI-00.01 -> JSI-01 -> JSI-00.01
	issues := st.Validate()
	found := false
	for _, msg := range issues {
		if strings.HasPrefix(msg, "dependency cycle:") {
			found = true
		}
	}
	if !found {
		t.Errorf("issues %v, want a dependency cycle", issues)
	}
}

func TestValidateIndex(t *testing.T) {
	if issues := sampleIndex().Validate(); len(issues) != 0 {
		t.Errorf("clean sample has issues: %v", issues)
	}
	idx := sampleIndex()
	idx.Changes = append(idx.Changes, IndexEntry{ID: "2026-09-18-15tbl", Title: "Dup", Created: "2026-09-18"})
	idx.Changes = append(idx.Changes, IndexEntry{ID: "", Title: "No id"})
	idx.Changes = append(idx.Changes, IndexEntry{ID: "2026-09-18-aaaaa", Title: "Bad date", Created: "soon"})
	issues := idx.Validate()
	for _, want := range []string{"duplicate change id 2026-09-18-15tbl", "empty id", `invalid created date "soon"`} {
		found := false
		for _, msg := range issues {
			if strings.Contains(msg, want) {
				found = true
			}
		}
		if !found {
			t.Errorf("issues %v, want one containing %q", issues, want)
		}
	}
}

func TestTaskLookupAndChildren(t *testing.T) {
	st := sampleChangeState()
	if got := st.Task("JSI-00.01"); got == nil || got.Title != "Tests" {
		t.Errorf("Task(JST-00.01) = %+v, want Tests", got)
	}
	if got := st.Task("NOPE"); got != nil {
		t.Errorf("Task(NOPE) = %+v, want nil", got)
	}
	top := st.Children("")
	if len(top) != 2 || top[0].ID != "JSI-00" || top[1].ID != "JSI-01" {
		t.Errorf("Children(\"\") = %+v, want JSI-00 then JSI-01", top)
	}
	kids := st.Children("JSI-00")
	if len(kids) != 2 || kids[0].ID != "JSI-00.00" || kids[1].ID != "JSI-00.01" {
		t.Errorf("Children(JSI-00) = %+v", kids)
	}
	if idx := sampleIndex(); idx.Find("2026-09-10-0") == nil || idx.Find("nope") != nil {
		t.Error("WorkflowIndex.Find broken")
	}
}
