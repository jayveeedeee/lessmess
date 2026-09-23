package server

import (
	"encoding/json"
	"net/http"
	"os"
	"os/exec"
	"strings"
	"testing"
)

func getSettingsView(t *testing.T, s *Server) settingsResponse {
	t.Helper()
	w := do(t, s.Handler(), "GET", "/api/settings", "")
	if w.Code != 200 {
		t.Fatalf("GET /api/settings: %d %s", w.Code, w.Body)
	}
	var resp settingsResponse
	if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
		t.Fatal(err)
	}
	return resp
}

func TestSettingsAPIDefaults(t *testing.T) {
	s := mappingServer(t, nil)
	resp := getSettingsView(t, s)
	if resp.Project != nil || resp.Personal != nil {
		t.Error("layers must be null when their files do not exist")
	}
	if !resp.Effective.UI.ShowArchived || !resp.Effective.Docs.AutoGardenerOnClose {
		t.Errorf("effective defaults = %+v", resp.Effective)
	}
	if resp.LoadError != "" {
		t.Errorf("loadError = %q", resp.LoadError)
	}
	for field, src := range resp.Sources {
		if src != "default" {
			t.Errorf("sources[%s] = %q", field, src)
		}
	}
}

func TestSettingsAPIPutScopesAndClear(t *testing.T) {
	s := mappingServer(t, nil)

	// Write project layer.
	w := do(t, s.Handler(), "PUT", "/api/settings?scope=project",
		`{"session":{"agent":"build"},"git":{"defaultBranch":"main"}}`)
	if w.Code != 200 {
		t.Fatalf("PUT project: %d %s", w.Code, w.Body)
	}
	resp := getSettingsView(t, s)
	if resp.Project == nil || resp.Project.Session.Agent != "build" {
		t.Fatalf("project layer = %+v", resp.Project)
	}
	if resp.Effective.Session.Agent != "build" || resp.Sources["session.agent"] != "project" {
		t.Errorf("effective agent = %q (%s)", resp.Effective.Session.Agent, resp.Sources["session.agent"])
	}
	if _, err := os.Stat(settingsProjectPath(s.st.Dir)); err != nil {
		t.Error("lessmess.json not written at repo root")
	}

	// Personal override wins.
	w = do(t, s.Handler(), "PUT", "/api/settings?scope=personal", `{"session":{"agent":"plan"}}`)
	if w.Code != 200 {
		t.Fatalf("PUT personal: %d %s", w.Code, w.Body)
	}
	var putResp settingsResponse
	json.Unmarshal(w.Body.Bytes(), &putResp) // PUT returns the updated view
	if putResp.Effective.Session.Agent != "plan" || putResp.Sources["session.agent"] != "personal" {
		t.Errorf("after personal PUT: agent = %q (%s)", putResp.Effective.Session.Agent, putResp.Sources["session.agent"])
	}
	if putResp.Effective.Git.DefaultBranch != "main" || putResp.Sources["git.defaultBranch"] != "project" {
		t.Errorf("project setting lost: %+v", putResp.Effective.Git)
	}

	// Clearing the personal field restores the project value.
	w = do(t, s.Handler(), "PUT", "/api/settings?scope=personal", `{"session":{"agent":""}}`)
	if w.Code != 200 {
		t.Fatalf("PUT clear: %d %s", w.Code, w.Body)
	}
	resp = getSettingsView(t, s)
	if resp.Effective.Session.Agent != "build" || resp.Sources["session.agent"] != "project" {
		t.Errorf("after clear: agent = %q (%s), want build (project)", resp.Effective.Session.Agent, resp.Sources["session.agent"])
	}
}

