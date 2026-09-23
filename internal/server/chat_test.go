package server

import (
	"bytes"
	"encoding/base64"
	"encoding/json"
	"errors"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"net/textproto"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"lessmess/internal/opencode"
)

func chatFake(t *testing.T, mutate func(*http.Request, map[string]any)) *Server {
	t.Helper()
	return mappingServer(t, func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.Method == http.MethodGet && r.URL.Path == "/api/session/ses_chat/message":
			if r.URL.Query().Get("limit") != "50" || r.URL.Query().Get("order") != "desc" {
				t.Errorf("message query = %s", r.URL.RawQuery)
			}
			w.Write([]byte(`{"data":[{"id":"msg_notice","type":"future-message","description":"later","time":{"created":3}},{"id":"msg_assistant","type":"assistant","agent":"build","model":{"providerID":"p","id":"m"},"time":{"created":2},"content":[{"type":"text","text":"working"},{"type":"reasoning","text":"checking"},{"type":"tool","id":"tool_1","name":"read","state":{"status":"completed","input":{},"content":[{"type":"text","text":"result"}]},"time":{"created":2}},{"type":"future-<img src=x onerror=alert(1)>"}]},{"id":"msg_user","type":"user","text":"hello **phone**","files":[{"data":"c2VjcmV0LWJ5dGVz","mime":"text/plain","name":"../../note.txt","source":{"type":"uri","uri":"file:///do/not/render"}}],"time":{"created":1}}],"cursor":{"previous":null,"next":"older-token"}}`))
		case r.Method == http.MethodGet && r.URL.Path == "/api/session/active":
			w.Write([]byte(`{"data":{"ses_chat":{"type":"running"}}}`))
		case r.Method == http.MethodGet && r.URL.Path == "/api/session/ses_chat/permission":
			w.Write([]byte(`{"data":[{"id":"per_1","sessionID":"ses_chat","action":"read","resources":["/repo/<secret>"],"message":"Allow read?"}]}`))
		case r.Method == http.MethodGet && r.URL.Path == "/api/session/ses_chat/form":
			w.Write([]byte(`{"data":[{"id":"frm_1","sessionID":"ses_chat","title":"Choose","fields":[{"key":"name","type":"string","title":"Name","required":true,"default":"Ada"},{"key":"environment","type":"string","title":"Environment","options":[{"value":"prod","label":"Production","description":"Customer-facing systems"}],"default":"prod"},{"key":"count","type":"integer","default":2},{"key":"confirm","type":"boolean","default":true},{"key":"targets","type":"multiselect","required":true,"options":[{"value":"a","label":"A","description":"First target"}],"default":["a"]},{"key":"docs","type":"external","url":"https://example.test"}]}]}`))
		default:
			var body map[string]any
			if r.Body != nil {
				_ = json.NewDecoder(r.Body).Decode(&body)
			}
			if mutate != nil {
				mutate(r, body)
			}
			switch {
			case strings.HasSuffix(r.URL.Path, "/interrupt"):
				w.Write([]byte(`{"data":{"interrupted":true}}`))
			case strings.Contains(r.URL.Path, "/reply"):
				w.WriteHeader(http.StatusNoContent)
			case strings.HasSuffix(r.URL.Path, "/prompt"):
				w.Write([]byte(`{"data":{}}`))
			default:
				t.Errorf("unexpected request %s %s", r.Method, r.URL.Path)
				w.WriteHeader(http.StatusNotFound)
			}
		}
	})
}

func TestChatSnapshotRendersSafeAuthoritativeState(t *testing.T) {
	s := chatFake(t, nil)
	w := do(t, s.Handler(), "GET", "/api/sessions/ses_chat/chat", "")
	if w.Code != http.StatusOK {
		t.Fatalf("code = %d body = %s", w.Code, w.Body.String())
	}
	body := w.Body.String()
	for _, want := range []string{
		`data-session="ses_chat"`, `data-busy="true"`, `data-message="msg_user"`,
		`data-chat-load-older`, `data-cursor="older-token"`,
		"<strong>phone</strong>",
		"Reference: note.txt", "/api/sessions/ses_chat/chat/messages/msg_user/files/0", "Download",
		`data-permission="per_1"`, "Allow read?", `data-chat-form="frm_1"`,
		`name="name"`, `value="Ada"`, `data-field-type="integer"`, "https://example.test",
		`class="chat-choice-list" role="radiogroup"`, `type="radio" data-field-type="string"`, "Customer-facing systems",
		`type="checkbox" data-field-type="multiselect"`, "First target",
		`data-chat-form-step="0"`, `data-chat-form-step="1" hidden`, `data-chat-form-review hidden`,
		`data-chat-form-progress role="status"`, `data-chat-form-prev hidden`, `data-chat-form-next`, `data-chat-form-submit hidden`,
		`aria-label="Form navigation"`, `novalidate`, `Review your answers`,
		"Unsupported message part:", "future-message", "chat-flow-unknown",
	} {
		if !strings.Contains(body, want) {
			t.Errorf("snapshot missing %q:\n%s", want, body)
		}
	}
	if strings.Contains(body, "<img src=x") {
		t.Fatalf("unknown part type rendered as markup: %s", body)
	}
	if strings.Count(body, `data-chat-form-step=`) != 6 {
		t.Fatalf("snapshot did not render exactly one wizard step per field: %s", body)
	}
	if strings.Contains(body, `<select name="environment"`) || strings.Contains(body, `<select name="targets"`) {
		t.Fatalf("choice questions regressed to native selects: %s", body)
	}
	for _, want := range []string{`data-chat-message-actions`, `class="chat-message-menu"`, `data-chat-copy-message`, `data-chat-fork="msg_user"`, `data-chat-revert="msg_user"`, `aria-haspopup="menu"`, `aria-expanded="false"`} {
		if !strings.Contains(body, want) {
			t.Errorf("snapshot missing contextual message action %q: %s", want, body)
		}
	}
	if strings.Count(body, `data-chat-message-actions`) != 2 || strings.Count(body, `class="chat-message-menu"`) != 2 || strings.Count(body, `data-chat-copy-message`) != 2 {
		t.Fatalf("snapshot did not render user actions and assistant copy action: %s", body)
	}
	if strings.Contains(body, `data-chat-fork="msg_assistant"`) || strings.Contains(body, `data-chat-revert="msg_assistant"`) {
		t.Fatalf("snapshot rendered lifecycle actions for an assistant message: %s", body)
	}
	for _, source := range []string{`class="chat-markdown-source" hidden>working</span>`, `class="chat-markdown-source" hidden>hello **phone**</span>`} {
		if !strings.Contains(body, source) {
			t.Errorf("snapshot missing raw Markdown source %q: %s", source, body)
		}
	}
	if strings.Contains(body, "<header>Assistant</header>") {
		t.Fatalf("snapshot rendered redundant assistant heading: %s", body)
	}
	if strings.Contains(body, "<header>You</header>") {
		t.Fatalf("snapshot rendered redundant user heading: %s", body)
	}
	if strings.Contains(body, "Review turn changes") || strings.Contains(body, "/chat/diff?from=") {
		t.Fatalf("snapshot rendered unsupported per-turn diff disclosure: %s", body)
	}
	if strings.Contains(body, "password") || strings.Contains(body, "Basic ") || strings.Contains(body, "secret-bytes") || strings.Contains(body, "do/not/render") {
		t.Fatalf("snapshot leaked credentials: %s", body)
	}
	if strings.Contains(body, "result") {
		t.Fatalf("snapshot eagerly included tool output: %s", body)
	}
	if strings.Contains(body, `class="chat-system-group`) || strings.Contains(body, "Reasoning") {
		t.Fatalf("pending interaction retained stale System activity: %s", body)
	}
	if strings.Count(body, `data-message="msg_assistant"`) != 1 {
		t.Fatalf("split assistant emitted duplicate source markers: %s", body)
	}
}

