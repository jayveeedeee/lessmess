package server

// Shared server-side docs seed job: one in-flight job per repository, started
// by normal-server POST /docs/seed and polled via /docs/seed-status. The job
// runs docs.Seed in a goroutine with a summarizer honoring the configured
// session agent/model; progress lines accumulate in memory for the poller.

import (
	"context"
	"log/slog"
	"strings"
	"sync"
	"time"

	"lessmess/internal/docs"
	"lessmess/internal/opencode"
)

// newSeedSummarizer builds the summarizer for a seed run; tests swap it to
// capture the agent/model and stub the LLM sessions.
var newSeedSummarizer = func(c docs.SessionClient, wait time.Duration, agent, model string) docs.Summarizer {
	return docs.NewOpenCodeSummarizerWith(c, wait, agent, model)
}

const seedMaxLines = 200

// docsSeedJob tracks one seed run.
type docsSeedJob struct {
	mu      sync.Mutex
	running bool
	done    bool
	err     string
	lines   []string
}

// seedJobs is the registry of in-flight/completed jobs keyed by absolute repo
// dir, keeping duplicate normal-server starts out of the same repository.
var seedJobs = struct {
	sync.Mutex
	byDir map[string]*docsSeedJob
}{byDir: map[string]*docsSeedJob{}}

func seedJobFor(dir string) *docsSeedJob {
	seedJobs.Lock()
	defer seedJobs.Unlock()
	j, ok := seedJobs.byDir[dir]
	if !ok {
		j = &docsSeedJob{}
		seedJobs.byDir[dir] = j
	}
	return j
}

// lineWriter feeds Seed's progress output into the job's bounded line buffer.
type lineWriter struct {
	job *docsSeedJob
	buf strings.Builder
}

func (w *lineWriter) Write(p []byte) (int, error) {
	w.buf.Write(p)
	for {
		s := w.buf.String()
		i := strings.IndexByte(s, '\n')
		if i < 0 {
			break
		}
		w.job.appendLine(strings.TrimRight(s[:i], "\r"))
		w.buf.Reset()
		w.buf.WriteString(s[i+1:])
	}
	return len(p), nil
}

func (w *lineWriter) flush() {
	if s := strings.TrimRight(w.buf.String(), "\r\n"); s != "" {
		w.job.appendLine(s)
		w.buf.Reset()
	}
}

func (j *docsSeedJob) appendLine(s string) {
	j.mu.Lock()
	defer j.mu.Unlock()
	j.lines = append(j.lines, s)
	if len(j.lines) > seedMaxLines {
		j.lines = j.lines[len(j.lines)-seedMaxLines:]
	}
}

// start runs the seed if none is in flight; ok=false means already running.
func (j *docsSeedJob) start(run func(out *lineWriter) error) (ok bool) {
	j.mu.Lock()
	if j.running {
		j.mu.Unlock()
		return false
	}
	j.running, j.done, j.err, j.lines = true, false, "", nil
	j.mu.Unlock()
	go func() {
		w := &lineWriter{job: j}
		err := run(w)
		w.flush()
		j.mu.Lock()
		j.running, j.done = false, true
		if err != nil {
			j.err = err.Error()
		}
		j.mu.Unlock()
	}()
	return true
}

func (j *docsSeedJob) status() docsSeedStatusResponse {
	j.mu.Lock()
	defer j.mu.Unlock()
	lines := make([]string, len(j.lines))
	copy(lines, j.lines)
	return docsSeedStatusResponse{Running: j.running, Done: j.done, Error: j.err, Lines: lines}
}

type docsSeedRequest struct {
	Budget int  `json:"budget"`
	Force  bool `json:"force"` // redo every covered dir, ignoring the cursor
}

type docsSeedStatusResponse struct {
	Running bool     `json:"running"`
	Done    bool     `json:"done"`
	Error   string   `json:"error,omitempty"`
	Lines   []string `json:"lines"`
}

// startDocsSeedJob starts the one in-flight seed job for the absolute repo
// dir; ok=false means one is already running. The resumable cursor
// (.lessmess/docs-seed.json) is shared with CLI seed runs.
func startDocsSeedJob(root string, cfg *docs.Config, oc *opencode.Client, opts docs.SeedOptions) bool {
	agent, model := SessionDefaults(root)
	sum := newSeedSummarizer(oc, 0, agent, model)
	job := seedJobFor(root)
	if !job.start(func(out *lineWriter) error {
		slog.Info("docs seed started", "dir", root, "budget", opts.Budget, "force", opts.Force, "agent", agent, "model", model)
		err := docs.Seed(context.Background(), root, cfg, sum, opts, out)
		if err != nil {
			slog.Warn("docs seed finished with failures", "dir", root, "err", err)
		} else {
			slog.Info("docs seed finished", "dir", root)
		}
		return err
	}) {
		return false
	}
	return true
}
