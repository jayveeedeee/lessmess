package server

import (
	"context"
	"encoding/json"
	"log/slog"
	"net/http"
	"os"
	"path/filepath"
	"sort"
	"sync"
	"time"

	"lessmess/internal/docs"
	"lessmess/internal/model"
)

// DocsJob is one queued doc-gardener unit of work: refresh the doc pairs of
// Dirs on behalf of a closed change (or a manual reconciliation).
type DocsJob struct {
	Change    string              `json:"change"`              // change ID, or "manual"
	Title     string              `json:"title"`               // change title, for prompt context
	Dirs      []string            `json:"dirs"`                // covered repo-relative dirs, sorted; may contain "."
	Ancestors []string            `json:"ancestors,omitempty"` // covered ancestors of Dirs: review-and-fix targets
	LintRefs  map[string][]string `json:"lintRefs,omitempty"`  // manual jobs: dir → missing paths flagged by the reference lint
	Enqueued  string              `json:"enqueued"`            // RFC3339
}

// DocsRunner executes one job. Implemented by the doc gardener (DOC-06);
// tests inject fakes.
type DocsRunner interface {
	RunDocsJob(ctx context.Context, job DocsJob) error
}

// docsQueuePath is the queue/stale state file (gitignored tooling state).
// Pending and stale live in one file so a single atomic write keeps them
// consistent.
const docsQueuePath = ".lessmess/docs-queue.json"

type docsQueueState struct {
	Pending []DocsJob         `json:"pending"`
	Stale   map[string]string `json:"stale"` // covered dir -> failure reason
}

// docsQueue is the serialized (single-writer) refresh queue. One worker
// goroutine drains Pending FIFO; failures move the job's dirs to Stale, which
// POST /docs/refresh reconciles.
type docsQueue struct {
	root string
	cfg  *docs.Config

	mu     sync.Mutex
	state  docsQueueState
	runner DocsRunner

	wake   chan struct{}
	stopCh chan struct{}
	done   chan struct{}
}

func newDocsQueue(root string, cfg *docs.Config) *docsQueue {
	q := &docsQueue{
		root:   root,
		cfg:    cfg,
		wake:   make(chan struct{}, 1),
		stopCh: make(chan struct{}),
		done:   make(chan struct{}),
	}
	q.state.Stale = map[string]string{}
	q.load()
	return q
}

func (q *docsQueue) statePath() string { return filepath.Join(q.root, docsQueuePath) }

func (q *docsQueue) load() {
	data, err := os.ReadFile(q.statePath())
	if err != nil {
		return
	}
	_ = json.Unmarshal(data, &q.state)
	if q.state.Stale == nil {
		q.state.Stale = map[string]string{}
	}
}

// persistLocked writes the state file; caller holds q.mu.
func (q *docsQueue) persistLocked() {
	data, err := json.MarshalIndent(q.state, "", "  ")
	if err != nil {
		slog.Error("docs queue marshal", "err", err)
		return
	}
	if err := os.MkdirAll(filepath.Dir(q.statePath()), 0o755); err != nil {
		slog.Error("docs queue mkdir", "err", err)
		return
	}
	if err := model.WriteFileAtomic(q.statePath(), append(data, '\n'), 0o644); err != nil {
		slog.Error("docs queue persist", "err", err)
	}
}

// start launches the worker. Pending jobs left by a previous process drain
// once a runner is (or becomes) available; without a runner they fail to
// stale immediately, which is the service-down fallback by design.
func (q *docsQueue) start() {
	go func() {
		defer close(q.done)
		for {
			job, ok := q.pop()
			if !ok {
				select {
				case <-q.wake:
					continue
				case <-q.stopCh:
					return
				}
			}
			q.run(job)
		}
	}()
}

func (q *docsQueue) stop() {
	close(q.stopCh)
	<-q.done
}

func (q *docsQueue) setRunner(r DocsRunner) {
	q.mu.Lock()
	q.runner = r
	q.mu.Unlock()
	select {
	case q.wake <- struct{}{}:
	default:
	}
}