func TestChatSnapshotGroupsConsecutiveActivityAcrossMessages(t *testing.T) {
	s := mappingServer(t, func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/api/session/ses_group/message":
			w.Write([]byte(`{"data":[{"id":"msg_shell","type":"shell","command":"go test ./...","status":"running","time":{"created":3}},{"id":"msg_tool","type":"assistant","content":[{"type":"tool","id":"tool_1","name":"read","state":{"status":"running","input":{}},"time":{"created":2}}],"time":{"created":2}},{"id":"msg_reason","type":"assistant","content":[{"type":"reasoning","text":"checking"}],"time":{"created":1}}],"cursor":{}}`))
		case "/api/session/active":
			w.Write([]byte(`{"data":{"ses_group":{"type":"running"}}}`))
		case "/api/session/ses_group/permission", "/api/session/ses_group/form":
			w.Write([]byte(`{"data":[]}`))
		default:
			w.WriteHeader(http.StatusNotFound)
		}
	})
	w := do(t, s.Handler(), "GET", "/api/sessions/ses_group/chat", "")
	body := w.Body.String()
	if w.Code != http.StatusOK {
		t.Fatalf("code = %d body = %s", w.Code, body)
	}
	if strings.Count(body, `class="chat-system-group`) != 1 {
		t.Fatalf("activity was not one group: %s", body)
	}
	for _, want := range []string{
		`data-chat-expand-key="system:reasoning:msg_reason:0"`,
		`data-chat-expand-key="reasoning:msg_reason:0"`,
		`data-chat-expand-key="tool:msg_tool:tool_1"`,
		`data-chat-expand-key="shell:msg_shell"`,
		`role="status"`, `class="spinner"`, "Running shell",
		`data-message="msg_reason"`, `data-message="msg_tool"`, `data-message="msg_shell"`,
	} {
		if !strings.Contains(body, want) {
			t.Errorf("group missing %q: %s", want, body)
		}
	}
	if strings.Contains(body, "OpenCode is working...</div>") || strings.Contains(body, "result") {
		t.Fatalf("group rendered duplicate running row or eager detail: %s", body)
	}
	if !(strings.Index(body, "Reasoning") < strings.Index(body, ">read<") && strings.Index(body, ">read<") < strings.Index(body, ">Shell<")) {
		t.Fatalf("activity order changed: %s", body)
	}
}

func TestChatSnapshotDoesNotReactivatePreviousActivity(t *testing.T) {
	s := mappingServer(t, func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/api/session/ses_new_turn/message":
			w.Write([]byte(`{"data":[{"id":"msg_user","type":"user","content":[{"type":"text","text":"new question"}],"time":{"created":2}},{"id":"msg_old","type":"assistant","content":[{"type":"reasoning","text":"old thought"}],"time":{"created":1}}],"cursor":{}}`))
		case "/api/session/active":
			w.Write([]byte(`{"data":{"ses_new_turn":{"type":"running"}}}`))
		case "/api/session/ses_new_turn/permission", "/api/session/ses_new_turn/form":
			w.Write([]byte(`{"data":[]}`))
		default:
			w.WriteHeader(http.StatusNotFound)
		}
	})
	w := do(t, s.Handler(), "GET", "/api/sessions/ses_new_turn/chat", "")
	body := w.Body.String()
	if w.Code != http.StatusOK {
		t.Fatalf("code = %d body = %s", w.Code, body)
	}
	if strings.Contains(body, "old thought") || !strings.Contains(body, "new question") {
		t.Fatalf("snapshot retained previous-turn activity or lost the new turn: %s", body)
	}
	for _, stale := range []string{`class="chat-system-group is-running"`, `class="spinner"`, "Thinking"} {
		if strings.Contains(body, stale) {
			t.Errorf("previous activity was reactivated by the new user turn %q: %s", stale, body)
		}
	}
}

func TestChatSnapshotDoesNotReactivateCompletedLatestActivity(t *testing.T) {
	s := mappingServer(t, func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/api/session/ses_settled/message":
			w.Write([]byte(`{"data":[{"id":"msg_done","type":"assistant","content":[{"type":"tool","id":"tool_1","name":"read","state":{"status":"completed","input":{}}}],"time":{"created":1,"completed":2}}],"cursor":{}}`))
		case "/api/session/active":
			w.Write([]byte(`{"data":{"ses_settled":{"type":"running"}}}`))
		case "/api/session/ses_settled/permission", "/api/session/ses_settled/form":
			w.Write([]byte(`{"data":[]}`))
		default:
			w.WriteHeader(http.StatusNotFound)
		}
	})
	body := do(t, s.Handler(), "GET", "/api/sessions/ses_settled/chat", "").Body.String()
	for _, stale := range []string{`class="chat-system-group`, `class="spinner"`, "Running read"} {
		if strings.Contains(body, stale) {
			t.Errorf("completed activity was reactivated by session status %q: %s", stale, body)
		}
	}
}

func TestChatSnapshotHidesRunningActivityWhileWaitingForForm(t *testing.T) {
	s := mappingServer(t, func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/api/session/ses_wait/message":
			w.Write([]byte(`{"data":[{"id":"msg_tool","type":"assistant","content":[{"type":"tool","id":"tool_1","name":"question","state":{"status":"running","input":{}}}],"time":{"created":1}}],"cursor":{}}`))
		case "/api/session/active":
			w.Write([]byte(`{"data":{"ses_wait":{"type":"running"}}}`))
		case "/api/session/ses_wait/permission":
			w.Write([]byte(`{"data":[]}`))
		case "/api/session/ses_wait/form":
			w.Write([]byte(`{"data":[{"id":"frm_wait","sessionID":"ses_wait","title":"Choose","fields":[{"key":"choice","type":"string","options":[{"value":"a","label":"A"}]}]}]}`))
		default:
			w.WriteHeader(http.StatusNotFound)
		}
	})
	body := do(t, s.Handler(), "GET", "/api/sessions/ses_wait/chat", "").Body.String()
	if !strings.Contains(body, `data-chat-form="frm_wait"`) {
		t.Fatalf("snapshot lost pending form: %s", body)
	}
	for _, stale := range []string{`class="chat-system-group`, `class="spinner"`, "Running question"} {
		if strings.Contains(body, stale) {
			t.Errorf("pending form was obscured by activity %q: %s", stale, body)
		}
	}
}

