package server

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"lessmess/internal/opencode"
)

func statusUpstream(t *testing.T, providerCode int) *httptest.Server {
	t.Helper()
	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if _, p, ok := r.BasicAuth(); !ok || p != "pw" {
			w.WriteHeader(http.StatusUnauthorized)
			return
		}
		switch r.URL.Path {
		case "/api/info":
			w.Write([]byte(`{"version":"2.0.6","pid":99,"urls":["canary-url"],"paths":{"tmp":"/private/canary"}}`))
		case "/api/location":
			w.Write([]byte(`{"directory":"` + strings.ReplaceAll(r.URL.Query().Get("location[directory]"), `\`, `\\`) + `","project":{"id":"project-1","directory":"/private/project","canonical":"secret"}}`))
		case "/api/provider":
			if providerCode != 0 {
				w.WriteHeader(providerCode)
				w.Write([]byte(`{"message":"raw secret canary"}`))
				return
			}
			w.Write([]byte(`{"data":[{"id":"p","name":"Provider","activation":"enabled","headers":{"x":"secret canary"}}]}`))
		case "/api/model":
			w.Write([]byte(`{"data":[{"id":"m","providerID":"p","name":"Model","enabled":true,"status":"future","headers":{"x":"secret"}}]}`))
		case "/api/model/default":
			w.Write([]byte(`{"data":null}`))
		case "/api/plugin":
			w.Write([]byte(`{"data":[{"id":"plug","source":{"type":"local","path":"/private/canary"},"state":{"status":"failed","error":"raw secret"}}]}`))
		case "/openapi.json":
			w.Write([]byte(`{"paths":{"/api/info":{"get":{}},"/api/provider":{"get":{}},"/api/model":{"get":{}},"/api/model/default":{"get":{}},"/api/plugin":{"get":{}},"/api/location":{"get":{}}}}`))
		default:
			w.WriteHeader(http.StatusNotFound)
		}
	}))
}

func TestOpencodeStatusPartialAndRedacted(t *testing.T) {
	st, _ := fixtureStore(t)
	upstream := statusUpstream(t, http.StatusServiceUnavailable)
	defer upstream.Close()
	s := New(st)
	s.SetOpencode(opencode.New(upstream.URL, "pw"))
	w := do(t, s.Handler(), "GET", "/api/opencode/status", "")
	if w.Code != http.StatusOK {
		t.Fatalf("code = %d: %s", w.Code, w.Body)
	}
	for key, want := range map[string]string{"Cache-Control": "no-store", "X-Content-Type-Options": "nosniff", "Referrer-Policy": "no-referrer"} {
		if got := w.Header().Get(key); got != want {
			t.Errorf("%s = %q", key, got)
		}
	}
	var got opencodeStatusResponse
	if err := json.Unmarshal(w.Body.Bytes(), &got); err != nil {
		t.Fatal(err)
	}
	if got.State != "partial" || got.Service.State != "healthy" || got.Providers.Failure != "upstream_unavailable" || got.Models.State != "available" || got.ModelList[0].Status != "unknown" || got.DefaultModel != nil {
		t.Fatalf("status = %#v", got)
	}
	for _, forbidden := range []string{"raw secret", "canary-url", "/private/canary", "canonical", "pid"} {
		if strings.Contains(w.Body.String(), forbidden) {
			t.Fatalf("response leaked %q: %s", forbidden, w.Body)
		}
	}
}

func TestOpencodeStatusUnavailableAndPageLinks(t *testing.T) {
	st, _ := fixtureStore(t)
	s := New(st)
	w := do(t, s.Handler(), "GET", "/api/opencode/status", "")
	if w.Code != 200 || !strings.Contains(w.Body.String(), `"state":"degraded"`) {
		t.Fatalf("response = %d %s", w.Code, w.Body)
	}
	page := htmlGet(t, s.Handler(), "/settings/opencode", false)
	if page.Header().Get("Cache-Control") != "no-store" || !strings.Contains(page.Body.String(), `id="opencode-status-page"`) {
		t.Fatalf("page missing status contract: %s", page.Body)
	}
	for _, id := range []string{`id="oc-mcp-list"`, `id="oc-mcp-resources"`, `id="oc-active-permissions"`, `id="oc-saved-permissions"`, `href="#oc-status-section"`, `href="#oc-connections-section"`, `id="oc-return-chat"`, `Public hosting is unsupported`} {
		if !strings.Contains(page.Body.String(), id) {
			t.Fatalf("management page missing %s", id)
		}
	}
	if !strings.Contains(htmlGet(t, s.Handler(), "/settings", false).Body.String(), `href="/settings/opencode"`) {
		t.Fatal("settings status link missing")
	}
	asset := do(t, s.Handler(), "GET", "/static/app.js", "").Body.String()
	if !strings.Contains(asset, "View service status") || !strings.Contains(asset, `href = "/settings/opencode"`) {
		t.Fatal("chat service-failure link missing")
	}
}

