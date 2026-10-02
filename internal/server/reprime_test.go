package server

import (
	"encoding/json"
	"net/http"
	"strings"
	"sync"
	"testing"
)

// Transcript pages for the rule. The rule reads one descending page at
// prompt time, so each fixture page models the tail the evaluator sees:
// a completed compaction as the newest event, the same tail after a
// wrapped prompt lands (a user message strictly newer than the
// compaction), a still-continued conversation, a failed compaction, and
// a tie.
var (
	reprimeCompacted = `{"data":[{"id":"msg_c","type":"compaction","status":"completed","time":{"created":2}}]}`
	reprimeContinued = `{"data":[{"id":"msg_u","type":"user","time":{"created":6}},{"id":"msg_c","type":"compaction","status":"completed","time":{"created":2}}]}`
	reprimeStale     = `{"data":[{"id":"msg_u","type":"user","time":{"created":9}},{"id":"msg_a","type":"assistant","time":{"created":7}},{"id":"msg_c","type":"compaction","status":"completed","time":{"created":2}}]}`
	reprimeFailed    = `{"data":[{"id":"msg_cf","type":"compaction","status":"failed","time":{"created":8}},{"id":"msg_u","type":"user","time":{"created":5}}]}`
	reprimeTie       = `{"data":[{"id":"msg_c","type":"compaction","status":"completed","time":{"created":2}},{"id":"msg_u","type":"user","time":{"created":2}}]}`
	reprimeEmpty     = `{"data":[]}`
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
			var page string
			if len(f.msgPages) > 0 {
				page = f.msgPages[0]
				if len(f.msgPages) > 1 {
					f.msgPages = f.msgPages[1:]
				}
			}
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

// assertWrapped pins the shape of a re-primed prompt: the preamble and
// ledger snapshot come first, the user's text last.
func assertWrapped(t *testing.T, got, text string) {
	t.Helper()
	for _, want := range []string{
		"Context restoration",
		"bound to task FIX-00 of change 2026-09-10-0",
		"Current state (tool-injected; authoritative)",
		"FIX-01", // the fixture change's ledger content
		"lessmess-*` skills",
		"-----",
		text,
	} {
		if !strings.Contains(got, want) {
			t.Errorf("wrapped prompt missing %q:\n%s", want, got)
		}
	}
	if !strings.HasSuffix(got, text) {
		t.Errorf("user text must come last:\n%s", got)
	}
}

func TestReprimeAfterQueuedCompaction(t *testing.T) {
	// The queued compaction surfaces as a completed compaction message
	// at the transcript tail; the rule reads it at prompt time.
	f := newReprimeFixture(t, []string{reprimeCompacted, reprimeContinued}, "FIX-00")
	// Queue a compaction through lessmess's own endpoint.
	if w := do(t, f.s.Handler(), "POST", "/api/sessions/ses_x/compact", `{"delivery":"queue","confirmation":{"sessionID":"ses_x","updated":20}}`); w.Code != http.StatusAccepted {
		t.Fatalf("compact code = %d body = %s", w.Code, w.Body)
	}
	assertWrapped(t, f.promptViaChat(t, "continue the work"), "continue the work")
	// The wrapped prompt is itself a user message; the next read sees
	// the conversation continued and the second prompt is sent raw.
	if got := f.promptViaChat(t, "and again"); got != "and again" {
		t.Errorf("second prompt = %q, want raw text", got)
	}
}

func TestReprimeAfterObservedCompaction(t *testing.T) {
	// No lessmess-queued compaction: the completed compaction message is
	// only visible in the transcript (the service-side auto path). The
	// prompt-time transcript read sees it without any snapshot walk.
	f := newReprimeFixture(t, []string{reprimeCompacted, reprimeContinued}, "FIX-00")
	assertWrapped(t, f.promptViaChat(t, "hello"), "hello")
	if got := f.promptViaChat(t, "again"); got != "again" {
		t.Errorf("second prompt = %q, want raw", got)
	}
}

func TestReprimeStaleCompactionNotRewrapped(t *testing.T) {
	// Restart amnesia regression: a session whose newest user message
	// postdates its newest completed compaction must never be wrapped,
	// no matter that a server restart emptied any in-memory marker.
	f := newReprimeFixture(t, []string{reprimeStale}, "FIX-00")
	if got := f.promptViaChat(t, "just talk"); got != "just talk" {
		t.Errorf("stale-compaction prompt = %q, want raw", got)
	}
}

func TestReprimeFailedCompactionIgnored(t *testing.T) {
	// A failed compaction never counts: the context was not compacted,
	// so the next prompt is sent unwrapped.
	f := newReprimeFixture(t, []string{reprimeFailed}, "FIX-00")
	if got := f.promptViaChat(t, "carry on"); got != "carry on" {
		t.Errorf("failed-compaction prompt = %q, want raw", got)
	}
}

func TestReprimeRacingCompactionWrapsOnce(t *testing.T) {
	// Double-fire regression: a prompt sent before a compaction
	// completes goes unwrapped; the first prompt after the completion
	// carries the single re-prime; every prompt after that is raw.
	f := newReprimeFixture(t, []string{`{"data":[{"id":"msg_u","type":"user","time":{"created":1}}]}`, reprimeCompacted, reprimeContinued}, "FIX-00")
	if got := f.promptViaChat(t, "before compact"); got != "before compact" {
		t.Errorf("in-flight prompt = %q, want raw", got)
	}
	assertWrapped(t, f.promptViaChat(t, "after compact"), "after compact")
	if got := f.promptViaChat(t, "after wrap"); got != "after wrap" {
		t.Errorf("post-wrap prompt = %q, want raw", got)
	}
}

func TestReprimeTieCountsContinued(t *testing.T) {
	// Equal timestamps count as already continued — a missed re-prime
	// is safer than a spurious one.
	f := newReprimeFixture(t, []string{reprimeTie}, "FIX-00")
	if got := f.promptViaChat(t, "tied"); got != "tied" {
		t.Errorf("tie prompt = %q, want raw", got)
	}
}

func TestReprimeEmptyAndUnreadableTranscript(t *testing.T) {
	// An empty transcript has nothing to compensate; an unreadable one
	// fails open. Both send the user's text unwrapped.
	empty := newReprimeFixture(t, []string{reprimeEmpty}, "FIX-00")
	if got := empty.promptViaChat(t, "empty"); got != "empty" {
		t.Errorf("empty-transcript prompt = %q, want raw", got)
	}
	unreadable := newReprimeFixture(t, nil, "FIX-00")
	if got := unreadable.promptViaChat(t, "opaque"); got != "opaque" {
		t.Errorf("unreadable-transcript prompt = %q, want raw", got)
	}
}

func TestReprimeUnboundAndFailOpen(t *testing.T) {
	// Unbound session (no mapping entry): completed compaction in the
	// transcript, but no wrap — a free chat has no binding to restore.
	f := newReprimeFixture(t, []string{reprimeCompacted}, "")
	f.s.sessions.remove("2026-09-10-0", "ses_x")
	w := do(t, f.s.Handler(), "POST", "/api/sessions/ses_x/chat/prompt", `{"text":"raw please"}`)
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
	broken := newReprimeFixture(t, []string{reprimeCompacted}, "FIX-00")
	_ = broken.s.sessions.add("2026-01-01-zzzzz", SessionEntry{Session: "ses_x", Title: "t", Created: "x"})
	_, _ = broken.s.sessions.remove("2026-09-10-0", "ses_x")
	if got := broken.promptViaChat(t, "still raw"); got != "still raw" {
		t.Errorf("fail-open prompt = %q, want raw", got)
	}
}

func TestSessionDeliverReprime(t *testing.T) {
	// The queued-delivery path (POST /deliver) must wrap exactly like
	// the chat prompt path — both post to the service's prompt endpoint.
	f := newReprimeFixture(t, []string{reprimeCompacted}, "FIX-00")
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
