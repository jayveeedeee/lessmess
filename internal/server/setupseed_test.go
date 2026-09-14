package server

import (
	"context"
	"encoding/json"
	"net/http"
	"os"
	"path/filepath"
	"testing"
	"time"

	"lessmess/internal/docs"
	"lessmess/internal/opencode"
)

// seedFixture builds a covered repo ({"include":["**"]}) with settings.
func seedFixture(t *testing.T, agent, model string) string {
	t.Helper()
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "README.md"), []byte("x\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, docs.ConfigFile), []byte(`{"include":["**"]}`+"\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if agent != "" || model != "" {
		if err := applySettingsPatch(dir, SettingsScopePersonal, []byte(`{"session":{"agent":"`+agent+`","model":"`+model+`"}}`)); err != nil {
			t.Fatal(err)
		}
	}
	return dir
}

// stubSeedSummarizer swaps the factory to capture agent/model and avoid LLM
// sessions. block, when non-nil, makes each SummarizeDir wait on it.
func stubSeedSummarizer(t *testing.T, block <-chan struct{}) (captured *struct{ agent, model string }) {
	t.Helper()
	captured = &struct{ agent, model string }{}
	orig := newSeedSummarizer
	t.Cleanup(func() { newSeedSummarizer = orig })
	newSeedSummarizer = func(_ docs.SessionClient, _ time.Duration, agent, model string) docs.Summarizer {
		captured.agent, captured.model = agent, model
		return &blockingSummarizer{block: block}
	}
	return captured
}

type blockingSummarizer struct{ block <-chan struct{} }

func (b *blockingSummarizer) SummarizeDir(ctx context.Context, _ string, _ *docs.Dir) error {
	if b.block != nil {
		select {
		case <-b.block:
		case <-ctx.Done():
			return ctx.Err()
		}
	}
	return nil
}

func waitSeedDone(t *testing.T, h http.Handler) docsSeedStatusResponse {
	t.Helper()
	deadline := time.Now().Add(10 * time.Second)
	for {
		var st docsSeedStatusResponse
		w := do(t, h, "GET", "/api/setup/docs-seed-status", "")
		if err := json.Unmarshal(w.Body.Bytes(), &st); err != nil {
			t.Fatalf("status json: %v", err)
		}
		if st.Done {
			return st
		}
		if time.Now().After(deadline) {
			t.Fatalf("seed did not finish; last status %+v", st)
		}
		time.Sleep(20 * time.Millisecond)
	}
}

func TestDocsSeed503WithoutOpencode(t *testing.T) {
	s, _, _ := setupShell(t, nil) // shell starts with no client
	w := do(t, s, "POST", "/api/setup/docs-seed", `{"budget":1}`)
	if w.Code != http.StatusServiceUnavailable {
		t.Fatalf("code = %d, want 503", w.Code)
	}
}

func TestDocsSeed503WithoutCoverage(t *testing.T) {
	s, _, _ := setupShell(t, nil)
	s.setOCClient(opencode.New("http://127.0.0.1:1", "pw"))
	w := do(t, s, "POST", "/api/setup/docs-seed", `{"budget":1}`)
	if w.Code != http.StatusServiceUnavailable {
		t.Fatalf("code = %d, want 503 (no agentsdocs.json)", w.Code)
	}
}

func TestDocsSeedRunsAndHonorsDefaults(t *testing.T) {
	dir := seedFixture(t, "build", "prov/m")
	captured := stubSeedSummarizer(t, nil)
	s := NewSetup(dir, nil)
	s.setOCClient(opencode.New("http://127.0.0.1:1", "pw"))

	w := do(t, s, "POST", "/api/setup/docs-seed", `{"budget":1}`)
	if w.Code != http.StatusAccepted {
		t.Fatalf("start = %d %s", w.Code, w.Body)
	}
	st := waitSeedDone(t, s)
	if st.Error != "" {
		t.Fatalf("seed error: %s (lines %v)", st.Error, st.Lines)
	}
	if len(st.Lines) == 0 {
		t.Error("no progress lines captured")
	}
	if captured.agent != "build" || captured.model != "prov/m" {
		t.Errorf("summarizer got agent=%q model=%q, want build / prov/m", captured.agent, captured.model)
	}
	if got := loadOnboarding(dir).Steps["docs"]; got != "seeded" {
		t.Errorf("onboarding docs step = %q, want seeded", got)
	}
}

func TestDocsSeedConflictWhileRunning(t *testing.T) {
	dir := seedFixture(t, "", "")
	block := make(chan struct{})
	stubSeedSummarizer(t, block)
	s := NewSetup(dir, nil)
	s.setOCClient(opencode.New("http://127.0.0.1:1", "pw"))

	if w := do(t, s, "POST", "/api/setup/docs-seed", `{"budget":0}`); w.Code != http.StatusAccepted {
		t.Fatalf("start = %d", w.Code)
	}
	// The single covered dir (".") blocks in SummarizeDir until released.
	deadline := time.Now().Add(5 * time.Second)
	for {
		var st docsSeedStatusResponse
		w := do(t, s, "GET", "/api/setup/docs-seed-status", "")
		_ = json.Unmarshal(w.Body.Bytes(), &st)
		if st.Running {
			break
		}
		if time.Now().After(deadline) {
			t.Fatal("job never entered running state")
		}
		time.Sleep(10 * time.Millisecond)
	}
	if w := do(t, s, "POST", "/api/setup/docs-seed", `{"budget":0}`); w.Code != http.StatusConflict {
		t.Fatalf("concurrent start = %d, want 409", w.Code)
	}
	close(block)
	if st := waitSeedDone(t, s); st.Error != "" {
		t.Fatalf("seed error after release: %s", st.Error)
	}
}

func TestDocsSeedBadBody(t *testing.T) {
	s, _, _ := setupShell(t, nil)
	s.setOCClient(opencode.New("http://127.0.0.1:1", "pw"))
	if w := do(t, s, "POST", "/api/setup/docs-seed", `{oops`); w.Code != http.StatusBadRequest {
		t.Fatalf("bad body = %d, want 400", w.Code)
	}
}

func TestSessionDefaultsHelper(t *testing.T) {
	dir := t.TempDir()
	if a, m := SessionDefaults(dir); a != "" || m != "" {
		t.Errorf("empty repo defaults = %q %q, want service defaults", a, m)
	}
	if err := applySettingsPatch(dir, SettingsScopeProject, []byte(`{"session":{"agent":"build","model":"p/m"}}`)); err != nil {
		t.Fatal(err)
	}
	if a, m := SessionDefaults(dir); a != "build" || m != "p/m" {
		t.Errorf("project layer defaults = %q %q", a, m)
	}
	if err := applySettingsPatch(dir, SettingsScopePersonal, []byte(`{"session":{"agent":"plan"}}`)); err != nil {
		t.Fatal(err)
	}
	if a, m := SessionDefaults(dir); a != "plan" || m != "p/m" {
		t.Errorf("personal override = %q %q, want plan / p/m", a, m)
	}
}
