package store

import (
	"os"
	"path/filepath"
	"testing"
)

func TestMigrateStateDirRenamesLegacy(t *testing.T) {
	root := t.TempDir()
	legacy := filepath.Join(root, ".tasktracker")
	if err := os.MkdirAll(legacy, 0o755); err != nil {
		t.Fatal(err)
	}
	for _, f := range []string{"sessions.json", "docs-queue.json", "docs-seed.json"} {
		if err := os.WriteFile(filepath.Join(legacy, f), []byte(f+"-data"), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	if err := MigrateStateDir(root); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(legacy); !os.IsNotExist(err) {
		t.Error("legacy dir still present")
	}
	for _, f := range []string{"sessions.json", "docs-queue.json", "docs-seed.json"} {
		data, err := os.ReadFile(filepath.Join(root, ".lessmess", f))
		if err != nil {
			t.Fatalf("%s: %v", f, err)
		}
		if string(data) != f+"-data" {
			t.Errorf("%s content %q", f, data)
		}
	}
}

func TestMigrateStateDirKeepsExistingCurrent(t *testing.T) {
	root := t.TempDir()
	for _, d := range []string{".tasktracker", ".lessmess"} {
		if err := os.MkdirAll(filepath.Join(root, d), 0o755); err != nil {
			t.Fatal(err)
		}
	}
	if err := os.WriteFile(filepath.Join(root, ".lessmess", "sessions.json"), []byte("current"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, ".tasktracker", "sessions.json"), []byte("legacy"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := MigrateStateDir(root); err != nil {
		t.Fatal(err)
	}
	data, _ := os.ReadFile(filepath.Join(root, ".lessmess", "sessions.json"))
	if string(data) != "current" {
		t.Error("existing .lessmess must win; no merge or overwrite")
	}
	if _, err := os.Stat(filepath.Join(root, ".tasktracker")); err != nil {
		t.Error("legacy dir must be left untouched when .lessmess exists")
	}
}

func TestMigrateStateDirNoopWhenNeitherExists(t *testing.T) {
	root := t.TempDir()
	if err := MigrateStateDir(root); err != nil {
		t.Fatal(err)
	}
	for _, d := range []string{".tasktracker", ".lessmess"} {
		if _, err := os.Stat(filepath.Join(root, d)); !os.IsNotExist(err) {
			t.Errorf("%s unexpectedly present", d)
		}
	}
}
