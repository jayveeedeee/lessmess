package server

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"

	"lessmess/internal/opencode"
)

// ocFake is a minimal fake opencode service that records prompts and
// creates sessions with incrementing IDs.
type ocFake struct {
	created  atomic.Int64
	prompts  chan string
	failMake bool
}

func newOCFake() *ocFake { return &ocFake{prompts: make(chan string, 16)} }

func (f *ocFake) handler() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.Method == http.MethodPost && strings.HasSuffix(r.URL.Path, "/api/session"):
			if f.failMake {
				w.WriteHeader(http.StatusBadGateway)
				return
			}
			id := f.created.Add(1)
			w.Write([]byte(`{"data":{"id":"ses_auto_` + itoa(id) + `","title":"t","location":{"directory":"/x"}}}`))
		case r.Method == http.MethodPost && strings.HasSuffix(r.URL.Path, "/prompt"):
			var body map[string]string
			json.NewDecoder(r.Body).Decode(&body)
			select {
			case f.prompts <- body["text"]:
			default:
			}
			w.Write([]byte(`{"data":{}}`))
		case r.Method == http.MethodDelete:
			w.Write([]byte(`{"data":{}}`))
		case r.Method == http.MethodGet:
			w.Write([]byte(`{"data":{"sessions":[]}}`))
		default:
			w.WriteHeader(http.StatusNotFound)
		}
	}
}

func itoa(n int64) string {
	if n == 0 {
		return "0"
	}
	var b [20]byte
	i := len(b)
	for n > 0 {
		i--
		b[i] = byte('0' + n%10)
		n /= 10
	}
	return string(b[i:])
}

func TestTaskPromptContent(t *testing.T) {
	s := mappingServer(t, nil)
	c, _ := s.st.Change("2026-09-10-0")
	n := c.Node("FIX-00")
	if n == nil {
		t.Fatal("FIX-00 node missing")
	}
	p := taskPrompt("2026-09-10-0", n)
	for _, want := range []string{
		"bound to task FIX-00 of change 2026-09-10-0",
		"tasks/00-first.md",
		"PLAN ONLY",
		"every subtask starts",
		"stays exactly in the state it was in when the plan was built",
		"Execution of any subtask begins only when the user explicitly says so",
		"NEVER set your task or its subtasks to Done",
		"PROPOSE it when work reveals complexity",
		"FIX-00.00: implement the parser",
		"NEVER create a new change directory",
	} {
		if !strings.Contains(p, want) {
			t.Errorf("task prompt missing %q", want)
		}
	}
}

func TestAutoSpawnOncePerContainer(t *testing.T) {
	fake := newOCFake()
	ts := httptest.NewServer(fake.handler())
	t.Cleanup(ts.Close)
	s := mappingServer(t, nil)
	s.SetOpencode(opencode.New(ts.URL, "pw"))

	if _, err := s.st.DecomposeTask("2026-09-10-0", "FIX-00"); err != nil {
		t.Fatal(err)
	}
	s.autospawnSweep()

	entries := s.sessions.listByTask("2026-09-10-0", "FIX-00")
	if len(entries) != 1 {
		t.Fatalf("entries = %+v, want exactly one auto-spawned session", entries)
	}
	if !strings.HasPrefix(entries[0].Title, "FIX-00: ") {
		t.Errorf("title = %q, want dotted-prefix shape", entries[0].Title)
	}
	// The prime was the task prompt.
	select {
	case p := <-fake.prompts:
		if !strings.Contains(p, "bound to task FIX-00 of change 2026-09-10-0") {
			t.Errorf("prime = %.120s", p)
		}
	default:
		t.Error("no prompt recorded")
	}

	// Sweeps are once-only: no second session even after reloads.
	s.st.Reload()
	s.autospawnSweep()
	if got := len(s.sessions.listByTask("2026-09-10-0", "FIX-00")); got != 1 {
		t.Errorf("entries after resweep = %d, want 1", got)
	}

	// Unlinking never respawns (marker persists).
	if _, err := s.sessions.remove("2026-09-10-0", entries[0].Session); err != nil {
		t.Fatal(err)
	}
	s.autospawnSweep()
	if got := len(s.sessions.listByTask("2026-09-10-0", "FIX-00")); got != 0 {
		t.Errorf("entries after unlink+resweep = %d, want 0 (no respawn)", got)
	}
}

