package server

import (
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"

	"lessmess/internal/opencode"
)

func integrationRequest(t *testing.T, h http.Handler, method, path, body string) *httptest.ResponseRecorder {
	t.Helper()
	r := httptest.NewRequest(method, path, strings.NewReader(body))
	r.Header.Set("Accept", "application/json")
	r.Header.Set("Content-Type", "application/json")
	r.Header.Set("X-Lessmess-UI", "1")
	w := httptest.NewRecorder()
	h.ServeHTTP(w, r)
	return w
}

func TestIntegrationHandlersNeverRetainOrReturnSecrets(t *testing.T) {
	const secret = "key-secret-canary"
	var received string
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.URL.Path == "/api/integration" && r.Method == http.MethodGet:
			w.Write([]byte(`{"data":[{"id":"p","name":"<img src=x onerror=alert(1)>","metadata":{"secret":"metadata-canary"},"methods":[{"type":"key","label":"API key","command":["argv-canary"]}],"connections":[]}]}`))
		case r.URL.Path == "/api/integration/p" && r.Method == http.MethodGet:
			w.Write([]byte(`{"data":{"id":"p","name":"Provider","metadata":{"secret":"metadata-canary"},"methods":[{"type":"key","label":"API key"}],"connections":[]}}`))
		case r.URL.Path == "/api/integration/p/connect/key" && r.Method == http.MethodPost:
			body, _ := io.ReadAll(r.Body)
			received = string(body)
			w.WriteHeader(http.StatusNoContent)
		default:
			w.WriteHeader(http.StatusNotFound)
		}
	}))
	defer upstream.Close()
	st, _ := fixtureStore(t)
	s := New(st)
	s.SetOpencode(opencode.New(upstream.URL, "service-password-canary"))

	list := do(t, s.Handler(), http.MethodGet, "/api/opencode/integrations", "")
	if list.Code != http.StatusOK || !strings.Contains(list.Body.String(), `\u003cimg`) {
		t.Fatalf("list = %d %s", list.Code, list.Body)
	}
	connect := integrationRequest(t, s.Handler(), http.MethodPost, "/api/opencode/integrations/p/connect/key", `{"key":"`+secret+`","confirm":true,"answer":{"account":"form-secret-canary"}}`)
	if connect.Code != http.StatusOK || !strings.Contains(received, secret) {
		t.Fatalf("connect = %d %s, upstream body = %s", connect.Code, connect.Body, received)
	}
	detail := do(t, s.Handler(), http.MethodGet, "/api/opencode/integrations/p", "")
	for _, response := range []string{list.Body.String(), connect.Body.String(), detail.Body.String()} {
		for _, forbidden := range []string{secret, "form-secret-canary", "metadata-canary", "argv-canary", "service-password-canary"} {
			if strings.Contains(response, forbidden) {
				t.Fatalf("response leaked %q: %s", forbidden, response)
			}
		}
	}
	if connect.Header().Get("Cache-Control") != "no-store" || connect.Header().Get("Referrer-Policy") != "no-referrer" {
		t.Fatalf("security headers = %v", connect.Header())
	}
}

