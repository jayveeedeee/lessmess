// Package model parses and serializes the markdown files of the
// changes/ workflow defined in AGENTS.md.
package model

import "fmt"

// TaskStatus is the execution status of a task row.
type TaskStatus string

const (
	StatusNotStarted TaskStatus = "Not started"
	StatusInProgress TaskStatus = "In progress"
	StatusBlocked    TaskStatus = "Blocked"
	StatusDone       TaskStatus = "Done"
	StatusCancelled  TaskStatus = "Cancelled"
)

// TaskStatusOrder is the kanban column order.
var TaskStatusOrder = []TaskStatus{StatusNotStarted, StatusInProgress, StatusBlocked, StatusDone, StatusCancelled}

func (s TaskStatus) Valid() bool {
	for _, v := range TaskStatusOrder {
		if s == v {
			return true
		}
	}
	return false
}

// OverallStatus is the status of a change as a whole.
type OverallStatus string

const (
	OverallPlanned    OverallStatus = "Planned"
	OverallInProgress OverallStatus = "In progress"
	OverallBlocked    OverallStatus = "Blocked"
	OverallDone       OverallStatus = "Done"
	OverallCancelled  OverallStatus = "Cancelled"
)

func (s OverallStatus) Valid() bool {
	switch s {
	case OverallPlanned, OverallInProgress, OverallBlocked, OverallDone, OverallCancelled:
		return true
	}
	return false
}

// Empty marks an empty table cell per the AGENTS.md schema.
const Empty = "—"

// Error describes a parse problem in a workflow file.
type Error struct {
	File string
	Msg  string
}

func (e *Error) Error() string { return fmt.Sprintf("%s: %s", e.File, e.Msg) }

func parseErr(file, format string, args ...any) *Error {
	return &Error{File: file, Msg: fmt.Sprintf(format, args...)}
}
