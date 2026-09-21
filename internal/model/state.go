package model

import (
	"bytes"
	"encoding/json"
	"fmt"
	"os"
	"path"
	"sort"
	"strings"
	"time"
)

// The JSON workflow state store under .lessmess/workflow/: index.json (the
// change registry) and changes/<id>.json (one change's task tree and
// status). These types are the canonical workflow state; markdown ledgers
// are a legacy input consumed only by migration.

// StateVersion is the schema version of the JSON state files. Loaders
// reject other versions; changing the schema bumps this.
const StateVersion = 1

// WorkflowIndex is the parsed .lessmess/workflow/index.json. It is the
// registry of change directories and deliberately carries no status or
// update dates: those are authoritative only in each change's ChangeState,
// so the two can never disagree.
type WorkflowIndex struct {
	Version int          `json:"version"`
	Changes []IndexEntry `json:"changes"`
}

// IndexEntry is one change in the workflow index.
type IndexEntry struct {
	ID       string `json:"id"`
	Title    string `json:"title"`
	Prefix   string `json:"prefix,omitempty"`
	Branch   string `json:"branch,omitempty"` // "" when none (renders as Empty)
	Created  string `json:"created"`          // YYYY-MM-DD
	Archived bool   `json:"archived,omitempty"`
}

// ChangeState is the parsed .lessmess/workflow/changes/<id>.json: the
// authoritative state of one change.
type ChangeState struct {
	Version  int             `json:"version"`
	ID       string          `json:"id"`
	Title    string          `json:"title"`
	Prefix   string          `json:"prefix,omitempty"`
	Branch   string          `json:"branch,omitempty"`
	Status   ChangeStatus    `json:"status"`
	Created  string          `json:"created"` // YYYY-MM-DD
	Updated  string          `json:"updated"` // date of the last state write
	Decision []DecisionEntry `json:"decisionLog,omitempty"`
	Tasks    []TaskState     `json:"tasks"`
	Archived bool            `json:"archived,omitempty"`
}

// ChangeStatus is a change's overall status. Derived statuses (Planned,
// In progress, Blocked) are recomputed by the tool from the task tree on
// every task change; user-set statuses (Done, Cancelled) are stored with
// Derived false.
type ChangeStatus struct {
	Value   OverallStatus `json:"value"`
	Derived bool          `json:"derived"`
}

// DecisionEntry is one row of a change's decision log.
type DecisionEntry struct {
	Date     string `json:"date"`
	Decision string `json:"decision"`
}

// TaskState is one task of a change's tree. Tasks is a flat array; array
// order is priority order within each parent level (Parent "" is
// top-level), and nesting is expressed by Parent plus dotted IDs
// ("JSI-00" -> "JSI-00.00").
type TaskState struct {
	ID        string     `json:"id"`
	Seq       int        `json:"seq"` // 0-based; matches the file-name prefix
	Parent    string     `json:"parent,omitempty"`
	Title     string     `json:"title"`
	File      string     `json:"file"` // prose path relative to the change dir, forward slashes
	Status    TaskStatus `json:"status"`
	DependsOn []string   `json:"dependsOn,omitempty"`
	Updated   string     `json:"updated"` // YYYY-MM-DD
	Notes     string     `json:"notes,omitempty"`
}

// Find returns the index entry for id, or nil.
func (idx *WorkflowIndex) Find(id string) *IndexEntry {
	for i := range idx.Changes {
		if idx.Changes[i].ID == id {
			return &idx.Changes[i]
		}
	}
	return nil
}

// Task returns the task with the given dotted ID, or nil.
func (s *ChangeState) Task(id string) *TaskState {
	for i := range s.Tasks {
		if s.Tasks[i].ID == id {
			return &s.Tasks[i]
		}
	}
	return nil
}

// Children returns the tasks whose parent is parent ("" for top-level),
// in priority (array) order.
func (s *ChangeState) Children(parent string) []*TaskState {
	var out []*TaskState
	for i := range s.Tasks {
		if s.Tasks[i].Parent == parent {
			out = append(out, &s.Tasks[i])
		}
	}
	return out
}

