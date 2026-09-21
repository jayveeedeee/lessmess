package server

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"lessmess/internal/model"
)

// doUI marks a request as coming from the board: the user's hands.
func doUI(t *testing.T, h http.Handler, method, path, body string) *httptest.ResponseRecorder {
	t.Helper()
	var r *http.Request
	if body != "" {
		r = httptest.NewRequest(method, path, strings.NewReader(body))
		r.Header.Set("Content-Type", "application/json")
	} else {
		r = httptest.NewRequest(method, path, nil)
	}
	r.Header.Set("Accept", "application/json")
	r.Header.Set("X-Lessmess-UI", "1")
	w := httptest.NewRecorder()
	h.ServeHTTP(w, r)
	return w
}

func TestTaskStatusEndpoint(t *testing.T) {
	s := mappingServer(t, nil)
	h := s.Handler()

	// Plain transition with evidence: 200, notes carry the evidence.
	w := do(t, h, "POST", "/changes/2026-09-10-0/tasks/FIX-00/status", `{"status":"In progress","evidence":"started JSI-00"}`)
	if w.Code != 200 {
		t.Fatalf("code = %d body = %s", w.Code, w.Body)
	}
	var ts model.TaskState
	json.Unmarshal(w.Body.Bytes(), &ts)
	if ts.Status != model.StatusInProgress || !strings.Contains(ts.Notes, "started JSI-00") {
		t.Fatalf("task = %+v", ts)
	}

	// Test demands verification evidence: none given, none recorded
	// (FIX-01 has no notes yet).
	w = do(t, h, "POST", "/changes/2026-09-10-0/tasks/FIX-01/status", `{"status":"Test"}`)
	if w.Code != http.StatusUnprocessableEntity || !strings.Contains(w.Body.String(), "evidence") {
		t.Fatalf("evidence gate: code = %d body = %s", w.Code, w.Body)
	}
	w = do(t, h, "POST", "/changes/2026-09-10-0/tasks/FIX-01/status", `{"status":"In progress","evidence":"vet+tests green"}`)
	if w.Code != 200 {
		t.Fatalf("In progress with evidence: code = %d body = %s", w.Code, w.Body)
	}

	// Invalid vocabulary and unknown tasks.
	if w := do(t, h, "POST", "/changes/2026-09-10-0/tasks/FIX-00/status", `{"status":"Later"}`); w.Code != 400 {
		t.Fatalf("bad status: code = %d", w.Code)
	}
	if w := do(t, h, "POST", "/changes/2026-09-10-0/tasks/FIX-99/status", `{"status":"Blocked"}`); w.Code != 404 {
		t.Fatalf("unknown task: code = %d", w.Code)
	}
}

func TestTaskStatusDoneUserGated(t *testing.T) {
	s := mappingServer(t, nil)
	h := s.Handler()

	// Agent-representing caller (no UI header): refused with guidance.
	w := do(t, h, "POST", "/changes/2026-09-10-0/tasks/FIX-00/status", `{"status":"Done"}`)
	if w.Code != http.StatusForbidden {
		t.Fatalf("agent Done: code = %d body = %s", w.Code, w.Body)
	}
	if !strings.Contains(w.Body.String(), "user-gated") {
		t.Errorf("body lacks guidance: %s", w.Body)
	}
	// The drag endpoint carries the same gate.
	w = do(t, h, "POST", "/changes/2026-09-10-0/move", `{"task":"FIX-00","status":"Done","index":0}`)
	if w.Code != http.StatusForbidden {
		t.Fatalf("agent Done via move: code = %d body = %s", w.Code, w.Body)
	}

	// The board (UI header) is the user's hands: allowed.
	w = doUI(t, h, "POST", "/changes/2026-09-10-0/tasks/FIX-00/status", `{"status":"Done"}`)
	if w.Code != 200 {
		t.Fatalf("UI Done: code = %d body = %s", w.Code, w.Body)
	}
	w = doUI(t, h, "POST", "/changes/2026-09-10-0/move", `{"task":"FIX-01","status":"Done","index":0}`)
	if w.Code != 200 {
		t.Fatalf("UI Done via move: code = %d body = %s", w.Code, w.Body)
	}
	c, _ := s.st.Change("2026-09-10-0")
	if c.Node("FIX-00").NodeStatus() != model.StatusDone || c.Node("FIX-01").NodeStatus() != model.StatusDone {
		t.Fatal("Done writes missing")
	}
}

