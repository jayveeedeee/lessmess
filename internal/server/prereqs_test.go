package server

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"lessmess/internal/opencode"
)

// fakePrereqs swaps every injectable probe and restores them on cleanup.
func fakePrereqs(t *testing.T) {
	t.Helper()
	origLook, origSvc, origGit, origWr := prereqLookPath, prereqProbeService, prereqProbeGit, prereqWritable
	t.Cleanup(func() {
		prereqLookPath, prereqProbeService, prereqProbeGit, prereqWritable = origLook, origSvc, origGit, origWr
	})
	prereqLookPath = func(string) (string, error) { return "/usr/local/bin/opencode2", nil }
	prereqProbeService = func(context.Context) (*opencode.Client, string, string, error) {
		return opencode.New("http://127.0.0.1:1", "pw"), "http://127.0.0.1:1", "", nil
	}
	prereqProbeGit = func(context.Context, string) (bool, bool) { return true, true }
	prereqWritable = func(string) error { return nil }
}

func prereqByID(t *testing.T, resp prereqsResponse, id string) prereqCheck {
	t.Helper()
	for _, c := range resp.Checks {
		if c.ID == id {
			return c
		}
	}
	t.Fatalf("check %q missing in %+v", id, resp.Checks)
	return prereqCheck{}
}

func TestPrereqsAllOK(t *testing.T) {
	fakePrereqs(t)
	s, _, _ := setupShell(t, nil)

	w := do(t, s, "GET", "/api/setup/prereqs", "")
	if w.Code != http.StatusOK {
		t.Fatalf("code = %d", w.Code)
	}
	var resp prereqsResponse
	if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
		t.Fatalf("json: %v", err)
	}
	if !resp.Ready {
		t.Fatalf("ready = false: %+v", resp.Checks)
	}
	if got := prereqByID(t, resp, "opencode2-binary").Status; got != "ok" {
		t.Errorf("binary = %s", got)
	}
	if got := prereqByID(t, resp, "opencode-service").Status; got != "ok" {
		t.Errorf("service = %s", got)
	}
	// No changes/ in the temp dir: informational warn, does not block ready.
	if got := prereqByID(t, resp, "changes-present").Status; got != "warn" {
		t.Errorf("changes-present = %s, want warn", got)
	}
	// A successful service probe stores the client on the setup shell.
	if s.ocClient() == nil {
		t.Error("setup shell did not retain the discovered client")
	}
}

func TestPrereqsBinaryMissing(t *testing.T) {
	fakePrereqs(t)
	prereqLookPath = func(name string) (string, error) { return "", errors.New("not found") }
	prereqProbeGit = func(context.Context, string) (bool, bool) { return false, false }
	s, _, _ := setupShell(t, nil)

	var resp prereqsResponse
	w := do(t, s, "GET", "/api/setup/prereqs", "")
	if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
		t.Fatalf("json: %v", err)
	}
	if resp.Ready {
		t.Fatal("ready = true with missing binary")
	}
	if got := prereqByID(t, resp, "opencode2-binary").Status; got != "fail" {
		t.Errorf("binary = %s, want fail", got)
	}
	svc := prereqByID(t, resp, "opencode-service")
	if svc.Status != "fail" || svc.Detail == "" {
		t.Errorf("service = %+v, want fail with skip detail", svc)
	}
	if got := prereqByID(t, resp, "git-binary").Status; got != "warn" {
		t.Errorf("git-binary = %s, want warn (soft)", got)
	}
}

func TestPrereqsServiceStages(t *testing.T) {
	fakePrereqs(t)
	stages := []struct {
		stage   string
		wantRem bool
	}{
		{"service", true},
		{"credentials", true},
		{"health", true},
	}
	for _, tc := range stages {
		t.Run(tc.stage, func(t *testing.T) {
			prereqProbeService = func(context.Context) (*opencode.Client, string, string, error) {
				return nil, "http://127.0.0.1:1", tc.stage, errors.New("boom")
			}
			s, _, _ := setupShell(t, nil)
			var resp prereqsResponse
			w := do(t, s, "GET", "/api/setup/prereqs", "")
			if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
				t.Fatalf("json: %v", err)
			}
			svc := prereqByID(t, resp, "opencode-service")
			if svc.Status != "fail" || svc.Detail == "" || svc.Remedy == "" {
				t.Errorf("stage %s: got %+v, want fail with detail+remedy", tc.stage, svc)
			}
			if resp.Ready {
				t.Errorf("stage %s: ready = true", tc.stage)
			}
		})
	}
}

func TestPrereqsNotWritable(t *testing.T) {
	fakePrereqs(t)
	prereqWritable = func(string) error { return errors.New("permission denied") }
	s, _, _ := setupShell(t, nil)

	var resp prereqsResponse
	w := do(t, s, "GET", "/api/setup/prereqs", "")
	if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
		t.Fatalf("json: %v", err)
	}
	if got := prereqByID(t, resp, "repo-writable").Status; got != "fail" {
		t.Errorf("repo-writable = %s, want fail", got)
	}
	if resp.Ready {
		t.Fatal("ready = true with unwritable repo")
	}
}

func TestPrereqsPartialChangesTree(t *testing.T) {
	fakePrereqs(t)
	dir := t.TempDir()
	if err := os.MkdirAll(filepath.Join(dir, "changes"), 0o755); err != nil {
		t.Fatal(err)
	}
	s := NewSetup(dir, nil)
	var resp prereqsResponse
	w := do(t, s, "GET", "/api/setup/prereqs", "")
	if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
		t.Fatalf("json: %v", err)
	}
	c := prereqByID(t, resp, "changes-present")
	if c.Status != "warn" || !strings.Contains(c.Detail, "no root ledger") {
		t.Errorf("changes-present = %+v, want warn about the missing root ledger", c)
	}
	if !resp.Ready {
		t.Error("a partial tree must not block ready (bootstrap fixes it)")
	}
}

func TestPrereqsChangesPresentOnNormalServer(t *testing.T) {
	fakePrereqs(t)
	st, _ := fixtureStore(t)
	w := do(t, New(st).Handler(), "GET", "/api/setup/prereqs", "")
	var resp prereqsResponse
	if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
		t.Fatalf("json: %v", err)
	}
	if got := prereqByID(t, resp, "changes-present").Status; got != "ok" {
		t.Errorf("changes-present = %s, want ok on initialized repo", got)
	}
	if !resp.Ready {
		t.Errorf("ready = false on healthy fixture: %+v", resp.Checks)
	}
}