// LoadWorkflowIndex reads and strictly parses an index.json.
func LoadWorkflowIndex(path string) (*WorkflowIndex, error) {
	var idx WorkflowIndex
	if err := decodeStateFile(path, &idx); err != nil {
		return nil, err
	}
	if idx.Version != StateVersion {
		return nil, parseErr(path, "unsupported state version %d (want %d)", idx.Version, StateVersion)
	}
	return &idx, nil
}

// Save atomically writes the index as 2-space-indented JSON with a
// trailing newline. The parent directory must exist.
func (idx *WorkflowIndex) Save(path string) error {
	return saveStateFile(path, idx)
}

// LoadChangeState reads and strictly parses one change's state file.
func LoadChangeState(path string) (*ChangeState, error) {
	var st ChangeState
	if err := decodeStateFile(path, &st); err != nil {
		return nil, err
	}
	if st.Version != StateVersion {
		return nil, parseErr(path, "unsupported state version %d (want %d)", st.Version, StateVersion)
	}
	return &st, nil
}

// Save atomically writes the change state. The parent directory must
// exist.
func (s *ChangeState) Save(path string) error {
	return saveStateFile(path, s)
}

func decodeStateFile(path string, v any) error {
	data, err := os.ReadFile(path)
	if err != nil {
		return err
	}
	dec := json.NewDecoder(bytes.NewReader(data))
	dec.DisallowUnknownFields()
	if err := dec.Decode(v); err != nil {
		return parseErr(path, "invalid JSON: %v", err)
	}
	if dec.More() {
		return parseErr(path, "unexpected trailing data after JSON value")
	}
	return nil
}

func saveStateFile(path string, v any) error {
	data, err := json.MarshalIndent(v, "", "  ")
	if err != nil {
		return fmt.Errorf("encode %s: %w", path, err)
	}
	data = append(data, '\n')
	return WriteFileAtomic(path, data, 0o644)
}

// Validate returns the workflow-rule issues of the index (empty when
// clean). Messages are sorted for deterministic output.
func (idx *WorkflowIndex) Validate() []string {
	var out []string
	if idx.Version != StateVersion {
		out = append(out, fmt.Sprintf("unsupported state version %d (want %d)", idx.Version, StateVersion))
	}
	seen := map[string]bool{}
	for _, e := range idx.Changes {
		if e.ID == "" {
			out = append(out, "index entry with empty id")
			continue
		}
		if seen[e.ID] {
			out = append(out, fmt.Sprintf("duplicate change id %s", e.ID))
		}
		seen[e.ID] = true
		if e.Title == "" {
			out = append(out, fmt.Sprintf("change %s: empty title", e.ID))
		}
		if !validDate(e.Created) {
			out = append(out, fmt.Sprintf("change %s: invalid created date %q", e.ID, e.Created))
		}
	}
	sort.Strings(out)
	return out
}

