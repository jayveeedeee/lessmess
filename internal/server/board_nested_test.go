package server

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"

	"lessmess/internal/model"
)

// doReq runs a pre-built request (for control over headers and raw paths).
func doReq(t *testing.T, h http.Handler, r *http.Request) *httptest.ResponseRecorder {
	t.Helper()
	w := httptest.NewRecorder()
	h.ServeHTTP(w, r)
	return w
}

// nestedBoardServer returns a server whose fixture change has FIX-00
// decomposed into two children (FIX-00.00 Test, FIX-00.01 Not started).
func nestedBoardServer(t *testing.T) *Server {
	t.Helper()
	s := mappingServer(t, nil)
	if _, err := s.st.DecomposeTask("2026-09-10-0", "FIX-00"); err != nil {
		t.Fatal(err)
	}
	for _, tc := range []struct{ title, status string }{
		{"Child One", "Test"},
		{"Child Two", "Not started"},
	} {
		row, err := s.st.CreateTask("2026-09-10-0", "FIX-00", tc.title)
		if err != nil {
			t.Fatal(err)
		}
		if err := s.st.MoveTask("2026-09-10-0", row.ID, model.TaskStatus(tc.status), 0); err != nil {
			t.Fatal(err)
		}
	}
	return s
}

func TestNestedTaskDetail(t *testing.T) {
	s := nestedBoardServer(t)
	h := s.Handler()

	req, _ := http.NewRequest("GET", "/changes/2026-09-10-0/tasks/00-first/tasks/00-child-one.md", nil)
	req.Header.Set("Accept", "text/html")
	w := doReq(t, h, req)
	if w.Code != 200 {
		t.Fatalf("HTML code = %d body = %s", w.Code, w.Body)
	}
	if !strings.Contains(w.Body.String(), "FIX-00.00") {
		t.Error("HTML missing child ID")
	}
	if !strings.Contains(w.Body.String(), "status-test") {
		t.Error("HTML missing governing-ledger status pill")
	}

	req2, _ := http.NewRequest("GET", "/changes/2026-09-10-0/tasks/00-first/tasks/00-child-one.md", nil)
	req2.Header.Set("Accept", "application/json")
	w2 := doReq(t, h, req2)
	if w2.Code != 200 {
		t.Fatalf("JSON code = %d", w2.Code)
	}
	var resp map[string]string
	json.Unmarshal(w2.Body.Bytes(), &resp)
	if resp["id"] != "FIX-00.00" {
		t.Errorf("json id = %q", resp["id"])
	}
}

func TestNestedTaskDetailTraversal(t *testing.T) {
	s := nestedBoardServer(t)
	h := s.Handler()
	for _, path := range []string{
		"/changes/2026-09-10-0/tasks/../../../cmd/lessmess/main.go",
		"/changes/2026-09-10-0/tasks/00-first/../../ledger.md",
	} {
		// Use a raw request so the client does not normalize the path.
		req, _ := http.NewRequest("GET", path, nil)
		req.URL.Path = path
		resp := doReq(t, h, req)
		// The mux may reject traversal outright (301/404) or the store
		// guard returns 404; anything but 200 with content is fine.
		if resp.Code == 200 && strings.Contains(resp.Body.String(), "package main") {
			t.Fatalf("traversal %q served file content", path)
		}
	}
}

func TestExpandEndpoint(t *testing.T) {
	s := mappingServer(t, nil)
	h := s.Handler()

	w := do(t, h, "POST", "/changes/2026-09-10-0/expand", `{"task":"FIX-01"}`)
	if w.Code != 201 {
		t.Fatalf("expand code = %d body = %s", w.Code, w.Body)
	}
	var resp map[string]string
	json.Unmarshal(w.Body.Bytes(), &resp)
	if resp["container"] != "tasks/01-second" {
		t.Errorf("container = %q", resp["container"])
	}

	// Second expand conflicts; unknown tasks and changes 404; empty body 400.
	if w := do(t, h, "POST", "/changes/2026-09-10-0/expand", `{"task":"FIX-01"}`); w.Code != 409 {
		t.Errorf("re-expand code = %d", w.Code)
	}
	if w := do(t, h, "POST", "/changes/2026-09-10-0/expand", `{"task":"FIX-99"}`); w.Code != 404 {
		t.Errorf("unknown task code = %d", w.Code)
	}
	if w := do(t, h, "POST", "/changes/2099-01-01-9/expand", `{"task":"FIX-00"}`); w.Code != 404 {
		t.Errorf("unknown change code = %d", w.Code)
	}
	if w := do(t, h, "POST", "/changes/2026-09-10-0/expand", `{}`); w.Code != 400 {
		t.Errorf("empty body code = %d", w.Code)
	}
}

