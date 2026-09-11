package server

import (
	"context"
	"encoding/json"
	"errors"
	"log/slog"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"tasktracker/internal/model"
	"tasktracker/internal/store"
)

// SessionEntry links one opencode session to a change.
type SessionEntry struct {
	Session string `json:"session"`
	Title   string `json:"title"`
	Created string `json:"created"`
}

// mapping is the .tasktracker/sessions.json file (tooling state, gitignored).
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

func (m *mapping) add(change string, e SessionEntry) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.data[change] = append(m.data[change], e)
	return m.save()
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

// --- endpoints ---

// sessionResponse is the enriched mapping entry returned to the browser.
type sessionResponse struct {
	Session string `json:"session"`
	Title   string `json:"title"`
	Created string `json:"created"`
	Live    bool   `json:"live"` // title enriched from the service
}

func (s *Server) listChangeSessions(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	if _, err := s.st.Change(id); err != nil {
		writeErr(w, err)
		return
	}
	if s.mapErr != nil {
		writeJSON(w, http.StatusServiceUnavailable, map[string]string{"error": "session mapping unreadable: " + s.mapErr.Error()})
		return
	}
	entries := s.sessions.list(id)
	out := make([]sessionResponse, 0, len(entries))
	for _, e := range entries {
		resp := sessionResponse{Session: e.Session, Title: e.Title, Created: e.Created}
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
	writeJSON(w, http.StatusOK, map[string]any{"sessions": out})
}

type createSessionRequest struct {
	Title string `json:"title"` // optional; defaults to the change title
}

func (s *Server) createChangeSession(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	if _, err := s.st.Change(id); err != nil {
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
	}
	title := strings.TrimSpace(req.Title)
	if title == "" {
		title = changeTitle(s.st, id)
	}

	ctx, cancel := context.WithTimeout(r.Context(), 15*time.Second)
	defer cancel()
	sess, err := s.oc.CreateSession(ctx, title, s.st.Dir)
	if err != nil {
		writeJSON(w, http.StatusBadGateway, map[string]string{"error": "create opencode session: " + err.Error()})
		return
	}
	entry := SessionEntry{Session: sess.ID, Title: title, Created: time.Now().Format(time.RFC3339)}
	if err := s.sessions.add(id, entry); err != nil {
		slog.Error("mapping add", "err", err)
		_ = s.oc.DeleteSession(context.Background(), sess.ID) // don't leak an unmapped session
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "persist mapping: " + err.Error()})
		return
	}
	slog.Info("session created", "change", id, "session", sess.ID)
	writeJSON(w, http.StatusCreated, sessionResponse{Session: entry.Session, Title: entry.Title, Created: entry.Created, Live: true})
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