func TestChatTranscriptActivityBoundaries(t *testing.T) {
	messages := []chatMessageView{
		{ID: "msg_a", Type: "assistant", Parts: []chatPartView{{Kind: "reasoning"}, {Kind: "text", Text: "prose"}, {Kind: "tool", ToolID: "tool_1"}}},
		{ID: "msg_synthetic", Type: "synthetic", Text: "internal generated context"},
		{ID: "msg_files", Type: "assistant", Files: []chatFileView{{Name: "note.txt"}}},
		{ID: "msg_user", Type: "user", Text: "question"},
		{ID: "msg_shell", Type: "shell", Shell: &chatShellView{Command: "pwd"}, Error: "failed"},
		{ID: "msg_unknown", Type: "assistant", Parts: []chatPartView{{Kind: "unknown", UnknownType: "future"}, {Kind: "reasoning"}}},
		{ID: "msg_system", Type: "system", Text: "notice"},
	}
	blocks := makeChatTranscriptBlocks(messages)
	want := []string{"activity", "message", "activity", "message", "message", "activity", "message", "message", "activity", "message"}
	if len(blocks) != len(want) {
		t.Fatalf("block count = %d, want %d: %#v", len(blocks), len(want), blocks)
	}
	for i := range want {
		if blocks[i].Kind != want[i] {
			t.Errorf("block %d kind = %q, want %q", i, blocks[i].Kind, want[i])
		}
	}
	for _, block := range blocks {
		for _, id := range block.SourceIDs {
			if id == "msg_synthetic" {
				t.Fatalf("synthetic message rendered in transcript block: %#v", block)
			}
		}
	}
	if got := runningActivitySummary(chatActivityView{Kind: "tool", Part: &chatPartView{ToolName: "  "}}); got != "Running tool" {
		t.Errorf("empty tool summary = %q", got)
	}
	if got := runningActivitySummary(chatActivityView{Kind: "reasoning"}); got != "Thinking" {
		t.Errorf("reasoning summary = %q", got)
	}
	grouped := makeChatTranscriptBlocks([]chatMessageView{
		{ID: "msg_reasoning", Type: "assistant", Parts: []chatPartView{{Kind: "reasoning"}}},
		{ID: "msg_synthetic", Type: "synthetic", Text: "internal"},
		{ID: "msg_tool", Type: "assistant", Parts: []chatPartView{{Kind: "tool", ToolID: "tool_2"}}},
	})
	if len(grouped) != 1 || grouped[0].Kind != "activity" || len(grouped[0].Activity) != 2 {
		t.Fatalf("synthetic message split activity group: %#v", grouped)
	}
	longName := strings.Repeat("x", 100)
	if got := runningActivitySummary(chatActivityView{Kind: "tool", Part: &chatPartView{ToolName: longName}}); got != "Running "+strings.Repeat("x", 64) {
		t.Errorf("bounded tool summary = %q", got)
	}
}

func TestChatHistoryActivityIsSettledAndPageBounded(t *testing.T) {
	s := mappingServer(t, func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api/session/ses_history/message" {
			w.WriteHeader(http.StatusNotFound)
			return
		}
		w.Write([]byte(`{"data":[{"id":"msg_old","type":"assistant","content":[{"type":"reasoning","text":"old"}],"time":{"created":1}}],"cursor":{}}`))
	})
	w := do(t, s.Handler(), "GET", "/api/sessions/ses_history/chat?cursor=page-2", "")
	body := w.Body.String()
	if !strings.Contains(body, `data-page="history"`) || !strings.Contains(body, ">System</span>") {
		t.Fatalf("history activity did not settle: %s", body)
	}
	for _, forbidden := range []string{`role="status"`, `class="spinner"`, "Thinking", "Running "} {
		if strings.Contains(body, forbidden) {
			t.Errorf("history rendered running state %q: %s", forbidden, body)
		}
	}
}

func TestChatExpansionJavaScriptContract(t *testing.T) {
	s := mappingServer(t, nil)
	js := do(t, s.Handler(), "GET", "/static/app.js", "").Body.String()
	for _, want := range []string{
		`details[data-chat-expand-key][open]`,
		`detail.dataset.chatExpandKey`,
		`detail.hasAttribute("data-chat-detail-url")`,
		`historyPage.className = "chat-history-page"`,
		`:scope > [data-chat-block]`,
		`function openChatMessageActions(message, focusMenu)`,
		`function closeChatMessageActions(returnFocus)`,
		`[data-chat-copy-message]`,
		`message.querySelectorAll(".chat-markdown-source")`,
		`navigator.clipboard.writeText(text)`,
		`option.dataset.fieldType === "multiselect" && option.checked`,
		`field.type === "radio"`,
		`data-chat-choice-required="true"`,
	} {
		if !strings.Contains(js, want) {
			t.Errorf("expansion/history contract missing %q", want)
		}
	}
	css := do(t, s.Handler(), "GET", "/static/app.css", "").Body.String()
	for _, want := range []string{`.chat-message-menu {`, `position: absolute`, `.chat-message-menu[hidden] { display: none; }`, `.chat-choice {`, `.chat-choice:has(input:checked)::before { content: "✓"; }`, `clip: rect(0, 0, 0, 0)`} {
		if !strings.Contains(css, want) {
			t.Errorf("contextual message action CSS missing %q", want)
		}
	}
}

func TestChatFormWizardControllerContract(t *testing.T) {
	s := mappingServer(t, nil)
	js := do(t, s.Handler(), "GET", "/static/app.js", "").Body.String()
	for _, want := range []string{
		`function collectChatFormAnswers(form)`, `parseInt(field.value, 10)`, `Number(field.value)`,
		`function validateChatFormStep(form, step, report)`, `Select at least one option.`,
		`document.addEventListener("click"`, `[data-chat-form-prev], #chat-transcript [data-chat-form-next]`,
		`function captureChatForms()`, `function restoreChatForms()`, `cstate.formStates`,
		`captureChatForms();`, `restoreChatForms();`, `state.answers = collectChatFormAnswers(form)`,
		`state.status === "submitting" || state.status === "submitted" || cstate.mutation`,
		`setChatFormSubmitting(form, "submitting", "")`, `setChatFormSubmitting(currentChatForm(formID) || form, "submitted", "")`,
		`setChatFormSubmitting(currentChatForm(formID) || form, "error", err.message)`, `Form submission failed: `,
		`var formID = form.dataset.formId || form.dataset.chatForm`, `"forms/" + encodeURIComponent(formID) + "/reply"`,
		`{ answers: answers }`, `compactChatUI() && requiredForm`,
	} {
		if !strings.Contains(js, want) {
			t.Errorf("chat form wizard controller missing %q", want)
		}
	}
	if strings.Count(js, `"forms/" + encodeURIComponent(formID) + "/reply"`) != 1 {
		t.Errorf("form reply path is not centralized to one submit call")
	}

	css := do(t, s.Handler(), "GET", "/static/app.css", "").Body.String()
	for _, want := range []string{
		`.chat-choice input {`, `position: absolute`, `opacity: 0`, `.chat-choice::before`,
		`.chat-choice:has(input:focus-visible)`, `min-height: 44px`,
		`.chat-form-body { min-height: 0; overflow-y: auto`, `.chat-form-nav {`,
		`height: var(--chat-viewport-height, 100dvh)`, `env(safe-area-inset-bottom)`,
	} {
		if !strings.Contains(css, want) {
			t.Errorf("chat form wizard CSS missing %q", want)
		}
	}
}

