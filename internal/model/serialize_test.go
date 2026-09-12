package model

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// outsideTable returns the document content with the table region removed,
// used to prove mutations never touch non-table content.
func outsideTable(lines []string, t *Table) string {
	out := Splice(lines, t.Start, t.End, nil)
	return strings.Join(out, "\n")
}

func TestChangeLedgerMoveTaskRoundTrip(t *testing.T) {
	orig := load(t, "testdata/change_ledger.md")
	l, err := ParseChangeLedger("change_ledger.md", orig)
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	outside := outsideTable(l.Lines, l.Table)

	// Move KAN-02 into "In progress" at top.
	if err := l.MoveTask("KAN-02", StatusInProgress, 0, "2026-09-12"); err != nil {
		t.Fatalf("move: %v", err)
	}
	if outsideTable(l.Lines, l.Table) != outside {
		t.Fatal("non-table content changed by MoveTask")
	}

	// Reparse and verify.
	l2, err := ParseChangeLedger("change_ledger.md", l.Content())
	if err != nil {
		t.Fatalf("reparse: %v", err)
	}
	row := l2.Row("KAN-02")
	if row == nil || row.Status != StatusInProgress || row.Updated != "2026-09-12" {
		t.Fatalf("row after move = %+v", row)
	}
	// KAN-01 is In progress at index 1 in the fixture; KAN-02 inserted at
	// visual index 0 of the group must come before it.
	pos := map[string]int{}
	for i, r := range l2.Rows {
		pos[r.ID] = i
	}
	if pos["KAN-02"] > pos["KAN-01"] {
		t.Errorf("KAN-02 at %d, KAN-01 at %d; want KAN-02 first", pos["KAN-02"], pos["KAN-01"])
	}
	if len(l2.Rows) != 8 {
		t.Errorf("rows = %d, want 8", len(l2.Rows))
	}
}

func TestChangeLedgerMoveTaskToEmptyColumn(t *testing.T) {
	l, err := ParseChangeLedger("change_ledger.md", load(t, "testdata/change_ledger.md"))
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	if err := l.MoveTask("KAN-04", StatusBlocked, 0, "2026-09-12"); err != nil {
		t.Fatalf("move: %v", err)
	}
	l2, err := ParseChangeLedger("change_ledger.md", l.Content())
	if err != nil {
		t.Fatalf("reparse: %v", err)
	}
	row := l2.Row("KAN-04")
	if row == nil || row.Status != StatusBlocked {
		t.Fatalf("row = %+v", row)
	}
	if len(l2.Rows) != 8 {
		t.Errorf("rows = %d, want 8", len(l2.Rows))
	}
}

func TestMoveTaskReorderWithinColumn(t *testing.T) {
	l, err := ParseChangeLedger("change_ledger.md", load(t, "testdata/change_ledger.md"))
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	// Move KAN-00 (Not started, row 0) below KAN-02 in the same column.
	if err := l.MoveTask("KAN-00", StatusNotStarted, 2, "2026-09-12"); err != nil {
		t.Fatalf("move: %v", err)
	}
	l2, _ := ParseChangeLedger("change_ledger.md", l.Content())
	pos := map[string]int{}
	for i, r := range l2.Rows {
		if r.Status == StatusNotStarted {
			pos[r.ID] = i
		}
	}
	if !(pos["KAN-01"] < pos["KAN-02"] && pos["KAN-02"] < pos["KAN-00"]) {
		t.Errorf("Not started order wrong: %v", pos)
	}
}

func TestMoveTaskNotFound(t *testing.T) {
	l, _ := ParseChangeLedger("change_ledger.md", load(t, "testdata/change_ledger.md"))
	if err := l.MoveTask("NOPE-99", StatusDone, 0, "2026-09-12"); err != ErrTaskNotFound {
		t.Fatalf("err = %v, want ErrTaskNotFound", err)
	}
}

func TestChangeLedgerMoveTaskToTest(t *testing.T) {
	l, err := ParseChangeLedger("change_ledger.md", load(t, "testdata/change_ledger.md"))
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	if !StatusTest.Valid() {
		t.Fatal("StatusTest must be a valid task status")
	}
	// Test must sit immediately before Done in the kanban column order.
	ti, di := -1, -1
	for i, s := range TaskStatusOrder {
		switch s {
		case StatusTest:
			ti = i
		case StatusDone:
			di = i
		}
	}
	if ti == -1 || di != ti+1 {
		t.Fatalf("TaskStatusOrder = %v; want Test immediately before Done", TaskStatusOrder)
	}
	if err := l.MoveTask("KAN-04", StatusTest, 0, "2026-09-12"); err != nil {
		t.Fatalf("move: %v", err)
	}
	l2, err := ParseChangeLedger("change_ledger.md", l.Content())
	if err != nil {
		t.Fatalf("reparse: %v", err)
	}
	if row := l2.Row("KAN-04"); row == nil || row.Status != StatusTest {
		t.Fatalf("row = %+v", row)
	}
}

