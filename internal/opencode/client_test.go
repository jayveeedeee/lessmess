package opencode

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

// fakeServer records requests and responds per the given handler.
func fakeServer(t *testing.T, wantUser, wantPass string, h http.HandlerFunc) (*Client, *httptest.Server) {
	t.Helper()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		u, p, ok := r.BasicAuth()
		if !ok || u != wantUser || p != wantPass {
			w.WriteHeader(http.StatusUnauthorized)
			return
		}
		h(w, r)
	}))
	t.Cleanup(srv.Close)
	return New(srv.URL, wantPass), srv
}

func TestAuthHeaderInjected(t *testing.T) {
	c, _ := fakeServer(t, "opencode", "secret", func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte(`{"data":{}}`))
	})
	var v map[string]any
	if err := c.do(context.Background(), http.MethodGet, "/x", nil, &v); err != nil {
		t.Fatalf("do: %v", err)
	}
}

func TestAuthHeaderValue(t *testing.T) {
	var got string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		got = r.Header.Get("Authorization")
		w.Write([]byte(`{"data":{}}`))
	}))
	defer srv.Close()
	c := New(srv.URL, "secret")
	var v map[string]any
	_ = c.do(context.Background(), http.MethodGet, "/x", nil, &v)
	want := "Basic " + base64.StdEncoding.EncodeToString([]byte("opencode:secret"))
	if got != want {
		t.Fatalf("Authorization = %q, want %q", got, want)
	}
}

func TestCreateSession(t *testing.T) {
	c, _ := fakeServer(t, "opencode", "pw", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost || r.URL.Path != "/api/session" {
			t.Errorf("%s %s", r.Method, r.URL.Path)
		}
		var body map[string]any
		json.NewDecoder(r.Body).Decode(&body)
		if body["title"] != "My session" {
			t.Errorf("title = %v", body["title"])
		}
		loc, _ := body["location"].(map[string]any)
		if loc["directory"] != "/repo" {
			t.Errorf("location = %v", body["location"])
		}
		w.Write([]byte(`{"data":{"id":"ses_1","title":"My session","location":{"directory":"/repo"}}}`))
	})
	s, err := c.CreateSession(context.Background(), "My session", "/repo")
	if err != nil {
		t.Fatalf("CreateSession: %v", err)
	}
	if s.ID != "ses_1" || s.Title != "My session" || s.Location.Directory != "/repo" {
		t.Fatalf("session = %+v", s)
	}
}

func TestListSessions(t *testing.T) {
	c, _ := fakeServer(t, "opencode", "pw", func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte(`{"data":[{"id":"ses_1","title":"a"},{"id":"ses_2","title":"b"}]}`))
	})
	ss, err := c.ListSessions(context.Background())
	if err != nil || len(ss) != 2 || ss[1].ID != "ses_2" {
		t.Fatalf("ss = %v, %v", ss, err)
	}
}

func TestListSessionsParsesParentID(t *testing.T) {
	c, _ := fakeServer(t, "opencode", "pw", func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte(`{"data":[{"id":"ses_child","title":"TSK-01: work","parentID":"ses_parent"},{"id":"ses_root","title":"root"}]}`))
	})
	ss, err := c.ListSessions(context.Background())
	if err != nil || len(ss) != 2 {
		t.Fatalf("ss = %v, %v", ss, err)
	}
	if ss[0].ParentID != "ses_parent" {
		t.Fatalf("child parentID = %q, want ses_parent", ss[0].ParentID)
	}
	if ss[1].ParentID != "" {
		t.Fatalf("root parentID = %q, want empty", ss[1].ParentID)
	}
}

func TestCreateSessionWithAgentAndModel(t *testing.T) {
	c, _ := fakeServer(t, "opencode", "pw", func(w http.ResponseWriter, r *http.Request) {
		var body map[string]any
		json.NewDecoder(r.Body).Decode(&body)
		if body["agent"] != "build" {
			t.Errorf("agent = %v, want build", body["agent"])
		}
		m, _ := body["model"].(map[string]any)
		if m["id"] != "accounts/f/m1" || m["providerID"] != "prov" {
			t.Errorf("model = %v", body["model"])
		}
		if _, ok := m["variant"]; ok {
			t.Errorf("variant must be omitted when empty: %v", m)
		}
		w.Write([]byte(`{"data":{"id":"ses_9","title":"t","location":{"directory":"/repo"}}}`))
	})
	s, err := c.CreateSessionWith(context.Background(), "t", "/repo", "build",
		&ModelRef{ID: "accounts/f/m1", ProviderID: "prov"})
	if err != nil {
		t.Fatalf("CreateSessionWith: %v", err)
	}
	if s.ID != "ses_9" {
		t.Fatalf("session = %+v", s)
	}
}

