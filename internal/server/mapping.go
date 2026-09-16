package server

import (
	"context"
	"encoding/json"
	"errors"
	"log/slog"
	"net/http"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"sync"
	"time"

	"lessmess/internal/model"
	"lessmess/internal/store"
)

// SessionEntry links one opencode session to a change. Task and Parent are
// optional subagent annotations: Task names the change task (e.g. "PSB-01")
// the session was delegated to, Parent names the session that spawned it.
type SessionEntry struct {
	Session string `json:"session"`
	Title   string `json:"title"`
	Created string `json:"created"`
	Task    string `json:"task,omitempty"`
	Parent  string `json:"parent,omitempty"`
}

// mapping is the .lessmess/sessions.json file (tooling state, gitignored).
type mapping struct {
	path string
	mu   sync.Mutex
	data map[string][]SessionEntry
}

func loadMapping(path string) (*mapping, error) {
	m := &mapping{path: path, data: map[string][]SessionEntry{}}
	b, err := os.ReadFile(path)
	if errors.Is(err, os.ErrNotExist) {
		return m, nil
	}
	if err != nil {
		return m, err
	}
	if err := json.Unmarshal(b, &m.data); err != nil {
		return m, err
	}
	if m.data == nil {
		m.data = map[string][]SessionEntry{}
	}
	return m, nil
}

func (m *mapping) save() error {
	if err := os.MkdirAll(filepath.Dir(m.path), 0o755); err != nil {
		return err
	}
	b, err := json.MarshalIndent(m.data, "", "  ")
	if err != nil {
		return err
	}
	return model.WriteFileAtomic(m.path, append(b, '\n'), 0o644)
}

func (m *mapping) list(change string) []SessionEntry {
	m.mu.Lock()
	defer m.mu.Unlock()
	out := make([]SessionEntry, len(m.data[change]))
	copy(out, m.data[change])
	return out
}

// listByTask returns the change's entries for one task; an empty task matches
// taskless entries (sessions bound to the change without a task annotation).
func (m *mapping) listByTask(change, task string) []SessionEntry {
	m.mu.Lock()
	defer m.mu.Unlock()
	var out []SessionEntry
	for _, e := range m.data[change] {
		if e.Task == task {
			out = append(out, e)
		}
	}
	return out
}

func (m *mapping) add(change string, e SessionEntry) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.data[change] = append(m.data[change], e)
	return m.save()
}

// addAll appends entries under one change, skipping sessions already mapped
// there (a concurrent reconcile or bind may have won the race). It reports
// how many entries were actually added and saves at most once.
func (m *mapping) addAll(change string, entries []SessionEntry) (int, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	existing := make(map[string]bool, len(m.data[change]))
	for _, e := range m.data[change] {
		existing[e.Session] = true
	}
	var add []SessionEntry
	for _, e := range entries {
		if !existing[e.Session] {
			add = append(add, e)
			existing[e.Session] = true
		}
	}
	if len(add) == 0 {
		return 0, nil
	}
	m.data[change] = append(m.data[change], add...)
	return len(add), m.save()
}

// knows reports whether a session id appears anywhere in the mapping,
// including the unassigned bucket.
func (m *mapping) knows(session string) bool {
	m.mu.Lock()
	defer m.mu.Unlock()
	for _, entries := range m.data {
		for _, e := range entries {
			if e.Session == session {
				return true
			}
		}
	}
	return false
}

func (m *mapping) remove(change, session string) (bool, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	entries := m.data[change]
	for i, e := range entries {
		if e.Session == session {
			m.data[change] = append(entries[:i], entries[i+1:]...)
			if len(m.data[change]) == 0 {
				delete(m.data, change)
			}
			return true, m.save()
		}
	}
	return false, nil
}

// unassignedKey is the reserved mapping key for pre-scaffold discussion sessions.
const unassignedKey = "_unassigned"

// addUnassigned links a discussion session that has no change yet.
func (m *mapping) addUnassigned(e SessionEntry) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.data[unassignedKey] = append(m.data[unassignedKey], e)
	return m.save()
}

// listUnassigned returns the pre-scaffold discussion sessions.
func (m *mapping) listUnassigned() []SessionEntry { return m.list(unassignedKey) }

// moveToChange relocates a session from the unassigned bucket to a change.
// Reports false when the session is not in the bucket (idempotent for retries).
func (m *mapping) moveToChange(session, changeID string) (bool, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	entries := m.data[unassignedKey]
	for i, e := range entries {
		if e.Session == session {
			m.data[unassignedKey] = append(entries[:i], entries[i+1:]...)
			if len(m.data[unassignedKey]) == 0 {
				delete(m.data, unassignedKey)
			}
			m.data[changeID] = append(m.data[changeID], e)
			return true, m.save()
		}
	}
	return false, nil
}

// changeOf reports which change a session is mapped to, if any. The
// unassigned bucket is not a change.
func (m *mapping) changeOf(session string) (string, bool) {
	m.mu.Lock()
	defer m.mu.Unlock()
	for change, entries := range m.data {
		if change == unassignedKey {
			continue
		}
		for _, e := range entries {
			if e.Session == session {
				return change, true
			}
		}
	}
	return "", false
}