func TestChangeLedgerAppendTaskRoundTrip(t *testing.T) {
	orig := load(t, "testdata/change_ledger.md")
	l, _ := ParseChangeLedger("change_ledger.md", orig)
	outside := outsideTable(l.Lines, l.Table)
	l.AppendTask(TaskRow{
		ID: "KAN-08", Href: "tasks/08-new.md", Title: "New task",
		Status: StatusNotStarted, Updated: "2026-09-12", Notes: Empty,
	})
	if outsideTable(l.Lines, l.Table) != outside {
		t.Fatal("non-table content changed by AppendTask")
	}
	l2, err := ParseChangeLedger("change_ledger.md", l.Content())
	if err != nil {
		t.Fatalf("reparse: %v", err)
	}
	if len(l2.Rows) != 9 || l2.Rows[8].ID != "KAN-08" {
		t.Fatalf("rows = %d", len(l2.Rows))
	}
}

func TestRootLedgerAppendAndUpdate(t *testing.T) {
	orig := load(t, "testdata/root_ledger.md")
	l, _ := ParseRootLedger("root_ledger.md", orig)
	outside := outsideTable(l.Lines, l.Table)

	l.AppendRow(RootRow{
		Change: "2026-09-12-0", Href: "2026-09-12-0/plan.md", Title: "Next",
		Prefix: Empty, Branch: Empty, Status: OverallPlanned,
		Created: "2026-09-12", Updated: "2026-09-12",
	})
	if err := l.Update("2026-09-11-1", OverallDone, "2026-09-12"); err != nil {
		t.Fatalf("update: %v", err)
	}
	if outsideTable(l.Lines, l.Table) != outside {
		t.Fatal("non-table content changed")
	}
	l2, err := ParseRootLedger("root_ledger.md", l.Content())
	if err != nil {
		t.Fatalf("reparse: %v", err)
	}
	if len(l2.Rows) != 3 {
		t.Fatalf("rows = %d, want 3", len(l2.Rows))
	}
	if l2.Rows[1].Status != OverallDone || l2.Rows[1].Updated != "2026-09-12" {
		t.Errorf("row 1 = %+v", l2.Rows[1])
	}
	if l2.Rows[2].Change != "2026-09-12-0" {
		t.Errorf("row 2 = %+v", l2.Rows[2])
	}
	if err := l2.Update("nope", OverallDone, "2026-09-12"); err != ErrChangeNotFound {
		t.Errorf("err = %v, want ErrChangeNotFound", err)
	}
}

func TestSetOverall(t *testing.T) {
	l, _ := ParseChangeLedger("change_ledger.md", load(t, "testdata/change_ledger.md"))
	l.SetOverall(OverallDone, "2026-09-12")
	l2, err := ParseChangeLedger("change_ledger.md", l.Content())
	if err != nil {
		t.Fatalf("reparse: %v", err)
	}
	if l2.Overall != OverallDone || l2.LastUpdated != "2026-09-12" {
		t.Errorf("overall = %q updated = %q", l2.Overall, l2.LastUpdated)
	}
}

func TestTemplatesRoundTrip(t *testing.T) {
	tf, err := ParseTaskFile("new.md", RenderTaskFile("KAN-09", "A new task"))
	if err != nil {
		t.Fatalf("task template does not parse: %v", err)
	}
	if tf.ID != "KAN-09" || tf.Title != "A new task" {
		t.Errorf("tf = %+v", tf)
	}
	cl, err := ParseChangeLedger("ledger.md", RenderChangeLedger("2026-09-12-0", "2026-09-12"))
	if err != nil {
		t.Fatalf("ledger template does not parse: %v", err)
	}
	if cl.ChangeID != "2026-09-12-0" || cl.Overall != OverallPlanned || len(cl.Rows) != 0 {
		t.Errorf("cl = %+v rows=%d", cl, len(cl.Rows))
	}
	if !bytes.Contains(RenderChangeLedger("2026-09-12-0", "2026-09-12"), []byte("| Test |")) {
		t.Error("ledger template must define the Test status")
	}
	if !bytes.Contains(RenderChangePlan("2026-09-12-0", "T", "2026-09-12"), []byte("[ledger.md](ledger.md)")) {
		t.Error("plan template must link ledger.md")
	}
}

func TestWriteFileAtomic(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "f.md")
	if err := WriteFileAtomic(path, []byte("hello\n"), 0o644); err != nil {
		t.Fatalf("write: %v", err)
	}
	b, err := os.ReadFile(path)
	if err != nil || string(b) != "hello\n" {
		t.Fatalf("read back = %q, %v", b, err)
	}
	if err := WriteFileAtomic(path, []byte("world\n"), 0o644); err != nil {
		t.Fatalf("rewrite: %v", err)
	}
	b, _ = os.ReadFile(path)
	if string(b) != "world\n" {
		t.Fatalf("read back = %q", b)
	}
	// No temp files left behind.
	entries, _ := os.ReadDir(dir)
	if len(entries) != 1 {
		t.Errorf("dir entries = %d, want 1", len(entries))
	}
}
