package server

import (
	"bufio"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"lessmess/internal/model"
	"lessmess/internal/store"
)

// fixtureStore opens a store on a tempdir fixture repo: one change with
// two Test tasks (close-ready), JSON state + prose files.
func fixtureStore(t *testing.T) (*store.Store, string) {
	t.Helper()
	dir := t.TempDir()
	cdir := filepath.Join(dir, "changes", "2026-09-10-0")
	if err := os.MkdirAll(filepath.Join(cdir, "tasks"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(cdir, "plan.md"), model.RenderChangePlan("2026-09-10-0", "Fixture change", "2026-09-10"), 0o644); err != nil {
		t.Fatal(err)
	}
	os.WriteFile(filepath.Join(cdir, "tasks", "00-first.md"), model.RenderTaskFile("FIX-00", "First"), 0o644)
	os.WriteFile(filepath.Join(cdir, "tasks", "01-second.md"), model.RenderTaskFile("FIX-01", "Second"), 0o644)

	wd := filepath.Join(dir, store.StateDirName, "workflow")
	if err := os.MkdirAll(filepath.Join(wd, "changes"), 0o755); err != nil {
		t.Fatal(err)
	}
	st := &model.ChangeState{
		Version: model.StateVersion,
		ID:      "2026-09-10-0",
		Title:   "Fixture change",
		Prefix:  "FIX",
		Status:  model.ChangeStatus{Value: model.OverallInProgress, Derived: true},
		Created: "2026-09-10",
		Updated: "2026-09-10",
		Tasks: []model.TaskState{
			{ID: "FIX-00", Seq: 0, Title: "First", File: "tasks/00-first.md", Status: model.StatusTest, Updated: "2026-09-10"},
			{ID: "FIX-01", Seq: 1, Title: "Second", File: "tasks/01-second.md", Status: model.StatusTest, Updated: "2026-09-10"},
		},
	}
	if err := st.Save(filepath.Join(wd, "changes", "2026-09-10-0.json")); err != nil {
		t.Fatal(err)
	}
	idx := &model.WorkflowIndex{
		Version: model.StateVersion,
		Changes: []model.IndexEntry{{ID: "2026-09-10-0", Title: "Fixture change", Prefix: "FIX", Created: "2026-09-10"}},
	}
	if err := idx.Save(filepath.Join(wd, "index.json")); err != nil {
		t.Fatal(err)
	}

	sto, err := store.Open(dir)
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	t.Cleanup(sto.Close)
	return sto, dir
}

// addFixtureChange registers another change in the fixture repo: prose
// directory, empty state, and index entry.
func addFixtureChange(t *testing.T, dir, id, title string) {
	t.Helper()
	date := id[:strings.LastIndex(id, "-")]
	cdir := filepath.Join(dir, "changes", id)
	if err := os.MkdirAll(filepath.Join(cdir, "tasks"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(cdir, "plan.md"), model.RenderChangePlan(id, title, date), 0o644); err != nil {
		t.Fatal(err)
	}
	wd := filepath.Join(dir, store.StateDirName, "workflow")
	st := &model.ChangeState{
		Version: model.StateVersion, ID: id, Title: title, Prefix: "NEW",
		Status:  model.ChangeStatus{Value: model.OverallInProgress, Derived: true},
		Created: date, Updated: date,
	}
	if err := st.Save(filepath.Join(wd, "changes", id+".json")); err != nil {
		t.Fatal(err)
	}
	idx, err := model.LoadWorkflowIndex(filepath.Join(wd, "index.json"))
	if err != nil {
		t.Fatal(err)
	}
	idx.Changes = append(idx.Changes, model.IndexEntry{ID: id, Title: title, Prefix: "NEW", Created: date})
	if err := idx.Save(filepath.Join(wd, "index.json")); err != nil {
		t.Fatal(err)
	}
}

// fixtureState rewrites the fixture change's state file.
func fixtureState(t *testing.T, dir string, f func(*model.ChangeState)) {
	t.Helper()
	p := filepath.Join(dir, store.StateDirName, "workflow", "changes", "2026-09-10-0.json")
	st, err := model.LoadChangeState(p)
	if err != nil {
		t.Fatal(err)
	}
	f(st)
	if err := st.Save(p); err != nil {
		t.Fatal(err)
	}
}

func do(t *testing.T, h http.Handler, method, path, body string) *httptest.ResponseRecorder {
	t.Helper()
	var r *http.Request
	if body != "" {
		r = httptest.NewRequest(method, path, strings.NewReader(body))
		r.Header.Set("Content-Type", "application/json")
	} else {
		r = httptest.NewRequest(method, path, nil)
	}
	r.Header.Set("Accept", "application/json")
	w := httptest.NewRecorder()
	h.ServeHTTP(w, r)
	return w
}

func TestIndex(t *testing.T) {
	st, _ := fixtureStore(t)
	w := do(t, New(st).Handler(), "GET", "/", "")
	if w.Code != 200 {
		t.Fatalf("code = %d body = %s", w.Code, w.Body)
	}
	var resp struct {
		Changes []map[string]any `json:"changes"`
	}
	if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
		t.Fatal(err)
	}
	if len(resp.Changes) != 1 || resp.Changes[0]["id"] != "2026-09-10-0" || resp.Changes[0]["tasks"] != float64(2) {
		t.Fatalf("resp = %v", resp.Changes)
	}
}

func TestIndexNewestFirst(t *testing.T) {
	st, dir := fixtureStore(t)
	addChange := func(id, title string) { addFixtureChange(t, dir, id, title) }
	addChange("2026-09-11-k3x9q", "Random A")
	addChange("2026-09-11-0", "Legacy numeric")
	addChange("2026-09-11-mz7t2", "Random B") // appended last → newest of the date
	addChange("2026-09-10-4", "Older date")
	st.Reload()

	w := do(t, New(st).Handler(), "GET", "/", "")
	if w.Code != 200 {
		t.Fatalf("code = %d body = %s", w.Code, w.Body)
	}
	var resp struct {
		Changes []map[string]any `json:"changes"`
	}
	if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
		t.Fatal(err)
	}
	// Newest first: the older date sorts last; within 2026-09-11 the
	// root-ledger append order is reversed (last row = newest). The suffix
	// itself (random or legacy numeric) carries no ordering.
	want := []string{"2026-09-11-mz7t2", "2026-09-11-0", "2026-09-11-k3x9q", "2026-09-10-4", "2026-09-10-0"}
	if len(resp.Changes) != len(want) {
		t.Fatalf("changes = %v, want %v", resp.Changes, want)
	}
	for i, id := range want {
		if resp.Changes[i]["id"] != id {
			t.Fatalf("changes = %v, want %v", resp.Changes, want)
		}
	}
}