func TestCreateSessionWithOmitsEmptyDefaults(t *testing.T) {
	c, _ := fakeServer(t, "opencode", "pw", func(w http.ResponseWriter, r *http.Request) {
		var body map[string]any
		json.NewDecoder(r.Body).Decode(&body)
		if _, ok := body["agent"]; ok {
			t.Errorf("agent key must be omitted when empty: %v", body)
		}
		if _, ok := body["model"]; ok {
			t.Errorf("model key must be omitted for incomplete ref: %v", body)
		}
		w.Write([]byte(`{"data":{"id":"ses_1","title":"t"}}`))
	})
	// Empty agent and a model ref missing providerID → both omitted.
	if _, err := c.CreateSessionWith(context.Background(), "t", "/repo", "", &ModelRef{ID: "x"}); err != nil {
		t.Fatalf("CreateSessionWith: %v", err)
	}
}

func TestListAgentsAndModels(t *testing.T) {
	c, _ := fakeServer(t, "opencode", "pw", func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/api/agent":
			// Real shape: top-level {location, data:[...]} — do() unwraps data.
			w.Write([]byte(`{"location":{"directory":"/r"},"data":[{"id":"build","name":"Build","description":"d","mode":"primary","hidden":false},{"id":"general","name":"General","mode":"subagent"}]}`))
		case "/api/model":
			w.Write([]byte(`{"location":{"directory":"/r"},"data":[{"id":"accounts/f/m1","providerID":"prov","name":"M1"}]}`))
		case "/api/model/default":
			w.Write([]byte(`{"location":{"directory":"/r"},"data":{"id":"accounts/f/m1","providerID":"prov","name":"M1"}}`))
		default:
			t.Errorf("unexpected path %s", r.URL.Path)
		}
	})
	ctx := context.Background()
	agents, err := c.ListAgents(ctx)
	if err != nil || len(agents) != 2 || agents[0].ID != "build" || agents[1].Mode != "subagent" {
		t.Fatalf("agents = %v, %v", agents, err)
	}
	models, err := c.ListModels(ctx)
	if err != nil || len(models) != 1 || models[0].ProviderID != "prov" || models[0].Name != "M1" {
		t.Fatalf("models = %v, %v", models, err)
	}
	def, err := c.DefaultModel(ctx)
	if err != nil || def.ID != "accounts/f/m1" {
		t.Fatalf("default = %v, %v", def, err)
	}
}

func TestListAgentsAndModelsLocationScoped(t *testing.T) {
	c, _ := fakeServer(t, "opencode", "pw", func(w http.ResponseWriter, r *http.Request) {
		if got := r.URL.Query().Get("location[directory]"); got != "/repo" {
			t.Errorf("location[directory] = %q, want /repo", got)
		}
		switch r.URL.Path {
		case "/api/agent":
			w.Write([]byte(`{"data":[]}`))
		case "/api/model":
			w.Write([]byte(`{"data":[]}`))
		}
	})
	ctx := context.Background()
	if _, err := c.ListAgentsFor(ctx, "/repo"); err != nil {
		t.Fatal(err)
	}
	if _, err := c.ListModelsFor(ctx, "/repo"); err != nil {
		t.Fatal(err)
	}
}

func TestRenameAndDelete(t *testing.T) {
	c, _ := fakeServer(t, "opencode", "pw", func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.Method == http.MethodPatch && r.URL.Path == "/api/session/ses_1":
			var body map[string]string
			json.NewDecoder(r.Body).Decode(&body)
			if body["title"] != "new" {
				t.Errorf("title = %v", body["title"])
			}
			w.WriteHeader(http.StatusOK)
		case r.Method == http.MethodDelete:
			w.WriteHeader(http.StatusNoContent)
		default:
			t.Errorf("unexpected %s %s", r.Method, r.URL.Path)
		}
	})
	if err := c.RenameSession(context.Background(), "ses_1", "new"); err != nil {
		t.Fatalf("RenameSession: %v", err)
	}
	if err := c.DeleteSession(context.Background(), "ses_1"); err != nil {
		t.Fatalf("DeleteSession: %v", err)
	}
}

func TestPrompt(t *testing.T) {
	c, _ := fakeServer(t, "opencode", "pw", func(w http.ResponseWriter, r *http.Request) {
		if !strings.HasSuffix(r.URL.Path, "/prompt") {
			t.Errorf("path = %s", r.URL.Path)
		}
		var body map[string]string
		json.NewDecoder(r.Body).Decode(&body)
		if body["text"] != "hello" {
			t.Errorf("text = %v", body["text"])
		}
		w.Write([]byte(`{"data":{}}`))
	})
	if err := c.Prompt(context.Background(), "ses_1", "hello"); err != nil {
		t.Fatalf("Prompt: %v", err)
	}
}

