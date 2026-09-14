package server

import (
	"os"
	"path/filepath"
	"testing"
)

func TestOnboardingRoundtrip(t *testing.T) {
	dir := t.TempDir()
	st := loadOnboarding(dir)
	if st.CompletedAt != "" || st.Dismissed || st.Steps != nil {
		t.Fatalf("missing file: got %+v, want zero value", st)
	}
	st.mark("bootstrap", "done")
	st.mark("agent", "set")
	if err := saveOnboarding(dir, st); err != nil {
		t.Fatalf("save: %v", err)
	}
	got := loadOnboarding(dir)
	if got.Version != 1 || got.Steps["bootstrap"] != "done" || got.Steps["agent"] != "set" {
		t.Fatalf("roundtrip: got %+v", got)
	}
	if !onboardingPending(dir) {
		t.Fatal("incomplete + not dismissed must be pending")
	}
}

func TestOnboardingFailOpenMalformed(t *testing.T) {
	dir := t.TempDir()
	p := onboardingPath(dir)
	if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(p, []byte("{not json"), 0o644); err != nil {
		t.Fatal(err)
	}
	st := loadOnboarding(dir)
	if st.CompletedAt != "" || st.Dismissed {
		t.Fatalf("malformed file must fail open to incomplete, got %+v", st)
	}
	if !onboardingPending(dir) {
		t.Fatal("malformed file must read as pending")
	}
}

func TestOnboardingPendingTransitions(t *testing.T) {
	dir := t.TempDir()
	if !onboardingPending(dir) {
		t.Fatal("no file: want pending")
	}
	st := onboardingState{Dismissed: true}
	if err := saveOnboarding(dir, st); err != nil {
		t.Fatalf("save dismissed: %v", err)
	}
	if onboardingPending(dir) {
		t.Fatal("dismissed: want not pending")
	}
	st = onboardingState{CompletedAt: "2026-09-13T10:00:00Z"}
	if err := saveOnboarding(dir, st); err != nil {
		t.Fatalf("save completed: %v", err)
	}
	if onboardingPending(dir) {
		t.Fatal("completed: want not pending")
	}
}