func TestIntegrationMutationGuardsAndStaleCredential(t *testing.T) {
	var mutations int
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/api/integration/p" {
			w.Write([]byte(`{"data":{"id":"p","name":"Provider","methods":[],"connections":[{"type":"credential","id":"cred","label":"Current"},{"type":"env","name":"TOKEN"}]}}`))
			return
		}
		mutations++
		w.WriteHeader(http.StatusNoContent)
	}))
	defer upstream.Close()
	st, _ := fixtureStore(t)
	s := New(st)
	s.SetOpencode(opencode.New(upstream.URL, "pw"))

	for name, mutate := range map[string]func(*http.Request){
		"missing ui header": func(r *http.Request) { r.Header.Del("X-Lessmess-UI") },
		"null origin":       func(r *http.Request) { r.Header.Set("Origin", "null") },
		"cross site":        func(r *http.Request) { r.Header.Set("Sec-Fetch-Site", "cross-site") },
		"wrong content":     func(r *http.Request) { r.Header.Set("Content-Type", "text/plain") },
	} {
		t.Run(name, func(t *testing.T) {
			r := httptest.NewRequest(http.MethodDelete, "/api/opencode/integrations/p/credentials/cred", strings.NewReader(`{"confirm":true,"expectedLabel":"Current"}`))
			r.Header.Set("Content-Type", "application/json")
			r.Header.Set("X-Lessmess-UI", "1")
			mutate(r)
			w := httptest.NewRecorder()
			s.Handler().ServeHTTP(w, r)
			if w.Code != http.StatusForbidden && w.Code != http.StatusUnsupportedMediaType {
				t.Fatalf("code = %d body = %s", w.Code, w.Body)
			}
		})
	}
	stale := integrationRequest(t, s.Handler(), http.MethodDelete, "/api/opencode/integrations/p/credentials/cred", `{"confirm":true,"expectedLabel":"Old"}`)
	if stale.Code != http.StatusConflict || mutations != 0 {
		t.Fatalf("stale = %d %s, mutations = %d", stale.Code, stale.Body, mutations)
	}
	environment := integrationRequest(t, s.Handler(), http.MethodDelete, "/api/opencode/integrations/p/credentials/TOKEN", `{"confirm":true}`)
	reconnect := integrationRequest(t, s.Handler(), http.MethodPost, "/api/opencode/integrations/p/connect/key", `{"key":"replacement-canary","confirm":true,"confirmReplace":true,"expectedCredentialID":"missing"}`)
	if environment.Code != http.StatusConflict || reconnect.Code != http.StatusConflict || mutations != 0 {
		t.Fatalf("environment/reconnect = %d/%d, mutations = %d", environment.Code, reconnect.Code, mutations)
	}
	unknown := integrationRequest(t, s.Handler(), http.MethodPost, "/api/opencode/integrations/p/connect/key", `{"key":"x","extra":"no"}`)
	if unknown.Code != http.StatusBadRequest {
		t.Fatalf("unknown field = %d %s", unknown.Code, unknown.Body)
	}
	large := integrationRequest(t, s.Handler(), http.MethodPost, "/api/opencode/integrations/p/connect/key", `{"key":"`+strings.Repeat("x", integrationBodyLimit)+`"}`)
	if large.Code != http.StatusRequestEntityTooLarge {
		t.Fatalf("large = %d %s", large.Code, large.Body)
	}
	unconfirmed := integrationRequest(t, s.Handler(), http.MethodPost, "/api/opencode/integrations/p/connect/key", `{"key":"credential-canary"}`)
	wrongType := integrationRequest(t, s.Handler(), http.MethodDelete, "/api/opencode/integrations/p/credentials/cred", `{"confirm":"yes","expectedLabel":"Current"}`)
	trailing := integrationRequest(t, s.Handler(), http.MethodDelete, "/api/opencode/integrations/p/credentials/cred", `{"confirm":true,"expectedLabel":"Current"} {}`)
	if unconfirmed.Code != http.StatusUnprocessableEntity || wrongType.Code != http.StatusBadRequest || trailing.Code != http.StatusBadRequest || mutations != 0 {
		t.Fatalf("strict confirmation = %d/%d/%d mutations=%d", unconfirmed.Code, wrongType.Code, trailing.Code, mutations)
	}
	rename := integrationRequest(t, s.Handler(), http.MethodPatch, "/api/opencode/integrations/p/credentials/cred", `{"label":"Renamed","expectedLabel":"Current","confirm":true}`)
	activate := integrationRequest(t, s.Handler(), http.MethodPost, "/api/opencode/integrations/p/credentials/cred/activate", `{"expectedLabel":"Current","confirm":true}`)
	remove := integrationRequest(t, s.Handler(), http.MethodDelete, "/api/opencode/integrations/p/credentials/cred", `{"confirm":true,"expectedLabel":"Current"}`)
	if rename.Code != http.StatusOK || activate.Code != http.StatusOK || remove.Code != http.StatusOK || mutations != 3 {
		t.Fatalf("credential actions = %d/%d/%d, mutations = %d", rename.Code, activate.Code, remove.Code, mutations)
	}
}

