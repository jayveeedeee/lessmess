package store

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"lessmess/internal/model"
)

// Violation is one breach of the AGENTS.md validation contract.
type Violation struct {
	Rule int
	File string
	Msg  string
}

func (v Violation) String() string {
	return fmt.Sprintf("%s: rule %d: %s", v.File, v.Rule, v.Msg)
}

// Validate checks the seven machine-checkable rules from AGENTS.md.
// Rule 7 (row order reflects intended priority) is advisory by nature and
// cannot be checked mechanically; it is intentionally not implemented.
func (s *Store) Validate() []Violation {
	s.mu.RLock()
	defer s.mu.RUnlock()
	var out []Violation

	// Parse failures are violations of rules 4 (status vocabulary) and 5
	// (pinned schemas) — the parser enforces both strictly.
	if s.rootErr != nil {
		out = append(out, Violation{Rule: 5, File: "changes/ledger.md", Msg: s.rootErr.Error()})
	}
	for _, c := range s.changes {
		if c.Err != nil {
			out = append(out, Violation{Rule: 5, File: "changes/" + c.ID + "/ledger.md", Msg: c.Err.Error()})
		}
		for href, err := range c.TaskErrs {
			out = append(out, Violation{Rule: 3, File: "changes/" + c.ID + "/" + href, Msg: err.Error()})
		}
	}

	out = append(out, s.validateDirNames()...)
	out = append(out, s.validateRequiredFiles()...)
	out = append(out, s.validateTaskConsistency()...)
	out = append(out, s.validateRootConsistency()...)

	sort.Slice(out, func(i, j int) bool {
		if out[i].File != out[j].File {
			return out[i].File < out[j].File
		}
		return out[i].Rule < out[j].Rule
	})
	return out
}

// Rule 1: change directories match changes/YYYY-MM-DD-N/ with valid dates.
func (s *Store) validateDirNames() []Violation {
	var out []Violation
	top, _ := os.ReadDir(s.ChangesDir)
	for _, e := range top {
		if !e.IsDir() {
			if e.Name() != "ledger.md" {
				out = append(out, Violation{Rule: 1, File: "changes/" + e.Name(), Msg: "unexpected file in changes/"})
			}
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

// Rule 2: every change directory contains plan.md, ledger.md, tasks/.
func (s *Store) validateRequiredFiles() []Violation {
	var out []Violation
	for _, c := range s.all() {
		for _, req := range []string{"plan.md", "ledger.md", "tasks"} {
			p := filepath.Join(c.Dir, req)
			st, err := os.Stat(p)
			missing := err != nil || (req == "tasks" && !st.IsDir()) || (req != "tasks" && st.IsDir())
			if missing {
				out = append(out, Violation{Rule: 2, File: "changes/" + c.ID + "/" + req, Msg: "required file missing"})
			}
		}
	}
	return out
}

// Rule 3: every task file has exactly one ledger row and vice versa;
// frontmatter id, ledger Task cell, and filename sequence agree.
func (s *Store) validateTaskConsistency() []Violation {
	var out []Violation
	for _, c := range s.all() {
		if c.Ledger == nil {
			continue // already reported as rule 5
		}
		rowByHref := map[string]model.TaskRow{}
		for _, r := range c.Ledger.Rows {
			rowByHref[r.Href] = r
			tf, ok := c.Tasks[r.Href]
			if !ok {
				out = append(out, Violation{Rule: 3, File: "changes/" + c.ID + "/" + r.Href, Msg: fmt.Sprintf("ledger row %s has no parseable task file", r.ID)})
				continue
			}
			if tf.ID != r.ID {
				out = append(out, Violation{Rule: 3, File: "changes/" + c.ID + "/" + r.Href, Msg: fmt.Sprintf("frontmatter id %q != ledger row %q", tf.ID, r.ID)})
			}
			if seqOf(r.Href) != idSuffixOf(r.ID) {
				out = append(out, Violation{Rule: 3, File: "changes/" + c.ID + "/" + r.Href, Msg: fmt.Sprintf("filename sequence %q != id suffix %q", seqOf(r.Href), idSuffixOf(r.ID))})
			}
		}
		for href := range c.Tasks {
			if _, ok := rowByHref[href]; !ok {
				out = append(out, Violation{Rule: 3, File: "changes/" + c.ID + "/" + href, Msg: "task file has no ledger row"})
			}
		}
		for href := range c.TaskErrs {
			if _, ok := rowByHref[href]; !ok {
				out = append(out, Violation{Rule: 3, File: "changes/" + c.ID + "/" + href, Msg: "unparseable task file has no ledger row"})
			}
		}
	}
	return out
}

func seqOf(href string) string {
	base := filepath.Base(href)
	if i := strings.Index(base, "-"); i > 0 {
		return base[:i]
	}
	return strings.TrimSuffix(base, ".md")
}

func idSuffixOf(id string) string {
	if i := strings.LastIndex(id, "-"); i >= 0 {
		return id[i+1:]
	}
	return id
}

// Rule 6: the root ledger has one row per change directory (including
// archived ones) and root status agrees with each change ledger.
func (s *Store) validateRootConsistency() []Violation {
	var out []Violation
	if s.root == nil {
		return out // already reported as rule 5
	}
	rowCount := map[string]int{}
	for _, r := range s.root.Rows {
		rowCount[r.Change]++
		found := false
		for _, c := range s.all() {
			if c.ID == r.Change {
				found = true
				if c.Ledger != nil && c.Ledger.Overall != r.Status {
					out = append(out, Violation{Rule: 6, File: "changes/ledger.md", Msg: fmt.Sprintf("root status %q != %s overall status %q", r.Status, r.Change, c.Ledger.Overall)})
				}
			}
		}
		if !found {
			out = append(out, Violation{Rule: 6, File: "changes/ledger.md", Msg: fmt.Sprintf("root row %s has no change directory", r.Change)})
		}
	}
	for _, c := range s.all() {
		if rowCount[c.ID] == 0 {
			out = append(out, Violation{Rule: 6, File: "changes/ledger.md", Msg: fmt.Sprintf("change %s has no root-ledger row", c.ID)})
		} else if rowCount[c.ID] > 1 {
			out = append(out, Violation{Rule: 6, File: "changes/ledger.md", Msg: fmt.Sprintf("change %s has %d root-ledger rows", c.ID, rowCount[c.ID])})
		}
	}
	return out
}

func (s *Store) all() []*Change {
	out := make([]*Change, 0, len(s.changes)+len(s.archived))
	for _, c := range s.changes {
		out = append(out, c)
	}
	for _, c := range s.archived {
		out = append(out, c)
	}
	return out
}