func TestSettingsAPIPutRejects(t *testing.T) {
	s := mappingServer(t, nil)
	cases := []struct {
		name, path, body string
		code             int
	}{
		{"bad scope", "/api/settings?scope=nowhere", `{}`, 400},
		{"missing scope", "/api/settings", `{}`, 400},
		{"malformed", "/api/settings?scope=project", `{`, 422},
		{"unknown section", "/api/settings?scope=project", `{"nope":{}}`, 422},
		{"unknown field", "/api/settings?scope=project", `{"session":{"agenta":"x"}}`, 422},
	}
	for _, tc := range cases {
		w := do(t, s.Handler(), "PUT", tc.path, tc.body)
		if w.Code != tc.code {
			t.Errorf("%s: code = %d, want %d (%s)", tc.name, w.Code, tc.code, w.Body)
		}
	}
}

func TestSettingsAPILoadErrorSurfaced(t *testing.T) {
	s := mappingServer(t, nil)
	if err := os.WriteFile(settingsProjectPath(s.st.Dir), []byte("{broken"), 0o644); err != nil {
		t.Fatal(err)
	}
	resp := getSettingsView(t, s)
	if !strings.Contains(resp.LoadError, "lessmess.json") {
		t.Errorf("loadError = %q, want mention of lessmess.json", resp.LoadError)
	}
	if !resp.Effective.UI.ShowArchived {
		t.Error("defaults must still materialize")
	}
}

// optionsFake serves one primary agent and one model, location-aware.
func optionsFake(w http.ResponseWriter, r *http.Request) {
	switch r.URL.Path {
	case "/api/agent":
		w.Write([]byte(`{"data":[{"id":"build","name":"Build","mode":"primary"},{"id":"general","name":"General","mode":"subagent"}]}`))
	case "/api/model":
		w.Write([]byte(`{"data":[{"id":"m1","providerID":"prov","name":"M1"}]}`))
	default:
		w.WriteHeader(http.StatusNotFound)
	}
}

func TestSettingsAPIValidatesAgentAndModel(t *testing.T) {
	s := mappingServer(t, optionsFake)

	// Unknown agent → 422, nothing written.
	w := do(t, s.Handler(), "PUT", "/api/settings?scope=project", `{"session":{"agent":"ghost"}}`)
	if w.Code != 422 {
		t.Fatalf("unknown agent: code = %d, want 422 (%s)", w.Code, w.Body)
	}
	if !strings.Contains(w.Body.String(), "ghost") {
		t.Errorf("error must name the value: %s", w.Body)
	}
	if _, err := os.Stat(settingsProjectPath(s.st.Dir)); !os.IsNotExist(err) {
		t.Error("rejected PUT must not write the file")
	}

	// Subagent-mode agent is not a valid session driver.
	if w := do(t, s.Handler(), "PUT", "/api/settings?scope=project", `{"session":{"agent":"general"}}`); w.Code != 422 {
		t.Errorf("subagent: code = %d, want 422", w.Code)
	}

	// Unknown model → 422.
	if w := do(t, s.Handler(), "PUT", "/api/settings?scope=project", `{"session":{"model":"prov/nope"}}`); w.Code != 422 {
		t.Errorf("unknown model: code = %d, want 422", w.Code)
	}

	// Unknown gardener model → 422, naming the field.
	w2 := do(t, s.Handler(), "PUT", "/api/settings?scope=personal", `{"docs":{"gardenerModel":"prov/nope"}}`)
	if w2.Code != 422 {
		t.Errorf("unknown gardener model: code = %d, want 422 (%s)", w2.Code, w2.Body)
	}
	if !strings.Contains(w2.Body.String(), "gardener") {
		t.Errorf("gardener validation error must name the field: %s", w2.Body)
	}

	// Valid gardener model saves.
	if w := do(t, s.Handler(), "PUT", "/api/settings?scope=personal", `{"docs":{"gardenerModel":"prov/m1"}}`); w.Code != 200 {
		t.Errorf("valid gardener model: code = %d (%s)", w.Code, w.Body)
	}

	// Clearing it (empty) skips validation and restores inheritance.
	if w := do(t, s.Handler(), "PUT", "/api/settings?scope=personal", `{"docs":{"gardenerModel":""}}`); w.Code != 200 {
		t.Errorf("clear gardener model: code = %d (%s)", w.Code, w.Body)
	}

	// Valid values save.
	if w := do(t, s.Handler(), "PUT", "/api/settings?scope=project", `{"session":{"agent":"build","model":"prov/m1"}}`); w.Code != 200 {
		t.Errorf("valid values: code = %d (%s)", w.Code, w.Body)
	}

	// Non-session sections skip validation entirely (no service calls
	// needed) and save fine.
	if w := do(t, s.Handler(), "PUT", "/api/settings?scope=project", `{"prompts":{"gardener":"x"}}`); w.Code != 200 {
		t.Errorf("prompts: code = %d (%s)", w.Code, w.Body)
	}
}

