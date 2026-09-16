// Package server exposes the store over HTTP: HTML pages, JSON write
// endpoints, and an SSE stream for live updates.
package server

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"lessmess/internal/docs"
	"lessmess/internal/model"
	"lessmess/internal/opencode"
	"lessmess/internal/store"
	"lessmess/internal/terminal"
)

// Server routes requests to the store.
type Server struct {
	st   *store.Store
	mux  *http.ServeMux
	rend *renderer
	term *terminal.Manager
	// SpawnCommand builds the command run in a PTY for a session ID.
	// Overridable in tests.
	SpawnCommand func(sessionID string) (string, []string)
	// PublicBase is the host:port the server listens on, used in
	// agent-facing prompts. Set by main; empty falls back to a default.
	PublicBase string

	oc       *opencode.Client // nil disables the opencode integration
	sessions *mapping
	mapErr   error
	autos    *autosession // once-only markers for auto-spawned task sessions
	docsQ    *docsQueue   // nil disables the docs system (no agentsdocs.json)
	docsW    *docsWatcher // nil when docs are disabled or the watcher failed
}

// New builds the route table.
func New(st *store.Store) *Server {
	if err := store.MigrateStateDir(st.Dir); err != nil {
		slog.Warn("state dir migration skipped", "err", err)
	}
	s := &Server{st: st, rend: newRenderer(), term: terminal.NewManager()}
	s.SpawnCommand = func(sessionID string) (string, []string) {
		return "opencode2", []string{"--session", sessionID}
	}
	m, err := loadMapping(filepath.Join(st.Dir, store.StateDirName, "sessions.json"))
	s.sessions, s.mapErr = m, err
	s.autos = loadAutosession(filepath.Join(st.Dir, store.StateDirName, "autosession.json"))

	if cfg, err := docs.LoadConfig(st.Dir); err != nil {
		slog.Warn("docs config unreadable; docs system disabled", "err", err)
	} else if cfg != nil {
		s.docsQ = newDocsQueue(st.Dir, cfg)
		s.docsQ.start()
		if dw, err := newDocsWatcher(st.Dir, cfg); err != nil {
			slog.Warn("docs watcher disabled", "err", err)
		} else {
			s.docsW = dw
		}
	}

	mux := http.NewServeMux()
	mux.HandleFunc("GET /{$}", s.index)
	mux.HandleFunc("GET /changes/{id}", s.board)
	mux.HandleFunc("GET /changes/{id}/plan", s.planDetail)
	mux.HandleFunc("GET /changes/{id}/ledger", s.ledgerDetail)
	mux.HandleFunc("GET /changes/{id}/tasks/{file...}", s.taskDetail)
	mux.HandleFunc("POST /changes/{id}/tasks", s.createTask)
	mux.HandleFunc("POST /changes/{id}/expand", s.expandTask)
	mux.HandleFunc("POST /changes/{id}/move", s.moveTask)
	mux.HandleFunc("POST /changes/{id}/close", s.closeChange)
	mux.HandleFunc("POST /changes/{id}/reopen", s.reopenChange)
	mux.HandleFunc("POST /changes/{id}/status", s.changeStatus)
	mux.HandleFunc("POST /changes/{id}/commit", s.commitChange)
	mux.HandleFunc("GET /changes/{id}/commit-status", s.commitStatus)
	mux.HandleFunc("POST /changes/{$}", s.createChange)
	mux.HandleFunc("POST /changes/session", s.createDiscussionSession)
	mux.HandleFunc("POST /changes/scaffold", s.scaffoldChange)
	mux.HandleFunc("GET /events", s.events)
	mux.HandleFunc("GET /api/validate", s.validate)
	mux.HandleFunc("GET /terminal/ws", s.terminalWS)
	mux.HandleFunc("GET /changes/{id}/sessions", s.listChangeSessions)
	mux.HandleFunc("POST /changes/{id}/sessions", s.createChangeSession)
	mux.HandleFunc("POST /changes/{id}/task-sessions", s.bindTaskSession)
	mux.HandleFunc("DELETE /changes/{id}/sessions/{sessionID}", s.unlinkChangeSession)
	mux.HandleFunc("GET /api/discussions", s.listDiscussions)
	mux.HandleFunc("DELETE /api/discussions/{sessionID}", s.unlinkDiscussion)
	mux.HandleFunc("GET /api/sessions/{sessionID}/change", s.sessionChange)
	mux.HandleFunc("GET /api/git/status", s.gitStatusAPI)
	mux.HandleFunc("POST /api/git/commit", s.commitAll)
	mux.HandleFunc("GET /api/git/commit-status", s.commitStatus)
	mux.HandleFunc("POST /docs/refresh", s.docsRefresh)
	mux.HandleFunc("POST /docs/seed", s.docsSeed)
	mux.HandleFunc("GET /docs/seed-status", s.docsSeedStatus)
	mux.HandleFunc("GET /docs/exclusions", s.docsExclusions)
	mux.HandleFunc("POST /docs/exclusions", s.docsExclusionsSave)
	mux.HandleFunc("GET /api/settings", s.getSettings)
	mux.HandleFunc("PUT /api/settings", s.putSettings)
	mux.HandleFunc("GET /api/settings/options", s.settingsOptions)
	mux.HandleFunc("GET /settings", s.settingsPage)
	mux.HandleFunc("POST /api/settings/change", s.settingsChange)
	mux.HandleFunc("GET /explorer", s.explorer)
	mux.HandleFunc("GET /explorer/tree", s.explorerTree)
	mux.HandleFunc("GET /explorer/detail", s.explorerDetail)
	mux.HandleFunc("POST /explorer/chat", s.explorerChat)
	// Dynamic brand assets: the effective accent injected into the icon
	// SVG, the favicon rasters, and the apple-touch tile.
	s.rend.accent = func() AccentColor { return ResolveAccent(s.st.Dir) }
	brand := newBrandRenderer(s.rend.accent)
	mux.HandleFunc("GET /icon.svg", brand.svg)
	mux.HandleFunc("GET /favicon.ico", brand.ico)
	mux.HandleFunc("GET /apple-touch-icon.png", brand.touch)
	registerSetupRoutes(mux, &setupEnv{
		dir:  st.Dir,
		rend: s.rend,
		oc:   func() *opencode.Client { return s.oc },
	})
	if sh, err := staticHandler(); err == nil {
		mux.Handle("GET /static/", sh)
	}
	s.mux = mux
	return s
}

