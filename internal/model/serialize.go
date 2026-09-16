package model

import (
	"errors"
	"strings"
)

// ErrTaskNotFound is returned when a task ID is not in the ledger.
var ErrTaskNotFound = errors.New("task not found")

// ErrChangeNotFound is returned when a change ID is not in the root ledger.
var ErrChangeNotFound = errors.New("change not found")

func (r RootRow) cells() []string {
	return []string{FormatLink(r.Change, r.Href), r.Title, r.Prefix, r.Branch, string(r.Status), r.Created, r.Updated}
}

// syncTable re-renders the table region from Rows, preserving all
// non-table content byte-for-byte.
func (l *RootLedger) syncTable() {
	rows := make([][]string, 0, len(l.Rows))
	for _, r := range l.Rows {
		rows = append(rows, r.cells())
	}
	t := RenderTable(RootColumns, rows)
	l.Lines = Splice(l.Lines, l.Table.Start, l.Table.End, t)
	l.Table.End = l.Table.Start + len(t)
	l.Table.Rows = rows
}

// Content returns the full document with any mutations applied.
func (l *RootLedger) Content() []byte { return []byte(strings.Join(l.Lines, "\n")) }

// AppendRow adds a change row at the bottom of the table.
func (l *RootLedger) AppendRow(r RootRow) {
	l.Rows = append(l.Rows, r)
	l.syncTable()
}

// Update sets the status and last-updated date of an existing change row.
func (l *RootLedger) Update(change string, status OverallStatus, updated string) error {
	for i := range l.Rows {
		if l.Rows[i].Change == change {
			l.Rows[i].Status = status
			l.Rows[i].Updated = updated
			l.syncTable()
			return nil
		}
	}
	return ErrChangeNotFound
}

func (r TaskRow) cells() []string {
	deps := Empty
	if len(r.Depends) > 0 {
		deps = strings.Join(r.Depends, ", ")
	}
	return []string{FormatLink(r.ID, r.Href), r.Title, string(r.Status), deps, r.Updated, r.Notes}
}

func (l *ChangeLedger) syncTable() { l.taskTable.syncTable() }

// syncTable re-renders the task-table region from Rows, preserving all
// non-table content byte-for-byte.
func (tt *taskTable) syncTable() {
	rows := make([][]string, 0, len(tt.Rows))
	for _, r := range tt.Rows {
		rows = append(rows, r.cells())
	}
	t := RenderTable(TaskColumns, rows)
	tt.Lines = Splice(tt.Lines, tt.Table.Start, tt.Table.End, t)
	tt.Table.End = tt.Table.Start + len(t)
	tt.Table.Rows = rows
}

// Content returns the full document with any mutations applied.
func (tt *taskTable) Content() []byte { return []byte(strings.Join(tt.Lines, "\n")) }

// AppendTask adds a task row at the bottom of the table.
func (tt *taskTable) AppendTask(r TaskRow) {
	tt.Rows = append(tt.Rows, r)
	tt.syncTable()
}

// SetOverall sets the overall status and last-updated header fields.
func (l *ChangeLedger) SetOverall(status OverallStatus, updated string) {
	l.Overall = status
	l.LastUpdated = updated
	for i, ln := range l.Lines {
		switch {
		case strings.HasPrefix(ln, "- Overall status:"):
			l.Lines[i] = "- Overall status: " + string(status)
		case strings.HasPrefix(ln, "- Last updated:"):
			l.Lines[i] = "- Last updated: " + updated
		}
	}
}

// insertionPos returns the row index at which a task with the given status
// must be inserted to land at visual position toIndex within its status
// group. Groups absent from the table append at the end.
func insertionPos(rows []TaskRow, status TaskStatus, toIndex int) int {
	count := 0
	for i, r := range rows {
		if r.Status != status {
			continue
		}
		if count == toIndex {
			return i
		}
		count++
	}
	if count > 0 {
		for i := len(rows) - 1; i >= 0; i-- {
			if rows[i].Status == status {
				return i + 1
			}
		}
	}
	return len(rows)
}

// MoveTask sets the task's status and repositions its row so it lands at
// visual index toIndex within the target status group. Row order is the
// kanban display and priority order per AGENTS.md.
func (tt *taskTable) MoveTask(id string, toStatus TaskStatus, toIndex int, updated string) error {
	if !toStatus.Valid() {
		return parseErr(tt.doc, "invalid status %q", string(toStatus))
	}
	if toIndex < 0 {
		toIndex = 0
	}
	idx := -1
	for i := range tt.Rows {
		if tt.Rows[i].ID == id {
			idx = i
			break
		}
	}
	if idx < 0 {
		return ErrTaskNotFound
	}
	row := tt.Rows[idx]
	row.Status = toStatus
	row.Updated = updated
	rest := make([]TaskRow, 0, len(tt.Rows)-1)
	rest = append(rest, tt.Rows[:idx]...)
	rest = append(rest, tt.Rows[idx+1:]...)
	pos := insertionPos(rest, toStatus, toIndex)
	rest = append(rest, TaskRow{})
	copy(rest[pos+1:], rest[pos:])
	rest[pos] = row
	tt.Rows = rest
	tt.syncTable()
	return nil
}
