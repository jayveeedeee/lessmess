package store

import (
	"os"
	"path/filepath"
	"testing"

	"lessmess/internal/model"
)

// setFixtureStatuses rewrites the fixture change's ledger so both tasks
// have the given statuses, then reopens the store.
func setFixtureStatuses(t *testing.T, s *Store, a, b model.TaskStatus, overall model.OverallStatus) *Store {
	t.Helper()
	cdir := filepath.Join(s.Dir, "changes", "2026-09-10-0")
	l, err := model.ParseChangeLedger("ledger.md", readFile(t, filepath.Join(cdir, "ledger.md")))
	if err != nil {
		t.Fatal(err)
	}
	l.MoveTask("FIX-00", a, 0, "2026-09-10")
	l.MoveTask("FIX-01", b, 0, "2026-09-10")
	l.SetOverall(overall, "2026-09-10")
	if err := os.WriteFile(filepath.Join(cdir, "ledger.md"), l.Content(), 0o644); err != nil {
		t.Fatal(err)
	}
	s.Reload()
	return s
}

func rootRowStatus(t *testing.T, s *Store, id string) model.OverallStatus {
	t.Helper()
	root, err := s.Root()
	if err != nil {
		t.Fatal(err)
	}
	for _, r := range root.Rows {
		if r.Change == id {
			return r.Status
		}
	}
	t.Fatal("root row missing")
	return ""
}

func TestDeriveOverallTable(t *testing.T) {
	s, _ := openFixture(t)
	// Fixture: FIX-00 Test, FIX-01 Test → started, nothing open.
	setFixtureStatuses(t, s, model.StatusTest, model.StatusTest, model.OverallInProgress)
	c, _ := s.Change("2026-09-10-0")
	if got := DeriveOverall(c); got != model.OverallInProgress {
		t.Errorf("complete tree derives %q, want In progress (Done is user-gated)", got)
	}
	// All Not started → Planned.
	setFixtureStatuses(t, s, model.StatusNotStarted, model.StatusNotStarted, model.OverallPlanned)
	c, _ = s.Change("2026-09-10-0")
	if got := DeriveOverall(c); got != model.OverallPlanned {
		t.Errorf("fresh tree derives %q, want Planned", got)
	}
	// One Blocked open task, rest complete → Blocked.
	setFixtureStatuses(t, s, model.StatusTest, model.StatusBlocked, model.OverallInProgress)
	c, _ = s.Change("2026-09-10-0")
	if got := DeriveOverall(c); got != model.OverallBlocked {
		t.Errorf("all-open-blocked derives %q, want Blocked", got)
	}
	// Open Not started + started work → In progress.
	setFixtureStatuses(t, s, model.StatusNotStarted, model.StatusInProgress, model.OverallInProgress)
	c, _ = s.Change("2026-09-10-0")
	if got := DeriveOverall(c); got != model.OverallInProgress {
		t.Errorf("mixed tree derives %q, want In progress", got)
	}
}

func TestSyncOverallOnMove(t *testing.T) {
	s, _ := openFixture(t)
	setFixtureStatuses(t, s, model.StatusNotStarted, model.StatusNotStarted, model.OverallPlanned)

	// First move flips the change (and root row) to In progress.
	if err := s.MoveTask("2026-09-10-0", "FIX-00", model.StatusInProgress, 0); err != nil {
		t.Fatal(err)
	}
	c, _ := s.Change("2026-09-10-0")
	if c.Ledger.Overall != model.OverallInProgress {
		t.Fatalf("overall after first move = %q, want In progress", c.Ledger.Overall)
	}
	if rootRowStatus(t, s, "2026-09-10-0") != model.OverallInProgress {
		t.Fatal("root row not synced")
	}

	// Everything complete → stays In progress (ready for close, not Done).
	if err := s.MoveTask("2026-09-10-0", "FIX-00", model.StatusTest, 0); err != nil {
		t.Fatal(err)
	}
	if err := s.MoveTask("2026-09-10-0", "FIX-01", model.StatusTest, 0); err != nil {
		t.Fatal(err)
	}
	c, _ = s.Change("2026-09-10-0")
	if c.Ledger.Overall != model.OverallInProgress {
		t.Fatalf("complete tree overall = %q, want In progress", c.Ledger.Overall)
	}

	// Reopening one task → back to In progress is a no-op; making it all
	// blocked derives Blocked.
	if err := s.MoveTask("2026-09-10-0", "FIX-01", model.StatusBlocked, 0); err != nil {
		t.Fatal(err)
	}
	c, _ = s.Change("2026-09-10-0")
	if c.Ledger.Overall != model.OverallBlocked {
		t.Fatalf("blocked-open overall = %q, want Blocked", c.Ledger.Overall)
	}
}

func TestSyncOverallPreservesDone(t *testing.T) {
	s, _ := openFixture(t)
	setFixtureStatuses(t, s, model.StatusTest, model.StatusTest, model.OverallInProgress)
	if err := s.SetChangeStatus("2026-09-10-0", model.OverallDone); err != nil {
		t.Fatal(err)
	}

	// A sync (as the watcher runs it) must not downgrade a complete Done change.
	s.SyncOverallStatuses()
	c, _ := s.Change("2026-09-10-0")
	if c.Ledger.Overall != model.OverallDone {
		t.Fatalf("Done downgraded to %q", c.Ledger.Overall)
	}

	// New open work flips it back to In progress implicitly.
	if err := s.MoveTask("2026-09-10-0", "FIX-01", model.StatusInProgress, 0); err != nil {
		t.Fatal(err)
	}
	c, _ = s.Change("2026-09-10-0")
	if c.Ledger.Overall != model.OverallInProgress {
		t.Fatalf("overall after reopening work = %q, want In progress", c.Ledger.Overall)
	}
	if rootRowStatus(t, s, "2026-09-10-0") != model.OverallInProgress {
		t.Fatal("root row not synced on implicit reopen")
	}
}

func TestSyncOverallConvergesExternalEdits(t *testing.T) {
	s, _ := openFixture(t)
	setFixtureStatuses(t, s, model.StatusTest, model.StatusTest, model.OverallInProgress)
	// An agent hand-edits the ledger header to a status that no longer
	// matches the board (all tasks complete, header says Planned).
	cdir := filepath.Join(s.Dir, "changes", "2026-09-10-0")
	rewrite(t, filepath.Join(cdir, "ledger.md"), "- Overall status: In progress", "- Overall status: Planned")
	rewrite(t, filepath.Join(s.Dir, "changes", "ledger.md"),
		"| [2026-09-10-0](2026-09-10-0/plan.md) | Fixture change | FIX | — | In progress",
		"| [2026-09-10-0](2026-09-10-0/plan.md) | Fixture change | FIX | — | Planned")
	s.Reload()

	s.SyncOverallStatuses()
	c, _ := s.Change("2026-09-10-0")
	if c.Ledger.Overall != model.OverallInProgress {
		t.Fatalf("external drift not corrected: %q", c.Ledger.Overall)
	}
	if rootRowStatus(t, s, "2026-09-10-0") != model.OverallInProgress {
		t.Fatal("root row drift not corrected")
	}
	// Validation stays clean (rule 6 agreement).
	if v := s.Validate(); len(v) != 0 {
		t.Fatalf("violations after sync: %v", v)
	}
}
