package store

import (
	"fmt"
	"path/filepath"
	"strings"

	"lessmess/internal/model"
)

// LedgerFile returns a deterministic, generated markdown view of a
// change's state: header, the top-level task table in the classic pinned
// schema, one section per decomposed task, and the decision log. It is a
// view only — the JSON state file is authoritative.
func (s *Store) LedgerFile(changeID string) (string, error) {
	c, err := s.Change(changeID)
	if err != nil {
		return "", err
	}
	if c.State == nil {
		return "", fmt.Errorf("%w: %v", ErrInvalid, c.Err)
	}
	st := c.State
	var b strings.Builder
	fmt.Fprintf(&b, "# Ledger — %s\n\n", st.ID)
	fmt.Fprintf(&b, "- Change ID: %s\n", st.ID)
	fmt.Fprintf(&b, "- Plan: [plan.md](plan.md)\n")
	if st.Branch != "" {
		fmt.Fprintf(&b, "- Branch: %s\n", st.Branch)
	}
	fmt.Fprintf(&b, "- Overall status: %s\n", st.Status.Value)
	fmt.Fprintf(&b, "- Last updated: %s\n\n", st.Updated)
	b.WriteString(renderTaskTable(st, ""))
	for _, n := range c.Roots {
		if n.HasContainer() {
			renderContainer(&b, st, n)
		}
	}
	if len(st.Decision) > 0 {
		b.WriteString("## Decision log\n\n")
		rows := make([][]string, 0, len(st.Decision))
		for _, d := range st.Decision {
			rows = append(rows, []string{d.Date, d.Decision})
		}
		b.WriteString(strings.Join(model.RenderTable([]string{"Date", "Decision"}, rows), "\n"))
		b.WriteString("\n")
	}
	return b.String(), nil
}

// renderContainer writes one decomposed task's section with its
// children's table, recursing into deeper sub plans.
func renderContainer(b *strings.Builder, st *model.ChangeState, n *TaskNode) {
	fmt.Fprintf(b, "## %s — %s\n\n", n.ID, n.Task.Title)
	b.WriteString(renderTaskTable(st, n.ID))
	for _, ch := range n.Children {
		if ch.HasContainer() {
			renderContainer(b, st, ch)
		}
	}
}

// renderTaskTable renders one level's task table in the classic pinned
// schema (the same columns AGENTS.md always used) from the state's tasks.
func renderTaskTable(st *model.ChangeState, parent string) string {
	tasks := st.Children(parent)
	rows := make([][]string, 0, len(tasks))
	for _, t := range tasks {
		deps := model.Empty
		if len(t.DependsOn) > 0 {
			deps = strings.Join(t.DependsOn, ", ")
		}
		notes := t.Notes
		if notes == "" {
			notes = model.Empty
		}
		rows = append(rows, []string{
			model.FormatLink(t.ID, t.File), t.Title, string(t.Status), deps, t.Updated, notes,
		})
	}
	var b strings.Builder
	b.WriteString("## Tasks\n\n")
	if parent != "" {
		b.WriteString(fmt.Sprintf("Subtasks of %s, in priority order.\n\n", parent))
	} else {
		b.WriteString("Row order is display and priority order; top row is highest priority.\n\n")
	}
	b.WriteString(strings.Join(model.RenderTable(model.TaskColumns, rows), "\n"))
	b.WriteString("\n\n")
	return b.String()
}

// ContainerLedgerFile returns the generated markdown view of one task
// container's level: the decomposed task's children table. href is
// change-relative and must be a tasks/…/ledger.md path (the legacy link
// shape); anything else is ErrNotFound.
func (s *Store) ContainerLedgerFile(changeID, href string) (string, error) {
	c, err := s.Change(changeID)
	if err != nil {
		return "", err
	}
	if c.State == nil {
		return "", fmt.Errorf("%w: %v", ErrInvalid, c.Err)
	}
	clean := filepath.Clean(filepath.FromSlash(href))
	if filepath.IsAbs(clean) || strings.HasPrefix(clean, "..") {
		return "", ErrNotFound
	}
	slashed := filepath.ToSlash(clean)
	parts := strings.Split(slashed, "/")
	if len(parts) < 3 || parts[0] != "tasks" || parts[len(parts)-1] != "ledger.md" {
		return "", ErrNotFound
	}
	containerRel := strings.TrimSuffix(slashed, "/ledger.md")
	var owner *TaskNode
	c.WalkTasks(func(n *TaskNode) bool {
		if n.ContainerRel() == containerRel {
			owner = n
			return false
		}
		return true
	})
	if owner == nil {
		return "", ErrNotFound
	}
	var b strings.Builder
	fmt.Fprintf(&b, "# Ledger — %s\n\n", owner.ID)
	fmt.Fprintf(&b, "- Task: %s (change %s)\n", owner.ID, c.ID)
	fmt.Fprintf(&b, "- Last updated: %s\n", c.State.Updated)
	b.WriteString("\n")
	b.WriteString(renderTaskTable(c.State, owner.ID))
	return b.String(), nil
}
