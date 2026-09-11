// Package server exposes the store over HTTP: HTML pages, JSON write
// endpoints, and an SSE stream for live updates.
package server

import (
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"path/filepath"
	"strings"
	"time"

	"tasktracker/internal/model"
	"tasktracker/internal/opencode"
	"tasktracker/internal/store"
	"tasktracker/internal/terminal"
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

	oc       *opencode.Client // nil disables the opencode integration
	sessions *mapping
	mapErr   error
}

// New builds the route table.
func New(st *store.Store) *Server {
	s := &Server{st: st, rend: newRenderer(), term: terminal.NewManager()}
	s.SpawnCommand = func(sessionID string) (string, []string) {
		return "opencode2", []string{"--session", sessionID}
	}
	m, err := loadMapping(filepath.Join(st.Dir, ".tasktracker", "sessions.json"))
	s.sessions, s.mapErr = m, err

	mux := http.NewServeMux()
	mux.HandleFunc("GET /{$}", s.index)
	mux.HandleFunc("GET /changes/{id}", s.board)
	mux.HandleFunc("GET /changes/{id}/plan", s.planDetail)
	mux.HandleFunc("GET /changes/{id}/tasks/{file}", s.taskDetail)
	mux.HandleFunc("POST /changes/{id}/tasks", s.createTask)
	mux.HandleFunc("POST /changes/{id}/move", s.moveTask)
	mux.HandleFunc("POST /changes/{id}/close", s.closeChange)
	mux.HandleFunc("POST /changes/{id}/reopen", s.reopenChange)
	mux.HandleFunc("POST /changes/{id}/commit", s.commitChange)
	mux.HandleFunc("POST /changes/{$}", s.createChange)
	mux.HandleFunc("POST /changes/session", s.createChangeWithSession)
	mux.HandleFunc("GET /events", s.events)
	mux.HandleFunc("GET /api/validate", s.validate)
	mux.HandleFunc("GET /terminal/ws", s.terminalWS)
	mux.HandleFunc("GET /changes/{id}/sessions", s.listChangeSessions)
	mux.HandleFunc("POST /changes/{id}/sessions", s.createChangeSession)
	mux.HandleFunc("DELETE /changes/{id}/sessions/{sessionID}", s.unlinkChangeSession)
	if sh, err := staticHandler(); err == nil {
		mux.Handle("GET /static/", sh)
	}
	s.mux = mux
	return s
}

// SetOpencode attaches the opencode client (nil disables the integration).
func (s *Server) SetOpencode(c *opencode.Client) { s.oc = c }

// Close releases resources (terminal PTYs).
func (s *Server) Close() { s.term.CloseAll() }

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
	case errors.Is(err, store.ErrInvalid):
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
	Tasks   int    `json:"tasks"`
}

func (s *Server) index(w http.ResponseWriter, r *http.Request) {
	root, err := s.st.Root()
	if err != nil {
		writeErr(w, fmt.Errorf("%w: %v", store.ErrInvalid, err))
		return
	}
	rows := make([]changeSummary, 0, len(root.Rows))
	byID := map[string]*store.Change{}
	for _, c := range s.st.Changes() {
		byID[c.ID] = c
	}
	for _, rr := range root.Rows {
		sum := changeSummary{ID: rr.Change, Title: rr.Title, Prefix: rr.Prefix, Status: string(rr.Status), Updated: rr.Updated}
		if c, ok := byID[rr.Change]; ok && c.Ledger != nil {
			sum.Tasks = len(c.Ledger.Rows)
		}
		rows = append(rows, sum)
	}
	if wantsHTML(r) {
		s.rend.render(w, s.rend.index, "layout", pageData{Title: "changes", Page: "index", Data: indexView{Changes: rows}})
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"changes": rows})
}

type boardResponse struct {
	ID      string          `json:"id"`
	Overall string          `json:"overall"`
	Columns []columnView    `json:"columns"`
	Tasks   []model.TaskRow `json:"tasks"`
	Error   string          `json:"error,omitempty"`
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
	if wantsHTML(r) {
		view := newBoardView(c)
		if isHX(r) {
			s.rend.render(w, s.rend.partial, "boardFragment", view)
		} else {
			s.rend.render(w, s.rend.board, "layout", pageData{Title: id, Page: "board", Data: view})
		}
		return
	}
	resp := boardResponse{ID: id}
	if c.Err != nil || c.Ledger == nil {
		resp.Error = fmt.Sprintf("ledger unreadable: %v", c.Err)
		writeJSON(w, http.StatusUnprocessableEntity, resp)
		return
	}
	resp.Overall = string(c.Ledger.Overall)
	resp.Tasks = c.Ledger.Rows
	for _, st := range model.TaskStatusOrder {
		col := columnView{Status: string(st)}
		for _, t := range c.Ledger.Rows {
			if t.Status == st {
				col.Count++
			}
		}
		resp.Columns = append(resp.Columns, col)
	}
	writeJSON(w, http.StatusOK, resp)
}

func (s *Server) taskDetail(w http.ResponseWriter, r *http.Request) {
	id, file := r.PathValue("id"), r.PathValue("file")
	tf, err := s.st.TaskFile(id, "tasks/"+file)
	if err != nil {
		writeErr(w, err)
		return
	}
	if wantsHTML(r) || isHX(r) {
		view := taskView{Task: tf, Body: dropLeadingH1(tf.Body, tf.ID)}
		if c, err := s.st.Change(id); err == nil && c.Ledger != nil {
			if row := c.Ledger.Row(tf.ID); row != nil {
				view.Status = string(row.Status)
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
	Title string `json:"title"`
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
	}
	row, err := s.st.CreateTask(id, req.Title)
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
	id, err := s.st.CreateChange(req.Title, req.Prefix, time.Now().Format("2006-01-02"))
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
	writeJSON(w, http.StatusOK, map[string]any{"violations": v})
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
		case <-heartbeat.C:
			fmt.Fprint(w, ": heartbeat\n\n")
			flusher.Flush()
		}
	}
}