func TestPromptFilesReferencesAndSingleMessageContracts(t *testing.T) {
	requests := 0
	c, _ := fakeServer(t, "opencode", "pw", func(w http.ResponseWriter, r *http.Request) {
		requests++
		switch {
		case r.Method == http.MethodPost && r.URL.Path == "/api/session/ses_1/prompt":
			var body struct {
				Text  string       `json:"text"`
				Files []PromptFile `json:"files"`
			}
			if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
				t.Fatal(err)
			}
			if body.Text != "inspect" || len(body.Files) != 2 || body.Files[0].URI != "data:text/plain;base64,aGk=" || body.Files[1].URI != "file:///repo/README.md" {
				t.Errorf("prompt body = %#v", body)
			}
			w.Write([]byte(`{"data":{}}`))
		case r.Method == http.MethodGet && r.URL.Path == "/api/reference":
			if got := r.URL.Query().Get("location[directory]"); got != "/repo" {
				t.Errorf("reference location = %q", got)
			}
			w.Write([]byte(`{"data":[{"name":"README","path":"README.md","description":"intro","source":{"type":"local","path":"README.md"}}]}`))
		case r.Method == http.MethodGet && r.URL.Path == "/api/session/ses_1/message/msg_1":
			w.Write([]byte(`{"data":{"id":"msg_1","type":"user","text":"inspect","time":{"created":1},"files":[{"data":"aGk=","mime":"text/plain","name":"note.txt","source":{"type":"inline"}}]}}`))
		default:
			t.Errorf("unexpected request %s %s", r.Method, r.URL.Path)
		}
	})
	files := []PromptFile{{URI: "data:text/plain;base64,aGk=", Name: "note.txt"}, {URI: "file:///repo/README.md", Name: "README"}}
	if err := c.PromptWithFiles(context.Background(), "ses_1", "inspect", files); err != nil {
		t.Fatal(err)
	}
	refs, err := c.ListReferencesFor(context.Background(), "/repo")
	if err != nil || len(refs) != 1 || refs[0].Path != "README.md" || !strings.Contains(string(refs[0].Source), `"local"`) {
		t.Fatalf("refs = %#v, err = %v", refs, err)
	}
	message, err := c.GetMessage(context.Background(), "ses_1", "msg_1")
	if err != nil || len(message.Files) != 1 || message.Files[0].Source.Type != "inline" || message.Files[0].Data != "aGk=" {
		t.Fatalf("message = %#v, err = %v", message, err)
	}
	if requests != 3 {
		t.Fatalf("requests = %d", requests)
	}
}

func TestSessionControlContractsAndSkillCapability(t *testing.T) {
	seen := map[string]map[string]any{}
	c, _ := fakeServer(t, "opencode", "pw", func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/openapi.json" {
			w.Write([]byte(`{"paths":{"/api/experimental/session/{sessionID}/skill":{"post":{}}}}`))
			return
		}
		if r.Method == http.MethodGet {
			switch r.URL.Path {
			case "/api/command":
				w.Write([]byte(`{"data":[{"name":"review","description":"Review","template":"not decoded"}]}`))
				return
			case "/api/skill":
				w.Write([]byte(`{"data":[{"id":"report","name":"Report","description":"File issue","location":"/secret","content":"secret"}]}`))
				return
			}
		}
		var body map[string]any
		_ = json.NewDecoder(r.Body).Decode(&body)
		seen[r.URL.Path] = body
		if strings.HasSuffix(r.URL.Path, "/command") || strings.HasSuffix(r.URL.Path, "/prompt") {
			w.Write([]byte(`{"data":{}}`))
			return
		}
		w.WriteHeader(http.StatusNoContent)
	})
	ctx := context.Background()
	commands, err := c.ListCommandsFor(ctx, "/repo")
	if err != nil || len(commands) != 1 || commands[0].Name != "review" {
		t.Fatalf("commands = %#v, %v", commands, err)
	}
	skills, err := c.ListSkillsFor(ctx, "/repo")
	if err != nil || len(skills) != 1 || skills[0].ID != "report" {
		t.Fatalf("skills = %#v, %v", skills, err)
	}
	if err := c.SwitchAgent(ctx, "ses_1", "build"); err != nil {
		t.Fatal(err)
	}
	if err := c.SwitchModel(ctx, "ses_1", ModelRef{ProviderID: "p", ID: "m"}); err != nil {
		t.Fatal(err)
	}
	if err := c.RunCommand(ctx, "ses_1", "review", "now"); err != nil {
		t.Fatal(err)
	}
	if err := c.PromptWithFilesAndSkills(ctx, "ses_1", "go", nil, []string{"report"}); err != nil {
		t.Fatal(err)
	}
	route, ok, err := c.StandaloneSkillRoute(ctx)
	if err != nil || !ok || route != "/api/experimental/session/{sessionID}/skill" {
		t.Fatalf("route = %q %v %v", route, ok, err)
	}
	if err := c.ActivateSkill(ctx, route, "ses_1", "report"); err != nil {
		t.Fatal(err)
	}
	if seen["/api/session/ses_1/agent"]["agent"] != "build" {
		t.Errorf("agent = %#v", seen)
	}
	model := seen["/api/session/ses_1/model"]["model"].(map[string]any)
	if model["providerID"] != "p" || model["id"] != "m" {
		t.Errorf("model = %#v", model)
	}
	if seen["/api/session/ses_1/command"]["arguments"] != "now" {
		t.Errorf("command = %#v", seen)
	}
	promptSkills := seen["/api/session/ses_1/prompt"]["skills"].([]any)
	if promptSkills[0].(map[string]any)["id"] != "report" {
		t.Errorf("prompt = %#v", seen)
	}
	if seen["/api/experimental/session/ses_1/skill"]["skill"] != "report" {
		t.Errorf("activation = %#v", seen)
	}
}

