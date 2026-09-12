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
	Change   string   `json:"change"`   // change ID, or "manual"
	Title    string   `json:"title"`    // change title, for prompt context
	Dirs     []string `json:"dirs"`     // covered repo-relative dirs, sorted; may contain "."
	Enqueued string   `json:"enqueued"` // RFC3339
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
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Minute)
	err := runner.RunDocsJob(ctx, job)
	cancel()
	if err != nil {
		slog.Warn("docs job failed", "change", job.Change, "err", err)
		q.failStale(job, err.Error())
		return
	}
	q.mu.Lock()
	for _, d := range job.Dirs {
		delete(q.state.Stale, d)
	}
	q.persistLocked()
	q.mu.Unlock()
	slog.Info("docs job done", "change", job.Change, "dirs", job.Dirs)
}

func (q *docsQueue) failStale(job DocsJob, reason string) {
	q.mu.Lock()
	for _, d := range job.Dirs {
		q.state.Stale[d] = reason
	}
	q.persistLocked()
	q.mu.Unlock()
	slog.Warn("docs dirs stale", "change", job.Change, "dirs", job.Dirs, "reason", reason)
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
// close itself.
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
	if root, err := s.st.Root(); err == nil {
		for _, rr := range root.Rows {
			if rr.Change == changeID {
				title = rr.Title
				break
			}
		}
	}
	s.docsQ.enqueue(DocsJob{Change: changeID, Title: title, Dirs: dirs, Enqueued: time.Now().Format(time.RFC3339)})
}

// SetDocsRunner attaches the job executor (tests and, via SetOpencode, the
// gardener). A nil queue (docs disabled) ignores it.
func (s *Server) SetDocsRunner(r DocsRunner) {
	if s.docsQ != nil {
		s.docsQ.setRunner(r)
	}
}

// docsRefresh handles POST /docs/refresh: enqueue one manual reconciliation
// job for the union of queue-stale and hash-stale dirs.
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
	dirs := make([]string, 0, len(set))
	for d := range set {
		dirs = append(dirs, d)
	}
	sort.Strings(dirs)
	if len(dirs) == 0 {
		writeJSON(w, http.StatusOK, map[string]string{"status": "nothing to refresh"})
		return
	}
	s.docsQ.enqueue(DocsJob{Change: "manual", Title: "manual reconciliation", Dirs: dirs, Enqueued: time.Now().Format(time.RFC3339)})
	writeJSON(w, http.StatusAccepted, map[string]any{"enqueued": dirs})
}