func (q *docsQueue) enqueue(job DocsJob) {
	q.mu.Lock()
	q.state.Pending = append(q.state.Pending, job)
	q.persistLocked()
	q.mu.Unlock()
	slog.Info("docs job enqueued", "change", job.Change, "dirs", job.Dirs)
	select {
	case q.wake <- struct{}{}:
	default:
	}
}

func (q *docsQueue) pop() (DocsJob, bool) {
	q.mu.Lock()
	defer q.mu.Unlock()
	if len(q.state.Pending) == 0 {
		return DocsJob{}, false
	}
	job := q.state.Pending[0]
	q.state.Pending = q.state.Pending[1:]
	q.persistLocked()
	return job, true
}

// run executes one job. On failure the job is dropped from Pending and its
// dirs become stale (reconcilable via POST /docs/refresh); on success any
// stale flags for those dirs clear.
func (q *docsQueue) run(job DocsJob) {
	q.mu.Lock()
	runner := q.runner
	q.mu.Unlock()
	if runner == nil {
		q.failStale(job, "opencode service unavailable")
		return
	}
	ctx, cancel := context.WithTimeout(context.Background(), jobTimeout(job))
	err := runner.RunDocsJob(ctx, job)
	cancel()
	if err != nil {
		slog.Warn("docs job failed", "change", job.Change, "err", err)
		q.failStale(job, err.Error())
		return
	}
	q.mu.Lock()
	for _, d := range append(append([]string{}, job.Dirs...), job.Ancestors...) {
		delete(q.state.Stale, d)
	}
	// Persist the clears too: without this, a restart would resurrect
	// stale flags from disk for dirs the job just refreshed.
	q.persistLocked()
	q.mu.Unlock()
	slog.Info("docs job done", "change", job.Change, "dirs", job.Dirs, "ancestors", job.Ancestors)
}

// failStale flags the job's dirs and ancestors with the failure reason.
func (q *docsQueue) failStale(job DocsJob, reason string) {
	q.mu.Lock()
	for _, d := range append(append([]string{}, job.Dirs...), job.Ancestors...) {
		q.state.Stale[d] = reason
	}
	q.persistLocked()
	q.mu.Unlock()
	slog.Warn("docs dirs stale", "change", job.Change, "dirs", job.Dirs, "ancestors", job.Ancestors, "reason", reason)
}

// staleDirs returns the sorted stale set.
func (q *docsQueue) staleDirs() []string {
	q.mu.Lock()
	defer q.mu.Unlock()
	dirs := make([]string, 0, len(q.state.Stale))
	for d := range q.state.Stale {
		dirs = append(dirs, d)
	}
	sort.Strings(dirs)
	return dirs
}

// staleReasons returns a copy of the stale map (dir -> reason).
func (q *docsQueue) staleReasons() map[string]string {
	q.mu.Lock()
	defer q.mu.Unlock()
	out := make(map[string]string, len(q.state.Stale))
	for d, r := range q.state.Stale {
		out[d] = r
	}
	return out
}

// enqueueDocsRefresh is the close-out hook: best-effort, never failing the
// close itself. The touched dirs are chunked (docsJobMaxDirs) into one job
// each, so a failed job flags and reverts only its own chunk; ancestors are
// claimed by the earliest chunk that touches their subtree and never shadow
// a primary target — every directory is updated or reviewed exactly once
// across the change's jobs.
func (s *Server) enqueueDocsRefresh(changeID string) {
	if s.docsQ == nil {
		return
	}
	dirs, err := touchedDocsDirs(s.st.Dir, changeID, s.docsQ.cfg)
	if err != nil {
		slog.Warn("docs refresh: compute touched dirs", "change", changeID, "err", err)
		return
	}
	if len(dirs) == 0 {
		return
	}
	title := changeID
	if e := s.st.Entry(changeID); e != nil {
		title = e.Title
	}
	primary := make(map[string]bool, len(dirs))
	for _, d := range dirs {
		primary[d] = true
	}
	now := time.Now().Format(time.RFC3339)
	seen := map[string]bool{}
	for _, chunk := range chunkDirs(dirs, docsJobMaxDirs) {
		var ancestors []string
		for _, a := range coveredAncestors(chunk, s.docsQ.cfg) {
			if primary[a] || seen[a] {
				continue
			}
			seen[a] = true
			ancestors = append(ancestors, a)
		}
		s.docsQ.enqueue(DocsJob{
			Change:    changeID,
			Title:     title,
			Dirs:      chunk,
			Ancestors: ancestors,
			Enqueued:  now,
		})
	}
}

