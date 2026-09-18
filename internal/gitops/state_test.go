package gitops

import (
	"errors"
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestStateRoundTrip(t *testing.T) {
	dir := filepath.Join(t.TempDir(), ".lessmess")
	st, err := LoadState(dir)
	if err != nil || len(st) != 0 {
		t.Fatalf("missing file: state = %v, %v; want empty", st, err)
	}

	want := WorktreesState{
		"2026-09-17-x": {Branch: "change/x", Path: "/wt/x", PRURL: "https://o/r/pull/1", ReviewState: ReviewPending, Created: time.Now().UTC().Format(time.RFC3339)},
	}
	if err := SaveState(dir, want); err != nil {
		t.Fatalf("SaveState: %v", err)
	}
	got, err := LoadState(dir)
	if err != nil {
		t.Fatal(err)
	}
	if got["2026-09-17-x"] != want["2026-09-17-x"] {
		t.Errorf("round trip = %+v, want %+v", got["2026-09-17-x"], want["2026-09-17-x"])
	}
}

func TestStateMalformedIsError(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, stateFileName), []byte("{broken"), 0o644); err != nil {
		t.Fatal(err)
	}
	if _, err := LoadState(dir); err == nil {
		t.Fatal("malformed state = nil error, want error")
	}
}

func TestStateEmptyDirErrors(t *testing.T) {
	var empty string
	if _, err := LoadState(empty); err != nil {
		t.Errorf("LoadState(\"\") = %v, want empty state", err)
	}
	if err := SaveState(empty, nil); err == nil {
		t.Error("SaveState(\"\") = nil error, want error")
	}
}

func TestClientUpdateAndDelete(t *testing.T) {
	dir := filepath.Join(t.TempDir(), ".lessmess")
	c := New("/repo", dir)

	if err := c.UpdateState("x", func(e Entry) Entry {
		return Entry{Branch: "change/x", Path: "/wt/x", Created: "t"}
	}); err != nil {
		t.Fatalf("UpdateState create: %v", err)
	}
	if err := c.UpdateState("x", func(e Entry) Entry {
		e.PRURL = "https://o/r/pull/2"
		e.ReviewState = ReviewDone
		return e
	}); err != nil {
		t.Fatalf("UpdateState amend: %v", err)
	}
	st, err := c.GetState()
	if err != nil {
		t.Fatal(err)
	}
	if st["x"].PRURL != "https://o/r/pull/2" || st["x"].ReviewState != ReviewDone {
		t.Errorf("state = %+v", st["x"])
	}
	if err := c.DeleteStateEntry("x"); err != nil {
		t.Fatalf("DeleteState: %v", err)
	}
	if err := c.DeleteStateEntry("x"); err != nil {
		t.Fatalf("DeleteState absent: %v", err)
	}
	st, _ = c.GetState()
	if len(st) != 0 {
		t.Errorf("state after delete = %v, want empty", st)
	}
	if _, ok := st["nope"]; ok {
		t.Error("unexpected entry")
	}
	if err := c.DeleteStateEntry(""); errors.Is(err, os.ErrNotExist) {
		t.Error("absent-file delete must not surface ErrNotExist")
	}
}
