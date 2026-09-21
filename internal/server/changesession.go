package server

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"time"

	"lessmess/internal/store"
)

var prefixRe = regexp.MustCompile(`^[A-Z0-9]{2,4}$`)

// discussionPrompt builds the prime for a pre-scaffold discussion
// session: pure module composition (no state to inject). Step 0 covers
// the empty state; the scaffold trigger carries the API base and the
// session's own ID.
func discussionPrompt(apiBase, sessionID string) string {
	text, _ := renderPrime("discussion", primeContext{APIBase: apiBase, SessionID: sessionID})
	return text
}

// changePrime builds the prime for a session bound to a change: the
// change-session modules (the worktree module only when worktree-backed),
// then the injected authoritative state snapshot. Returns the text and
// the selected module IDs for audit logging.
func (s *Server) changePrime(changeID, sessionID string) (string, []string) {
	pc := primeContext{APIBase: s.apiBase(), ChangeID: changeID, SessionID: sessionID}
	if e, ok := s.worktreeEntry(changeID); ok {
		pc.Worktree, pc.WorktreeBranch = e.Path, e.Branch
	}
	if view, err := s.st.LedgerFile(changeID); err == nil {
		pc.Snapshot = view
	}
	return renderPrime("change", pc)
}

// taskPrime builds the prime for a session bound to one task (top-level
// or nested): the task-session modules plus the state snapshot.
func (s *Server) taskPrime(changeID string, n *store.TaskNode, sessionID string) (string, []string) {
	pc := primeContext{
		APIBase: s.apiBase(), ChangeID: changeID, SessionID: sessionID,
		TaskID: n.ID, TaskHref: n.Href,
	}
	if e, ok := s.worktreeEntry(changeID); ok {
		pc.Worktree, pc.WorktreeBranch = e.Path, e.Branch
	}
	if view, err := s.st.LedgerFile(changeID); err == nil {
		pc.Snapshot = view
	}
	return renderPrime("task", pc)
}

// apiBase returns the lessmess base URL used in agent-facing prompts.
func (s *Server) apiBase() string {
	if s.PublicBase != "" {
		return "http://" + s.PublicBase
	}
	return "http://127.0.0.1:8080"
}

// createDiscussionSession handles POST /changes/session: create a
// discussion-only opencode session (no repository writes) and map it to the
// unassigned bucket.
func (s *Server) createDiscussionSession(w http.ResponseWriter, r *http.Request) {
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
		title = "New change discussion"
	}

	ctx, cancel := context.WithTimeout(r.Context(), 15*time.Second)
	defer cancel()
	sess, err := s.spawnSession(ctx, title)
	if err != nil {
		writeJSON(w, http.StatusBadGateway, map[string]string{"error": "create opencode session: " + err.Error()})
		return
	}
	if err := s.oc.Prompt(ctx, sess.ID, s.promptWith(discussionPrompt(s.apiBase(), sess.ID), "discussion")); err != nil {
		_ = s.oc.DeleteSession(context.Background(), sess.ID)
		writeJSON(w, http.StatusBadGateway, map[string]string{"error": "prime discussion session: " + err.Error()})
		return
	}
	entry := SessionEntry{Session: sess.ID, Title: title, Created: time.Now().Format(time.RFC3339)}
	if err := s.sessions.addUnassigned(entry); err != nil {
		slog.Error("mapping add", "err", err)
		_ = s.oc.DeleteSession(context.Background(), sess.ID)
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "persist mapping: " + err.Error()})
		return
	}
	slog.Info("discussion session created", "session", sess.ID)
	writeJSON(w, http.StatusCreated, sessionResponse{Session: entry.Session, Title: entry.Title, Created: entry.Created, Live: true})
}

// taskSessionRequest is the body of POST /changes/{id}/task-sessions. Task
// is a task ID from the change's ledger, Sub the subagent session ID to
// bind, and Session an optional caller that must be bound to the change
// when supplied (agent calls are rare: the parent model cannot know the
// sub's session ID — PSB-00).
type taskSessionRequest struct {
	Task    string `json:"task"`
	Sub     string `json:"sub"`
	Session string `json:"session,omitempty"`
}

