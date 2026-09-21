package server

import (
	"encoding/json"
	"net/http"
	"strings"
	"testing"
)

func TestInstructionManifestValid(t *testing.T) {
	// Every module: id, text, audiences present; no unknown conditions.
	for _, m := range instructions {
		if m.ID == "" || m.Text == "" || len(m.Audiences) == 0 {
			t.Errorf("module %+v missing id/text/audiences", m)
		}
		switch m.When {
		case "", "worktree":
		default:
			t.Errorf("module %s: unknown condition %q", m.ID, m.When)
		}
	}
	// Self-containment: no cross-module references ("see module X").
	for _, m := range instructions {
		if strings.Contains(m.Text, "see module") || strings.Contains(m.Text, "see the ") && strings.Contains(m.Text, "module") {
			t.Errorf("module %s references another module; modules must stand alone", m.ID)
		}
	}
}

func TestSelectionDeterministic(t *testing.T) {
	base := primeContext{APIBase: "http://x", ChangeID: "c", SessionID: "s"}
	cases := []struct {
		audience string
		pc       primeContext
		want     string
	}{
		{"discussion", base, "discussion"},
		{"chat", primeContext{}, "chat"},
		{"change", base, "change.session,change.handoff,closeout"},
		{"task", base, "task.session,closeout"},
		{"change", primeContext{APIBase: "http://x", Worktree: "/wt", WorktreeBranch: "b"}, "change.session,change.handoff,worktree,closeout"},
		{"task", primeContext{APIBase: "http://x", Worktree: "/wt", WorktreeBranch: "b"}, "worktree,task.session,closeout"},
	}
	for _, tc := range cases {
		var ids []string
		for _, m := range selectModules(tc.audience, tc.pc) {
			ids = append(ids, m.ID)
		}
		if got := strings.Join(ids, ","); got != tc.want {
			t.Errorf("select(%s, worktree=%q) = %s, want %s", tc.audience, tc.pc.Worktree, got, tc.want)
		}
		// Selection is a pure function: same inputs, same output.
		again := selectModules(tc.audience, tc.pc)
		if len(again) != len(ids) {
			t.Errorf("selection unstable for %s", tc.audience)
		}
	}
}

func TestRenderPrimePlaceholdersAndSnapshot(t *testing.T) {
	pc := primeContext{
		APIBase: "http://127.0.0.1:9090", ChangeID: "2026-09-18-15tbl", SessionID: "ses_x",
		TaskID: "JSI-04", TaskHref: "tasks/04-instruction-injection.md",
		Snapshot: "| Task | Title | Status |\n| --- | --- | --- |\n| [JSI-04](tasks/04.md) | Injection | In progress |",
	}
	text, ids := renderPrime("task", pc)
	if strings.Contains(text, "{{") {
		t.Errorf("unsubstituted placeholder in prime:\n%s", text)
	}
	for _, want := range []string{
		"http://127.0.0.1:9090/changes/2026-09-18-15tbl/tasks",
		"JSI-04.00: implement the parser",
		"tasks/04-instruction-injection.md",
		"Current state (tool-injected; authoritative)",
		"[JSI-04](tasks/04.md)",
	} {
		if !strings.Contains(text, want) {
			t.Errorf("prime missing %q", want)
		}
	}
	if len(ids) == 0 {
		t.Error("no module ids returned for audit")
	}
}

func TestDiscussionPrimeTransitionsToExecution(t *testing.T) {
	text, ids := renderPrime("discussion", primeContext{
		APIBase:   "http://127.0.0.1:9090",
		SessionID: "ses_x",
	})
	if len(ids) != 1 || ids[0] != "discussion" {
		t.Fatalf("module ids = %v", ids)
	}
	for _, want := range []string{
		"Before scaffolding, do not modify the repository",
		"continue in this same session",
		"In progress to Test",
		"never mark tasks Done",
	} {
		if !strings.Contains(text, want) {
			t.Errorf("discussion prime missing %q", want)
		}
	}
	if strings.Contains(text, "Discussion only.") {
		t.Error("discussion prime still imposes a permanent discussion-only restriction")
	}
}

func TestInstructionsEndpoint(t *testing.T) {
	s := mappingServer(t, nil)
	w := do(t, s.Handler(), "GET", "/workflow/instructions", "")
	if w.Code != http.StatusOK {
		t.Fatalf("code = %d", w.Code)
	}
	var resp struct {
		Version int `json:"version"`
		Modules []struct {
			ID        string   `json:"id"`
			Version   int      `json:"version"`
			Audiences []string `json:"audiences"`
		} `json:"modules"`
		Selection map[string][]string `json:"selection"`
	}
	if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
		t.Fatal(err)
	}
	if len(resp.Modules) == 0 {
		t.Fatal("empty manifest")
	}
	// The discussion audience selects exactly the discussion module.
	if sel := resp.Selection["discussion"]; len(sel) != 1 || sel[0] != "discussion" {
		t.Errorf("discussion selection = %v", sel)
	}
}
