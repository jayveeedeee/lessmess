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