// --- endpoints ---

// sessionResponse is the enriched mapping entry returned to the browser.
type sessionResponse struct {
	Session string `json:"session"`
	Title   string `json:"title"`
	Created string `json:"created"`
	Task    string `json:"task,omitempty"`
	Parent  string `json:"parent,omitempty"`
	Live    bool   `json:"live"` // title enriched from the service
}

// enrich adds live titles from the service to mapping entries.
func (s *Server) enrich(r *http.Request, entries []SessionEntry) []sessionResponse {
	out := make([]sessionResponse, 0, len(entries))
	for _, e := range entries {
		resp := sessionResponse{Session: e.Session, Title: e.Title, Created: e.Created, Task: e.Task, Parent: e.Parent}
		if s.oc != nil {
			ctx, cancel := context.WithTimeout(r.Context(), 3*time.Second)
			if live, err := s.oc.GetSession(ctx, e.Session); err == nil {
				resp.Title = live.Title
				resp.Live = true
			}
			cancel()
		}
		out = append(out, resp)
	}
	return out
}

// taskTitleRe parses the task-ID prefix the change prompt teaches: a spawned
// subagent's description becomes the child session's title verbatim, so
// "FIX-00: do the work" is how a child carries its task association.
// Dotted segments extend it to decomposed tasks ("FIX-00.01: …").
var taskTitleRe = regexp.MustCompile(`^([A-Z][A-Z0-9]{1,3}-\d+(?:\.\d+)*):`)

// reconcileTaskSessions maps subagent children that were spawned by one of
// the change's sessions but never mapped: one ListSessions call finds
// sessions whose parentID points at a bound session, and the task comes
// from the title prefix above (unknown prefixes map taskless). Purely
// additive and fail-open: a service error leaves the stored list untouched.
func (s *Server) reconcileTaskSessions(r *http.Request, c *store.Change) {
	if s.oc == nil || s.mapErr != nil {
		return
	}
	bound := s.sessions.list(c.ID)
	if len(bound) == 0 {
		return
	}
	parents := make(map[string]bool, len(bound))
	for _, e := range bound {
		parents[e.Session] = true
	}
	ctx, cancel := context.WithTimeout(r.Context(), 5*time.Second)
	defer cancel()
	all, err := s.oc.ListSessions(ctx)
	if err != nil {
		slog.Debug("task session reconcile skipped", "change", c.ID, "err", err)
		return
	}
	knownTasks := map[string]bool{}
	c.WalkTasks(func(n *store.TaskNode) bool {
		if n.ID != "" {
			knownTasks[n.ID] = true
		}
		return true
	})
	var fresh []SessionEntry
	for _, sess := range all {
		if sess.ParentID == "" || !parents[sess.ParentID] || s.sessions.knows(sess.ID) {
			continue
		}
		task := ""
		if m := taskTitleRe.FindStringSubmatch(sess.Title); m != nil && knownTasks[m[1]] {
			task = m[1]
		}
		fresh = append(fresh, SessionEntry{
			Session: sess.ID,
			Title:   sess.Title,
			Created: time.UnixMilli(sess.Time.Created).UTC().Format(time.RFC3339),
			Task:    task,
			Parent:  sess.ParentID,
		})
	}
	added, err := s.sessions.addAll(c.ID, fresh)
	if err != nil {
		slog.Warn("task session reconcile persist failed", "change", c.ID, "err", err)
		return
	}
	if added > 0 {
		slog.Info("reconciled task sessions", "change", c.ID, "count", added)
	}
}

func (s *Server) listChangeSessions(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	c, err := s.st.Change(id)
	if err != nil {
		writeErr(w, err)
		return
	}
	if s.mapErr != nil {
		writeJSON(w, http.StatusServiceUnavailable, map[string]string{"error": "session mapping unreadable: " + s.mapErr.Error()})
		return
	}
	s.reconcileTaskSessions(r, c)
	writeJSON(w, http.StatusOK, map[string]any{"sessions": s.enrich(r, s.sessions.list(id))})
}

// listDiscussions handles GET /api/discussions: unassigned (pre-scaffold) sessions.
func (s *Server) listDiscussions(w http.ResponseWriter, r *http.Request) {
	if s.mapErr != nil {
		writeJSON(w, http.StatusServiceUnavailable, map[string]string{"error": "session mapping unreadable: " + s.mapErr.Error()})
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"sessions": s.enrich(r, s.sessions.listUnassigned())})
}

// unlinkDiscussion handles DELETE /api/discussions/{sessionID}: remove a
// session from the unassigned bucket (the opencode session itself is kept).
func (s *Server) unlinkDiscussion(w http.ResponseWriter, r *http.Request) {
	sessionID := r.PathValue("sessionID")
	if s.mapErr != nil {
		writeJSON(w, http.StatusServiceUnavailable, map[string]string{"error": "session mapping unreadable: " + s.mapErr.Error()})
		return
	}
	found, err := s.sessions.remove(unassignedKey, sessionID)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}
	if !found {
		writeErr(w, store.ErrNotFound)
		return
	}
	slog.Info("discussion unlinked", "session", sessionID)
	writeJSON(w, http.StatusOK, map[string]string{"ok": "true"})
}

