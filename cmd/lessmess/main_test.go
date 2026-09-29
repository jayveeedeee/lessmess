package main

import (
	"os"
	"path/filepath"
	"testing"

	"lessmess/internal/docs"
)

func initCLIRepo(t *testing.T) string {
	t.Helper()
	root := t.TempDir()
	if _, err := docs.Init(root); err != nil {
		t.Fatal(err)
	}
	return root
}

func TestRunValidateHonorsDocsEnabled(t *testing.T) {
	root := initCLIRepo(t)
	if err := os.WriteFile(filepath.Join(root, docs.ConfigFile), []byte("{broken"), 0o644); err != nil {
		t.Fatal(err)
	}

	// The workflow remains valid and the malformed dormant docs config is
	// deliberately ignored while the experimental feature is off.
	if code := runValidate([]string{"--dir", root}); code != 0 {
		t.Fatalf("validate with docs off = %d, want 0", code)
	}
	if err := os.WriteFile(filepath.Join(root, "lessmess.json"), []byte(`{"docs":{"enabled":true}}`+"\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if code := runValidate([]string{"--dir", root}); code != 1 {
		t.Fatalf("validate with docs on and malformed config = %d, want 1", code)
	}
}

func TestRunDocsSeedHonorsDocsEnabled(t *testing.T) {
	root := initCLIRepo(t)
	if err := os.WriteFile(filepath.Join(root, docs.ConfigFile), []byte(`{"include":["**"]}`+"\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if code := runDocsSeed([]string{"--dir", root, "--dry-run"}); code != 1 {
		t.Fatalf("docs seed with docs off = %d, want 1", code)
	}
	if err := os.WriteFile(filepath.Join(root, "lessmess.json"), []byte(`{"docs":{"enabled":true}}`+"\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if code := runDocsSeed([]string{"--dir", root, "--dry-run"}); code != 0 {
		t.Fatalf("docs seed with docs on = %d, want 0", code)
	}
}
