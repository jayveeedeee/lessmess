package server

import (
	"encoding/json"
	"net/http"
	"strings"
	"sync"
	"testing"
)

// reprimeFixture wires a server whose fake opencode service records the
// prompt bodies sent to ses_x and serves a canned message page.
type reprimeFixture struct {
	s        *Server
	mu       sync.Mutex
	prompts  []string
	msgPages []string // responses served in order for /message; last one sticks
}

func newReprimeFixture(t *testing.T, msgs []string, task string) *reprimeFixture {
	t.Helper()
	f := &reprimeFixture{msgPages: msgs}
	f.s = mappingServer(t, func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.URL.Path == "/openapi.json":
			// The delivery path is capability-gated: teach the fake the
			// prompt-delivery contract (same shape lifecycle_test pins).
			w.Write([]byte(`{"paths":{"/api/session/{sessionID}/prompt":{"post":{"requestBody":{"content":{"application/json":{"schema":{"properties":{"id":{},"text":{},"files":{},"skills":{},"delivery":{}}}}}}}},"/api/session/{sessionID}/compact":{"post":{}},"/api/session/{sessionID}/inbox":{"get":{}}}}`))
		case r.Method == http.MethodPost && strings.HasSuffix(r.URL.Path, "/compact"):
			w.Write([]byte(`{"data":{"id":"msg_cp","sessionID":"ses_x","timeCreated":9,"type":"compaction","payload":{},"delivery":"queue"}}`))
		case r.Method == http.MethodPost && strings.HasSuffix(r.URL.Path, "/prompt"):
			var body struct {
				Text string `json:"text"`
			}
			json.NewDecoder(r.Body).Decode(&body)
			f.mu.Lock()
			f.prompts = append(f.prompts, body.Text)
			f.mu.Unlock()
			w.Write([]byte(`{"data":{"id":"msg_p","sessionID":"ses_x","role":"user","timeCreated":1}}`))
		case r.Method == http.MethodGet && strings.HasSuffix(r.URL.Path, "/message"):
			f.mu.Lock()
			page := f.msgPages[len(f.msgPages)-1]
			f.mu.Unlock()
			w.Write([]byte(page))
		case r.Method == http.MethodGet && strings.HasPrefix(r.URL.Path, "/api/session/ses_") && !strings.Contains(r.URL.Path[len("/api/session/"):], "/"):
			// confirmedIdle reads the live session for the confirmation.
			w.Write([]byte(`{"data":{"id":"ses_x","time":{"created":1,"updated":20}}}`))
		case r.Method == http.MethodGet && strings.HasSuffix(r.URL.Path, "/active"):
			w.Write([]byte(`{"data":{}}`))
		case r.Method == http.MethodGet && (strings.HasSuffix(r.URL.Path, "/permission") || strings.HasSuffix(r.URL.Path, "/form")):
			w.Write([]byte(`{"data":[]}`))
		default:
			w.WriteHeader(http.StatusNotFound)
		}
	})
	if err := f.s.sessions.add("2026-09-10-0", SessionEntry{Session: "ses_x", Title: "t", Created: "x", Task: task}); err != nil {
		t.Fatal(err)
	}
	return f
}

func (f *reprimeFixture) promptViaChat(t *testing.T, text string) string {
	t.Helper()
	w := do(t, f.s.Handler(), "POST", "/api/sessions/ses_x/chat/prompt", `{"text":"`+text+`"}`)
	if w.Code != http.StatusAccepted {
		t.Fatalf("prompt code = %d body = %s", w.Code, w.Body)
	}
	f.mu.Lock()
	defer f.mu.Unlock()
	if len(f.prompts) == 0 {
		t.Fatal("no prompt captured")
	}
	return f.prompts[len(f.prompts)-1]
}

func TestReprimeAfterQueuedCompaction(t *testing.T) {
	f := newReprimeFixture(t, nil, "FIX-00")
	// Queue a compaction through lessmess's own endpoint.
	if w := do(t, f.s.Handler(), "POST", "/api/sessions/ses_x/compact", `{"delivery":"queue","confirmation":{"sessionID":"ses_x","updated":20}}`); w.Code != http.StatusAccepted {
		t.Fatalf("compact code = %d body = %s", w.Code, w.Body)
	}
	first := f.promptViaChat(t, "continue the work")
	for _, want := range []string{
		"Context restoration",
		"bound to task FIX-00 of change 2026-09-10-0",
		"Current state (tool-injected; authoritative)",
		"FIX-01", // the fixture change's ledger content
		"lessmess-*` skills",
		"-----",
		"continue the work",
	} {
		if !strings.Contains(first, want) {
			t.Errorf("wrapped prompt missing %q:\n%s", want, first)
		}
	}
	if !strings.HasSuffix(first, "continue the work") {
		t.Errorf("user text must come last:\n%s", first)
	}
	// The marker is consumed: the second prompt is sent raw.
	second := f.promptViaChat(t, "and again")
	if second != "and again" {
		t.Errorf("second prompt = %q, want raw text", second)
	}
}