// SetOpencode attaches the opencode client (nil disables the integration)
// and, when the docs system is enabled, the gardener job runner.
func (s *Server) SetOpencode(c *opencode.Client) {
	s.oc = c
	if c != nil && s.docsQ != nil {
		gr := &gardenerRunner{oc: c, root: s.st.Dir, cfg: s.docsQ.cfg}
		gr.spawn = func(ctx context.Context, title string) (*opencode.Session, error) {
			model := GardenerModel(s.st.Dir)
			if model != "" {
				slog.Info("gardener session model", "model", model)
			}
			return spawnSessionWithModel(ctx, c, s.st.Dir, title, model)
		}
		s.docsQ.setRunner(gr)
	}
}

// Close releases resources (terminal PTYs, docs queue worker, docs watcher).
func (s *Server) Close() {
	if s.docsQ != nil {
		s.docsQ.stop()
	}
	if s.docsW != nil {
		_ = s.docsW.Close()
	}
	s.term.CloseAll()
}

// Handler returns the root http.Handler.
func (s *Server) Handler() http.Handler { return s.mux }

// --- helpers ---

func wantsHTML(r *http.Request) bool {
	accept := r.Header.Get("Accept")
	return strings.Contains(accept, "text/html") && !strings.Contains(accept, "application/json")
}

func isHX(r *http.Request) bool { return r.Header.Get("HX-Request") == "true" }

func writeJSON(w http.ResponseWriter, code int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(code)
	_ = json.NewEncoder(w).Encode(v)
}

func writeErr(w http.ResponseWriter, err error) {
	switch {
	case errors.Is(err, store.ErrNotFound):
		writeJSON(w, http.StatusNotFound, map[string]string{"error": err.Error()})
	case errors.Is(err, store.ErrInvalid), errors.Is(err, store.ErrNoContainer):
		writeJSON(w, http.StatusUnprocessableEntity, map[string]string{"error": err.Error()})
	default:
		slog.Error("handler", "err", err)
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "internal error"})
	}
}

// --- read endpoints ---

type changeSummary struct {
	ID      string `json:"id"`
	Title   string `json:"title"`
	Prefix  string `json:"prefix"`
	Status  string `json:"status"`
	Updated string `json:"updated"`
	Tasks    int    `json:"tasks"`
	Complete int    `json:"complete"` // Test + Done descendants
	Open     int    `json:"open"`     // non-cancelled descendants not yet complete
}

