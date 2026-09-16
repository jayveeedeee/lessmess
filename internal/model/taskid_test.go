package model

import "testing"

func TestTaskIDHelpers(t *testing.T) {
	cases := []struct {
		id       string
		parent   string
		last     string
		depth    int
		segments bool
	}{
		{"EXC-00", "", "00", 1, true},
		{"00", "", "00", 1, true},
		{"EXC-00.00", "EXC-00", "00", 2, true},
		{"EXC-00.01.02", "EXC-00.01", "02", 3, true},
		{"EXC-00.1", "EXC-00", "1", 2, false},     // not zero-padded
		{"EXC-00.001", "EXC-00", "001", 2, false}, // three digits
		{"EXC-00.a1", "EXC-00", "a1", 2, false},   // not numeric
	}
	for _, tc := range cases {
		if got := ParentTaskID(tc.id); got != tc.parent {
			t.Errorf("ParentTaskID(%q) = %q, want %q", tc.id, got, tc.parent)
		}
		if got := LastTaskSegment(tc.id); got != tc.last {
			t.Errorf("LastTaskSegment(%q) = %q, want %q", tc.id, got, tc.last)
		}
		if got := TaskIDDepth(tc.id); got != tc.depth {
			t.Errorf("TaskIDDepth(%q) = %d, want %d", tc.id, got, tc.depth)
		}
		if got := DottedSegmentsValid(tc.id); got != tc.segments {
			t.Errorf("DottedSegmentsValid(%q) = %v, want %v", tc.id, got, tc.segments)
		}
	}
}

func TestChildTaskID(t *testing.T) {
	if got := ChildTaskID("EXC-00", "01"); got != "EXC-00.01" {
		t.Errorf("ChildTaskID = %q", got)
	}
	if got := ChildTaskID(ChildTaskID("EXC-00", "00"), "02"); got != "EXC-00.00.02" {
		t.Errorf("nested ChildTaskID = %q", got)
	}
}
