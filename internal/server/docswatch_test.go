package server

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	"lessmess/internal/docs"
)

func watcherRepo(t *testing.T) (string, *docs.Config) {
	t.Helper()
	root := t.TempDir()
	if err := os.MkdirAll(filepath.Join(root, "internal"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, "internal", "x.go"), []byte("x\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, docs.ConfigFile), []byte(`{"include":["**"]}`+"\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	cfg, err := docs.LoadConfig(root)
	if err != nil {
		t.Fatal(err)
	}
	return root, cfg
}

func awaitEvent(t *testing.T, w *docsWatcher, what string, timeout time.Duration) {
	t.Helper()
	select {
	case <-w.Events():
	case <-time.After(timeout):
		t.Fatalf("timed out waiting for event: %s", what)
	}
}

func TestDocsWatcherEmitsOnWrite(t *testing.T) {
	root, cfg := watcherRepo(t)
	w, err := newDocsWatcher(root, cfg)
	if err != nil {
		t.Fatal(err)
	}
	defer w.Close()
	if err := os.WriteFile(filepath.Join(root, "internal", "x.go"), []byte("y\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	awaitEvent(t, w, "write in covered dir", 3*time.Second)
}

func TestDocsWatcherIgnoresTempAndHidden(t *testing.T) {
	root, cfg := watcherRepo(t)
	w, err := newDocsWatcher(root, cfg)
	if err != nil {
		t.Fatal(err)
	}
	defer w.Close()
	os.WriteFile(filepath.Join(root, "internal", ".tt-tmp123"), []byte("x\n"), 0o644)
	os.WriteFile(filepath.Join(root, "internal", ".DS_Store"), []byte("x\n"), 0o644)
	select {
	case <-w.Events():
		t.Fatal("temp/hidden writes must be ignored")
	case <-time.After(700 * time.Millisecond):
	}
}

func TestDocsWatcherResyncsNewDirs(t *testing.T) {
	root, cfg := watcherRepo(t)
	w, err := newDocsWatcher(root, cfg)
	if err != nil {
		t.Fatal(err)
	}
	defer w.Close()

	// Create a new covered dir: the parent watch fires, the quiet-period
	// resync picks the new dir up, and a write inside it fires too.
	if err := os.MkdirAll(filepath.Join(root, "internal", "sub"), 0o755); err != nil {
		t.Fatal(err)
	}
	awaitEvent(t, w, "new covered dir", 3*time.Second)
	time.Sleep(100 * time.Millisecond) // let resync settle past the debounce

	if err := os.WriteFile(filepath.Join(root, "internal", "sub", "y.go"), []byte("y\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	awaitEvent(t, w, "write inside newly watched dir", 3*time.Second)
}
