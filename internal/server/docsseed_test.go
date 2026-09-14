package server

import (
	"context"
	"encoding/json"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	"lessmess/internal/docs"
	"lessmess/internal/model"
	"lessmess/internal/opencode"
	"lessmess/internal/store"
)

// docsSeedServer is a docs-enabled fixture server with a (never-called)
// opencode client; tests stub newSeedSummarizer, so sessions never run.
func docsSeedServer(t *testing.T) (*Server, string) {
	t.Helper()
	st, dir := fixtureStore(t)
	if err := os.WriteFile(filepath.Join(dir, docs.ConfigFile), []byte(`{"include":["**"]}`+"\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	s := New(st)
	t.Cleanup(s.Close)
	s.SetOpencode(opencode.New("http://127.0.0.1:1", "pw"))
	return s, dir
}

func seedCursorWrite(t *testing.T, dir string, summarized map[string]bool) {
	t.Helper()
	if err := os.MkdirAll(filepath.Join(dir, store.StateDirName), 0o755); err != nil {
		t.Fatal(err)
	}
	data, err := json.Marshal(struct {
		Summarized map[string]bool `json:"summarized"`
	}{summarized})
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, store.StateDirName, "docs-seed.json"), data, 0o644); err != nil {
		t.Fatal(err)
	}
}

func seedPending(t *testing.T, s *Server) int {
	t.Helper()
	w := do(t, s.Handler(), "GET", "/api/validate", "")
	if w.Code != 200 {
		t.Fatalf("validate: %d %s", w.Code, w.Body)
	}
	var resp struct {
		DocsSeedPending *int `json:"docsSeedPending"`
	}
	if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
		t.Fatal(err)
	}
	if resp.DocsSeedPending == nil {
		t.Fatal("docsSeedPending missing from /api/validate payload")
	}
	return *resp.DocsSeedPending
}

