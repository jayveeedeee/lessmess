package store

import (
	"lessmess/internal/model"
)

// DeriveOverall computes a change's overall status from its task tree:
//
//   - Planned: nothing has started yet (all tasks Not started, or none).
//   - Blocked: work has started and every remaining open task is Blocked.
//   - In progress: anything else, including a fully complete tree (close
//     remains user-gated, so completeness does not imply Done).
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

// CloseOutReady reports whether a change may be closed by the user:
// every non-cancelled task at any depth must be Test or Done. It returns
// the offending task IDs otherwise.
func (s *Store) CloseOutReady(id string) (bool, []string, error) {
	c, err := s.Change(id)
	if err != nil {
		return false, nil, err
	}
	offending := c.CloseOutReady()
	return len(offending) == 0, offending, nil
}

// CloseOutReady returns the IDs of every non-cancelled task at any depth
// that is not Test or Done. An empty result means the change may be
// closed by the user.
func (c *Change) CloseOutReady() []string {
	var offenders []string
	c.WalkTasks(func(n *TaskNode) bool {
		st := n.NodeStatus()
		if st != model.StatusCancelled && st != model.StatusTest && st != model.StatusDone {
			offenders = append(offenders, n.ID)
		}
		return true
	})
	return offenders
}