func TestChatSnapshotLoadsOlderPageWithoutStateFanout(t *testing.T) {
	requests := 0
	s := mappingServer(t, func(w http.ResponseWriter, r *http.Request) {
		requests++
		if r.URL.Path != "/api/session/ses_chat/message" {
			t.Errorf("unexpected history request %s", r.URL.Path)
			w.WriteHeader(http.StatusNotFound)
			return
		}
		if r.URL.Query().Get("limit") != "50" || r.URL.Query().Get("cursor") != "opaque/+ cursor" || r.URL.Query().Has("order") {
			t.Errorf("history query = %s", r.URL.RawQuery)
		}
		w.Write([]byte(`{"data":[{"id":"msg_2","type":"user","text":"second","time":{"created":2}},{"id":"msg_1","type":"user","text":"first","time":{"created":1}}],"cursor":{"previous":"newer","next":"still-older"}}`))
	})
	w := do(t, s.Handler(), "GET", "/api/sessions/ses_chat/chat?cursor=opaque%2F%2B%20cursor", "")
	if w.Code != http.StatusOK {
		t.Fatalf("code = %d body = %s", w.Code, w.Body.String())
	}
	body := w.Body.String()
	if requests != 1 || !strings.Contains(body, `data-page="history"`) || !strings.Contains(body, `data-cursor="still-older"`) {
		t.Fatalf("requests = %d body = %s", requests, body)
	}
	if strings.Index(body, `data-message="msg_1"`) > strings.Index(body, `data-message="msg_2"`) {
		t.Fatalf("history not rendered chronologically: %s", body)
	}
}

func TestChatSnapshotRendersBoundedPublishedShellOutput(t *testing.T) {
	output := "<script>hostile</script>\n" + strings.Repeat("x", chatShellOutputMax+100)
	s := mappingServer(t, func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/api/session/ses_shell/message":
			json.NewEncoder(w).Encode(map[string]any{"data": []any{map[string]any{
				"id": "msg_shell", "type": "shell", "shellID": "sh_1", "command": "printf '<unsafe>'", "status": "exited", "exit": 7,
				"output": map[string]any{"output": output, "size": len(output), "truncated": false}, "time": map[string]any{"created": 1},
			}}, "cursor": map[string]any{}})
		case "/api/session/active":
			w.Write([]byte(`{"data":{}}`))
		case "/api/session/ses_shell/permission", "/api/session/ses_shell/form":
			w.Write([]byte(`{"data":[]}`))
		default:
			w.WriteHeader(http.StatusNotFound)
		}
	})
	w := do(t, s.Handler(), "GET", "/api/sessions/ses_shell/chat", "")
	body := w.Body.String()
	for _, want := range []string{"Shell operation", "chat-flow-shell", "Status:", "exited", "Exit:", "7", "printf &#39;&lt;unsafe&gt;&#39;", "Shell output truncated for browser display"} {
		if !strings.Contains(body, want) {
			t.Errorf("shell transcript missing %q: %.500s", want, body)
		}
	}
	if strings.Contains(body, "<script>") || len(body) > 100<<10 {
		t.Fatalf("shell output unsafe or unbounded: bytes=%d", len(body))
	}
}

func TestChatToolDetailStatesEscapingAndChunks(t *testing.T) {
	large := "<script>alert(1)</script>" + strings.Repeat("x", chatDetailMax+200)
	s := mappingServer(t, func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api/session/ses_chat/message/msg_tools" {
			t.Errorf("unexpected path %s", r.URL.Path)
			w.WriteHeader(http.StatusNotFound)
			return
		}
		message := map[string]any{
			"id": "msg_tools", "type": "assistant", "time": map[string]any{"created": 1000},
			"content": []any{
				map[string]any{"type": "tool", "id": "tool_stream", "name": "shell", "state": map[string]any{"status": "streaming", "input": "<b>partial</b>"}, "time": map[string]any{"created": 1000}},
				map[string]any{"type": "tool", "id": "tool_run", "name": "read", "state": map[string]any{"status": "running", "input": map[string]any{"path": "<img src=x>"}, "metadata": map[string]any{"secret": "not rendered"}}, "time": map[string]any{"created": 1000, "ran": 1010}},
				map[string]any{"type": "tool", "id": "tool_done", "name": "edit", "state": map[string]any{"status": "completed", "input": map[string]any{"file": "a.go"}, "content": []any{map[string]any{"type": "text", "text": large}, map[string]any{"type": "file", "name": "<bad>.txt", "mime": "text/plain", "uri": "file:///tmp/<bad>"}}}, "time": map[string]any{"created": 1000, "ran": 1010, "completed": 1040}},
				map[string]any{"type": "tool", "id": "tool_error", "name": "write", "state": map[string]any{"status": "error", "input": map[string]any{}, "error": map[string]any{"type": "Failure", "message": "<svg onload=bad>"}}, "time": map[string]any{"created": 1000, "completed": 1005}},
			},
		}
		json.NewEncoder(w).Encode(map[string]any{"data": message})
	})

	for _, id := range []string{"tool_stream", "tool_run", "tool_done", "tool_error"} {
		path := "/api/sessions/ses_chat/chat/messages/msg_tools/tools/" + id
		w := do(t, s.Handler(), "GET", path, "")
		if w.Code != http.StatusOK {
			t.Fatalf("%s: %d %s", id, w.Code, w.Body.String())
		}
		if strings.Contains(w.Body.String(), "<script>") || strings.Contains(w.Body.String(), "<img src") || strings.Contains(w.Body.String(), "<svg onload") {
			t.Fatalf("%s rendered hostile markup: %s", id, w.Body.String())
		}
		if strings.Contains(w.Body.String(), "not rendered") {
			t.Fatalf("%s rendered raw metadata: %s", id, w.Body.String())
		}
	}
	done := do(t, s.Handler(), "GET", "/api/sessions/ses_chat/chat/messages/msg_tools/tools/tool_done", "").Body.String()
	for _, want := range []string{"Load more output", "offset=32768", "&lt;bad&gt;.txt", "started &#43;10ms", "finished &#43;30ms"} {
		if !strings.Contains(done, want) {
			t.Errorf("completed detail missing %q: %s", want, done)
		}
	}
	chunk := do(t, s.Handler(), "GET", "/api/sessions/ses_chat/chat/messages/msg_tools/tools/tool_done?offset=32768&limit=100", "")
	if chunk.Code != http.StatusOK || chunk.Body.Len() > 10000 {
		t.Fatalf("chunk response = %d bytes=%d", chunk.Code, chunk.Body.Len())
	}
	capped := do(t, s.Handler(), "GET", "/api/sessions/ses_chat/chat/messages/msg_tools/tools/tool_done?offset=1048566&limit=100", "")
	if capped.Code != http.StatusOK || !strings.Contains(capped.Body.String(), "Tool detail capped at 1 MiB") {
		t.Fatalf("capped response = %d %s", capped.Code, capped.Body.String())
	}
}

func TestChatDiffRendersMobileSafeUnifiedPatchAndFallbacks(t *testing.T) {
	s := mappingServer(t, func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api/session/ses_chat/diff" {
			t.Errorf("unexpected path %s", r.URL.Path)
		}
		if r.URL.Query().Get("from") != "msg_from" || r.URL.Query().Get("to") != "msg_to" || r.URL.Query().Get("context") != "3" {
			t.Errorf("diff query = %s", r.URL.RawQuery)
		}
		w.Write([]byte(`{"data":[{"file":"<unsafe>.go","patch":"diff --git a/a.go b/a.go\n--- a/a.go\n+++ b/a.go\n@@ -1 +1 @@\n-<old>\n+<script>new</script>\n","additions":1,"deletions":1,"status":"modified"},{"file":"image.png","patch":"Binary files a/image.png and b/image.png differ\n","additions":0,"deletions":0,"status":"modified"},{"file":"empty.bin","patch":"","additions":0,"deletions":0,"status":"added"}]}`))
	})
	w := do(t, s.Handler(), "GET", "/api/sessions/ses_chat/chat/diff?from=msg_from&to=msg_to", "")
	body := w.Body.String()
	if w.Code != http.StatusOK {
		t.Fatalf("code = %d body = %s", w.Code, body)
	}
	for _, want := range []string{"3 affected files", "diff-add", "diff-delete", "diff-hunk", "&lt;unsafe&gt;.go", "Binary file changed", "No text patch", "Copy patch"} {
		if !strings.Contains(body, want) {
			t.Errorf("diff missing %q: %s", want, body)
		}
	}
	if strings.Contains(body, "<script>") || strings.Contains(body, "<old>") {
		t.Fatalf("diff rendered hostile patch as markup: %s", body)
	}
}

