package server

import (
	"encoding/json"
	"net/http"
	"strings"
	"sync"
	"testing"

	"lessmess/internal/opencode"
)

// Transcript pages for the rule. The rule reads one descending page at
// prompt time, so each fixture page models the tail the evaluator sees:
// a completed compaction as the newest event, the same tail after the
// restoration synthetic is delivered, a still-continued conversation, a
// failed compaction, and a tie.
var (
	reprimeCompacted = `{"data":[{"id":"msg_c","type":"compaction","status":"completed","time":{"created":2}}]}`
	reprimeDelivered = `{"data":[{"id":"msg_s","type":"synthetic","text":"Context restoration: the conversation above was compacted.","time":{"created":3}},{"id":"msg_c","type":"compaction","status":"completed","time":{"created":2}}]}`
	reprimeContinued = `{"data":[{"id":"msg_u","type":"user","time":{"created":6}},{"id":"msg_c","type":"compaction","status":"completed","time":{"created":2}}]}`
	reprimeStale     = `{"data":[{"id":"msg_u","type":"user","time":{"created":9}},{"id":"msg_a","type":"assistant","time":{"created":7}},{"id":"msg_c","type":"compaction","status":"completed","time":{"created":2}}]}`
	reprimeFailed    = `{"data":[{"id":"msg_cf","type":"compaction","status":"failed","time":{"created":8}},{"id":"msg_u","type":"user","time":{"created":5}}]}`
	reprimeTie       = `{"data":[{"id":"msg_c","type":"compaction","status":"completed","time":{"created":2}},{"id":"msg_u","type":"user","time":{"created":2}}]}`
	reprimeEmpty     = `{"data":[]}`
)

// reprimeFixture wires a server whose fake opencode service records the
// prompt bodies and synthetic injections sent to ses_x, serves a canned
// message page and inbox, and can be taught to fail the synthetic
// endpoint.
type reprimeFixture struct {
	s  *Server
	mu struct {
		sync.Mutex
		prompts            []string
		inboxUpdates       int
		syntheticsAtUpdate int
		synthetics         []string
		msgPages           []string // responses served in order for /message; last one sticks
		inbox              string   // response for GET /inbox (default empty)
		synthCode          int      // non-zero: /synthetic responds with this status
	}
	synthCapable bool
}

