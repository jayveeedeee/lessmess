package server

import (
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"sync"
	"testing"

	"lessmess/internal/opencode"
)

const managementSpec = `{"paths":{"/api/location":{"get":{}},"/api/mcp":{"get":{}},"/api/mcp/resource":{"get":{}},"/api/experimental/mcp/{server}/connect":{"post":{}},"/api/experimental/mcp/{server}/disconnect":{"post":{}},"/api/permission/request":{"get":{}},"/api/permission/saved":{"get":{}},"/api/permission/saved/{id}":{"delete":{}}}}`

func TestMCPOverviewNeedsAuthFailureResourcesAndHostileLabels(t *testing.T) {
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/openapi.json":
			w.Write([]byte(managementSpec))
		case "/api/mcp":
			w.Write([]byte(`{"data":[{"name":"<svg onload=alert(1)>","status":{"status":"failed","error":"Authorization secret /private/key"}},{"name":"auth","status":{"status":"needs_auth","error":"token=secret"},"integrationID":"known"},{"name":"cli-auth","status":{"status":"needs_auth"},"integrationID":"missing"}]}`))
		case "/api/mcp/resource":
			w.Write([]byte(`{"data":{"resources":[{"server":"bad","name":"<b>resource</b>","uri":"https://user:pass@example.test/private?token=secret"}],"templates":[]}}`))
		case "/api/integration":
			w.Write([]byte(`{"data":[{"id":"known","name":"Known","methods":[],"connections":[]}]}`))
		default:
			w.WriteHeader(http.StatusNotFound)
		}
	}))
	defer upstream.Close()
	st, _ := fixtureStore(t)
	s := New(st)
	s.SetOpencode(opencode.New(upstream.URL, "pw"))
	w := do(t, s.Handler(), http.MethodGet, "/api/opencode/mcp", "")
	if w.Code != http.StatusOK || !strings.Contains(w.Body.String(), `"authIntegrationID":"known"`) || strings.Contains(w.Body.String(), `"authIntegrationID":"missing"`) {
		t.Fatalf("overview = %d %s", w.Code, w.Body)
	}
	for _, forbidden := range []string{"Authorization", "user:pass", "token=", "/private", "secret"} {
		if strings.Contains(w.Body.String(), forbidden) {
			t.Fatalf("response leaked %q: %s", forbidden, w.Body)
		}
	}
	if !strings.Contains(w.Body.String(), `\u003csvg`) || !strings.Contains(w.Body.String(), `\u003cb`) || !strings.Contains(w.Body.String(), "may not survive") {
		t.Fatalf("hostile labels or runtime notice missing: %s", w.Body)
	}
}

func TestMCPUnavailableAndFailedReconnect(t *testing.T) {
	t.Run("unavailable", func(t *testing.T) {
		upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { w.Write([]byte(`{"paths":{}}`)) }))
		defer upstream.Close()
		st, _ := fixtureStore(t)
		s := New(st)
		s.SetOpencode(opencode.New(upstream.URL, "pw"))
		w := do(t, s.Handler(), http.MethodGet, "/api/opencode/mcp", "")
		if w.Code != http.StatusOK || !strings.Contains(w.Body.String(), `"available":false`) {
			t.Fatalf("unavailable = %d %s", w.Code, w.Body)
		}
	})
	t.Run("failed reconnect redacted", func(t *testing.T) {
		var mutationPath string
		upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			switch r.URL.Path {
			case "/openapi.json":
				w.Write([]byte(managementSpec))
			case "/api/mcp":
				w.Write([]byte(`{"data":[{"name":"broken","status":{"status":"failed","error":"secret"}}]}`))
			case "/api/experimental/mcp/broken/connect":
				mutationPath = r.URL.RequestURI()
				w.WriteHeader(http.StatusBadRequest)
				w.Write([]byte(`{"message":"header secret /private/key"}`))
			default:
				w.WriteHeader(http.StatusNotFound)
			}
		}))
		defer upstream.Close()
		st, dir := fixtureStore(t)
		s := New(st)
		s.SetOpencode(opencode.New(upstream.URL, "pw"))
		w := integrationRequest(t, s.Handler(), http.MethodPost, "/api/opencode/mcp/broken/connect", `{"expectedStatus":"failed"}`)
		if w.Code != http.StatusBadGateway || strings.Contains(w.Body.String(), "secret") || !strings.Contains(mutationPath, "location[directory]=") || !strings.Contains(mutationPath, url.QueryEscape(dir)) {
			t.Fatalf("reconnect = %d %s path=%q", w.Code, w.Body, mutationPath)
		}
	})
}

func TestManagementUnsupportedAndEmptyStates(t *testing.T) {
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/openapi.json" {
			w.Write([]byte(`{"paths":{}}`))
			return
		}
		w.WriteHeader(http.StatusNotFound)
	}))
	defer upstream.Close()
	st, _ := fixtureStore(t)
	s := New(st)
	s.SetOpencode(opencode.New(upstream.URL, "pw"))
	for path, marker := range map[string]string{
		"/api/opencode/integrations": `"available":false`,
		"/api/opencode/mcp":          `"available":false`,
		"/api/opencode/permissions":  `"activeAvailable":false`,
	} {
		w := do(t, s.Handler(), http.MethodGet, path, "")
		if w.Code != http.StatusOK || !strings.Contains(w.Body.String(), marker) {
			t.Errorf("%s = %d %s", path, w.Code, w.Body)
		}
	}
}

