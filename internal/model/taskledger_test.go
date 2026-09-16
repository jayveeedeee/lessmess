package model

import (
	"strings"
	"testing"
)

func TestParseTaskLedger(t *testing.T) {
	l, err := ParseTaskLedger("task_ledger.md", load(t, "testdata/task_ledger.md"))
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	if l.TaskID != "EXC-00" || l.ChangeID != "2026-09-16-q7t4k" {
		t.Errorf("TaskID/ChangeID = %q/%q", l.TaskID, l.ChangeID)
	}
	if l.LastUpdated != "2026-09-16" {
		t.Errorf("LastUpdated = %q", l.LastUpdated)
	}
	if len(l.Rows) != 3 {
		t.Fatalf("rows = %d, want 3", len(l.Rows))
	}
	if l.Rows[0].ID != "EXC-00.00" || l.Rows[0].Status != StatusInProgress {
		t.Errorf("row 0 = %+v", l.Rows[0])
	}
	if deps := l.Rows[2].Depends; len(deps) != 1 || deps[0] != "EXC-00.01" {
		t.Errorf("EXC-00.02 depends = %v", deps)
	}
	if got := l.Row("EXC-00.01"); got == nil || got.Title != "Column mapping" {
		t.Errorf("Row(EXC-00.01) = %+v", got)
	}
	// The status-definitions table must not be mistaken for the task table.
	if l.Table.Cols[0] != "Task" {
		t.Errorf("matched table cols = %v, want task schema", l.Table.Cols)
	}
}

func TestParseTaskLedgerBadHeaders(t *testing.T) {
	orig := string(load(t, "testdata/task_ledger.md"))

	cases := []struct {
		name string
		body string
		want string
	}{
		{"missing task header", strings.Replace(orig, "- Task: EXC-00 (change 2026-09-16-q7t4k)", "", 1), "- Task:"},
		{"malformed task header", strings.Replace(orig, "- Task: EXC-00 (change 2026-09-16-q7t4k)", "- Task: EXC-00", 1), "malformed '- Task:'"},
		{"invalid status", strings.Replace(orig, "Streaming CSV writer | In progress", "Streaming CSV writer | Weird", 1), "invalid status"},
		{"pipe in cell", strings.Replace(orig, "| Golden-file fixtures |", "| Golden | file |", 1), "want 6"},
	}
	for _, tc := range cases {
		if _, err := ParseTaskLedger("bad.md", []byte(tc.body)); err == nil {
			t.Errorf("%s: expected error", tc.name)
		} else if !strings.Contains(err.Error(), tc.want) {
			t.Errorf("%s: err = %v, want %q", tc.name, err, tc.want)
		}
	}
}

func TestTaskLedgerMutationRoundTrip(t *testing.T) {
	orig := load(t, "testdata/task_ledger.md")
	l, err := ParseTaskLedger("task_ledger.md", orig)
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	outside := outsideTable(l.Lines, l.Table)

	// Move EXC-00.01 into "In progress" at the top of its group, then
	// append a new dotted task.
	if err := l.MoveTask("EXC-00.01", StatusInProgress, 0, "2026-09-17"); err != nil {
		t.Fatalf("move: %v", err)
	}
	l.AppendTask(TaskRow{
		ID: "EXC-00.03", Href: "tasks/03-docs.md", Title: "Document the writer",
		Status: StatusNotStarted, Updated: "2026-09-17", Notes: Empty,
	})
	if outsideTable(l.Lines, l.Table) != outside {
		t.Fatal("non-table content changed by mutations")
	}

	l2, err := ParseTaskLedger("task_ledger.md", l.Content())
	if err != nil {
		t.Fatalf("reparse: %v", err)
	}
	row := l2.Row("EXC-00.01")
	if row == nil || row.Status != StatusInProgress || row.Updated != "2026-09-17" {
		t.Fatalf("row after move = %+v", row)
	}
	pos := map[string]int{}
	for i, r := range l2.Rows {
		pos[r.ID] = i
	}
	if pos["EXC-00.01"] > pos["EXC-00.00"] {
		t.Errorf("EXC-00.01 at %d after EXC-00.00 at %d; want 01 first in group", pos["EXC-00.01"], pos["EXC-00.00"])
	}
	if pos["EXC-00.03"] != len(l2.Rows)-1 {
		t.Errorf("appended row at %d, want last", pos["EXC-00.03"])
	}
	if err := l2.MoveTask("EXC-99", StatusDone, 0, "2026-09-17"); err == nil {
		t.Error("expected ErrTaskNotFound for unknown child")
	}
}

func TestRenderTaskLedgerRoundTrip(t *testing.T) {
	data := RenderTaskLedger("EXC-00", "2026-09-16-q7t4k", "2026-09-16")
	l, err := ParseTaskLedger("rendered.md", data)
	if err != nil {
		t.Fatalf("parse rendered: %v", err)
	}
	if l.TaskID != "EXC-00" || l.ChangeID != "2026-09-16-q7t4k" {
		t.Errorf("TaskID/ChangeID = %q/%q", l.TaskID, l.ChangeID)
	}
	if len(l.Rows) != 0 {
		t.Errorf("rows = %d, want 0", len(l.Rows))
	}
	// A row appended to the rendered skeleton must survive a round trip.
	l.AppendTask(TaskRow{
		ID: "EXC-00.00", Href: "tasks/00-child.md", Title: "Child",
		Status: StatusNotStarted, Updated: "2026-09-16", Notes: Empty,
	})
	if _, err := ParseTaskLedger("rendered.md", l.Content()); err != nil {
		t.Fatalf("reparse after append: %v", err)
	}
}
