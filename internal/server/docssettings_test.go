package server

import (
	"encoding/json"
	"net/http"
	"os"
	"path/filepath"
	"testing"

	"lessmess/internal/docs"
)

func getDocsStatus(t *testing.T, s *Server) docsRuntimeState {
	t.Helper()
	w := do(t, s.Handler(), "GET", "/docs/status", "")
	if w.Code != http.StatusOK {
		t.Fatalf("GET /docs/status: %d %s", w.Code, w.Body)
	}
	var state docsRuntimeState
	if err := json.Unmarshal(w.Body.Bytes(), &state); err != nil {
		t.Fatal(err)
	}
	return state
}

func TestDocsStatusAndInitialize(t *testing.T) {
	s := mappingServer(t, nil)
	if state := getDocsStatus(t, s); state.Enabled || state.ConfigExists || state.Active {
		t.Fatalf("default status = %+v, want off/unconfigured/inactive", state)
	}
	if w := do(t, s.Handler(), "POST", "/docs/initialize", ""); w.Code != http.StatusConflict {
		t.Fatalf("initialize while off: %d %s", w.Code, w.Body)
	}

	writeJSONFile(t, settingsPersonalPath(s.st.Dir), Settings{Docs: DocsSettings{Enabled: boolp(true)}})
	state := getDocsStatus(t, s)
	if !state.Enabled || state.ConfigExists || state.Active || state.RestartRequired {
		t.Fatalf("enabled pre-init status = %+v", state)
	}
	w := do(t, s.Handler(), "POST", "/docs/initialize", "")
	if w.Code != http.StatusCreated {
		t.Fatalf("initialize: %d %s", w.Code, w.Body)
	}
	state = getDocsStatus(t, s)
	if !state.ConfigExists || !state.ConfigReadable || state.Active || !state.RestartRequired {
		t.Fatalf("post-init status = %+v, want configured/restart required", state)
	}
	if _, err := docs.LoadConfig(s.st.Dir); err != nil {
		t.Fatalf("initialized config unreadable: %v", err)
	}

	before, err := os.ReadFile(filepath.Join(s.st.Dir, docs.ConfigFile))
	if err != nil {
		t.Fatal(err)
	}
	w = do(t, s.Handler(), "POST", "/docs/initialize", "")
	if w.Code != http.StatusOK {
		t.Fatalf("idempotent initialize: %d %s", w.Code, w.Body)
	}
	after, _ := os.ReadFile(filepath.Join(s.st.Dir, docs.ConfigFile))
	if string(after) != string(before) {
		t.Error("idempotent initialize changed the existing config")
	}
}

func TestDocsInitializeRefusesMalformedExistingConfig(t *testing.T) {
	s := mappingServer(t, nil)
	writeJSONFile(t, settingsPersonalPath(s.st.Dir), Settings{Docs: DocsSettings{Enabled: boolp(true)}})
	path := filepath.Join(s.st.Dir, docs.ConfigFile)
	original := []byte("{broken")
	if err := os.WriteFile(path, original, 0o644); err != nil {
		t.Fatal(err)
	}
	state := getDocsStatus(t, s)
	if !state.ConfigExists || state.ConfigReadable || state.ConfigError == "" {
		t.Fatalf("malformed status = %+v", state)
	}
	w := do(t, s.Handler(), "POST", "/docs/initialize", "")
	if w.Code != http.StatusConflict {
		t.Fatalf("initialize malformed config: %d %s", w.Code, w.Body)
	}
	after, _ := os.ReadFile(path)
	if string(after) != string(original) {
		t.Error("malformed config was overwritten")
	}
}

func TestDocsStatusActive(t *testing.T) {
	s, _ := docsServer(t)
	state := getDocsStatus(t, s)
	if !state.Enabled || !state.ConfigReadable || !state.Active || state.RestartRequired {
		t.Fatalf("active status = %+v", state)
	}
}