func TestStandaloneSkillCapabilityUnavailableAndRejectsUnknownRoute(t *testing.T) {
	c, _ := fakeServer(t, "opencode", "pw", func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte(`{"paths":{"/api/session/{sessionID}/skill":{"get":{}}}}`))
	})
	if route, ok, err := c.StandaloneSkillRoute(context.Background()); err != nil || ok || route != "" {
		t.Fatalf("route = %q %v %v", route, ok, err)
	}
	if err := c.ActivateSkill(context.Background(), "/api/other", "ses_1", "x"); err == nil {
		t.Fatal("expected unsupported route error")
	}
}

func TestGetSessionDiffContract(t *testing.T) {
	c, _ := fakeServer(t, "opencode", "pw", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet || r.URL.Path != "/api/session/ses_1/diff" {
			t.Errorf("request = %s %s", r.Method, r.URL.Path)
		}
		if r.URL.Query().Get("from") != "msg_from" || r.URL.Query().Get("to") != "msg_to" || r.URL.Query().Get("context") != "3" {
			t.Errorf("query = %s", r.URL.RawQuery)
		}
		w.Write([]byte(`{"data":[{"file":"a.go","patch":"@@ -1 +1 @@\n-old\n+new\n","additions":1,"deletions":1,"status":"modified"}]}`))
	})
	diffs, err := c.GetSessionDiff(context.Background(), "ses_1", "msg_from", "msg_to", 3)
	if err != nil || len(diffs) != 1 || diffs[0].File != "a.go" || diffs[0].Additions != 1 || diffs[0].Status != "modified" {
		t.Fatalf("diffs = %#v, err = %v", diffs, err)
	}
}

