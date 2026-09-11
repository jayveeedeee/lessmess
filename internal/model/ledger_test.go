package model

import (
	"os"
	"strings"
	"testing"
)

func load(t *testing.T, path string) []byte {
	t.Helper()
	b, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read fixture: %v", err)
	}
	return b
}

func TestParseRootLedger(t *testing.T) {
	l, err := ParseRootLedger("root_ledger.md", load(t, "testdata/root_ledger.md"))
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	if len(l.Rows) != 2 {
		t.Fatalf("rows = %d, want 2", len(l.Rows))
	}
	r := l.Rows[0]
	if r.Change != "2026-09-11-0" || r.Prefix != "CHW" || r.Status != OverallDone {
		t.Errorf("row 0 = %+v", r)
	}
	if r.Href != "2026-09-11-0/plan.md" {
		t.Errorf("href = %q", r.Href)
	}
	if l.Rows[1].Status != OverallPlanned || l.Rows[1].Prefix != "KAN" {
		t.Errorf("row 1 = %+v", l.Rows[1])
	}
}

func TestParseRootLedgerBadColumn(t *testing.T) {
	bad := strings.Replace(string(load(t, "testdata/root_ledger.md")), "| Change |", "| Name |", 1)
	if _, err := ParseRootLedger("bad.md", []byte(bad)); err == nil {
		t.Fatal("expected error for renamed column")
	} else if !strings.Contains(err.Error(), "not found") {
		t.Fatalf("err = %v", err)
	}
}

func TestParseRootLedgerBadStatus(t *testing.T) {
	bad := strings.Replace(string(load(t, "testdata/root_ledger.md")), "| Done |", "| Doing |", 1)
	if _, err := ParseRootLedger("bad.md", []byte(bad)); err == nil {
		t.Fatal("expected error for invalid status")
	} else if !strings.Contains(err.Error(), "invalid status") {
		t.Fatalf("err = %v", err)
	}
}

func TestParseRootLedgerPipeInCell(t *testing.T) {
	bad := strings.Replace(string(load(t, "testdata/root_ledger.md")),
		"Bootstrap change-management workflow extensions",
		"Bootstrap | workflow extensions", 1)
	if _, err := ParseRootLedger("bad.md", []byte(bad)); err == nil {
		t.Fatal("expected error for literal pipe in cell")
	} else if !strings.Contains(err.Error(), "literal |") {
		t.Fatalf("err = %v", err)
	}
}

func TestParseChangeLedger(t *testing.T) {
	l, err := ParseChangeLedger("change_ledger.md", load(t, "testdata/change_ledger.md"))
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	if l.ChangeID != "2026-09-11-1" {
		t.Errorf("ChangeID = %q", l.ChangeID)
	}
	if l.Overall != OverallInProgress {
		t.Errorf("Overall = %q", l.Overall)
	}
	if len(l.Rows) != 8 {
		t.Fatalf("rows = %d, want 8", len(l.Rows))
	}
	for i, r := range l.Rows {
		wantID := "KAN-0" + string(rune('0'+i))
		if r.ID != wantID {
			t.Errorf("row %d ID = %q, want %q", i, r.ID, wantID)
		}
		if !r.Status.Valid() {
			t.Errorf("row %d invalid status %q", i, r.Status)
		}
	}
	if deps := l.Rows[1].Depends; len(deps) != 1 || deps[0] != "KAN-00" {
		t.Errorf("KAN-01 depends = %v, want [KAN-00]", deps)
	}
	if deps := l.Rows[7].Depends; len(deps) != 2 || deps[0] != "KAN-05" || deps[1] != "KAN-06" {
		t.Errorf("KAN-07 depends = %v, want [KAN-05 KAN-06]", deps)
	}
	if got := l.Row("KAN-03"); got == nil || got.Title == "" {
		t.Errorf("Row(KAN-03) = %+v", got)
	}
}

func TestParseChangeLedgerMissingMeta(t *testing.T) {
	bad := strings.Replace(string(load(t, "testdata/change_ledger.md")), "- Overall status: In progress", "", 1)
	if _, err := ParseChangeLedger("bad.md", []byte(bad)); err == nil {
		t.Fatal("expected error for missing overall status")
	} else if !strings.Contains(err.Error(), "Overall status") {
		t.Fatalf("err = %v", err)
	}
}

func TestParseChangeLedgerIgnoresStatusDefinitionsTable(t *testing.T) {
	// The status-definitions table (Status | Meaning) must not be
	// mistaken for the task table.
	l, err := ParseChangeLedger("change_ledger.md", load(t, "testdata/change_ledger.md"))
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	if l.Table.Cols[0] != "Task" {
		t.Errorf("matched table cols = %v, want task schema", l.Table.Cols)
	}
}

func TestParseTaskFile(t *testing.T) {
	f, err := ParseTaskFile("task_file.md", load(t, "testdata/task_file.md"))
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	if f.ID != "KAN-00" || f.Title != "Project scaffold and CLI entry" {
		t.Errorf("meta = %+v", f)
	}
	if !strings.Contains(f.Body, "## Objective") {
		t.Errorf("body missing skeleton headings")
	}
}

func TestParseTaskFileNoFrontmatter(t *testing.T) {
	if _, err := ParseTaskFile("bad.md", []byte("# No frontmatter\n")); err == nil {
		t.Fatal("expected error")
	}
}

func TestParseTaskFileUnknownKey(t *testing.T) {
	doc := "---\nid: KAN-00\ntitle: X\nstatus: Done\n---\n\nbody\n"
	if _, err := ParseTaskFile("bad.md", []byte(doc)); err == nil {
		t.Fatal("expected error for unknown frontmatter key")
	} else if !strings.Contains(err.Error(), "only id and title") {
		t.Fatalf("err = %v", err)
	}
}

func TestParseTaskFileMissingTitle(t *testing.T) {
	doc := "---\nid: KAN-00\n---\n\nbody\n"
	if _, err := ParseTaskFile("bad.md", []byte(doc)); err == nil {
		t.Fatal("expected error for missing title")
	}
}

func TestParseTaskFileClosingFenceAtEOF(t *testing.T) {
	doc := "---\nid: KAN-00\ntitle: X\n---"
	f, err := ParseTaskFile("ok.md", []byte(doc))
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	if f.ID != "KAN-00" {
		t.Errorf("id = %q", f.ID)
	}
}
