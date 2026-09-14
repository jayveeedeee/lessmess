package server

import (
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

// setupShell builds a SetupServer on a temp dir; boot records calls and
// returns a marker handler.
func setupShell(t *testing.T, boot func(string) (http.Handler, error)) (*SetupServer, string, *int) {
	t.Helper()
	dir := t.TempDir()
	calls := 0
	s := NewSetup(dir, func(d string) (http.Handler, error) {
		calls++
		if boot != nil {
			return boot(d)
		}
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte("full server"))
		}), nil
	})
	return s, dir, &calls
}

func TestSetupServesWizardAndGuards(t *testing.T) {
	s, _, _ := setupShell(t, nil)

	// GET / serves the wizard page.
	w := do(t, s, "GET", "/", "")
	if w.Code != http.StatusOK || !strings.Contains(w.Body.String(), `id="setup-page"`) {
		t.Fatalf("GET / = %d %q", w.Code, w.Body.String())
	}

	// GET /setup serves the wizard page as well.
	w = do(t, s, "GET", "/setup", "")
	if w.Code != http.StatusOK || !strings.Contains(w.Body.String(), `id="setup-page"`) {
		t.Fatalf("GET /setup = %d", w.Code)
	}

	// Normal API routes are refused with 503 JSON.
	w = do(t, s, "GET", "/api/validate", "")
	if w.Code != http.StatusServiceUnavailable {
		t.Fatalf("/api/validate = %d, want 503", w.Code)
	}
	var body map[string]string
	if err := json.Unmarshal(w.Body.Bytes(), &body); err != nil || body["error"] == "" {
		t.Fatalf("503 body = %s", w.Body)
	}

	// HTML page GETs redirect to the wizard.
	r := httptest.NewRequest("GET", "/changes/2026-09-10-0", nil)
	r.Header.Set("Accept", "text/html")
	w = httptest.NewRecorder()
	s.ServeHTTP(w, r)
	if w.Code != http.StatusSeeOther || w.Header().Get("Location") != "/" {
		t.Fatalf("HTML GET = %d loc=%q", w.Code, w.Header().Get("Location"))
	}

	// Static assets are served (the wizard needs app.js/app.css).
	w = do(t, s, "GET", "/static/app.css", "")
	if w.Code != http.StatusOK {
		t.Fatalf("/static/app.css = %d", w.Code)
	}
}

func TestSetupBootSwapsOnce(t *testing.T) {
	s, _, calls := setupShell(t, nil)

	if err := s.tryBoot(); err != nil {
		t.Fatalf("tryBoot: %v", err)
	}
	if err := s.tryBoot(); err != nil {
		t.Fatalf("tryBoot again: %v", err)
	}
	if *calls != 1 {
		t.Fatalf("boot called %d times, want 1", *calls)
	}
	w := do(t, s, "GET", "/", "")
	if w.Body.String() != "full server" {
		t.Fatalf("after swap GET / = %q", w.Body.String())
	}
}

func TestSetupBootErrorKeepsShell(t *testing.T) {
	boom := errors.New("boom")
	s, _, calls := setupShell(t, func(string) (http.Handler, error) { return nil, boom })

	if err := s.tryBoot(); !errors.Is(err, boom) {
		t.Fatalf("tryBoot err = %v", err)
	}
	// A failed boot leaves the shell active and may be retried.
	w := do(t, s, "GET", "/", "")
	if !strings.Contains(w.Body.String(), `id="setup-page"`) {
		t.Fatalf("after failed boot GET / = %q", w.Body.String())
	}
	if err := s.tryBoot(); !errors.Is(err, boom) {
		t.Fatalf("retry err = %v", err)
	}
	if *calls != 2 {
		t.Fatalf("boot called %d times, want 2 (retry after failure)", *calls)
	}
}

func TestSetupCompleteAndDismiss(t *testing.T) {
	s, dir, _ := setupShell(t, nil)

	if !onboardingPending(dir) {
		t.Fatal("fresh dir must be pending")
	}
	w := do(t, s, "POST", "/api/setup/dismiss", "")
	if w.Code != http.StatusOK {
		t.Fatalf("dismiss = %d", w.Code)
	}
	if onboardingPending(dir) {
		t.Fatal("dismissed: want not pending")
	}

	w = do(t, s, "POST", "/api/setup/complete", "")
	if w.Code != http.StatusOK {
		t.Fatalf("complete = %d", w.Code)
	}
	st := loadOnboarding(dir)
	if st.CompletedAt == "" || !st.Dismissed {
		t.Fatalf("state = %+v, want completed and still dismissed", st)
	}
}

func TestSetupRoutesOnNormalServer(t *testing.T) {
	st, dir := fixtureStore(t)
	h := New(st).Handler()

	w := do(t, h, "GET", "/setup", "")
	if w.Code != http.StatusOK || !strings.Contains(w.Body.String(), `id="setup-page"`) {
		t.Fatalf("GET /setup on normal server = %d", w.Code)
	}
	// The normal server keeps serving its own routes alongside.
	w = do(t, h, "GET", "/", "")
	if w.Code != http.StatusOK || !strings.Contains(w.Header().Get("Content-Type"), "application/json") {
		t.Fatalf("GET / JSON on normal server = %d", w.Code)
	}
	w = do(t, h, "POST", "/api/setup/complete", "")
	if w.Code != http.StatusOK {
		t.Fatalf("complete on normal server = %d", w.Code)
	}
	if onboardingPending(dir) {
		t.Fatal("completed via normal server: want not pending")
	}
}