func TestReprimeAfterObservedCompaction(t *testing.T) {
	// No lessmess-queued compaction: the completed compaction message is
	// only visible in the transcript (the service-side auto path).
	msgs := `{"data":[{"id":"msg_c","type":"compaction","status":"completed","time":{"created":2}}]}`
	f := newReprimeFixture(t, []string{msgs}, "FIX-00")
	if w := do(t, f.s.Handler(), "GET", "/api/sessions/ses_x/chat", ""); w.Code != http.StatusOK {
		t.Fatalf("snapshot code = %d body = %s", w.Code, w.Body)
	}
	first := f.promptViaChat(t, "hello")
	if !strings.Contains(first, "Context restoration") || !strings.Contains(first, "hello") {
		t.Errorf("prompt not wrapped after observed compaction:\n%s", first)
	}
	second := f.promptViaChat(t, "again")
	if second != "again" {
		t.Errorf("second prompt = %q, want raw", second)
	}
}

func TestReprimeUnboundAndFailOpen(t *testing.T) {
	// Unbound session (no mapping entry): queued compaction, but no wrap.
	f := newReprimeFixture(t, nil, "")
	if w := do(t, f.s.Handler(), "POST", "/api/sessions/ses_free/compact", `{"delivery":"queue","confirmation":{"sessionID":"ses_free","updated":20}}`); w.Code != http.StatusAccepted {
		t.Fatalf("compact code = %d", w.Code)
	}
	w := do(t, f.s.Handler(), "POST", "/api/sessions/ses_free/chat/prompt", `{"text":"raw please"}`)
	if w.Code != http.StatusAccepted {
		t.Fatalf("prompt code = %d", w.Code)
	}
	f.mu.Lock()
	got := f.prompts[len(f.prompts)-1]
	f.mu.Unlock()
	if got != "raw please" {
		t.Errorf("unbound prompt = %q, want raw", got)
	}
	// Bound to a nonexistent change: ledger read fails, fail open.
	broken := newReprimeFixture(t, nil, "FIX-00")
	_ = broken.s.sessions.add("2026-01-01-zzzzz", SessionEntry{Session: "ses_x", Title: "t", Created: "x"})
	_, _ = broken.s.sessions.remove("2026-09-10-0", "ses_x")
	if w := do(t, broken.s.Handler(), "POST", "/api/sessions/ses_x/compact", `{"delivery":"queue","confirmation":{"sessionID":"ses_x","updated":20}}`); w.Code != http.StatusAccepted {
		t.Fatalf("compact code = %d", w.Code)
	}
	if got := broken.promptViaChat(t, "still raw"); got != "still raw" {
		t.Errorf("fail-open prompt = %q, want raw", got)
	}
}

func TestSessionDeliverReprime(t *testing.T) {
	// The queued-delivery path (POST /deliver) must wrap exactly like
	// the chat prompt path — both post to the service's prompt endpoint.
	f := newReprimeFixture(t, nil, "FIX-00")
	if w := do(t, f.s.Handler(), "POST", "/api/sessions/ses_x/compact", `{"delivery":"queue","confirmation":{"sessionID":"ses_x","updated":20}}`); w.Code != http.StatusAccepted {
		t.Fatalf("compact code = %d", w.Code)
	}
	w := do(t, f.s.Handler(), "POST", "/api/sessions/ses_x/deliver", `{"id":"msg_q","text":"deliver probe","delivery":"queue"}`)
	if w.Code != http.StatusAccepted {
		t.Fatalf("deliver code = %d body = %s", w.Code, w.Body)
	}
	f.mu.Lock()
	got := f.prompts[len(f.prompts)-1]
	f.mu.Unlock()
	if !strings.Contains(got, "Context restoration") || !strings.Contains(got, "bound to task FIX-00 of change 2026-09-10-0") || !strings.HasSuffix(got, "deliver probe") {
		t.Errorf("deliver path prompt = %q", got)
	}
}
