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
	"sort"
	"strings"
	"sync"
	"time"

	"lessmess/internal/model"
	"lessmess/internal/opencode"
	"lessmess/internal/store"
)

// SessionEntry links one opencode session to a change. Task and Parent are
// optional subagent annotations: Task names the change task (e.g. "PSB-01")
// the session was delegated to, Parent names the session that spawned it.
// SpawnedFrom names a source change when the session was created by a
// handoff (POST /changes/{source}/spawn-change).
type SessionEntry struct {
	Session     string   `json:"session"`
	Title       string   `json:"title"`
	Created     string   `json:"created"`
	Task        string   `json:"task,omitempty"`
	Parent      string   `json:"parent,omitempty"`
	SpawnedFrom string   `json:"spawnedFrom,omitempty"`
	Modules     []string `json:"modules,omitempty"` // instruction modules injected at spawn (audit)
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
// anywhere (a concurrent reconcile or bind may have won the race). It reports
// how many entries were actually added and saves at most once.
func (m *mapping) addAll(change string, entries []SessionEntry) (int, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	existing := make(map[string]bool)
	for _, mapped := range m.data {
		for _, e := range mapped {
			existing[e.Session] = true
		}
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
			original := append([]SessionEntry(nil), entries...)
			m.data[change] = append(entries[:i], entries[i+1:]...)
			if len(m.data[change]) == 0 {
				delete(m.data, change)
			}
			if err := m.save(); err != nil {
				m.data[change] = original
				return false, err
			}
			return true, nil
		}
	}
	return false, nil
}

// updateTitle keeps the persisted fallback title aligned with OpenCode. The
// update is all-or-nothing in memory and on disk so a failed save can be
// retried without losing the prior mapping state.
func (m *mapping) updateTitle(session, title string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	for change, entries := range m.data {
		for i := range entries {
			if entries[i].Session != session {
				continue
			}
			old := entries[i].Title
			m.data[change][i].Title = title
			if err := m.save(); err != nil {
				m.data[change][i].Title = old
				return err
			}
			return nil
		}
	}
	return nil
}

// removeEverywhere removes a set of deleted OpenCode sessions from every
// mapping bucket in one atomic save.
func (m *mapping) removeEverywhere(sessions map[string]bool) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	original := make(map[string][]SessionEntry, len(m.data))
	for change, entries := range m.data {
		original[change] = append([]SessionEntry(nil), entries...)
	}
	changed := false
	for change, entries := range m.data {
		kept := entries[:0]
		for _, entry := range entries {
			if sessions[entry.Session] {
				changed = true
				continue
			}
			kept = append(kept, entry)
		}
		if len(kept) == 0 {
			delete(m.data, change)
		} else {
			m.data[change] = kept
		}
	}
	if !changed {
		return nil
	}
	if err := m.save(); err != nil {
		m.data = original
		return err
	}
	return nil
}

// mappingClosure returns the root and all mapped descendants linked through
// persisted Parent annotations. It is used only to reconcile a retry after
// OpenCode has already deleted the authoritative tree.
func (m *mapping) mappingClosure(root string) map[string]bool {
	m.mu.Lock()
	defer m.mu.Unlock()
	ids := map[string]bool{root: true}
	for changed := true; changed; {
		changed = false
		for _, entries := range m.data {
			for _, entry := range entries {
				if ids[entry.Parent] && !ids[entry.Session] {
					ids[entry.Session] = true
					changed = true
				}
			}
		}
	}
	return ids
}

type mappedSessionOwner struct {
	Session string `json:"session"`
	Change  string `json:"change,omitempty"`
	Task    string `json:"task,omitempty"`
	Title   string `json:"title"`
}

func (m *mapping) owners(sessions map[string]bool) []mappedSessionOwner {
	m.mu.Lock()
	defer m.mu.Unlock()
	var owners []mappedSessionOwner
	for change, entries := range m.data {
		for _, entry := range entries {
			if !sessions[entry.Session] {
				continue
			}
			owner := mappedSessionOwner{Session: entry.Session, Task: entry.Task, Title: entry.Title}
			if change != unassignedKey {
				owner.Change = change
			}
			owners = append(owners, owner)
		}
	}
	sort.Slice(owners, func(i, j int) bool {
		if owners[i].Session == owners[j].Session {
			return owners[i].Change < owners[j].Change
		}
		return owners[i].Session < owners[j].Session
	})
	return owners
}

func (m *mapping) entry(session string) (string, SessionEntry, bool) {
	m.mu.Lock()
	defer m.mu.Unlock()
	for change, entries := range m.data {
		for _, entry := range entries {
			if entry.Session == session {
				return change, entry, true
			}
		}
	}
	return "", SessionEntry{}, false
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
	Session     string `json:"session"`
	Title       string `json:"title"`
	Created     string `json:"created"`
	Task        string `json:"task,omitempty"`
	Parent      string `json:"parent,omitempty"`
	SpawnedFrom string `json:"spawnedFrom,omitempty"`
	Live        bool   `json:"live"` // title enriched from the service
}

