package store

import (
	"log/slog"

	"lessmess/internal/model"
)

// DeriveOverall computes a change's overall status from its task tree:
//
//   - Planned: nothing has started yet (all tasks Not started, or none).
//   - Blocked: work has started and every remaining open task is Blocked.
//   - In progress: anything else, including a fully complete tree (close
//     remains user-gated per AGENTS.md rule 9, so completeness does not
//     imply Done).
//
// Open work means non-cancelled tasks that are not Test or Done.
func DeriveOverall(c *Change) model.OverallStatus {
	started, open, openBlocked := false, 0, 0
	c.WalkTasks(func(n *TaskNode) bool {
		st := n.NodeStatus()
		if st == model.StatusCancelled {
			return true
		}
		switch st {
		case model.StatusInProgress, model.StatusTest, model.StatusDone:
			started = true
		case model.StatusBlocked:
			open++
			openBlocked++
		case model.StatusNotStarted:
			open++
		}
		return true
	})
	switch {
	case !started:
		return model.OverallPlanned // no tasks, or all Not started
	case open == 0:
		return model.OverallInProgress // complete: ready for user close
	case open == openBlocked:
		return model.OverallBlocked
	default:
		return model.OverallInProgress
	}
}

// syncOverall re-derives one change's overall status and, on drift, writes
// it to both ledgers via SetChangeStatus (atomic, root row included).
// A Done change is never downgraded while its tree is still complete —
// Done stays user-gated — but new open work flips it back to In progress.
func (s *Store) syncOverall(changeID string) {
	c, err := s.Change(changeID)
	if err != nil || c.Ledger == nil {
		return
	}
	stored := c.Ledger.Overall
	derived := DeriveOverall(c)
	if stored == derived {
		return
	}
	if stored == model.OverallDone && derived == model.OverallInProgress {
		// Done with a complete tree stays Done; Done with open work
		// (someone moved a task back or added one) reopens implicitly.
		if ready, _, err := s.CloseOutReady(changeID); err == nil && ready {
			return
		}
	}
	if err := s.SetChangeStatus(changeID, derived); err != nil {
		slog.Warn("overall status sync", "change", changeID, "err", err)
		return
	}
	slog.Info("overall status derived", "change", changeID, "status", string(derived))
}

// SyncOverallStatuses converges every active change's overall status with
// its task tree. Called after watcher reloads so external (agent) ledger
// edits drift back automatically.
func (s *Store) SyncOverallStatuses() {
	for _, c := range s.Changes() {
		s.syncOverall(c.ID)
	}
}