func TestBoard(t *testing.T) {
	st, _ := fixtureStore(t)
	w := do(t, New(st).Handler(), "GET", "/changes/2026-09-10-0", "")
	if w.Code != 200 {
		t.Fatalf("code = %d body = %s", w.Code, w.Body)
	}
	var resp struct {
		Overall string `json:"overall"`
		Columns []struct {
			Status string `json:"status"`
			Count  int    `json:"count"`
		} `json:"columns"`
		Tasks []model.TaskRow `json:"tasks"`
	}
	if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
		t.Fatal(err)
	}
	if resp.Overall != "In progress" || len(resp.Tasks) != 2 || len(resp.Columns) != 6 {
		t.Fatalf("resp = %+v", resp)
	}
	if resp.Columns[0].Status != "Not started" || resp.Columns[0].Count != 0 {
		t.Fatalf("columns = %+v", resp.Columns)
	}
	if resp.Columns[3].Status != "Test" || resp.Columns[3].Count != 2 {
		t.Fatalf("columns = %+v; want Test at index 3 with 2 tasks", resp.Columns)
	}
	if resp.Columns[4].Status != "Done" {
		t.Fatalf("columns = %+v; want Done at index 4", resp.Columns)
	}
}

func TestBoardNotFound(t *testing.T) {
	st, _ := fixtureStore(t)
	w := do(t, New(st).Handler(), "GET", "/changes/2099-01-01-9", "")
	if w.Code != 404 {
		t.Fatalf("code = %d", w.Code)
	}
}

