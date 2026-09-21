package store

import (
	"io/fs"
	"path"
	"path/filepath"
	"sort"
	"strings"

	"lessmess/internal/model"
)

// TaskNode is one task in a change's task tree: a top-level task or a
// nested subtask inside a decomposed parent's container. Href is the
// change-relative prose path ("tasks/00-a/tasks/01-b.md"); identity and
// state come from the JSON store (Task).
type TaskNode struct {
	ID       string
	Href     string
	Task     *model.TaskState
	Parent   *TaskNode   // nil for top-level tasks
	Children []*TaskNode // in priority order

	// containerDir marks tasks whose container prose directory exists
	// (decomposed, possibly with no subtasks yet).
	containerDir bool
}

// HasContainer reports whether the task is decomposed: it has subtasks or
// its container prose directory exists (empty right after DecomposeTask).
func (n *TaskNode) HasContainer() bool { return n.containerDir || len(n.Children) > 0 }

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
		st.ByStatus[ch.NodeStatus()]++
		if ch.NodeStatus() != model.StatusCancelled {
			st.Total++
			if ch.NodeStatus() == model.StatusTest || ch.NodeStatus() == model.StatusDone {
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

// NodeStatus returns the node's status (Not started when absent) — the
// display-facing accessor used by view builders.
func (n *TaskNode) NodeStatus() model.TaskStatus {
	if n.Task != nil {
		return n.Task.Status
	}
	return model.StatusNotStarted
}

// AllTaskStats aggregates the change's entire task tree — the top-level
// tasks themselves included, unlike SubtreeStats, which rolls up one
// node's descendants only. This is the change-level progress shown in the
// board header and the index counts.
func (c *Change) AllTaskStats() SubtreeStats {
	st := SubtreeStats{ByStatus: map[model.TaskStatus]int{}}
	c.WalkTasks(func(n *TaskNode) bool {
		s := n.NodeStatus()
		st.ByStatus[s]++
		if s != model.StatusCancelled {
			st.Total++
			if s == model.StatusTest || s == model.StatusDone {
				st.Complete++
			}
		}
		return true
	})
	return st
}

// Node returns the task node with the given task ID, or nil.
func (c *Change) Node(id string) *TaskNode { return c.Nodes[id] }

// WalkTasks visits every task node in the tree (depth-first, children in
// priority order) until visit returns false.
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

// scanProse cross-checks the prose tree under tasks/ against the state's
// referenced files: any .md file not referenced by a task is an orphan,
// any directory that is neither an ancestor of a referenced file nor the
// container of a referenced sibling task file is a stray. Container dirs
// named after referenced sibling files stay allowed even while empty
// (DecomposeTask creates them before the first subtask exists). Results
// feed validation and HasContainer.
func (c *Change) scanProse() {
	c.OrphanFiles = nil
	c.StrayDirs = nil
	c.containers = map[string]bool{}
	allowed := map[string]bool{"tasks": true}
	candidates := map[string]bool{} // container dirs named after sibling task files
	if c.State != nil {
		for i := range c.State.Tasks {
			f := c.State.Tasks[i].File
			// Ancestor directories of the referenced file are allowed.
			for d := path.Dir(f); d != "." && d != "/"; d = path.Dir(d) {
				allowed[d] = true
			}
			// The container dir named after this task file is allowed
			// (the sub plan's prose home, empty until the first child).
			base := strings.TrimSuffix(path.Base(f), ".md")
			container := path.Join(path.Dir(f), base)
			allowed[container] = true
			candidates[container] = true
		}
	}
	tasksDir := filepath.Join(c.Dir, "tasks")
	_ = filepath.WalkDir(tasksDir, func(p string, d fs.DirEntry, err error) error {
		if err != nil {
			return nil
		}
		rel, rerr := filepath.Rel(c.Dir, p)
		if rerr != nil {
			return nil
		}
		slashed := filepath.ToSlash(rel)
		if d.IsDir() {
			if candidates[slashed] {
				c.containers[slashed] = true // the container dir really exists
			}
			if !allowed[slashed] {
				c.StrayDirs = append(c.StrayDirs, slashed)
			}
			return nil
		}
		if strings.HasSuffix(slashed, ".md") && slashed != "plan.md" && !referenced(c.State, slashed) {
			c.OrphanFiles = append(c.OrphanFiles, slashed)
		}
		return nil
	})
	sort.Strings(c.OrphanFiles)
	sort.Strings(c.StrayDirs)
}

func referenced(st *model.ChangeState, file string) bool {
	if st == nil {
		return false
	}
	for i := range st.Tasks {
		if st.Tasks[i].File == file {
			return true
		}
	}
	return false
}
