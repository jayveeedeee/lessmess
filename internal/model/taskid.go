package model

import (
	"regexp"
	"strings"
)

// Dotted task-ID helpers for the decomposition scheme: a top-level task ID
// is PREFIX-NN (or bare NN); each nesting level appends a ".NN" segment
// ("EXC-00" -> "EXC-00.00" -> "EXC-00.00.01" per AGENTS.md).

// taskSegmentRe matches one two-digit zero-padded child segment.
var taskSegmentRe = regexp.MustCompile(`^\d{2}$`)

// ParentTaskID returns the parent task's ID ("EXC-00.01" -> "EXC-00"), or
// "" for a top-level task.
func ParentTaskID(id string) string {
	if i := strings.LastIndex(id, "."); i >= 0 {
		return id[:i]
	}
	return ""
}

// LastTaskSegment returns the final numeric segment of a task ID: after the
// last dot when the ID is dotted, else after the last dash ("EXC-00.01" ->
// "01", "EXC-00" -> "00", "00" -> "00"). It is the segment that must match
// the task file's filename sequence within its directory.
func LastTaskSegment(id string) string {
	if i := strings.LastIndex(id, "."); i >= 0 {
		return id[i+1:]
	}
	if i := strings.LastIndex(id, "-"); i >= 0 {
		return id[i+1:]
	}
	return id
}

// ChildTaskID appends a child segment to a parent ID ("EXC-00", "01" ->
// "EXC-00.01").
func ChildTaskID(parent, seq string) string { return parent + "." + seq }

// TaskIDDepth returns the nesting depth of a task ID: 1 for top-level,
// +1 per dotted segment.
func TaskIDDepth(id string) int { return 1 + strings.Count(id, ".") }

// DottedSegmentsValid reports whether every dotted segment of the ID is a
// two-digit zero-padded number, as container children are numbered. The
// top-level segment is not constrained here (legacy numeric suffixes vary
// in width).
func DottedSegmentsValid(id string) bool {
	for i, seg := range strings.Split(id, ".") {
		if i == 0 {
			continue
		}
		if !taskSegmentRe.MatchString(seg) {
			return false
		}
	}
	return true
}