func TestTaskDetail(t *testing.T) {
	st, _ := fixtureStore(t)
	w := do(t, New(st).Handler(), "GET", "/changes/2026-09-10-0/tasks/00-first.md", "")
	if w.Code != 200 {
		t.Fatalf("code = %d body = %s", w.Code, w.Body)
	}
	var resp map[string]string
	json.Unmarshal(w.Body.Bytes(), &resp)
	if resp["id"] != "FIX-00" || !strings.Contains(resp["body"], "## Objective") {
		t.Fatalf("resp = %v", resp)
	}
}

func TestLedgerDetail(t *testing.T) {
	st, _ := fixtureStore(t)
	h := New(st).Handler()
	w := do(t, h, "GET", "/changes/2026-09-10-0/ledger", "")
	if w.Code != 200 {
		t.Fatalf("code = %d body = %s", w.Code, w.Body)
	}
	var resp map[string]string
	json.Unmarshal(w.Body.Bytes(), &resp)
	if resp["id"] != "2026-09-10-0" || !strings.Contains(resp["body"], "## Tasks") {
		t.Fatalf("resp = %v", resp)
	}
	// Unknown change ids 404 like planDetail.
	if w := do(t, h, "GET", "/changes/2099-01-01-0/ledger", ""); w.Code != http.StatusNotFound {
		t.Fatalf("unknown change code = %d", w.Code)
	}
}

func TestMoveTaskRoute(t *testing.T) {
	st, dir := fixtureStore(t)
	h := New(st).Handler()
	w := doUI(t, h, "POST", "/changes/2026-09-10-0/move", `{"task":"FIX-00","status":"Done","index":0}`)
	if w.Code != 200 {
		t.Fatalf("code = %d body = %s", w.Code, w.Body)
	}
	// Verify on disk.
	p := filepath.Join(dir, store.StateDirName, "workflow", "changes", "2026-09-10-0.json")
	stt, err := model.LoadChangeState(p)
	if err != nil {
		t.Fatal(err)
	}
	if ts := stt.Task("FIX-00"); ts == nil || ts.Status != model.StatusDone {
		t.Fatal("state not updated")
	}
}

func TestMoveTaskErrors(t *testing.T) {
	st, _ := fixtureStore(t)
	h := New(st).Handler()
	if w := do(t, h, "POST", "/changes/2026-09-10-0/move", `{"task":"FIX-99","status":"Blocked","index":0}`); w.Code != 404 {
		t.Fatalf("unknown task: code = %d", w.Code)
	}
	if w := do(t, h, "POST", "/changes/2026-09-10-0/move", `{"task":"FIX-00","status":"Doing","index":0}`); w.Code != 400 {
		t.Fatalf("bad status: code = %d", w.Code)
	}
	if w := do(t, h, "POST", "/changes/2026-09-10-0/move", `{bad json`); w.Code != 400 {
		t.Fatalf("bad json: code = %d", w.Code)
	}
}

func TestMoveTaskBrokenLedger(t *testing.T) {
	st, dir := fixtureStore(t)
	p := filepath.Join(dir, store.StateDirName, "workflow", "changes", "2026-09-10-0.json")
	if err := os.WriteFile(p, []byte("{broken"), 0o644); err != nil {
		t.Fatal(err)
	}
	st.Reload()
	w := do(t, New(st).Handler(), "POST", "/changes/2026-09-10-0/move", `{"task":"FIX-00","status":"Blocked","index":0}`)
	if w.Code != 422 {
		t.Fatalf("code = %d, want 422", w.Code)
	}
}

func TestCreateTaskRoute(t *testing.T) {
	st, _ := fixtureStore(t)
	h := New(st).Handler()
	w := do(t, h, "POST", "/changes/2026-09-10-0/tasks", `{"title":"Third task"}`)
	if w.Code != 201 {
		t.Fatalf("code = %d body = %s", w.Code, w.Body)
	}
	var task model.TaskState
	if err := json.Unmarshal(w.Body.Bytes(), &task); err != nil {
		t.Fatal(err)
	}
	if task.ID != "FIX-02" || task.File != "tasks/02-third-task.md" {
		t.Fatalf("task = %+v", task)
	}
	if w := do(t, h, "POST", "/changes/2026-09-10-0/tasks", `{"title":""}`); w.Code != 422 {
		t.Fatalf("empty title: code = %d", w.Code)
	}
}

