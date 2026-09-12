package store

import (
	"os"
	"path/filepath"
)

// StateDirName is the tooling-state directory at the repository root
// (sessions, docs queue, docs seed cursor). It must stay gitignored.
const StateDirName = ".lessmess"

// legacyStateDirName is the pre-rename tooling-state directory.
const legacyStateDirName = ".tasktracker"

// MigrateStateDir renames a legacy .tasktracker tooling-state directory to
// .lessmess when the new location does not exist yet, preserving all state
// files (sessions.json, docs-queue.json, docs-seed.json). It is a no-op when
// .lessmess already exists or when neither directory exists, and it never
// merges or deletes anything.
func MigrateStateDir(root string) error {
	current := filepath.Join(root, StateDirName)
	if _, err := os.Stat(current); err == nil {
		return nil
	} else if !os.IsNotExist(err) {
		return err
	}
	legacy := filepath.Join(root, legacyStateDirName)
	if _, err := os.Stat(legacy); os.IsNotExist(err) {
		return nil
	} else if err != nil {
		return err
	}
	return os.Rename(legacy, current)
}