func newReprimeFixture(t *testing.T, msgs []string, task string) *reprimeFixture {
	t.Helper()
	f := &reprimeFixture{synthCapable: true}
	f.mu.msgPages = msgs
	f.mu.inbox = `{"data":[]}`
	f.s = mappingServer(t, func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.URL.Path == "/openapi.json":
			// The injection path is capability-gated: teach the fake the
			// prompt-delivery and synthetic contracts.
			synthetic := `,"/api/session/{sessionID}/synthetic":{"post":{}}`
			if !f.synthCapable {
				synthetic = ""
			}
			w.Write([]byte(`{"paths":{"/api/session/{sessionID}/prompt":{"post":{"requestBody":{"content":{"application/json":{"schema":{"properties":{"id":{},"text":{},"files":{},"skills":{},"delivery":{}}}}}}}},"/api/session/{sessionID}/compact":{"post":{}},"/api/session/{sessionID}/inbox":{"get":{}},"/api/session/{sessionID}/inbox/{inboxID}":{"patch":{}}` + synthetic + `}}`))
		case r.Method == http.MethodPatch && r.URL.Path == "/api/session/ses_x/inbox/msg_q":
			f.mu.Lock()
			f.mu.inboxUpdates++
			f.mu.syntheticsAtUpdate = len(f.mu.synthetics)
			f.mu.Unlock()
			w.WriteHeader(http.StatusNoContent)
		case r.Method == http.MethodPost && strings.HasSuffix(r.URL.Path, "/compact"):
			w.Write([]byte(`{"data":{"id":"msg_cp","sessionID":"ses_x","timeCreated":9,"type":"compaction","payload":{},"delivery":"queue"}}`))
		case r.Method == http.MethodPost && strings.HasSuffix(r.URL.Path, "/synthetic"):
			f.mu.Lock()
			code := f.mu.synthCode
			f.mu.Unlock()
			if code != 0 {
				w.WriteHeader(code)
				return
			}
			var body struct {
				Text   string `json:"text"`
				Resume *bool  `json:"resume"`
			}
			json.NewDecoder(r.Body).Decode(&body)
			f.mu.Lock()
			f.mu.synthetics = append(f.mu.synthetics, body.Text)
			f.mu.Unlock()
			w.Write([]byte(`{"data":{"id":"msg_sy","sessionID":"ses_x","timeCreated":9,"type":"synthetic","payload":{"text":"x"},"delivery":"steer"}}`))
		case r.Method == http.MethodPost && strings.HasSuffix(r.URL.Path, "/prompt"):
			var body struct {
				Text string `json:"text"`
			}
			json.NewDecoder(r.Body).Decode(&body)
			f.mu.Lock()
			f.mu.prompts = append(f.mu.prompts, body.Text)
			f.mu.Unlock()
			w.Write([]byte(`{"data":{"id":"msg_p","sessionID":"ses_x","role":"user","timeCreated":1}}`))
		case r.Method == http.MethodGet && strings.HasSuffix(r.URL.Path, "/message"):
			f.mu.Lock()
			var page string
			if len(f.mu.msgPages) > 0 {
				page = f.mu.msgPages[0]
				if len(f.mu.msgPages) > 1 {
					f.mu.msgPages = f.mu.msgPages[1:]
				}
			}
			f.mu.Unlock()
			w.Write([]byte(page))
		case r.Method == http.MethodGet && strings.HasSuffix(r.URL.Path, "/inbox"):
			f.mu.Lock()
			inbox := f.mu.inbox
			f.mu.Unlock()
			w.Write([]byte(inbox))
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
	return f.lastPrompt(t)
}

func (f *reprimeFixture) lastPrompt(t *testing.T) string {
	t.Helper()
	f.mu.Lock()
	defer f.mu.Unlock()
	if len(f.mu.prompts) == 0 {
		t.Fatal("no prompt captured")
	}
	return f.mu.prompts[len(f.mu.prompts)-1]
}

func (f *reprimeFixture) syntheticCount() int {
	f.mu.Lock()
	defer f.mu.Unlock()
	return len(f.mu.synthetics)
}

func (f *reprimeFixture) lastSynthetic() string {
	f.mu.Lock()
	defer f.mu.Unlock()
	if len(f.mu.synthetics) == 0 {
		return ""
	}
	return f.mu.synthetics[len(f.mu.synthetics)-1]
}

// assertRestoration pins the shape of the injected synthetic: marker,
// binding, ledger snapshot, skills pointer.
func assertRestoration(t *testing.T, got string) {
	t.Helper()
	for _, want := range []string{
		"Context restoration:",
		"bound to task FIX-00 of change 2026-09-10-0",
		"Current state (tool-injected; authoritative)",
		"FIX-01", // the fixture change's ledger content
		"lessmess-*` skills",
	} {
		if !strings.Contains(got, want) {
			t.Errorf("restoration synthetic missing %q:\n%s", want, got)
		}
	}
}

func TestReprimeInjectsSyntheticAfterCompaction(t *testing.T) {
	// The completed compaction is the newest event; the prompt goes
	// through verbatim and exactly one restoration synthetic is injected.
	f := newReprimeFixture(t, []string{reprimeCompacted, reprimeDelivered}, "FIX-00")
	if got := f.promptViaChat(t, "continue the work"); got != "continue the work" {
		t.Errorf("prompt = %q, want verbatim text", got)
	}
	if n := f.syntheticCount(); n != 1 {
		t.Fatalf("synthetic count = %d, want 1", n)
	}
	assertRestoration(t, f.lastSynthetic())
	// The delivered synthetic compensates: the second prompt is verbatim
	// with no further injection.
	if got := f.promptViaChat(t, "and again"); got != "and again" {
		t.Errorf("second prompt = %q, want verbatim", got)
	}
	if n := f.syntheticCount(); n != 1 {
		t.Errorf("synthetic count after second prompt = %d, want 1", n)
	}
}

func TestReprimePendingSyntheticSuppressesInjection(t *testing.T) {
	// The queue-blindness regression: a restoration already pending in
	// the inbox compensates — repeated sends or queued follow-ups must
	// not inject again while it waits for delivery.
	f := newReprimeFixture(t, []string{reprimeCompacted}, "FIX-00")
	f.mu.Lock()
	f.mu.inbox = `{"data":[{"id":"msg_sy","sessionID":"ses_x","timeCreated":9,"type":"synthetic","payload":{"text":"Context restoration: pending"},"delivery":"steer"}]}`
	f.mu.Unlock()
	if got := f.promptViaChat(t, "queued twice"); got != "queued twice" {
		t.Errorf("prompt = %q, want verbatim", got)
	}
	if got := f.promptViaChat(t, "queued thrice"); got != "queued thrice" {
		t.Errorf("second prompt = %q, want verbatim", got)
	}
	if n := f.syntheticCount(); n != 0 {
		t.Errorf("synthetic count = %d, want 0 (pending item compensates)", n)
	}
}

func TestReprimeStaleCompactionNotInjected(t *testing.T) {
	// Restart amnesia regression: a session whose newest user message
	// postdates its newest completed compaction is never compensated.
	f := newReprimeFixture(t, []string{reprimeStale}, "FIX-00")
	if got := f.promptViaChat(t, "just talk"); got != "just talk" {
		t.Errorf("stale-compaction prompt = %q, want verbatim", got)
	}
	if n := f.syntheticCount(); n != 0 {
		t.Errorf("synthetic count = %d, want 0", n)
	}
}

func TestReprimeFailedCompactionIgnored(t *testing.T) {
	f := newReprimeFixture(t, []string{reprimeFailed}, "FIX-00")
	if got := f.promptViaChat(t, "carry on"); got != "carry on" {
		t.Errorf("failed-compaction prompt = %q, want verbatim", got)
	}
	if n := f.syntheticCount(); n != 0 {
		t.Errorf("synthetic count = %d, want 0", n)
	}
}

func TestReprimeRacingCompactionInjectsOnce(t *testing.T) {
	// A prompt sent before a compaction completes injects nothing; the
	// first prompt after the completion carries the single injection.
	f := newReprimeFixture(t, []string{`{"data":[{"id":"msg_u","type":"user","time":{"created":1}}]}`, reprimeCompacted, reprimeDelivered}, "FIX-00")
	if got := f.promptViaChat(t, "before compact"); got != "before compact" {
		t.Errorf("in-flight prompt = %q, want verbatim", got)
	}
	if n := f.syntheticCount(); n != 0 {
		t.Errorf("pre-completion synthetic count = %d, want 0", n)
	}
	if got := f.promptViaChat(t, "after compact"); got != "after compact" {
		t.Errorf("post-completion prompt = %q, want verbatim", got)
	}
	if n := f.syntheticCount(); n != 1 {
		t.Fatalf("synthetic count = %d, want 1", n)
	}
	if got := f.promptViaChat(t, "after inject"); got != "after inject" {
		t.Errorf("post-inject prompt = %q, want verbatim", got)
	}
	if n := f.syntheticCount(); n != 1 {
		t.Errorf("synthetic count = %d, want 1", n)
	}
}

func TestReprimeTieCountsContinued(t *testing.T) {
	f := newReprimeFixture(t, []string{reprimeTie}, "FIX-00")
	if got := f.promptViaChat(t, "tied"); got != "tied" {
		t.Errorf("tie prompt = %q, want verbatim", got)
	}
	if n := f.syntheticCount(); n != 0 {
		t.Errorf("synthetic count = %d, want 0", n)
	}
}

func TestReprimeEmptyAndUnreadableTranscript(t *testing.T) {
	empty := newReprimeFixture(t, []string{reprimeEmpty}, "FIX-00")
	if got := empty.promptViaChat(t, "empty"); got != "empty" {
		t.Errorf("empty-transcript prompt = %q, want verbatim", got)
	}
	unreadable := newReprimeFixture(t, nil, "FIX-00")
	if got := unreadable.promptViaChat(t, "opaque"); got != "opaque" {
		t.Errorf("unreadable-transcript prompt = %q, want verbatim", got)
	}
	if n := unreadable.syntheticCount(); n != 0 {
		t.Errorf("synthetic count = %d, want 0", n)
	}
}

func TestReprimeUnboundAndFailOpen(t *testing.T) {
	// Unbound session (no mapping entry): completed compaction in the
	// transcript, but no injection — a free chat has no binding.
	f := newReprimeFixture(t, []string{reprimeCompacted}, "")
	f.s.sessions.remove("2026-09-10-0", "ses_x")
	if got := f.promptViaChat(t, "raw please"); got != "raw please" {
		t.Errorf("unbound prompt = %q, want verbatim", got)
	}
	if n := f.syntheticCount(); n != 0 {
		t.Errorf("unbound synthetic count = %d, want 0", n)
	}
	// Bound to a nonexistent change: ledger read fails, fail open with
	// no injection.
	broken := newReprimeFixture(t, []string{reprimeCompacted}, "FIX-00")
	_ = broken.s.sessions.add("2026-01-01-zzzzz", SessionEntry{Session: "ses_x", Title: "t", Created: "x"})
	_, _ = broken.s.sessions.remove("2026-09-10-0", "ses_x")
	if got := broken.promptViaChat(t, "still raw"); got != "still raw" {
		t.Errorf("fail-open prompt = %q, want verbatim", got)
	}
	if n := broken.syntheticCount(); n != 0 {
		t.Errorf("fail-open synthetic count = %d, want 0", n)
	}
}

func TestReprimeCapabilityAbsentSkipsInjection(t *testing.T) {
	// A service without the synthetic endpoint: prompts still go through
	// verbatim and no error surfaces — restoration is skipped, never
	// simulated by rewriting the prompt.
	f := newReprimeFixture(t, []string{reprimeCompacted}, "FIX-00")
	f.synthCapable = false
	if got := f.promptViaChat(t, "old service"); got != "old service" {
		t.Errorf("capability-absent prompt = %q, want verbatim", got)
	}
	if n := f.syntheticCount(); n != 0 {
		t.Errorf("capability-absent synthetic count = %d, want 0", n)
	}
}

func TestReprimeInjectionFailureFailsOpen(t *testing.T) {
	f := newReprimeFixture(t, []string{reprimeCompacted}, "FIX-00")
	f.mu.Lock()
	f.mu.synthCode = http.StatusInternalServerError
	f.mu.Unlock()
	if got := f.promptViaChat(t, "service hiccup"); got != "service hiccup" {
		t.Errorf("injection-failure prompt = %q, want verbatim", got)
	}
}

func TestSessionInboxHidesPendingRestoration(t *testing.T) {
	// The pending queue shows only the user's own messages: the re-prime
	// restoration synthetic must never surface as a "Steering" row while
	// it waits for delivery.
	f := newReprimeFixture(t, nil, "FIX-00")
	f.mu.Lock()
	f.mu.inbox = `{"data":[` +
		`{"id":"msg_sy","sessionID":"ses_x","timeCreated":9,"type":"synthetic","payload":{"text":"Context restoration: pending"},"delivery":"steer"},` +
		`{"id":"msg_q","sessionID":"ses_x","timeCreated":10,"type":"user","payload":{"text":"my follow-up"},"delivery":"queue"},` +
		`{"id":"msg_other","sessionID":"ses_x","timeCreated":11,"type":"synthetic","payload":{"text":"<shell state=\"cancelled\"/>"},"delivery":"steer"}]}`
	f.mu.Unlock()
	w := do(t, f.s.Handler(), "GET", "/api/sessions/ses_x/inbox", "")
	if w.Code != http.StatusOK {
		t.Fatalf("inbox code = %d body = %s", w.Code, w.Body)
	}
	var view struct {
		Items []opencode.InboxItem `json:"items"`
	}
	if err := json.Unmarshal(w.Body.Bytes(), &view); err != nil {
		t.Fatal(err)
	}
	if len(view.Items) != 2 {
		t.Fatalf("items = %d, want 2 (restoration hidden, follow-up and service synthetic kept)", len(view.Items))
	}
	for _, item := range view.Items {
		if item.Type == "synthetic" && strings.HasPrefix(item.Text, reprimeMarker) {
			t.Errorf("restoration leaked into the inbox feed: %+v", item)
		}
	}
}

func TestSessionDeliverReprimeInjects(t *testing.T) {
	// The queued-delivery path (POST /deliver) must behave like the chat
	// prompt path: inject the restoration, deliver the text verbatim.
	f := newReprimeFixture(t, []string{reprimeCompacted}, "FIX-00")
	w := do(t, f.s.Handler(), "POST", "/api/sessions/ses_x/deliver", `{"id":"msg_q","text":"deliver probe","delivery":"queue"}`)
	if w.Code != http.StatusAccepted {
		t.Fatalf("deliver code = %d body = %s", w.Code, w.Body)
	}
	if got := f.lastPrompt(t); got != "deliver probe" {
		t.Errorf("deliver path prompt = %q, want verbatim", got)
	}
	if n := f.syntheticCount(); n != 1 {
		t.Fatalf("deliver path synthetic count = %d, want 1", n)
	}
	assertRestoration(t, f.lastSynthetic())
}

func TestSessionInboxResumeReprimesAfterCompactionWithoutResubmitting(t *testing.T) {
	for _, tc := range []struct {
		name, page, delivery string
		wantSynthetic        int
	}{
		{"resume after compaction", reprimeCompacted, "steer", 1},
		{"already restored", reprimeDelivered, "steer", 0},
		{"failed compaction", reprimeFailed, "steer", 0},
		{"keep queued", reprimeCompacted, "queue", 0},
	} {
		t.Run(tc.name, func(t *testing.T) {
			f := newReprimeFixture(t, []string{tc.page}, "FIX-00")
			w := do(t, f.s.Handler(), "PATCH", "/api/sessions/ses_x/inbox/msg_q", `{"delivery":"`+tc.delivery+`"}`)
			if w.Code != 204 || f.syntheticCount() != tc.wantSynthetic {
				t.Fatalf("resume = %d %s; synthetics = %d, want %d", w.Code, w.Body.String(), f.syntheticCount(), tc.wantSynthetic)
			}
			f.mu.Lock()
			defer f.mu.Unlock()
			if f.mu.inboxUpdates != 1 || len(f.mu.prompts) != 0 {
				t.Fatalf("resume resubmitted a prompt: updates=%d prompts=%v", f.mu.inboxUpdates, f.mu.prompts)
			}
			if f.mu.syntheticsAtUpdate != tc.wantSynthetic {
				t.Fatal("context restoration must be admitted before waking the existing item")
			}
		})
	}
}