func TestCreateChangeRoute(t *testing.T) {
	st, _ := fixtureStore(t)
	h := New(st).Handler()
	w := do(t, h, "POST", "/changes/", `{"title":"New objective","prefix":"NO"}`)
	if w.Code != 201 {
		t.Fatalf("code = %d body = %s", w.Code, w.Body)
	}
	var resp map[string]string
	json.Unmarshal(w.Body.Bytes(), &resp)
	if resp["id"] == "" {
		t.Fatalf("resp = %v", resp)
	}
	// Board of the new change is reachable.
	if w := do(t, h, "GET", "/changes/"+resp["id"], ""); w.Code != 200 {
		t.Fatalf("new board: code = %d", w.Code)
	}
}

func TestPlanRouteJSON(t *testing.T) {
	st, _ := fixtureStore(t)
	w := do(t, New(st).Handler(), "GET", "/changes/2026-09-10-0/plan", "")
	if w.Code != 200 {
		t.Fatalf("code = %d body = %s", w.Code, w.Body)
	}
	var resp map[string]string
	if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
		t.Fatal(err)
	}
	if resp["id"] != "2026-09-10-0" || !strings.Contains(resp["body"], "Fixture change") {
		t.Fatalf("resp = %v", resp)
	}
	if w := do(t, New(st).Handler(), "GET", "/changes/2099-01-01-9/plan", ""); w.Code != 404 {
		t.Fatalf("unknown change: code = %d", w.Code)
	}
}

func TestPlanRouteHTML(t *testing.T) {
	st, _ := fixtureStore(t)
	w := htmlGet(t, New(st).Handler(), "/changes/2026-09-10-0/plan", false)
	if w.Code != 200 {
		t.Fatalf("code = %d", w.Code)
	}
	body := w.Body.String()
	if !strings.Contains(body, `class="modal"`) || !strings.Contains(body, "Plan") {
		t.Errorf("plan modal markup missing: %.200s", body)
	}
	// goldmark rendered the plan markdown.
	if !strings.Contains(body, "<h2") {
		t.Errorf("plan markdown not rendered: %.200s", body)
	}
}

func TestValidateRoute(t *testing.T) {
	st, _ := fixtureStore(t)
	w := do(t, New(st).Handler(), "GET", "/api/validate", "")
	if w.Code != 200 {
		t.Fatalf("code = %d", w.Code)
	}
	var resp struct {
		Violations []store.Violation `json:"violations"`
	}
	json.Unmarshal(w.Body.Bytes(), &resp)
	if len(resp.Violations) != 0 {
		t.Fatalf("violations = %v", resp.Violations)
	}
}

func TestSSE(t *testing.T) {
	st, _ := fixtureStore(t)
	srv := httptest.NewServer(New(st).Handler())
	t.Cleanup(srv.Close)

	req, _ := http.NewRequest("GET", srv.URL+"/events", nil)
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	if ct := resp.Header.Get("Content-Type"); ct != "text/event-stream" {
		t.Fatalf("content-type = %q", ct)
	}

	// Trigger a store write; expect an SSE event.
	time.Sleep(100 * time.Millisecond) // let the handler subscribe
	if err := st.MoveTask("2026-09-10-0", "FIX-00", model.StatusDone, 0); err != nil {
		t.Fatal(err)
	}
	lines := bufio.NewScanner(resp.Body)
	deadline := time.After(5 * time.Second)
	got := make(chan bool, 1)
	go func() {
		for lines.Scan() {
			if strings.HasPrefix(lines.Text(), "event: write") {
				got <- true
				return
			}
		}
	}()
	select {
	case <-got:
	case <-deadline:
		t.Fatal("no SSE event within 5s")
	}
}

func mustRead(t *testing.T, p string) []byte {
	t.Helper()
	b, err := os.ReadFile(p)
	if err != nil {
		t.Fatal(err)
	}
	return b
}
