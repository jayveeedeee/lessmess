package gitops

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"

	"lessmess/internal/model"
)

// WorktreesState is the on-disk shape of .lessmess/worktrees.json: one entry
// per change with a worktree. Git (`worktree list`) remains the truth about
// existence; this file records only what git cannot tell us — the PR URL and
// review state — plus the branch and path for fast display. A stale entry
// self-heals: callers probe WorktreeList before trusting Path.
type WorktreesState map[string]Entry

// Review states for a closed change's PR review.
const (
	ReviewNone    = "" // no review attempted yet
	ReviewPending = "pending"
	ReviewDone    = "done"
	ReviewFailed  = "failed"
)

// Entry describes one change's worktree.
type Entry struct {
	Branch      string `json:"branch"`
	Base        string `json:"base,omitempty"` // base branch the change was cut from
	Path        string `json:"path"`
	PRURL       string `json:"prUrl,omitempty"`
	ReviewState string `json:"reviewState,omitempty"`
	Created     string `json:"created"` // RFC3339
}

// stateFileName is the worktree state file inside the state dir, following
// the one-file-per-feature tooling-state convention.
const stateFileName = "worktrees.json"

// statePath locates the state file. An empty StateDir means the client was
// constructed without persistence.
func (c *Client) statePath() string {
	if c.StateDir == "" {
		return ""
	}
	return filepath.Join(c.StateDir, stateFileName)
}

// LoadState reads the worktrees state file. A missing file yields an empty,
// non-nil state; a malformed file is an error (unlike fail-open reads
// elsewhere, writes below depend on this file being parseable).
func LoadState(stateDir string) (WorktreesState, error) {
	st := WorktreesState{}
	if stateDir == "" {
		return st, nil
	}
	b, err := os.ReadFile(filepath.Join(stateDir, stateFileName))
	if errors.Is(err, os.ErrNotExist) {
		return st, nil
	}
	if err != nil {
		return st, err
	}
	if err := json.Unmarshal(b, &st); err != nil {
		return nil, fmt.Errorf("worktrees state malformed: %w", err)
	}
	return st, nil
}

// SaveState writes the state file atomically with stable key order (Go's
// encoding/json sorts map keys).
func SaveState(stateDir string, st WorktreesState) error {
	if stateDir == "" {
		return errors.New("no state dir configured")
	}
	if st == nil {
		st = WorktreesState{}
	}
	data, err := json.MarshalIndent(st, "", "  ")
	if err != nil {
		return err
	}
	if err := os.MkdirAll(stateDir, 0o755); err != nil {
		return err
	}
	return model.WriteFileAtomic(filepath.Join(stateDir, stateFileName), append(data, '\n'), 0o644)
}

// GetState returns the client's current state.
func (c *Client) GetState() (WorktreesState, error) {
	return LoadState(c.StateDir)
}

// SaveState writes the client's state.
func (c *Client) SaveState(st WorktreesState) error {
	return SaveState(c.StateDir, st)
}

// UpdateState rewrites one change's entry read-modify-write and persists.
func (c *Client) UpdateState(changeID string, fn func(Entry) Entry) error {
	st, err := c.GetState()
	if err != nil {
		return err
	}
	entry := st[changeID]
	st[changeID] = fn(entry)
	return c.SaveState(st)
}

// DeleteStateEntry removes one change's entry; absent keys are fine.
func (c *Client) DeleteStateEntry(changeID string) error {
	st, err := c.GetState()
	if err != nil {
		return err
	}
	if _, ok := st[changeID]; !ok {
		return nil
	}
	delete(st, changeID)
	return c.SaveState(st)
}