// bindTaskSession handles POST /changes/{id}/task-sessions: record that a
// subagent session belongs to one of the change's tasks. The sub's live
// parentID and title are captured so the mapping is self-describing.
func (s *Server) bindTaskSession(w http.ResponseWriter, r *http.Request) {
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
	var req taskSessionRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "bad JSON body"})
		return
	}
	req.Task = strings.TrimSpace(req.Task)
	req.Sub = strings.TrimSpace(req.Sub)
	if req.Task == "" || !strings.HasPrefix(req.Sub, "ses_") {
		writeJSON(w, http.StatusUnprocessableEntity, map[string]string{"error": "task and a ses_ sub session id are required"})
		return
	}
	// The task must exist anywhere in the change's task tree.
	if c.Node(req.Task) == nil {
		writeJSON(w, http.StatusUnprocessableEntity, map[string]string{"error": "unknown task " + req.Task + " in change " + id})
		return
	}
	// A supplied caller must belong to this change.
	if req.Session != "" {
		if bound, ok := s.sessions.changeOf(req.Session); ok && bound != id {
			writeJSON(w, http.StatusConflict, map[string]string{
				"change": bound,
				"error":  "session already bound to change " + bound,
			})
			return
		}
	}
	// The sub must exist; capture its live parent and title.
	ctx, cancel := context.WithTimeout(r.Context(), 10*time.Second)
	defer cancel()
	sub, err := s.oc.GetSession(ctx, req.Sub)
	if err != nil {
		writeJSON(w, http.StatusUnprocessableEntity, map[string]string{"error": "unknown sub session: " + err.Error()})
		return
	}
	// Idempotent on exact retries; conflicting on a different task or change.
	for _, e := range s.sessions.list(id) {
		if e.Session != req.Sub {
			continue
		}
		if e.Task == req.Task {
			writeJSON(w, http.StatusOK, sessionResponse{Session: e.Session, Title: e.Title, Created: e.Created, Live: true})
			return
		}
		writeJSON(w, http.StatusConflict, map[string]string{"error": "session already bound to task " + e.Task + " in " + id})
		return
	}
	if other, ok := s.sessions.changeOf(req.Sub); ok && other != id {
		writeJSON(w, http.StatusConflict, map[string]string{
			"change": other,
			"error":  "session already bound to change " + other,
		})
		return
	}
	entry := SessionEntry{Session: sub.ID, Title: sub.Title, Created: time.Now().Format(time.RFC3339), Task: req.Task, Parent: sub.ParentID}
	if err := s.sessions.add(id, entry); err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "persist mapping: " + err.Error()})
		return
	}
	slog.Info("task session bound", "change", id, "task", req.Task, "session", req.Sub, "parent", sub.ParentID)
	writeJSON(w, http.StatusCreated, sessionResponse{Session: entry.Session, Title: entry.Title, Created: entry.Created, Live: true})
}

// scaffoldRequest is the body of POST /changes/scaffold, the deterministic
// trigger the discussion agent calls after explicit user approval.
type scaffoldRequest struct {
	Title   string `json:"title"`
	Prefix  string `json:"prefix"`
	Session string `json:"session"`
}