func TestListMessagesDecodesCommonAndUnknownParts(t *testing.T) {
	c, _ := fakeServer(t, "opencode", "pw", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet || r.URL.Path != "/api/session/ses_1/message" {
			t.Errorf("request = %s %s", r.Method, r.URL.Path)
		}
		if r.URL.RawQuery != "" {
			t.Errorf("compatibility query = %s", r.URL.RawQuery)
		}
		w.Write([]byte(`{"data":[
			{"id":"msg_user","type":"user","time":{"created":1},"text":"hello"},
			{"id":"msg_assistant","type":"assistant","time":{"created":2,"completed":3},"agent":"build","model":{"providerID":"p","id":"m"},"content":[
				{"type":"text","text":"answer","state":{"ignored":true}},
				{"type":"reasoning","text":"thinking","time":{"created":2}},
				{"type":"tool","id":"call_1","name":"read","state":{"status":"completed","input":{"path":"x"},"content":[{"type":"text","text":"contents"},{"type":"future-output","value":7}]},"time":{"created":2,"completed":3}},
				{"type":"future-part","payload":"kept"}
			],"error":{"type":"ProviderError","message":"display me","status":500}},
			{"id":"msg_future","type":"future-message","time":{"created":4},"payload":"kept"}
		],"cursor":{"previous":null,"next":null}}`))
	})
	messages, err := c.ListMessages(context.Background(), "ses_1")
	if err != nil {
		t.Fatalf("ListMessages: %v", err)
	}
	if len(messages) != 3 || messages[0].Type != "user" || messages[0].Text != "hello" {
		t.Fatalf("messages = %#v", messages)
	}
	assistant := messages[1]
	if assistant.Agent != "build" || assistant.Model == nil || assistant.Model.ProviderID != "p" || assistant.Error.Message != "display me" {
		t.Fatalf("assistant = %#v", assistant)
	}
	if len(assistant.Content) != 4 {
		t.Fatalf("content count = %d", len(assistant.Content))
	}
	text, ok := assistant.Content[0].(TextPart)
	if !ok || text.Text != "answer" || !strings.Contains(string(text.Raw), `"ignored":true`) {
		t.Fatalf("text part = %T %#v", assistant.Content[0], assistant.Content[0])
	}
	reasoning, ok := assistant.Content[1].(ReasoningPart)
	if !ok || reasoning.Text != "thinking" || reasoning.Time.Created != 2 {
		t.Fatalf("reasoning part = %T %#v", assistant.Content[1], assistant.Content[1])
	}
	tool, ok := assistant.Content[2].(ToolPart)
	if !ok || tool.Name != "read" || tool.State.Status != "completed" || len(tool.State.Content) != 2 || tool.State.Content[0].Text != "contents" {
		t.Fatalf("tool part = %T %#v", assistant.Content[2], assistant.Content[2])
	}
	if tool.State.Content[1].Type != "future-output" || !strings.Contains(string(tool.State.Content[1].Raw), `"value":7`) {
		t.Fatalf("unknown tool content = %#v", tool.State.Content[1])
	}
	unknown, ok := assistant.Content[3].(UnknownPart)
	if !ok || unknown.PartType() != "future-part" || !strings.Contains(string(unknown.PartRaw()), `"payload":"kept"`) {
		t.Fatalf("unknown part = %T %#v", assistant.Content[3], assistant.Content[3])
	}
	if messages[2].Type != "future-message" || !strings.Contains(string(messages[2].Raw), `"payload":"kept"`) {
		t.Fatalf("unknown message = %#v", messages[2])
	}
}

func TestListMessagesEmpty(t *testing.T) {
	c, _ := fakeServer(t, "opencode", "pw", func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte(`{"data":[],"cursor":{}}`))
	})
	messages, err := c.ListMessages(context.Background(), "ses_empty")
	if err != nil || len(messages) != 0 {
		t.Fatalf("messages = %#v, err = %v", messages, err)
	}
}

func TestListMessagesPagePreservesCursorAndQuery(t *testing.T) {
	var queries []url.Values
	c, _ := fakeServer(t, "opencode", "pw", func(w http.ResponseWriter, r *http.Request) {
		queries = append(queries, r.URL.Query())
		if r.URL.Query().Get("cursor") == "older/+ token" {
			w.Write([]byte(`{"data":[{"id":"msg_old","type":"user","text":"old","time":{"created":1}}],"cursor":{"previous":"newer","next":null}}`))
			return
		}
		w.Write([]byte(`{"data":[{"id":"msg_new","type":"user","text":"new","time":{"created":2}}],"cursor":{"previous":null,"next":"older/+ token"}}`))
	})

	page, err := c.ListMessagesPage(context.Background(), "ses_1", ListMessagesOptions{Limit: 50, Order: "desc"})
	if err != nil || len(page.Messages) != 1 || page.Cursor.Next != "older/+ token" {
		t.Fatalf("newest page = %#v, err = %v", page, err)
	}
	page, err = c.ListMessagesPage(context.Background(), "ses_1", ListMessagesOptions{Limit: 50, Order: "desc", Cursor: page.Cursor.Next})
	if err != nil || page.Messages[0].ID != "msg_old" || page.Cursor.Previous != "newer" {
		t.Fatalf("older page = %#v, err = %v", page, err)
	}
	if len(queries) != 2 || queries[0].Get("limit") != "50" || queries[0].Get("order") != "desc" || queries[0].Has("cursor") {
		t.Fatalf("initial query = %#v", queries)
	}
	if queries[1].Get("limit") != "50" || queries[1].Get("cursor") != "older/+ token" || queries[1].Has("order") {
		t.Fatalf("cursor query = %#v", queries[1])
	}
}

