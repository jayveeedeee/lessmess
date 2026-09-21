package store

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"
)

// Violation is one breach of the workflow validation contract.
type Violation struct {
	Rule int
	File string
	Msg  string
}

func (v Violation) String() string {
	return fmt.Sprintf("%s: rule %d: %s", v.File, v.Rule, v.Msg)
}

// Validate checks the machine-checkable rules of the JSON workflow state:
//
//   - Rule 1: prose directory names are valid change IDs.
//   - Rule 2: every change has plan.md and tasks/.
//   - Rule 3: prose tree and state agree (referenced files exist, no
//     orphan files, no stray directories).
//   - Rule 4: status vocabularies.
//   - Rule 5: state schema (the model's Validate issues).
//   - Rule 6: index and prose tree agree (every entry resolvable, every
//     directory indexed, archive placement consistent).
//
// Row order (priority) is advisory and cannot be checked mechanically.
func (s *Store) Validate() []Violation {
	s.mu.RLock()
	defer s.mu.RUnlock()
	var out []Violation

	if s.indexErr != nil {
		out = append(out, Violation{Rule: 5, File: "workflow/index.json", Msg: s.indexErr.Error()})
	}
	// Unresolved index entries: no prose directory in the main tree,
	// archive, or worktree (rule 6).
	for _, c := range s.sortedUnresolved() {
		out = append(out, Violation{Rule: 6, File: "changes/" + c.ID, Msg: "index entry " + c.ID + " has no prose directory (main tree, archive, or worktree)"})
	}
	for _, c := range s.sortedAll() {
		if c.Err != nil {
			out = append(out, Violation{Rule: 5, File: "workflow/changes/" + c.ID + ".json", Msg: c.Err.Error()})
		}
		if c.Dir == "" {
			continue
		}
		out = append(out, validateChange(c)...)
	}

	out = append(out, s.validateDirNames()...)
	out = append(out, s.validateRootConsistency()...)

	sort.Slice(out, func(i, j int) bool {
		if out[i].File != out[j].File {
			return out[i].File < out[j].File
		}
		return out[i].Rule < out[j].Rule
	})
	return out
}

// sortedAll returns active plus archived changes sorted by ID.
func (s *Store) sortedAll() []*Change {
	out := append(sortedChanges(s.changes), sortedChanges(s.archived)...)
	sort.Slice(out, func(i, j int) bool { return out[i].ID < out[j].ID })
	return out
}

// sortedUnresolved returns unresolved index entries sorted by ID.
func (s *Store) sortedUnresolved() []*Change { return sortedChanges(s.unresolved) }

// validateChange checks one change's state schema (rules 4/5), required
// prose files (rule 2), and state/prose agreement (rule 3).
func validateChange(c *Change) []Violation {
	var out []Violation
	if c.State == nil {
		return out
	}
	for _, issue := range c.State.Validate() {
		rule := 5
		if strings.Contains(issue, "status") {
			rule = 4
		}
		out = append(out, Violation{Rule: rule, File: "changes/" + c.ID, Msg: issue})
	}
	// Rule 2: required prose files.
	for _, req := range []string{"plan.md", "tasks"} {
		p := filepath.Join(c.Dir, req)
		st, err := os.Stat(p)
		missing := err != nil || (req == "tasks" && !st.IsDir()) || (req != "tasks" && st.IsDir())
		if missing {
			out = append(out, Violation{Rule: 2, File: "changes/" + c.ID + "/" + req, Msg: "required path missing"})
		}
	}
	// Rule 3: referenced prose files exist; no orphans or strays.
	for i := range c.State.Tasks {
		t := &c.State.Tasks[i]
		if _, err := os.Stat(filepath.Join(c.Dir, filepath.FromSlash(t.File))); err != nil {
			out = append(out, Violation{Rule: 3, File: "changes/" + c.ID + "/" + t.File, Msg: "task " + t.ID + " references a missing prose file"})
		}
	}
	for _, f := range c.OrphanFiles {
		out = append(out, Violation{Rule: 3, File: "changes/" + c.ID + "/" + f, Msg: "prose file is not referenced by any task in the state"})
	}
	for _, d := range c.StrayDirs {
		out = append(out, Violation{Rule: 3, File: "changes/" + c.ID + "/" + d, Msg: "directory under tasks/ is not part of any sub plan"})
	}
	return out
}

// Rule 1: change directories match changes/YYYY-MM-DD-(N|xxxxx)/ with
// valid dates; unexpected entries under changes/ are violations.
func (s *Store) validateDirNames() []Violation {
	var out []Violation
	top, _ := os.ReadDir(s.ChangesDir)
	for _, e := range top {
		if !e.IsDir() {
			out = append(out, Violation{Rule: 1, File: "changes/" + e.Name(), Msg: "unexpected file in changes/ (prose only; state lives in .lessmess/workflow/)"})
			continue
		}
		if e.Name() == "archive" {
			arch, _ := os.ReadDir(filepath.Join(s.ChangesDir, "archive"))
			for _, a := range arch {
				if !a.IsDir() || !validChangeDirName(a.Name()) {
					out = append(out, Violation{Rule: 1, File: "changes/archive/" + a.Name(), Msg: "not a valid change directory name"})
				}
			}
			continue
		}
		if !validChangeDirName(e.Name()) {
			out = append(out, Violation{Rule: 1, File: "changes/" + e.Name(), Msg: "not a valid change directory name (want YYYY-MM-DD-N or YYYY-MM-DD-xxxxx, five lowercase alphanumerics)"})
		}
	}
	return out
}

func validChangeDirName(name string) bool {
	if !changeIDRe.MatchString(name) {
		return false
	}
	_, err := time.Parse("2006-01-02", name[:10])
	return err == nil
}

// Rule 6: index and prose tree agree — every changes/<id> directory has
// an index entry, and archived entries live under changes/archive/.
func (s *Store) validateRootConsistency() []Violation {
	var out []Violation
	indexed := map[string]bool{}
	if s.index != nil {
		for _, e := range s.index.Changes {
			indexed[e.ID] = true
			if e.Archived {
				expected := filepath.Join(s.ChangesDir, "archive", e.ID)
				if c := s.archived[e.ID]; c != nil && c.Dir != "" && c.Dir != expected {
					out = append(out, Violation{Rule: 6, File: "changes/" + e.ID, Msg: "archived change's prose is not under changes/archive/" + e.ID})
				}
			}
		}
	}
	for _, base := range []string{s.ChangesDir, filepath.Join(s.ChangesDir, "archive")} {
		entries, _ := os.ReadDir(base)
		for _, e := range entries {
			if !e.IsDir() || e.Name() == "archive" || !validChangeDirName(e.Name()) {
				continue
			}
			if !indexed[e.Name()] {
				out = append(out, Violation{Rule: 6, File: "changes/" + e.Name(), Msg: "change directory has no index entry"})
			}
		}
	}
	return out
}
