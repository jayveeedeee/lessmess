package store

import (
	"errors"
	"fmt"
	"os"
	"path"
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
	}

	out = append(out, s.validateDirNames()...)
	out = append(out, s.validateRequiredFiles()...)
	out = append(out, s.validateTaskTree()...)
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

// validateTaskTree checks rule 3 (and the container parts of rules 2 and
// 5) recursively over the change's task tree: every task file has exactly
// one row in its governing ledger and vice versa, IDs and filename
// sequences agree, dotted ID depth matches the nesting, containers are
// well-formed, and directories under tasks/ always match a sibling task
// file.
func (s *Store) validateTaskTree() []Violation {
	var out []Violation
	for _, c := range s.all() {
		file := "changes/" + c.ID + "/"
		for _, d := range c.StrayDirs {
			out = append(out, Violation{Rule: 3, File: file + d, Msg: "directory under tasks/ has no matching sibling task file"})
		}
		if c.Ledger != nil {
			// Top-level rows link relative to the change directory.
			out = append(out, validateLevel(c, "", c.Ledger.Rows, c.Roots)...)
		}
		c.WalkTasks(func(n *TaskNode) bool {
			if !n.HasContainer() {
				return true
			}
			rel := n.ContainerRel()
			switch {
			case errors.Is(n.ContainerErr, os.ErrNotExist):
				out = append(out, Violation{Rule: 2, File: file + rel + "/ledger.md", Msg: "task container missing ledger.md"})
			case n.ContainerErr != nil:
				out = append(out, Violation{Rule: 5, File: file + rel + "/ledger.md", Msg: n.ContainerErr.Error()})
			}
			if st, err := os.Stat(filepath.Join(c.Dir, filepath.FromSlash(rel), "tasks")); err != nil || !st.IsDir() {
				out = append(out, Violation{Rule: 2, File: file + rel + "/tasks", Msg: "task container missing tasks/ directory"})
			}
			if n.Container != nil {
				out = append(out, validateLevel(c, rel+"/", n.Container.Rows, n.Children)...)
			}
			return true
		})
	}
	return out
}

// validateLevel cross-checks one governing ledger's rows against the
// nodes of the level it governs. hrefPrefix is "" for the change ledger
// (its rows already link change-relative) or "<container>/" for container
// ledgers (their rows link container-relative).
func validateLevel(c *Change, hrefPrefix string, rows []model.TaskRow, nodes []*TaskNode) []Violation {
	var out []Violation
	file := "changes/" + c.ID + "/"
	nodeByHref := make(map[string]*TaskNode, len(nodes))
	for _, n := range nodes {
		nodeByHref[n.Href] = n
	}
	rowHrefs := make(map[string]bool, len(rows))
	for _, r := range rows {
		href := hrefPrefix + r.Href
		rowHrefs[href] = true
		n := nodeByHref[href]
		if n == nil {
			out = append(out, Violation{Rule: 3, File: file + href, Msg: fmt.Sprintf("ledger row %s has no parseable task file", r.ID)})
			continue
		}
		if n.FileErr != nil {
			out = append(out, Violation{Rule: 3, File: file + href, Msg: n.FileErr.Error()})
		} else if n.File != nil && n.File.ID != r.ID {
			out = append(out, Violation{Rule: 3, File: file + href, Msg: fmt.Sprintf("frontmatter id %q != ledger row %q", n.File.ID, r.ID)})
		}
		if seqOf(href) != model.LastTaskSegment(r.ID) {
			out = append(out, Violation{Rule: 3, File: file + href, Msg: fmt.Sprintf("filename sequence %q != id segment %q", seqOf(href), model.LastTaskSegment(r.ID))})
		}
		if !model.DottedSegmentsValid(r.ID) {
			out = append(out, Violation{Rule: 3, File: file + href, Msg: fmt.Sprintf("task id %q has malformed dotted segments (want two-digit)", r.ID)})
		}
		if depth := 1 + ancestors(n); model.TaskIDDepth(r.ID) != depth {
			out = append(out, Violation{Rule: 3, File: file + href, Msg: fmt.Sprintf("task id %q depth %d != nesting depth %d", r.ID, model.TaskIDDepth(r.ID), depth)})
		}
	}
	for _, n := range nodes {
		if rowHrefs[n.Href] {
			continue
		}
		if n.FileErr != nil {
			out = append(out, Violation{Rule: 3, File: file + n.Href, Msg: "unparseable task file has no ledger row: " + n.FileErr.Error()})
			continue
		}
		id := n.ID
		if id == "" {
			id = n.Href
		}
		out = append(out, Violation{Rule: 3, File: file + n.Href, Msg: fmt.Sprintf("task file %s has no ledger row", id)})
	}
	return out
}

// ancestors returns the number of ancestor tasks of n (0 for top level).
func ancestors(n *TaskNode) int {
	d := 0
	for p := n.Parent; p != nil; p = p.Parent {
		d++
	}
	return d
}

// seqOf extracts the filename sequence from a task href ("…/01-x.md" → "01").
func seqOf(href string) string {
	base := path.Base(href)
	if i := strings.Index(base, "-"); i > 0 {
		return base[:i]
	}
	return strings.TrimSuffix(base, ".md")
}

// CloseOutReady reports whether every non-cancelled task in the change's
// whole tree is Test or Done (the recursive close-out gate). It lists the
// offending task IDs (or hrefs for broken files) when not ready.
func (s *Store) CloseOutReady(changeID string) (bool, []string, error) {
	c, err := s.Change(changeID)
	if err != nil {
		return false, nil, err
	}
	var offending []string
	c.WalkTasks(func(n *TaskNode) bool {
		st := n.status()
		if st != model.StatusTest && st != model.StatusDone && st != model.StatusCancelled {
			if n.ID != "" {
				offending = append(offending, n.ID)
			} else {
				offending = append(offending, n.Href)
			}
		}
		return true
	})
	return len(offending) == 0, offending, nil
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
