package store

import (
	"os"
	"path"
	"path/filepath"
	"sort"
	"strings"

	"lessmess/internal/model"
)

// TaskNode is one task in a change's task tree: a top-level task or a
// nested subtask inside a decomposed parent's container. Hrefs are
// change-relative slash paths ("tasks/00-a/tasks/01-b.md"); ledger rows
// link relative to their own ledger's directory.
type TaskNode struct {
	ID           string
	Href         string
	File         *model.TaskFile // parsed task file (nil when FileErr set)
	FileErr      error
	Row          *model.TaskRow // governing-ledger row when one matches (by href)
	Parent       *TaskNode      // nil for top-level tasks
	Children     []*TaskNode    // in governing-ledger row order
	Container    *model.TaskLedger // set when the task is decomposed
	ContainerErr error              // container ledger parse error, if any
}

// HasContainer reports whether the task is decomposed.
func (n *TaskNode) HasContainer() bool { return n.Container != nil || n.ContainerErr != nil }

// ContainerRel is the change-relative path of the task's container
// directory (its href minus the .md suffix).
func (n *TaskNode) ContainerRel() string { return strings.TrimSuffix(n.Href, ".md") }

// SubtreeStats summarizes a node's descendants for display rollup. It is
// derived data only: nothing is ever written back from it.
type SubtreeStats struct {
	ByStatus map[model.TaskStatus]int
	Total    int // non-cancelled descendants
	Complete int // Test + Done descendants
}

// SubtreeStats aggregates the node's whole subtree (not the node itself).
func (n *TaskNode) SubtreeStats() SubtreeStats {
	st := SubtreeStats{ByStatus: map[model.TaskStatus]int{}}
	for _, ch := range n.Children {
		st.ByStatus[ch.status()]++
		if ch.status() != model.StatusCancelled {
			st.Total++
			if ch.status() == model.StatusTest || ch.status() == model.StatusDone {
				st.Complete++
			}
		}
		chs := ch.SubtreeStats()
		for k, v := range chs.ByStatus {
			st.ByStatus[k] += v
		}
		st.Total += chs.Total
		st.Complete += chs.Complete
	}
	return st
}

// NodeStatus returns the node's governing-row status (Not started when no
// row matched) — the display-facing accessor used by view builders.
func (n *TaskNode) NodeStatus() model.TaskStatus { return n.status() }

// status returns the node's status from its governing row, defaulting to
// Not started when no row matched yet.
func (n *TaskNode) status() model.TaskStatus {
	if n.Row != nil {
		return n.Row.Status
	}
	return model.StatusNotStarted
}

// AllTaskStats aggregates the whole task tree of a change.
func (c *Change) AllTaskStats() SubtreeStats {
	st := SubtreeStats{ByStatus: map[model.TaskStatus]int{}}
	for _, root := range c.Roots {
		s := root.SubtreeStats()
		for k, v := range s.ByStatus {
			st.ByStatus[k] += v
		}
		st.Total += s.Total
		st.Complete += s.Complete
	}
	return st
}

// Node returns the task node with the given task ID, or nil.
func (c *Change) Node(id string) *TaskNode { return c.Nodes[id] }

// WalkTasks visits every task node in the tree (depth-first, children in
// ledger row order) until visit returns false.
func (c *Change) WalkTasks(visit func(*TaskNode) bool) {
	var walk func(nodes []*TaskNode) bool
	walk = func(nodes []*TaskNode) bool {
		for _, n := range nodes {
			if !visit(n) || !walk(n.Children) {
				return false
			}
		}
		return true
	}
	walk(c.Roots)
}

// scanTasks reads one level of a tasks/ directory: task files become
// nodes, matching subdirectories become containers and are scanned
// recursively, and directories without a matching sibling task file are
// recorded as strays for validation. relDir is the change-relative path
// of the directory with a trailing slash; rows are the governing ledger's
// rows for this level (nil when that ledger failed to parse).
func (s *Store) scanTasks(c *Change, absDir, relDir string, parent *TaskNode, rows []model.TaskRow) []*TaskNode {
	entries, err := os.ReadDir(absDir)
	if err != nil {
		return nil
	}
	var names []string
	containers := map[string]bool{}
	for _, e := range entries {
		if e.IsDir() {
			containers[e.Name()] = true
		} else if strings.HasSuffix(e.Name(), ".md") {
			names = append(names, e.Name())
		}
	}
	sort.Strings(names)

	nodes := map[string]*TaskNode{}
	for _, name := range names {
		href := relDir + name
		n := &TaskNode{Href: href, Parent: parent}
		data, err := os.ReadFile(filepath.Join(absDir, name))
		switch {
		case err != nil:
			n.FileErr = err
		default:
			if tf, err := model.ParseTaskFile(c.ID+"/"+href, data); err != nil {
				n.FileErr = err
			} else {
				n.File, n.ID = tf, tf.ID
			}
		}
		nodes[name] = n
	}

	// Order nodes by the governing ledger's row order; files without a
	// row keep filename order after them.
	var out []*TaskNode
	seen := map[string]bool{}
	for i := range rows {
		r := rows[i]
		base := path.Base(r.Href)
		if n, ok := nodes[base]; ok && !seen[base] {
			seen[base] = true
			row := r
			n.Row = &row
			if n.ID == "" {
				n.ID = r.ID
			}
			out = append(out, n)
		}
	}
	for _, name := range names {
		if !seen[name] {
			out = append(out, nodes[name])
		}
	}

	// Containers: recurse into directories that match a sibling task file.
	for _, name := range names {
		base := strings.TrimSuffix(name, ".md")
		if !containers[base] {
			continue
		}
		n := nodes[name]
		containerRel := relDir + base
		var tl *model.TaskLedger
		var terr error
		if data, err := os.ReadFile(filepath.Join(absDir, base, "ledger.md")); err != nil {
			terr = err
		} else if l, err := model.ParseTaskLedger(c.ID+"/"+containerRel+"/ledger.md", data); err != nil {
			terr = err
		} else {
			tl = l
		}
		n.Container, n.ContainerErr = tl, terr
		var childRows []model.TaskRow
		if tl != nil {
			childRows = tl.Rows
		}
		n.Children = s.scanTasks(c, filepath.Join(absDir, base, "tasks"), containerRel+"/tasks/", n, childRows)
	}
	for _, d := range sortedKeys(containers) {
		if _, ok := nodes[d+".md"]; !ok {
			c.StrayDirs = append(c.StrayDirs, relDir+d)
		}
	}
	return out
}

// indexNodes populates the change's ID -> node map from its tree.
func (c *Change) indexNodes() {
	c.Nodes = map[string]*TaskNode{}
	c.WalkTasks(func(n *TaskNode) bool {
		if n.ID != "" {
			c.Nodes[n.ID] = n
		}
		return true
	})
}

func sortedKeys(m map[string]bool) []string {
	out := make([]string, 0, len(m))
	for k := range m {
		out = append(out, k)
	}
	sort.Strings(out)
	return out
}
