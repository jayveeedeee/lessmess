package server

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"testing"
	"time"

	"lessmess/internal/docs"
	"lessmess/internal/model"
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
	// "internal/model" contributes "internal"; "." is the root and has none.
	if got, wantAnc := strings.Join(job.Ancestors, ","), "internal"; got != wantAnc {
		t.Errorf("job ancestors %v, want %v", job.Ancestors, wantAnc)
	}
}

// A close-out touching more than docsJobMaxDirs dirs enqueues one job per
// chunk; ancestors are deduped across chunks and never shadow a primary
// target, so a failed job's stale set is bounded to its own chunk.
func TestCloseEnqueuesChunkedJobs(t *testing.T) {
	s, dir := docsServer(t)
	writeTaskFilesAffected(t, dir, "2026-09-10-0", "00-first.md", `
- README.md
- cmd/tasktracker/main.go
- internal/docs/seed.go
- internal/model/docfile.go
- internal/server/server.go
`)
	runner := &fakeRunner{}
	s.SetDocsRunner(runner)

	w := do(t, s.Handler(), "POST", "/changes/2026-09-10-0/close", "")
	if w.Code != 200 {
		t.Fatalf("close: %d %s", w.Code, w.Body)
	}
	// 5 touched dirs at 3 per chunk: 2 jobs.
	waitForCond(t, "chunked jobs", func() bool { return len(runner.got()) == 2 })

	wantDirs := []string{".", "cmd/tasktracker", "internal/docs", "internal/model", "internal/server"}
	var allDirs []string
	covered := map[string]bool{}
	for _, job := range runner.got() {
		if job.Change != "2026-09-10-0" || job.Title != "Fixture change" {
			t.Errorf("job identity: %+v", job)
		}
		if len(job.Dirs) > docsJobMaxDirs {
			t.Errorf("job has %d dirs, want at most %d", len(job.Dirs), docsJobMaxDirs)
		}
		for _, d := range append(append([]string{}, job.Dirs...), job.Ancestors...) {
			if covered[d] {
				t.Errorf("dir %s claimed by more than one job: %+v", d, runner.got())
			}
			covered[d] = true
		}
		allDirs = append(allDirs, job.Dirs...)
	}
	sort.Strings(allDirs)
	if strings.Join(allDirs, ",") != strings.Join(wantDirs, ",") {
		t.Errorf("union of job dirs = %v, want %v", allDirs, wantDirs)
	}
	// Nothing lost versus the old single-job semantics: the union of all
	// dirs and ancestors equals dirs ∪ coveredAncestors(dirs).
	want := map[string]bool{}
	for _, d := range append(append([]string{}, wantDirs...), coveredAncestors(wantDirs, s.docsQ.cfg)...) {
		want[d] = true
	}
	if len(covered) != len(want) {
		t.Errorf("covered set %v, want %v", covered, want)
	}
	for d := range want {
		if !covered[d] {
			t.Errorf("dir %s missing from the jobs' union", d)
		}
	}
}