func TestChatDiffTruncatesLargePatch(t *testing.T) {
	s := mappingServer(t, func(w http.ResponseWriter, r *http.Request) {
		json.NewEncoder(w).Encode(map[string]any{"data": []any{map[string]any{
			"file": "large.txt", "patch": strings.Repeat("+large & hostile <line>\n", chatDiffFileMax/8),
			"additions": 40000, "deletions": 0, "status": "modified",
		}}})
	})
	w := do(t, s.Handler(), "GET", "/api/sessions/ses_chat/chat/diff?from=msg_from", "")
	if w.Code != http.StatusOK || !strings.Contains(w.Body.String(), "Patch truncated for browser transfer") {
		t.Fatalf("large diff = %d, bytes=%d", w.Code, w.Body.Len())
	}
	if w.Body.Len() > 1<<20 {
		t.Fatalf("escaped diff response exceeded cap allowance: %d bytes", w.Body.Len())
	}
}

func TestChatDetailValidationCapsAndUpstreamFailure(t *testing.T) {
	s := mappingServer(t, func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusBadGateway)
		w.Write([]byte(`{"message":"upstream failed"}`))
	})
	cases := []struct {
		path string
		code int
	}{
		{"/api/sessions/bad/chat/messages/msg_x/tools/tool_x", http.StatusBadRequest},
		{"/api/sessions/ses_x/chat/messages/bad/tools/tool_x", http.StatusBadRequest},
		{"/api/sessions/ses_x/chat/messages/msg_x/tools/tool_x?offset=-1", http.StatusBadRequest},
		{"/api/sessions/ses_x/chat/messages/msg_x/tools/tool_x?limit=32769", http.StatusBadRequest},
		{"/api/sessions/ses_x/chat/messages/msg_x/tools/tool_x?wat=1", http.StatusBadRequest},
		{"/api/sessions/ses_x/chat/diff", http.StatusBadRequest},
		{"/api/sessions/ses_x/chat/diff?from=bad", http.StatusBadRequest},
		{"/api/sessions/ses_x/chat/diff?from=msg_x&context=9", http.StatusBadRequest},
		{"/api/sessions/ses_x/chat/diff?from=msg_x", http.StatusBadGateway},
	}
	for _, tc := range cases {
		w := do(t, s.Handler(), "GET", tc.path, "")
		if w.Code != tc.code {
			t.Errorf("%s: code=%d want=%d body=%s", tc.path, w.Code, tc.code, w.Body.String())
		}
	}
	if got, ok := sliceChatBytes("a😀b", 2, 2); !ok || got != "" {
		t.Fatalf("UTF-8 chunk = %q, %v", got, ok)
	}
	if got := normalizeToolStatus("future"); got != "unknown (future)" {
		t.Fatalf("fallback status = %q", got)
	}
}

func TestChatMutations(t *testing.T) {
	seen := map[string]map[string]any{}
	s := chatFake(t, func(r *http.Request, body map[string]any) { seen[r.URL.Path] = body })

	cases := []struct {
		path string
		body string
		code int
	}{
		{"/api/sessions/ses_chat/chat/prompt", `{"text":"continue"}`, http.StatusAccepted},
		{"/api/sessions/ses_chat/chat/interrupt", `{}`, http.StatusOK},
		{"/api/sessions/ses_chat/chat/permissions/per_1/reply", `{"decision":"once"}`, http.StatusNoContent},
		{"/api/sessions/ses_chat/chat/forms/frm_1/reply", `{"answers":{"name":"Ada","confirm":true}}`, http.StatusNoContent},
	}
	for _, tc := range cases {
		w := do(t, s.Handler(), "POST", tc.path, tc.body)
		if w.Code != tc.code {
			t.Errorf("POST %s: code = %d body = %s", tc.path, w.Code, w.Body.String())
		}
	}
	if seen["/api/session/ses_chat/prompt"]["text"] != "continue" {
		t.Errorf("prompt body = %#v", seen["/api/session/ses_chat/prompt"])
	}
	if seen["/api/session/ses_chat/permission/per_1/reply"]["decision"] != "once" {
		t.Errorf("permission body = %#v", seen["/api/session/ses_chat/permission/per_1/reply"])
	}
	answer, _ := seen["/api/session/ses_chat/form/frm_1/reply"]["answer"].(map[string]any)
	if answer["name"] != "Ada" || answer["confirm"] != true {
		t.Errorf("form body = %#v", seen["/api/session/ses_chat/form/frm_1/reply"])
	}
}

func TestChatValidationAndUnavailable(t *testing.T) {
	s := mappingServer(t, nil)
	if w := do(t, s.Handler(), "GET", "/api/sessions/ses_chat/chat", ""); w.Code != http.StatusServiceUnavailable {
		t.Errorf("no service code = %d", w.Code)
	}

	s = chatFake(t, nil)
	cases := []struct {
		method string
		path   string
		body   string
		code   int
	}{
		{"GET", "/api/sessions/nope/chat", "", http.StatusBadRequest},
		{"GET", "/api/sessions/ses_chat/chat?cursor=", "", http.StatusBadRequest},
		{"GET", "/api/sessions/ses_chat/chat?cursor=a&cursor=b", "", http.StatusBadRequest},
		{"POST", "/api/sessions/ses_chat/chat/prompt", `{"text":" "}`, http.StatusUnprocessableEntity},
		{"POST", "/api/sessions/ses_chat/chat/prompt", `{"text":"ok","extra":true}`, http.StatusBadRequest},
		{"POST", "/api/sessions/ses_chat/chat/permissions/per_1/reply", `{"decision":"maybe"}`, http.StatusUnprocessableEntity},
		{"POST", "/api/sessions/ses_chat/chat/permissions/bad/reply", `{"decision":"once"}`, http.StatusBadRequest},
		{"POST", "/api/sessions/ses_chat/chat/forms/bad/reply", `{"answers":{}}`, http.StatusBadRequest},
		{"POST", "/api/sessions/ses_chat/chat/forms/frm_1/reply", `{}`, http.StatusUnprocessableEntity},
	}
	for _, tc := range cases {
		req, _ := http.NewRequest(tc.method, tc.path, strings.NewReader(tc.body))
		if tc.body != "" {
			req.Header.Set("Content-Type", "application/json")
		}
		w := doReq(t, s.Handler(), req)
		if w.Code != tc.code {
			t.Errorf("%s %s: code = %d, want %d; body=%s", tc.method, tc.path, w.Code, tc.code, w.Body.String())
		}
	}
}