func (s *Server) index(w http.ResponseWriter, r *http.Request) {
	root, err := s.st.Root()
	if err != nil {
		writeErr(w, fmt.Errorf("%w: %v", store.ErrInvalid, err))
		return
	}
	showArchived := s.effectiveSettings().UI.ShowArchived
	rows := make([]changeSummary, 0, len(root.Rows))
	pos := make([]int, 0, len(root.Rows))
	byID := map[string]*store.Change{}
	for _, c := range s.st.Changes() {
		byID[c.ID] = c
	}
	for i, rr := range root.Rows {
		if !showArchived && strings.HasPrefix(rr.Href, "archive/") {
			continue
		}
		sum := changeSummary{ID: rr.Change, Title: rr.Title, Prefix: rr.Prefix, Status: string(rr.Status), Updated: rr.Updated}
		if c, ok := byID[rr.Change]; ok && c.Ledger != nil {
			// Recursive: every task in the tree, at any depth.
			c.WalkTasks(func(n *store.TaskNode) bool {
				sum.Tasks++
				return true
			})
			st := c.AllTaskStats()
			sum.Complete = st.Complete
			sum.Open = st.Total - st.Complete
		}
		rows = append(rows, sum)
		pos = append(pos, i)
	}
	// Newest change first: the date prefix dominates, and within a date the
	// root-ledger row position decides (rows are append-mostly, so a later
	// position is newer). Change IDs end in a random five-character suffix
	// with no inherent order, and legacy numeric suffixes are no longer
	// allocation-ordered either, so the position is the only chronological
	// signal; ID ascending breaks ties deterministically.
	sort.Slice(rows, func(i, j int) bool {
		ai, aj := datePrefixOf(rows[i].ID), datePrefixOf(rows[j].ID)
		if ai != aj {
			return ai > aj
		}
		if pos[i] != pos[j] {
			return pos[i] > pos[j]
		}
		return rows[i].ID < rows[j].ID
	})
	if wantsHTML(r) {
		view := indexView{Changes: rows, OnboardingPending: onboardingPending(s.st.Dir)}
		if gs := gitStatus(s.st.Dir); gs.Repo {
			view.GitRepo = true
			view.GitDirty = len(gs.Changes) > 0
		}
		s.rend.render(w, s.rend.index, "layout", pageData{Title: "changes", Page: "index", Data: view})
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"changes": rows})
}

// datePrefixOf returns the YYYY-MM-DD prefix of a change ID, or "" when the
// ID is too short to have one.
func datePrefixOf(id string) string {
	if len(id) < 10 {
		return ""
	}
	return id[:10]
}

type boardResponse struct {
	ID            string          `json:"id"`
	Task          string          `json:"task,omitempty"` // drill-down task ID
	Overall       string          `json:"overall"`
	Columns       []columnView    `json:"columns"`
	Tasks         []model.TaskRow `json:"tasks"`
	ProgressDone  int             `json:"progressDone"`
	ProgressTotal int             `json:"progressTotal"`
	Error         string          `json:"error,omitempty"`
}

type columnView struct {
	Status string `json:"status"`
	Count  int    `json:"count"`
}

func (s *Server) board(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	c, err := s.st.Change(id)
	if err != nil {
		writeErr(w, err)
		return
	}
	// Drill-down: ?task=<id> renders that task's sub-board (its children
	// in the kanban); absent renders the change's root board.
	taskID := strings.TrimSpace(r.URL.Query().Get("task"))
	var view boardView
	if taskID != "" {
		n := c.Node(taskID)
		if n == nil {
			writeErr(w, store.ErrNotFound)
			return
		}
		view = newTaskBoardView(c, n)
	} else {
		view = newBoardView(c)
	}
	// Auto-spawn (best-effort, async): newly decomposed tasks visible on
	// a board render get their task-scoped session exactly once.
	if s.oc != nil && s.mapErr == nil {
		go s.autospawnChange(c, 1)
	}
	if wantsHTML(r) {
		if isHX(r) {
			s.rend.render(w, s.rend.partial, "boardFragment", view)
		} else {
			s.rend.render(w, s.rend.board, "layout", pageData{Title: id, Page: "board", Data: view})
		}
		return
	}
	resp := boardResponse{ID: id, Task: view.Task}
	if c.Err != nil || c.Ledger == nil {
		resp.Error = fmt.Sprintf("ledger unreadable: %v", c.Err)
		writeJSON(w, http.StatusUnprocessableEntity, resp)
		return
	}
	resp.Overall = string(c.Ledger.Overall)
	resp.Tasks = nodeRows(c.Roots)
	for _, col := range view.Columns {
		resp.Columns = append(resp.Columns, columnView{Status: col.Status, Count: len(col.Tasks)})
	}
	resp.ProgressDone = view.ProgressDone
	resp.ProgressTotal = view.ProgressTotal
	writeJSON(w, http.StatusOK, resp)
}