// SetDocsRunner attaches the job executor (tests and, via SetOpencode, the
// gardener). A nil queue (docs disabled) ignores it.
func (s *Server) SetDocsRunner(r DocsRunner) {
	if s.docsQ != nil {
		s.docsQ.setRunner(r)
	}
}

// docsJobMaxDirs bounds one docs job — both close-out chunks and manual
// reconciliation unions: a job covering many directories becomes one long
// gardener session whose failure flags every covered dir stale at once
// (observed 2026-09-19: whole-tree close-out and refresh jobs all died at
// the fixed job budget while still busy). Three dirs keeps a session
// comfortably inside the scaled job budget; the serialized queue drains
// the chunks sequentially.
const docsJobMaxDirs = 3

// The gardener session budget: a fixed 15 minutes provably cannot fit a
// multi-dir session (2026-09-19: three consecutive jobs died at exactly
// 15:00 while still busy; the model gardens on the order of five minutes
// per directory), so the budget scales with the job's target count.
const (
	docsJobBaseTimeout   = 15 * time.Minute
	docsJobPerDirTimeout = 5 * time.Minute
)

// jobTimeout returns the gardener session budget for one job: the base
// plus per-dir time for every update and review target — one session
// gardens both lists. A 1-dir job keeps today's effective budget (20 min);
// a 3-dir chunk with ancestors gets 30–40+.
func jobTimeout(job DocsJob) time.Duration {
	return docsJobBaseTimeout + time.Duration(len(job.Dirs)+len(job.Ancestors))*docsJobPerDirTimeout
}

// chunkDirs splits dirs into consecutive chunks of at most size entries.
func chunkDirs(dirs []string, size int) [][]string {
	var out [][]string
	for i := 0; i < len(dirs); i += size {
		end := i + size
		if end > len(dirs) {
			end = len(dirs)
		}
		out = append(out, dirs[i:end])
	}
	return out
}

// docsRefresh handles POST /docs/refresh: enqueue manual reconciliation
// jobs for the union of queue-stale, hash-stale, and lint-flagged dirs,
// chunked to at most docsJobMaxDirs directories per job.
func (s *Server) docsRefresh(w http.ResponseWriter, r *http.Request) {
	if s.docsQ == nil {
		writeJSON(w, http.StatusOK, map[string]string{"status": "docs system disabled"})
		return
	}
	set := map[string]bool{}
	for _, d := range s.docsQ.staleDirs() {
		set[d] = true
	}
	hashStale, err := docs.StaleDirs(s.st.Dir)
	if err != nil {
		slog.Warn("docs refresh: hash staleness check", "err", err)
	} else {
		for _, d := range hashStale {
			set[d] = true
		}
	}
	lintRefs, err := docs.StaleLearningRefs(s.st.Dir)
	if err != nil {
		slog.Warn("docs refresh: reference lint", "err", err)
	} else {
		for d := range lintRefs {
			set[d] = true
		}
	}
	dirs := make([]string, 0, len(set))
	for d := range set {
		dirs = append(dirs, d)
	}
	sort.Strings(dirs)
	if len(dirs) == 0 {
		writeJSON(w, http.StatusOK, map[string]string{"status": "nothing to refresh"})
		return
	}
	for _, chunk := range chunkDirs(dirs, docsJobMaxDirs) {
		// Lint refs ride along only for dirs this job covers, so the
		// gardener prompt names exactly the flagged references it can fix.
		chunkRefs := map[string][]string{}
		for _, d := range chunk {
			if refs := lintRefs[d]; len(refs) > 0 {
				chunkRefs[d] = refs
			}
		}
		s.docsQ.enqueue(DocsJob{
			Change:   "manual",
			Title:    "manual reconciliation",
			Dirs:     chunk,
			LintRefs: chunkRefs,
			Enqueued: time.Now().Format(time.RFC3339),
		})
	}
	writeJSON(w, http.StatusAccepted, map[string]any{"enqueued": dirs})
}
