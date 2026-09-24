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
	// decomposed one. No header progress pill, no status select.
	w := htmlGet(t, h, "/changes/2026-09-10-0", false)
	body := w.Body.String()
	for _, want := range []string{
		`data-expand="FIX-01"`,
		`class="card-sub"`,
		`1/2 ✓`,
	} {
		if !strings.Contains(body, want) {
			t.Errorf("root board missing %q", want)
		}
	}
	if strings.Contains(body, `pill progress`) || strings.Contains(body, `id="overall-status"`) {
		t.Error("root board must not render the header progress pill or a status select")
	}
	parentDetail := htmlGet(t, h, "/changes/2026-09-10-0/tasks/00-first.md", false).Body.String()
	for _, want := range []string{`data-open-subtasks`, `data-task="FIX-00"`, `View subtasks`, `1/2`} {
		if !strings.Contains(parentDetail, want) {
			t.Errorf("decomposed task detail missing %q", want)
		}
	}

	// Drill-down: breadcrumb and scoped board marker.
	w = htmlGet(t, h, "/changes/2026-09-10-0?task=FIX-00", false)
	body = w.Body.String()
	if !strings.Contains(body, `>Fixture change</a>`) {
		t.Fatalf("task-board root breadcrumb did not use the change title: %s", body)
	}
	for _, want := range []string{
		`class="crumbs"`,
		`href="/changes/2026-09-10-0"`,
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