// expandTask handles POST /changes/{id}/expand: decompose a task into a
// sub plan (creates the container). User-instructed only — this endpoint
// is the board's explicit action.
func (s *Server) expandTask(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	var req struct {
		Task string `json:"task"`
	}
	if isJSON(r) {
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			writeJSON(w, http.StatusBadRequest, map[string]string{"error": "bad JSON body"})
			return
		}
	} else {
		_ = r.ParseForm()
		req.Task = r.FormValue("task")
	}
	if strings.TrimSpace(req.Task) == "" {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "task is required"})
		return
	}
	rel, err := s.st.DecomposeTask(id, strings.TrimSpace(req.Task))
	if err != nil {
		if errors.Is(err, store.ErrContainerExists) {
			writeJSON(w, http.StatusConflict, map[string]string{"error": err.Error()})
			return
		}
		writeErr(w, err)
		return
	}
	slog.Info("task expanded", "change", id, "task", req.Task, "container", rel)
	// Best-effort, async: the new container's task-scoped session.
	if c, err := s.st.Change(id); err == nil {
		go s.autospawnChange(c, 1)
	}
	writeJSON(w, http.StatusCreated, map[string]string{"change": id, "task": req.Task, "container": rel})
}

func (s *Server) taskDetail(w http.ResponseWriter, r *http.Request) {
	id, file := r.PathValue("id"), r.PathValue("file")
	tf, err := s.st.TaskFile(id, "tasks/"+file)
	if err != nil {
		writeErr(w, err)
		return
	}
	if wantsHTML(r) || isHX(r) {
		view := taskView{Task: tf, Body: dropLeadingH1(tf.Body, tf.ID), Doc: "tasks/" + file}
		if c, err := s.st.Change(id); err == nil {
			// Status lives in the governing ledger: the tree node knows
			// its row wherever the task nests.
			if n := c.Node(tf.ID); n != nil && n.Row != nil {
				view.Status = string(n.Row.Status)
			} else if c.Ledger != nil {
				if row := c.Ledger.Row(tf.ID); row != nil {
					view.Status = string(row.Status)
				}
			}
		}
		s.rend.render(w, s.rend.partial, "taskDetail", view)
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"id": tf.ID, "title": tf.Title, "body": tf.Body})
}

// planDetail serves the change's plan.md rendered in the detail modal.
func (s *Server) planDetail(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	body, err := s.st.PlanFile(id)
	if err != nil {
		writeErr(w, err)
		return
	}
	if wantsHTML(r) || isHX(r) {
		s.rend.render(w, s.rend.partial, "planDetail", planView{ID: id, Body: body})
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"id": id, "body": body})
}

// ledgerDetail serves a change's ledger.md (or, with ?href=, a task
// container's ledger.md) rendered in the detail modal.
func (s *Server) ledgerDetail(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	var body string
	var err error
	label := id
	if href := strings.TrimSpace(r.URL.Query().Get("href")); href != "" {
		body, err = s.st.ContainerLedgerFile(id, href)
		label = id + "/" + href
	} else {
		body, err = s.st.LedgerFile(id)
	}
	if err != nil {
		writeErr(w, err)
		return
	}
	if wantsHTML(r) || isHX(r) {
		s.rend.render(w, s.rend.partial, "ledgerDetail", ledgerView{ID: label, Body: dropLeadingH1(body, label), Doc: strings.TrimSpace(r.URL.Query().Get("href"))})
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"id": id, "body": body})
}

// dropLeadingH1 removes the task file's own H1 ("# ID: title") when it
// duplicates the modal header (presentation only; the file is untouched).
func dropLeadingH1(body, id string) string {
	b := strings.TrimLeft(body, "\n")
	if !strings.HasPrefix(b, "# ") {
		return body
	}
	i := strings.IndexByte(b, '\n')
	if i < 0 || !strings.Contains(b[:i], id) {
		return body
	}
	return b[i+1:]
}

// --- write endpoints ---

type moveRequest struct {
	Task   string `json:"task"`
	Status string `json:"status"`
	Index  int    `json:"index"`
}