func TestListMessagesMalformedResponses(t *testing.T) {
	for _, tc := range []struct {
		name string
		body string
	}{
		{name: "missing envelope", body: `{"cursor":{}}`},
		{name: "wrong data shape", body: `{"data":{}}`},
		{name: "malformed known part", body: `{"data":[{"id":"msg_1","type":"assistant","time":{"created":1},"content":[{"type":"text","text":9}]}]}`},
	} {
		t.Run(tc.name, func(t *testing.T) {
			c, _ := fakeServer(t, "opencode", "pw", func(w http.ResponseWriter, r *http.Request) {
				w.Write([]byte(tc.body))
			})
			if _, err := c.ListMessages(context.Background(), "ses_1"); err == nil {
				t.Fatal("expected decode error")
			}
		})
	}
}

func TestChatSessionControls(t *testing.T) {
	c, _ := fakeServer(t, "opencode", "pw", func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.Method == http.MethodGet && r.URL.Path == "/api/session/active":
			w.Write([]byte(`{"data":{"ses_busy":{"type":"running"}}}`))
		case r.Method == http.MethodPost && r.URL.Path == "/api/session/ses_busy/interrupt":
			if r.ContentLength != 0 {
				t.Errorf("interrupt content length = %d", r.ContentLength)
			}
			w.Write([]byte(`{"data":{"interrupted":true}}`))
		case r.Method == http.MethodPost && r.URL.Path == "/api/session/ses_idle/interrupt":
			w.Write([]byte(`{"data":{"interrupted":false}}`))
		default:
			t.Errorf("unexpected request %s %s", r.Method, r.URL.Path)
		}
	})
	active, err := c.ListActiveSessions(context.Background())
	if err != nil || active["ses_busy"].Type != "running" {
		t.Fatalf("active = %#v, err = %v", active, err)
	}
	interrupted, err := c.Interrupt(context.Background(), "ses_busy")
	if err != nil || !interrupted {
		t.Fatalf("busy interrupt = %v, %v", interrupted, err)
	}
	interrupted, err = c.Interrupt(context.Background(), "ses_idle")
	if err != nil || interrupted {
		t.Fatalf("idle interrupt = %v, %v", interrupted, err)
	}
}

func TestChatStateListsEmpty(t *testing.T) {
	c, _ := fakeServer(t, "opencode", "pw", func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/api/session/active":
			w.Write([]byte(`{"data":{}}`))
		case "/api/session/ses_1/permission", "/api/session/ses_1/form":
			w.Write([]byte(`{"data":[]}`))
		default:
			t.Errorf("unexpected path %s", r.URL.Path)
		}
	})
	active, err := c.ListActiveSessions(context.Background())
	if err != nil || len(active) != 0 {
		t.Fatalf("active = %#v, err = %v", active, err)
	}
	permissions, err := c.ListPermissions(context.Background(), "ses_1")
	if err != nil || len(permissions) != 0 {
		t.Fatalf("permissions = %#v, err = %v", permissions, err)
	}
	forms, err := c.ListForms(context.Background(), "ses_1")
	if err != nil || len(forms) != 0 {
		t.Fatalf("forms = %#v, err = %v", forms, err)
	}
}

func TestChatStateMalformedData(t *testing.T) {
	for name, call := range map[string]func(*Client) error{
		"active": func(c *Client) error {
			_, err := c.ListActiveSessions(context.Background())
			return err
		},
		"interrupt": func(c *Client) error {
			_, err := c.Interrupt(context.Background(), "ses_1")
			return err
		},
		"permissions": func(c *Client) error {
			_, err := c.ListPermissions(context.Background(), "ses_1")
			return err
		},
		"forms": func(c *Client) error {
			_, err := c.ListForms(context.Background(), "ses_1")
			return err
		},
	} {
		t.Run(name, func(t *testing.T) {
			c, _ := fakeServer(t, "opencode", "pw", func(w http.ResponseWriter, r *http.Request) {
				w.Write([]byte(`{"data":"wrong shape"}`))
			})
			if err := call(c); err == nil {
				t.Fatal("expected decode error")
			}
		})
	}
}

func TestPermissionListAndReply(t *testing.T) {
	var replies int
	c, _ := fakeServer(t, "opencode", "pw", func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.Method == http.MethodGet && r.URL.Path == "/api/session/ses_1/permission":
			w.Write([]byte(`{"data":[{"id":"per_1","sessionID":"ses_1","action":"read","resources":["/repo/a"],"save":["/repo/*"],"metadata":{"risk":"low"},"source":{"type":"tool","messageID":"msg_1","id":"call_1"},"message":"Allow read?"}]}`))
		case r.Method == http.MethodPost && r.URL.Path == "/api/session/ses_1/permission/per_1/reply":
			replies++
			var body map[string]any
			if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
				t.Fatal(err)
			}
			if body["decision"] != "always" || body["message"] != "trusted" {
				t.Errorf("reply body = %#v", body)
			}
			w.WriteHeader(http.StatusNoContent)
		default:
			t.Errorf("unexpected request %s %s", r.Method, r.URL.Path)
		}
	})
	requests, err := c.ListPermissions(context.Background(), "ses_1")
	if err != nil || len(requests) != 1 || requests[0].Source.MessageID != "msg_1" || requests[0].Save[0] != "/repo/*" {
		t.Fatalf("permissions = %#v, err = %v", requests, err)
	}
	if err := c.ReplyPermission(context.Background(), "ses_1", "per_1", PermissionAlways, "trusted"); err != nil {
		t.Fatalf("ReplyPermission: %v", err)
	}
	if replies != 1 {
		t.Fatalf("replies = %d", replies)
	}
}