func TestIntegrationAttemptLifecycleIsTransientAndRedacted(t *testing.T) {
	var mu sync.Mutex
	cancelled := false
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		mu.Lock()
		defer mu.Unlock()
		switch {
		case r.URL.Path == "/api/integration/p/connect/oauth" && r.Method == http.MethodPost:
			w.Write([]byte(`{"data":{"attemptID":"attempt-transient","url":"https://auth.example/start","instructions":"Enter the displayed code","mode":"code","time":{"created":1,"expires":2}}}`))
		case r.URL.Path == "/api/integration/p/connect/oauth/attempt-transient" && r.Method == http.MethodGet:
			w.Write([]byte(`{"data":{"status":"pending","message":"pending-secret-canary","time":{"created":1,"expires":2}}}`))
		case r.URL.Path == "/api/integration/p/connect/oauth/attempt-transient" && r.Method == http.MethodDelete:
			if cancelled {
				w.WriteHeader(http.StatusBadRequest)
				w.Write([]byte(`{"message":"duplicate raw-error-canary"}`))
				return
			}
			cancelled = true
			w.WriteHeader(http.StatusNoContent)
		case r.URL.Path == "/api/integration/p/connect/oauth/attempt-transient/complete":
			w.WriteHeader(http.StatusBadRequest)
			w.Write([]byte(`{"message":"callback raw-error-canary"}`))
		default:
			w.WriteHeader(http.StatusNotFound)
		}
	}))
	defer upstream.Close()
	st, _ := fixtureStore(t)
	s := New(st)
	s.SetOpencode(opencode.New(upstream.URL, "pw"))

	start := integrationRequest(t, s.Handler(), http.MethodPost, "/api/opencode/integrations/p/connect/oauth", `{"methodID":"oauth","confirm":true,"answer":{"account":"oauth-form-canary"}}`)
	if start.Code != http.StatusOK || !strings.Contains(start.Body.String(), "attempt-transient") || strings.Contains(start.Body.String(), "oauth-form-canary") {
		t.Fatalf("start = %d %s", start.Code, start.Body)
	}
	status := do(t, s.Handler(), http.MethodGet, "/api/opencode/integrations/p/attempts/oauth/attempt-transient", "")
	if status.Code != http.StatusOK || !strings.Contains(status.Body.String(), `"status":"pending"`) || strings.Contains(status.Body.String(), "pending-secret-canary") {
		t.Fatalf("status = %d %s", status.Code, status.Body)
	}
	first := integrationRequest(t, s.Handler(), http.MethodPost, "/api/opencode/integrations/p/attempts/oauth/attempt-transient/cancel", `{"confirm":true,"expectedStatus":"pending"}`)
	second := integrationRequest(t, s.Handler(), http.MethodPost, "/api/opencode/integrations/p/attempts/oauth/attempt-transient/cancel", `{"confirm":true,"expectedStatus":"pending"}`)
	callback := integrationRequest(t, s.Handler(), http.MethodPost, "/api/opencode/integrations/p/attempts/oauth/attempt-transient/complete", `{"code":"callback-secret-canary","confirm":true,"expectedStatus":"pending"}`)
	if first.Code != http.StatusOK || second.Code != http.StatusUnprocessableEntity || callback.Code != http.StatusUnprocessableEntity {
		t.Fatalf("lifecycle = %d/%d/%d", first.Code, second.Code, callback.Code)
	}
	for _, body := range []string{second.Body.String(), callback.Body.String()} {
		if strings.Contains(body, "raw-error-canary") || strings.Contains(body, "callback-secret-canary") {
			t.Fatalf("raw attempt failure leaked: %s", body)
		}
	}
}