// scaffoldChange handles POST /changes/scaffold: build the change with the
// agreed title/prefix, rename the session, and move it to the change.
func (s *Server) scaffoldChange(w http.ResponseWriter, r *http.Request) {
	var req scaffoldRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "bad JSON body"})
		return
	}
	req.Title = strings.TrimSpace(req.Title)
	if req.Title == "" || strings.Contains(req.Title, "|") {
		writeJSON(w, http.StatusUnprocessableEntity, map[string]string{"error": "title must be non-empty and contain no |"})
		return
	}
	if req.Prefix != "" && !prefixRe.MatchString(req.Prefix) {
		writeJSON(w, http.StatusUnprocessableEntity, map[string]string{"error": "prefix must be 2–4 uppercase letters/digits or empty"})
		return
	}
	if !strings.HasPrefix(req.Session, "ses_") {
		writeJSON(w, http.StatusUnprocessableEntity, map[string]string{"error": "valid ses_ session id required"})
		return
	}
	if s.mapErr != nil {
		writeJSON(w, http.StatusServiceUnavailable, map[string]string{"error": "session mapping unreadable: " + s.mapErr.Error()})
		return
	}

	// A session already bound to a change can never scaffold a new one: all
	// of its work belongs to that change. The message is agent-facing so a
	// misled agent self-corrects — including sessions primed before the
	// handoff endpoint existed, so the text teaches that endpoint by name.
	if bound, ok := s.sessions.changeOf(req.Session); ok {
		slog.Warn("scaffold refused: session already bound", "session", req.Session, "change", bound)
		writeJSON(w, http.StatusConflict, map[string]string{
			"change": bound,
			"error":  "session already bound to change " + bound + "; continue the work within that change (refine plan.md, add task files and ledger rows) — do not scaffold a new change. To spawn a genuinely new change out of this one, write changes/" + bound + "/handoff-<topic>.md with the full context, then POST /changes/" + bound + "/spawn-change with {\"title\",\"prefix\",\"artifact\",\"session\"} (explicit user approval required)",
		})
		return
	}

	ctx, cancel := context.WithTimeout(r.Context(), 30*time.Second)
	defer cancel()
	id, branch, wtPath, warning, err := s.createChangeRecord(ctx, req.Title, req.Prefix)
	if err != nil {
		writeErr(w, err)
		return
	}
	slog.Info("change scaffolded via API trigger", "id", id, "title", req.Title, "prefix", req.Prefix, "session", req.Session, "branch", branch, "worktree", wtPath)

	// Rename the session server-side (best effort) and move the mapping.
	if s.oc != nil {
		ctx2, cancel2 := context.WithTimeout(r.Context(), 10*time.Second)
		cap, capErr := s.oc.LifecycleCapabilities(ctx2)
		var renameErr error
		if capErr == nil {
			renameErr = s.oc.RenameSessionCompatible(ctx2, cap, req.Session, id+" — "+req.Title)
		} else {
			// If OpenAPI cannot be read, use the current published contract.
			renameErr = s.oc.RenameSession(ctx2, req.Session, id+" — "+req.Title)
		}
		if renameErr != nil {
			slog.Warn("session rename failed", "session", req.Session, "err", renameErr)
		}
		cancel2()
	}
	moved, err := s.sessions.moveToChange(req.Session, id)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"change": id, "error": "persist mapping: " + err.Error()})
		return
	}
	if !moved {
		// Idempotent for retries; also covers sessions created outside the flow.
		slog.Warn("session not in unassigned bucket; linking directly", "session", req.Session, "change", id)
		if s.oc != nil {
			title := id + " — " + req.Title
			_ = s.sessions.add(id, SessionEntry{Session: req.Session, Title: title, Created: time.Now().Format(time.RFC3339)})
		}
	}
	resp := map[string]string{"change": id, "session": req.Session, "title": req.Title}
	if branch != "" {
		resp["branch"] = branch
		resp["worktree"] = wtPath
		resp["worktree-note"] = fmt.Sprintf(
			"This change's files live inside the worktree at %[1]s (branch %[2]s), including changes/%[3]s/. Edit them there by absolute path — your own working directory is the main tree. Later sessions for this change start inside the worktree.",
			wtPath, branch, id)
	}
	if warning != "" {
		resp["warning"] = warning
	}
	writeJSON(w, http.StatusCreated, resp)
}

// spawnChangeRequest is the body of POST /changes/{id}/spawn-change: create
// a new change out of change {id} with a fresh session seeded from a
// handoff artifact authored inside {id}. Session is the optional caller
// session; when supplied it must be bound to {id} (the board omits it).
type spawnChangeRequest struct {
	Title    string `json:"title"`
	Prefix   string `json:"prefix"`
	Artifact string `json:"artifact"`
	Session  string `json:"session,omitempty"`
}

// handoffAddendum is appended to the new session's change prime: the
// authoritative starting context lives in the source change's artifact and
// must be distilled into canonical workflow data — task files together
// with their governing-ledger rows — before implementation.
func handoffAddendum(source, artifact string) string {
	return fmt.Sprintf(`This session was spawned by a handoff from change %[1]s. First read changes/%[1]s/%[2]s — it is the authoritative starting context authored by the source change. Distill it into this change's plan.md and tasks, creating every task file together with its row in the governing ledger in the same pass (AGENTS.md rule 3: one ledger row per task file, never one without the other). Run the validator (lessmess validate or GET /api/validate) and fix every violation before any implementation, keeping the user in the loop; then work the change normally.`, source, artifact)
}

// handoffArtifactPath resolves one handoff artifact inside the change's
// directory. Strict: a bare handoff*.md filename at the change root — the
// same convention the handoffs listing (HOF-01) serves, so plan.md,
// ledger.md, and tasks/ can never be mistaken for context. changeDir is
// the resolved change directory (the worktree for worktree-backed
// changes).
func handoffArtifactPath(changeDir, artifact string) (string, error) {
	if artifact == "" {
		return "", errors.New("artifact is required")
	}
	if filepath.Base(artifact) != artifact || artifact == "." || artifact == ".." || strings.ContainsRune(artifact, '/') || strings.ContainsRune(artifact, '\\') {
		return "", errors.New("artifact must be a bare filename inside the change directory")
	}
	if !strings.HasPrefix(artifact, "handoff") || !strings.HasSuffix(artifact, ".md") {
		return "", errors.New("artifact must be a handoff*.md file in the change directory")
	}
	p := filepath.Join(changeDir, artifact)
	if info, err := os.Stat(p); err != nil || info.IsDir() {
		return "", errors.New("artifact not found in the change directory: " + artifact)
	}
	return p, nil
}

