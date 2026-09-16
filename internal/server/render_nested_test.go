package server

import (
	"net/http"
	"strings"
	"testing"
)

func TestNestedBoardMarkup(t *testing.T) {
	s := nestedBoardServer(t)
	h := s.Handler()

	// Root board: expand affordance on plain tasks, rollup badge on the
	// decomposed one, and change-level progress.
	w := htmlGet(t, h, "/changes/2026-09-10-0", false)
	body := w.Body.String()
	for _, want := range []string{
		`data-expand="FIX-01"`,
		`class="card-sub"`,
		`1/2 ✓`,
		`class="pill progress"`,
		`New task title`,
	} {
		if !strings.Contains(body, want) {
			t.Errorf("root board missing %q", want)
		}
	}

	// Drill-down: breadcrumb, parent form field, scoped board marker.
	w = htmlGet(t, h, "/changes/2026-09-10-0?task=FIX-00", false)
	body = w.Body.String()
	for _, want := range []string{
		`class="crumbs"`,
		`href="/changes/2026-09-10-0"`,
		`name="parent" value="FIX-00"`,
		`New subtask title`,
		`data-task="FIX-00"`,
	} {
		if !strings.Contains(body, want) {
			t.Errorf("drill-down board missing %q", want)
		}
	}
}

func TestNestedTaskDetailDocContext(t *testing.T) {
	s := nestedBoardServer(t)
	w := htmlGet(t, s.Handler(), "/changes/2026-09-10-0/tasks/00-first/tasks/00-child-one.md", false)
	if w.Code != 200 {
		t.Fatalf("code = %d", w.Code)
	}
	if !strings.Contains(w.Body.String(), `data-doc="tasks/00-first/tasks/00-child-one.md"`) {
		t.Error("task detail missing data-doc context for link resolution")
	}
}

func TestContainerLedgerEndpoint(t *testing.T) {
	s := nestedBoardServer(t)
	h := s.Handler()

	w := htmlGet(t, h, "/changes/2026-09-10-0/ledger?href=tasks/00-first/ledger.md", false)
	if w.Code != 200 {
		t.Fatalf("code = %d body = %s", w.Code, w.Body)
	}
	body := w.Body.String()
	if !strings.Contains(body, "Task: FIX-00") && !strings.Contains(body, "FIX-00") {
		t.Error("container ledger content missing")
	}
	if !strings.Contains(body, `data-doc="tasks/00-first/ledger.md"`) {
		t.Error("container ledger missing data-doc context")
	}

	// Path guard: escaping or non-container paths are refused.
	for _, href := range []string{"../ledger.md", "plan.md", "tasks/00-child-one.md", "tasks/ledger.md"} {
		req, _ := http.NewRequest("GET", "/changes/2026-09-10-0/ledger?href="+href, nil)
		req.Header.Set("Accept", "text/html")
		if resp := doReq(t, h, req); resp.Code != 404 {
			t.Errorf("href=%q code = %d, want 404", href, resp.Code)
		}
	}
}