func TestFormListAndReply(t *testing.T) {
	c, _ := fakeServer(t, "opencode", "pw", func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.Method == http.MethodGet && r.URL.Path == "/api/session/ses_1/form":
			w.Write([]byte(`{"data":[{"id":"frm_1","sessionID":"ses_1","title":"Deploy","metadata":{"source":"tool"},"fields":[{"key":"environment","type":"string","title":"Environment","required":true,"default":"staging","options":[{"value":"prod","label":"Production"}]},{"key":"confirm","type":"boolean","default":false},{"key":"docs","type":"external","url":"https://example.test"}]}]}`))
		case r.Method == http.MethodPost && r.URL.Path == "/api/session/ses_1/form/frm_1/reply":
			var body struct {
				Answer map[string]any `json:"answer"`
			}
			if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
				t.Fatal(err)
			}
			if body.Answer["environment"] != "prod" || body.Answer["confirm"] != true {
				t.Errorf("reply body = %#v", body)
			}
			w.WriteHeader(http.StatusNoContent)
		default:
			t.Errorf("unexpected request %s %s", r.Method, r.URL.Path)
		}
	})
	forms, err := c.ListForms(context.Background(), "ses_1")
	if err != nil || len(forms) != 1 || forms[0].Title != "Deploy" || len(forms[0].Fields) != 3 {
		t.Fatalf("forms = %#v, err = %v", forms, err)
	}
	field := forms[0].Fields[0]
	if field.Type != "string" || !field.Required || string(field.Default) != `"staging"` || field.Options[0].Label != "Production" {
		t.Fatalf("field = %#v", field)
	}
	if err := c.ReplyForm(context.Background(), "ses_1", "frm_1", FormAnswer{"environment": "prod", "confirm": true}); err != nil {
		t.Fatalf("ReplyForm: %v", err)
	}
}

func TestChatMethodsPropagateAPIError(t *testing.T) {
	c, _ := fakeServer(t, "opencode", "pw", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNotFound)
		w.Write([]byte(`{"_tag":"SessionNotFoundError","message":"gone"}`))
	})
	for name, call := range map[string]func() error{
		"messages":    func() error { _, err := c.ListMessages(context.Background(), "ses_x"); return err },
		"diff":        func() error { _, err := c.GetSessionDiff(context.Background(), "ses_x", "msg_x", "", 3); return err },
		"active":      func() error { _, err := c.ListActiveSessions(context.Background()); return err },
		"interrupt":   func() error { _, err := c.Interrupt(context.Background(), "ses_x"); return err },
		"permissions": func() error { _, err := c.ListPermissions(context.Background(), "ses_x"); return err },
		"permission reply": func() error {
			return c.ReplyPermission(context.Background(), "ses_x", "per_x", PermissionReject, "")
		},
		"forms": func() error { _, err := c.ListForms(context.Background(), "ses_x"); return err },
		"form reply": func() error {
			return c.ReplyForm(context.Background(), "ses_x", "frm_x", FormAnswer{})
		},
	} {
		t.Run(name, func(t *testing.T) {
			var apiErr *APIError
			if err := call(); !errors.As(err, &apiErr) || apiErr.StatusCode != http.StatusNotFound || apiErr.Tag != "SessionNotFoundError" {
				t.Fatalf("error = %T %v", err, err)
			}
		})
	}
}

func TestGetSessionContextPathAndAssistantTokenUsage(t *testing.T) {
	c, _ := fakeServer(t, "opencode", "pw", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet || r.URL.Path != "/api/session/ses_context/context" {
			t.Fatalf("request = %s %s", r.Method, r.URL.Path)
		}
		w.Write([]byte(`{"data":[{"id":"msg_user","type":"user","text":"hello"},{"id":"msg_assistant","type":"assistant","tokens":{"input":120,"output":30,"reasoning":4,"cache":{"read":200,"write":50}},"content":[]}]}`))
	})
	messages, err := c.GetSessionContext(context.Background(), "ses_context")
	if err != nil {
		t.Fatalf("GetSessionContext: %v", err)
	}
	if len(messages) != 2 || messages[1].Tokens == nil {
		t.Fatalf("messages = %#v", messages)
	}
	usage := messages[1].Tokens
	if usage.Input != 120 || usage.Output != 30 || usage.Reasoning != 4 || usage.Cache.Read != 200 || usage.Cache.Write != 50 {
		t.Fatalf("usage = %+v", usage)
	}
}