func TestChatMapsOpenCodeErrors(t *testing.T) {
	s := mappingServer(t, func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNotFound)
		w.Write([]byte(`{"_tag":"SessionNotFoundError","message":"gone"}`))
	})
	w := do(t, s.Handler(), "GET", "/api/sessions/ses_missing/chat", "")
	if w.Code != http.StatusNotFound || !strings.Contains(w.Body.String(), "not found") || strings.Contains(w.Body.String(), "gone") {
		t.Fatalf("code = %d body = %s", w.Code, w.Body.String())
	}
}

func TestChatControlsSwitchCommandSkillAndRedaction(t *testing.T) {
	var switchedAgent, switchedModel, command, activated bool
	var switchedVariant string
	s := mappingServer(t, func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/api/agent":
			w.Write([]byte(`{"data":[{"id":"build","name":"Build","mode":"primary"},{"id":"hidden","name":"Hidden","mode":"primary","hidden":true}]}`))
		case "/api/model":
			w.Write([]byte(`{"data":[{"id":"m1","providerID":"p","name":"Enabled","enabled":true,"variants":[{"id":"low"},{"id":"high","settings":{"reasoning":"high"}}],"limit":{"context":1000,"output":100}},{"id":"m2","providerID":"p","name":"Disabled","enabled":false,"limit":{"context":1000,"output":100}}]}`))
		case "/api/command":
			w.Write([]byte(`{"data":[{"name":"review","description":"Review changes","template":"secret template"}]}`))
		case "/api/skill":
			w.Write([]byte(`{"data":[{"id":"safe","name":"Safe skill","description":"Useful","location":"/secret/skill.md","content":"secret content"}]}`))
		case "/api/session/ses_chat":
			w.Write([]byte(`{"data":{"id":"ses_chat","agent":"build","model":{"providerID":"p","id":"m1","variant":"high"},"tokens":{"input":700,"output":100,"reasoning":50,"cache":{"read":9,"write":2}},"location":{"directory":"/repo"},"time":{"created":1,"updated":2}}}`))
		case "/api/session/ses_chat/context":
			w.Write([]byte(`{"data":[{"id":"msg_old","type":"assistant","tokens":{"input":10,"output":1,"reasoning":0,"cache":{"read":2,"write":0}}},{"id":"msg_user","type":"user"},{"id":"msg_latest","type":"assistant","tokens":{"input":700,"output":100,"reasoning":50,"cache":{"read":9,"write":2}}}]}`))
		case "/openapi.json":
			w.Write([]byte(`{"paths":{"/api/session/{sessionID}/skill":{"post":{}}}}`))
		case "/api/session/ses_chat/agent":
			switchedAgent = true
			w.WriteHeader(http.StatusNoContent)
		case "/api/session/ses_chat/model":
			switchedModel = true
			var body struct {
				Model opencode.ModelRef `json:"model"`
			}
			if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
				t.Fatal(err)
			}
			switchedVariant = body.Model.Variant
			w.WriteHeader(http.StatusNoContent)
		case "/api/session/ses_chat/command":
			command = true
			w.Write([]byte(`{"data":{}}`))
		case "/api/session/ses_chat/skill":
			activated = true
			w.WriteHeader(http.StatusNoContent)
		default:
			t.Errorf("unexpected %s %s", r.Method, r.URL.Path)
			w.WriteHeader(http.StatusNotFound)
		}
	})
	w := do(t, s.Handler(), "GET", "/api/sessions/ses_chat/chat/controls", "")
	if w.Code != http.StatusOK {
		t.Fatalf("controls = %d %s", w.Code, w.Body.String())
	}
	body := w.Body.String()
	for _, forbidden := range []string{"Disabled", "secret template", "/secret/skill.md", "secret content"} {
		if strings.Contains(body, forbidden) {
			t.Fatalf("controls leaked %q: %s", forbidden, body)
		}
	}
	for _, want := range []string{`"agent":"build"`, `"model":"p/m1"`, `"variant":"high"`, `"variants":["low","high"]`, `"contextAvailable":true`, `"estimatedContext":861`, `"percent":86`, `"warning":true`, `"standaloneSkill":true`} {
		if !strings.Contains(body, want) {
			t.Errorf("controls missing %q: %s", want, body)
		}
	}
	for _, call := range []struct {
		path, body string
		code       int
	}{
		{"agent", `{"agent":"build"}`, 204}, {"model", `{"model":"p/m1","variant":"high"}`, 204},
		{"command", `{"command":"review","arguments":"now"}`, 202}, {"skill", `{"skill":"safe"}`, 204},
	} {
		w = do(t, s.Handler(), "POST", "/api/sessions/ses_chat/chat/"+call.path, call.body)
		if w.Code != call.code {
			t.Errorf("%s = %d %s", call.path, w.Code, w.Body.String())
		}
	}
	if !switchedAgent || !switchedModel || switchedVariant != "high" || !command || !activated {
		t.Fatalf("mutations = %v %v %v %v", switchedAgent, switchedModel, command, activated)
	}
	w = do(t, s.Handler(), "POST", "/api/sessions/ses_chat/chat/model", `{"model":"p/m1","variant":"gone"}`)
	if w.Code != http.StatusUnprocessableEntity || !strings.Contains(w.Body.String(), "variant") {
		t.Fatalf("invalid variant = %d %s", w.Code, w.Body.String())
	}
}

func TestChatUsageUsesLatestActiveAssistantAndKeepsAccountingSeparate(t *testing.T) {
	s := mappingServer(t, func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/api/session/ses_chat":
			w.Write([]byte(`{"data":{"id":"ses_chat","model":{"providerID":"p","id":"m"},"tokens":{"input":9000,"output":8000,"reasoning":7000,"cache":{"read":6000,"write":5000}}}}`))
		case "/api/model":
			w.Write([]byte(`{"data":[{"id":"m","providerID":"p","enabled":true,"limit":{"context":1000}}]}`))
		case "/api/session/ses_chat/context":
			w.Write([]byte(`{"data":[{"id":"msg_1","type":"assistant","tokens":{"input":700,"output":100,"reasoning":20,"cache":{"read":30,"write":10}}},{"id":"msg_user","type":"user"},{"id":"msg_2","type":"assistant","tokens":{"input":500,"output":100,"reasoning":25,"cache":{"read":200,"write":25}}}]}`))
		default:
			t.Errorf("unexpected %s %s", r.Method, r.URL.Path)
			w.WriteHeader(http.StatusNotFound)
		}
	})
	w := do(t, s.Handler(), "GET", "/api/sessions/ses_chat/chat/usage", "")
	if w.Code != http.StatusOK {
		t.Fatalf("usage = %d %s", w.Code, w.Body.String())
	}
	var usage chatUsageView
	if err := json.Unmarshal(w.Body.Bytes(), &usage); err != nil {
		t.Fatal(err)
	}
	if usage.Input != 9000 || usage.CacheRead != 6000 {
		t.Fatalf("accounting totals = %+v", usage)
	}
	if !usage.ContextAvailable || usage.EstimatedContext != 850 || usage.Percent != 85 || !usage.Warning {
		t.Fatalf("active usage = %+v", usage)
	}
}