func waitDocsSeedDone(t *testing.T, s *Server) docsSeedStatusResponse {
	t.Helper()
	deadline := time.Now().Add(10 * time.Second)
	for {
		w := do(t, s.Handler(), "GET", "/docs/seed-status", "")
		var st docsSeedStatusResponse
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

func TestDocsSeedEndpoint503WithoutOpencode(t *testing.T) {
	s, _ := docsServer(t) // docs enabled, no opencode client
	w := do(t, s.Handler(), "POST", "/docs/seed", `{}`)
	if w.Code != http.StatusServiceUnavailable {
		t.Fatalf("code = %d, want 503", w.Code)
	}
}

func TestDocsSeedEndpoint503WithoutCoverage(t *testing.T) {
	s := mappingServer(t, nil)
	s.SetOpencode(opencode.New("http://127.0.0.1:1", "pw"))
	w := do(t, s.Handler(), "POST", "/docs/seed", `{}`)
	if w.Code != http.StatusServiceUnavailable {
		t.Fatalf("code = %d, want 503 (no agentsdocs.json)", w.Code)
	}
}

func TestDocsSeedEndpointContinuesPending(t *testing.T) {
	s, dir := docsSeedServer(t)
	// A real covered subdir whose cursor claims it is summarized but whose
	// doc files are missing: the run must still cover it — file existence,
	// not the cursor, decides.
	if err := os.MkdirAll(filepath.Join(dir, "internal", "model"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "internal", "model", "model.go"), []byte("x\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	seedCursorWrite(t, dir, map[string]bool{"internal/model": true, ".": true})
	before := seedPending(t, s)
	if before <= 0 {
		t.Fatalf("pending = %d, want > 0 when doc files are missing", before)
	}

	sum := stubFileSeedSummarizer(t)
	w := do(t, s.Handler(), "POST", "/docs/seed", `{}`)
	if w.Code != http.StatusAccepted {
		t.Fatalf("start: %d %s", w.Code, w.Body)
	}
	st := waitDocsSeedDone(t, s)
	if st.Error != "" {
		t.Fatalf("seed error: %s", st.Error)
	}
	joined := strings.Join(st.Lines, "\n")
	if !strings.Contains(joined, "internal/model (summarized)") {
		t.Errorf("missing dir not run despite cursor:\n%s", joined)
	}
	if after := seedPending(t, s); after != 0 {
		t.Errorf("pending after run = %d, want 0", after)
	}
	if len(sum.calls) == 0 {
		t.Error("summarizer never called")
	}
}

func TestDocsSeedEndpointForceRedoesAll(t *testing.T) {
	s, dir := docsSeedServer(t)
	// Everything seeded and complete: a plain run is a no-op…
	if err := os.MkdirAll(filepath.Join(dir, "internal", "model"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "internal", "model", "model.go"), []byte("x\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	sum := stubFileSeedSummarizer(t)
	if w := do(t, s.Handler(), "POST", "/docs/seed", `{}`); w.Code != http.StatusAccepted {
		t.Fatalf("initial seed: %d %s", w.Code, w.Body)
	}
	waitDocsSeedDone(t, s)
	if w := do(t, s.Handler(), "POST", "/docs/seed", `{}`); w.Code != 200 || !strings.Contains(w.Body.String(), "use force") {
		t.Fatalf("plain re-run: %d %s, want the nothing-to-do hint", w.Code, w.Body)
	}
	first := len(sum.calls)

	// …but force redoes every covered dir regardless of the cursor.
	seedCursorWrite(t, dir, map[string]bool{".": true, "internal": true, "internal/model": true})
	if w := do(t, s.Handler(), "POST", "/docs/seed", `{"force":true}`); w.Code != http.StatusAccepted {
		t.Fatalf("force seed: %d %s", w.Code, w.Body)
	}
	waitDocsSeedDone(t, s)
	if len(sum.calls) <= first {
		t.Errorf("force run made %d calls (total %d), must redo directories", len(sum.calls)-first, len(sum.calls))
	}
}

// fileSeedSummarizer is a summarizer that behaves like a real one for
// pending purposes: it writes the directory's AGENTS.md (with markers) so
// the file-existence pending check converges after a run.
type fileSeedSummarizer struct {
	calls []string
	mu    sync.Mutex
}

func (f *fileSeedSummarizer) SummarizeDir(_ context.Context, _ string, d *docs.Dir) error {
	f.mu.Lock()
	f.calls = append(f.calls, d.Rel)
	f.mu.Unlock()
	agents := model.DocMarkerBegin + "\n- (seed) stub learning for " + d.Rel + "\n" + model.DocMarkerEnd + "\n"
	return os.WriteFile(filepath.Join(d.Abs, docs.AgentsFile), []byte(agents), 0o644)
}

func (f *fileSeedSummarizer) total() int {
	f.mu.Lock()
	defer f.mu.Unlock()
	return len(f.calls)
}

// stubFileSeedSummarizer swaps the factory to a file-writing summarizer.
func stubFileSeedSummarizer(t *testing.T) *fileSeedSummarizer {
	t.Helper()
	orig := newSeedSummarizer
	t.Cleanup(func() { newSeedSummarizer = orig })
	f := &fileSeedSummarizer{}
	newSeedSummarizer = func(_ docs.SessionClient, _ time.Duration, _, _ string) docs.Summarizer {
		return f
	}
	return f
}

func TestDocsSeedEndpoint409WhileRunning(t *testing.T) {
	s, _ := docsSeedServer(t)
	block := make(chan struct{})
	stubSeedSummarizer(t, block)
	if w := do(t, s.Handler(), "POST", "/docs/seed", `{}`); w.Code != http.StatusAccepted {
		t.Fatalf("first start: %d %s", w.Code, w.Body)
	}
	if w := do(t, s.Handler(), "POST", "/docs/seed", `{}`); w.Code != http.StatusConflict {
		t.Errorf("second start: %d, want 409", w.Code)
	}
	close(block)
	waitDocsSeedDone(t, s)
}

func TestDocsExclusionsRoundTrip(t *testing.T) {
	s, dir := docsSeedServer(t)
	// Real excludable dirs (the fixture has none beyond changes/).
	for _, d := range []string{"web", "internal"} {
		if err := os.MkdirAll(filepath.Join(dir, d), 0o755); err != nil {
			t.Fatal(err)
		}
	}
	// A hand-authored stale pattern must survive editor saves.
	if err := os.WriteFile(filepath.Join(dir, docs.ConfigFile),
		[]byte("{\"include\":[\"**\"],\"exclude\":[\"nonexistent-dir\"]}\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	w := do(t, s.Handler(), "GET", "/docs/exclusions", "")
	if w.Code != 200 {
		t.Fatalf("GET: %d %s", w.Code, w.Body)
	}
	var listing struct {
		Dirs      []setupDirEntry `json:"dirs"`
		HasConfig bool            `json:"hasConfig"`
	}
	if err := json.Unmarshal(w.Body.Bytes(), &listing); err != nil {
		t.Fatal(err)
	}
	if !listing.HasConfig {
		t.Fatal("hasConfig = false, want true")
	}
	for _, d := range listing.Dirs {
		if d.Rel == "changes" {
			t.Error("changes/ must never be listed")
		}
	}

	if w := do(t, s.Handler(), "POST", "/docs/exclusions", `{"excludeDirs":["web","../evil"]}`); w.Code != 422 {
		t.Errorf("invalid pattern: code = %d, want 422", w.Code)
	}
	if w := do(t, s.Handler(), "POST", "/docs/exclusions", `{"excludeDirs":["web","internal"]}`); w.Code != 200 {
		t.Fatalf("save: %d %s", w.Code, w.Body)
	}

	// Round-trip: the saved selection reads back excluded; the stale
	// hand-authored pattern is preserved in the file.
	w = do(t, s.Handler(), "GET", "/docs/exclusions", "")
	if err := json.Unmarshal(w.Body.Bytes(), &listing); err != nil {
		t.Fatal(err)
	}
	excluded := map[string]bool{}
	for _, d := range listing.Dirs {
		excluded[d.Rel] = d.Excluded
	}
	if !excluded["web"] || !excluded["internal"] {
		t.Errorf("saved exclusions not reflected: %v", listing.Dirs)
	}
	raw, err := os.ReadFile(filepath.Join(dir, docs.ConfigFile))
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(raw), "nonexistent-dir") {
		t.Errorf("hand-authored pattern not preserved: %s", raw)
	}
}

func TestDocsExclusions503WithoutConfig(t *testing.T) {
	s := mappingServer(t, nil)
	if w := do(t, s.Handler(), "POST", "/docs/exclusions", `{"excludeDirs":["web"]}`); w.Code != http.StatusServiceUnavailable {
		t.Errorf("save without config: code = %d, want 503", w.Code)
	}
	w := do(t, s.Handler(), "GET", "/docs/exclusions", "")
	var listing struct {
		HasConfig bool `json:"hasConfig"`
	}
	json.Unmarshal(w.Body.Bytes(), &listing)
	if listing.HasConfig {
		t.Error("hasConfig must be false without agentsdocs.json")
	}
}