// enrich adds live titles from the service to mapping entries.
func (s *Server) enrich(r *http.Request, entries []SessionEntry) []sessionResponse {
	out := make([]sessionResponse, 0, len(entries))
	for _, e := range entries {
		resp := sessionResponse{Session: e.Session, Title: e.Title, Created: e.Created, Task: e.Task, Parent: e.Parent, SpawnedFrom: e.SpawnedFrom}
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

type sessionDescendant struct {
	Session opencode.Session
	Depth   int
}

// sessionDescendants walks every direct-child page and then each child. The
// service's parentID is checked again because it is the only authoritative
// hierarchy signal. Repeated cursors, duplicate sessions, and hostile cycles
// terminate locally instead of poisoning the rest of the traversal.
func (s *Server) sessionDescendants(ctx context.Context, roots []string) []sessionDescendant {
	type pending struct {
		id    string
		depth int
	}
	queue := make([]pending, 0, len(roots))
	seen := make(map[string]bool, len(roots))
	for _, id := range roots {
		if id != "" && !seen[id] {
			seen[id] = true
			queue = append(queue, pending{id: id})
		}
	}
	var out []sessionDescendant
	for len(queue) > 0 && ctx.Err() == nil {
		parent := queue[0]
		queue = queue[1:]
		cursor := ""
		cursors := map[string]bool{}
		for {
			page, err := s.oc.ListChildrenPage(ctx, parent.id, 50, cursor)
			if err != nil {
				slog.Debug("session child traversal skipped", "parent", parent.id, "err", err)
				break
			}
			for _, child := range page.Sessions {
				if child.ID == "" || child.ParentID != parent.id || seen[child.ID] {
					continue
				}
				seen[child.ID] = true
				out = append(out, sessionDescendant{Session: child, Depth: parent.depth + 1})
				queue = append(queue, pending{id: child.ID, depth: parent.depth + 1})
			}
			next := page.Cursor.Next
			if next == "" || next == cursor || cursors[next] {
				break
			}
			cursors[next] = true
			cursor = next
		}
	}
	return out
}

// reconcileTaskSessions maps all unmapped descendants of this change's live
// sessions. Hierarchy always comes from parentID; title parsing is retained
// only as the documented fallback for assigning a newly discovered task.
// Sessions already owned by another change are never moved, and their branch
// is not used to claim otherwise-unmapped descendants.
func (s *Server) reconcileTaskSessions(r *http.Request, c *store.Change) {
	if s.oc == nil || s.mapErr != nil {
		return
	}
	bound := s.sessions.list(c.ID)
	if len(bound) == 0 {
		return
	}
	roots := make([]string, 0, len(bound))
	owned := make(map[string]bool, len(bound))
	for _, e := range bound {
		roots = append(roots, e.Session)
		owned[e.Session] = true
	}
	ctx, cancel := context.WithTimeout(r.Context(), 10*time.Second)
	defer cancel()
	descendants := s.sessionDescendants(ctx, roots)
	knownTasks := map[string]bool{}
	c.WalkTasks(func(n *store.TaskNode) bool {
		if n.ID != "" {
			knownTasks[n.ID] = true
		}
		return true
	})
	var fresh []SessionEntry
	for _, descendant := range descendants {
		sess := descendant.Session
		if !owned[sess.ParentID] {
			continue
		}
		if change, _, mapped := s.sessions.entry(sess.ID); mapped {
			if change == c.ID {
				owned[sess.ID] = true
			}
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
		owned[sess.ID] = true
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
	if taskID := strings.TrimSpace(req.Task); taskID != "" {
		taskNode = c.Node(taskID)
		if taskNode == nil {
			writeJSON(w, http.StatusUnprocessableEntity, map[string]string{"error": "unknown task " + taskID + " in change " + id})
			return
		}
		if title == "" {
			title = taskSessionTitle(taskNode)
		}
	}
	if title == "" {
		title = changeTitle(s.st, id)
	}

	ctx, cancel := context.WithTimeout(r.Context(), 15*time.Second)
	defer cancel()
	sess, err := s.spawnSessionIn(ctx, s.changeSessionDir(id), title)
	if err != nil {
		writeJSON(w, http.StatusBadGateway, map[string]string{"error": "create opencode session: " + err.Error()})
		return
	}
	// The prime is built after the spawn: primes carry the session's own
	// ID so the agent can identify itself to session-taking endpoints.
	// The engine selects the modules (worktree only when backed) and
	// injects the authoritative state snapshot.
	var prime string
	var modules []string
	if taskNode != nil {
		var text string
		text, modules = s.taskPrime(id, taskNode, sess.ID)
		prime = s.promptWith(text, "change")
	} else {
		var text string
		text, modules = s.changePrime(id, sess.ID)
		prime = s.promptWith(text, "change")
	}
	if err := s.oc.Prompt(ctx, sess.ID, prime); err != nil {
		// Don't leak an unbound session: the binding is the whole point.
		_ = s.oc.DeleteSession(context.Background(), sess.ID)
		writeJSON(w, http.StatusBadGateway, map[string]string{"error": "prime change session: " + err.Error()})
		return
	}
	logPrime(sess.ID, "task", modules)
	entry := SessionEntry{Session: sess.ID, Title: title, Created: time.Now().Format(time.RFC3339), Modules: modules}
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

// changeTitle looks up a change's title from the workflow index.
func changeTitle(st *store.Store, id string) string {
	if e := st.Entry(id); e != nil {
		return id + " — " + e.Title
	}
	return id
}