func TestPermissionOverviewMappingAndStaleRuleRemoval(t *testing.T) {
	var mu sync.Mutex
	rulePresent := true
	var removes int
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		mu.Lock()
		defer mu.Unlock()
		switch {
		case r.URL.Path == "/openapi.json":
			w.Write([]byte(managementSpec))
		case r.URL.Path == "/api/location":
			if r.URL.Query().Get("location[directory]") == "" {
				t.Error("location used OpenCode default")
			}
			w.Write([]byte(`{"directory":"/repo","project":{"id":"project-1","directory":"/repo","canonical":"/repo"}}`))
		case r.URL.Path == "/api/permission/request":
			w.Write([]byte(`{"data":[{"id":"per_known","sessionID":"ses_known","action":"shell","resources":["/private/one","/private/two"],"metadata":{"secret":"canary"}},{"id":"per_unknown","sessionID":"ses_unknown","action":"read","resources":[]}]}`))
		case r.URL.Path == "/api/permission/saved":
			if r.URL.Query().Get("projectID") != "project-1" {
				t.Errorf("projectID = %q", r.URL.Query().Get("projectID"))
			}
			if rulePresent {
				w.Write([]byte(`{"data":[{"id":"rule-1","projectID":"project-1","action":"shell","resource":"git status"}]}`))
			} else {
				w.Write([]byte(`{"data":[]}`))
			}
		case r.URL.Path == "/api/permission/saved/rule-1" && r.Method == http.MethodDelete:
			removes++
			rulePresent = false
			w.WriteHeader(http.StatusNoContent)
		default:
			w.WriteHeader(http.StatusNotFound)
		}
	}))
	defer upstream.Close()
	st, _ := fixtureStore(t)
	s := New(st)
	s.SetOpencode(opencode.New(upstream.URL, "pw"))
	if err := s.sessions.add("change", SessionEntry{Session: "ses_known", Title: "Known session"}); err != nil {
		t.Fatal(err)
	}
	w := do(t, s.Handler(), http.MethodGet, "/api/opencode/permissions", "")
	if w.Code != http.StatusOK || !strings.Contains(w.Body.String(), `"chat":true`) || !strings.Contains(w.Body.String(), `"chat":false`) || !strings.Contains(w.Body.String(), `"resourceCount":2`) {
		t.Fatalf("overview = %d %s", w.Code, w.Body)
	}
	for _, forbidden := range []string{"/private/one", "metadata", "canary"} {
		if strings.Contains(w.Body.String(), forbidden) {
			t.Fatalf("active details leaked %q: %s", forbidden, w.Body)
		}
	}
	stale := integrationRequest(t, s.Handler(), http.MethodDelete, "/api/opencode/permissions/saved/rule-1", `{"confirm":true,"expectedProject":"project-1","expectedAction":"shell","expectedResource":"old"}`)
	if stale.Code != http.StatusConflict || removes != 0 {
		t.Fatalf("stale = %d %s removes=%d", stale.Code, stale.Body, removes)
	}
	remove := integrationRequest(t, s.Handler(), http.MethodDelete, "/api/opencode/permissions/saved/rule-1", `{"confirm":true,"expectedProject":"project-1","expectedAction":"shell","expectedResource":"git status"}`)
	if remove.Code != http.StatusOK || removes != 1 {
		t.Fatalf("remove = %d %s removes=%d", remove.Code, remove.Body, removes)
	}
	again := integrationRequest(t, s.Handler(), http.MethodDelete, "/api/opencode/permissions/saved/rule-1", `{"confirm":true,"expectedProject":"project-1","expectedAction":"shell","expectedResource":"git status"}`)
	if again.Code != http.StatusConflict || removes != 1 {
		t.Fatalf("second = %d %s removes=%d", again.Code, again.Body, removes)
	}
}

func TestManagementMutationsUseStrictJSONAndSecurityGuards(t *testing.T) {
	upstreamCalls := 0
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		upstreamCalls++
		_, _ = io.Copy(io.Discard, r.Body)
		w.Write([]byte(managementSpec))
	}))
	defer upstream.Close()
	st, _ := fixtureStore(t)
	s := New(st)
	s.SetOpencode(opencode.New(upstream.URL, "pw"))
	unknown := integrationRequest(t, s.Handler(), http.MethodPost, "/api/opencode/mcp/s/connect", `{"expectedStatus":"failed","extra":true}`)
	if unknown.Code != http.StatusBadRequest || upstreamCalls != 0 {
		t.Fatalf("unknown = %d calls=%d", unknown.Code, upstreamCalls)
	}
	r := httptest.NewRequest(http.MethodDelete, "/api/opencode/permissions/saved/r", strings.NewReader(`{"confirm":true}`))
	r.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	s.Handler().ServeHTTP(w, r)
	if w.Code != http.StatusForbidden || w.Header().Get("Cache-Control") != "no-store" || upstreamCalls != 0 {
		t.Fatalf("guard = %d headers=%v calls=%d", w.Code, w.Header(), upstreamCalls)
	}
}