func TestManagementGeneratedErrorsHaveSecurityHeaders(t *testing.T) {
	st, _ := fixtureStore(t)
	s := New(st)
	for _, tc := range []struct {
		method string
		path   string
		code   int
	}{
		{http.MethodPut, "/api/opencode/status", http.StatusMethodNotAllowed},
		{http.MethodGet, "/api/opencode/not-a-route", http.StatusNotFound},
		{http.MethodPost, "/settings/opencode", http.StatusMethodNotAllowed},
	} {
		w := do(t, s.Handler(), tc.method, tc.path, "")
		if w.Code != tc.code {
			t.Fatalf("%s %s = %d", tc.method, tc.path, w.Code)
		}
		for key, want := range map[string]string{"Cache-Control": "no-store", "Pragma": "no-cache", "X-Content-Type-Options": "nosniff", "Referrer-Policy": "no-referrer"} {
			if got := w.Header().Get(key); got != want {
				t.Errorf("%s %s: %s = %q", tc.method, tc.path, key, got)
			}
		}
	}
}

func TestManagementOriginAndIdentifierValidation(t *testing.T) {
	st, _ := fixtureStore(t)
	s := New(st)
	for _, origin := range []string{"null", "http://example.com/path", "http://user@example.com", "https://example.com"} {
		r := httptest.NewRequest(http.MethodPost, "/api/opencode/rediscover", nil)
		r.Host = "example.com"
		r.Header.Set("X-Lessmess-UI", "1")
		r.Header.Set("Origin", origin)
		w := httptest.NewRecorder()
		s.Handler().ServeHTTP(w, r)
		if w.Code != http.StatusForbidden {
			t.Errorf("origin %q = %d", origin, w.Code)
		}
	}

	upstreamCalls := 0
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { upstreamCalls++; w.WriteHeader(http.StatusNoContent) }))
	defer upstream.Close()
	s.SetOpencode(opencode.New(upstream.URL, "pw"))
	w := integrationRequest(t, s.Handler(), http.MethodPost, "/api/opencode/mcp/bad%5Cname/connect", `{"expectedStatus":"failed"}`)
	if w.Code != http.StatusBadRequest || upstreamCalls != 0 {
		t.Fatalf("unsafe id = %d calls=%d body=%s", w.Code, upstreamCalls, w.Body)
	}
}

func TestManagementDoesNotSerializeIntoSettings(t *testing.T) {
	st, dir := fixtureStore(t)
	settingsPath := filepath.Join(dir, "lessmess.json")
	want := []byte("{\n  \"general\": {\"projectName\": \"Isolation canary\"}\n}\n")
	if err := os.WriteFile(settingsPath, want, 0o644); err != nil {
		t.Fatal(err)
	}
	s := New(st)
	status := do(t, s.Handler(), http.MethodGet, "/api/opencode/status", "")
	settings := do(t, s.Handler(), http.MethodGet, "/api/settings", "")
	got, err := os.ReadFile(settingsPath)
	if err != nil {
		t.Fatal(err)
	}
	if status.Code != http.StatusOK || settings.Code != http.StatusOK || string(got) != string(want) {
		t.Fatalf("status/settings = %d/%d settings=%q", status.Code, settings.Code, got)
	}
	for _, forbidden := range []string{"providerList", "capabilities", "integrations", "runtimeNotice", "savedAvailable"} {
		if strings.Contains(settings.Body.String(), forbidden) {
			t.Fatalf("settings payload contains management field %q: %s", forbidden, settings.Body)
		}
	}
}

func TestOpencodeRediscoverRequiresSameOriginUI(t *testing.T) {
	st, _ := fixtureStore(t)
	s := New(st)
	for _, mutate := range []func(*http.Request){func(r *http.Request) {}, func(r *http.Request) {
		r.Header.Set("X-Lessmess-UI", "1")
		r.Header.Set("Origin", "https://evil.example")
	}} {
		r := httptest.NewRequest(http.MethodPost, "/api/opencode/rediscover", nil)
		mutate(r)
		w := httptest.NewRecorder()
		s.Handler().ServeHTTP(w, r)
		if w.Code != http.StatusForbidden || w.Header().Get("Cache-Control") != "no-store" {
			t.Fatalf("code = %d headers = %v", w.Code, w.Header())
		}
	}
}