func TestAPIError(t *testing.T) {
	c, _ := fakeServer(t, "opencode", "pw", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusForbidden)
		w.Write([]byte(`{"_tag":"ForbiddenError","message":"nope"}`))
	})
	_, err := c.GetSession(context.Background(), "ses_x")
	aerr, ok := err.(*APIError)
	if !ok {
		t.Fatalf("err = %T %v", err, err)
	}
	if aerr.StatusCode != 403 || aerr.Tag != "ForbiddenError" || aerr.Message != "nope" {
		t.Fatalf("aerr = %+v", aerr)
	}
}

func TestParseServiceURL(t *testing.T) {
	for in, want := range map[string]string{
		"http://127.0.0.1:49374\n":          "http://127.0.0.1:49374",
		"running at http://localhost:8080 ": "http://localhost:8080",
		"nothing here":                      "",
	} {
		if got := parseServiceURL(in); got != want {
			t.Errorf("parseServiceURL(%q) = %q, want %q", in, got, want)
		}
	}
}

func TestPasswordFromFile(t *testing.T) {
	dir := t.TempDir()
	p := filepath.Join(dir, "service.json")
	if err := os.WriteFile(p, []byte(`{"password":"s3cret"}`), 0o600); err != nil {
		t.Fatal(err)
	}
	pw, err := PasswordFromFile(p)
	if err != nil || pw != "s3cret" {
		t.Fatalf("pw = %q, %v", pw, err)
	}
	if _, err := PasswordFromFile(filepath.Join(dir, "missing.json")); err == nil {
		t.Fatal("expected error for missing file")
	}
	os.WriteFile(p, []byte(`{}`), 0o600)
	if _, err := PasswordFromFile(p); err == nil {
		t.Fatal("expected error for missing password key")
	}
}

// TestLiveSmoke exercises discovery and a create/delete cycle against the
// real service. Only runs with TT_LIVE_OPENCODE=1.
func TestLiveSmoke(t *testing.T) {
	if os.Getenv("TT_LIVE_OPENCODE") != "1" {
		t.Skip("set TT_LIVE_OPENCODE=1 to run live smoke test")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	defer cancel()
	c, err := DiscoverClient(ctx)
	if err != nil {
		t.Fatalf("DiscoverClient: %v", err)
	}
	s, err := c.CreateSession(ctx, "opencode-client live smoke (throwaway)", "/tmp")
	if err != nil {
		t.Fatalf("CreateSession: %v", err)
	}
	if err := c.RenameSession(ctx, s.ID, "renamed"); err != nil {
		t.Fatalf("RenameSession: %v", err)
	}
	if err := c.DeleteSession(ctx, s.ID); err != nil {
		t.Fatalf("DeleteSession: %v", err)
	}
}

func TestWaitDoneRetriesTransportTimeout(t *testing.T) {
	var calls int
	c, _ := fakeServer(t, "opencode", "secret", func(w http.ResponseWriter, r *http.Request) {
		calls++
		if calls < 3 {
			time.Sleep(300 * time.Millisecond) // forces transport timeout
		}
		w.WriteHeader(http.StatusNoContent)
	})
	c.hc.Timeout = 100 * time.Millisecond
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	if err := c.WaitDone(ctx, "ses_x"); err != nil {
		t.Fatalf("WaitDone: %v", err)
	}
	if calls < 2 {
		t.Errorf("expected retries after transport timeout, got %d call(s)", calls)
	}
}

func TestWaitDoneCtxExpiryReportsBusy(t *testing.T) {
	c, _ := fakeServer(t, "opencode", "secret", func(w http.ResponseWriter, r *http.Request) {
		time.Sleep(300 * time.Millisecond) // session never goes idle in time
		w.WriteHeader(http.StatusNoContent)
	})
	c.hc.Timeout = 100 * time.Millisecond
	ctx, cancel := context.WithTimeout(context.Background(), 400*time.Millisecond)
	defer cancel()
	if err := c.WaitDone(ctx, "ses_x"); err == nil {
		t.Fatal("expected error when ctx expires before the session idles")
	}
}