func TestAutoSpawnServiceDownLeavesMarkerUnset(t *testing.T) {
	fake := newOCFake()
	fake.failMake = true
	ts := httptest.NewServer(fake.handler())
	t.Cleanup(ts.Close)
	s := mappingServer(t, nil)
	s.SetOpencode(opencode.New(ts.URL, "pw"))

	if _, err := s.st.DecomposeTask("2026-09-10-0", "FIX-01"); err != nil {
		t.Fatal(err)
	}
	s.autospawnSweep()
	if got := len(s.sessions.listByTask("2026-09-10-0", "FIX-01")); got != 0 {
		t.Fatalf("entries = %d, want 0 after failed spawn", got)
	}
	if s.autos.has("2026-09-10-0\x00FIX-01") {
		t.Error("marker set despite spawn failure; retry would never happen")
	}
	// Service recovers: the next sweep retries.
	fake.failMake = false
	s.autospawnSweep()
	if got := len(s.sessions.listByTask("2026-09-10-0", "FIX-01")); got != 1 {
		t.Errorf("entries after recovery = %d, want 1", got)
	}
}

func TestAutoSpawnNoopWithoutService(t *testing.T) {
	s := mappingServer(t, nil)
	if _, err := s.st.DecomposeTask("2026-09-10-0", "FIX-00"); err != nil {
		t.Fatal(err)
	}
	s.autospawnSweep() // must not panic or spawn
	if got := len(s.sessions.list("2026-09-10-0")); got != 0 {
		t.Errorf("entries = %d, want 0 without service", got)
	}
}

func TestCreateTaskSessionEndpoint(t *testing.T) {
	var prompted string
	s := mappingServer(t, func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.Method == http.MethodPost && strings.HasSuffix(r.URL.Path, "/api/session"):
			w.Write([]byte(`{"data":{"id":"ses_task","title":"t","location":{"directory":"/x"}}}`))
		case r.Method == http.MethodPost && strings.HasSuffix(r.URL.Path, "/prompt"):
			var body map[string]string
			json.NewDecoder(r.Body).Decode(&body)
			prompted = body["text"]
			w.Write([]byte(`{"data":{}}`))
		default:
			w.WriteHeader(http.StatusNotFound)
		}
	})
	w := do(t, s.Handler(), "POST", "/changes/2026-09-10-0/sessions", `{"task":"FIX-01"}`)
	if w.Code != 201 {
		t.Fatalf("code = %d body = %s", w.Code, w.Body)
	}
	var resp sessionResponse
	json.Unmarshal(w.Body.Bytes(), &resp)
	if resp.Task != "FIX-01" {
		t.Errorf("task = %q", resp.Task)
	}
	if !strings.Contains(prompted, "bound to task FIX-01 of change 2026-09-10-0") {
		t.Errorf("prompt = %.120s", prompted)
	}
	// The manual creation satisfies the auto-spawn marker.
	if !s.autos.has("2026-09-10-0\x00FIX-01") {
		t.Error("marker not set by manual task session")
	}
	// Unknown task is rejected.
	if w := do(t, s.Handler(), "POST", "/changes/2026-09-10-0/sessions", `{"task":"FIX-99"}`); w.Code != 422 {
		t.Errorf("unknown task code = %d", w.Code)
	}
}

func TestBindTaskSessionDotted(t *testing.T) {
	s := mappingServer(t, func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodGet && strings.HasPrefix(r.URL.Path, "/api/session/") {
			w.Write([]byte(`{"data":{"id":"ses_sub","title":"FIX-00.00: work","parentID":"ses_parent"}}`))
			return
		}
		w.WriteHeader(http.StatusNotFound)
	})
	if _, err := s.st.DecomposeTask("2026-09-10-0", "FIX-00"); err != nil {
		t.Fatal(err)
	}
	if _, err := s.st.CreateTask("2026-09-10-0", "FIX-00", "Child One"); err != nil {
		t.Fatal(err)
	}
	w := do(t, s.Handler(), "POST", "/changes/2026-09-10-0/task-sessions", `{"task":"FIX-00.00","sub":"ses_sub","session":"ses_parent"}`)
	if w.Code != 201 && w.Code != 200 {
		t.Fatalf("code = %d body = %s", w.Code, w.Body)
	}
	entries := s.sessions.listByTask("2026-09-10-0", "FIX-00.00")
	if len(entries) != 1 {
		t.Fatalf("entries = %+v", entries)
	}
}

func TestTaskTitleReDotted(t *testing.T) {
	for _, tc := range []struct {
		title string
		want  string
	}{
		{"FIX-00: plain", "FIX-00"},
		{"FIX-00.01: dotted", "FIX-00.01"},
		{"FIX-00.01.02: deep", "FIX-00.01.02"},
		{"FIX-00.1: single digit", "FIX-00.1"}, // regex stays permissive; unknown IDs map taskless in reconcile
		{"No prefix here", ""},
	} {
		m := taskTitleRe.FindStringSubmatch(tc.title)
		got := ""
		if m != nil {
			got = m[1]
		}
		if got != tc.want {
			t.Errorf("taskTitleRe(%q) = %q, want %q", tc.title, got, tc.want)
		}
	}
}
