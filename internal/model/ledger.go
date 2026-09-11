package model

import "strings"

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

// ChangeLedger is the parsed changes/<id>/ledger.md.
type ChangeLedger struct {
	Lines       []string
	Table       *Table
	ChangeID    string
	Branch      string
	Overall     OverallStatus
	LastUpdated string
	Rows        []TaskRow
}

// ParseChangeLedger parses a per-change ledger strictly against the pinned schema.
func ParseChangeLedger(name string, data []byte) (*ChangeLedger, error) {
	lines := strings.Split(string(data), "\n")
	l := &ChangeLedger{Lines: lines}
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
	t, err := FindTable(lines, TaskColumns, name)
	if err != nil {
		return nil, err
	}
	l.Table = t
	for i, cells := range t.Rows {
		text, href, ok := ParseLink(cells[0])
		if !ok {
			return nil, parseErr(name, "task row %d: Task cell is not a link", i+1)
		}
		st := TaskStatus(cells[2])
		if !st.Valid() {
			return nil, parseErr(name, "task row %d: invalid status %q", i+1, cells[2])
		}
		var deps []string
		if cells[3] != Empty {
			for _, d := range strings.Split(cells[3], ",") {
				deps = append(deps, strings.TrimSpace(d))
			}
		}
		l.Rows = append(l.Rows, TaskRow{
			ID: text, Href: href, Title: cells[1], Status: st,
			Depends: deps, Updated: cells[4], Notes: cells[5],
		})
	}
	return l, nil
}

// Row returns the task row with the given ID, or nil.
func (l *ChangeLedger) Row(id string) *TaskRow {
	for i := range l.Rows {
		if l.Rows[i].ID == id {
			return &l.Rows[i]
		}
	}
	return nil
}