func TestBoardDrillDown(t *testing.T) {
	s := nestedBoardServer(t)
	h := s.Handler()

	req, _ := http.NewRequest("GET", "/changes/2026-09-10-0?task=FIX-00", nil)
	req.Header.Set("Accept", "text/html")
	w := doReq(t, h, req)
	if w.Code != 200 {
		t.Fatalf("HTML code = %d body = %s", w.Code, w.Body)
	}
	body := w.Body.String()
	for _, want := range []string{"Child One", "Child Two", "FIX-00.00", "data-task=\"FIX-00.00\""} {
		if !strings.Contains(body, want) {
			t.Errorf("drill-down HTML missing %q", want)
		}
	}
	// Root-board tasks do not leak into the drill-down.
	if strings.Contains(body, ">Second<") {
		t.Error("drill-down shows root-level task Second")
	}

	reqJSON, _ := http.NewRequest("GET", "/changes/2026-09-10-0?task=FIX-00", nil)
	reqJSON.Header.Set("Accept", "application/json")
	w2 := doReq(t, h, reqJSON)
	var resp struct {
		Task    string `json:"task"`
		Columns []struct {
			Status string `json:"status"`
			Count  int    `json:"count"`
		} `json:"columns"`
	}
	json.Unmarshal(w2.Body.Bytes(), &resp)
	if resp.Task != "FIX-00" {
		t.Errorf("task = %q", resp.Task)
	}
	for _, col := range resp.Columns {
		if col.Status == "Not started" && col.Count != 1 {
			t.Errorf("Not started count = %d, want 1", col.Count)
		}
	}

	if w := do(t, h, "GET", "/changes/2026-09-10-0?task=FIX-99", ""); w.Code != 404 {
		t.Errorf("unknown drill-down code = %d", w.Code)
	}
}

func TestIndexRecursiveCounts(t *testing.T) {
	s := nestedBoardServer(t)
	req, _ := http.NewRequest("GET", "/", nil)
	req.Header.Set("Accept", "application/json")
	w := doReq(t, s.Handler(), req)
	var resp struct {
		Changes []struct {
			ID       string `json:"id"`
			Tasks    int    `json:"tasks"`
			Complete int    `json:"complete"`
			Open     int    `json:"open"`
		} `json:"changes"`
	}
	json.Unmarshal(w.Body.Bytes(), &resp)
	if len(resp.Changes) == 0 {
		t.Fatal("no changes")
	}
	c := resp.Changes[0]
	// All four tasks recursive (2 roots + 2 children); change-level counts
	// include the roots: complete = FIX-00, FIX-01, FIX-00.00 (all Test).
	if c.Tasks != 4 {
		t.Errorf("tasks = %d, want 4 (recursive)", c.Tasks)
	}
	if c.Complete != 3 || c.Open != 1 {
		t.Errorf("complete/open = %d/%d, want 3/1", c.Complete, c.Open)
	}
}

func TestCloseGateRecursive(t *testing.T) {
	s := nestedBoardServer(t)
	h := s.Handler()

	// FIX-00.01 (Not started) blocks the close.
	w := do(t, h, "POST", "/changes/2026-09-10-0/close", `{}`)
	if w.Code != 422 {
		t.Fatalf("close code = %d body = %s", w.Code, w.Body)
	}
	var resp struct {
		Tasks []string `json:"tasks"`
	}
	json.Unmarshal(w.Body.Bytes(), &resp)
	if len(resp.Tasks) == 0 || resp.Tasks[0] != "FIX-00.01" {
		t.Errorf("offending = %v, want [FIX-00.01]", resp.Tasks)
	}

	// Complete the tree and close.
	if err := s.st.MoveTask("2026-09-10-0", "FIX-00.01", model.StatusTest, 0); err != nil {
		t.Fatal(err)
	}
	if w := do(t, h, "POST", "/changes/2026-09-10-0/close", `{}`); w.Code != 200 {
		t.Fatalf("close code = %d body = %s", w.Code, w.Body)
	}
}

func TestCreateSubtaskEndpoint(t *testing.T) {
	s := nestedBoardServer(t)
	w := do(t, s.Handler(), "POST", "/changes/2026-09-10-0/tasks", `{"title":"Child Three","parent":"FIX-00"}`)
	if w.Code != 201 {
		t.Fatalf("code = %d body = %s", w.Code, w.Body)
	}
	var row model.TaskRow
	json.Unmarshal(w.Body.Bytes(), &row)
	if row.ID != "FIX-00.02" {
		t.Errorf("row = %+v, want FIX-00.02", row)
	}

	// Subtask under a non-decomposed task is a clean 422.
	if w := do(t, s.Handler(), "POST", "/changes/2026-09-10-0/tasks", `{"title":"Nope","parent":"FIX-01"}`); w.Code != 422 {
		t.Errorf("non-decomposed parent code = %d", w.Code)
	}

	// Form-encoded variant carries the parent too.
	form := url.Values{"title": {"Child Four"}, "parent": {"FIX-00"}}
	req, _ := http.NewRequest("POST", "/changes/2026-09-10-0/tasks", strings.NewReader(form.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	w2 := doReq(t, s.Handler(), req)
	if w2.Code != 201 {
		t.Fatalf("form code = %d body = %s", w2.Code, w2.Body)
	}
	var row2 model.TaskRow
	json.Unmarshal(w2.Body.Bytes(), &row2)
	if row2.ID != "FIX-00.03" {
		t.Errorf("form row = %+v, want FIX-00.03", row2)
	}
}