func TestChatUsageContextUnavailableIsExplicit(t *testing.T) {
	s := mappingServer(t, func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/api/session/ses_chat":
			w.Write([]byte(`{"data":{"id":"ses_chat","model":{"providerID":"p","id":"m"},"tokens":{"input":9000,"output":8000,"reasoning":7000}}}`))
		case "/api/model":
			w.Write([]byte(`{"data":[{"id":"m","providerID":"p","enabled":true,"limit":{"context":1000}}]}`))
		case "/api/session/ses_chat/context":
			w.WriteHeader(http.StatusNotFound)
			w.Write([]byte(`{"message":"route unavailable"}`))
		default:
			t.Errorf("unexpected %s %s", r.Method, r.URL.Path)
		}
	})
	w := do(t, s.Handler(), "GET", "/api/sessions/ses_chat/chat/usage", "")
	if w.Code != http.StatusOK {
		t.Fatalf("usage = %d %s", w.Code, w.Body.String())
	}
	var usage chatUsageView
	if err := json.Unmarshal(w.Body.Bytes(), &usage); err != nil {
		t.Fatal(err)
	}
	if usage.ContextAvailable || usage.EstimatedContext != 0 || usage.Percent != 0 || usage.Warning {
		t.Fatalf("unavailable usage = %+v", usage)
	}
}

func TestChatControlInvalidRaceAndUnavailableSkill(t *testing.T) {
	mutated := false
	s := mappingServer(t, func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/api/agent", "/api/model", "/api/command", "/api/skill":
			w.Write([]byte(`{"data":[]}`))
		case "/openapi.json":
			w.Write([]byte(`{"paths":{}}`))
		default:
			mutated = true
			w.WriteHeader(http.StatusNoContent)
		}
	})
	for _, call := range []struct{ path, body string }{
		{"agent", `{"agent":"gone"}`}, {"model", `{"model":"p/gone"}`}, {"command", `{"command":"gone"}`}, {"skill", `{"skill":"gone"}`},
	} {
		w := do(t, s.Handler(), "POST", "/api/sessions/ses_chat/chat/"+call.path, call.body)
		if w.Code != http.StatusUnprocessableEntity {
			t.Errorf("%s = %d %s", call.path, w.Code, w.Body.String())
		}
	}
	if mutated {
		t.Fatal("invalid fresh selections reached mutation endpoint")
	}

	// A real listed skill remains attachable, but standalone activation is gated.
	s = mappingServer(t, func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/api/skill" {
			w.Write([]byte(`{"data":[{"id":"safe","name":"Safe","location":"x","content":"x"}]}`))
			return
		}
		if r.URL.Path == "/openapi.json" {
			w.Write([]byte(`{"paths":{}}`))
			return
		}
		t.Errorf("unexpected request %s", r.URL.Path)
	})
	w := do(t, s.Handler(), "POST", "/api/sessions/ses_chat/chat/skill", `{"skill":"safe"}`)
	if w.Code != http.StatusServiceUnavailable || !strings.Contains(w.Body.String(), "attach") {
		t.Fatalf("unavailable = %d %s", w.Code, w.Body.String())
	}
}

func TestChatTranscriptRetryProviderAndCompactionAreSanitized(t *testing.T) {
	s := mappingServer(t, func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/api/session/ses_chat/message":
			w.Write([]byte(`{"data":[{"id":"msg_c3","type":"compaction","status":"failed","reason":"auto","error":{"type":"ProviderError","message":"credential sk-secret at /private/path"},"time":{"created":6}},{"id":"msg_c2","type":"compaction","status":"completed","reason":"auto","summary":"private summary","recent":"private recent","time":{"created":5}},{"id":"msg_c1","type":"compaction","status":"running","reason":"manual","summary":"private summary","recent":"private recent","time":{"created":4}},{"id":"msg_a","type":"assistant","agent":"build","model":{"providerID":"p","id":"m"},"finish":"error","retry":{"attempt":2,"at":3,"error":{"type":"ProviderError","message":"raw retry secret"}},"error":{"type":"ProviderError","message":"raw provider secret","status":500},"providerState":{"authorization":"secret"},"content":[],"time":{"created":3}}],"cursor":{}}`))
		case "/api/session/active":
			w.Write([]byte(`{"data":{}}`))
		case "/api/session/ses_chat/permission", "/api/session/ses_chat/form":
			w.Write([]byte(`{"data":[]}`))
		}
	})
	w := do(t, s.Handler(), "GET", "/api/sessions/ses_chat/chat", "")
	body := w.Body.String()
	for _, want := range []string{"Retrying provider request", "provider could not finish", "Compacting conversation context", "Conversation context compacted", "Context compaction failed"} {
		if !strings.Contains(body, want) {
			t.Errorf("missing %q: %s", want, body)
		}
	}
	for _, secret := range []string{"raw retry secret", "raw provider secret", "providerState", "authorization", "private summary", "/private/path"} {
		if strings.Contains(body, secret) {
			t.Fatalf("transcript leaked %q: %s", secret, body)
		}
	}
}

func TestGeneralChatSession(t *testing.T) {
	var prime string
	s := mappingServer(t, func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.Method == http.MethodPost && r.URL.Path == "/api/session":
			w.Write([]byte(`{"data":{"id":"ses_general","title":"Codebase chat","location":{"directory":"/repo"}}}`))
		case r.Method == http.MethodPost && r.URL.Path == "/api/session/ses_general/prompt":
			var body map[string]string
			_ = json.NewDecoder(r.Body).Decode(&body)
			prime = body["text"]
			w.Write([]byte(`{"data":{}}`))
		default:
			t.Errorf("unexpected request %s %s", r.Method, r.URL.Path)
			w.WriteHeader(http.StatusNotFound)
		}
	})
	w := do(t, s.Handler(), "POST", "/chat/session", `{}`)
	if w.Code != http.StatusCreated {
		t.Fatalf("code = %d body = %s", w.Code, w.Body.String())
	}
	if !strings.Contains(prime, "general-purpose coding assistant") || !strings.Contains(prime, "never call workflow endpoints") {
		t.Fatalf("unexpected prime: %s", prime)
	}
	entries := s.sessions.listUnassigned()
	if len(entries) != 1 || entries[0].Session != "ses_general" || len(entries[0].Modules) != 1 || entries[0].Modules[0] != "chat" {
		t.Fatalf("mapping = %#v", entries)
	}
}

func multipartPrompt(t *testing.T, text string, files map[string][]byte, refs []string) (*bytes.Buffer, string) {
	t.Helper()
	var body bytes.Buffer
	mw := multipart.NewWriter(&body)
	if err := mw.WriteField("text", text); err != nil {
		t.Fatal(err)
	}
	for _, ref := range refs {
		if err := mw.WriteField("references", ref); err != nil {
			t.Fatal(err)
		}
	}
	for name, data := range files {
		h := make(textproto.MIMEHeader)
		h.Set("Content-Disposition", `form-data; name="files"; filename="`+name+`"`)
		h.Set("Content-Type", "text/plain")
		part, err := mw.CreatePart(h)
		if err != nil {
			t.Fatal(err)
		}
		if _, err := part.Write(data); err != nil {
			t.Fatal(err)
		}
	}
	if err := mw.Close(); err != nil {
		t.Fatal(err)
	}
	return &body, mw.FormDataContentType()
}