func (s *Server) moveTask(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	var req moveRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "bad JSON body"})
		return
	}
	if req.Task == "" || !model.TaskStatus(req.Status).Valid() {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "task and a valid status are required"})
		return
	}
	if err := s.st.MoveTask(id, req.Task, model.TaskStatus(req.Status), req.Index); err != nil {
		writeErr(w, err)
		return
	}
	slog.Info("move", "change", id, "task", req.Task, "status", req.Status, "index", req.Index)
	writeJSON(w, http.StatusOK, map[string]string{"ok": "true"})
}

type createTaskRequest struct {
	Title  string `json:"title"`
	Parent string `json:"parent"` // optional task ID: create a subtask in that task's container
}

// isJSON reports whether the request body is JSON (vs. an htmx form post).
func isJSON(r *http.Request) bool {
	return strings.HasPrefix(r.Header.Get("Content-Type"), "application/json")
}

func (s *Server) createTask(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	var req createTaskRequest
	if isJSON(r) {
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			writeJSON(w, http.StatusBadRequest, map[string]string{"error": "bad JSON body"})
			return
		}
	} else {
		if err := r.ParseForm(); err != nil {
			writeJSON(w, http.StatusBadRequest, map[string]string{"error": "bad form body"})
			return
		}
		req.Title = r.FormValue("title")
		req.Parent = r.FormValue("parent")
	}
	row, err := s.st.CreateTask(id, strings.TrimSpace(req.Parent), req.Title)
	if err != nil {
		writeErr(w, err)
		return
	}
	slog.Info("create task", "change", id, "task", row.ID)
	writeJSON(w, http.StatusCreated, row)
}

type createChangeRequest struct {
	Title  string `json:"title"`
	Prefix string `json:"prefix"`
}

func (s *Server) createChange(w http.ResponseWriter, r *http.Request) {
	var req createChangeRequest
	if isJSON(r) {
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			writeJSON(w, http.StatusBadRequest, map[string]string{"error": "bad JSON body"})
			return
		}
	} else {
		if err := r.ParseForm(); err != nil {
			writeJSON(w, http.StatusBadRequest, map[string]string{"error": "bad form body"})
			return
		}
		req.Title = r.FormValue("title")
		req.Prefix = r.FormValue("prefix")
	}
	id, err := s.st.CreateChange(req.Title, req.Prefix, s.effectiveSettings().Git.DefaultBranch, time.Now().Format("2006-01-02"))
	if err != nil {
		writeErr(w, err)
		return
	}
	slog.Info("create change", "id", id)
	if isHX(r) {
		w.Header().Set("HX-Redirect", "/changes/"+id)
		w.WriteHeader(http.StatusOK)
		return
	}
	writeJSON(w, http.StatusCreated, map[string]string{"id": id})
}

// --- validation + events ---

func (s *Server) validate(w http.ResponseWriter, r *http.Request) {
	v := s.st.Validate()
	if v == nil {
		v = []store.Violation{}
	}
	var stale map[string]string
	if s.docsQ != nil {
		stale = s.docsQ.staleReasons()
	}
	df := docs.ValidateDocs(s.st.Dir, stale)
	if df == nil {
		df = []docs.Finding{}
	}
	payload := map[string]any{"violations": v, "docs": df}
	// Dirs missing their doc files: the bell's "Run missing docs" button
	// hides at zero; -1 means the lookup failed (walk trouble).
	if missing, err := docs.MissingDocDirs(s.st.Dir); err == nil {
		payload["docsSeedPending"] = len(missing)
	}
	writeJSON(w, http.StatusOK, payload)
}

func (s *Server) events(w http.ResponseWriter, r *http.Request) {
	flusher, ok := w.(http.Flusher)
	if !ok {
		http.Error(w, "streaming unsupported", http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Connection", "keep-alive")

	ch := s.st.Subscribe()
	defer s.st.Unsubscribe(ch)
	var docsCh <-chan struct{}
	if s.docsW != nil {
		docsCh = s.docsW.Events()
	}

	heartbeat := time.NewTicker(25 * time.Second)
	defer heartbeat.Stop()

	fmt.Fprint(w, ": connected\n\n")
	flusher.Flush()

	for {
		select {
		case <-r.Context().Done():
			return
		case <-s.st.Done():
			return
		case ev := <-ch:
			fmt.Fprintf(w, "event: %s\ndata: %s\n\n", ev.Kind, ev.Path)
			flusher.Flush()
		case <-docsCh:
			fmt.Fprint(w, "event: docs\ndata: docs changed\n\n")
			flusher.Flush()
		case <-heartbeat.C:
			fmt.Fprint(w, ": heartbeat\n\n")
			flusher.Flush()
		}
	}
}
