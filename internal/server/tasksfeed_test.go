package server

import (
	"encoding/json"
	"net/http"
	"testing"
)

// TestTasksFeedRoot pins the change-root feed the chat Work panel renders.
func TestTasksFeedRoot(t *testing.T) {
	s := mappingServer(t, nil)
	h := s.Handler()

	req, _ := http.NewRequest("GET", "/changes/2026-09-10-0/tasks", nil)
	req.Header.Set("Accept", "application/json")
	w := doReq(t, h, req)
	if w.Code != 200 {
		t.Fatalf("code = %d body = %s", w.Code, w.Body)
	}
	var feed tasksFeed
	if err := json.Unmarshal(w.Body.Bytes(), &feed); err != nil {
		t.Fatal(err)
	}
	if feed.Change != "2026-09-10-0" || feed.Title != "Fixture change" {
		t.Errorf("feed change/title = %q/%q", feed.Change, feed.Title)
	}
	if feed.Overall != "In progress" {
		t.Errorf("overall = %q", feed.Overall)
	}
	if len(feed.Tasks) != 2 {
		t.Fatalf("tasks = %+v", feed.Tasks)
	}
	first := feed.Tasks[0]
	if first.ID != "FIX-00" || first.Title != "First" || first.Status != "Test" {
		t.Errorf("first row = %+v", first)
	}
	if first.Path != "00-first.md" || first.Href != "/changes/2026-09-10-0/tasks/00-first.md" {
		t.Errorf("first href shape = %+v", first)
	}
	if first.HasSub || first.SubDone != 0 || first.SubTotal != 0 {
		t.Errorf("first rollup = %+v", first)
	}
	if feed.Tasks[0].Updated != "2026-09-10" || feed.Tasks[1].ID != "FIX-01" {
		t.Errorf("rows out of order or unupdated = %+v", feed.Tasks)
	}
}

// TestTasksFeedScope pins the container scope: only that container's
// children, with the scope title and per-row rollups.
func TestTasksFeedScope(t *testing.T) {
	s := nestedBoardServer(t)
	h := s.Handler()

	req, _ := http.NewRequest("GET", "/changes/2026-09-10-0/tasks?task=FIX-00", nil)
	req.Header.Set("Accept", "application/json")
	w := doReq(t, h, req)
	if w.Code != 200 {
		t.Fatalf("code = %d body = %s", w.Code, w.Body)
	}
	var feed tasksFeed
	if err := json.Unmarshal(w.Body.Bytes(), &feed); err != nil {
		t.Fatal(err)
	}
	if feed.Scope != "FIX-00" || feed.ScopeTitle == "" {
		t.Errorf("scope = %q/%q", feed.Scope, feed.ScopeTitle)
	}
	if len(feed.Tasks) != 2 {
		t.Fatalf("tasks = %+v", feed.Tasks)
	}
	if feed.Tasks[0].ID != "FIX-00.00" || feed.Tasks[0].Status != "Test" {
		t.Errorf("first child = %+v", feed.Tasks[0])
	}
	if feed.Tasks[0].Path != "00-first/tasks/00-child-one.md" {
		t.Errorf("nested path = %q", feed.Tasks[0].Path)
	}
	for _, row := range feed.Tasks {
		if row.HasSub {
			t.Errorf("leaf child carries rollup: %+v", row)
		}
	}

	// The decomposed parent itself reports its rollup on the root feed.
	req2, _ := http.NewRequest("GET", "/changes/2026-09-10-0/tasks", nil)
	req2.Header.Set("Accept", "application/json")
	w2 := doReq(t, h, req2)
	var root tasksFeed
	json.Unmarshal(w2.Body.Bytes(), &root)
	for _, row := range root.Tasks {
		if row.ID != "FIX-00" {
			continue
		}
		if !row.HasSub || row.SubTotal != 2 || row.SubDone != 1 {
			t.Errorf("parent rollup = %+v", row)
		}
	}
}

// TestTasksFeedErrors: unknown change and unknown scope both 404.
func TestTasksFeedErrors(t *testing.T) {
	s := mappingServer(t, nil)
	h := s.Handler()

	req, _ := http.NewRequest("GET", "/changes/nope/tasks", nil)
	req.Header.Set("Accept", "application/json")
	if w := doReq(t, h, req); w.Code != 404 {
		t.Errorf("unknown change code = %d", w.Code)
	}

	req2, _ := http.NewRequest("GET", "/changes/2026-09-10-0/tasks?task=NOPE", nil)
	req2.Header.Set("Accept", "application/json")
	if w := doReq(t, h, req2); w.Code != 404 {
		t.Errorf("unknown scope code = %d", w.Code)
	}
}
