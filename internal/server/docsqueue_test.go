package server

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	"tasktracker/internal/docs"
	"tasktracker/internal/model"
)

// fakeRunner records jobs and can be told to fail.
type fakeRunner struct {
	mu   sync.Mutex
	jobs []DocsJob
	err  error
}

func (f *fakeRunner) RunDocsJob(_ context.Context, j DocsJob) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.jobs = append(f.jobs, j)
	return f.err
}

func (f *fakeRunner) got() []DocsJob {
	f.mu.Lock()
	defer f.mu.Unlock()
	return append([]DocsJob{}, f.jobs...)
}

func waitForCond(t *testing.T, what string, cond func() bool) {
	t.Helper()
	deadline := time.Now().Add(5 * time.Second)
	for time.Now().Before(deadline) {
		if cond() {
			return
		}
		time.Sleep(10 * time.Millisecond)
	}
	t.Fatalf("timed out waiting for %s", what)
}

func writeTaskFilesAffected(t *testing.T, dir, changeID, file, section string) {
	t.Helper()
	body := `---
id: FIX-00
title: First
---

# FIX-00: First

## Files affected

` + section + `

## Notes
`
	if err := os.WriteFile(filepath.Join(dir, "changes", changeID, "tasks", file), []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
}

func TestTouchedDocsDirs(t *testing.T) {
	st, dir := fixtureStore(t)
	st.Close()
	if err := os.WriteFile(filepath.Join(dir, docs.ConfigFile), []byte(`{"include":["**"],"exclude":["web/static"]}`+"\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	cfg, err := docs.LoadConfig(dir)
	if err != nil {
		t.Fatal(err)
	}
	writeTaskFilesAffected(t, dir, "2026-09-10-0", "00-first.md", `
- internal/model/docfile.go (new)
- internal/model/docfile_test.go (new)
- `+"`README.md`"+`
- web/static/app.js
- changes/2026-09-10-0/plan.md
- internal/model/AGENTS.md
- docs are updated automatically (prose line, ignored)
- .tasktracker/docs-queue.json
`)
	writeTaskFilesAffected(t, dir, "2026-09-10-0", "01-second.md", `
- cmd/tasktracker/main.go
- internal/docs/ (new package)
`)
	got, err := touchedDocsDirs(dir, "2026-09-10-0", cfg)
	if err != nil {
		t.Fatal(err)
	}
	// cmd/tasktracker/main.go maps to its (config-covered) parent dir;
	// web/static/app.js bubbles up to web via the user exclude.
	want := []string{".", "cmd/tasktracker", "internal/docs", "internal/model", "web"}
	if strings.Join(got, ",") != strings.Join(want, ",") {
		t.Errorf("got %v, want %v", got, want)
	}
}

func TestFilesAffectedSectionParsing(t *testing.T) {
	body := "# T\n\n## Files affected\n\n- `a/b.go` (new)\n- c.go\n- not a path\n- https://x.y/z\n\n## Notes\n\n- ignored/elsewhere.go\n"
	got := filesAffected(body)
	want := []string{"a/b.go", "c.go"}
	if strings.Join(got, ",") != strings.Join(want, ",") {
		t.Errorf("got %v, want %v", got, want)
	}
}

// docsServer builds a server over the fixture repo with docs enabled.
func docsServer(t *testing.T) (*Server, string) {
	t.Helper()
	st, dir := fixtureStore(t)
	if err := os.WriteFile(filepath.Join(dir, docs.ConfigFile), []byte(`{"include":["**"],"exclude":["web/static"]}`+"\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	s := New(st)
	t.Cleanup(s.Close)
	return s, dir
}

func TestCloseEnqueuesAndRunsJob(t *testing.T) {
	s, dir := docsServer(t)
	writeTaskFilesAffected(t, dir, "2026-09-10-0", "00-first.md", `
- internal/model/docfile.go (new)
- README.md
- changes/2026-09-10-0/plan.md (ignored: workflow tree)
`)
	runner := &fakeRunner{}
	s.SetDocsRunner(runner)

	w := do(t, s.Handler(), "POST", "/changes/2026-09-10-0/close", "")
	if w.Code != 200 {
		t.Fatalf("close: %d %s", w.Code, w.Body)
	}
	waitForCond(t, "docs job", func() bool { return len(runner.got()) == 1 })
	job := runner.got()[0]
	if job.Change != "2026-09-10-0" || job.Title != "Fixture change" {
		t.Errorf("job identity: %+v", job)
	}
	want := []string{".", "internal/model"}
	if strings.Join(job.Dirs, ",") != strings.Join(want, ",") {
		t.Errorf("job dirs %v, want %v", job.Dirs, want)
	}
}

func TestCloseWithoutRunnerMarksStale(t *testing.T) {
	s, dir := docsServer(t)
	writeTaskFilesAffected(t, dir, "2026-09-10-0", "00-first.md", "- internal/model/docfile.go\n")
	// No runner: simulates the opencode service being down.
	w := do(t, s.Handler(), "POST", "/changes/2026-09-10-0/close", "")
	if w.Code != 200 {
		t.Fatalf("close: %d %s", w.Code, w.Body)
	}
	waitForCond(t, "stale flags", func() bool { return len(s.docsQ.staleDirs()) == 1 })
	if got := s.docsQ.staleDirs(); got[0] != "internal/model" {
		t.Errorf("stale %v", got)
	}
	// State persisted.
	data, err := os.ReadFile(filepath.Join(dir, ".tasktracker", "docs-queue.json"))
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(data), "internal/model") {
		t.Error("stale set not persisted")
	}
}

func TestDocsRefreshReconcilesStale(t *testing.T) {
	s, dir := docsServer(t)
	writeTaskFilesAffected(t, dir, "2026-09-10-0", "00-first.md", "- internal/model/docfile.go\n")
	// Seed skeletons so the only staleness is the queue's (union endpoint
	// would otherwise add every undocumented dir).
	if err := os.MkdirAll(filepath.Join(dir, "internal", "model"), 0o755); err != nil {
		t.Fatal(err)
	}
	if _, err := docs.RefreshSkeletons(dir, s.docsQ.cfg, model.DocMeta{Refreshed: "2026-09-12", Source: "seed"}); err != nil {
		t.Fatal(err)
	}
	w := do(t, s.Handler(), "POST", "/changes/2026-09-10-0/close", "")
	if w.Code != 200 {
		t.Fatalf("close: %d", w.Code)
	}
	waitForCond(t, "stale flags", func() bool { return len(s.docsQ.staleDirs()) == 1 })

	runner := &fakeRunner{}
	s.SetDocsRunner(runner)
	w = do(t, s.Handler(), "POST", "/docs/refresh", "")
	if w.Code != 202 {
		t.Fatalf("refresh: %d %s", w.Code, w.Body)
	}
	waitForCond(t, "reconciliation job", func() bool { return len(runner.got()) == 1 })
	job := runner.got()[0]
	if job.Change != "manual" || strings.Join(job.Dirs, ",") != "internal/model" {
		t.Errorf("reconciliation job: %+v", job)
	}
	waitForCond(t, "stale cleared", func() bool { return len(s.docsQ.staleDirs()) == 0 })
}

func TestDocsRefreshUnionCoversHashStale(t *testing.T) {
	s, dir := docsServer(t)
	if err := os.MkdirAll(filepath.Join(dir, "internal", "model"), 0o755); err != nil {
		t.Fatal(err)
	}
	if _, err := docs.RefreshSkeletons(dir, s.docsQ.cfg, model.DocMeta{Refreshed: "2026-09-12", Source: "seed"}); err != nil {
		t.Fatal(err)
	}
	runner := &fakeRunner{}
	s.SetDocsRunner(runner)

	// Fresh: nothing to refresh.
	w := do(t, s.Handler(), "POST", "/docs/refresh", "")
	if w.Code != 200 || !strings.Contains(w.Body.String(), "nothing to refresh") {
		t.Fatalf("fresh: %d %s", w.Code, w.Body)
	}

	// Add a file: hash-stale at dir + ancestors; union enqueued as one job.
	if err := os.WriteFile(filepath.Join(dir, "internal", "model", "new.go"), []byte("x\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	w = do(t, s.Handler(), "POST", "/docs/refresh", "")
	if w.Code != 202 {
		t.Fatalf("hash-stale refresh: %d %s", w.Code, w.Body)
	}
	waitForCond(t, "union job", func() bool { return len(runner.got()) == 1 })
	job := runner.got()[0]
	want := []string{".", "internal", "internal/model"}
	if job.Change != "manual" || strings.Join(job.Dirs, ",") != strings.Join(want, ",") {
		t.Errorf("union job: %+v, want dirs %v", job, want)
	}
}

func TestCloseDocOnlyChangeEnqueuesNothing(t *testing.T) {
	s, dir := docsServer(t)
	writeTaskFilesAffected(t, dir, "2026-09-10-0", "00-first.md", `
- internal/model/AGENTS.md
- STRUCTURE.md
- changes/2026-09-10-0/ledger.md
`)
	runner := &fakeRunner{}
	s.SetDocsRunner(runner)
	w := do(t, s.Handler(), "POST", "/changes/2026-09-10-0/close", "")
	if w.Code != 200 {
		t.Fatalf("close: %d", w.Code)
	}
	// Give the worker a chance to (wrongly) pick something up.
	time.Sleep(200 * time.Millisecond)
	if len(runner.got()) != 0 {
		t.Errorf("doc-only change must not enqueue: %v", runner.got())
	}
	if len(s.docsQ.staleDirs()) != 0 {
		t.Errorf("doc-only change must not stale: %v", s.docsQ.staleDirs())
	}
}

func TestDocsDisabledWithoutConfig(t *testing.T) {
	st, _ := fixtureStore(t)
	s := New(st)
	t.Cleanup(s.Close)
	if s.docsQ != nil {
		t.Fatal("docs queue must be nil without agentsdocs.json")
	}
	w := do(t, s.Handler(), "POST", "/changes/2026-09-10-0/close", "")
	if w.Code != 200 {
		t.Fatalf("close without docs: %d", w.Code)
	}
	w = do(t, s.Handler(), "POST", "/docs/refresh", "")
	if w.Code != 200 || !strings.Contains(w.Body.String(), "disabled") {
		t.Errorf("refresh without docs: %d %s", w.Code, w.Body)
	}
}

func TestDocsQueueStatePersistsAcrossRestart(t *testing.T) {
	_, dir := docsServer(t) // only for the fixture repo layout
	cfg := docs.DefaultConfig()

	q1 := newDocsQueue(dir, cfg) // not started: no worker draining
	q1.enqueue(DocsJob{Change: "2026-09-10-0", Title: "T", Dirs: []string{"internal/model"}, Enqueued: time.Now().Format(time.RFC3339)})

	q2 := newDocsQueue(dir, cfg) // "restarted"
	q2.mu.Lock()
	pending := append([]DocsJob{}, q2.state.Pending...)
	q2.mu.Unlock()
	if len(pending) != 1 || pending[0].Change != "2026-09-10-0" {
		t.Fatalf("pending not restored: %v", pending)
	}

	// After a runner appears, the restored job drains.
	runner := &fakeRunner{}
	q2.start()
	defer q2.stop()
	q2.setRunner(runner)
	waitForCond(t, "restored job", func() bool { return len(runner.got()) == 1 })
}