func TestTaskUpdateEndpoint(t *testing.T) {
	s := mappingServer(t, nil)
	h := s.Handler()

	w := do(t, h, "POST", "/changes/2026-09-10-0/tasks/FIX-00/update", `{"notes":"verified on a real-tree copy","dependsOn":["FIX-01"]}`)
	if w.Code != 200 {
		t.Fatalf("code = %d body = %s", w.Code, w.Body)
	}
	var ts model.TaskState
	json.Unmarshal(w.Body.Bytes(), &ts)
	if ts.Notes != "verified on a real-tree copy" || len(ts.DependsOn) != 1 || ts.DependsOn[0] != "FIX-01" {
		t.Fatalf("task = %+v", ts)
	}

	// Unknown dependencies are refused by state validation.
	w = do(t, h, "POST", "/changes/2026-09-10-0/tasks/FIX-00/update", `{"dependsOn":["GONE-00"]}`)
	if w.Code != http.StatusUnprocessableEntity || !strings.Contains(w.Body.String(), "unknown dependency") {
		t.Fatalf("unknown dep: code = %d body = %s", w.Code, w.Body)
	}
	// Empty title refused; unknown task 404.
	if w := do(t, h, "POST", "/changes/2026-09-10-0/tasks/FIX-00/update", `{"title":"  "}`); w.Code != 422 {
		t.Fatalf("empty title: code = %d", w.Code)
	}
	if w := do(t, h, "POST", "/changes/2026-09-10-0/tasks/NOPE-9/update", `{"title":"x"}`); w.Code != 404 {
		t.Fatalf("unknown task: code = %d", w.Code)
	}
}

func TestReorderEndpoint(t *testing.T) {
	s := mappingServer(t, nil)
	h := s.Handler()

	w := do(t, h, "POST", "/changes/2026-09-10-0/tasks/reorder", `{"ordered":["FIX-01","FIX-00"]}`)
	if w.Code != 200 {
		t.Fatalf("code = %d body = %s", w.Code, w.Body)
	}
	c, _ := s.st.Change("2026-09-10-0")
	if c.Roots[0].ID != "FIX-01" || c.Roots[1].ID != "FIX-00" {
		t.Fatalf("roots after reorder = [%s, %s]", c.Roots[0].ID, c.Roots[1].ID)
	}

	// Partial lists, duplicates, and foreign tasks are refused.
	if w := do(t, h, "POST", "/changes/2026-09-10-0/tasks/reorder", `{"ordered":["FIX-00"]}`); w.Code != 422 {
		t.Fatalf("partial: code = %d", w.Code)
	}
	if w := do(t, h, "POST", "/changes/2026-09-10-0/tasks/reorder", `{"ordered":["FIX-00","FIX-00"]}`); w.Code != 422 {
		t.Fatalf("dup: code = %d", w.Code)
	}
	if w := do(t, h, "POST", "/changes/2026-09-10-0/tasks/reorder", `{"ordered":["FIX-00","FIX-99"]}`); w.Code != 422 {
		t.Fatalf("foreign: code = %d", w.Code)
	}
	if w := do(t, h, "POST", "/changes/2026-09-10-0/tasks/reorder", `{"ordered":[]}`); w.Code != 400 {
		t.Fatalf("empty: code = %d", w.Code)
	}
}

func TestDecisionsEndpoint(t *testing.T) {
	s := mappingServer(t, nil)
	h := s.Handler()

	if w := do(t, h, "POST", "/changes/2026-09-10-0/decisions", `{"decision":"Board renders from JSON"}`); w.Code != 201 {
		t.Fatalf("code = %d body = %s", w.Code, w.Body)
	}
	if w := do(t, h, "POST", "/changes/2026-09-10-0/decisions", `{"decision":"  "}`); w.Code != 422 {
		t.Fatalf("empty decision: code = %d", w.Code)
	}
	view, err := s.st.LedgerFile("2026-09-10-0")
	if err != nil || !strings.Contains(view, "Board renders from JSON") {
		t.Fatalf("decision not in state view: %v", err)
	}
	// Explicit date is honored.
	if w := do(t, h, "POST", "/changes/2026-09-10-0/decisions", `{"date":"2026-09-01","decision":"dated"}`); w.Code != 201 {
		t.Fatalf("dated: code = %d", w.Code)
	}
}