func TestSettingsAPIValidationSkippedWhenServiceDown(t *testing.T) {
	failing := func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
		w.Write([]byte(`{"message":"boom"}`))
	}
	s := mappingServer(t, failing)
	// Values cannot be checked → saved as typed (documented behavior).
	if w := do(t, s.Handler(), "PUT", "/api/settings?scope=project", `{"session":{"agent":"ghost"}}`); w.Code != 200 {
		t.Fatalf("offline save: code = %d, want 200 (%s)", w.Code, w.Body)
	}
}

func TestSettingsOptionsLive(t *testing.T) {
	fake := func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/api/agent":
			w.Write([]byte(`{"data":[
				{"id":"build","name":"Build","description":"The default agent.","mode":"primary","hidden":false},
				{"id":"plan","name":"Plan","mode":"primary","hidden":false},
				{"id":"general","name":"General","mode":"subagent"},
				{"id":"shy","name":"Shy","mode":"primary","hidden":true}
			]}`))
		case "/api/model":
			w.Write([]byte(`{"data":[
				{"id":"accounts/f/m1","providerID":"prov","name":"Model One"},
				{"id":"m2","providerID":"other","name":"Model Two"}
			]}`))
		case "/api/model/default":
			w.Write([]byte(`{"data":{"id":"accounts/f/m1","providerID":"prov","name":"Model One"}}`))
		default:
			w.WriteHeader(http.StatusNotFound)
		}
	}
	s := mappingServer(t, fake)

	w := do(t, s.Handler(), "GET", "/api/settings/options", "")
	if w.Code != 200 {
		t.Fatalf("code = %d", w.Code)
	}
	var resp settingsOptionsResponse
	json.Unmarshal(w.Body.Bytes(), &resp)
	if !resp.Available {
		t.Fatal("available = false with a live service")
	}
	// Only primary, non-hidden agents.
	if len(resp.Agents) != 2 || resp.Agents[0].ID != "build" || resp.Agents[1].ID != "plan" {
		t.Errorf("agents = %+v", resp.Agents)
	}
	if len(resp.Models) != 2 || resp.Models[0].Value != "prov/accounts/f/m1" {
		t.Errorf("models = %+v", resp.Models)
	}
	if resp.DefaultModel != "prov/accounts/f/m1" {
		t.Errorf("defaultModel = %q", resp.DefaultModel)
	}
}

// scopedEmptyFake mirrors the real service outside its home location: the
// location-scoped agent list is empty while the default-location list has
// the built-in primaries.
func scopedEmptyFake(w http.ResponseWriter, r *http.Request) {
	switch r.URL.Path {
	case "/api/agent":
		if r.URL.Query().Get("location[directory]") != "" {
			w.Write([]byte(`{"data":[]}`))
			return
		}
		w.Write([]byte(`{"data":[{"id":"build","name":"Build","mode":"primary"},{"id":"plan","name":"Plan","mode":"primary"}]}`))
	case "/api/model":
		w.Write([]byte(`{"data":[{"id":"m1","providerID":"prov","name":"M1"}]}`))
	default:
		w.WriteHeader(http.StatusNotFound)
	}
}

