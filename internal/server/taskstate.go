package server

import (
	"encoding/json"
	"errors"
	"log/slog"
	"net/http"
	"strings"

	"lessmess/internal/model"
	"lessmess/internal/store"
)

// The deterministic task-state API: every workflow mutation an agent or
// the board needs, with the workflow rules enforced server-side.
//
// Caller identity: the board's fetches carry X-Lessmess-UI; agent-side
// calls (curl from a session) do not. Transitions reserved for the user
// (task Done) are refused for non-UI callers — the workflow's "only the
// user moves a task to Done" rule becomes an endpoint invariant instead
// of an instruction.

// uiClient reports whether the request comes from the board UI.
func uiClient(r *http.Request) bool { return r.Header.Get("X-Lessmess-UI") != "" }

type taskStatusRequest struct {
	Status   string `json:"status"`
	Evidence string `json:"evidence,omitempty"`
}

// setTaskStatus handles POST /changes/{id}/tasks/{task}/status.
func (s *Server) setTaskStatus(w http.ResponseWriter, r *http.Request) {
	id, task := r.PathValue("id"), r.PathValue("task")
	var req taskStatusRequest
	if isJSON(r) {
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			writeJSON(w, http.StatusBadRequest, map[string]string{"error": "bad JSON body"})
			return
		}
	} else {
		_ = r.ParseForm()
		req.Status = r.FormValue("status")
		req.Evidence = r.FormValue("evidence")
	}
	status := model.TaskStatus(strings.TrimSpace(req.Status))
	if !status.Valid() {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid status " + req.Status})
		return
	}
	// Done is user-gated: only the board (the user's hands) may set it.
	// An agent asked to finish a task stops at Test and reports readiness.
	if status == model.StatusDone && !uiClient(r) {
		writeJSON(w, http.StatusForbidden, map[string]string{
			"error": "Done is user-gated: set Test with verification evidence instead; the user accepts Done on the board",
		})
		return
	}
	ts, err := s.st.SetTaskStatus(id, task, status, req.Evidence)
	if err != nil {
		if errors.Is(err, store.ErrInvalid) && strings.Contains(err.Error(), "evidence") {
			writeJSON(w, http.StatusUnprocessableEntity, map[string]string{"error": err.Error()})
			return
		}
		writeErr(w, err)
		return
	}
	slog.Info("task status", "change", id, "task", task, "status", status, "ui", uiClient(r))
	writeJSON(w, http.StatusOK, ts)
}

type taskUpdateRequest struct {
	Title     *string   `json:"title,omitempty"`
	Notes     *string   `json:"notes,omitempty"`
	DependsOn *[]string `json:"dependsOn,omitempty"`
}

// updateTask handles POST /changes/{id}/tasks/{task}/update: patch title,
// notes, and dependencies.
func (s *Server) updateTask(w http.ResponseWriter, r *http.Request) {
	id, task := r.PathValue("id"), r.PathValue("task")
	var req taskUpdateRequest
	if !isJSON(r) {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "JSON body required"})
		return
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "bad JSON body"})
		return
	}
	ts, err := s.st.UpdateTask(id, task, store.TaskUpdate{
		Title: req.Title, Notes: req.Notes, DependsOn: req.DependsOn,
	})
	if err != nil {
		writeErr(w, err)
		return
	}
	slog.Info("task update", "change", id, "task", task)
	writeJSON(w, http.StatusOK, ts)
}

type reorderRequest struct {
	Parent  string   `json:"parent,omitempty"` // governing level ("" = top-level)
	Ordered []string `json:"ordered"`
}

// reorderTasks handles POST /changes/{id}/tasks/reorder: set one level's
// priority order. Row order is the display and priority order.
func (s *Server) reorderTasks(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	var req reorderRequest
	if !isJSON(r) {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "JSON body required"})
		return
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "bad JSON body"})
		return
	}
	if len(req.Ordered) == 0 {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "ordered is required"})
		return
	}
	if err := s.st.ReorderTasks(id, strings.TrimSpace(req.Parent), req.Ordered); err != nil {
		writeErr(w, err)
		return
	}
	slog.Info("tasks reordered", "change", id, "parent", req.Parent, "count", len(req.Ordered))
	writeJSON(w, http.StatusOK, map[string]string{"ok": "true"})
}

type decisionRequest struct {
	Date     string `json:"date,omitempty"`
	Decision string `json:"decision"`
}

// appendDecision handles POST /changes/{id}/decisions: record a material
// implementation or scope decision in the change's decision log.
func (s *Server) appendDecision(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	var req decisionRequest
	if !isJSON(r) {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "JSON body required"})
		return
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "bad JSON body"})
		return
	}
	if err := s.st.AppendDecision(id, req.Date, req.Decision); err != nil {
		writeErr(w, err)
		return
	}
	slog.Info("decision appended", "change", id)
	writeJSON(w, http.StatusCreated, map[string]string{"ok": "true"})
}