// sourceChangeDir resolves the directory of the source change (main tree
// or worktree) for handoff reads.
func (s *Server) sourceChangeDir(id string) (string, error) {
	c, err := s.st.Change(id)
	if err != nil {
		return "", err
	}
	return c.Dir, nil
}

// spawnChange handles POST /changes/{id}/spawn-change: the one sanctioned
// way for an existing change to spawn a new change. The new change is
// canonical once created (never rolled back); a failed spawn or prime
// deletes the session so nothing unbound leaks, and the 502 names the new
// change so the user can continue it from the board.
func (s *Server) spawnChange(w http.ResponseWriter, r *http.Request) {
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
	var req spawnChangeRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "bad JSON body"})
		return
	}
	req.Title = strings.TrimSpace(req.Title)
	if req.Title == "" || strings.Contains(req.Title, "|") {
		writeJSON(w, http.StatusUnprocessableEntity, map[string]string{"error": "title must be non-empty and contain no |"})
		return
	}
	if req.Prefix != "" && !prefixRe.MatchString(req.Prefix) {
		writeJSON(w, http.StatusUnprocessableEntity, map[string]string{"error": "prefix must be 2–4 uppercase letters/digits or empty"})
		return
	}
	req.Artifact = strings.TrimSpace(req.Artifact)
	sourceDir, err := s.sourceChangeDir(id)
	if err != nil {
		writeErr(w, err)
		return
	}
	if _, err := handoffArtifactPath(sourceDir, req.Artifact); err != nil {
		writeJSON(w, http.StatusUnprocessableEntity, map[string]string{"error": err.Error()})
		return
	}
	// A supplied caller must belong to the source change.
	if req.Session != "" {
		if bound, ok := s.sessions.changeOf(req.Session); ok && bound != id {
			writeJSON(w, http.StatusConflict, map[string]string{
				"change": bound,
				"error":  "session already bound to change " + bound + "; a handoff can only be spawned by a session of change " + id,
			})
			return
		}
	}

	spawned, branch, wtPath, _, err := s.createChangeRecord(r.Context(), req.Title, req.Prefix)
	if err != nil {
		writeErr(w, err)
		return
	}
	slog.Info("change spawned via handoff", "source", id, "change", spawned, "title", req.Title, "prefix", req.Prefix, "artifact", req.Artifact, "session", req.Session, "branch", branch, "worktree", wtPath)

	title := spawned + " — " + req.Title
	ctx, cancel := context.WithTimeout(r.Context(), 15*time.Second)
	defer cancel()
	sess, err := s.spawnSessionIn(ctx, s.changeSessionDir(spawned), title)
	if err != nil {
		writeJSON(w, http.StatusBadGateway, map[string]string{"change": spawned, "error": "create opencode session: " + err.Error()})
		return
	}
	base, modules := s.changePrime(spawned, sess.ID)
	prime := s.promptWith(base+"\n\n"+handoffAddendum(id, req.Artifact), "change")
	if err := s.oc.Prompt(ctx, sess.ID, prime); err != nil {
		_ = s.oc.DeleteSession(context.Background(), sess.ID)
		writeJSON(w, http.StatusBadGateway, map[string]string{"change": spawned, "error": "prime handoff session: " + err.Error()})
		return
	}
	logPrime(sess.ID, "change", modules)
	if err := s.sessions.add(spawned, SessionEntry{Session: sess.ID, Title: title, Created: time.Now().Format(time.RFC3339), SpawnedFrom: id, Modules: modules}); err != nil {
		slog.Error("mapping add", "err", err)
		_ = s.oc.DeleteSession(context.Background(), sess.ID) // don't leak an unmapped session
		writeJSON(w, http.StatusInternalServerError, map[string]string{"change": spawned, "error": "persist mapping: " + err.Error()})
		return
	}
	slog.Info("handoff session created", "source", id, "change", spawned, "session", sess.ID)
	writeJSON(w, http.StatusCreated, map[string]string{"change": spawned, "session": sess.ID, "title": req.Title})
}

// listHandoffs handles GET /changes/{id}/handoffs: the handoff*.md
// artifacts authored inside the change directory, sorted — the picker
// behind the board's spawn action.
func (s *Server) listHandoffs(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	c, err := s.st.Change(id)
	if err != nil {
		writeErr(w, err)
		return
	}
	matches, err := filepath.Glob(filepath.Join(c.Dir, "handoff*.md"))
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}
	sort.Strings(matches)
	handoffs := make([]string, 0, len(matches))
	for _, m := range matches {
		handoffs = append(handoffs, filepath.Base(m))
	}
	writeJSON(w, http.StatusOK, map[string]any{"handoffs": handoffs})
}