func TestSettingsAgentsMergedWhenScopedEmpty(t *testing.T) {
	s := mappingServer(t, scopedEmptyFake)

	// Options still offer the built-in primaries.
	w := do(t, s.Handler(), "GET", "/api/settings/options", "")
	if w.Code != 200 {
		t.Fatalf("options code = %d", w.Code)
	}
	var resp settingsOptionsResponse
	json.Unmarshal(w.Body.Bytes(), &resp)
	if !resp.Available {
		t.Fatal("available = false with a live service")
	}
	if len(resp.Agents) != 2 || resp.Agents[0].ID != "build" || resp.Agents[1].ID != "plan" {
		t.Errorf("agents = %+v, want built-ins merged from the default location", resp.Agents)
	}

	// Save-time validation accepts a built-in primary for this repo…
	if w := do(t, s.Handler(), "PUT", "/api/settings?scope=project", `{"session":{"agent":"plan"}}`); w.Code != 200 {
		t.Errorf("PUT plan: code = %d (%s)", w.Code, w.Body)
	}
	// …and still rejects a typo.
	if w := do(t, s.Handler(), "PUT", "/api/settings?scope=project", `{"session":{"agent":"ghost"}}`); w.Code != 422 {
		t.Errorf("PUT ghost: code = %d, want 422", w.Code)
	}
}

func TestSettingsOptionsBranches(t *testing.T) {
	// A git-backed fixture lists its local branches even with the opencode
	// service down; a non-repo degrades to an empty list.
	s := gitFixtureServer(t, true, true)
	s.SetOpencode(nil)
	branch := func(name string) {
		t.Helper()
		cmd := exec.Command("git", "branch", name)
		cmd.Dir = s.st.Dir
		if out, err := cmd.CombinedOutput(); err != nil {
			t.Fatalf("git branch %s: %v\n%s", name, err, out)
		}
	}
	branch("feature/two")
	branch("feature/one")

	w := do(t, s.Handler(), "GET", "/api/settings/options", "")
	if w.Code != 200 {
		t.Fatalf("code = %d", w.Code)
	}
	var resp settingsOptionsResponse
	if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
		t.Fatal(err)
	}
	if len(resp.Branches) != 3 || resp.Branches[0] != "feature/one" || resp.Branches[1] != "feature/two" || resp.Branches[2] != "main" {
		t.Errorf("branches = %v, want [feature/one feature/two main]", resp.Branches)
	}
	if resp.Available {
		t.Error("available should stay false without the opencode service")
	}

	// Non-repo: empty, still 200.
	s2 := mappingServer(t, nil)
	w2 := do(t, s2.Handler(), "GET", "/api/settings/options", "")
	var resp2 settingsOptionsResponse
	json.Unmarshal(w2.Body.Bytes(), &resp2)
	if len(resp2.Branches) != 0 {
		t.Errorf("non-repo branches = %v, want empty", resp2.Branches)
	}
}

func TestSettingsOptionsDegraded(t *testing.T) {
	// No opencode client at all.
	s := mappingServer(t, nil)
	w := do(t, s.Handler(), "GET", "/api/settings/options", "")
	if w.Code != 200 {
		t.Fatalf("code = %d, want 200 (graceful)", w.Code)
	}
	var resp settingsOptionsResponse
	json.Unmarshal(w.Body.Bytes(), &resp)
	if resp.Available || len(resp.Agents) != 0 || len(resp.Models) != 0 {
		t.Errorf("resp = %+v, want unavailable with empty lists", resp)
	}

	// Service errors on the list endpoints → same graceful shape.
	failing := func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
		w.Write([]byte(`{"message":"boom"}`))
	}
	s2 := mappingServer(t, failing)
	w = do(t, s2.Handler(), "GET", "/api/settings/options", "")
	if w.Code != 200 {
		t.Fatalf("code = %d, want 200 (graceful)", w.Code)
	}
	json.Unmarshal(w.Body.Bytes(), &resp)
	if resp.Available {
		t.Error("available = true despite service errors")
	}
}
