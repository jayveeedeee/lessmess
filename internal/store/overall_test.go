package store

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"lessmess/internal/model"
)

// setFixtureStatuses rewrites the fixture change's state so both tasks
// have the given statuses, then reloads the store.
func setFixtureStatuses(t *testing.T, s *Store, a, b model.TaskStatus, overall model.OverallStatus) *Store {
	t.Helper()
	p := filepath.Join(s.Dir, StateDirName, "workflow", "changes", "2026-09-10-0.json")
	st, err := model.LoadChangeState(p)
	if err != nil {
		t.Fatal(err)
	}
	st.Tasks[0].Status = a
	st.Tasks[1].Status = b
	st.Status = model.ChangeStatus{Value: overall, Derived: derivableOverall(overall)}
	if err := st.Save(p); err != nil {
		t.Fatal(err)
	}
	s.Reload()
	return s
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

	// First move flips the change to In progress.
	if err := s.MoveTask("2026-09-10-0", "FIX-00", model.StatusInProgress, 0); err != nil {
		t.Fatal(err)
	}
	c, _ := s.Change("2026-09-10-0")
	if c.Overall() != model.OverallInProgress {
		t.Fatalf("overall after first move = %q, want In progress", c.Overall())
	}

	// Everything complete → stays In progress (ready for close, not Done).
	if err := s.MoveTask("2026-09-10-0", "FIX-00", model.StatusTest, 0); err != nil {
		t.Fatal(err)
	}
	if err := s.MoveTask("2026-09-10-0", "FIX-01", model.StatusTest, 0); err != nil {
		t.Fatal(err)
	}
	c, _ = s.Change("2026-09-10-0")
	if c.Overall() != model.OverallInProgress {
		t.Fatalf("complete tree overall = %q, want In progress", c.Overall())
	}

	// All tasks back to Not started (external edit) → Planned again.
	setFixtureStatuses(t, s, model.StatusNotStarted, model.StatusNotStarted, model.OverallInProgress)
	s.SyncOverallStatuses()
	c, _ = s.Change("2026-09-10-0")
	if c.Overall() != model.OverallPlanned {
		t.Fatalf("reset tree overall = %q, want Planned", c.Overall())
	}
	// …and the converged status is on disk.
	p := filepath.Join(s.Dir, StateDirName, "workflow", "changes", "2026-09-10-0.json")
	data, err := os.ReadFile(p)
	if err != nil {
		t.Fatal(err)
	}
	if want := `"value": "Planned"`; !strings.Contains(string(data), want) {
		t.Errorf("state on disk missing %q", want)
	}
}