func TestChatAttachmentsAndReferenceAliases(t *testing.T) {
	var root string
	var prompt map[string]any
	s := mappingServer(t, func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/api/reference":
			if got := r.URL.Query().Get("location[directory]"); got != root {
				t.Errorf("reference location = %q, want %q", got, root)
			}
			w.Write([]byte(`{"data":[{"name":"../README label","path":"README.md","source":{"type":"local","path":"README.md"}},{"name":"escape","path":"../secret.txt","source":{"type":"local","path":"../secret.txt"}}]}`))
		case "/api/session/ses_chat/prompt":
			if err := json.NewDecoder(r.Body).Decode(&prompt); err != nil {
				t.Fatal(err)
			}
			w.Write([]byte(`{"data":{}}`))
		default:
			t.Errorf("unexpected request %s %s", r.Method, r.URL.Path)
			w.WriteHeader(http.StatusNotFound)
		}
	})
	root = s.st.Dir
	if err := os.WriteFile(filepath.Join(root, "README.md"), []byte("project"), 0o644); err != nil {
		t.Fatal(err)
	}
	w := do(t, s.Handler(), "GET", "/api/sessions/ses_chat/chat/references", "")
	if w.Code != http.StatusOK {
		t.Fatalf("references code = %d body = %s", w.Code, w.Body.String())
	}
	var listed struct {
		References []chatReferenceView `json:"references"`
	}
	if err := json.Unmarshal(w.Body.Bytes(), &listed); err != nil || len(listed.References) != 1 || listed.References[0].Alias == "" {
		t.Fatalf("listed = %#v, err = %v", listed, err)
	}
	body, contentType := multipartPrompt(t, "inspect", map[string][]byte{"../../notes.txt": []byte("hello")}, []string{listed.References[0].Alias})
	req := httptest.NewRequest(http.MethodPost, "/api/sessions/ses_chat/chat/prompt", body)
	req.Header.Set("Content-Type", contentType)
	w = doReq(t, s.Handler(), req)
	if w.Code != http.StatusAccepted {
		t.Fatalf("prompt code = %d body = %s", w.Code, w.Body.String())
	}
	files, _ := prompt["files"].([]any)
	if len(files) != 2 {
		t.Fatalf("prompt files = %#v", prompt["files"])
	}
	upload := files[0].(map[string]any)
	reference := files[1].(map[string]any)
	if upload["name"] != "notes.txt" || !strings.HasPrefix(upload["uri"].(string), "data:text/plain;base64,") {
		t.Errorf("upload = %#v", upload)
	}
	resolved, err := filepath.EvalSymlinks(filepath.Join(root, "README.md"))
	if err != nil {
		t.Fatal(err)
	}
	if reference["name"] != "README label" || reference["uri"] != "file://"+filepath.ToSlash(resolved) {
		t.Errorf("reference = %#v", reference)
	}
	if strings.Contains(w.Body.String(), "hello") || strings.Contains(w.Body.String(), root) {
		t.Fatalf("response leaked attachment data or path: %s", w.Body.String())
	}

	prompt = nil
	w = do(t, s.Handler(), "POST", "/api/sessions/ses_chat/chat/prompt", `{"text":"x","references":["README.md"]}`)
	if w.Code != http.StatusUnprocessableEntity || prompt != nil {
		t.Fatalf("forged alias code = %d prompt = %#v body = %s", w.Code, prompt, w.Body.String())
	}
}

func TestChatHistoricAttachmentFetchIsProjectedAndSafe(t *testing.T) {
	requests := 0
	s := mappingServer(t, func(w http.ResponseWriter, r *http.Request) {
		requests++
		switch r.URL.Path {
		case "/api/session/ses_chat/message/msg_file":
			w.Write([]byte(`{"data":{"id":"msg_file","type":"user","text":"see file","time":{"created":1},"files":[{"data":"aGVsbG8=","mime":"text/plain","name":"../../secret.txt","source":{"type":"uri","uri":"file:///private/secret.txt"}}]}}`))
		case "/api/session/ses_chat/message/msg_svg":
			w.Write([]byte(`{"data":{"id":"msg_svg","type":"user","text":"svg","time":{"created":1},"files":[{"data":"PHN2Zz48L3N2Zz4=","mime":"image/svg+xml","name":"image.svg","source":{"type":"inline"}}]}}`))
		case "/api/session/ses_chat/message/msg_bad":
			w.Write([]byte(`{"data":{"id":"msg_bad","type":"user","text":"bad","time":{"created":1},"files":[{"data":"cHJpdmF0ZS1ieXRlcw==","mime":"application/pdf","name":"secret.pdf","source":{"type":"inline"}}]}}`))
		default:
			t.Errorf("unexpected request %s", r.URL.Path)
			w.WriteHeader(http.StatusNotFound)
		}
	})
	w := do(t, s.Handler(), "GET", "/api/sessions/ses_chat/chat/messages/msg_file/files/0?download=1", "")
	if w.Code != http.StatusOK || w.Body.String() != "hello" {
		t.Fatalf("code = %d body = %q requests = %d", w.Code, w.Body.String(), requests)
	}
	if disposition := w.Header().Get("Content-Disposition"); !strings.Contains(disposition, "secret.txt") || strings.Contains(disposition, "../") {
		t.Fatalf("unsafe disposition = %q", disposition)
	}
	if strings.Contains(w.Header().Get("Content-Disposition"), "/private") || w.Header().Get("X-Content-Type-Options") != "nosniff" {
		t.Fatalf("unsafe attachment headers = %#v", w.Header())
	}
	w = do(t, s.Handler(), "GET", "/api/sessions/ses_chat/chat/messages/msg_svg/files/0", "")
	if w.Code != http.StatusOK || w.Body.String() != "<svg></svg>" || !strings.HasPrefix(w.Header().Get("Content-Type"), "text/plain") {
		t.Fatalf("SVG preview code = %d type = %q body = %q", w.Code, w.Header().Get("Content-Type"), w.Body.String())
	}
	w = do(t, s.Handler(), "GET", "/api/sessions/ses_chat/chat/messages/msg_bad/files/0", "")
	if w.Code != http.StatusUnsupportedMediaType || strings.Contains(w.Body.String(), "private-bytes") || requests != 3 {
		t.Fatalf("unsupported code = %d requests = %d body = %q", w.Code, requests, w.Body.String())
	}
}

func TestChatFileValidationLimitsAndMIME(t *testing.T) {
	if got := safeChatFilename(`..\\..\\evil.txt`); got != "evil.txt" {
		t.Fatalf("safe filename = %q", got)
	}
	if _, err := makePromptFiles([]chatPromptFile{{Name: "bad.txt", MIME: "text/plain", Data: base64.StdEncoding.EncodeToString([]byte{0xff})}}); err == nil || !strings.Contains(err.Error(), "UTF-8") {
		t.Fatalf("invalid UTF-8 err = %v", err)
	}
	if _, err := makePromptFiles([]chatPromptFile{{Name: "fake.png", MIME: "image/png", Data: base64.StdEncoding.EncodeToString([]byte("not png"))}}); err == nil || !strings.Contains(err.Error(), "does not match") {
		t.Fatalf("spoofed image err = %v", err)
	}
	chunk := base64.StdEncoding.EncodeToString(bytes.Repeat([]byte("a"), chatMaxFileTotal/2+1))
	if _, err := makePromptFiles([]chatPromptFile{{Name: "a.txt", MIME: "text/plain", Data: chunk}, {Name: "b.txt", MIME: "text/plain", Data: chunk}}); !errors.Is(err, errChatFileTooLarge) {
		t.Fatalf("aggregate limit err = %v", err)
	}
	inputs := make([]chatPromptFile, chatMaxFiles+1)
	if _, err := makePromptFiles(inputs); !errors.Is(err, errChatFileTooLarge) {
		t.Fatalf("count limit err = %v", err)
	}
}
