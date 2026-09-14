package server

import (
	"encoding/json"
	"log/slog"
	"os"
	"path/filepath"

	"lessmess/internal/model"
	"lessmess/internal/store"
)

// Onboarding state lives in .lessmess/onboarding.json (gitignored tooling
// state, one file per feature). It records whether the first-run setup
// wizard was completed or dismissed, plus per-step outcomes. A missing or
// malformed file reads fail-open as "incomplete" — never an error.

const onboardingFile = "onboarding.json"

// onboardingState is the schema of .lessmess/onboarding.json. CompletedAt
// empty means incomplete; Dismissed suppresses the index banner without
// completing. Steps holds short per-step outcomes ("ok", "done", "set",
// "skipped", "seeded") keyed by step name and stays open-ended.
type onboardingState struct {
	Version     int               `json:"version"`
	CompletedAt string            `json:"completedAt,omitempty"`
	Dismissed   bool              `json:"dismissed,omitempty"`
	Steps       map[string]string `json:"steps,omitempty"`
}

func onboardingPath(repoDir string) string {
	return filepath.Join(repoDir, store.StateDirName, onboardingFile)
}

// loadOnboarding reads the state file: missing → zero value and no error;
// malformed → zero value with a logged warning (fail open to incomplete).
func loadOnboarding(repoDir string) onboardingState {
	var st onboardingState
	data, err := os.ReadFile(onboardingPath(repoDir))
	if err != nil {
		return st
	}
	if err := json.Unmarshal(data, &st); err != nil {
		slog.Warn("onboarding state malformed; treating as incomplete", "err", err)
		return onboardingState{}
	}
	return st
}

// saveOnboarding writes the state atomically, creating .lessmess/ as needed.
func saveOnboarding(repoDir string, st onboardingState) error {
	if st.Version == 0 {
		st.Version = 1
	}
	data, err := json.MarshalIndent(st, "", "  ")
	if err != nil {
		return err
	}
	p := onboardingPath(repoDir)
	if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
		return err
	}
	return model.WriteFileAtomic(p, append(data, '\n'), 0o644)
}

// mark records one step outcome on the state.
func (st *onboardingState) mark(step, value string) {
	if st.Steps == nil {
		st.Steps = map[string]string{}
	}
	st.Steps[step] = value
}

// onboardingPending reports whether the index banner should show: setup
// was neither completed nor dismissed.
func onboardingPending(repoDir string) bool {
	st := loadOnboarding(repoDir)
	return st.CompletedAt == "" && !st.Dismissed
}
