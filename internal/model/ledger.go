package model

import (
	"regexp"
	"strings"
)

// RootColumns is the pinned root-ledger schema from AGENTS.md.
var RootColumns = []string{"Change", "Title", "ID prefix", "Branch", "Status", "Created", "Last updated"}

// RootRow is one row of the root ledger.
type RootRow struct {
	Change  string
	Href    string
	Title   string
	Prefix  string
	Branch  string
	Status  OverallStatus
	Created string
	Updated string
}

// RootLedger is the parsed changes/ledger.md.
type RootLedger struct {
	Lines []string
	Table *Table
	Rows  []RootRow
}

// ParseRootLedger parses changes/ledger.md strictly against the pinned schema.
func ParseRootLedger(name string, data []byte) (*RootLedger, error) {
	lines := strings.Split(string(data), "\n")
	t, err := FindTable(lines, RootColumns, name)
	if err != nil {
		return nil, err
	}
	l := &RootLedger{Lines: lines, Table: t}
	for i, cells := range t.Rows {
		text, href, ok := ParseLink(cells[0])
		if !ok {
			return nil, parseErr(name, "root ledger row %d: Change cell is not a link", i+1)
		}
		st := OverallStatus(cells[4])
		if !st.Valid() {
			return nil, parseErr(name, "root ledger row %d: invalid status %q", i+1, cells[4])
		}
		l.Rows = append(l.Rows, RootRow{
			Change: text, Href: href, Title: cells[1], Prefix: cells[2],
			Branch: cells[3], Status: st, Created: cells[5], Updated: cells[6],
		})
	}
	return l, nil
}

// TaskColumns is the pinned per-change ledger schema from AGENTS.md.
var TaskColumns = []string{"Task", "Title", "Status", "Depends on", "Updated", "Notes"}

// TaskRow is one row of a change ledger's task table.
type TaskRow struct {
	ID      string
	Href    string
	Title   string
	Status  TaskStatus
	Depends []string
	Updated string
	Notes   string
}

// taskTable is the shared mutable core of every ledger that governs tasks:
// the document lines, the located task table, and its rows. Change ledgers
// and task-container ledgers embed it so mutations (append, move) behave
// identically at any nesting level.
type taskTable struct {
	doc   string // document name for error attribution
	Lines []string
	Table *Table
	Rows  []TaskRow
}

// parseTaskRows locates the pinned task table in lines and parses its rows.
func parseTaskRows(lines []string, name string) (*Table, []TaskRow, error) {
	t, err := FindTable(lines, TaskColumns, name)
	if err != nil {
		return nil, nil, err
	}
	var rows []TaskRow
	for i, cells := range t.Rows {
		text, href, ok := ParseLink(cells[0])
		if !ok {
			return nil, nil, parseErr(name, "task row %d: Task cell is not a link", i+1)
		}
		st := TaskStatus(cells[2])
		if !st.Valid() {
			return nil, nil, parseErr(name, "task row %d: invalid status %q", i+1, cells[2])
		}
		var deps []string
		if cells[3] != Empty {
			for _, d := range strings.Split(cells[3], ",") {
				deps = append(deps, strings.TrimSpace(d))
			}
		}
		rows = append(rows, TaskRow{
			ID: text, Href: href, Title: cells[1], Status: st,
			Depends: deps, Updated: cells[4], Notes: cells[5],
		})
	}
	return t, rows, nil
}

// ChangeLedger is the parsed changes/<id>/ledger.md.
type ChangeLedger struct {
	taskTable
	ChangeID    string
	Branch      string
	Overall     OverallStatus
	LastUpdated string
}

// ParseChangeLedger parses a per-change ledger strictly against the pinned schema.
func ParseChangeLedger(name string, data []byte) (*ChangeLedger, error) {
	lines := strings.Split(string(data), "\n")
	l := &ChangeLedger{taskTable: taskTable{doc: name, Lines: lines}}
	for _, ln := range lines {
		switch {
		case strings.HasPrefix(ln, "- Change ID:"):
			l.ChangeID = strings.TrimSpace(strings.TrimPrefix(ln, "- Change ID:"))
		case strings.HasPrefix(ln, "- Branch:"):
			l.Branch = strings.TrimSpace(strings.TrimPrefix(ln, "- Branch:"))
		case strings.HasPrefix(ln, "- Overall status:"):
			l.Overall = OverallStatus(strings.TrimSpace(strings.TrimPrefix(ln, "- Overall status:")))
		case strings.HasPrefix(ln, "- Last updated:"):
			l.LastUpdated = strings.TrimSpace(strings.TrimPrefix(ln, "- Last updated:"))
		}
	}
	if l.ChangeID == "" {
		return nil, parseErr(name, "missing '- Change ID:' header")
	}
	if !l.Overall.Valid() {
		return nil, parseErr(name, "missing or invalid '- Overall status:' header")
	}
	t, rows, err := parseTaskRows(lines, name)
	if err != nil {
		return nil, err
	}
	l.Table, l.Rows = t, rows
	return l, nil
}

// taskHeaderRe matches the container ledger's "- Task:" value:
// "<task-id> (change <change-id>)".
var taskHeaderRe = regexp.MustCompile(`^(\S+) \(change (\S+)\)$`)

// TaskLedger is the parsed ledger.md of a decomposed task's container
// (tasks/<NN-slug>/ledger.md), per the AGENTS.md decomposition rules.
type TaskLedger struct {
	taskTable
	TaskID      string
	ChangeID    string
	LastUpdated string
}

// ParseTaskLedger parses a task-container ledger strictly: the pinned
// "- Task: <id> (change <change-id>)" header, an optional "- Last updated:"
// header, and the same pinned task table as a change ledger.
func ParseTaskLedger(name string, data []byte) (*TaskLedger, error) {
	lines := strings.Split(string(data), "\n")
	l := &TaskLedger{taskTable: taskTable{doc: name, Lines: lines}}
	for _, ln := range lines {
		switch {
		case strings.HasPrefix(ln, "- Task:"):
			m := taskHeaderRe.FindStringSubmatch(strings.TrimSpace(strings.TrimPrefix(ln, "- Task:")))
			if m == nil {
				return nil, parseErr(name, "malformed '- Task:' header (want '<task-id> (change <change-id>)')")
			}
			l.TaskID, l.ChangeID = m[1], m[2]
		case strings.HasPrefix(ln, "- Last updated:"):
			l.LastUpdated = strings.TrimSpace(strings.TrimPrefix(ln, "- Last updated:"))
		}
	}
	if l.TaskID == "" || l.ChangeID == "" {
		return nil, parseErr(name, "missing '- Task: <id> (change <change-id>)' header")
	}
	t, rows, err := parseTaskRows(lines, name)
	if err != nil {
		return nil, err
	}
	l.Table, l.Rows = t, rows
	return l, nil
}

// Row returns the task row with the given ID, or nil.
func (tt *taskTable) Row(id string) *TaskRow {
	for i := range tt.Rows {
		if tt.Rows[i].ID == id {
			return &tt.Rows[i]
		}
	}
	return nil
}
