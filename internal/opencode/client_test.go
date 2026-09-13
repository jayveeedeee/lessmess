package opencode

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"net/http"
	"net/http/httptest"
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
		case r.Method == http.MethodPost && strings.HasSuffix(r.URL.Path, "/rename"):
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