func TestCoveredAncestors(t *testing.T) {
	cfg := docs.DefaultConfig()
	cases := []struct {
		name string
		dirs []string
		want []string
	}{
		{"nested", []string{"internal/model"}, []string{".", "internal"}},
		{"siblings dedup", []string{"internal/model", "internal/docs"}, []string{".", "internal"}},
		{"root dir has none", []string{"."}, nil},
		{"primary dirs excluded", []string{"internal", "internal/model"}, []string{"."}},
		{"empty", nil, nil},
	}
	for _, tc := range cases {
		got := coveredAncestors(tc.dirs, cfg)
		if strings.Join(got, ",") != strings.Join(tc.want, ",") {
			t.Errorf("%s: got %v, want %v", tc.name, got, tc.want)
		}
	}

	// Uncovered intermediates are skipped, but the walk still bubbles to
	// the covered root.
	tmp := t.TempDir()
	raw := []byte(`{"include":["**"],"exclude":["internal"]}` + "\n")
	if err := os.WriteFile(filepath.Join(tmp, docs.ConfigFile), raw, 0o644); err != nil {
		t.Fatal(err)
	}
	// LoadConfig only reads the config; coverage of missing dirs is lexical.
	cfg2, err := docs.LoadConfig(tmp)
	if err != nil {
		t.Fatal(err)
	}
	got := coveredAncestors([]string{"internal/model"}, cfg2)
	if strings.Join(got, ",") != "." {
		t.Errorf("uncovered intermediate: got %v, want [.]", got)
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
	// Ancestors are review-and-fix targets, so they stale alongside the
	// primary dirs and are reconcilable the same way.
	waitForCond(t, "stale flags", func() bool { return len(s.docsQ.staleDirs()) == 3 })
	if got := strings.Join(s.docsQ.staleDirs(), ","); got != ".,internal,internal/model" {
		t.Errorf("stale %v", got)
	}
	// State persisted.
	data, err := os.ReadFile(filepath.Join(dir, ".lessmess", "docs-queue.json"))
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
	waitForCond(t, "stale flags", func() bool { return len(s.docsQ.staleDirs()) == 3 })

	runner := &fakeRunner{}
	s.SetDocsRunner(runner)
	w = do(t, s.Handler(), "POST", "/docs/refresh", "")
	if w.Code != 202 {
		t.Fatalf("refresh: %d %s", w.Code, w.Body)
	}
	waitForCond(t, "reconciliation job", func() bool { return len(runner.got()) == 1 })
	job := runner.got()[0]
	if job.Change != "manual" || strings.Join(job.Dirs, ",") != ".,internal,internal/model" {
		t.Errorf("reconciliation job: %+v", job)
	}
	waitForCond(t, "stale cleared", func() bool { return len(s.docsQ.staleDirs()) == 0 })

	// The cleared flags must also reach disk: a restart must not resurrect
	// them from the persisted state.
	data, err := os.ReadFile(filepath.Join(dir, ".lessmess", "docs-queue.json"))
	if err != nil {
		t.Fatal(err)
	}
	var persisted docsQueueState
	if err := json.Unmarshal(data, &persisted); err != nil {
		t.Fatal(err)
	}
	if len(persisted.Stale) != 0 {
		t.Errorf("persisted stale after success: %v", persisted.Stale)
	}
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

func TestChunkDirs(t *testing.T) {
	var dirs []string
	for i := 0; i < 25; i++ {
		dirs = append(dirs, "dir")
	}
	chunks := chunkDirs(dirs, 10)
	if len(chunks) != 3 {
		t.Fatalf("chunks = %d, want 3", len(chunks))
	}
	for i, c := range chunks[:2] {
		if len(c) != 10 {
			t.Errorf("chunk %d len = %d, want 10", i, len(c))
		}
	}
	if len(chunks[2]) != 5 {
		t.Errorf("last chunk len = %d, want 5", len(chunks[2]))
	}
	if got := chunkDirs(nil, 10); len(got) != 0 {
		t.Errorf("empty input: %v", got)
	}
}

func TestJobTimeoutScalesWithDirs(t *testing.T) {
	cases := []struct {
		name      string
		dirs      []string
		ancestors []string
		want      time.Duration
	}{
		{"empty keeps the base", nil, nil, docsJobBaseTimeout},
		{"one dir", []string{"a"}, nil, docsJobBaseTimeout + docsJobPerDirTimeout},
		{"dirs and ancestors both count", []string{"a", "b", "c"}, []string{".", "x"}, docsJobBaseTimeout + 5*docsJobPerDirTimeout},
	}
	for _, tc := range cases {
		got := jobTimeout(DocsJob{Dirs: tc.dirs, Ancestors: tc.ancestors})
		if got != tc.want {
			t.Errorf("%s: jobTimeout = %v, want %v", tc.name, got, tc.want)
		}
	}
}

// deadlineRunner records the context deadline its job ran under.
type deadlineRunner struct {
	mu       sync.Mutex
	deadline time.Time
	ok       bool
}

func (d *deadlineRunner) RunDocsJob(ctx context.Context, _ DocsJob) error {
	d.mu.Lock()
	defer d.mu.Unlock()
	d.deadline, d.ok = ctx.Deadline()
	return nil
}

func (d *deadlineRunner) got() (time.Time, bool) {
	d.mu.Lock()
	defer d.mu.Unlock()
	return d.deadline, d.ok
}

// The worker's job context must carry jobTimeout's budget, not a constant.
func TestRunUsesScaledJobTimeout(t *testing.T) {
	_, dir := docsServer(t) // fixture repo layout only
	cfg := docs.DefaultConfig()
	q := newDocsQueue(dir, cfg)
	q.start()
	defer q.stop()

	job := DocsJob{
		Change:    "2026-09-10-0",
		Dirs:      []string{"internal/model", "internal/docs"},
		Ancestors: []string{".", "internal"},
		Enqueued:  time.Now().Format(time.RFC3339),
	}
	runner := &deadlineRunner{}
	start := time.Now()
	q.setRunner(runner)
	q.enqueue(job)
	waitForCond(t, "job under a scaled deadline", func() bool { _, ok := runner.got(); return ok })
	deadline, _ := runner.got()
	want := start.Add(jobTimeout(job))
	if deadline.Before(want) || deadline.After(want.Add(5*time.Second)) {
		t.Errorf("deadline = %v, want %v (pop latency allowance 5s)", deadline, want)
	}
}

// A union larger than docsJobMaxDirs splits into several sequential jobs
// so one fragile giant session cannot flag everything stale at once.
func TestDocsRefreshChunksLargeUnions(t *testing.T) {
	s, dir := docsServer(t)
	// 12 covered subdirs, none seeded: the hash-stale union is 13 dirs.
	for i := 0; i < 12; i++ {
		d := filepath.Join(dir, "pkg"+string(rune('a'+i)))
		if err := os.MkdirAll(d, 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(d, "x.go"), []byte("x"), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	runner := &fakeRunner{}
	s.SetDocsRunner(runner)
	w := do(t, s.Handler(), "POST", "/docs/refresh", "")
	if w.Code != 202 {
		t.Fatalf("refresh: %d %s", w.Code, w.Body)
	}
	// 13 dirs at docsJobMaxDirs per job: 4 full chunks plus a remainder.
	waitForCond(t, "chunked jobs", func() bool { return len(runner.got()) == 5 })
	for _, job := range runner.got() {
		if len(job.Dirs) > docsJobMaxDirs {
			t.Errorf("job dir count %d exceeds the chunk bound", len(job.Dirs))
		}
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

func TestDocsQueueRoundtripAncestorsAndRefs(t *testing.T) {
	_, dir := docsServer(t)
	cfg := docs.DefaultConfig()

	q1 := newDocsQueue(dir, cfg)
	q1.enqueue(DocsJob{
		Change:    "2026-09-10-0",
		Title:     "T",
		Dirs:      []string{"internal/model"},
		Ancestors: []string{".", "internal"},
		LintRefs:  map[string][]string{"internal/model": {"gone/x.go"}},
		Enqueued:  time.Now().Format(time.RFC3339),
	})

	q2 := newDocsQueue(dir, cfg)
	q2.mu.Lock()
	pending := append([]DocsJob{}, q2.state.Pending...)
	q2.mu.Unlock()
	if len(pending) != 1 {
		t.Fatalf("pending not restored: %v", pending)
	}
	job := pending[0]
	if strings.Join(job.Ancestors, ",") != ".,internal" {
		t.Errorf("ancestors = %v", job.Ancestors)
	}
	if len(job.LintRefs["internal/model"]) != 1 || job.LintRefs["internal/model"][0] != "gone/x.go" {
		t.Errorf("lintRefs = %v", job.LintRefs)
	}
}

func TestDocsRefreshUnionCoversLintStale(t *testing.T) {
	s, dir := docsServer(t)
	if err := os.MkdirAll(filepath.Join(dir, "internal", "model"), 0o755); err != nil {
		t.Fatal(err)
	}
	// Seed skeletons so the only staleness is the lint's (the union would
	// otherwise include every undocumented dir as hash-stale).
	if _, err := docs.RefreshSkeletons(dir, s.docsQ.cfg, model.DocMeta{Refreshed: "2026-09-12", Source: "seed"}); err != nil {
		t.Fatal(err)
	}
	// A learning citing a path that does not exist.
	agents := model.DocMarkerBegin + "\n- (2026-09-10-0) see `gone/deleted.go` for details\n" + model.DocMarkerEnd + "\n"
	if err := os.WriteFile(filepath.Join(dir, "internal", "model", docs.AgentsFile), []byte(agents), 0o644); err != nil {
		t.Fatal(err)
	}

	runner := &fakeRunner{}
	s.SetDocsRunner(runner)
	w := do(t, s.Handler(), "POST", "/docs/refresh", "")
	if w.Code != 202 {
		t.Fatalf("refresh: %d %s", w.Code, w.Body)
	}
	waitForCond(t, "lint union job", func() bool { return len(runner.got()) == 1 })
	job := runner.got()[0]
	if job.Change != "manual" || strings.Join(job.Dirs, ",") != "internal/model" {
		t.Fatalf("union job: %+v", job)
	}
	refs := job.LintRefs["internal/model"]
	if len(refs) != 1 || refs[0] != "gone/deleted.go" {
		t.Errorf("lintRefs = %v, want [gone/deleted.go]", refs)
	}
}