// sessionChange handles GET /api/sessions/{sessionID}/change: the change
// the session is bound to (empty when unassigned). Drives the terminal
// task panel on non-board pages.
func (s *Server) sessionChange(w http.ResponseWriter, r *http.Request) {
	if s.mapErr != nil {
		writeJSON(w, http.StatusServiceUnavailable, map[string]string{"error": "session mapping unreadable: " + s.mapErr.Error()})
		return
	}
	change, _ := s.sessions.changeOf(r.PathValue("sessionID"))
	writeJSON(w, http.StatusOK, map[string]string{"change": change})
}

type createSessionRequest struct {
	Title string `json:"title"` // optional; defaults to the change title
	Task  string `json:"task"`  // optional; bind + prime for this task (change sessions only)
}

func (s *Server) createChangeSession(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	c, err := s.st.Change(id)
	if err != nil {
		writeErr(w, err)
		return
	}
	if s.oc == nil {
		writeJSON(w, http.StatusServiceUnavailable, map[string]string{"error": "opencode service unavailable"})
		return
	}
	if s.mapErr != nil {
		writeJSON(w, http.StatusServiceUnavailable, map[string]string{"error": "session mapping unreadable: " + s.mapErr.Error()})
		return
	}
	var req createSessionRequest
	if isJSON(r) {
		_ = json.NewDecoder(r.Body).Decode(&req)
	} else {
		_ = r.ParseForm()
		req.Title = r.FormValue("title")
		req.Task = r.FormValue("task")
	}
	title := strings.TrimSpace(req.Title)
	var taskNode *store.TaskNode
	var prime string
	if taskID := strings.TrimSpace(req.Task); taskID != "" {
		taskNode = c.Node(taskID)
		if taskNode == nil {
			writeJSON(w, http.StatusUnprocessableEntity, map[string]string{"error": "unknown task " + taskID + " in change " + id})
			return
		}
		prime = s.promptWith(taskPrompt(id, taskNode), "change")
		if title == "" {
			title = taskSessionTitle(taskNode)
		}
	} else {
		prime = s.promptWith(changePrompt(id), "change")
	}
	if title == "" {
		title = changeTitle(s.st, id)
	}

	ctx, cancel := context.WithTimeout(r.Context(), 15*time.Second)
	defer cancel()
	sess, err := s.spawnSession(ctx, title)
	if err != nil {
		writeJSON(w, http.StatusBadGateway, map[string]string{"error": "create opencode session: " + err.Error()})
		return
	}
	if err := s.oc.Prompt(ctx, sess.ID, prime); err != nil {
		// Don't leak an unbound session: the binding is the whole point.
		_ = s.oc.DeleteSession(context.Background(), sess.ID)
		writeJSON(w, http.StatusBadGateway, map[string]string{"error": "prime change session: " + err.Error()})
		return
	}
	entry := SessionEntry{Session: sess.ID, Title: title, Created: time.Now().Format(time.RFC3339)}
	if taskNode != nil {
		entry.Task = taskNode.ID
		// A manually created task session satisfies the auto-spawn marker.
		_ = s.autos.mark(id + "\x00" + taskNode.ID)
	}
	if err := s.sessions.add(id, entry); err != nil {
		slog.Error("mapping add", "err", err)
		_ = s.oc.DeleteSession(context.Background(), sess.ID) // don't leak an unmapped session
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "persist mapping: " + err.Error()})
		return
	}
	slog.Info("session created", "change", id, "session", sess.ID, "task", entry.Task)
	writeJSON(w, http.StatusCreated, sessionResponse{Session: entry.Session, Title: entry.Title, Created: entry.Created, Task: entry.Task, Live: true})
}

func (s *Server) unlinkChangeSession(w http.ResponseWriter, r *http.Request) {
	id, sessionID := r.PathValue("id"), r.PathValue("sessionID")
	if _, err := s.st.Change(id); err != nil {
		writeErr(w, err)
		return
	}
	if s.mapErr != nil {
		writeJSON(w, http.StatusServiceUnavailable, map[string]string{"error": "session mapping unreadable: " + s.mapErr.Error()})
		return
	}
	found, err := s.sessions.remove(id, sessionID)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}
	if !found {
		writeErr(w, store.ErrNotFound)
		return
	}
	slog.Info("session unlinked", "change", id, "session", sessionID)
	writeJSON(w, http.StatusOK, map[string]string{"ok": "true"})
}

// changeTitle looks up a change's title from the root ledger.
func changeTitle(st *store.Store, id string) string {
	if root, err := st.Root(); err == nil && root != nil {
		for _, r := range root.Rows {
			if r.Change == id {
				return id + " — " + r.Title
			}
		}
	}
	return id
}