// Validate returns the workflow-rule issues of the change state (empty
// when clean). Messages are sorted for deterministic output. It checks
// everything that needs no filesystem access: vocabularies, unique and
// coherent task IDs, sequence/file agreement, parent/dependency
// references, and dependency cycles. File existence is the store's job.
func (s *ChangeState) Validate() []string {
	var out []string
	if s.Version != StateVersion {
		out = append(out, fmt.Sprintf("unsupported state version %d (want %d)", s.Version, StateVersion))
	}
	if s.ID == "" {
		out = append(out, "change state with empty id")
	}
	if s.Title == "" {
		out = append(out, "empty title")
	}
	if !s.Status.Value.Valid() {
		out = append(out, fmt.Sprintf("invalid overall status %q", s.Status.Value))
	}
	if !validDate(s.Created) {
		out = append(out, fmt.Sprintf("invalid created date %q", s.Created))
	}
	if !validDate(s.Updated) {
		out = append(out, fmt.Sprintf("invalid updated date %q", s.Updated))
	}

	ids := map[string]bool{}
	for i := range s.Tasks {
		t := &s.Tasks[i]
		label := t.ID
		if label == "" {
			label = fmt.Sprintf("task[%d]", i)
		}
		if t.ID == "" {
			out = append(out, fmt.Sprintf("%s: empty id", label))
		} else if ids[t.ID] {
			out = append(out, fmt.Sprintf("duplicate task id %s", t.ID))
		}
		ids[t.ID] = true
		if t.Title == "" {
			out = append(out, fmt.Sprintf("%s: empty title", label))
		}
		if !t.Status.Valid() {
			out = append(out, fmt.Sprintf("%s: invalid status %q", label, t.Status))
		}
		if !validDate(t.Updated) {
			out = append(out, fmt.Sprintf("%s: invalid updated date %q", label, t.Updated))
		}
		if !DottedSegmentsValid(t.ID) {
			out = append(out, fmt.Sprintf("%s: dotted segments must be two-digit", label))
		}
		// Sequence agreement: the ID's last segment and the prose file's
		// name prefix must both encode Seq.
		want := fmt.Sprintf("%02d", t.Seq)
		if t.ID != "" && LastTaskSegment(t.ID) != want {
			out = append(out, fmt.Sprintf("%s: id segment %q does not match seq %s", label, LastTaskSegment(t.ID), want))
		}
		if fileSeq, ok := taskFileSeq(t.File); !ok {
			out = append(out, fmt.Sprintf("%s: file %q must be an .md path", label, t.File))
		} else if fileSeq != want {
			out = append(out, fmt.Sprintf("%s: file %q does not start with seq %s", label, t.File, want))
		}
		// Parent coherence: an existing parent one level up, or "" exactly
		// for top-level tasks.
		if p := ParentTaskID(t.ID); p != t.Parent {
			out = append(out, fmt.Sprintf("%s: parent %q does not match id (want %q)", label, t.Parent, p))
		} else if t.Parent != "" && !taskListed(s.Tasks, t.Parent) {
			out = append(out, fmt.Sprintf("%s: unknown parent %q", label, t.Parent))
		}
	}

	// Dependencies: known targets, no self, no cycles.
	for i := range s.Tasks {
		t := &s.Tasks[i]
		for _, d := range t.DependsOn {
			if d == t.ID {
				out = append(out, fmt.Sprintf("%s: depends on itself", t.ID))
			} else if !ids[d] {
				out = append(out, fmt.Sprintf("%s: unknown dependency %q", t.ID, d))
			}
		}
	}
	out = append(out, depCycles(s.Tasks)...)
	sort.Strings(out)
	return out
}

// taskFileSeq extracts the sequence prefix of a task prose path: the
// basename up to the first "-" ("tasks/00-engine.md" -> "00", "01.md" ->
// "01"). ok is false unless the path is a .md file.
func taskFileSeq(file string) (string, bool) {
	if file == "" || !strings.HasSuffix(file, ".md") {
		return "", false
	}
	base := path.Base(file)
	seq := base
	if i := strings.IndexByte(base, '-'); i >= 0 {
		seq = base[:i]
	}
	if seq == "" {
		return "", false
	}
	return seq, true
}

func taskListed(tasks []TaskState, id string) bool {
	for i := range tasks {
		if tasks[i].ID == id {
			return true
		}
	}
	return false
}

// depCycles reports every dependency cycle in the task list once.
func depCycles(tasks []TaskState) []string {
	deps := make(map[string][]string, len(tasks))
	for _, t := range tasks {
		deps[t.ID] = t.DependsOn
	}
	const (
		white = iota // unvisited
		grey         // on the current path
		black        // done
	)
	color := make(map[string]int, len(tasks))
	var out []string
	var visit func(id string, stack []string)
	visit = func(id string, stack []string) {
		color[id] = grey
		stack = append(stack, id)
		for _, d := range deps[id] {
			if _, ok := deps[d]; !ok {
				continue // unknown deps are reported separately
			}
			switch color[d] {
			case grey:
				out = append(out, fmt.Sprintf("dependency cycle: %s", strings.Join(append(stack, d), " -> ")))
			case white:
				visit(d, stack)
			}
		}
		color[id] = black
	}
	for _, t := range tasks {
		if color[t.ID] == white {
			visit(t.ID, nil)
		}
	}
	return out
}

func validDate(s string) bool {
	if s == "" {
		return false
	}
	_, err := time.Parse("2006-01-02", s)
	return err == nil
}
