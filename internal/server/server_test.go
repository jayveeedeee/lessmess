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

// fixtureStore opens a store on a tempdir fixture repo.
func fixtureStore(t *testing.T) (*store.Store, string) {
	t.Helper()
	dir := t.TempDir()
	cdir := filepath.Join(dir, "changes", "2026-09-10-0")
	if err := os.MkdirAll(filepath.Join(cdir, "tasks"), 0o755); err != nil {
		t.Fatal(err)
	}
	root := `# Changes — Root Ledger

Task statuses live exclusively in each change's ledger.

| Change | Title | ID prefix | Branch | Status | Created | Last updated |
| --- | --- | --- | --- | --- | --- | --- |
| [2026-09-10-0](2026-09-10-0/plan.md) | Fixture change | FIX | — | In progress | 2026-09-10 | 2026-09-10 |
`
	if err := os.WriteFile(filepath.Join(dir, "changes", "ledger.md"), []byte(root), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(cdir, "plan.md"), model.RenderChangePlan("2026-09-10-0", "Fixture change", "2026-09-10"), 0o644); err != nil {
		t.Fatal(err)
	}
	l, _ := model.ParseChangeLedger("l", model.RenderChangeLedger("2026-09-10-0", "2026-09-10"))
	l.SetOverall(model.OverallInProgress, "2026-09-10")
	l.AppendTask(model.TaskRow{ID: "FIX-00", Href: "tasks/00-first.md", Title: "First", Status: model.StatusNotStarted, Updated: "2026-09-10", Notes: model.Empty})
	l.AppendTask(model.TaskRow{ID: "FIX-01", Href: "tasks/01-second.md", Title: "Second", Status: model.StatusInProgress, Updated: "2026-09-10", Notes: model.Empty})
	if err := os.WriteFile(filepath.Join(cdir, "ledger.md"), l.Content(), 0o644); err != nil {
		t.Fatal(err)
	}
	os.WriteFile(filepath.Join(cdir, "tasks", "00-first.md"), model.RenderTaskFile("FIX-00", "First"), 0o644)
	os.WriteFile(filepath.Join(cdir, "tasks", "01-second.md"), model.RenderTaskFile("FIX-01", "Second"), 0o644)

	st, err := store.Open(dir)
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	t.Cleanup(st.Close)
	return st, dir
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
	addChange := func(id, title string) {
		date := id[:strings.LastIndex(id, "-")]
		cdir := filepath.Join(dir, "changes", id)
		if err := os.MkdirAll(filepath.Join(cdir, "tasks"), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(cdir, "plan.md"), model.RenderChangePlan(id, title, date), 0o644); err != nil {
			t.Fatal(err)
		}
		l, _ := model.ParseChangeLedger("l", model.RenderChangeLedger(id, date))
		l.SetOverall(model.OverallInProgress, date)
		if err := os.WriteFile(filepath.Join(cdir, "ledger.md"), l.Content(), 0o644); err != nil {
			t.Fatal(err)
		}
		root := mustRead(t, filepath.Join(dir, "changes", "ledger.md"))
		row := "| [" + id + "](" + id + "/plan.md) | " + title + " | NEW | — | In progress | " + date + " | " + date + " |\n"
		if err := os.WriteFile(filepath.Join(dir, "changes", "ledger.md"), append(root, []byte(row)...), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	addChange("2026-09-11-0", "Newer change")
	addChange("2026-09-11-2", "Two")
	addChange("2026-09-11-10", "Ten") // unpadded counter must sort numerically
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
	want := []string{"2026-09-11-10", "2026-09-11-2", "2026-09-11-0", "2026-09-10-0"}
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
	if resp.Columns[0].Status != "Not started" || resp.Columns[0].Count != 1 {
		t.Fatalf("columns = %+v", resp.Columns)
	}
	if resp.Columns[3].Status != "Test" || resp.Columns[4].Status != "Done" {
		t.Fatalf("columns = %+v; want Test at index 3, Done at index 4", resp.Columns)
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

func TestMoveTaskRoute(t *testing.T) {
	st, dir := fixtureStore(t)
	h := New(st).Handler()
	w := do(t, h, "POST", "/changes/2026-09-10-0/move", `{"task":"FIX-00","status":"Done","index":0}`)
	if w.Code != 200 {
		t.Fatalf("code = %d body = %s", w.Code, w.Body)
	}
	// Verify on disk.
	l, err := model.ParseChangeLedger("l", mustRead(t, filepath.Join(dir, "changes", "2026-09-10-0", "ledger.md")))
	if err != nil {
		t.Fatal(err)
	}
	if l.Row("FIX-00").Status != model.StatusDone {
		t.Fatal("ledger not updated")
	}
}

func TestMoveTaskErrors(t *testing.T) {
	st, _ := fixtureStore(t)
	h := New(st).Handler()
	if w := do(t, h, "POST", "/changes/2026-09-10-0/move", `{"task":"FIX-99","status":"Done","index":0}`); w.Code != 404 {
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
	p := filepath.Join(dir, "changes", "2026-09-10-0", "ledger.md")
	if err := os.WriteFile(p, []byte("# broken\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	st.Reload()
	w := do(t, New(st).Handler(), "POST", "/changes/2026-09-10-0/move", `{"task":"FIX-00","status":"Done","index":0}`)
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
	var row model.TaskRow
	if err := json.Unmarshal(w.Body.Bytes(), &row); err != nil {
		t.Fatal(err)
	}
	if row.ID != "FIX-02" || row.Href != "tasks/02-third-task.md" {
		t.Fatalf("row = %+v", row)
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
